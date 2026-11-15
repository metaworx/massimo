// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package view

import (
	"fmt"
	"time"
)

// Kinds of Mismatch (AP RadioHealth v1.2, Block 7): a device's room for
// the Cube is its C line's room byte, not the metadata's, and the members
// of a room share its week programme and temperatures.
const (
	// MismatchRoom is a device whose C line names another room than the
	// metadata.
	MismatchRoom = "room"
	// MismatchUnlisted is a C line in a room for a device the metadata
	// does not list, which can block the room's commands.
	MismatchUnlisted = "unlisted"
	// MismatchNoConfig is a device the metadata lists and the Cube has
	// no C line for.
	MismatchNoConfig = "no_config"
	// MismatchType is a device whose C line gives another type than the
	// metadata.
	MismatchType = "type"
	// MismatchMaster is a room whose master in the metadata, the address
	// of its M entry, is no device of the Cube (Why "no_device"), a
	// device of another room ("other_room") or no thermostat
	// ("not_thermostat").
	MismatchMaster = "master"
	// MismatchProgramme and MismatchTemperatures are a member of a room
	// whose week programme, or whose comfort, eco, maximum, minimum or
	// window-open temperature, differs from the room's.
	MismatchProgramme    = "programme"
	MismatchTemperatures = "temperatures"
)

// Mismatch is a difference the server finds between a Cube's C lines and
// its metadata, or among the members of a room. Device is the device
// concerned, a room's master for MismatchMaster; Room the room
// concerned, the metadata's where it lists the device; CubeRoom the room
// of the device's C line and ListRoom the metadata's, each with its name
// where the metadata has the room; CubeType and ListType the two types;
// Since when the server first found it.
type Mismatch struct {
	Finding      string    `json:"finding"`
	Device       string    `json:"device,omitempty"`
	Name         string    `json:"name,omitempty"` // the device's name in the metadata
	Room         uint8     `json:"room"`
	RoomName     string    `json:"room_name,omitempty"`
	CubeRoom     *uint8    `json:"cube_room,omitempty"`
	CubeRoomName string    `json:"cube_room_name,omitempty"`
	ListRoom     *uint8    `json:"list_room,omitempty"`
	ListRoomName string    `json:"list_room_name,omitempty"`
	CubeType     string    `json:"cube_type,omitempty"`
	ListType     string    `json:"list_type,omitempty"`
	Why          string    `json:"why,omitempty"`
	Since        time.Time `json:"since"`
}

// Key names the finding: two mismatches with the same key are one, found
// again; its names and times are not part of it.
func (m Mismatch) Key() string {
	room := func(r *uint8) string {
		if r == nil {
			return "-"
		}
		return fmt.Sprint(*r)
	}
	return fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s|%s", m.Finding, m.Device, m.Room, room(m.CubeRoom), room(m.ListRoom), m.CubeType, m.ListType, m.Why)
}

// Concerns reports whether the mismatch concerns room: its own room, or
// the room of either side.
func (m Mismatch) Concerns(room uint8) bool {
	return m.Room == room || m.CubeRoom != nil && *m.CubeRoom == room || m.ListRoom != nil && *m.ListRoom == room
}
