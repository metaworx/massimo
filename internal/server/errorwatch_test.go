// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/view"
)

// stateChange matches rf's state lines whose change of key is to, or
// all of them for key "".
func stateChange(rf cube.RFAddress, key string, to any) func(event) bool {
	return func(e event) bool {
		c, ok := e.Changes[key]
		return logLine("state", rf.String())(e) && (key == "" || ok && c.To == to)
	}
}

// settleState changes rf's entry to st, waits for the pump's next state
// line of rf whose change of key is to (any next one for key ""), then
// for the queue to send what the line queued. The log is marked once the
// pump has handled what came before (pumpSync), so that a line of an
// earlier report is not taken for the next.
func settleState(t *testing.T, fc *fakecube.Server, ts *httptest.Server, s *Server, rf cube.RFAddress, st fakecube.State, key string, to any) {
	t.Helper()
	pumpSync(t, s)
	m := logMark(t, ts)
	if err := fc.SetState(rf, st); err != nil {
		t.Fatal(err)
	}
	logAfter(t, m, "the state line of "+rf.String(), stateChange(rf, key, to))
	rt := s.runtimes()[0]
	if res := rt.queue.Enqueue(engine.Command{Priority: engine.PriorityLow, Lines: []cube.Message{cube.CmdGetNTP}}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
}

// watchLines are rf's error_watch lines in the whole log, oldest first.
func watchLines(t *testing.T, ts *httptest.Server, rf cube.RFAddress) []event {
	t.Helper()
	return logLines(t, logStart(ts), logLine("error_watch", rf.String()))
}

// waitWatch waits until rf has n error_watch lines in the log, and no
// more, and returns them.
func waitWatch(t *testing.T, ts *httptest.Server, rf cube.RFAddress, n int) []event {
	t.Helper()
	watch := logLine("error_watch", rf.String())
	lines, next := logAfterN(t, logStart(ts), "the error_watch lines of "+rf.String(), n, watch)
	if more := logLines(t, next, watch); len(more) != 0 {
		t.Fatalf("%s: %d error_watch lines, want %d: %+v", rf, n+len(more), n, append(lines, more...))
	}
	return lines
}

// watchAlerts are the log's error_watch alerts of rf.
func watchAlerts(t *testing.T, ts *httptest.Server, rf cube.RFAddress) []logAlert {
	t.Helper()
	var out logAlertsView
	if code := getJSON(t, ts, "", "/api/v1/log/alerts", &out); code != 200 {
		t.Fatalf("GET /api/v1/log/alerts: %d", code)
	}
	var mine []logAlert
	for _, a := range out.Alerts {
		if a.Kind == "error_watch" && a.Device == rf.String() {
			mine = append(mine, a)
		}
	}
	return mine
}

// wantWatch checks an error_watch line's level and summary.
func wantWatch(t *testing.T, ev event, level string, want map[string]any) {
	t.Helper()
	if ev.Kind != "error_watch" || ev.Level != level || ev.Source != "device" {
		t.Fatalf("line %+v: want error_watch at %s", ev, level)
	}
	for k, v := range want {
		if got := ev.Summary[k]; got != v {
			t.Fatalf("line %+v: summary %s = %v, want %v", ev, k, got, v)
		}
	}
}

// A communication error back after the automatic reset (AP RadioHealth
// v1.2, Block 6, D102-B): a line the pump takes late, read before the
// Cube answered the reset, is its aftermath; the flag cleared, then set
// again at the device's next report is a warning with the radiogram
// that failed, an alert until the flag clears, and the hook holds the
// device; back again the same day is no second warning; clear for a day,
// the resets take the device again.
func TestErrorWatchBack(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC))
	r := startServer(t, resetsOn(clock))
	fc, ts, s, received := r.fc, r.ts, r.s, r.lines
	rt := s.runtimes()[0]
	const rf = cube.RFAddress(0x0a010d)
	on, off := true, false
	valve := func(n int) *int { return &n }

	reset := logMark(t, ts)
	settleState(t, fc, ts, s, rf, fakecube.State{Reported: &on, Error: &on, ValvePct: valve(10)}, "valve_pct", float64(10))
	logAfter(t, reset, "the error_reset", logLine("error_reset", "0a010d"))
	if n := received.Count(0, resetOf(rf)); n != 1 {
		t.Fatalf("the first report: %d resets, want 1", n)
	}
	// The pump takes a report read before the answer late: the entry has
	// not changed since, and the flag it shows is the reset's aftermath.
	s.watchErrors(context.Background(), rt, rf, map[string]view.Change{"valve_pct": {From: float64(5), To: float64(10)}})
	if lines := watchLines(t, ts, rf); len(lines) != 0 || rt.resets.hold(rf, clock.Now()) {
		t.Fatalf("the aftermath: lines %+v, held %v", lines, rt.resets.hold(rf, clock.Now()))
	}
	// The Cube clears the flag after the r: (the fake does not by itself).
	settleState(t, fc, ts, s, rf, fakecube.State{Error: &off}, "error", false)
	if lines := watchLines(t, ts, rf); len(lines) != 0 {
		t.Fatalf("the flag cleared by the reset: %+v", lines)
	}

	clock.Add(time.Hour)
	settleState(t, fc, ts, s, rf, fakecube.State{Error: &on, ValvePct: valve(20)}, "valve_pct", float64(20))
	lines := waitWatch(t, ts, rf, 1)
	wantWatch(t, lines[0], "warn", map[string]any{"what": "back", "flag": "error", "minutes": float64(60), "next_report": true, "held": true})
	if r, _ := lines[0].Summary["failed_radiogram"].(map[string]any); r == nil || r["code"] != float64(9) {
		t.Fatalf("the failed radiogram: %v", lines[0].Summary["failed_radiogram"])
	}
	if !rt.resets.hold(rf, clock.Now()) {
		t.Fatal("the device back is not held")
	}
	if n := received.Count(0, resetOf(rf)); n != 1 {
		t.Fatalf("back: %d resets, want 1", n)
	}
	if a := watchAlerts(t, ts, rf); len(a) != 1 || a[0].Level != "warn" || a[0].ID != lines[0].ID {
		t.Fatalf("the alerts: %+v", a)
	}

	// Clear: an info line settles the warning; the device stays held.
	settleState(t, fc, ts, s, rf, fakecube.State{Error: &off}, "error", false)
	lines = waitWatch(t, ts, rf, 2)
	wantWatch(t, lines[1], "info", map[string]any{"what": "clear", "flag": "error", "held": true, "hours": float64(0)})
	if a := watchAlerts(t, ts, rf); len(a) != 0 {
		t.Fatalf("the alerts after the clear: %+v", a)
	}
	if !rt.resets.hold(rf, clock.Now()) {
		t.Fatal("held: let go before a day clear")
	}

	// Back again the same day: no second warning, no reset.
	clock.Add(2 * time.Hour)
	settleState(t, fc, ts, s, rf, fakecube.State{Error: &on, ValvePct: valve(30)}, "valve_pct", float64(30))
	if lines := watchLines(t, ts, rf); len(lines) != 2 {
		t.Fatalf("back again the same day: %+v", lines)
	}
	if n := received.Count(0, resetOf(rf)); n != 1 {
		t.Fatalf("back again: %d resets, want 1", n)
	}

	// Clear for a day: the next report while flagged is reset again,
	// and is no return (the reset is more than a day old).
	settleState(t, fc, ts, s, rf, fakecube.State{Error: &off}, "error", false)
	waitWatch(t, ts, rf, 3)
	clock.Add(25 * time.Hour)
	settleState(t, fc, ts, s, rf, fakecube.State{Error: &on, ValvePct: valve(40)}, "valve_pct", float64(40))
	if n := received.Count(0, resetOf(rf)); n != 2 {
		t.Fatalf("clear for a day: %d resets, want 2", n)
	}
	if lines := watchLines(t, ts, rf); len(lines) != 3 {
		t.Fatalf("after a day clear: %+v", lines)
	}
	if rt.resets.hold(rf, clock.Now()) {
		t.Fatal("held after a day clear")
	}
}

