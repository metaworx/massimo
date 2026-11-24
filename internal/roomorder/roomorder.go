// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package roomorder is the one order in which a member of a room stands
// for its room (AP RoomMajority, D9, D19-A): which of its thermostats is
// dead and passed over (Dead), the order of the others (Of: the master,
// the wall thermostat, the radiator thermostats), and the state that
// speaks for the room by them (State: what more than half of them show,
// else the newest change, else the first of the order). It lies below
// internal/view and internal/cubeio, so that the virtual wall
// thermostat, the radio's addressee, the room's configuration and the
// room checks' re-send take one rule; Massimo's own command to a room
// and the state held for it before a structure change, which only the
// server knows, come first there (internal/server/drift.go,
// referenceState). It reads a cubelan.Snapshot and never writes to the
// Cube.
package roomorder

import (
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Dead reports whether the Cube holds no valid state for rf
// (RoomMajority.D9): no L entry, or one whose valid bit (0x10 of its
// first flags byte) is clear, as for a device not heard since the
// Cube's restart or one whose entry the Cube marks out of date. The
// communication error and the link error do not make a device dead,
// since it answers. view.Silent lists a room's dead members by this
// test, and the room checks leave them out. A nil snapshot holds every
// device alive.
func Dead(snap *cubelan.Snapshot, rf cube.RFAddress) bool {
	if snap == nil {
		return false
	}
	st, ok := snap.State(rf)
	return !ok || !st.Valid
}

// Of lists the thermostats among members in the one order in which a
// member stands for its room (RoomMajority.D9, which replaces
// DeviceUnPairingFix.D167; one function, as its D169-A asked): the
// room's master where it is one of them, then their wall thermostat,
// then their first radiator thermostat and each other one, in the
// order the caller lists the members: the Cube's where they are
// cubeio.ConfigMembers' (room commands, the repairs), the metadata's
// where they are cubeio.RoomMembers' (a move's and a removal's links).
// A dead member (Dead, by snap) is passed over, and leave, the device
// that leaves or joins the room, is never listed. For a room's state,
// what more than half of them show and the newest change come first
// (State); a radiogram with the room flag names the first
// (cubeio.Addressee).
func Of(snap *cubelan.Snapshot, master cube.RFAddress, members []cube.DeviceMeta, leave cube.RFAddress) []cube.RFAddress {
	var order, walls, radiators []cube.RFAddress
	for _, d := range members {
		if d.RFAddress == leave || !d.Type.IsThermostat() || Dead(snap, d.RFAddress) {
			continue
		}
		switch {
		case master != 0 && d.RFAddress == master:
			order = append(order, d.RFAddress)
		case d.Type == cube.DeviceWallThermostat:
			walls = append(walls, d.RFAddress)
		default:
			radiators = append(radiators, d.RFAddress)
		}
	}
	return append(append(order, walls...), radiators...)
}
