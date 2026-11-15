// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// A Cube whose memory slots stay full (AP RadioHealth v1.2, Block 5). A
// radiogram for a device that does not answer holds its memory slot until
// the device answers or the Cube restarts: one Cube held 49 of its 50 for
// months, which only its greetings showed. The Cube reports its free
// slots in the S reply to each radio line and in the H line of each
// greeting, and each of them is a reading here. A reading counts while
// nothing of Massimo is in flight to the Cube: no s: line accepted
// through its queue (the server's own, its proxy clients') for slotsQuiet
// before it, since a radiogram just sent holds a slot until its device
// takes it, at the device's next wakeup at the latest. An r: holds none.
// Two counting readings slotsHold apart, each with slotsLow free slots or
// fewer, and none above in between, give the Cube's log event slots at
// warn, once, which is an alert until the next reading above slotsLow
// gives it at info. The Cube's panel says so (slots_full in GET
// /api/v1/cubes) and points to its Reboot, which drops the held
// radiograms (cube_reboot.go). At the Cube's first greeting the watch
// takes its state from the store's newest slots event, so that a restart
// neither warns again nor forgets to say that the slots are back.

import (
	"context"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
)

// The rule of the full slots.
const (
	// slotsLow is the count of free slots at or below which a Cube's
	// slots count as full.
	slotsLow = 1
	// slotsHold is how long they stay full, from the first counting
	// reading to a later one, before the warning.
	slotsHold = time.Hour
	// slotsQuiet is how long after an s: line accepted a reading does
	// not count: its radiogram may still wait for its device's wakeup.
	slotsQuiet = 15 * time.Minute
)

// slotsFull is a Cube's memory slots full for slotsHold at least, as GET
// /api/v1/cubes shows it in slots_full.
type slotsFull struct {
	// Since is the first counting reading of the run, FreeSlots the
	// count at the last reading, Warned when the warning was given.
	Since     time.Time `json:"since"`
	FreeSlots int       `json:"free_slots"`
	Warned    time.Time `json:"warned"`
}

// slotWatch is a Cube's readings of its free slots, for the warning.
type slotWatch struct {
	mu        sync.Mutex
	seeded    bool       // the store's newest slots event read
	lastRadio time.Time  // when an s: line was last accepted
	lowSince  time.Time  // the first counting reading at slotsLow or fewer; zero for none
	full      *slotsFull // the warning given, nil for none
	readings  int        // the readings taken, which the tests wait for
}

// checkNow is the clock of the radio health checks, the readings of the
// free slots among them: Options.checkClock in the tests.
func (s *Server) checkNow() time.Time {
	if s.opts.checkClock != nil {
		return s.opts.checkClock()
	}
	return time.Now()
}

// slotsGateway is a Cube's client as its queue sees it: each radio line
// it sends is a reading of the Cube's free slots. Reconnect and
// ReconnectIfMoved are the client's, so that the queue can reconnect.
type slotsGateway struct {
	*cubelan.Client
	sent func(m cube.Message, r *cube.SendResult)
}

// SendRadio sends an s: or r: line through the client and hands its S
// reply to the slot watch.
func (g slotsGateway) SendRadio(ctx context.Context, m cube.Message) (*cube.SendResult, error) {
	r, err := g.Client.SendRadio(ctx, m)
	if err == nil && r != nil {
		g.sent(m, r)
	}
	return r, err
}

// slotsSent takes the S reply to a radio line the Cube's queue sent: an
// s: line accepted puts a radiogram in flight, and the reply is a reading
// that does not count, but for a count above slotsLow.
func (s *Server) slotsSent(rt *runtime, m cube.Message, r *cube.SendResult) {
	now := s.checkNow()
	if m.Type == 's' && !r.Discarded {
		rt.slots.mu.Lock()
		rt.slots.lastRadio = now
		rt.slots.mu.Unlock()
	}
	s.slotsRead(rt, r.FreeMemorySlots, now)
}

// slotsGreeting takes the H line of a greeting, once the greeting is
// complete: first the store's newest slots event, then the reading. A
// short H line, the portal's, says nothing of the slots.
func (s *Server) slotsGreeting(ctx context.Context, rt *runtime) {
	h := rt.client.Snapshot().Hello()
	if h == nil || len(h.Fields) <= 6 {
		return
	}
	s.seedSlots(ctx, rt)
	s.slotsRead(rt, h.FreeMemorySlots, s.checkNow())
}

// slotsRead is one reading of the Cube's free slots, taken at now: the
// warning once they stayed full for slotsHold, its counterpart once they
// are free again.
func (s *Server) slotsRead(rt *runtime, free int, now time.Time) {
	w := &rt.slots
	w.mu.Lock()
	w.readings++
	var ev *event
	switch {
	case free > slotsLow:
		if f := w.full; f != nil {
			ev = &event{Kind: "slots", Source: "cube", Level: "info", Time: time.Now(),
				Summary: map[string]any{"state": "free", "free_slots": free, "since": f.Since.UTC().Format(time.RFC3339)}}
		}
		w.lowSince, w.full = time.Time{}, nil
	case w.full != nil:
		w.full.FreeSlots = free
	case !w.lastRadio.IsZero() && now.Sub(w.lastRadio) < slotsQuiet:
		// a radiogram of Massimo's may hold a slot: no count
	case w.lowSince.IsZero():
		w.lowSince = now
	case now.Sub(w.lowSince) >= slotsHold:
		w.full = &slotsFull{Since: w.lowSince, FreeSlots: free, Warned: now}
		ev = &event{Kind: "slots", Source: "cube", Level: "warn", Time: time.Now(),
			Summary: map[string]any{"state": "full", "free_slots": free, "since": w.lowSince.UTC().Format(time.RFC3339)}}
	}
	w.mu.Unlock()
	if ev != nil {
		rt.emit(*ev)
	}
}

// seedSlots takes, once, the watch's state from the Cube's newest slots
// event in the store: a warning there that nothing settled is the
// warning given. A Cube without a row in the store has none to read.
func (s *Server) seedSlots(ctx context.Context, rt *runtime) {
	w := &rt.slots
	w.mu.Lock()
	done := w.seeded
	w.mu.Unlock()
	id := rt.storeID.Load()
	if done || rt.store == nil || id == 0 {
		return
	}
	evs, err := s.storedLog(ctx, rt.store, map[int64]string{id: rt.cubeSerial()}, logQuery{kind: "slots", minLevel: store.LevelDebug, limit: 1})
	if err != nil {
		s.log.Warn("slots: the log's last slots event not read", "cube", rt.cubeSerial(), "error", err)
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seeded = true
	if len(evs) == 0 || evs[0].Level != "warn" || w.full != nil {
		return
	}
	since, err := time.Parse(time.RFC3339, summaryText(&evs[0], "since"))
	if err != nil {
		since = evs[0].Time
	}
	free := slotsLow
	if n, ok := evs[0].Summary["free_slots"].(float64); ok {
		free = int(n)
	}
	w.lowSince, w.full = since, &slotsFull{Since: since, FreeSlots: free, Warned: evs[0].Time}
}

// slotsState is the Cube's warning of full slots for its view, nil for
// none.
func (rt *runtime) slotsState() *slotsFull {
	rt.slots.mu.Lock()
	defer rt.slots.mu.Unlock()
	if rt.slots.full == nil {
		return nil
	}
	f := *rt.slots.full
	return &f
}