// A flag set for 24 hours (AP RadioHealth v1.2, Block 6): the
// communication error of a device the Cube holds no state for, gone
// silent, and the link error of one that reports, each an error by its
// timer and an alert until its clear, an info line; a flag cleared
// before stops its timer.
func TestErrorWatchLasting(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC))
	r := startServer(t, resetsOn(clock))
	fc, ts, s := r.fc, r.ts, r.s
	const silent, linked, cleared = cube.RFAddress(0x0a010d), cube.RFAddress(0x0a0108), cube.RFAddress(0x0a0109)
	on, off := true, false
	t0 := clock.Now()
	settleState(t, fc, ts, s, silent, fakecube.State{Error: &on}, "", nil)
	settleState(t, fc, ts, s, linked, fakecube.State{Reported: &on, LinkError: &on}, "link_error", true)
	settleState(t, fc, ts, s, cleared, fakecube.State{Error: &on}, "", nil)
	clock.Add(10 * time.Hour)
	settleState(t, fc, ts, s, cleared, fakecube.State{Error: &off}, "", nil)

	clock.Add(14*time.Hour - time.Second)
	for _, rf := range []cube.RFAddress{silent, linked, cleared} {
		if lines := watchLines(t, ts, rf); len(lines) != 0 {
			t.Fatalf("%s before 24 hours: %+v", rf, lines)
		}
	}
	clock.Add(time.Second)
	lines := waitWatch(t, ts, silent, 1)
	wantWatch(t, lines[0], "error", map[string]any{"what": "lasting", "flag": "error", "hours": float64(24), "silent": true, "held": false,
		"since": t0.Format(time.RFC3339)})
	link := waitWatch(t, ts, linked, 1)
	wantWatch(t, link[0], "error", map[string]any{"what": "lasting", "flag": "link_error", "since": t0.Format(time.RFC3339)})
	if _, ok := link[0].Summary["silent"]; ok {
		t.Fatalf("a device that reports, silent: %+v", link[0])
	}
	if lines := watchLines(t, ts, cleared); len(lines) != 0 {
		t.Fatalf("a flag cleared before: %+v", lines)
	}
	for _, rf := range []cube.RFAddress{silent, linked} {
		if a := watchAlerts(t, ts, rf); len(a) != 1 || a[0].Level != "error" {
			t.Fatalf("%s: the alerts %+v", rf, a)
		}
	}
	if n := clock.Pending(); n != 0 {
		t.Fatalf("%d timers left", n)
	}

	clock.Add(time.Hour)
	settleState(t, fc, ts, s, linked, fakecube.State{LinkError: &off}, "link_error", false)
	link = waitWatch(t, ts, linked, 2)
	wantWatch(t, link[1], "info", map[string]any{"what": "clear", "flag": "link_error", "hours": float64(25)})
	if _, ok := link[1].Summary["held"]; ok {
		t.Fatalf("the link error's clear says held: %+v", link[1])
	}
	settleState(t, fc, ts, s, silent, fakecube.State{Error: &off}, "", nil)
	lines = waitWatch(t, ts, silent, 2)
	wantWatch(t, lines[1], "info", map[string]any{"what": "clear", "flag": "error", "held": false})
	for _, rf := range []cube.RFAddress{silent, linked} {
		if a := watchAlerts(t, ts, rf); len(a) != 0 {
			t.Fatalf("%s: the alerts after the clear %+v", rf, a)
		}
	}
}

