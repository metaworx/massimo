// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
)

// stepClock is a clock the test moves on (a notify.Clock): add runs the
// timers that come due, in their order, on the test's goroutine.
type stepClock struct {
	mu     sync.Mutex
	t      time.Time
	timers []*stepTimer
}

// stepTimer is a stepClock's timer.
type stepTimer struct {
	c       *stepClock
	at      time.Time
	f       func()
	stopped bool
}

func (t *stepTimer) Stop() bool {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	was := !t.stopped
	t.stopped = true
	return was
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *stepClock) AfterFunc(d time.Duration, f func()) notify.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &stepTimer{c: c, at: c.t.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

func (c *stepClock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	var due []*stepTimer
	kept := c.timers[:0]
	for _, t := range c.timers {
		switch {
		case t.stopped:
		case !t.at.After(c.t):
			t.stopped = true
			due = append(due, t)
		default:
			kept = append(kept, t)
		}
	}
	c.timers = kept
	c.mu.Unlock()
	slices.SortStableFunc(due, func(a, b *stepTimer) int { return a.at.Compare(b.at) })
	for _, t := range due {
		t.f()
	}
}

// pending counts the timers that have not fired or stopped.
func (c *stepClock) pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, t := range c.timers {
		if !t.stopped {
			n++
		}
	}
	return n
}

// startResets starts a server over the fake Cube at addr, the store's
// Cube KEQ0000100, whose automatic resets keep clock's time; prep, where
// given, changes the server before it runs. stop ends it.
func startResets(t *testing.T, st *store.Store, addr string, clock *stepClock, prep func(*Server)) (ts *httptest.Server, s *Server, stop func()) {
	t.Helper()
	ctx := context.Background()
	row, ok, err := st.Cube(ctx, "KEQ0000100")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		if row, err = st.AddCube(ctx, store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv}); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := CubeFromRow(row)
	if err != nil {
		t.Fatal(err)
	}
	s = New(Options{Cubes: []CubeConfig{cfg}, Store: st, PollInterval: 50 * time.Millisecond, Queue: engine.Options{Gap: time.Millisecond}, clock: clock})
	if prep != nil {
		prep(s)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { s.Run(runCtx); close(done) }()
	ts = httptest.NewServer(s.Handler())
	var once sync.Once
	stop = func() { once.Do(func() { ts.Close(); cancel(); <-done }) }
	t.Cleanup(stop)
	waitConnected(t, ts, "KEQ0000100")
	// The pump takes the greeting's states after the API shows the Cube
	// connected: a report before that would be the devices' first state
	// there, without changes.
	pumpSync(t, s)
	return ts, s, stop
}

// resetOf matches the r: line for rf.
func resetOf(rf cube.RFAddress) func(cube.Message) bool {
	return lineIs(cube.CmdResetError(rf).String())
}

// reportAndSettle makes rf report st and waits until the server's pump
// took the report, its state event in the log with key changed to to,
// then until the queue sent what the report queued: a line queued at
// low priority after the report runs after a reset the report queued.
// (The device's view in the API comes from the snapshot, before the
// pump.)
func reportAndSettle(t *testing.T, fc *fakecube.Server, ts *httptest.Server, s *Server, rf cube.RFAddress, st fakecube.State, key string, to any) {
	t.Helper()
	if err := fc.SetState(rf, st); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the report of "+rf.String(), func() bool {
		var evs []event
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log?limit=500", &evs)
		for _, e := range evs {
			if c, ok := e.Changes[key]; ok && e.Kind == "state" && e.Device == rf.String() && c.To == to {
				return true
			}
		}
		return false
	})
	rt := s.runtimes()[0]
	if res := rt.queue.Enqueue(engine.Command{Priority: engine.PriorityLow, Lines: []cube.Message{cube.CmdGetNTP}}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
}

