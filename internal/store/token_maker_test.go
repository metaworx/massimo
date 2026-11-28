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

// Migration 0101 is additive (AP KeyLife, Block 2): a key written as a
// binary for 100 writes one, without the column, has no maker; binaries
// for 26 and for 100 open the file; and the down part takes the schema
// back to the one of 100, losing which key made a key and only that.
func TestTokenMakerMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := fixedClock()
	at100 := t.TempDir()
	migrateOnly(t, at100, migrationsUpTo(t, 100), clock)
	dir := copyStore(t, at100)
	upTo101 := migrationsUpTo(t, 101) // 0101 alone, whatever comes after it
	migrateOnly(t, dir, upTo101, clock)
	s, err := Open(dir, Options{Logger: quiet(), Now: clock, migrations: upTo101})
	if err != nil {
		t.Fatal(err)
	}
	anna, err := s.AddUser(ctx, User{Name: "anna", Role: RoleManager}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	maker, _, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "maker"})
	_, heirSecret, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "heir", MadeBy: maker.ID})
	if _, err := s.db.ExecContext(ctx, `INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, origin) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), anna.ID, TokenAPI, HashToken("placeholder-old-binary"), "written as 100 writes it", s.nowText(), s.nodeID); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := s.TokenByHash(ctx, HashToken("placeholder-old-binary")); err != nil || !ok || got.MadeBy != 0 {
		t.Fatalf("a key written as 100 writes it: %+v %v %v", got, ok, err)
	}
	if got, ok, _ := s.TokenByHash(ctx, HashToken(heirSecret)); !ok || got.MadeBy != maker.ID {
		t.Fatalf("the heir: %+v %v", got, ok)
	}
	s.Close()

	for _, reader := range []int{26, 100} {
		old, err := Open(dir, Options{Logger: quiet(), migrations: migrationsUpTo(t, reader), readerVersion: reader})
		if err != nil {
			t.Fatalf("a binary for %d: %v", reader, err)
		}
		if v, mr := old.SchemaVersion(); v != 101 || mr != 24 {
			t.Fatalf("a binary for %d reads version %d, min_reader %d", reader, v, mr)
		}
		old.Close()
	}

	down, err := fs.ReadFile(embeddedMigrations(), "0101_token_maker.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(Path(dir)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	want, got := fileSchema(t, rawDB(t, at100)), fileSchema(t, db)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("after the down part:\n%v\nwant the schema of 100:\n%v", got, want)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM tokens`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("the tokens after the down part: %d %v", n, err)
	}
}
