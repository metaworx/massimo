// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Linkable reports whether a device type takes part in room links.
func Linkable(t cube.DeviceType) bool {
	return t.IsRadiatorThermostat() || t == cube.DeviceWallThermostat || t == cube.DeviceShutterContact
}

// isThermostat reports whether t is a radiator or wall thermostat.
func isThermostat(t cube.DeviceType) bool {
	return t.IsRadiatorThermostat() || t == cube.DeviceWallThermostat
}

// linkPair reports whether the original software links two room members:
// every pair except two shutter contacts (observed: a contact moved
// into a room with a thermostat and a contact was linked with the
// thermostat only).
func linkPair(a, b cube.DeviceType) bool {
	return Linkable(a) && Linkable(b) && (isThermostat(a) || isThermostat(b))
}

// linkTarget addresses a link radiogram to a room member the way the
// original software does: radiator thermostats with the room flag and
// their room, every other device directly. The device being moved is
// always addressed directly, as its group is being changed.
func linkTarget(rf cube.RFAddress, typ cube.DeviceType, room uint8) cube.Target {
	if typ.IsRadiatorThermostat() {
		return cube.Target{RFAddress: rf, RoomID: room, Room: true}
	}
	return cube.Target{RFAddress: rf}
}

// MoveCommands builds the two commands that move a device to another
// room the way the original software does: unlink it from every partner
// of the old room (as the Cube lists it now) in both directions and
// remove its group, then link it with every partner among newMembers in
// both directions and set the new group, each half after a wakeup of
// the room concerned. The metadata write that records the new room is
// not included.
func MoveCommands(snap *cubelan.Snapshot, rf cube.RFAddress, typ cube.DeviceType, oldRoom, newRoom uint8, newMembers []cube.DeviceMeta) (out, in engine.Command) {
	var outLines []cube.Message
	if md := snap.Metadata(); md != nil {
		for _, d := range md.Devices {
			if d.RFAddress == rf || d.RoomID != oldRoom || !linkPair(d.Type, typ) {
				continue
			}
			outLines = append(outLines, cube.CmdLinkPartner(linkTarget(d.RFAddress, d.Type, oldRoom), false, rf, typ))
			outLines = append(outLines, cube.CmdLinkPartner(cube.Target{RFAddress: rf}, false, d.RFAddress, d.Type))
		}
	}
	outLines = append(outLines, cube.CmdSetGroup(rf, oldRoom, true))
	out = engine.Command{Key: "move-out:" + rf.String(), Target: rf, RoomID: oldRoom, Wakeup: oldRoom != 0, Lines: outLines}

	var inLines []cube.Message
	for _, d := range newMembers {
		if d.RFAddress == rf || !linkPair(d.Type, typ) {
			continue
		}
		inLines = append(inLines, cube.CmdLinkPartner(linkTarget(d.RFAddress, d.Type, newRoom), true, rf, typ))
		inLines = append(inLines, cube.CmdLinkPartner(cube.Target{RFAddress: rf}, true, d.RFAddress, d.Type))
	}
	inLines = append(inLines, cube.CmdSetGroup(rf, newRoom, false))
	in = engine.Command{Key: "move-in:" + rf.String(), Target: rf, RoomID: newRoom, Wakeup: true, Lines: inLines}
	return out, in
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
