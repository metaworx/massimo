// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// A Cube whose memory slots stay held (AP RadioHealth v1.2, Block 5,
// D192-A): two counting readings an hour apart with fewer than 5 free
// warn once, with fewer than 2 an error once, and the steps between them
// are logged; an s: line accepted shortly before a reading leaves it
// uncounted; a reading at 5 or more has them free again, which settles
// the alert; a level in the store is the level logged after a restart.

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// slotsCube is a server with a store over a fake Cube whose S replies
// and greetings say free slots, its readings on clock; seed, where set,
// writes to the store before the server starts.
type slotsCube struct {
	*rig
	clock *testkit.Clock
}

func startSlotsCube(t *testing.T, free int, seed func(st *store.Store, cubeID int64)) *slotsCube {
	t.Helper()
	clock := testkit.NewClock(time.Date(2026, 11, 15, 8, 0, 0, 0, time.UTC))
	opts := []startOpt{stored(), withFake(func(fc *fakecube.Server) { fc.SetFreeSlots(free) }),
		withOptions(Options{Queue: engine.Options{Gap: time.Millisecond, SlotWait: time.Millisecond}, checkClock: clock.Now})}
	if seed != nil {
		opts = append(opts, seedStore(seed))
	}
	c := &slotsCube{rig: startServer(t, opts...), clock: clock}
	c.waitReadings(t, 1) // the first greeting's, before the clock moves
	return c
}

// readings is how many readings of the free slots the server took.
func (c *slotsCube) readings() int {
	w := &c.s.runtimes()[0].slots
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.readings
}

// waitReadings waits until the server took n readings.
func (c *slotsCube) waitReadings(t *testing.T, n int) {
	t.Helper()
	testkit.Eventually(t, "the reading of the free slots", func() bool { return c.readings() == n })
}