// A restart does not set a flag's day back (AP RadioHealth v1.2,
// Block 6): a server started anew over the same store reads the flags'
// starts from the state lines and gives each error at its day; a device
// without a state has no flag in its lines, and its day starts with the
// server; one started after the errors gives none again.
func TestErrorWatchLastingAcrossRestart(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC))
	r := startServer(t, resetsOn(clock))
	fc, ts, s, st := r.fc, r.ts, r.s, r.st
	const flagged, linked, silent = cube.RFAddress(0x0a010a), cube.RFAddress(0x0a010b), cube.RFAddress(0x0a010c)
	on := true
	valve := func(n int) *int { return &n }
	t0 := clock.Now()
	reset := logMark(t, ts)
	settleState(t, fc, ts, s, flagged, fakecube.State{Reported: &on, Error: &on, ValvePct: valve(10)}, "error", true)
	settleState(t, fc, ts, s, linked, fakecube.State{Reported: &on, LinkError: &on}, "link_error", true)
	settleState(t, fc, ts, s, silent, fakecube.State{Error: &on}, "", nil)
	logAfter(t, reset, "the error_reset", logLine("error_reset", "0a010a"))
	if err := st.FlushEvents(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.stop()

	clock.Add(10 * time.Hour)
	r = startServer(t, resetsOn(clock), again(r))
	ts, s = r.ts, r.s
	w := &s.runtimes()[0].errWatch
	w.mu.Lock()
	d := w.devices[flagged]
	since, linkSince, silentSince := d.err.since, w.devices[linked].link.since, w.devices[silent].err.since
	w.mu.Unlock()
	if !since.Equal(t0) || !linkSince.Equal(t0) || !silentSince.Equal(clock.Now()) {
		t.Fatalf("seeded since %v, %v and %v", since, linkSince, silentSince)
	}
	clock.Add(14*time.Hour - time.Second)
	if lines := watchLines(t, ts, flagged); len(lines) != 0 {
		t.Fatalf("before 24 hours: %+v", lines)
	}
	clock.Add(time.Second)
	lines := waitWatch(t, ts, flagged, 1)
	wantWatch(t, lines[0], "error", map[string]any{"what": "lasting", "flag": "error", "since": t0.Format(time.RFC3339)})
	link := waitWatch(t, ts, linked, 1)
	wantWatch(t, link[0], "error", map[string]any{"what": "lasting", "flag": "link_error"})
	if lines := watchLines(t, ts, silent); len(lines) != 0 {
		t.Fatalf("the device without a state, a day after the first start: %+v", lines)
	}
	clock.Add(10 * time.Hour)
	wantWatch(t, waitWatch(t, ts, silent, 1)[0], "error", map[string]any{"what": "lasting", "flag": "error", "silent": true})
	if err := st.FlushEvents(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.stop()

	clock.Add(time.Hour)
	r = startServer(t, resetsOn(clock), again(r))
	ts, s = r.ts, r.s
	w = &s.runtimes()[0].errWatch
	w.mu.Lock()
	lasting, open, linkLasting := w.devices[flagged].err.lasting, w.devices[flagged].err.open, w.devices[linked].link.lasting
	silentLasting := w.devices[silent].err.lasting
	w.mu.Unlock()
	if !lasting || !open || !linkLasting || !silentLasting {
		t.Fatalf("seeded lasting %v, open %v, link lasting %v, silent lasting %v", lasting, open, linkLasting, silentLasting)
	}
	clock.Add(48 * time.Hour)
	if lines := watchLines(t, ts, flagged); len(lines) != 1 {
		t.Fatalf("after the second restart: %+v", lines)
	}
	if lines := watchLines(t, ts, linked); len(lines) != 1 {
		t.Fatalf("the link error after the second restart: %+v", lines)
	}
	if lines := watchLines(t, ts, silent); len(lines) != 1 {
		t.Fatalf("the device without a state after the second restart: %+v", lines)
	}
	if n := clock.Pending(); n != 0 {
		t.Fatalf("%d timers left", n)
	}
}

// A flag's day that comes up while the Cube is away is no error yet: its
// flags are not known then, and the timer looks again in a minute.
func TestErrorWatchWaitsForTheCube(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC))
	r := startServer(t, resetsOn(clock))
	fc, ts, s := r.fc, r.ts, r.s
	const rf = cube.RFAddress(0x0a010d)
	on := true
	settleState(t, fc, ts, s, rf, fakecube.State{Error: &on}, "", nil)
	fc.Close()
	rt := s.runtimes()[0]
	testkit.Eventually(t, "the Cube away", func() bool { return !rt.client.Connected() }, awayWait)
	clock.Add(25 * time.Hour)
	if lines := watchLines(t, ts, rf); len(lines) != 0 {
		t.Fatalf("with the Cube away: %+v", lines)
	}
	if n := clock.Pending(); n != 1 {
		t.Fatalf("%d timers, want the one looking again", n)
	}
}
