// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The log's sources in the store (AP Logging v1.1, Block 3; migration
// 0009): the rebuild and its backfill, an older binary on the new file,
// the server's rows beside a Cube's, the levels, and the prune of the
// server's rows.

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// openAt opens a store whose file stops at a schema version.
func openAt(t *testing.T, dir string, version int) *Store {
	t.Helper()
	s, err := Open(dir, Options{Version: "test", Logger: quiet(), readerVersion: version})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := s.SchemaVersion(); v != version {
		t.Fatalf("version %d, want %d", v, version)
	}
	return s
}

// Migration 0009 keeps every row, its id and its uuid, takes a NULL
// cube_id, and fills source, level and serial from the rows' JSON: the
// serial from the event, its device view, another row of the device, or
// for the Cube's own C line the Cube's.
func TestMigration9(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s8 := openAt(t, dir, 8)
	cube, err := s8.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s8.db.Exec(`UPDATE cubes SET rf_address = '0a0001' WHERE id = ?`, cube.ID); err != nil {
		t.Fatal(err)
	}
	rows := []struct{ kind, device, data string }{
		{"state", "0a0101", `{"kind":"state","device":"0a0101","state":{"serial":"MEQ0000101"}}`},
		{"config", "0a0101", `{"kind":"config","device":"0a0101"}`},                                         // another row of the device
		{"config", "0a0001", `{"kind":"config","device":"0a0001"}`},                                         // the Cube's own C line
		{"config", "0a0102", `{"kind":"config","device":"0a0102","serial":"MEQ0000102","source":"device"}`}, // Block 1's
		{"connected", "", `{"kind":"connected"}`},
		{"disconnected", "", `{"kind":"disconnected","error":"gone"}`},
		{"hello", "", `{"kind":"hello"}`},
		{"restore_done", "", `{"kind":"restore_done"}`},
		{"device_forgotten", "0a0103", `{"kind":"device_forgotten","device":"0a0103"}`}, // no serial anywhere
	}
	for i, r := range rows {
		if _, err := s8.db.Exec(`INSERT INTO events (uuid, cube_id, time, kind, device, data) VALUES (?, ?, ?, ?, ?, ?)`,
			"uuid-"+r.kind+r.device, cube.ID, formatTime(time.Unix(int64(1000+i), 0)), r.kind, r.device, r.data); err != nil {
			t.Fatal(err)
		}
	}
	s8.Close()

	s := open(t, dir)
	if v, mr := s.SchemaVersion(); v != SchemaVersion() || mr != 1 {
		t.Fatalf("schema %d min_reader %d", v, mr)
	}
	page, err := s.Events(ctx, EventQuery{CubeID: cube.ID, Limit: 100})
	if err != nil || len(page) != len(rows) {
		t.Fatalf("%d of %d rows, %v", len(page), len(rows), err)
	}
	want := map[string]struct {
		source, serial string
		level          int
	}{
		"state0a0101":            {"device", "MEQ0000101", LevelInfo},
		"config0a0101":           {"device", "MEQ0000101", LevelInfo},
		"config0a0001":           {"cube", "KEQ0000001", LevelInfo},
		"config0a0102":           {"device", "MEQ0000102", LevelInfo},
		"connected":              {"cube", "", LevelInfo},
		"disconnected":           {"cube", "", LevelWarn},
		"hello":                  {"cube", "", LevelDebug},
		"restore_done":           {"system", "", LevelInfo},
		"device_forgotten0a0103": {"device", "", LevelInfo},
	}
	for i, e := range page {
		w := want[e.Kind+e.Device]
		if e.Source != w.source || e.Serial != w.serial || e.Level != w.level || e.UserID != 0 {
			t.Errorf("%s %s: source %q serial %q level %d, want %+v", e.Kind, e.Device, e.Source, e.Serial, e.Level, w)
		}
		if e.ID != int64(len(rows)-i) {
			t.Errorf("%s: id %d, want %d", e.Kind, e.ID, len(rows)-i)
		}
	}
	var uuid string
	if err := s.db.QueryRow(`SELECT uuid FROM events WHERE kind = 'hello'`).Scan(&uuid); err != nil || uuid != "uuid-hello" {
		t.Fatalf("uuid %q %v", uuid, err)
	}
	startWriter(t, s)
	s.AddEvent(Event{Kind: "server_started", Source: "system", Data: json.RawMessage(`{}`)})
	flush(t, s)
	if got, err := s.Events(ctx, EventQuery{Kind: "server_started", Limit: 1}); err != nil || len(got) != 1 || got[0].CubeID != 0 {
		t.Fatalf("a row of the server: %+v %v", got, err)
	}
}

