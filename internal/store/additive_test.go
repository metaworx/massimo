// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"
)

// additiveMigration checks the additive migration of version v, whose
// file is name.sql, after version prev (docs/store.md section 5,
// "Migrations"): a file at prev, migrated to v alone, with what fill
// writes into it there in plain SQL (nil for nothing), keeps min_reader
// 24 and opens with a binary for prev, and the migration's down part,
// name.down.sql, takes it back to the schema of prev with every row of
// tokens kept. It returns a copy of the filled file at v, taken before
// the down part, for the test's checks with this binary, which Open
// migrates on to the newest version: this binary's reads need every
// later migration's columns, so a test that read a file at v with them
// broke at the next migration (AP KeyLife, Block 3).
func additiveMigration(t *testing.T, prev, v int, name string, fill func(ctx context.Context, db *sql.DB)) string {
	t.Helper()
	ctx := context.Background()
	clock := fixedClock()
	atPrev := t.TempDir()
	migrateOnly(t, atPrev, migrationsUpTo(t, prev), clock)
	dir := copyStore(t, atPrev)
	migrateOnly(t, dir, migrationsUpTo(t, v), clock)
	if fill != nil {
		db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(Path(dir)))
		if err != nil {
			t.Fatal(err)
		}
		fill(ctx, db)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
	filled := copyStore(t, dir)

	old, err := Open(dir, Options{Logger: quiet(), migrations: migrationsUpTo(t, prev), readerVersion: prev})
	if err != nil {
		t.Fatalf("a binary for %d: %v", prev, err)
	}
	if got, mr := old.SchemaVersion(); got != v || mr != 24 {
		t.Fatalf("a binary for %d reads version %d, min_reader %d; want %d, 24", prev, got, mr, v)
	}
	old.Close()

	down, err := fs.ReadFile(embeddedMigrations(), name+".down.sql")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(Path(dir)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	count := func(db *sql.DB) int {
		t.Helper()
		var n int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM tokens`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count(db)
	if _, err := db.ExecContext(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	want, got := fileSchema(t, rawDB(t, atPrev)), fileSchema(t, db)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("after %s.down.sql:\n%v\nwant the schema of %d:\n%v", name, got, prev, want)
	}
	if after := count(db); after != before {
		t.Fatalf("the tokens after %s.down.sql: %d of %d", name, after, before)
	}
	return filled
}
