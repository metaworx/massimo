// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// A Cube whose memory slots stay held (AP RadioHealth v1.2, Block 5,
// D192-A). A radiogram for a device that does not answer holds its
// memory slot until the device answers or the Cube restarts: one Cube
// held 49 of its 50 for months, which only its greetings showed. The
// Cube reports its free slots in the S reply to each radio line and in
// the H line of each greeting, and each of them is a reading here. A
// reading counts while nothing of Massimo is in flight to the Cube: no s:
// line accepted through its queue (the server's own, its proxy clients')
// for slotsQuiet before it, since a radiogram just sent holds a slot
// until its device takes it, at the device's next wakeup at the latest.
// An r: holds none. Two counting readings slotsHold apart with fewer than
// slotsWarnBelow free, and none with more in between, give the Cube's log
// event slots at warn (state low); two with fewer than slotsErrorBelow,
// at error (state full); each once in a run. A reading with
// slotsErrorBelow or more ends the run below it, its step back to low
// logged at warn; one with slotsWarnBelow or more ends the run, at info
// (state free). The newest slots event is the alert. The Cube's panel
// says so (slots_low in GET /api/v1/cubes) and points to its Reboot,
// which drops the held radiograms (cube_reboot.go). At the Cube's first
// greeting the watch takes its state from the store's newest slots event,
// so that a restart neither warns again nor forgets to say that the
// slots are back.

import (
	"context"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
)

// The rule of the held slots (D192-A).
const (
	// slotsWarnBelow is the count of free slots below which a Cube's
	// slots are short, a warning once it lasts; the queue holds radio
	// commands below it too (engine.Options.MinFreeSlots).
	slotsWarnBelow = 5
	// slotsErrorBelow is the count below which they are full, an error
	// once it lasts.
	slotsErrorBelow = 2
	// slotsHold is how long they stay so, from the first counting reading
	// to a later one, before the warning or the error.
	slotsHold = time.Hour
	// slotsQuiet is how long after an s: line accepted a reading does
	// not count: its radiogram may still wait for its device's wakeup.
	slotsQuiet = 15 * time.Minute
)

// The states of a slots event, in its summary.
const (
	slotsStateLow  = "low"  // fewer than slotsWarnBelow free, at warn
	slotsStateFull = "full" // fewer than slotsErrorBelow free, at error
	slotsStateFree = "free" // slotsWarnBelow or more free again, at info
)

// slotsLow is a Cube's memory slots short or full for slotsHold at least,
// as GET /api/v1/cubes shows it in slots_low.
type slotsLow struct {
	// Level is warn, fewer than slotsWarnBelow free, or error, fewer than
	// slotsErrorBelow; Since the first counting reading below
	// slotsWarnBelow of the run, FullSince the first below
	// slotsErrorBelow, for error; FreeSlots the count at the last
	// reading; Logged when the level was logged.
	Level     string     `json:"level"`
	Since     time.Time  `json:"since"`
	FullSince *time.Time `json:"full_since,omitempty"`
	FreeSlots int        `json:"free_slots"`
	Logged    time.Time  `json:"logged"`
}

// slotWatch is a Cube's readings of its free slots, for the warning and
// the error.
type slotWatch struct {
	mu        sync.Mutex
	seeded    bool      // the store's newest slots event read
	lastRadio time.Time // when an s: line was last accepted
	lowSince  time.Time // the first counting reading below slotsWarnBelow; zero for none
	fullSince time.Time // the first counting reading below slotsErrorBelow; zero for none
	level     string    // the level logged in the run: "", warn or error
	logged    time.Time // when it was logged
	free      int       // the count at the last reading
	readings  int       // the readings taken, which the tests wait for
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
// that does not count, but for a count that ends a run.
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
// warning or the error once they stayed short or full for slotsHold, the
// step back from full to short, and the counterpart once they are free
// again.
func (s *Server) slotsRead(rt *runtime, free int, now time.Time) {
	w := &rt.slots
	w.mu.Lock()
	w.readings++
	w.free = free
	var ev *event
	if free >= slotsWarnBelow {
		if w.level != "" {
			ev = w.event(slotsStateFree, "", now)
		}
		w.lowSince, w.fullSince, w.level = time.Time{}, time.Time{}, ""
	} else {
		if free >= slotsErrorBelow {
			w.fullSince = time.Time{}
			if w.level == "error" {
				w.level, w.logged = "warn", now
				ev = w.event(slotsStateLow, slotsStateFull, now)
			}
		}
		if w.lastRadio.IsZero() || now.Sub(w.lastRadio) >= slotsQuiet { // a radiogram of Massimo's may hold a slot: no count
			if w.lowSince.IsZero() {
				w.lowSince = now
			}
			if free < slotsErrorBelow && w.fullSince.IsZero() {
				w.fullSince = now
			}
			switch from := w.level; {
			case from != "error" && !w.fullSince.IsZero() && now.Sub(w.fullSince) >= slotsHold:
				w.level, w.logged = "error", now
				was := ""
				if from == "warn" {
					was = slotsStateLow
				}
				ev = w.event(slotsStateFull, was, now)
			case from == "" && now.Sub(w.lowSince) >= slotsHold:
				w.level, w.logged = "warn", now
				ev = w.event(slotsStateLow, "", now)
			}
		}
	}
	w.mu.Unlock()
	if ev != nil {
		rt.emit(*ev)
	}
}

// event is the slots event of a state, a step from another where from is
// set, as the watch has it now; w.mu is held.
func (w *slotWatch) event(state, from string, now time.Time) *event {
	level := map[string]string{slotsStateLow: "warn", slotsStateFull: "error", slotsStateFree: "info"}[state]
	summary := map[string]any{"state": state, "free_slots": w.free, "since": w.lowSince.UTC().Format(time.RFC3339)}
	if !w.fullSince.IsZero() {
		summary["full_since"] = w.fullSince.UTC().Format(time.RFC3339)
	}
	if from != "" {
		summary["from"] = from
	}
	return &event{Kind: "slots", Source: "cube", Level: level, Time: time.Now(), Summary: summary}
}

// seedSlots takes, once, the watch's state from the Cube's newest slots
// event in the store: a warning or an error there, which only the next
// slots event settles, is the level logged. A Cube without a row in the
// store has none to read.
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
	if len(evs) == 0 || evs[0].Level != "warn" && evs[0].Level != "error" || w.level != "" {
		return
	}
	at := func(key string) time.Time {
		t, err := time.Parse(time.RFC3339, summaryText(&evs[0], key))
		if err != nil {
			return evs[0].Time
		}
		return t
	}
	w.level, w.logged, w.lowSince = evs[0].Level, evs[0].Time, at("since")
	if w.level == "error" {
		w.fullSince = at("full_since")
	}
	w.free = slotsErrorBelow - 1
	if n, ok := evs[0].Summary["free_slots"].(float64); ok {
		w.free = int(n)
	}
}

// slotsState is the Cube's short or full slots for its view, nil while
// no level is logged.
func (rt *runtime) slotsState() *slotsLow {
	w := &rt.slots
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.level == "" {
		return nil
	}
	v := &slotsLow{Level: w.level, Since: w.lowSince, FreeSlots: w.free, Logged: w.logged}
	if w.level == "error" {
		full := w.fullSince
		v.FullSince = &full
	}
	return v
}
