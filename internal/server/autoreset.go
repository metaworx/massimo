// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// A flagged device that reports is reset (AP RadioHealth v1.2, Block 4,
// D102-B). The Cube sets a device's communication error, byte 5's 0x08
// of its L entry, when its last radiogram to the device failed, and only
// an r: or a Cube restart clears it, never the device's own reports
// (docs/cube-protocol.md). A device that reports while flagged can be
// reached again: serve then sends it one r:01 through the queue, which
// costs no radio time and takes no slot, at low priority and past the
// queue's holds (D183-B), and logs it at info as error_reset. At most
// once a day per device; not while a command of Massimo's own to the device or its room
// waits or runs in the queue, which resets the device itself where it
// needs it (Block 2); not while the hold hook holds it (Block 6). It is
// checked at the device's report, so it needs no job queue: the Cube is
// polled every 10 s, and a changed entry is the event. The last reset of
// each device is kept, and seeded from the store's log, its error_reset
// events, at the first check once the Cube's row in the store is known,
// so that a restart does not set it back; until then no reset goes out.

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// autoResetGap is the least time between two automatic resets of one
// device.
const autoResetGap = 24 * time.Hour

// autoResets are a Cube's automatic resets: the last of each device, and
// the devices whose reset is in the queue.
type autoResets struct {
	// hold, where set, holds a device back from its automatic reset: it
	// is asked at each report of a flagged device rf, with the report's
	// time, before the day's limit and the queue are looked at, and true
	// leaves rf alone this time. It is the hook of a flag that keeps
	// coming back (AP RadioHealth v1.2, Block 6), which leaves such a
	// device alone until it stays clear for a day. Set it before the
	// runtime starts; only the pump calls it, with no lock held.
	hold func(rf cube.RFAddress, at time.Time) bool

	mu     sync.Mutex
	last   map[cube.RFAddress]time.Time
	queued map[cube.RFAddress]bool
	seeded bool // the store's log was read, or there is none to read
}

// lastReset is rf's last automatic reset, and whether it had one in the
// last day or since the start: what Block 6 asks of a flag that came
// back.
func (a *autoResets) lastReset(rf cube.RFAddress) (time.Time, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t, ok := a.last[rf]
	return t, ok
}

// claim reports whether rf's automatic reset is due at now, neither
// queued nor less than autoResetGap after the last, and marks it queued
// where it is.
func (a *autoResets) claim(rf cube.RFAddress, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.queued[rf] {
		return false
	}
	if t, ok := a.last[rf]; ok && now.Sub(t) < autoResetGap {
		return false
	}
	if a.queued == nil {
		a.queued = map[cube.RFAddress]bool{}
	}
	a.queued[rf] = true
	return true
}

// done ends rf's queued reset: sent at at, or not sent.
func (a *autoResets) done(rf cube.RFAddress, at time.Time, sent bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.queued, rf)
	if sent {
		a.noteLocked(rf, at)
	}
}

// noteLocked keeps at as rf's last reset where it is later than the one
// kept.
func (a *autoResets) noteLocked(rf cube.RFAddress, at time.Time) {
	if a.last == nil {
		a.last = map[cube.RFAddress]time.Time{}
	}
	if at.After(a.last[rf]) {
		a.last[rf] = at
	}
}

// deviceValues are the values of a state that the device reports
// itself, as view.Changes names them: a change of any of them, or the
// Cube holding a state for it again ("reported"), means the Cube heard
// it. The communication error, and the failed radiogram that comes with
// it, are the Cube's own.
var deviceValues = []string{"reported", "mode", "setpoint", "until", "actual_temp", "valve_pct", "window_open", "battery_low", "link_error", "panel_locked"}

// reported reports whether a state event's changes say the device was
// heard.
func reported(changes map[string]view.Change) bool {
	for _, k := range deviceValues {
		if _, ok := changes[k]; ok {
			return true
		}
	}
	return false
}

// now is the time of the pump's events, the automatic resets and the
// error watch (errorwatch.go): Options.clock in the tests.
func (s *Server) now() time.Time { return s.clock().Now() }