// A binary of version 8 opens a file of version 9 (min_reader 1), reads
// its Cube's rows and writes rows without the new columns; this binary's
// writer fills those at its start.
func TestOlderReaderOnNine(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s9 := open(t, dir)
	cube, err := s9.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	startWriter(t, s9)
	s9.AddEvent(Event{CubeID: cube.ID, Kind: "connected", Source: "cube", Data: json.RawMessage(`{"kind":"connected"}`)})
	s9.AddEvent(Event{Kind: "server_started", Source: "system", Data: json.RawMessage(`{}`)})
	flush(t, s9)
	s9.Close()

	s8, err := Open(dir, Options{Version: "test", Logger: quiet(), readerVersion: 8})
	if err != nil {
		t.Fatalf("a binary of version 8: %v", err)
	}
	// Its queries, as events.go of version 8 had them.
	var n int
	if err := s8.db.QueryRow(`SELECT count(*) FROM events WHERE cube_id = ?`, cube.ID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("its Cube's rows: %d %v", n, err)
	}
	if _, err := s8.db.Exec(`INSERT INTO events (uuid, cube_id, time, kind, device, data) VALUES (?, ?, ?, ?, ?, ?)`,
		"old-1", cube.ID, formatTime(time.Now()), "state", "0a0101", `{"kind":"state","device":"0a0101","state":{"serial":"MEQ0000101"}}`); err != nil {
		t.Fatalf("its insert: %v", err)
	}
	if _, err := s8.db.Exec(`INSERT INTO events (uuid, cube_id, time, kind, device, data) VALUES (?, ?, ?, ?, ?, ?)`,
		"old-2", cube.ID, formatTime(time.Now()), "disconnected", "", `{"kind":"disconnected"}`); err != nil {
		t.Fatalf("its insert: %v", err)
	}
	s8.Close()

	s := open(t, dir)
	startWriter(t, s)
	flush(t, s)
	page, err := s.Events(ctx, EventQuery{CubeID: cube.ID, Limit: 10})
	if err != nil || len(page) != 3 {
		t.Fatalf("%d rows %v", len(page), err)
	}
	if e := page[1]; e.Kind != "state" || e.Source != "device" || e.Serial != "MEQ0000101" || e.Level != LevelInfo {
		t.Fatalf("the older binary's state row, filled: %+v", e)
	}
	if e := page[0]; e.Kind != "disconnected" || e.Source != "cube" || e.Level != LevelWarn {
		t.Fatalf("the older binary's disconnected row, filled: %+v", e)
	}
}

// A Cube's page with the server's rows merges two walks by id: every row
// once when paged back by id, the other Cube's never, within the time
// bounds and the terms.
func TestEventsWithServerRows(t *testing.T) {
	s, a, b := eventsStore(t, t.TempDir())
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Millisecond)
	var wantA []int // the numbers of a's page: a's and the server's
	for i := 0; i < 90; i++ {
		e := ev(a.ID, at.Add(time.Duration(i)*time.Second), "state", "0a0101", i)
		switch i % 3 {
		case 1:
			e.CubeID, e.Kind, e.Source = 0, "server_started", "system"
		case 2:
			e.CubeID = b.ID
		}
		if e.CubeID != b.ID {
			wantA = append(wantA, i)
		}
		s.AddEvent(e)
	}
	flush(t, s)
	var got []int
	before := int64(0)
	for {
		page, err := s.Events(ctx, EventQuery{CubeID: a.ID, Server: true, BeforeID: before, Limit: 7})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for i, e := range page {
			if e.CubeID == b.ID || i > 0 && e.ID >= page[i-1].ID {
				t.Fatalf("page before %d: %+v", before, page)
			}
			got = append([]int{numberOf(t, e)}, got...)
		}
		before = page[len(page)-1].ID
	}
	if len(got) != len(wantA) {
		t.Fatalf("paged %d of %d", len(got), len(wantA))
	}
	for i := range got {
		if got[i] != wantA[i] {
			t.Fatalf("event %d: %d, want %d", i, got[i], wantA[i])
		}
	}
	// Within a time range, of a source.
	page, err := s.Events(ctx, EventQuery{CubeID: a.ID, Server: true, Since: at.Add(30 * time.Second), Before: at.Add(60 * time.Second), Source: "system", Limit: 100})
	if err != nil || len(page) != 10 {
		t.Fatalf("the server's rows from 30 s to 60 s: %d %v", len(page), err)
	}
	for _, e := range page {
		if e.CubeID != 0 || e.Source != "system" || e.Time.Before(at.Add(30*time.Second)) || !e.Time.Before(at.Add(60*time.Second)) {
			t.Fatalf("%+v", e)
		}
	}
	// Without Server, a Cube's own rows alone.
	if page, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Limit: 100}); len(page) != 30 {
		t.Fatalf("a's own rows: %d", len(page))
	}
}

