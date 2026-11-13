// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Rooms (docs/store.md section 5, migration 0017): each Cube's rooms as
// the server knows them, with a uid a client keeps for the room's life
// and the settings of the room's virtual wall thermostat. The server
// keeps the rows in step with the Cube's metadata (SyncRooms): a room it
// names gets a row, one it leaves out is marked deleted, and a room that
// takes a deleted room's number later gets a row and a uid of its own.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
)

// The ways a room's temperature is taken from its radiator thermostats.
const (
	TempMedian = "median"
	TempMean   = "mean"
)

// ErrNoRoom is a room the store has no row for.
var ErrNoRoom = errors.New("store: no such room")

// Room is one live row of rooms.
type Room struct {
	UUID       string
	CubeID     int64
	RoomID     uint8
	TempMethod string // TempMedian or TempMean
	// RFAddress is the virtual wall thermostat's radio address, six hex
	// digits; "" means the room number.
	RFAddress string
}

// rfAddressText is a radio address as the store keeps it.
var rfAddressText = regexp.MustCompile(`^[0-9a-f]{6}$`)

const roomColumns = `uuid, cube_id, room_id, temp_method, rf_address`

func scanRoom(row scanner) (Room, error) {
	var r Room
	var id int64
	if err := row.Scan(&r.UUID, &r.CubeID, &id, &r.TempMethod, &r.RFAddress); err != nil {
		return Room{}, err
	}
	r.RoomID = uint8(id)
	return r, nil
}

// Rooms returns a Cube's live rooms, by room number.
func (s *Store) Rooms(ctx context.Context, cubeID int64) ([]Room, error) {
	return s.rooms(ctx, s.db, cubeID)
}

func (s *Store) rooms(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}, cubeID int64) ([]Room, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+roomColumns+` FROM rooms WHERE cube_id = ? AND deleted_at IS NULL ORDER BY room_id`, cubeID)
	if err != nil {
		return nil, fmt.Errorf("store: rooms: %w", err)
	}
	defer rows.Close()
	var out []Room
	for rows.Next() {
		r, err := scanRoom(rows)
		if err != nil {
			return nil, fmt.Errorf("store: rooms: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SyncRooms brings a Cube's rooms in step with the room numbers its
// metadata names: a number without a live row gets one, with a new uid
// and the default settings; a live row whose number is not named is
// marked deleted. It returns the live rooms afterwards; a store opened
// read-only returns them unchanged.
func (s *Store) SyncRooms(ctx context.Context, cubeID int64, ids []uint8) ([]Room, error) {
	if s.readOnly {
		return s.Rooms(ctx, cubeID)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("store: rooms: %w", err)
	}
	defer tx.Rollback()
	have, err := s.rooms(ctx, tx, cubeID)
	if err != nil {
		return nil, err
	}
	named := map[uint8]bool{}
	for _, id := range ids {
		named[id] = true
	}
	now := s.nowText()
	live := map[uint8]bool{}
	for _, r := range have {
		live[r.RoomID] = true
		if named[r.RoomID] {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE rooms SET deleted_at = ?, updated_at = ?, version = version + 1 WHERE uuid = ?`, now, now, r.UUID); err != nil {
			return nil, fmt.Errorf("store: rooms: %w", err)
		}
	}
	for _, id := range ids {
		if live[id] {
			continue
		}
		live[id] = true
		if _, err := tx.ExecContext(ctx, `INSERT INTO rooms (uuid, cube_id, room_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			newUUID(), cubeID, int(id), now, now); err != nil {
			return nil, fmt.Errorf("store: rooms: %w", err)
		}
	}
	out, err := s.rooms(ctx, tx, cubeID)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit()
}

// RoomSettings is a change of a room's settings; nil fields stay.
type RoomSettings struct {
	TempMethod *string
	RFAddress  *string // "" for the room number
}

// SetRoomSettings changes a live room's settings, for the user by (0:
// none), and returns the row; a room without a live row is ErrNoRoom.
func (s *Store) SetRoomSettings(ctx context.Context, cubeID int64, roomID uint8, set RoomSettings, by int64) (Room, error) {
	if err := s.writable(); err != nil {
		return Room{}, err
	}
	if m := set.TempMethod; m != nil && *m != TempMedian && *m != TempMean {
		return Room{}, fmt.Errorf("store: temp_method %q: median or mean", *m)
	}
	if a := set.RFAddress; a != nil && *a != "" && !rfAddressText.MatchString(*a) {
		return Room{}, fmt.Errorf("store: rf_address %q: six hex digits in lower case", *a)
	}
	var user any
	if by != 0 {
		user = by
	}
	res, err := s.db.ExecContext(ctx, `UPDATE rooms SET temp_method = coalesce(?, temp_method), rf_address = coalesce(?, rf_address),
		updated_at = ?, updated_by = ?, version = version + 1 WHERE cube_id = ? AND room_id = ? AND deleted_at IS NULL`,
		set.TempMethod, set.RFAddress, s.nowText(), user, cubeID, int(roomID))
	if err != nil {
		return Room{}, fmt.Errorf("store: room %d: %w", roomID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Room{}, ErrNoRoom
	}
	r, err := scanRoom(s.db.QueryRowContext(ctx, `SELECT `+roomColumns+` FROM rooms WHERE cube_id = ? AND room_id = ? AND deleted_at IS NULL`, cubeID, int(roomID)))
	if err != nil {
		return Room{}, fmt.Errorf("store: room %d: %w", roomID, err)
	}
	return r, nil
}
