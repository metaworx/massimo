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

// Migration 0100 is additive (AP KeyLife, Block 2): a key written as a
// binary for 26 writes one, without the column, has no maker; a binary
// for 26 opens the file; and the down part takes the schema back to the
// one of 26, losing which key made a key and only that.
func TestTokenMakerMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := fixedClock()
	at26 := t.TempDir()
	migrateOnly(t, at26, migrationsUpTo(t, 26), clock)
	dir := copyStore(t, at26)
	upTo100 := migrationsUpTo(t, 100) // 0100 alone, whatever comes after it
	migrateOnly(t, dir, upTo100, clock)
	s, err := Open(dir, Options{Logger: quiet(), Now: clock, migrations: upTo100})
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
		newUUID(), anna.ID, TokenAPI, HashToken("placeholder-old-binary"), "written as 26 writes it", s.nowText(), s.nodeID); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := s.TokenByHash(ctx, HashToken("placeholder-old-binary")); err != nil || !ok || got.MadeBy != 0 {
		t.Fatalf("a key written as 26 writes it: %+v %v %v", got, ok, err)
	}
	if got, ok, _ := s.TokenByHash(ctx, HashToken(heirSecret)); !ok || got.MadeBy != maker.ID {
		t.Fatalf("the heir: %+v %v", got, ok)
	}
	s.Close()

	old, err := Open(dir, Options{Logger: quiet(), migrations: migrationsUpTo(t, 26), readerVersion: 26})
	if err != nil {
		t.Fatalf("a binary for 26: %v", err)
	}
	if v, mr := old.SchemaVersion(); v != 100 || mr != 24 {
		t.Fatalf("a binary for 26 reads version %d, min_reader %d", v, mr)
	}
	old.Close()

	down, err := fs.ReadFile(embeddedMigrations(), "0100_token_maker.down.sql")
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
	want, got := fileSchema(t, rawDB(t, at26)), fileSchema(t, db)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("after the down part:\n%v\nwant the schema of 26:\n%v", got, want)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM tokens`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("the tokens after the down part: %d %v", n, err)
	}
}
