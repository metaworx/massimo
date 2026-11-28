// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"testing"
)

// Migration 0101 is additive (AP KeyLife, Block 2): a key written as a
// binary for 100 writes one, without the column, has no maker; binaries
// for 26 and 100 open the file; and the down part takes the schema back
// to the one of 100, losing which key made a key and only that.
func TestTokenMakerMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := fixedClock()
	at := formatTime(clock())
	dir := additiveMigration(t, 100, 101, "0101_token_maker", func(ctx context.Context, db *sql.DB) {
		exec := func(q string, args ...any) {
			t.Helper()
			if _, err := db.ExecContext(ctx, q, args...); err != nil {
				t.Fatal(err)
			}
		}
		exec(`INSERT INTO users (uuid, name, role_id, created_at, updated_at) SELECT ?, 'anna', id, ?, ? FROM roles WHERE key = ?`, newUUID(), at, at, RoleManager)
		key := `INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, origin, made_by) SELECT ?, id, 'api', ?, ?, ?, '', ? FROM users WHERE name = 'anna'`
		exec(key, newUUID(), HashToken("placeholder-maker"), "maker", at, nil)
		exec(key, newUUID(), HashToken("placeholder-heir"), "heir", at, nil)
		exec(`UPDATE tokens SET made_by = (SELECT id FROM tokens WHERE label = 'maker') WHERE label = 'heir'`)
		exec(`INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, origin) SELECT ?, id, 'api', ?, ?, ?, '' FROM users WHERE name = 'anna'`,
			newUUID(), HashToken("placeholder-old-binary"), "written as 100 writes it", at)
	}, 26)

	s, err := Open(dir, Options{Logger: quiet(), Now: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	maker, ok, err := s.TokenByHash(ctx, HashToken("placeholder-maker"))
	if err != nil || !ok || maker.MadeBy != 0 {
		t.Fatalf("the maker: %+v %v %v", maker, ok, err)
	}
	if got, ok, err := s.TokenByHash(ctx, HashToken("placeholder-old-binary")); err != nil || !ok || got.MadeBy != 0 {
		t.Fatalf("a key written as 100 writes it: %+v %v %v", got, ok, err)
	}
	if got, ok, _ := s.TokenByHash(ctx, HashToken("placeholder-heir")); !ok || got.MadeBy != maker.ID {
		t.Fatalf("the heir: %+v %v", got, ok)
	}
}
