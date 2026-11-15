// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// A Cube whose memory slots stay full (AP RadioHealth v1.2, Block 5):
// two counting readings an hour apart with one free slot warn once; an
// s: line accepted shortly before a reading leaves it uncounted; a
// reading above one has them free again, which settles the warning; a
// warning in the store is the warning given after a restart.

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/store"
)

// testClock is a clock the test moves on.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// slotsCube is a server with a store over a fake Cube whose S replies
// and greetings say free slots, its readings on clock; seed, where set,
// writes to the store before the server starts.
type slotsCube struct {
	fc    *fakecube.Server
	ts    *httptest.Server
	s     *Server
	clock *testClock
}

func startSlotsCube(t *testing.T, free int, seed func(st *store.Store, cubeID int64)) *slotsCube {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	fc.SetFreeSlots(free)
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	st := testStore(t, "")
	row, err := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv})
	if err != nil {
		t.Fatal(err)
	}
	if seed != nil {
		seed(st, row.ID)
	}
	cfg, err := CubeFromRow(row)
	if err != nil {
		t.Fatal(err)
	}
	clock := &testClock{t: time.Date(2026, 11, 15, 8, 0, 0, 0, time.UTC)}
	s := New(Options{Cubes: []CubeConfig{cfg}, Store: st, Queue: engine.Options{Gap: time.Millisecond, SlotWait: time.Millisecond}, checkClock: clock.now})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	waitConnected(t, ts, "KEQ0000100")
	c := &slotsCube{fc: fc, ts: ts, s: s, clock: clock}
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
	waitFor(t, "the reading of the free slots", func() bool { return c.readings() == n })
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

// slotsFullOf is the Cube's slots_full in GET /api/v1/cubes/{sn}.
func (c *slotsCube) slotsFullOf(t *testing.T) *slotsFull {
	t.Helper()
	var v cubeView
	getJSON(t, c.ts, "", "/api/v1/cubes/KEQ0000100", &v)
	return v.SlotsFull
}

// hasSlotsAlert reports whether GET /api/v1/log/alerts has a slots alert.
func (c *slotsCube) hasSlotsAlert(t *testing.T) bool {
	t.Helper()
	var v logAlertsView
	getJSON(t, c.ts, "", "/api/v1/log/alerts", &v)
	for _, a := range v.Alerts {
		if a.Kind == "slots" {
			return true
		}
	}
	return false
}

// One free slot at the first greeting and at one an hour later: one
// warning, an alert, and slots_full since the first; another reading at
// one free warns no more; 50 free at the next greeting: free again, at
// info, which settles the alert.
func TestSlotsStayFull(t *testing.T) {
	c := startSlotsCube(t, 1, nil)
	t0 := c.clock.now()
	c.clock.add(30 * time.Minute)
	c.reconnect(t)
	c.clock.add(31 * time.Minute)
	c.reconnect(t)
	var warn event
	waitFor(t, "the slots warning", func() bool {
		evs := c.events(t, "slots")
		if len(evs) == 1 {
			warn = evs[0]
		}
		return len(evs) == 1
	})
	if warn.Level != "warn" || warn.Source != "cube" || warn.Summary["state"] != "full" || warn.Summary["free_slots"] != float64(1) ||
		warn.Summary["since"] != t0.Format(time.RFC3339) {
		t.Fatalf("warning %+v", warn)
	}
	if f := c.slotsFullOf(t); f == nil || !f.Since.Equal(t0) || f.FreeSlots != 1 {
		t.Fatalf("slots_full %+v", f)
	}
	if !c.hasSlotsAlert(t) {
		t.Fatal("no slots alert")
	}

	c.fc.SetFreeSlots(0)
	c.clock.add(time.Hour)
	c.reconnect(t)
	waitFor(t, "the count of the last reading", func() bool { f := c.slotsFullOf(t); return f != nil && f.FreeSlots == 0 })

	c.fc.SetFreeSlots(50)
	c.reconnect(t)
	waitFor(t, "the slots free again", func() bool { return len(c.events(t, "slots")) == 2 })
	evs := c.events(t, "slots")
	if evs[0].Level != "warn" || evs[1].Level != "info" || evs[1].Summary["state"] != "free" || evs[1].Summary["free_slots"] != float64(50) ||
		evs[1].Summary["since"] != t0.Format(time.RFC3339) {
		t.Fatalf("events %+v", evs)
	}
	if f := c.slotsFullOf(t); f != nil {
		t.Fatalf("slots_full %+v after the slots are free", f)
	}
	if c.hasSlotsAlert(t) {
		t.Fatal("the slots alert is not settled")
	}
}

// A reading within 15 minutes of an s: line the Cube accepted does not
// count, the greeting's either: the warning comes with the first
// counting reading an hour after the first.
func TestSlotsInFlightNotCounted(t *testing.T) {
	c := startSlotsCube(t, 1, nil)
	c.clock.add(61 * time.Minute)
	if code, out := do(t, c.ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint", `{"mode":"manual","temp":21.5}`); code != 200 {
		t.Fatalf("setpoint: %d %v", code, out)
	}
	c.reconnect(t)
	c.clock.add(16 * time.Minute)
	c.reconnect(t)
	waitFor(t, "the slots warning", func() bool { return len(c.events(t, "slots")) == 1 })
	connected := c.events(t, "connected")
	if warn := c.events(t, "slots")[0]; warn.Level != "warn" || warn.ID < connected[2].ID {
		t.Fatalf("warning %+v, before the third greeting (%d)", warn, connected[2].ID)
	}
}

// A warning in the store that nothing settled is the warning given: the
// first greeting after the start, with 50 free, says the slots are free
// again.
func TestSlotsSeededFromTheLog(t *testing.T) {
	since := time.Date(2026, 11, 14, 8, 0, 0, 0, time.UTC)
	c := startSlotsCube(t, 50, func(st *store.Store, cubeID int64) {
		st.AddEvent(storedEvent(cubeID, event{Kind: "slots", Source: "cube", Level: "warn", Time: since.Add(time.Hour),
			Summary: map[string]any{"state": "full", "free_slots": 1, "since": since.Format(time.RFC3339)}}))
	})
	waitFor(t, "the slots free again", func() bool { return len(c.events(t, "slots")) == 2 })
	if ev := c.events(t, "slots")[1]; ev.Level != "info" || ev.Summary["state"] != "free" || ev.Summary["since"] != since.Format(time.RFC3339) {
		t.Fatalf("event %+v", ev)
	}
}
