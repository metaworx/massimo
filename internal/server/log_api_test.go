// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
)

// logOf reads a page of the fake Cube's log.
func logOf(t *testing.T, ts *httptest.Server, query string) (int, []event) {
	t.Helper()
	var evs []event
	code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log"+query, &evs)
	return code, evs
}

// rawLog reads a page of the log as JSON objects, to see what a stored
// event holds.
func rawLog(t *testing.T, ts *httptest.Server, query string) []map[string]any {
	t.Helper()
	var evs []map[string]any
	if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log"+query, &evs); code != 200 {
		t.Fatalf("log%s: %d", query, code)
	}
	return evs
}

// The log of a Cube with a row is the store's: ids in order, pages by
// id and by time, filters by kind and device, the stored copy without
// the received times, a change of a device as a state event.
func TestLogFromStore(t *testing.T) {
	fc, ts, _, _ := startStored(t, Options{PollInterval: 50 * time.Millisecond})
	var all []event
	eventually(t, "the connect burst in the log", func() bool {
		_, all = logOf(t, ts, "?limit=1000")
		connected, states := false, 0
		for _, e := range all {
			connected = connected || e.Kind == "connected"
			if e.Kind == "state" {
				states++
			}
		}
		return connected && states >= 6
	})
	for i, e := range all {
		if e.ID == 0 || i > 0 && e.ID <= all[i-1].ID {
			t.Fatalf("event %d: id %d after %d", i, e.ID, all[max(i-1, 0)].ID)
		}
	}
	if code, page := logOf(t, ts, "?limit=5"); code != 200 || len(page) != 5 || page[4].ID != all[len(all)-1].ID || page[0].ID != all[len(all)-5].ID {
		t.Fatalf("the last five, oldest first: %d %v", code, ids(page))
	}

	// Paged back by id, every event once.
	seen := map[int64]bool{}
	for before := all[len(all)-1].ID + 1; ; {
		_, page := logOf(t, ts, "?limit=3&before="+strconv.FormatInt(before, 10))
		if len(page) == 0 {
			break
		}
		for _, e := range page {
			if seen[e.ID] || e.ID >= before {
				t.Fatalf("paging: %d before %d, seen %v", e.ID, before, seen[e.ID])
			}
			seen[e.ID] = true
		}
		before = page[0].ID
	}
	if len(seen) != len(all) {
		t.Fatalf("paged %d of %d events", len(seen), len(all))
	}

	// By kind, by device, by time.
	_, states := logOf(t, ts, "?kind=state")
	for _, e := range states {
		if e.Kind != "state" {
			t.Fatalf("kind=state answered %s", e.Kind)
		}
	}
	if len(states) < 6 {
		t.Fatalf("%d state events", len(states))
	}
	rf := states[0].Device
	_, mine := logOf(t, ts, "?device="+rf)
	for _, e := range mine {
		if e.Device != rf {
			t.Fatalf("device=%s answered %s", rf, e.Device)
		}
	}
	if len(mine) == 0 {
		t.Fatal("no event of the device")
	}
	if _, both := logOf(t, ts, "?kind=state&device="+rf); len(both) == 0 || len(both) > len(mine) {
		t.Fatalf("kind and device: %d of %d", len(both), len(mine))
	}
	// The burst's events share a millisecond, which is why the pages go
	// by id; a time bound takes the events before that millisecond.
	first, last := all[0], all[len(all)-1]
	if code, page := logOf(t, ts, "?limit=1000&before="+url.QueryEscape(first.Time.Format(time.RFC3339Nano))); code != 200 || len(page) != 0 {
		t.Fatalf("before the first event's time: %d %d", code, len(page))
	}
	if code, page := logOf(t, ts, "?limit=1000&before="+url.QueryEscape(last.Time.Add(time.Millisecond).Format(time.RFC3339Nano))); code != 200 || len(page) != len(all) {
		t.Fatalf("before a millisecond after the last event: %d %d of %d", code, len(page), len(all))
	}
	// since takes the events at or after a time, with before too.
	if code, page := logOf(t, ts, "?limit=1000&since="+url.QueryEscape(first.Time.Format(time.RFC3339Nano))); code != 200 || len(page) != len(all) {
		t.Fatalf("since the first event's time: %d %d of %d", code, len(page), len(all))
	}
	if code, page := logOf(t, ts, "?limit=1000&since="+url.QueryEscape(last.Time.Add(time.Millisecond).Format(time.RFC3339Nano))); code != 200 || len(page) != 0 {
		t.Fatalf("since a millisecond after the last event: %d %d", code, len(page))
	}
	if code, page := logOf(t, ts, "?limit=1000&since="+url.QueryEscape(last.Time.Format(time.RFC3339Nano))+
		"&before="+url.QueryEscape(last.Time.Add(time.Millisecond).Format(time.RFC3339Nano))); code != 200 || len(page) == 0 || page[len(page)-1].ID != last.ID {
		t.Fatalf("since and before around the last event: %d %v", code, ids(page))
	}
	for _, q := range []string{"?before=x", "?before=0", "?limit=0", "?limit=1001", "?limit=x", "?device=zz", "?since=x", "?since=4711",
		"?since=2026-11-06T21:00:00Z&before=2026-11-06T20:00:00Z"} {
		if code, _ := logOf(t, ts, q); code != 400 {
			t.Errorf("%s: %d", q, code)
		}
	}

	// A change of a device's values is a state event with its changes.
	greeting, _ := fakecube.LoadGreeting(fixture)
	l := greeting[len(greeting)-1]
	raw, _ := base64.StdEncoding.DecodeString(l.Data)
	copy(raw, []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12, 0x01, 30, 42, 0, 0, 0})
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	raw[7] = 55
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	eventually(t, "the change in the log", func() bool {
		_, page := logOf(t, ts, "?kind=state&device=0a010d&limit=1")
		return len(page) == 1 && page[0].Changes["valve_pct"].To == 55.0
	})
	// The device view has its received times; the stored copy not.
	var dev map[string]any
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/0a010d", &dev)
	if dev["received"] == nil {
		t.Fatalf("the device view without received: %v", dev)
	}
	for _, e := range rawLog(t, ts, "?kind=state&device=0a010d&limit=1000") {
		st, _ := e["state"].(map[string]any)
		if _, has := st["received"]; has {
			t.Fatalf("a stored state event with received: %v", e)
		}
	}
}

