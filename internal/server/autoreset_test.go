// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// resetsOn is the start of the automatic resets' tests: the Cube a row
// of a store, polled often so that a state the fake is given arrives,
// the resets and the error watch on clock's time.
func resetsOn(clock *testkit.Clock) startOpt {
	return func(c *startConfig) {
		stored()(c)
		editOptions(func(o *Options) { o.PollInterval, o.clock = 50*time.Millisecond, kitClock{clock} })(c)
	}
}

// resetOf matches the r: line for rf.
func resetOf(rf cube.RFAddress) func(cube.Message) bool {
	return lineIs(cube.CmdResetError(rf).String())
}

// A flagged device that reports is reset once (AP RadioHealth v1.2,
// Block 4, D102-B), with an error_reset at info; its next report the
// same day is not, one a day later is, and a server started anew over
// the same store reads the last reset from the log and sends none. A
// device with the link error alone is never reset. Block 6's hold is
// left out: the fake Cube's r: does not clear the flag, so it would be
// back at every report after the first (TestErrorWatchBack).
func TestAutoReset(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC))
	noHold := beforeRun(func(s *Server) { s.runtimes()[0].resets.hold = nil })
	r := startServer(t, resetsOn(clock), noHold)
	fc, ts, s, st, received := r.fc, r.ts, r.s, r.st, r.lines

	const rf = cube.RFAddress(0x0a010d) // room 6
	on := true
	valve := func(n int) *int { return &n }
	settleState(t, fc, ts, s, rf, fakecube.State{Reported: &on, Error: &on, ValvePct: valve(10)}, "valve_pct", float64(10))
	if n := received.Count(0, resetOf(rf)); n != 1 {
		t.Fatalf("the first report: %d resets, want 1", n)
	}
	var ev event
	testkit.Eventually(t, "the error_reset event", func() bool {
		var evs []event
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log?limit=500", &evs)
		for _, e := range evs {
			if e.Kind == "error_reset" && e.Device == "0a010d" {
				ev = e
				return true
			}
		}
		return false
	})
	if ev.Level != "info" || ev.Source != "device" || ev.Summary["room"] != float64(6) || ev.Summary["automatic"] != true {
		t.Fatalf("event %+v", ev)
	}

	clock.Add(23 * time.Hour)
	settleState(t, fc, ts, s, rf, fakecube.State{ValvePct: valve(20)}, "valve_pct", float64(20))
	if n := received.Count(0, resetOf(rf)); n != 1 {
		t.Fatalf("a report the same day: %d resets, want 1", n)
	}
	clock.Add(2 * time.Hour)
	settleState(t, fc, ts, s, rf, fakecube.State{ValvePct: valve(30)}, "valve_pct", float64(30))
	if n := received.Count(0, resetOf(rf)); n != 2 {
		t.Fatalf("a report a day later: %d resets, want 2", n)
	}

	// The second reset's event is in the store; a new server reads it.
	testkit.Eventually(t, "the second error_reset", func() bool {
		n := 0
		for _, k := range logKinds(t, ts, 0) {
			if k == "error_reset:0a010d" {
				n++
			}
		}
		return n == 2
	})
	if err := st.FlushEvents(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.stop()
	clock.Add(time.Hour)
	r = startServer(t, resetsOn(clock), again(r), noHold)
	ts, s = r.ts, r.s
	settleState(t, fc, ts, s, rf, fakecube.State{ValvePct: valve(40)}, "valve_pct", float64(40))
	if n := received.Count(0, resetOf(rf)); n != 2 {
		t.Fatalf("a report after the restart: %d resets, want 2", n)
	}

	// The link error alone: no reset.
	const linked = cube.RFAddress(0x0a0108)
	settleState(t, fc, ts, s, linked, fakecube.State{Reported: &on, LinkError: &on, ValvePct: valve(15)}, "link_error", true)
	if n := received.Count(0, resetOf(linked)); n != 0 {
		t.Fatalf("the link error alone: %d resets, want 0", n)
	}
}

// The hold hook (Block 6's) holds a flagged device back: it is asked
// with the device and the report's time, and nothing is sent.
func TestAutoResetHold(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC))
	var mu sync.Mutex
	var asked []string
	r := startServer(t, resetsOn(clock), beforeRun(func(s *Server) {
		s.runtimes()[0].resets.hold = func(rf cube.RFAddress, at time.Time) bool {
			mu.Lock()
			defer mu.Unlock()
			asked = append(asked, rf.String()+"@"+at.Format(time.RFC3339))
			return true
		}
	}))
	fc, ts, s, received := r.fc, r.ts, r.s, r.lines
	on := true
	settleState(t, fc, ts, s, 0x0a010d, fakecube.State{Reported: &on, Error: &on}, "error", true)
	if n := received.Count(0, resetOf(0x0a010d)); n != 0 {
		t.Fatalf("held: %d resets, want 0", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(asked) != 1 || asked[0] != "0a010d@2026-11-15T12:00:00Z" {
		t.Fatalf("the hook was asked %v", asked)
	}
}

// While the Cube's row in the store is not known, the log of its resets
// cannot be read, and a flagged device that reports is not reset: a
// reset less than a day old may be there. Once the row is known, the
// device's next report reads the log and resets it.
func TestAutoResetWaitsForTheStoresRow(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	clock := testkit.NewClock(time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC))
	// A store without a row for the Cube: the runtime has no store ID.
	r := startServer(t, withOptions(Options{Store: st, PollInterval: 50 * time.Millisecond, clock: kitClock{clock}}))
	fc, addr, ts, s, received := r.fc, r.addr, r.ts, r.s, r.lines
	rt := s.runtimes()[0]
	const rf = cube.RFAddress(0x0a010d)
	on := true
	valve := func(n int) *int { return &n }
	settleState(t, fc, ts, s, rf, fakecube.State{Reported: &on, Error: &on, ValvePct: valve(10)}, "valve_pct", float64(10))
	if id := rt.storeID.Load(); id != 0 {
		t.Fatalf("the runtime learned a store ID %d: the test needs none", id)
	}
	if n := received.Count(0, resetOf(rf)); n != 0 {
		t.Fatalf("without the store's row: %d resets, want 0", n)
	}

	row, err := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv})
	if err != nil {
		t.Fatal(err)
	}
	rt.storeID.Store(row.ID)
	settleState(t, fc, ts, s, rf, fakecube.State{ValvePct: valve(20)}, "valve_pct", float64(20))
	if n := received.Count(0, resetOf(rf)); n != 1 {
		t.Fatalf("with the row known: %d resets, want 1", n)
	}
}