// A flagged device that reports is reset once (AP RadioHealth v1.2,
// Block 4, D102-B), with an error_reset at info; its next report the
// same day is not, one a day later is, and a server started anew over
// the same store reads the last reset from the log and sends none. A
// device with the link error alone is never reset. Block 6's hold is
// left out: the fake Cube's r: does not clear the flag, so it would be
// back at every report after the first (TestErrorWatchBack).
func TestAutoReset(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fc.Close() })
	received := cubewatch.Lines(fc)
	st := testStore(t, "")
	clock := &stepClock{t: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	noHold := func(s *Server) { s.runtimes()[0].resets.hold = nil }
	ts, s, stop := startResets(t, st, addr, clock, noHold)

	const rf = cube.RFAddress(0x0a010d) // room 6
	on := true
	valve := func(n int) *int { return &n }
	reportAndSettle(t, fc, ts, s, rf, fakecube.State{Reported: &on, Error: &on, ValvePct: valve(10)}, "valve_pct", float64(10))
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

	clock.add(23 * time.Hour)
	reportAndSettle(t, fc, ts, s, rf, fakecube.State{ValvePct: valve(20)}, "valve_pct", float64(20))
	if n := received.Count(0, resetOf(rf)); n != 1 {
		t.Fatalf("a report the same day: %d resets, want 1", n)
	}
	clock.add(2 * time.Hour)
	reportAndSettle(t, fc, ts, s, rf, fakecube.State{ValvePct: valve(30)}, "valve_pct", float64(30))
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
	stop()
	clock.add(time.Hour)
	ts, s, _ = startResets(t, st, addr, clock, noHold)
	reportAndSettle(t, fc, ts, s, rf, fakecube.State{ValvePct: valve(40)}, "valve_pct", float64(40))
	if n := received.Count(0, resetOf(rf)); n != 2 {
		t.Fatalf("a report after the restart: %d resets, want 2", n)
	}

	// The link error alone: no reset.
	const linked = cube.RFAddress(0x0a0108)
	reportAndSettle(t, fc, ts, s, linked, fakecube.State{Reported: &on, LinkError: &on, ValvePct: valve(15)}, "link_error", true)
	if n := received.Count(0, resetOf(linked)); n != 0 {
		t.Fatalf("the link error alone: %d resets, want 0", n)
	}
}

// The hold hook (Block 6's) holds a flagged device back: it is asked
// with the device and the report's time, and nothing is sent.
func TestAutoResetHold(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fc.Close() })
	received := cubewatch.Lines(fc)
	clock := &stepClock{t: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	var mu sync.Mutex
	var asked []string
	ts, s, _ := startResets(t, testStore(t, ""), addr, clock, func(s *Server) {
		s.runtimes()[0].resets.hold = func(rf cube.RFAddress, at time.Time) bool {
			mu.Lock()
			defer mu.Unlock()
			asked = append(asked, rf.String()+"@"+at.Format(time.RFC3339))
			return true
		}
	})
	on := true
	reportAndSettle(t, fc, ts, s, 0x0a010d, fakecube.State{Reported: &on, Error: &on}, "error", true)
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
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	received := cubewatch.Lines(fc)
	st := testStore(t, "")
	clock := &stepClock{t: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	// A store without a row for the Cube: the runtime has no store ID.
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, Store: st, PollInterval: 50 * time.Millisecond,
		Queue: engine.Options{Gap: time.Millisecond}, clock: clock})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); <-done; fc.Close() })
	waitConnected(t, ts, "KEQ0000100")
	pumpSync(t, s) // the greeting's states, before the report
	rt := s.runtimes()[0]
	const rf = cube.RFAddress(0x0a010d)
	on := true
	valve := func(n int) *int { return &n }
	reportAndSettle(t, fc, ts, s, rf, fakecube.State{Reported: &on, Error: &on, ValvePct: valve(10)}, "valve_pct", float64(10))
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
	reportAndSettle(t, fc, ts, s, rf, fakecube.State{ValvePct: valve(20)}, "valve_pct", float64(20))
	if n := received.Count(0, resetOf(rf)); n != 1 {
		t.Fatalf("with the row known: %d resets, want 1", n)
	}
}