// A page of several Cubes, with the server's rows, merges a walk each by
// id: every row once when paged back by id, newest first, within the
// terms; without Server, the Cubes' own rows; a Cube named twice is
// walked once.
func TestEventsOfSeveralCubes(t *testing.T) {
	s, a, b := eventsStore(t, t.TempDir())
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Millisecond)
	for i := 0; i < 60; i++ {
		e := ev(a.ID, at.Add(time.Duration(i)*time.Second), "state", "0a0101", i)
		switch i % 3 {
		case 1:
			e.CubeID, e.Kind, e.Source = 0, "server_started", "system"
		case 2:
			e.CubeID = b.ID
		}
		s.AddEvent(e)
	}
	flush(t, s)
	var got []int
	before := int64(0)
	for {
		page, err := s.Events(ctx, EventQuery{CubeIDs: []int64{b.ID, a.ID, b.ID}, Server: true, BeforeID: before, Limit: 7})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for i, e := range page {
			if i > 0 && e.ID >= page[i-1].ID {
				t.Fatalf("page before %d not newest first: %+v", before, page)
			}
			got = append([]int{numberOf(t, e)}, got...)
		}
		before = page[len(page)-1].ID
	}
	if len(got) != 60 {
		t.Fatalf("paged %d of 60", len(got))
	}
	for i := range got {
		if got[i] != i {
			t.Fatalf("event %d: %d", i, got[i])
		}
	}
	if page, _ := s.Events(ctx, EventQuery{CubeIDs: []int64{a.ID, b.ID}, Limit: 100}); len(page) != 40 {
		t.Fatalf("the Cubes' own rows: %d", len(page))
	}
	page, err := s.Events(ctx, EventQuery{CubeIDs: []int64{a.ID, b.ID}, Server: true, Source: "system", Limit: 100})
	if err != nil || len(page) != 20 {
		t.Fatalf("the server's rows by their source: %d %v", len(page), err)
	}
}

// A level keeps that level and above; nil keeps every level.
func TestEventsByLevel(t *testing.T) {
	s, a, _ := eventsStore(t, t.TempDir())
	ctx := context.Background()
	levels := []int{LevelDebug, LevelInfo, LevelWarn, LevelError, LevelDebug, LevelInfo}
	for i, l := range levels {
		e := ev(a.ID, time.Now(), "state", "", i)
		e.Level = l
		s.AddEvent(e)
	}
	flush(t, s)
	for _, c := range []struct {
		min  *int
		want int
	}{{nil, 6}, {ptr(LevelDebug), 6}, {ptr(LevelInfo), 4}, {ptr(LevelWarn), 2}, {ptr(LevelError), 1}} {
		page, err := s.Events(ctx, EventQuery{CubeID: a.ID, MinLevel: c.min, Limit: 100})
		if err != nil || len(page) != c.want {
			t.Fatalf("min %v: %d events, want %d (%v)", c.min, len(page), c.want, err)
		}
		for _, e := range page {
			if c.min != nil && e.Level < *c.min {
				t.Fatalf("min %d: %+v", *c.min, e)
			}
		}
	}
	for name, want := range map[string]int{"debug": LevelDebug, "INFO": LevelInfo, "warn": LevelWarn, "warning": LevelWarn, "error": LevelError} {
		l, ok := ParseLevel(name)
		back, _ := ParseLevel(LevelName(l))
		if !ok || l != want || back != want {
			t.Errorf("level %q: %d %v, back %d", name, l, ok, back)
		}
	}
	if _, ok := ParseLevel("loud"); ok {
		t.Error("an unknown level read")
	}
}

func ptr(n int) *int { return &n }

// The prune takes the server's old rows as it takes a Cube's.
func TestPruneServerEvents(t *testing.T) {
	s, a, _ := eventsStore(t, t.TempDir())
	ctx := context.Background()
	now := time.Now().UTC()
	old, recent := now.Add(-40*24*time.Hour), now.Add(-time.Hour)
	for i, at := range []time.Time{old, recent, old, recent} {
		e := ev(0, at, "server_started", "", i)
		if i >= 2 {
			e.CubeID = a.ID
		}
		s.AddEvent(e)
	}
	flush(t, s)
	removed, err := s.PruneEvents(ctx, now)
	if err != nil || removed != 2 {
		t.Fatalf("removed %d, %v", removed, err)
	}
	page, _ := s.Events(ctx, EventQuery{CubeID: a.ID, Server: true, Kind: "server_started", Limit: 10})
	if len(page) != 2 || page[0].Time.Before(recent.Add(-time.Second)) || page[1].Time.Before(recent.Add(-time.Second)) {
		t.Fatalf("left: %+v", page)
	}
}