func ids(evs []event) []int64 {
	out := []int64{}
	for _, e := range evs {
		out = append(out, e.ID)
	}
	return out
}

// The log survives a restart of the server on the same store: the
// first run's events come before the second's connected.
func TestLogSurvivesRestart(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer fc.Close()
	st := testStore(t, "")
	row, _ := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv})
	run := func() (*httptest.Server, func()) {
		cfg, _ := CubeFromRow(row)
		s := New(Options{Cubes: []CubeConfig{cfg}, Store: st, Queue: engine.Options{Gap: time.Millisecond}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { s.Run(ctx); close(done) }()
		ts := httptest.NewServer(s.Handler())
		waitConnected(t, ts, "KEQ0000100")
		return ts, func() { ts.Close(); cancel(); <-done }
	}
	ts, stop := run()
	eventually(t, "the first run's connected", func() bool {
		_, page := logOf(t, ts, "?kind=connected")
		return len(page) == 1
	})
	stop()
	ts, stop = run()
	defer stop()
	eventually(t, "the second run's connected", func() bool {
		_, page := logOf(t, ts, "?kind=connected")
		return len(page) == 2 && page[0].ID < page[1].ID && !page[0].Time.After(page[1].Time)
	})
	if _, all := logOf(t, ts, "?limit=1000"); len(all) < 4 {
		t.Fatalf("%d events after the restart", len(all))
	}
}

// What a goroutine of the server emits as Run ends, as a restore's
// restore_done does, is in the store once Run has returned: the writer
// stops after the server's goroutines, not with them.
func TestLogLastEvents(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer fc.Close()
	st := testStore(t, "")
	row, _ := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv})
	cfg, _ := CubeFromRow(row)
	s := New(Options{Cubes: []CubeConfig{cfg}, Store: st, Queue: engine.Options{Gap: time.Millisecond}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	waitConnected(t, ts, "KEQ0000100")
	tctx := s.track()
	if tctx == nil {
		t.Fatal("Run tracks nothing")
	}
	go func() {
		defer s.wg.Done()
		<-tctx.Done()
		time.Sleep(50 * time.Millisecond)
		for _, rt := range s.runtimes() {
			rt.emit(event{Kind: "restore_done", Time: time.Now()})
		}
	}()
	cancel()
	<-done
	page, err := st.Events(context.Background(), store.EventQuery{CubeID: row.ID, Kind: "restore_done"})
	if err != nil || len(page) != 1 {
		t.Fatalf("the event emitted as Run ended: %d, %v", len(page), err)
	}
}

// A Cube without a row keeps its log in the ring: no ids, the filters
// applied in memory, an id bound empty, a device and a time as on the
// store.
func TestLogWithoutStore(t *testing.T) {
	_, ts, _ := start(t, "")
	var all []event
	eventually(t, "the connect burst in the ring", func() bool {
		_, all = logOf(t, ts, "?limit=1000")
		return len(all) >= 6
	})
	for _, e := range all {
		if e.ID != 0 {
			t.Fatalf("a ring event with an id: %+v", e)
		}
	}
	_, states := logOf(t, ts, "?kind=state&limit=2")
	if len(states) != 2 || states[0].Kind != "state" || states[1].Kind != "state" {
		t.Fatalf("kind=state&limit=2: %+v", states)
	}
	if _, page := logOf(t, ts, "?before=5"); len(page) != 0 {
		t.Fatalf("an id bound on the ring: %d", len(page))
	}
	rf := states[0].Device
	_, mine := logOf(t, ts, "?limit=1000&device="+rf)
	if len(mine) == 0 {
		t.Fatalf("device=%s: nothing", rf)
	}
	for _, e := range mine {
		if e.Device != rf {
			t.Fatalf("device=%s: %+v", rf, e)
		}
	}
	last := all[len(all)-1].Time
	_, older := logOf(t, ts, "?limit=1000&before="+url.QueryEscape(last.Format(time.RFC3339Nano)))
	for _, e := range older {
		if !e.Time.Before(last) {
			t.Fatalf("before=%v on the ring: %+v", last, e)
		}
	}
	if len(older) == 0 && all[0].Time.Before(last) {
		t.Fatalf("before=%v on the ring: nothing", last)
	}
	_, newer := logOf(t, ts, "?limit=1000&since="+url.QueryEscape(last.Format(time.RFC3339Nano)))
	// The connect burst may have added events since the first read; of
	// those since its last one, count the ones it had.
	read := 0
	for _, e := range newer {
		if e.Time.Before(last) {
			t.Fatalf("since=%v on the ring: %+v", last, e)
		}
		if !e.Time.After(last) {
			read++
		}
	}
	if read == 0 || read+len(older) != len(all) {
		t.Fatalf("since=%v on the ring: %d of the first read (%d now), and %d before, of %d", last, read, len(newer), len(older), len(all))
	}
	if code, _ := logOf(t, ts, "?limit=1001"); code != http.StatusBadRequest {
		t.Fatalf("limit=1001: %d", code)
	}
	var raw json.RawMessage
	if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log?kind=nothing", &raw); code != 200 || string(raw) != "[]" {
		t.Fatalf("an empty page: %d %s", code, raw)
	}
}
