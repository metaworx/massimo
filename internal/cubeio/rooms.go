// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// Addressee picks the member a radiogram with the room flag names in its
// header (AP DeviceUnPairingFix, D167 and D169): the room's master where
// it is one of members and a thermostat, else the members' wall
// thermostat, else their first radiator thermostat. leave, the device
// that leaves or joins the room, is never picked. ok is false where no
// member but leave is a thermostat. Every member of the numbered room
// takes such a radiogram, whichever member the header names (T135,
// docs/cube-protocol.md, "Set temperature (0x40) addressing").
func Addressee(master cube.RFAddress, members []cube.DeviceMeta, leave cube.RFAddress) (cube.RFAddress, bool) {
	var wall, radiator cube.RFAddress
	for _, d := range members {
		if d.RFAddress == leave || !d.Type.IsThermostat() {
			continue
		}
		switch {
		case master != 0 && d.RFAddress == master:
			return master, true
		case d.Type == cube.DeviceWallThermostat && wall == 0:
			wall = d.RFAddress
		case d.Type.IsRadiatorThermostat() && radiator == 0:
			radiator = d.RFAddress
		}
	}
	if wall != 0 {
		return wall, true
	}
	return radiator, radiator != 0
}

// RoomAddressee is Addressee for a room as the Cube has it: its members
// are the devices whose C line names the room, the devices in skip left
// out, and its master is the metadata's.
func RoomAddressee(snap *cubelan.Snapshot, room uint8, skip view.Hidden) (cube.RFAddress, bool) {
	var members []cube.DeviceMeta
	for _, rf := range snap.Devices() {
		c := snap.Config(rf)
		if c == nil || skip[rf] || c.RoomID != room || c.Type == cube.DeviceCube {
			continue
		}
		members = append(members, cube.DeviceMeta{Type: c.Type, RFAddress: rf, RoomID: room})
	}
	return Addressee(roomMaster(snap, room), members, 0)
}

// RoomMembers lists the devices the metadata places in room, the
// devices in skip left out.
func RoomMembers(snap *cubelan.Snapshot, room uint8, skip view.Hidden) []cube.DeviceMeta {
	var members []cube.DeviceMeta
	if md := snap.Metadata(); md != nil {
		for _, d := range md.Devices {
			if d.RoomID == room && !skip[d.RFAddress] {
				members = append(members, d)
			}
		}
	}
	return members
}

// roomMaster is the metadata's master of room: the address its M entry
// gives the room, 0 where the metadata does not list the room.
func roomMaster(snap *cubelan.Snapshot, room uint8) cube.RFAddress {
	if md := snap.Metadata(); md != nil {
		for _, r := range md.Rooms {
			if r.ID == room {
				return r.RFAddress
			}
		}
	}
	return 0
}

// flaggedResets is one r: for each of members but except whose error
// flag (byte 5's 0x08) is set (AP DeviceUnPairingFix, D161-B): free, and
// a radiogram the Cube might hold back for a flagged member is not.
// except is the command's target, which the queue resets itself.
func flaggedResets(snap *cubelan.Snapshot, members []cube.DeviceMeta, except cube.RFAddress) []cube.Message {
	var out []cube.Message
	for _, d := range members {
		if d.RFAddress == except {
			continue
		}
		if st, ok := snap.State(d.RFAddress); ok && st.Error {
			out = append(out, cube.CmdResetError(d.RFAddress))
		}
	}
	return out
}

// linkLines links (add) or unlinks rf, of type typ, with its partners
// among members, the devices of room, as the original software did on
// 2026-11-15 and in T134: one radiogram with the room flag naming rf,
// to the member Addressee picks, which every member of the room takes.
// Where no member but rf is a thermostat, its partners are window
// contacts, each addressed directly. Nothing where rf has no partner.
func linkLines(master, rf cube.RFAddress, typ cube.DeviceType, room uint8, members []cube.DeviceMeta, add bool) []cube.Message {
	var partners []cube.DeviceMeta
	for _, d := range members {
		if d.RFAddress != rf && linkPair(d.Type, typ) {
			partners = append(partners, d)
		}
	}
	if len(partners) == 0 {
		return nil
	}
	if a, ok := Addressee(master, members, rf); ok {
		return []cube.Message{cube.CmdLinkPartner(cube.Target{RFAddress: a, RoomID: room, Room: true}, add, rf, typ)}
	}
	var out []cube.Message
	for _, d := range partners {
		out = append(out, cube.CmdLinkPartner(cube.Target{RFAddress: d.RFAddress}, add, rf, typ))
	}
	return out
}
