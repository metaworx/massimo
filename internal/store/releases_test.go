// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
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
// has its source, the log, the users and the backups read, the
// backups' and the commands' sequences start at their highest id
// (migrations 0013 and 0014), and 0024 has given every row of the log
// and the audit with a user its real user, the same, and every live
// backup its file in backup_data. The migrated file's schema is a fresh
// store's, by SQLite's pragmas (schemaOf), and no column holds fewer
// values that are not NULL than before, but for those a migration moves,
// drops or empties on purpose (columnFates; AP MaintBranch v1.0, D4-A):
// a migration that makes a table again and leaves out a column of
// another line of releases fails here (docs/store.md, "Two lines of
// releases"). TestReleasedStoresDownAgain takes each down again
// (raise_test.go).
func TestMigratesReleasedStores(t *testing.T) {
	t.Parallel()
	dumps, err := filepath.Glob(filepath.Join("testdata", "releases", "v*.sql"))
	if err != nil || len(dumps) < 6 {
		t.Fatalf("the releases' dumps: %v, %v", dumps, err)
	}
	want := schemaOf(t, openFresh(t, t.TempDir()).db)
	if len(want) == 0 {
		t.Fatal("a fresh store has no schema")
	}
	for _, dump := range dumps {
		t.Run(strings.TrimSuffix(filepath.Base(dump), ".sql"), func(t *testing.T) {
			dir := t.TempDir()
			before, version := loadDump(t, dump, Path(dir))
			valuesBefore := dumpValues(t, Path(dir))
			// A tag's own dump, which make release commits before the
			// release commit, is at the binary's schema: opened as it is.
			if version > SchemaVersion() {
				t.Fatalf("the dump is at schema %d, this binary's is %d", version, SchemaVersion())
			}
			s := openFresh(t, dir)
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
			if got := schemaOf(t, s.db); !slices.Equal(got, want) {
				for _, l := range got {
					if !slices.Contains(want, l) {
						t.Errorf("schema: %s, which a fresh store does not have", l)
					}
				}
				for _, l := range want {
					if !slices.Contains(got, l) {
						t.Errorf("schema: no %s, which a fresh store has", l)
					}
				}
			}
			valuesAfter := columnValues(t, s.db)
			for column, n := range valuesBefore {
				moved, ok := columnAfter(column)
				switch {
				case !ok:
				case valuesAfter[moved] < n:
					t.Errorf("%s: %d values after the migration (%s), %d before", column, valuesAfter[moved], moved, n)
				}
			}
			after := tableRows(t, s.db)
			// 0014 renamed the events the log.
			after["events"] = after["log"]
			for table, n := range before {
				switch {
				case table == "events" && after[table] != n:
					t.Errorf("events: %d rows of the log after the migration, %d before", after[table], n)
				case after[table] < n:
					t.Errorf("%s: %d rows after the migration, %d before", table, after[table], n)
				}
			}
			var unsourced int
			if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM log WHERE source = ''`).Scan(&unsourced); err != nil || unsourced != 0 {
				t.Errorf("%d events without a source, %v", unsourced, err)
			}
			for _, table := range []string{"log", "audit"} {
				var users, real int
				// The names are the test's own.
				if err := s.db.QueryRowContext(ctx, `SELECT count(effective_user_id), count(*) FILTER (WHERE real_user_id IS effective_user_id AND real_user_id IS NOT NULL) FROM `+table).
					Scan(&users, &real); err != nil || real != users {
					t.Errorf("%s: %d rows with a user, %d with the same real user, %v", table, users, real, err)
				}
			}
			var live, files int
			if err := s.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM backups WHERE deleted_at IS NULL), (SELECT count(*) FROM backup_data)`).Scan(&live, &files); err != nil || live != files {
				t.Errorf("%d live backups, %d files, %v", live, files, err)
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
			for _, table := range []string{"backups", "commands"} {
				var top, seq int64
				// The names are the test's own.
				if err := s.db.QueryRowContext(ctx, `SELECT coalesce(max(id), 0), coalesce((SELECT seq FROM sqlite_sequence WHERE name = ?), 0) FROM `+table, table).Scan(&top, &seq); err != nil || seq != top {
					t.Errorf("%s: sqlite_sequence at %d, the highest id %d, %v", table, seq, top, err)
				}
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

// columnFate is what a migration did to a column of a released store
// that leaves fewer values under its name: where they went, or that they
// went on purpose.
type columnFate struct {
	// migration is the number of the migration that did it.
	migration int
	// to is the table.column that holds the values since, "" where the
	// migration drops or empties the column on purpose.
	to string
	// why says what the migration did.
	why string
}

// columnFates are the columns of the released stores, as table.column,
// whose values a migration moves elsewhere, drops or empties on purpose
// (AP MaintBranch v1.0, D4-A): every other column holds at least as many
// values that are not NULL after the migration as before. A table
// renamed as a whole is in renamedTables. All are 0024's, the raise of
// AP StoreRaise v1.2, whose decisions (D2-A, …) and losses (L4, L7, L8,
// as losses0024 and 0024_raise.down.sql name them) the reasons cite.
var columnFates = map[string]columnFate{
	"log.user_id":                 {24, "log.effective_user_id", "renamed, whose rights a row was made with (D2-A)"},
	"audit.user_id":               {24, "audit.effective_user_id", "renamed, as the log's (D2-A)"},
	"user_identities.user_id":     {24, "user_identities.effective_user_id", "renamed, as the log's (D2-A)"},
	"user_identities.own_user_id": {24, "user_identities.real_user_id", "renamed, as the log's (D2-A)"},
	"backups.body":                {24, "backup_data.data", "the backups' files in a table of their own (D5-B)"},
	"cube_options.proxy_virtual":  {24, "cubes.proxy_virtual", "the table dropped, its one option a column of cubes"},
	"cube_options.cube_id":        {24, "", "the table dropped: its key"},
	"cube_options.updated_at":     {24, "", "the table dropped: who set an option, and when, is lost (L7)"},
	"cube_options.updated_by":     {24, "", "the table dropped: who set an option, and when, is lost (L7)"},
	"tokens.revoked_at":           {24, "tokens.deleted_at", "renamed, every table's tombstone (D7)"},
	"tokens.deleted_at":           {24, "", "dropped, never written (D7, L8)"},
	"commands.deleted_at":         {24, "", "dropped, never written (D9-A, L8)"},
	"integrations.key":            {24, "", "dropped: an integration's users are named by its uuid (D3-A, L4)"},
	"integrations.user_id":        {24, "", "dropped: its users name it in users.integration_id (D8-C)"},
	"integrations.system_user_id": {24, "", "dropped: its users name it in users.integration_id (D8-C)"},
}

// renamedTables are the tables a migration renamed, every column with
// them: 0014 made the events the log.
var renamedTables = map[string]string{"events": "log"}

// columnAfter is the table.column of a migrated store that holds the
// values of column of a released one, and false where a migration drops
// or empties it on purpose (columnFates).
func columnAfter(column string) (string, bool) {
	if f, ok := columnFates[column]; ok {
		return f.to, f.to != ""
	}
	table, name, _ := strings.Cut(column, ".")
	if to, ok := renamedTables[table]; ok {
		if f, ok := columnFates[to+"."+name]; ok {
			return f.to, f.to != ""
		}
		return to + "." + name, true
	}
	return column, true
}

// dumpValues counts the values of every column of the file at path, as
// the release left it (columnValues).
func dumpValues(t *testing.T, path string) map[string]int {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	return columnValues(t, db)
}

// columnValues counts the values that are not NULL of every column of
// every table of db, by table.column.
func columnValues(t *testing.T, db *sql.DB) map[string]int {
	t.Helper()
	out := map[string]int{}
	for table := range tableRows(t, db) {
		for _, c := range tableColumns(t, db, table) {
			var n int
			// The names are SQLite's own.
			if err := db.QueryRow(`SELECT count("` + c.name + `") FROM "` + table + `"`).Scan(&n); err != nil {
				t.Fatalf("%s.%s: %v", table, c.name, err)
			}
			out[table+"."+c.name] = n
		}
	}
	return out
}

// tableColumn is a column as pragma_table_info describes it.
type tableColumn struct {
	name, typ string
	notNull   bool
	dflt      sql.NullString
	pk        int
}

// tableColumns are the columns of table in db, in their order.
func tableColumns(t *testing.T, db *sql.DB, table string) []tableColumn {
	t.Helper()
	rows, err := db.Query(`SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []tableColumn
	for rows.Next() {
		var c tableColumn
		if err := rows.Scan(&c.name, &c.typ, &c.notNull, &c.dflt, &c.pk); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// schemaOf describes the schema of db as SQLite's pragmas see it, one
// sorted line per fact, so that two stores compare by their structure
// rather than by the text of their statements: each table's columns in
// order with type, NOT NULL, default and key, its foreign keys, its
// indexes with their uniqueness, origin, partiality and columns, and
// every view and trigger by name.
func schemaOf(t *testing.T, db *sql.DB) []string {
	t.Helper()
	var out []string
	query := func(q string, args ...any) [][]any {
		rows, err := db.Query(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		defer rows.Close()
		cols, _ := rows.Columns()
		var all [][]any
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			all = append(all, vals)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return all
	}
	for table := range tableRows(t, db) {
		for i, c := range tableColumns(t, db, table) {
			dflt := "no default"
			if c.dflt.Valid {
				dflt = "default " + c.dflt.String
			}
			out = append(out, fmt.Sprintf("table %s column %d %s %s notnull=%v %s pk=%d", table, i, c.name, c.typ, c.notNull, dflt, c.pk))
		}
		for _, r := range query(`SELECT id, seq, "table", "from", "to", on_update, on_delete, match FROM pragma_foreign_key_list(?)`, table) {
			out = append(out, fmt.Sprintf("table %s foreign key %v", table, r))
		}
		for _, r := range query(`SELECT name, "unique", origin, partial FROM pragma_index_list(?)`, table) {
			name := fmt.Sprint(r[0])
			var cols []string
			for _, c := range query(`SELECT seqno, coalesce(name, cid) FROM pragma_index_info(?) ORDER BY seqno`, name) {
				cols = append(cols, fmt.Sprint(c[1]))
			}
			out = append(out, fmt.Sprintf("table %s index %s unique=%v origin=%v partial=%v on (%s)", table, name, r[1], r[2], r[3], strings.Join(cols, ", ")))
		}
	}
	for _, r := range query(`SELECT type, name, tbl_name FROM sqlite_master WHERE type IN ('view', 'trigger')`) {
		out = append(out, fmt.Sprintf("%v %v on %v", r[0], r[1], r[2]))
	}
	sort.Strings(out)
	return out
}
