// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// openBefore opens a store in dir with the migrations before version,
// as a binary built for the one before it.
func openBefore(t *testing.T, dir string, version int, now func() time.Time) *Store {
	t.Helper()
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(embedded, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() >= fmt.Sprintf("%04d", version) {
			break
		}
		data, _ := embedded.ReadFile("migrations/" + e.Name())
		before[e.Name()] = &fstest.MapFile{Data: data}
	}
	s, err := Open(dir, Options{Logger: quiet(), migrations: before, readerVersion: version - 1, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Migration 0014 keeps every command with its id and its steps, every
// audit row's link to its command, and every event as a row of the log;
// the commands' sequence starts at the highest id, so that the prune of
// the newest command leaves its id unused; and the pruned command's log
// rows lose their link, not themselves.
func TestLogMigration(t *testing.T) {
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Second)
	clock := func() time.Time { return at }
	dir := t.TempDir()
	old := openBefore(t, dir, 14, clock)
	c, _ := old.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	first, err := old.AddCommand(ctx, Command{CubeID: c.ID, Action: "setpoint", Target: "device:0a0101"})
	if err != nil {
		t.Fatal(err)
	}
	step, err := old.AddStep(ctx, first.ID, Command{CubeID: c.ID, Target: "device:0a0101"})
	if err != nil {
		t.Fatal(err)
	}
	newest, err := old.AddCommand(ctx, Command{CubeID: c.ID, Action: "setpoint", Target: "device:0a0102"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{first.ID, step.ID, newest.ID} {
		if err := old.SetCommandState(ctx, id, "acknowledged", 200, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := old.Audit(ctx, AuditEntry{Action: "setpoint", CubeID: c.ID, CommandID: newest.ID}); err != nil {
		t.Fatal(err)
	}
	// The writer of this binary writes the log; the older binary's wrote
	// the events.
	if _, err := old.db.ExecContext(ctx, `INSERT INTO events (uuid, cube_id, time, kind, source, level, device, serial, user_id, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), c.ID, formatTime(at), "state", "device", 0, "0a0101", "MEQ0000001", nil, `{"kind":"state"}`); err != nil {
		t.Fatal(err)
	}
	old.Close()

	s, err := Open(dir, Options{Logger: quiet(), Now: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var fk int
	if err := s.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys %d after the migration, %v", fk, err)
	}
	got, ok, err := s.Command(ctx, first.ID)
	if err != nil || !ok || len(got.Steps) != 1 || got.Steps[0].ID != step.ID {
		t.Fatalf("the first command after the migration: %+v %v %v", got, ok, err)
	}
	var linked int64
	if err := s.db.QueryRow(`SELECT coalesce(command_id, 0) FROM audit`).Scan(&linked); err != nil || linked != newest.ID {
		t.Fatalf("the audit row names command %d, %v; want %d", linked, err, newest.ID)
	}
	page, err := s.Events(ctx, EventQuery{CubeID: c.ID})
	if err != nil || len(page) != 1 || page[0].Serial != "MEQ0000001" || page[0].Visibility != VisibilityRead || page[0].CommandID != 0 {
		t.Fatalf("the log after the migration: %+v %v", page, err)
	}
	var seq int64
	if err := s.db.QueryRow(`SELECT seq FROM sqlite_sequence WHERE name = 'commands'`).Scan(&seq); err != nil || seq != newest.ID {
		t.Fatalf("the commands' sqlite_sequence %d, %v; want %d", seq, err, newest.ID)
	}

	// A row of the newest command, which the prune then removes: the
	// next command takes a new id, and the row stays without its link.
	s.AddEvent(Event{CubeID: c.ID, Kind: "state", Data: json.RawMessage(`{}`), CommandID: newest.ID})
	if err := s.InsertEvent(ctx, Event{CubeID: c.ID, Kind: "command", Data: json.RawMessage(`{}`), CommandID: newest.ID}); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.Events(ctx, EventQuery{CubeID: c.ID, CommandID: newest.ID}); err != nil || len(rows) != 1 || rows[0].CommandID != newest.ID {
		t.Fatalf("the command's rows: %+v %v", rows, err)
	}
	if err := s.SetSetting(ctx, CommandsKeepDaysKey, 1); err != nil {
		t.Fatal(err)
	}
	at = at.Add(48 * time.Hour)
	next, err := s.AddCommand(ctx, Command{CubeID: c.ID, Action: "setpoint"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Command(ctx, newest.ID); ok {
		t.Fatal("the newest command was not pruned")
	}
	if next.ID <= newest.ID {
		t.Fatalf("the next command is %d, the pruned newest was %d", next.ID, newest.ID)
	}
	if rows, err := s.Events(ctx, EventQuery{CubeID: c.ID, Kind: "command"}); err != nil || len(rows) != 1 || rows[0].CommandID != 0 {
		t.Fatalf("the pruned command's row: %+v %v", rows, err)
	}
}

// A binary built before migration 0014 reads and writes the log through
// the view events, with its own statements: its writer's insert (and
// one before 0009, without source, level and serial), the update of its
// start and its prune's delete, which goes on chunk by chunk.
func TestEventsViewForOlderBinaries(t *testing.T) {
	ctx := context.Background()
	s := open(t, t.TempDir())
	c, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	old := time.Now().Add(-90 * 24 * time.Hour)
	for i := range 3 {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO events (uuid, cube_id, time, kind, source, level, device, serial, user_id, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newUUID(), c.ID, formatTime(old.Add(time.Duration(i)*time.Second)), "state", "", 0, "0a0101", "", nil, `{"source":"device","serial":"MEQ0000001"}`); err != nil {
			t.Fatalf("v13's insert: %v", err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO events (uuid, cube_id, time, kind, device, data) VALUES (?, ?, ?, ?, ?, ?)`,
		newUUID(), c.ID, formatTime(time.Now()), "hello", "", `{}`); err != nil {
		t.Fatalf("v8's insert: %v", err)
	}
	if err := s.fillSources(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE events SET source = 'cube' WHERE cube_id = ? AND source = 'device' AND kind = 'state' AND id = (SELECT min(id) FROM events)`, c.ID); err != nil {
		t.Fatalf("v13's update: %v", err)
	}
	var unsourced, viewed, logged, cubeRows int
	s.db.QueryRow(`SELECT count(*) FROM log WHERE source = ''`).Scan(&unsourced)
	s.db.QueryRow(`SELECT count(*) FROM events`).Scan(&viewed)
	s.db.QueryRow(`SELECT count(*) FROM log WHERE visibility = 'read'`).Scan(&logged)
	s.db.QueryRow(`SELECT count(*) FROM events WHERE source = 'cube'`).Scan(&cubeRows)
	if unsourced != 0 || viewed != 4 || logged != 4 || cubeRows != 2 {
		t.Fatalf("through the view: %d unsourced, %d viewed, %d in the log, %d of the Cube", unsourced, viewed, logged, cubeRows)
	}
	// v13's prune, chunk by chunk: the view counts no row, so it removes
	// one chunk a pass.
	cutoff := formatTime(time.Now().Add(-30 * 24 * time.Hour))
	if _, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE id IN (SELECT id FROM events WHERE cube_id = ? AND time < ? LIMIT ?)`, c.ID, cutoff, 2); err != nil {
		t.Fatalf("v13's prune: %v", err)
	}
	s.db.QueryRow(`SELECT count(*) FROM log`).Scan(&logged)
	if logged != 2 {
		t.Fatalf("%d rows after one chunk of 2, want 2", logged)
	}
}

// A page by command holds that command's rows alone, and the rows of
// the audit visibility only for a page that asks for them.
func TestLogCommandAndVisibility(t *testing.T) {
	ctx := context.Background()
	s := open(t, t.TempDir())
	c, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	cmd, err := s.AddCommand(ctx, Command{CubeID: c.ID, Action: "setpoint"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []Event{
		{CubeID: c.ID, Kind: "command", CommandID: cmd.ID},
		{CubeID: c.ID, Kind: "state", CommandID: cmd.ID},
		{CubeID: c.ID, Kind: "state"},
		{Kind: "login", Visibility: VisibilityAudit},
	} {
		e.Data = json.RawMessage(`{}`)
		if err := s.InsertEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	kinds := func(q EventQuery) string {
		t.Helper()
		rows, err := s.Events(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, r := range rows {
			out = append(out, r.Kind)
		}
		return strings.Join(out, " ")
	}
	if got := kinds(EventQuery{CubeID: c.ID, CommandID: cmd.ID}); got != "state command" {
		t.Errorf("the command's page: %q", got)
	}
	if got := kinds(EventQuery{CubeID: c.ID, Server: true}); got != "state state command" {
		t.Errorf("a reader's page: %q", got)
	}
	if got := kinds(EventQuery{CubeID: c.ID, Server: true, Audit: true}); got != "login state state command" {
		t.Errorf("an auditor's page: %q", got)
	}
	if _, err := s.db.Exec(`INSERT INTO log (uuid, time, kind, data, visibility) VALUES ('x', '', 'k', '{}', 'secret')`); err == nil {
		t.Error("a visibility of neither read nor audit was taken")
	}
}

// A migration declaring "-- foreign_keys: off" runs without them and is
// refused, the file left as it was, when a row then names no row; the
// connection has its foreign keys again either way.
func TestMigrationForeignKeysOff(t *testing.T) {
	dir := t.TempDir()
	orphan := testMigrations(t, map[string]string{"0002_orphan.sql": "-- min_reader: 1\n-- foreign_keys: off\n" +
		"INSERT INTO audit (uuid, time, action, cube_id) VALUES ('a', '', 'x', 99);\n"})
	if _, err := Open(dir, Options{Logger: quiet(), migrations: orphan}); err == nil || !strings.Contains(err.Error(), "names no row of cubes") {
		t.Fatalf("open with an orphan: %v", err)
	}
	clean := testMigrations(t, map[string]string{"0002_clean.sql": "-- min_reader: 1\n-- foreign_keys: off\n" +
		"CREATE TABLE cubes_new AS SELECT * FROM cubes; DROP TABLE cubes_new;\n"})
	s, err := Open(dir, Options{Logger: quiet(), migrations: clean})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var fk, audit int
	s.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk)
	s.db.QueryRow(`SELECT count(*) FROM audit`).Scan(&audit)
	if v, _ := s.SchemaVersion(); v != 2 || fk != 1 || audit != 0 {
		t.Fatalf("version %d, foreign_keys %d, %d audit rows", v, fk, audit)
	}
}
