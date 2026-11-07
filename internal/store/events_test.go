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
	"strings"
	"sync"
	"sync/atomic"
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
	since, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Since: cut, Limit: 1000})
	if len(since) != 200 {
		t.Fatalf("since a time: %d, want 200", len(since))
	}
	for _, e := range since {
		if e.Time.Before(cut) {
			t.Fatalf("event at %v before %v", e.Time, cut)
		}
	}
	upTo := t0.Add(40 * time.Minute) // events 100 to 199 lie from cut up to it
	between, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Since: cut, Before: upTo, Limit: 1000})
	if len(between) != 50 {
		t.Fatalf("since and before: %d, want 50", len(between))
	}
	if after, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Since: t0.Add(2 * time.Hour), Limit: 1000}); len(after) != 0 {
		t.Fatalf("since a time after every event: %d", len(after))
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

	// A request prunes now: the two events older than the retention go.
	s.AddEvent(ev(a.ID, t0, "state", "", 100))
	s.AddEvent(ev(b.ID, t0, "state", "", 100))
	flush(t, s)
	s.PruneEventsSoon()
	s.PruneEventsSoon()
	cutoff := now.Add(-time.Duration(DefaultLogKeepDays) * 24 * time.Hour)
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		pa, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Before: cutoff})
		pb, _ := s.Events(ctx, EventQuery{CubeID: b.ID, Before: cutoff})
		if len(pa)+len(pb) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("PruneEventsSoon left %d and %d", len(pa), len(pb))
		}
	}
	// Without a writer, a request while one is pending adds nothing.
	idle := open(t, t.TempDir())
	idle.PruneEventsSoon()
	idle.PruneEventsSoon()
	if n := len(idle.ev.prune); n != 1 {
		t.Fatalf("%d requests pending", n)
	}
}

// stateData is a state event of the shape the log keeps, about 420
// bytes of JSON.
const stateData = `{"kind":"state","device":"0abc01","time":"2026-11-06T10:00:00.000+01:00","state":{"rf_address":"0abc01","name":"Radiator 1","type":"radiator thermostat","room_id":1,"room":"Room 1","mode":"auto","setpoint":20.5,"temperature":21.3,"valve_pct":30,"battery_low":false,"window_open":false,"link_error":false,"panel_locked":false,"stale":false},"changes":{"valve_pct":{"from":20,"to":30}}}`

// fill writes n events of a Cube straight into the table, step apart
// from start: one in 500 connected, one in 97 config, the others state,
// over thirty devices.
func fill(t *testing.T, s *Store, cube int64, n int, start time.Time, step time.Duration) {
	t.Helper()
	_, err := s.db.Exec(`WITH RECURSIVE n(i) AS (SELECT 0 UNION ALL SELECT i + 1 FROM n WHERE i < ? - 1)
		INSERT INTO events (uuid, cube_id, time, kind, device, data)
		SELECT printf('fill-%d-%d-%d', ?, ?, i), ?, strftime('%Y-%m-%dT%H:%M:%fZ', ? + i * ?, 'unixepoch'),
			CASE WHEN i % 500 = 0 THEN 'connected' WHEN i % 97 = 0 THEN 'config' ELSE 'state' END,
			printf('0abc%02d', i % 30), ? FROM n`,
		n, cube, start.UnixMilli(), cube, float64(start.UnixMilli())/1000, step.Seconds(), stateData)
	if err != nil {
		t.Fatal(err)
	}
}

