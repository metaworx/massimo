// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// rowOrigin reads updated_by (0: NULL) and origin of a table's row.
func rowOrigin(t *testing.T, s *Store, table, where string, args ...any) (int64, string) {
	t.Helper()
	var by sql.NullInt64
	var origin string
	// table and where are the tests' own text.
	if err := s.db.QueryRow(`SELECT updated_by, origin FROM `+table+` WHERE `+where, args...).Scan(&by, &origin); err != nil {
		t.Fatalf("%s: %v", table, err)
	}
	return by.Int64, origin
}

// A change of a Cube, a setting or a room says who made it, the server
// itself as NULL, and every write of a room or a token names this node
// as its origin (AP StoreRaise v1.2, Block 2).
func TestWritersFillUpdatedBy(t *testing.T) {
	ctx := context.Background()
	s := open(t, t.TempDir())
	anna, err := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	if err != nil {
		t.Fatal(err)
	}
	node := s.NodeID()
	check := func(what, table, where string, wantBy int64, args ...any) {
		t.Helper()
		if by, origin := rowOrigin(t, s, table, where, args...); by != wantBy || origin != node {
			t.Errorf("%s: updated_by %d origin %q, want %d %q", what, by, origin, wantBy, node)
		}
	}

	cube, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateCube(ctx, cube.Serial, func(c *Cube) error { c.Name = "Hall"; return nil }, anna.ID); err != nil {
		t.Fatal(err)
	}
	check("cube by anna", "cubes", "id = ?", anna.ID, cube.ID)
	if _, err := s.UpdateCube(ctx, cube.Serial, func(c *Cube) error { c.Name = "Hall 2"; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	check("cube by the server", "cubes", "id = ?", 0, cube.ID)

	if err := s.SetSettingBy(ctx, StaleAfterKey, "12h", anna.ID); err != nil {
		t.Fatal(err)
	}
	check("setting by anna", "settings", "key = ?", anna.ID, StaleAfterKey)
	if err := s.SetSetting(ctx, StaleAfterKey, "24h"); err != nil {
		t.Fatal(err)
	}
	check("setting by the server", "settings", "key = ?", 0, StaleAfterKey)
	if err := s.DeleteSettingBy(ctx, StaleAfterKey, anna.ID); err != nil {
		t.Fatal(err)
	}
	check("setting removed by anna", "settings", "key = ?", anna.ID, StaleAfterKey)
	if err := s.SetSettingBy(ctx, StaleAfterKey, "36h", anna.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSetting(ctx, StaleAfterKey); err != nil {
		t.Fatal(err)
	}
	check("setting removed by the server", "settings", "key = ?", 0, StaleAfterKey)

	// The rooms: made and removed by the Cube's metadata, the server's;
	// a room's settings by anna. A row another node wrote becomes this
	// node's on every write.
	if _, err := s.SyncRooms(ctx, cube.ID, []uint8{1, 2}); err != nil {
		t.Fatal(err)
	}
	check("room made", "rooms", "cube_id = ? AND room_id = 1", 0, cube.ID)
	if _, err := s.db.Exec(`UPDATE rooms SET origin = 'other' WHERE cube_id = ?`, cube.ID); err != nil {
		t.Fatal(err)
	}
	method := TempMean
	if _, err := s.SetRoomSettings(ctx, cube.ID, 1, RoomSettings{TempMethod: &method}, anna.ID); err != nil {
		t.Fatal(err)
	}
	check("room by anna", "rooms", "cube_id = ? AND room_id = 1", anna.ID, cube.ID)
	if _, err := s.db.Exec(`UPDATE rooms SET origin = 'other' WHERE cube_id = ?`, cube.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncRooms(ctx, cube.ID, []uint8{2}); err != nil {
		t.Fatal(err)
	}
	check("room removed", "rooms", "cube_id = ? AND room_id = 1", 0, cube.ID)

	// The tokens: every revocation names this node.
	revoke := func(what string, do func(Token) error) {
		t.Helper()
		tok, _, err := s.AddToken(ctx, anna.ID, TokenSession, what, time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(`UPDATE tokens SET origin = 'other' WHERE id = ?`, tok.ID); err != nil {
			t.Fatal(err)
		}
		if err := do(tok); err != nil {
			t.Fatal(err)
		}
		var origin string
		s.db.QueryRow(`SELECT origin FROM tokens WHERE id = ?`, tok.ID).Scan(&origin)
		if origin != node {
			t.Errorf("%s: token origin %q, want %q", what, origin, node)
		}
	}
	revoke("revoked", func(tok Token) error { return s.RevokeToken(ctx, tok.ID) })
	revoke("sessions", func(Token) error { return s.RevokeSessions(ctx, anna.ID) })
	revoke("user removed", func(Token) error { return s.DeleteUser(ctx, "anna", 0) })
}
