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
	code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/log"+query, &evs)
	return code, evs
}

// rawLog reads a page of the log as JSON objects, to see what a stored
// event holds.
func rawLog(t *testing.T, ts *httptest.Server, query string) []map[string]any {
	t.Helper()
	var evs []map[string]any
	if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/log"+query, &evs); code != 200 {
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
	for _, q := range []string{"?before=x", "?before=0", "?limit=0", "?limit=1001", "?limit=x", "?device=zz"} {
		if code, _ := logOf(t, ts, q); code != 400 {
			t.Errorf("%s: %d", q, code)
		}
	}

	// A change of a device's values is a state event with its changes.
	greeting, _ := fakecube.LoadGreeting(fixture)
	l := greeting[len(greeting)-1]
	raw, _ := base64.StdEncoding.DecodeString(l.Data)
	copy(raw, []byte{0x0b, 0x15, 0x14, 0xb6, 0x09, 0x12, 0x01, 30, 42, 0, 0, 0})
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	raw[7] = 55
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	eventually(t, "the change in the log", func() bool {
		_, page := logOf(t, ts, "?kind=state&device=1514b6&limit=1")
		return len(page) == 1 && page[0].Changes["valve_pct"].To == 55.0
	})
	// The device view has its received times; the stored copy not.
	var dev map[string]any
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/devices/1514b6", &dev)
	if dev["received"] == nil {
		t.Fatalf("the device view without received: %v", dev)
	}
	for _, e := range rawLog(t, ts, "?kind=state&device=1514b6&limit=1000") {
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
	row, _ := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0537249", Configured: true, Address: addr, Source: store.SourceEnv})
	run := func() (*httptest.Server, func()) {
		cfg, _ := CubeFromRow(row)
		s := New(Options{Cubes: []CubeConfig{cfg}, Store: st, Queue: engine.Options{Gap: time.Millisecond}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { s.Run(ctx); close(done) }()
		ts := httptest.NewServer(s.Handler())
		waitConnected(t, ts, "KEQ0537249")
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

// A Cube without a row keeps its log in the ring: no ids, the filters
// applied in memory, an id bound empty.
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
	if code, _ := logOf(t, ts, "?limit=1001"); code != http.StatusBadRequest {
		t.Fatalf("limit=1001: %d", code)
	}
	var raw json.RawMessage
	if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/log?kind=nothing", &raw); code != 200 || string(raw) != "[]" {
		t.Fatalf("an empty page: %d %s", code, raw)
	}
}
