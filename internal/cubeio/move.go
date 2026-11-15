// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// Linkable reports whether a device type takes part in room links.
func Linkable(t cube.DeviceType) bool {
	return t.IsThermostat() || t == cube.DeviceShutterContact
}

// linkPair reports whether the original software links two room members:
// every pair except two shutter contacts (observed: a contact moved
// into a room with a thermostat and a contact was linked with the
// thermostat only).
func linkPair(a, b cube.DeviceType) bool {
	return Linkable(a) && Linkable(b) && (a.IsThermostat() || b.IsThermostat())
}

// MoveCommands builds the two commands that move a device to another
// room as the original software moved one in its capture of 2026-11-04
// (AP DeviceUnPairingFix v1.3, Block 2, D159-A, D170-A): out, the
// removal's unlink half for the old room (unlinkFromRoom) with a 0x23
// first (D160-A) and the device's own side; in, the placement's link
// half for the new room (linkIntoRoom). Each half goes after a wakeup
// of its room and carries its room's number. A device in no
// room has no partner to leave: out is the 0x23 alone, without a
// wakeup. The metadata write that records the new room is not
// included. A device in skip, a hidden one, is no partner: every write
// keeps away from it.
func MoveCommands(snap *cubelan.Snapshot, rf cube.RFAddress, typ cube.DeviceType, oldRoom, newRoom uint8, newMembers []cube.DeviceMeta, skip view.Hidden) (out, in engine.Command) {
	outLines := []cube.Message{cube.CmdSetGroup(rf, oldRoom, true)}
	if oldRoom != 0 {
		outLines = unlinkFromRoom(snap, rf, typ, oldRoom, skip, true)
	}
	out = engine.Command{Key: "move-out:" + rf.String(), Target: rf, RoomID: oldRoom, Wakeup: oldRoom != 0, Lines: outLines}
	in = engine.Command{Key: "move-in:" + rf.String(), Target: rf, RoomID: newRoom, Wakeup: true,
		Lines: linkIntoRoom(snap, rf, typ, newRoom, newMembers, skip)}
	return out, in
}

// unlinkFromRoom is the unlink half of a removal and of a move: one r:
// for each other member of room whose error flag is set (D161-B), then,
// for a move, 0x23 for the device, then the 0x21s that unlink it from
// its partners (linkLines), for a move on its own side too. The members
// are the metadata's, the devices in skip left out.
func unlinkFromRoom(snap *cubelan.Snapshot, rf cube.RFAddress, typ cube.DeviceType, room uint8, skip view.Hidden, move bool) []cube.Message {
	members := RoomMembers(snap, room, skip)
	lines := flaggedResets(snap, members, rf)
	if move {
		lines = append(lines, cube.CmdSetGroup(rf, room, true))
	}
	return append(lines, linkLines(roomMaster(snap, room), rf, typ, room, members, false, move)...)
}

// linkIntoRoom is the link half of a placement and of a move: one r:
// for each of members whose error flag is set, the 0x20s that link the
// device with its partners among them, on both sides (linkLines), then
// 0x22, which gives the device the room's group. members are the
// devices of room the caller names, the devices in skip left out.
func linkIntoRoom(snap *cubelan.Snapshot, rf cube.RFAddress, typ cube.DeviceType, room uint8, members []cube.DeviceMeta, skip view.Hidden) []cube.Message {
	var in []cube.DeviceMeta
	for _, d := range members {
		if d.RFAddress != rf && !skip[d.RFAddress] {
			in = append(in, d)
		}
	}
	lines := flaggedResets(snap, in, rf)
	lines = append(lines, linkLines(roomMaster(snap, room), rf, typ, room, in, true, true)...)
	return append(lines, cube.CmdSetGroup(rf, room, false))
}

// RoomRadiator returns the configuration of a radiator thermostat in the
// room (other than rf), or the original software's defaults: the values
// a wall thermostat does not store are taken from it when writing one.
func RoomRadiator(snap *cubelan.Snapshot, room uint8, rf cube.RFAddress) cube.ThermostatConfig {
	for _, other := range snap.Devices() {
		if other == rf {
			continue
		}
		if c := snap.Config(other); c != nil && c.Thermostat != nil && c.RoomID == room {
			return *c.Thermostat
		}
	}
	return cube.ThermostatConfig{WindowOpenMins: 15, DecalcDay: cube.Saturday, DecalcHour: 12, MaxValvePct: 100, ValveOffsetPct: 0}
}
