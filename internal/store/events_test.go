// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

// eventsStore opens a store with two Cubes and starts its writer, which
// the test's cleanup stops.
func eventsStore(t *testing.T, dir string) (*Store, Cube, Cube) {
	t.Helper()
	s := open(t, dir)
	ctx := context.Background()
	a, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000002", Configured: true})
	startWriter(t, s)
	return s, a, b
}

// startWriter runs the store's writer until the test ends.
func startWriter(t *testing.T, s *Store) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.RunEvents(ctx); close(done) }()
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { cancel(); <-done }) })
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		s.ev.mu.Lock()
		running := s.ev.done != nil
		s.ev.mu.Unlock()
		if running {
			return
		}
	}
	t.Fatal("the writer did not start")
}

func ev(cube int64, at time.Time, kind, device string, n int) Event {
	return Event{CubeID: cube, Time: at, Kind: kind, Device: device, Data: json.RawMessage(fmt.Sprintf(`{"kind":%q,"n":%d}`, kind, n))}
}

func numberOf(t *testing.T, e Event) int {
	t.Helper()
	var d struct{ N int }
	if err := json.Unmarshal(e.Data, &d); err != nil {
		t.Fatal(err)
	}
	return d.N
}

func flush(t *testing.T, s *Store) {
	t.Helper()
	if err := s.FlushEvents(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// A burst is one transaction; 250 events need three; a flush makes them
// readable, in order.
func TestEventsBatched(t *testing.T) {
	s, a, _ := eventsStore(t, t.TempDir())
	ctx := context.Background()
	at := time.Now()
	_, _, before := s.EventStats()
	for i := 0; i < 60; i++ {
		s.AddEvent(ev(a.ID, at, "state", "0abc01", i))
	}
	flush(t, s)
	written, dropped, flushes := s.EventStats()
	if flushes-before != 1 || written != 60 || dropped != 0 {
		t.Fatalf("a burst of 60: %d flushes, %d written, %d dropped", flushes-before, written, dropped)
	}
	page, err := s.Events(ctx, EventQuery{CubeID: a.ID, Limit: 1000})
	if err != nil || len(page) != 60 {
		t.Fatalf("%d events %v", len(page), err)
	}
	for i, e := range page {
		if numberOf(t, e) != 59-i || e.Kind != "state" || e.Device != "0abc01" || !e.Time.Equal(at.UTC().Truncate(time.Millisecond)) {
			t.Fatalf("event %d: %+v", i, e)
		}
	}
	before = flushes
	for i := 0; i < 250; i++ {
		s.AddEvent(ev(a.ID, at, "state", "", i))
	}
	flush(t, s)
	_, _, flushes = s.EventStats()
	if n := flushes - before; n < 1 || n > 3 {
		t.Fatalf("250 events in %d transactions, want at most 3", n)
	}
	// Bad events are dropped and counted.
	_, dropped, _ = s.EventStats()
	s.AddEvent(Event{CubeID: a.ID, Data: json.RawMessage(`nope`)})
	s.AddEvent(Event{Data: json.RawMessage(`{}`)})
	if _, d, _ := s.EventStats(); d != dropped+2 {
		t.Fatalf("dropped %d, want %d", d, dropped+2)
	}
	// An event without a time takes the store's clock.
	s.AddEvent(Event{CubeID: a.ID, Kind: "hello", Data: json.RawMessage(`{"kind":"hello"}`)})
	flush(t, s)
	if page, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Kind: "hello", Limit: 1}); len(page) != 1 || page[0].Time.IsZero() {
		t.Fatalf("no time: %+v", page)
	}
}

// A full queue drops and counts; the writer then takes what was queued.
// A store opened read-only drops with the error logged.
func TestEventsDropped(t *testing.T) {
	keep := eventBuffer
	eventBuffer = 8
	t.Cleanup(func() { eventBuffer = keep })
	dir := t.TempDir()
	s := open(t, dir)
	a, _ := s.AddCube(context.Background(), Cube{Serial: "KEQ0000001", Configured: true})
	for i := 0; i < 20; i++ {
		s.AddEvent(ev(a.ID, time.Now(), "state", "", i))
	}
	if _, dropped, _ := s.EventStats(); dropped != 12 {
		t.Fatalf("dropped %d, want 12", dropped)
	}
	startWriter(t, s)
	flush(t, s)
	if written, _, _ := s.EventStats(); written != 8 {
		t.Fatalf("written %d, want 8", written)
	}

	var logged bytes.Buffer
	ro, err := OpenReadOnly(dir, Options{Logger: slog.New(slog.NewTextHandler(&logged, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	ro.AddEvent(ev(a.ID, time.Now(), "state", "", 0))
	if _, dropped, _ := ro.EventStats(); dropped != 1 || !bytes.Contains(logged.Bytes(), []byte("event dropped")) {
		t.Fatalf("read-only: dropped %d, log %q", dropped, logged.String())
	}
	if err := ro.FlushEvents(context.Background()); err != nil {
		t.Fatalf("flush without a writer: %v", err)
	}
}

// Paging by id returns every event of a Cube once, newest first, also
// where several share a time; by time, the events before it; the kind
// and the device filter, together too.
func TestEventsPaged(t *testing.T) {
	s, a, b := eventsStore(t, t.TempDir())
	ctx := context.Background()
	t0 := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	devices := []string{"0abc01", "0abc02", "0abc03"}
	for i := 0; i < 500; i++ {
		cube := a.ID
		if i%2 == 1 {
			cube = b.ID
		}
		kind := "state"
		if i%4 == 0 {
			kind = "config"
		}
		s.AddEvent(ev(cube, t0.Add(time.Duration(i/5)*time.Minute), kind, devices[i%3], i))
	}
	flush(t, s)
	seen := map[int64]bool{}
	var last int64
	for before := int64(0); ; {
		page, err := s.Events(ctx, EventQuery{CubeID: a.ID, BeforeID: before, Limit: 7})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, e := range page {
			if e.CubeID != a.ID || seen[e.ID] || last != 0 && e.ID >= last {
				t.Fatalf("event %+v (last %d, seen %v)", e, last, seen[e.ID])
			}
			seen[e.ID], last = true, e.ID
		}
		before = page[len(page)-1].ID
	}
	if len(seen) != 250 {
		t.Fatalf("paged %d events of cube a, want 250", len(seen))
	}
	cut := t0.Add(20 * time.Minute) // events 0 to 99 lie before it
	page, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Before: cut, Limit: 1000})
	if len(page) != 50 {
		t.Fatalf("before a time: %d, want 50", len(page))
	}
	for _, e := range page {
		if !e.Time.Before(cut) {
			t.Fatalf("event at %v not before %v", e.Time, cut)
		}
	}
	configs, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Kind: "config", Limit: 1000})
	if len(configs) != 125 {
		t.Fatalf("config events of cube a: %d, want 125", len(configs))
	}
	both, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Kind: "config", Device: "0abc02", Limit: 1000})
	for _, e := range both {
		if e.Kind != "config" || e.Device != "0abc02" {
			t.Fatalf("filtered %+v", e)
		}
	}
	if n := len(both); n == 0 || n >= len(configs) {
		t.Fatalf("kind and device: %d of %d", n, len(configs))
	}
	if none, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Kind: "paired", Limit: 10}); len(none) != 0 {
		t.Fatalf("an absent kind: %d", len(none))
	}
}

