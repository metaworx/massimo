// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/testkit"
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
	testkit.Eventually(t, "the writer's start", func() bool {
		s.ev.mu.Lock()
		defer s.ev.mu.Unlock()
		return s.ev.done != nil
	})
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
	t.Parallel()
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
	// Bad events are dropped and counted; one of no Cube is the server's.
	_, dropped, _ = s.EventStats()
	s.AddEvent(Event{CubeID: a.ID, Data: json.RawMessage(`nope`)})
	s.AddEvent(Event{Data: json.RawMessage(``)})
	s.AddEvent(Event{Kind: "server_started", Data: json.RawMessage(`{}`)})
	if _, d, _ := s.EventStats(); d != dropped+2 {
		t.Fatalf("dropped %d, want %d", d, dropped+2)
	}
	flush(t, s)
	if page, _ := s.Events(ctx, EventQuery{Kind: "server_started", Limit: 1}); len(page) != 1 || page[0].CubeID != 0 {
		t.Fatalf("the server's event: %+v", page)
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
	testkit.Swapping(t)
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

	log, logged := testkit.LogWatch(nil)
	ro, err := OpenReadOnly(dir, Options{Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	ro.AddEvent(ev(a.ID, time.Now(), "state", "", 0))
	if _, dropped, _ := ro.EventStats(); dropped != 1 || logged.Count(0, testkit.Msg("store: event dropped")) == 0 {
		t.Fatalf("read-only: dropped %d, log %v", dropped, logged.Since(0, nil))
	}
	if err := ro.FlushEvents(context.Background()); err != nil {
		t.Fatalf("flush without a writer: %v", err)
	}
}

// Paging by id returns every event of a Cube once, newest first, also
// where several share a time; by time, the events before it; the kind
// and the device filter, together too.
func TestEventsPaged(t *testing.T) {
	t.Parallel()
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

// Paging forward from a cursor (AP LogCursor) returns every event once,
// oldest first, each page the oldest after its cursor and the cursor
// itself not again: one Cube's, and every Cube's with the server's,
// through a burst many pages long; with a kind, inside a window before
// an id, and since a time, the same events as a page back. The newest id
// is the log's newest row, the audit's too, 0 for an empty log.
func TestEventsForward(t *testing.T) {
	t.Parallel()
	s, a, b := eventsStore(t, t.TempDir())
	ctx := context.Background()
	if id, err := s.NewestEventID(ctx); err != nil || id != 0 {
		t.Fatalf("an empty log's newest id: %d, %v", id, err)
	}
	if page, err := s.Events(ctx, EventQuery{CubeID: a.ID, Server: true, Forward: true}); err != nil || len(page) != 0 {
		t.Fatalf("an empty log forward: %v, %v", page, err)
	}
	if page, err := s.Events(ctx, EventQuery{CubeID: a.ID, Server: true, Forward: true, AfterID: 5}); !errors.Is(err, ErrBeyondNewest) {
		t.Fatalf("an empty log after 5: %v, %v, want ErrBeyondNewest", page, err)
	}
	t0 := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 600; i++ {
		kind := "state"
		if i%4 == 0 {
			kind = "config"
		}
		s.AddEvent(ev([]int64{a.ID, b.ID, 0}[i%3], t0.Add(time.Duration(i/5)*time.Minute), kind, "0abc01", i))
	}
	flush(t, s)
	// back is a page newest first, turned round: what a walk forward is
	// to read.
	back := func(q EventQuery) []int64 {
		t.Helper()
		q.Limit = MaxEventsPage
		page, err := s.Events(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		ids := eventIDs(page)
		slices.Reverse(ids)
		return ids
	}
	// forward walks pages of 7 from q's cursor until one is empty.
	forward := func(q EventQuery) []int64 {
		t.Helper()
		q.Forward, q.Limit = true, 7
		read := []int64{}
		for {
			page, err := s.Events(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
			if len(page) == 0 {
				return read
			}
			for i, e := range page {
				if e.ID <= q.AfterID || i > 0 && e.ID <= page[i-1].ID {
					t.Fatalf("the page after %d: %v", q.AfterID, eventIDs(page))
				}
			}
			read = append(read, eventIDs(page)...)
			q.AfterID = page[len(page)-1].ID
		}
	}
	every := EventQuery{CubeIDs: []int64{a.ID, b.ID}, Server: true}
	if got, want := forward(every), back(every); len(want) != 600 || !slices.Equal(got, want) {
		t.Fatalf("every Cube's forward: %d of %d events, %v", len(got), len(want), got)
	}
	one := EventQuery{CubeID: a.ID}
	want := back(one)
	if got := forward(one); len(want) != 200 || !slices.Equal(got, want) {
		t.Fatalf("cube a forward: %d of %d events, %v", len(got), len(want), got)
	}
	for _, q := range []EventQuery{
		{CubeID: a.ID, Kind: "config"},
		{CubeID: a.ID, AfterID: want[10], BeforeID: want[50]},
		{CubeID: a.ID, Server: true, Kind: "state", AfterID: want[10], BeforeID: want[150]},
		{CubeID: a.ID, Since: t0.Add(20 * time.Minute)},
		{CubeID: a.ID, Since: t0.Add(20 * time.Minute), Before: t0.Add(40 * time.Minute)},
	} {
		w := back(q)
		if got := forward(q); len(w) == 0 || !slices.Equal(got, w) {
			t.Fatalf("%+v forward: %v, want %v", q, got, w)
		}
	}
	if got := forward(EventQuery{CubeID: a.ID, AfterID: want[len(want)-1]}); len(got) != 0 {
		t.Fatalf("after cube a's newest: %v", got)
	}
	// A cursor beyond the newest row: one the log never gave out.
	if page, err := s.Events(ctx, EventQuery{CubeID: a.ID, Forward: true, AfterID: 601}); !errors.Is(err, ErrBeyondNewest) {
		t.Fatalf("after 601 of 600 rows: %d rows, %v", len(page), err)
	}
	// The newest row of the audit's counts, as the table holds it: a page
	// of an auditor's would stop below it otherwise.
	s.AddEvent(Event{CubeID: b.ID, Time: t0, Kind: "login", Visibility: VisibilityAudit, Data: json.RawMessage(`{"kind":"login"}`)})
	flush(t, s)
	var top int64
	if err := s.db.QueryRowContext(ctx, `SELECT max(id) FROM log`).Scan(&top); err != nil {
		t.Fatal(err)
	}
	all := back(EventQuery{CubeIDs: []int64{a.ID, b.ID}, Server: true, Audit: true})
	if id, err := s.NewestEventID(ctx); err != nil || id != top || all[len(all)-1] != top || id <= want[len(want)-1] {
		t.Fatalf("the newest id %d, %v; the table's newest row %d, an auditor's page's %d", id, err, top, all[len(all)-1])
	}
}

// A page is a walk of the Cube's rows and one of the server's, two
// statements, between which a write can commit: every walk stops at the
// newest row read before them, so that a page forward leaves no row out
// below its last. One Cube's rows and the server's, written one of each
// a transaction while a reader pages forward, are read each once.
func TestEventsForwardWhileWriting(t *testing.T) {
	t.Parallel()
	s, a, _ := eventsStore(t, t.TempDir())
	ctx := context.Background()
	const n = 1000
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range n {
			if err := s.writeEvents(ctx, []Event{ev(a.ID, time.Now(), "state", "0abc01", i), ev(0, time.Now(), "settings_read", "", i)}); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	read := []int64{}
	var after int64
	testkit.Eventually(t, "the writes' end", func() bool {
		over := false
		select {
		case <-done:
			over = true // and the page below comes after the last write
		default:
		}
		page, err := s.Events(ctx, EventQuery{CubeID: a.ID, Server: true, Forward: true, AfterID: after, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		for i, e := range page {
			if e.ID <= after || i > 0 && e.ID <= page[i-1].ID {
				t.Fatalf("the page after %d: %v", after, eventIDs(page))
			}
		}
		read = append(read, eventIDs(page)...)
		if len(page) > 0 {
			after = page[len(page)-1].ID
			return false
		}
		return over
	}, testkit.For(4*testkit.Wait), testkit.Every(100*time.Microsecond))
	if len(read) != 2*n || read[0] != 1 || read[len(read)-1] != 2*n {
		missed := []int64{}
		for id := int64(1); id <= 2*n && len(missed) < 10; id++ {
			if _, found := slices.BinarySearch(read, id); !found {
				missed = append(missed, id)
			}
		}
		t.Fatalf("read %d of %d rows, the first missed %v", len(read), 2*n, missed)
	}
}

// A page back, newest first, while the writer commits one Cube row and
// one server row a transaction: every row is the Cube's or the server's,
// so a page holds every id between its lowest and its highest. The one
// newest id bounding all of the page's walks ensures it; a client's
// first cursor forward is the top of such a page, and a hole below it
// would never be read.
func TestEventsBackWhileWriting(t *testing.T) {
	t.Parallel()
	s, a, _ := eventsStore(t, t.TempDir())
	ctx := context.Background()
	const n = 1000
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range n {
			if err := s.writeEvents(ctx, []Event{ev(a.ID, time.Now(), "state", "0abc01", i), ev(0, time.Now(), "settings_read", "", i)}); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	var hole []int64
	pages := 0
	for over := false; !over && hole == nil; pages++ {
		select {
		case <-done:
			over = true // and the page below comes after the last write
		default:
		}
		page, err := s.Events(ctx, EventQuery{CubeID: a.ID, Server: true, Limit: 50})
		if err != nil {
			t.Error(err)
			break
		}
		for i := 1; i < len(page); i++ {
			if page[i].ID != page[i-1].ID-1 {
				hole = eventIDs(page)
				break
			}
		}
	}
	<-done // the writer ends before the test does, which closes the store
	if hole != nil {
		t.Fatalf("page %d back has a hole: %v", pages, hole)
	}
}

// The prune removes the events older than the retention, per Cube and
// in chunks; 0 keeps everything; a second prune finds nothing more.
func TestEventsPruned(t *testing.T) {
	testkit.Swapping(t)
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
	var pa, pb []Event
	if !testkit.Within(5*time.Second, func() bool {
		pa, _ = s.Events(ctx, EventQuery{CubeID: a.ID, Before: cutoff})
		pb, _ = s.Events(ctx, EventQuery{CubeID: b.ID, Before: cutoff})
		return len(pa)+len(pb) == 0
	}) {
		t.Fatalf("PruneEventsSoon left %d and %d", len(pa), len(pb))
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

// fill writes n events of a Cube, or with cube 0 of the server, straight
// into the table, step apart from start: one in 500 connected (source
// cube), one in 97 config, the others state, over thirty devices, each
// with its serial; one in 13 at debug, one in 1000 at warn.
func fill(t testing.TB, s *Store, cube int64, n int, start time.Time, step time.Duration) {
	t.Helper()
	_, err := s.db.Exec(`WITH RECURSIVE n(i) AS (SELECT 0 UNION ALL SELECT i + 1 FROM n WHERE i < ? - 1)
		INSERT INTO log (uuid, cube_id, time, kind, source, level, device, serial, data)
		SELECT printf('fill-%d-%d-%d', ?, ?, i), ?, strftime('%Y-%m-%dT%H:%M:%fZ', ? + i * ?, 'unixepoch'),
			CASE WHEN i % 500 = 0 THEN 'connected' WHEN i % 97 = 0 THEN 'config' ELSE 'state' END,
			CASE WHEN i % 500 = 0 THEN 'cube' ELSE 'device' END,
			CASE WHEN i % 1000 = 0 THEN 4 WHEN i % 13 = 0 THEN -4 ELSE 0 END,
			printf('0abc%02d', i % 30), printf('MEQ00000%02d', i % 30), ? FROM n`,
		n, cube, start.UnixMilli(), nullID(cube), float64(start.UnixMilli())/1000, step.Seconds(), stateData)
	if err != nil {
		t.Fatal(err)
	}
}

// A page reads one index of migration 0004 in id order, whatever its
// terms, instead of sorting every one of the Cube's rows (the review of
// 2026-11-06: 1.3 s a page at 90,000 events); each page holds what its
// terms ask, newest first, and a page by time is the events before it;
// forward from a cursor the same, oldest first, the oldest after it.
// A page of warnings or errors with no other term reads the index of
// those rows, migration 0011's, rather than every row of the Cube (11 s
// at 90,000 events on a Raspberry Pi 1, docs/store.md section 5).
// 90,000 events of one Cube beside 10,000 of another, 3000 under -short;
// the time of each page is logged. The rows lie from 29 days ago to one
// day ago, inside log.keep_days' default of 30: the writer's first prune
// runs in its goroutine and may come after the fill, and would delete
// rows filled at the edge (under QEMU, the server's one warning).
func TestEventsPageQuery(t *testing.T) {
	t.Parallel()
	s, a, b := eventsStore(t, t.TempDir())
	ctx := context.Background()
	n := 90000
	if testing.Short() {
		n = 3000
	}
	start := time.Now().UTC().Add(-29 * 24 * time.Hour).Truncate(time.Second)
	step := 28 * 24 * time.Hour / time.Duration(n)
	fill(t, s, a.ID, n, start, step)
	fill(t, s, b.ID, n/9, start, 9*step)
	fill(t, s, 0, n/30, start, 30*step)
	mid := start.Add(14 * 24 * time.Hour)
	anna, err := s.AddUser(ctx, User{Name: "anna", Role: RoleViewer}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE log SET effective_user_id = ?, real_user_id = ? WHERE cube_id = ? AND id % 211 = 0`, anna.ID, anna.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	// Anna's rows acting as bruno, as an integration's user linked to him.
	bruno, err := s.AddUser(ctx, User{Name: "bruno", Role: RoleViewer}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE log SET effective_user_id = ?, real_user_id = ? WHERE cube_id = ? AND id % 223 = 0`, bruno.ID, anna.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE log SET level = 8 WHERE level = 0 AND id % 997 = 0`); err != nil {
		t.Fatal(err)
	}
	cmd, err := s.AddCommand(ctx, Command{CubeID: a.ID, Action: "setpoint"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE log SET command_id = ? WHERE cube_id = ? AND id % 301 = 0`, cmd.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	info, warn, errs := LevelInfo, LevelWarn, LevelError
	shapes := []struct {
		name string
		q    EventQuery
	}{
		{"none", EventQuery{CubeID: a.ID}},
		{"id", EventQuery{CubeID: a.ID, BeforeID: int64(n / 2)}},
		{"time", EventQuery{CubeID: a.ID, Before: mid}},
		{"kind", EventQuery{CubeID: a.ID, Kind: "connected"}},
		{"kind and id", EventQuery{CubeID: a.ID, Kind: "connected", BeforeID: int64(n / 2)}},
		{"serial", EventQuery{CubeID: a.ID, Serial: "MEQ0000007"}},
		{"kind and serial", EventQuery{CubeID: a.ID, Kind: "state", Serial: "MEQ0000007"}},
		{"time and serial", EventQuery{CubeID: a.ID, Serial: "MEQ0000007", Before: mid}},
		{"source", EventQuery{CubeID: a.ID, Source: "cube"}},
		{"source and id", EventQuery{CubeID: a.ID, Source: "cube", BeforeID: int64(n / 2)}},
		{"info and above", EventQuery{CubeID: a.ID, MinLevel: &info}},
		{"warnings", EventQuery{CubeID: a.ID, MinLevel: &warn}},
		{"errors", EventQuery{CubeID: a.ID, MinLevel: &errs}},
		{"warnings and id", EventQuery{CubeID: a.ID, MinLevel: &warn, BeforeID: int64(n / 2)}},
		{"warnings by time", EventQuery{CubeID: a.ID, MinLevel: &warn, Before: mid}},
		{"warnings, with the server's", EventQuery{CubeID: a.ID, Server: true, MinLevel: &warn}},
		{"the server's warnings", EventQuery{CubeID: 0, MinLevel: &warn}},
		{"warnings of a kind", EventQuery{CubeID: a.ID, Kind: "connected", MinLevel: &warn}},
		{"a user", EventQuery{CubeID: a.ID, UserID: anna.ID}},
		{"a command", EventQuery{CubeID: a.ID, CommandID: cmd.ID}},
		{"a command, with the server's and the audit's", EventQuery{CubeID: a.ID, Server: true, CommandID: cmd.ID, Audit: true}},
		{"with the server's", EventQuery{CubeID: a.ID, Server: true}},
		{"with the server's, by time", EventQuery{CubeID: a.ID, Server: true, Before: mid, MinLevel: &info}},
		{"with the server's, a source", EventQuery{CubeID: a.ID, Server: true, Source: "cube"}},
		{"the server's", EventQuery{CubeID: 0}},
		{"since", EventQuery{CubeID: a.ID, Since: mid}},
		{"since and kind", EventQuery{CubeID: a.ID, Kind: "connected", Since: mid}},
		{"since and before", EventQuery{CubeID: a.ID, Since: start.Add(24 * time.Hour), Before: mid}},
		{"the other Cube", EventQuery{CubeID: b.ID}},
		// Forward from a cursor, oldest first (AP LogCursor).
		{"forward", EventQuery{CubeID: a.ID, Forward: true, AfterID: int64(n / 2)}},
		{"forward from the start", EventQuery{CubeID: a.ID, Forward: true}},
		{"forward, a kind", EventQuery{CubeID: a.ID, Forward: true, AfterID: int64(n / 2), Kind: "connected"}},
		{"forward, a serial", EventQuery{CubeID: a.ID, Forward: true, AfterID: int64(n / 2), Serial: "MEQ0000007"}},
		{"forward, a source", EventQuery{CubeID: a.ID, Forward: true, AfterID: int64(n / 2), Source: "cube"}},
		{"forward, warnings", EventQuery{CubeID: a.ID, Forward: true, AfterID: int64(n / 2), MinLevel: &warn}},
		{"forward, with the server's", EventQuery{CubeID: a.ID, Server: true, Forward: true, AfterID: int64(n / 2)}},
		{"forward, with the server's warnings", EventQuery{CubeID: a.ID, Server: true, Forward: true, AfterID: int64(n / 2), MinLevel: &warn}},
		{"forward, a user", EventQuery{CubeID: a.ID, Forward: true, AfterID: int64(n / 2), UserID: anna.ID}},
		{"forward, a command", EventQuery{CubeID: a.ID, Forward: true, AfterID: int64(n / 2), CommandID: cmd.ID}},
		{"forward and before", EventQuery{CubeID: a.ID, Forward: true, AfterID: int64(n / 4), BeforeID: int64(n / 2)}},
		{"forward since", EventQuery{CubeID: a.ID, Forward: true, Since: mid}},
		{"forward, the other Cube", EventQuery{CubeID: b.ID, Forward: true, AfterID: int64(n + n/18)}},
		// The forward shapes a log route sends, the server's walk and a
		// level with each.
		{"forward by time", EventQuery{CubeID: a.ID, Server: true, Forward: true, AfterID: int64(n / 4), Before: mid, MinLevel: &info}},
		{"forward since and before", EventQuery{CubeID: a.ID, Server: true, Forward: true, Since: start.Add(24 * time.Hour), Before: mid, MinLevel: &info}},
		{"forward since and kind", EventQuery{CubeID: a.ID, Server: true, Forward: true, Since: mid, Kind: "connected", MinLevel: &info}},
		{"forward, warnings of a kind", EventQuery{CubeID: a.ID, Server: true, Forward: true, AfterID: int64(n / 2), Kind: "connected", MinLevel: &warn}},
		{"forward, errors", EventQuery{CubeID: a.ID, Server: true, Forward: true, MinLevel: &errs}},
		{"forward, a device", EventQuery{CubeID: a.ID, Server: true, Forward: true, AfterID: int64(n / 2), Device: "0abc07", MinLevel: &info}},
		{"forward, a user with the server's", EventQuery{CubeID: a.ID, Server: true, Forward: true, AfterID: int64(n / 2), UserID: anna.ID, MinLevel: &info}},
		{"forward, a command, with the server's and the audit's", EventQuery{CubeID: a.ID, Server: true, Forward: true, AfterID: int64(n / 2), CommandID: cmd.ID, Audit: true, MinLevel: &info}},
		{"forward, every Cube", EventQuery{CubeIDs: []int64{a.ID, b.ID}, Server: true, Forward: true, AfterID: int64(n / 2), MinLevel: &info}},
	}
	// The shapes each walk of which reads migration 0011's index.
	warnIndex := map[string]bool{"warnings": true, "errors": true, "warnings and id": true,
		"warnings by time": true, "warnings, with the server's": true, "the server's warnings": true,
		"forward, warnings": true, "forward, with the server's warnings": true, "forward, errors": true}
	// The shapes each walk of which reads the index of its term, also with
	// the page's bound, which is a filter and no range of the walk.
	termIndex := map[string]string{"forward, a kind": "log_cube_kind", "forward, a source": "log_cube_source",
		"forward, a serial": "log_cube_serial", "with the server's, a source": "log_cube_source", "forward, warnings of a kind": "log_cube_kind"}
	// Each walk as Events sends it: bounded by the newest row where the
	// page goes forward or has more than one walk.
	newest, err := s.NewestEventID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, sh := range shapes {
		walks, queries, bounded := pageWalks(sh.q)
		if bounded {
			for i := range queries {
				queries[i] = pageBound(queries[i], newest)
			}
		}
		var plan []string
		for _, cube := range walks {
			for _, wq := range queries {
				query, args := eventsQuery(wq, cube, 200)
				rows, err := s.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, args...)
				if err != nil {
					t.Fatal(err)
				}
				for rows.Next() {
					var id, parent, unused int
					var detail string
					if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
						t.Fatal(err)
					}
					plan = append(plan, detail)
				}
				rows.Close()
			}
		}
		joined := strings.Join(plan, "; ")
		if sh.q.UserID != 0 && (!strings.Contains(joined, "log_user") || !strings.Contains(joined, "log_real_user")) {
			t.Errorf("%s: not by log_user and log_real_user: %s", sh.name, joined)
		}
		if strings.Contains(joined, "TEMP B-TREE") || strings.Contains(joined, "SCAN log") {
			t.Errorf("%s: %s", sh.name, joined)
		}
		if warnIndex[sh.name] && strings.Count(joined, "USING INDEX log_cube_warn (") != len(walks) {
			t.Errorf("%s: not every walk reads log_cube_warn: %s", sh.name, joined)
		}
		if ix := termIndex[sh.name]; ix != "" && strings.Count(joined, "USING INDEX "+ix+" (") != len(walks) {
			t.Errorf("%s: not every walk reads %s: %s", sh.name, ix, joined)
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
			if !slices.Contains(walks, e.CubeID) || (q.Kind != "" && e.Kind != q.Kind) || (q.Serial != "" && e.Serial != q.Serial) ||
				(q.Device != "" && e.Device != q.Device) || (q.CommandID != 0 && e.CommandID != q.CommandID) ||
				(q.Source != "" && e.Source != q.Source) || (q.UserID != 0 && e.EffectiveUserID != q.UserID && e.RealUserID != q.UserID) || (q.MinLevel != nil && e.Level < *q.MinLevel) ||
				(q.BeforeID > 0 && e.ID >= q.BeforeID) || (!q.Before.IsZero() && !e.Time.Before(q.Before)) ||
				(!q.Since.IsZero() && e.Time.Before(q.Since)) || (q.AfterID > 0 && e.ID <= q.AfterID) ||
				(i > 0 && !q.Forward && e.ID >= page[i-1].ID) || (i > 0 && q.Forward && e.ID <= page[i-1].ID) {
				t.Fatalf("%s: event %d: %+v", sh.name, i, e)
			}
		}
		if q.UserID != 0 && !slices.ContainsFunc(page, func(e Event) bool { return e.EffectiveUserID == bruno.ID && e.RealUserID == anna.ID }) {
			t.Errorf("%s: anna's rows acting as bruno left out", sh.name)
		}
		t.Logf("%-16s %3d events in %8v: %s", sh.name, len(page), took, joined)
	}
	page, err := s.Events(ctx, EventQuery{CubeID: a.ID, Before: mid, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM log WHERE cube_id = ? AND time < ? ORDER BY id DESC LIMIT 5`, a.ID, formatTime(mid))
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
	// A page forward is the oldest rows after its cursor, not the newest.
	after := int64(n / 2)
	fwd, err := s.Events(ctx, EventQuery{CubeID: a.ID, Server: true, Forward: true, AfterID: after, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.db.QueryContext(ctx, `SELECT id FROM log WHERE (cube_id = ? OR cube_id IS NULL) AND id > ? AND visibility = 'read' ORDER BY id LIMIT 5`, a.ID, after)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	want := []int64{}
	for next.Next() {
		var id int64
		next.Scan(&id)
		want = append(want, id)
	}
	if got := eventIDs(fwd); !slices.Equal(got, want) {
		t.Fatalf("the page after %d: %v, want %v", after, got, want)
	}
}

// eventIDs are the ids of a page, in its order.
func eventIDs(page []Event) []int64 {
	out := []int64{}
	for _, e := range page {
		out = append(out, e.ID)
	}
	return out
}

// A prune hands over between its chunks: a flush during it is answered,
// the event written, while the prune goes on to the end.
func TestEventsPruneYields(t *testing.T) {
	testkit.Swapping(t)
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
	case <-time.After(testkit.Scale(5 * time.Second)):
		t.Fatal("no prune at the writer's start")
	}
	s.AddEvent(ev(a.ID, time.Now(), "hello", "", 1))
	flush(t, s)
	flushed.Store(true)
	if page, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Kind: "hello"}); len(page) != 1 {
		t.Fatalf("the flush during the prune: %d events", len(page))
	}
	testkit.Eventually(t, "the prune's end", func() bool {
		page, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Kind: "state", Limit: 1})
		return len(page) == 0
	}, testkit.For(10*time.Second))
	if after.Load() == 0 {
		t.Fatalf("the flush waited for the whole prune (%d chunks)", steps.Load())
	}
}

// A done ctx ends a prune between two chunks; the writer's last write
// still follows.
func TestEventsPruneStops(t *testing.T) {
	testkit.Swapping(t)
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
	case <-time.After(testkit.Scale(10 * time.Second)):
		t.Fatal("the writer did not stop")
	}
	var left int
	// The prune's own line is the server's (logPruned), not one of the 500.
	if err := s.db.QueryRow(`SELECT count(*) FROM log WHERE kind NOT IN ('hello', 'log_pruned')`).Scan(&left); err != nil || left != 480 {
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
	t.Parallel()
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
		s.AddEvent(Event{CubeID: a.ID, Time: at.Add(time.Duration(i) * time.Second), Kind: "state", Source: "device", Device: rf,
			Serial: fmt.Sprintf("MEQ00000%02d", i%30), Data: json.RawMessage(data)})
	}
	flush(t, s)
	grown := size() - start
	per := grown / n
	t.Logf("a state event of %d bytes of JSON takes %d bytes in the file, indexes included (%d events, %d bytes)", sample, per, n, grown)
	if per > 1024 {
		t.Fatalf("%d bytes per event, more than 1 KB", per)
	}
}