// A page reads one index of migration 0004 in id order, whatever its
// terms, instead of sorting every one of the Cube's rows (the review of
// 2026-11-06: 1.3 s a page at 90,000 events); each page holds what its
// terms ask, newest first, and a page by time is the events before it.
// 90,000 events of one Cube beside 10,000 of another, 3000 under -short;
// the time of each page is logged.
func TestEventsPageQuery(t *testing.T) {
	s, a, b := eventsStore(t, t.TempDir())
	ctx := context.Background()
	n := 90000
	if testing.Short() {
		n = 3000
	}
	start := time.Now().UTC().Add(-30 * 24 * time.Hour).Truncate(time.Second)
	step := 30 * 24 * time.Hour / time.Duration(n)
	fill(t, s, a.ID, n, start, step)
	fill(t, s, b.ID, n/9, start, 9*step)
	mid := start.Add(15 * 24 * time.Hour)
	shapes := []struct {
		name string
		q    EventQuery
	}{
		{"none", EventQuery{CubeID: a.ID}},
		{"id", EventQuery{CubeID: a.ID, BeforeID: int64(n / 2)}},
		{"time", EventQuery{CubeID: a.ID, Before: mid}},
		{"kind", EventQuery{CubeID: a.ID, Kind: "connected"}},
		{"kind and id", EventQuery{CubeID: a.ID, Kind: "connected", BeforeID: int64(n / 2)}},
		{"device", EventQuery{CubeID: a.ID, Device: "0abc07"}},
		{"kind and device", EventQuery{CubeID: a.ID, Kind: "state", Device: "0abc07"}},
		{"time and device", EventQuery{CubeID: a.ID, Device: "0abc07", Before: mid}},
		{"since", EventQuery{CubeID: a.ID, Since: mid}},
		{"since and kind", EventQuery{CubeID: a.ID, Kind: "connected", Since: mid}},
		{"since and before", EventQuery{CubeID: a.ID, Since: start.Add(24 * time.Hour), Before: mid}},
		{"the other Cube", EventQuery{CubeID: b.ID}},
	}
	for _, sh := range shapes {
		query, args := eventsQuery(sh.q, 200)
		rows, err := s.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, args...)
		if err != nil {
			t.Fatal(err)
		}
		var plan []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, detail)
		}
		rows.Close()
		joined := strings.Join(plan, "; ")
		if strings.Contains(joined, "TEMP B-TREE") || strings.Contains(joined, "SCAN events") {
			t.Errorf("%s: %s", sh.name, joined)
		}
		q := sh.q
		q.Limit = 200
		began := time.Now()
		page, err := s.Events(ctx, q)
		took := time.Since(began)
		if err != nil || len(page) == 0 {
			t.Fatalf("%s: %d events, %v", sh.name, len(page), err)
		}
		for i, e := range page {
			if e.CubeID != q.CubeID || (q.Kind != "" && e.Kind != q.Kind) || (q.Device != "" && e.Device != q.Device) ||
				(q.BeforeID > 0 && e.ID >= q.BeforeID) || (!q.Before.IsZero() && !e.Time.Before(q.Before)) ||
				(!q.Since.IsZero() && e.Time.Before(q.Since)) ||
				(i > 0 && e.ID >= page[i-1].ID) {
				t.Fatalf("%s: event %d: %+v", sh.name, i, e)
			}
		}
		t.Logf("%-16s %3d events in %8v: %s", sh.name, len(page), took, joined)
	}
	page, err := s.Events(ctx, EventQuery{CubeID: a.ID, Before: mid, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM events WHERE cube_id = ? AND time < ? ORDER BY id DESC LIMIT 5`, a.ID, formatTime(mid))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for i := 0; rows.Next(); i++ {
		var id int64
		rows.Scan(&id)
		if i >= len(page) || page[i].ID != id {
			t.Fatalf("the page before %v: %+v, event %d is %d", mid, page, i, id)
		}
	}
}

// A prune hands over between its chunks: a flush during it is answered,
// the event written, while the prune goes on to the end.
func TestEventsPruneYields(t *testing.T) {
	keep := pruneChunk
	pruneChunk = 10
	// Registered before the writer starts, so run after it stopped.
	t.Cleanup(func() { pruneChunk, pruneStep = keep, nil })
	s := open(t, t.TempDir())
	ctx := context.Background()
	a, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	fill(t, s, a.ID, 500, time.Now().Add(-60*24*time.Hour), time.Second)
	var steps, after atomic.Int32
	var flushed atomic.Bool
	inPrune := make(chan struct{})
	pruneStep = func() {
		if steps.Add(1) == 1 {
			close(inPrune)
		}
		if flushed.Load() {
			after.Add(1)
		}
		time.Sleep(2 * time.Millisecond)
	}
	startWriter(t, s)
	select {
	case <-inPrune:
	case <-time.After(5 * time.Second):
		t.Fatal("no prune at the writer's start")
	}
	s.AddEvent(ev(a.ID, time.Now(), "hello", "", 1))
	flush(t, s)
	flushed.Store(true)
	if page, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Kind: "hello"}); len(page) != 1 {
		t.Fatalf("the flush during the prune: %d events", len(page))
	}
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if page, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Kind: "state", Limit: 1}); len(page) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the prune did not end")
		}
	}
	if after.Load() == 0 {
		t.Fatalf("the flush waited for the whole prune (%d chunks)", steps.Load())
	}
}

// A done ctx ends a prune between two chunks; the writer's last write
// still follows.
func TestEventsPruneStops(t *testing.T) {
	keep := pruneChunk
	pruneChunk = 10
	t.Cleanup(func() { pruneChunk, pruneStep = keep, nil })
	s := open(t, t.TempDir())
	a, err := s.AddCube(context.Background(), Cube{Serial: "KEQ0000001", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	fill(t, s, a.ID, 500, time.Now().Add(-60*24*time.Hour), time.Second)
	s.AddEvent(ev(a.ID, time.Now(), "hello", "", 1)) // queued before the writer runs
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var steps atomic.Int32
	pruneStep = func() {
		if steps.Add(1) == 2 {
			cancel()
		}
	}
	done := make(chan struct{})
	go func() { s.RunEvents(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the writer did not stop")
	}
	var left int
	if err := s.db.QueryRow(`SELECT count(*) FROM events WHERE kind != 'hello'`).Scan(&left); err != nil || left != 480 {
		t.Fatalf("%d left after two chunks (%d steps), %v", left, steps.Load(), err)
	}
	if page, _ := s.Events(context.Background(), EventQuery{CubeID: a.ID, Kind: "hello"}); len(page) != 1 {
		t.Fatalf("the pending event: %d", len(page))
	}
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