// The prune removes the events older than the retention, per Cube and
// in chunks; 0 keeps everything; a second prune finds nothing more.
func TestEventsPruned(t *testing.T) {
	keep := pruneChunk
	pruneChunk = 7
	t.Cleanup(func() { pruneChunk = keep })
	s, a, b := eventsStore(t, t.TempDir())
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Hour)
	t0 := now.Add(-40 * 24 * time.Hour)
	for d := 0; d < 40; d++ {
		for h := 0; h < 4; h++ {
			at := t0.Add(time.Duration(d)*24*time.Hour + time.Duration(h)*6*time.Hour)
			s.AddEvent(ev(a.ID, at, "state", "", d))
			s.AddEvent(ev(b.ID, at, "state", "", d))
		}
	}
	flush(t, s)
	if err := s.SetSetting(ctx, LogKeepDaysKey, 0); err != nil {
		t.Fatal(err)
	}
	if n, err := s.PruneEvents(ctx, now); err != nil || n != 0 {
		t.Fatalf("keep all: %d %v", n, err)
	}
	if err := s.DeleteSetting(ctx, LogKeepDaysKey); err != nil {
		t.Fatal(err)
	}
	if d, _ := s.LogKeepDays(ctx); d != DefaultLogKeepDays {
		t.Fatalf("the default: %d", d)
	}
	n, err := s.PruneEvents(ctx, now)
	if err != nil || n != 80 {
		t.Fatalf("pruned %d %v, want the 40 oldest of each Cube", n, err)
	}
	for _, cube := range []int64{a.ID, b.ID} {
		page, _ := s.Events(ctx, EventQuery{CubeID: cube, Limit: 1000})
		if len(page) != 120 || page[len(page)-1].Time.Before(now.Add(-30*24*time.Hour)) {
			t.Fatalf("cube %d kept %d, the oldest %v", cube, len(page), page[len(page)-1].Time)
		}
	}
	if n, _ := s.PruneEvents(ctx, now); n != 0 {
		t.Fatalf("a second prune removed %d", n)
	}
	s.PruneEventsSoon()
	s.PruneEventsSoon() // one pending is enough
	flush(t, s)
}

// TestEventSize measures what a state event of the measured shape costs
// in the file, the indexes included: 2000 of them written and the WAL
// checkpointed. docs/store.md states the figure.
func TestEventSize(t *testing.T) {
	dir := t.TempDir()
	s, a, _ := eventsStore(t, dir)
	ctx := context.Background()
	size := func() int64 {
		if _, err := s.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(Path(dir))
		if err != nil {
			t.Fatal(err)
		}
		return fi.Size()
	}
	start := size()
	at := time.Now().UTC()
	const n = 2000
	sample := 0
	for i := 0; i < n; i++ {
		rf := fmt.Sprintf("0abc%02x", i%30)
		data := fmt.Sprintf(`{"kind":"state","device":%q,"state":{"rf_address":%q,"type":"radiator_thermostat","name":"Radiator %d","room_id":%d,"serial":"KEQ00000%02d","reported":true,`+
			`"state":{"mode":"auto","setpoint":21.5,"actual_temp":20.3,"valve_pct":%d,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-06T20:00:00.000Z"},`+
			`"changes":{"valve_pct":{"from":20,"to":%d}},"time":"2026-11-06T20:00:%02d.123456789+01:00"}`, rf, rf, i%30, 1+i%8, i%30, i%100, i%100, i%60)
		sample = len(data)
		s.AddEvent(Event{CubeID: a.ID, Time: at.Add(time.Duration(i) * time.Second), Kind: "state", Device: rf, Data: json.RawMessage(data)})
	}
	flush(t, s)
	grown := size() - start
	per := grown / n
	t.Logf("a state event of %d bytes of JSON takes %d bytes in the file, indexes included (%d events, %d bytes)", sample, per, n, grown)
	if per > 1024 {
		t.Fatalf("%d bytes per event, more than 1 KB", per)
	}
}
