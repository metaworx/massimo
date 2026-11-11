// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The stores the released binaries wrote (docs/store.md, "Migrations";
// AP SuiteGaps v1.0, Block 2): testdata/releases/<tag>.sql is the text
// dump of the store a release's serve wrote against the fake Cube, made
// by tools/store-fixture/make.sh, one for each release since the store
// came, v0.7.0-alpha.1. Each is loaded into a file as the release left
// it and opened, which migrates it to this binary's version: the file
// stays sound, every table keeps its rows, the log's exactly, every event
// has its source, the log, the users and the backups read, and the
// backups' sequence starts at their highest id (migration 0013).
func TestMigratesReleasedStores(t *testing.T) {
	dumps, err := filepath.Glob(filepath.Join("testdata", "releases", "v*.sql"))
	if err != nil || len(dumps) < 6 {
		t.Fatalf("the releases' dumps: %v, %v", dumps, err)
	}
	for _, dump := range dumps {
		t.Run(strings.TrimSuffix(filepath.Base(dump), ".sql"), func(t *testing.T) {
			dir := t.TempDir()
			before, version := loadDump(t, dump, Path(dir))
			if version >= SchemaVersion() {
				t.Fatalf("the dump is at schema %d, this binary's is %d", version, SchemaVersion())
			}
			s := open(t, dir)
			ctx := context.Background()
			if v, _ := s.SchemaVersion(); v != SchemaVersion() {
				t.Fatalf("schema %d after the migration, want %d", v, SchemaVersion())
			}
			var ok string
			if err := s.db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&ok); err != nil || ok != "ok" {
				t.Fatalf("integrity_check: %q, %v", ok, err)
			}
			rows, err := s.db.QueryContext(ctx, `PRAGMA foreign_key_check`)
			if err != nil {
				t.Fatal(err)
			}
			if rows.Next() {
				var table string
				rows.Scan(&table)
				t.Errorf("foreign_key_check: a row of %s points nowhere", table)
			}
			rows.Close()
			after := tableRows(t, s.db)
			for table, n := range before {
				switch {
				case table == "events" && after[table] != n:
					t.Errorf("events: %d rows after the migration, %d before", after[table], n)
				case after[table] < n:
					t.Errorf("%s: %d rows after the migration, %d before", table, after[table], n)
				}
			}
			var unsourced int
			if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM events WHERE source = ''`).Scan(&unsourced); err != nil || unsourced != 0 {
				t.Errorf("%d events without a source, %v", unsourced, err)
			}
			cubes, err := s.Cubes(ctx)
			if err != nil || len(cubes) != 1 {
				t.Fatalf("cubes: %+v, %v", cubes, err)
			}
			page, err := s.Events(ctx, EventQuery{CubeID: cubes[0].ID, Server: true})
			if err != nil || len(page) != min(after["events"], MaxEventsPage) {
				t.Errorf("the log: %d events of %d, %v", len(page), after["events"], err)
			}
			if _, err := s.Users(ctx); err != nil {
				t.Errorf("users: %v", err)
			}
			if b, err := s.Backups(ctx, cubes[0].ID); err != nil || len(b) != after["backups"] {
				t.Errorf("backups: %d of %d, %v", len(b), after["backups"], err)
			}
			var top, seq int64
			if err := s.db.QueryRowContext(ctx, `SELECT coalesce(max(id), 0), coalesce((SELECT seq FROM sqlite_sequence WHERE name = 'backups'), 0) FROM backups`).Scan(&top, &seq); err != nil || seq != top {
				t.Errorf("backups: sqlite_sequence at %d, the highest id %d, %v", seq, top, err)
			}
			if _, err := s.Settings(ctx); err != nil {
				t.Errorf("settings: %v", err)
			}
		})
	}
}

// loadDump writes the dump into a new file at path, as the release left
// it, and returns the rows of each table and the file's schema version.
func loadDump(t *testing.T, dump, path string) (map[string]int, int) {
	t.Helper()
	text, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(string(text)); err != nil {
		t.Fatalf("%s: %v", dump, err)
	}
	var version int
	if err := db.QueryRow(`SELECT max(version) FROM schema_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	return tableRows(t, db), version
}

// tableRows counts the rows of every table of db.
func tableRows(t *testing.T, db *sql.DB) map[string]int {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
	}
	rows.Close()
	out := map[string]int{}
	for _, table := range tables {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM "` + table + `"`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out[table] = n
	}
	return out
}
