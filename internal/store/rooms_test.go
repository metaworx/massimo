// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"testing"
)

// The rooms follow the metadata: a named room gets a row and a uid, a
// room left out is marked deleted, and a room that takes its number
// later gets a uid of its own; the settings change per room.
func TestRoomsSync(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	c, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	rooms, err := s.SyncRooms(ctx, c.ID, []uint8{1, 2})
	if err != nil || len(rooms) != 2 || rooms[0].RoomID != 1 || rooms[0].TempMethod != TempMedian || rooms[0].RFAddress != "" {
		t.Fatalf("first sync %+v %v", rooms, err)
	}
	uid1, uid2 := rooms[0].UUID, rooms[1].UUID
	if uid1 == "" || uid1 == uid2 {
		t.Fatalf("uids %q %q", uid1, uid2)
	}
	again, _ := s.SyncRooms(ctx, c.ID, []uint8{2, 1})
	if again[0].UUID != uid1 || again[1].UUID != uid2 {
		t.Fatalf("a second sync changed the uids: %+v", again)
	}
	rooms, _ = s.SyncRooms(ctx, c.ID, []uint8{2})
	if len(rooms) != 1 || rooms[0].UUID != uid2 {
		t.Fatalf("room 1 left out: %+v", rooms)
	}
	rooms, _ = s.SyncRooms(ctx, c.ID, []uint8{1, 2})
	if rooms[0].RoomID != 1 || rooms[0].UUID == uid1 {
		t.Fatalf("a new room 1 took the old uid: %+v", rooms)
	}

	mean, addr := TempMean, "000105"
	r, err := s.SetRoomSettings(ctx, c.ID, 2, RoomSettings{TempMethod: &mean, RFAddress: &addr}, 0)
	if err != nil || r.TempMethod != TempMean || r.RFAddress != "000105" || r.UUID != uid2 {
		t.Fatalf("settings %+v %v", r, err)
	}
	none := ""
	if r, _ = s.SetRoomSettings(ctx, c.ID, 2, RoomSettings{RFAddress: &none}, 0); r.RFAddress != "" || r.TempMethod != TempMean {
		t.Fatalf("address reset %+v", r)
	}
	bad := "mode"
	if _, err := s.SetRoomSettings(ctx, c.ID, 2, RoomSettings{TempMethod: &bad}, 0); err == nil {
		t.Fatal("a bad temp_method taken")
	}
	bad = "0A0105"
	if _, err := s.SetRoomSettings(ctx, c.ID, 2, RoomSettings{RFAddress: &bad}, 0); err == nil {
		t.Fatal("a bad address taken")
	}
	if _, err := s.SetRoomSettings(ctx, c.ID, 9, RoomSettings{TempMethod: &mean}, 0); !errors.Is(err, ErrNoRoom) {
		t.Fatalf("no room 9: %v", err)
	}
}