// reconnect makes the server read a new greeting and waits until it
// took its reading, so that the clock moves only after it.
func (c *slotsCube) reconnect(t *testing.T) {
	t.Helper()
	n := c.readings()
	rt := c.s.runtimes()[0]
	if res := rt.queue.Enqueue(engine.Command{Key: "refresh", Reconnect: true}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
	c.waitReadings(t, n+1)
}

// events are the Cube's log events of kind, oldest first.
func (c *slotsCube) events(t *testing.T, kind string) []event {
	t.Helper()
	var evs []event
	getJSON(t, c.ts, "", "/api/v1/cubes/KEQ0000100/log?level=debug&limit=500&kind="+kind, &evs)
	return evs
}

// slotsLowOf is the Cube's slots_low in GET /api/v1/cubes/{sn}.
func (c *slotsCube) slotsLowOf(t *testing.T) *slotsLow {
	t.Helper()
	var v cubeView
	getJSON(t, c.ts, "", "/api/v1/cubes/KEQ0000100", &v)
	return v.SlotsLow
}

// slotsAlerts are the levels of the slots alerts of GET /api/v1/log/alerts.
func (c *slotsCube) slotsAlerts(t *testing.T) []string {
	t.Helper()
	var v logAlertsView
	getJSON(t, c.ts, "", "/api/v1/log/alerts", &v)
	var out []string
	for _, a := range v.Alerts {
		if a.Kind == "slots" {
			out = append(out, a.Level)
		}
	}
	return out
}

// waitSlotsEvents waits until the Cube's log has n slots events and
// returns them.
func (c *slotsCube) waitSlotsEvents(t *testing.T, n int) []event {
	t.Helper()
	var evs []event
	testkit.Eventually(t, fmt.Sprintf("%d slots events", n), func() bool {
		evs = c.events(t, "slots")
		return len(evs) == n
	})
	return evs
}

// One free slot at the first greeting and at one an hour later: one
// error, no warning before it, an alert, and slots_low since the first;
// another reading at none says no more; 50 free at the next greeting:
// free again, at info, which settles the alert.
func TestSlotsStayFull(t *testing.T) {
	c := startSlotsCube(t, 1, nil)
	t0 := c.clock.Now()
	c.clock.Add(30 * time.Minute)
	c.reconnect(t)
	c.clock.Add(31 * time.Minute)
	c.reconnect(t)
	full := c.waitSlotsEvents(t, 1)[0]
	if full.Level != "error" || full.Source != "cube" || full.Summary["state"] != slotsStateFull || full.Summary["free_slots"] != float64(1) ||
		full.Summary["since"] != t0.Format(time.RFC3339) || full.Summary["full_since"] != t0.Format(time.RFC3339) || full.Summary["from"] != nil {
		t.Fatalf("error %+v", full)
	}
	if f := c.slotsLowOf(t); f == nil || f.Level != "error" || !f.Since.Equal(t0) || f.FullSince == nil || !f.FullSince.Equal(t0) || f.FreeSlots != 1 {
		t.Fatalf("slots_low %+v", f)
	}
	if a := c.slotsAlerts(t); !slices.Equal(a, []string{"error"}) {
		t.Fatalf("slots alerts %v", a)
	}

	c.fc.SetFreeSlots(0)
	c.clock.Add(time.Hour)
	c.reconnect(t)
	testkit.Eventually(t, "the count of the last reading", func() bool { f := c.slotsLowOf(t); return f != nil && f.FreeSlots == 0 })

	c.fc.SetFreeSlots(50)
	c.reconnect(t)
	evs := c.waitSlotsEvents(t, 2)
	if evs[1].Level != "info" || evs[1].Summary["state"] != slotsStateFree || evs[1].Summary["free_slots"] != float64(50) ||
		evs[1].Summary["since"] != t0.Format(time.RFC3339) {
		t.Fatalf("events %+v", evs)
	}
	if f := c.slotsLowOf(t); f != nil {
		t.Fatalf("slots_low %+v after the slots are free", f)
	}
	if a := c.slotsAlerts(t); len(a) != 0 {
		t.Fatalf("slots alerts %v after the slots are free", a)
	}
}

// Four free for an hour: a warning; one free for an hour after that: the
// step to the error, the newest the alert; three free: the step back to
// the warning; four free still short; five free: free again (D192-A).
func TestSlotsShortThenFullAndBack(t *testing.T) {
	c := startSlotsCube(t, 4, nil)
	c.clock.Add(61 * time.Minute)
	c.reconnect(t)
	low := c.waitSlotsEvents(t, 1)[0]
	if low.Level != "warn" || low.Summary["state"] != slotsStateLow || low.Summary["free_slots"] != float64(4) || low.Summary["full_since"] != nil {
		t.Fatalf("warning %+v", low)
	}
	if f := c.slotsLowOf(t); f == nil || f.Level != "warn" || f.FullSince != nil {
		t.Fatalf("slots_low %+v", f)
	}

	c.fc.SetFreeSlots(1)
	c.reconnect(t) // below 2 from now on
	c.clock.Add(61 * time.Minute)
	c.reconnect(t)
	full := c.waitSlotsEvents(t, 2)[1]
	if full.Level != "error" || full.Summary["state"] != slotsStateFull || full.Summary["from"] != slotsStateLow {
		t.Fatalf("the step to the error %+v", full)
	}
	if a := c.slotsAlerts(t); !slices.Equal(a, []string{"error"}) {
		t.Fatalf("slots alerts %v after the step to the error", a)
	}

	c.fc.SetFreeSlots(3)
	c.reconnect(t)
	back := c.waitSlotsEvents(t, 3)[2]
	if back.Level != "warn" || back.Summary["state"] != slotsStateLow || back.Summary["from"] != slotsStateFull || back.Summary["free_slots"] != float64(3) {
		t.Fatalf("the step back %+v", back)
	}
	if f := c.slotsLowOf(t); f == nil || f.Level != "warn" {
		t.Fatalf("slots_low %+v after the step back", f)
	}
	if a := c.slotsAlerts(t); !slices.Equal(a, []string{"warn"}) {
		t.Fatalf("slots alerts %v after the step back", a)
	}

	c.fc.SetFreeSlots(4)
	c.clock.Add(2 * time.Hour)
	c.reconnect(t)
	c.fc.SetFreeSlots(5)
	c.reconnect(t)
	evs := c.waitSlotsEvents(t, 4)
	if evs[3].Level != "info" || evs[3].Summary["state"] != slotsStateFree {
		t.Fatalf("events %+v", evs)
	}
	if a := c.slotsAlerts(t); len(a) != 0 {
		t.Fatalf("slots alerts %v after the slots are free", a)
	}
}

// A reading within 15 minutes of an s: line the Cube accepted does not
// count, the greeting's either: the error comes with the first counting
// reading an hour after the first.
func TestSlotsInFlightNotCounted(t *testing.T) {
	c := startSlotsCube(t, 1, nil)
	c.clock.Add(61 * time.Minute)
	if code, out := do(t, c.ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint", `{"mode":"manual","temp":21.5}`); code != 200 {
		t.Fatalf("setpoint: %d %v", code, out)
	}
	c.reconnect(t)
	c.clock.Add(16 * time.Minute)
	c.reconnect(t)
	full := c.waitSlotsEvents(t, 1)[0]
	connected := c.events(t, "connected")
	if full.Level != "error" || full.ID < connected[2].ID {
		t.Fatalf("error %+v, before the third greeting (%d)", full, connected[2].ID)
	}
}

// A warning or an error in the store that no later slots event settled is
// the level logged: with 50 free at the first greeting after the start,
// the slots are free again; with 3 free after a warning, nothing new.
func TestSlotsSeededFromTheLog(t *testing.T) {
	since := time.Date(2026, 11, 14, 8, 0, 0, 0, time.UTC)
	seed := func(level, state string) func(st *store.Store, cubeID int64) {
		return func(st *store.Store, cubeID int64) {
			st.AddEvent(storedEvent(cubeID, event{Kind: "slots", Source: "cube", Level: level, Time: since.Add(time.Hour),
				Summary: map[string]any{"state": state, "free_slots": 1, "since": since.Format(time.RFC3339), "full_since": since.Format(time.RFC3339)}}))
		}
	}
	c := startSlotsCube(t, 50, seed("error", slotsStateFull))
	if ev := c.waitSlotsEvents(t, 2)[1]; ev.Level != "info" || ev.Summary["state"] != slotsStateFree || ev.Summary["since"] != since.Format(time.RFC3339) {
		t.Fatalf("event %+v", ev)
	}

	c = startSlotsCube(t, 3, seed("warn", slotsStateLow))
	if f := c.slotsLowOf(t); f == nil || f.Level != "warn" || !f.Since.Equal(since) {
		t.Fatalf("slots_low %+v", f)
	}
	if evs := c.events(t, "slots"); len(evs) != 1 {
		t.Fatalf("slots events %+v, the stored one alone", evs)
	}
}