// autoReset resets rf's communication error where dv, its state event's
// view, and changes, the event's, are a report of the flagged device,
// and the reset is due. Only the pump calls it.
func (s *Server) autoReset(ctx context.Context, rt *runtime, rf cube.RFAddress, dv view.Device, changes map[string]view.Change) {
	if dv.Virtual || dv.State == nil || !dv.State.Error || !reported(changes) {
		return
	}
	now := s.now()
	if !s.seedResets(ctx, rt, now) {
		return // the device's next report tries again
	}
	if h := rt.resets.hold; h != nil && h(rf, now) {
		return
	}
	if rt.queue.Concerns(rf) || !rt.resets.claim(rf, now) {
		return
	}
	var room uint8
	if c := rt.client.Snapshot().Config(rf); c != nil {
		room = c.RoomID
	}
	t := rt.queue.Enqueue(engine.Command{Key: "error-reset:" + rf.String(), Priority: engine.PriorityLow, Lines: []cube.Message{cube.CmdResetError(rf)}})
	go s.autoResetDone(ctx, rt, rf, room, t)
}

// autoResetDone waits for rf's automatic reset and logs it: an
// error_reset event once the Cube took it, a warning where it did not.
func (s *Server) autoResetDone(ctx context.Context, rt *runtime, rf cube.RFAddress, room uint8, t *engine.Ticket) {
	res := t.Wait(ctx)
	at := s.now()
	sent := res.Err == nil && !res.Discarded
	if sent {
		// Before done: the error watch takes no entry before the answer
		// for a return of the flag (errorwatch.go).
		times, _ := rt.client.Snapshot().Times(rf)
		rt.errWatch.answered(rf, at, times.Changed)
	}
	rt.resets.done(rf, at, sent)
	switch {
	case sent:
		snap := rt.client.Snapshot()
		ev := event{Kind: "error_reset", Source: "device", Device: rf.String(), Serial: snap.Serial(rf), Level: "info", Time: at,
			Summary: map[string]any{"room": room, "automatic": true}}
		if _, ok := snap.State(rf); ok {
			ev.State = view.DeviceOf(snap, rf, s.staleness())
		}
		rt.emit(ev)
	case ctx.Err() != nil || errors.Is(res.Err, engine.ErrSuperseded):
	default:
		s.log.Warn("communication error not reset", "cube", rt.cubeSerial(), "device", rf, "room", room, "discarded", res.Discarded, "error", res.Err)
	}
}

// seedResets fills a Cube's last automatic resets from the store's log,
// its error_reset events of the last day, once, and reports whether they
// are seeded: true once the log was read, or for a Cube without a store,
// which has nothing to seed; false while the Cube's row in the store is
// not known yet (it may come with the connection, learnStoreID) or the
// log could not be read, so that no reset goes out that the log might
// forbid.
func (s *Server) seedResets(ctx context.Context, rt *runtime, now time.Time) bool {
	a := &rt.resets
	a.mu.Lock()
	seeded := a.seeded
	a.mu.Unlock()
	if seeded {
		return true
	}
	id := rt.storeID.Load()
	if rt.store != nil && id == 0 {
		return false
	}
	var rows []store.Event
	if rt.store != nil {
		q := store.EventQuery{CubeID: id, Kind: "error_reset", Source: "device", Since: now.Add(-autoResetGap), Limit: store.MaxEventsPage}
		for {
			page, err := rt.store.Events(ctx, q)
			if err != nil {
				s.log.Warn("automatic resets not read from the log", "cube", rt.cubeSerial(), "error", err)
				return false
			}
			rows = append(rows, page...)
			if len(page) < store.MaxEventsPage {
				break
			}
			q.BeforeID = page[len(page)-1].ID
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range rows {
		var row struct {
			Summary struct {
				Automatic bool `json:"automatic"`
			} `json:"summary"`
		}
		rf, err := cube.ParseRFAddress(r.Device)
		if err != nil || json.Unmarshal(r.Data, &row) != nil || !row.Summary.Automatic {
			continue
		}
		a.noteLocked(rf, r.Time)
	}
	a.seeded = true
	return true
}
