// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/roomorder"
	"go.metaworx.ch/massimo/internal/view"
)

// Addressee picks the member a radiogram with the room flag names in its
// header: the first of the room's one order (roomorder.Of: the master,
// the wall thermostat, the radiator thermostats in members' order), the
// dead passed over; where every thermostat among members but leave is
// dead, the first of the same order with the dead in it, since such a
// radiogram must name one (RoomMajority.D9). ok is false where no
// member but leave is a thermostat. Every member of the numbered room
// takes such a radiogram, whichever member the header names (T135,
// docs/cube-protocol.md, "Set temperature (0x40) addressing"). Passing
// over a dead member has not been tried on hardware.
func Addressee(snap *cubelan.Snapshot, master cube.RFAddress, members []cube.DeviceMeta, leave cube.RFAddress) (cube.RFAddress, bool) {
	if order := roomorder.Of(snap, master, members, leave); len(order) > 0 {
		return order[0], true
	}
	if order := roomorder.Of(nil, master, members, leave); len(order) > 0 {
		return order[0], true
	}
	return 0, false
}

// RoomAddressee is Addressee for a room as the Cube has it: its members
// are the devices whose C line names the room, the devices in skip left
// out, and its master is the metadata's.
func RoomAddressee(snap *cubelan.Snapshot, room uint8, skip view.Hidden) (cube.RFAddress, bool) {
	return Addressee(snap, RoomMaster(snap, room), ConfigMembers(snap, room, skip), 0)
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

// PartnerCount is how many of members rf, of type typ, is linked with
// in their room: the thermostats, and a thermostat's window contacts.
func PartnerCount(members []cube.DeviceMeta, rf cube.RFAddress, typ cube.DeviceType) int {
	n := 0
	for _, d := range members {
		if d.RFAddress != rf && linkPair(d.Type, typ) {
			n++
		}
	}
	return n
}

// RoomMaster is the metadata's master of room: the address its M entry
// gives the room, 0 where the metadata does not list the room.
func RoomMaster(snap *cubelan.Snapshot, room uint8) cube.RFAddress {
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
// among members, the devices of room. The rest of the room gets one
// radiogram with the room flag naming rf, to the member Addressee picks
// (the dead passed over, by snap), which every member of the room
// takes; where no member but rf is a thermostat, its partners are
// window contacts, each addressed directly. With both, as the original
// software moved and placed devices in its captures of 2026-11-04
// (docs/cube-protocol.md, "Pairing a device, as the original software
// does it"; AP DeviceUnPairingFix v1.3, D170-A), each window contact
// among the partners gets one of its own as well, and rf one for each
// partner, so that its own partners follow it. A removal sends neither
// (both false): t: resets the device. Nothing where rf has no partner.
func linkLines(snap *cubelan.Snapshot, master, rf cube.RFAddress, typ cube.DeviceType, room uint8, members []cube.DeviceMeta, add, both bool) []cube.Message {
	var partners []cube.DeviceMeta
	for _, d := range members {
		if d.RFAddress != rf && linkPair(d.Type, typ) {
			partners = append(partners, d)
		}
	}
	if len(partners) == 0 {
		return nil
	}
	var out []cube.Message
	a, ok := Addressee(snap, master, members, rf)
	if ok {
		out = append(out, cube.CmdLinkPartner(cube.Target{RFAddress: a, RoomID: room, Room: true}, add, rf, typ))
	}
	for _, d := range partners {
		if !ok || (both && !d.Type.IsThermostat()) {
			out = append(out, cube.CmdLinkPartner(cube.Target{RFAddress: d.RFAddress}, add, rf, typ))
		}
	}
	if both {
		for _, d := range partners {
			out = append(out, cube.CmdLinkPartner(cube.Target{RFAddress: rf}, add, d.RFAddress, d.Type))
		}
	}
	return out
}
