// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"testing"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

const (
	rad1    cube.RFAddress = 0x0a0101
	rad2    cube.RFAddress = 0x0a0102
	wall    cube.RFAddress = 0x0a0103
	contact cube.RFAddress = 0x0a0104
	other   cube.RFAddress = 0x0a0105
)

func meta(rf cube.RFAddress, typ cube.DeviceType) cube.DeviceMeta {
	return cube.DeviceMeta{Type: typ, RFAddress: rf, RoomID: 1}
}

// The addressee in the owner's order (D167): the master where it is a
// thermostat of the room, else the wall thermostat, else the first
// radiator thermostat; never the device that leaves or joins.
func TestAddressee(t *testing.T) {
	room := []cube.DeviceMeta{meta(rad1, cube.DeviceRadiatorThermostat), meta(contact, cube.DeviceShutterContact),
		meta(rad2, cube.DeviceRadiatorThermostat), meta(wall, cube.DeviceWallThermostat)}
	for _, c := range []struct {
		name          string
		master, leave cube.RFAddress
		members       []cube.DeviceMeta
		want          cube.RFAddress
		ok            bool
	}{
		{"the master, a radiator thermostat of the room", rad2, 0, room, rad2, true},
		{"a master in another room: the wall thermostat", other, 0, room, wall, true},
		{"the master leaving: the wall thermostat", rad2, rad2, room, wall, true},
		{"the wall thermostat leaving: the first radiator thermostat", wall, wall, room, rad1, true},
		{"a contact as master is no thermostat", contact, 0, room, wall, true},
		{"no master in the metadata", 0, 0, room, wall, true},
		{"no wall thermostat: the first radiator thermostat", other, 0, room[:2], rad1, true},
		{"contacts only", 0, 0, []cube.DeviceMeta{meta(contact, cube.DeviceShutterContact)}, 0, false},
		{"the only thermostat leaving", rad1, rad1, room[:2], 0, false},
	} {
		if got, ok := Addressee(c.master, c.members, c.leave); got != c.want || ok != c.ok {
			t.Errorf("%s: %s %v, want %s %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

// One radiogram with the room flag to the addressee names the device;
// a room of window contacts without another thermostat gets them one by
// one; a move or a placement adds the contacts' own and the device's
// own; a device without partners gets nothing.
func TestLinkLines(t *testing.T) {
	room := []cube.DeviceMeta{meta(rad1, cube.DeviceRadiatorThermostat), meta(wall, cube.DeviceWallThermostat),
		meta(contact, cube.DeviceShutterContact)}
	got := linkLines(rad1, rad2, cube.DeviceRadiatorThermostat, 1, room, false, false)
	want := cube.CmdLinkPartner(cube.Target{RFAddress: rad1, RoomID: 1, Room: true}, false, rad2, cube.DeviceRadiatorThermostat)
	if len(got) != 1 || got[0].String() != want.String() {
		t.Fatalf("unlink from a room of three: %v, want %s", got, want)
	}
	got = linkLines(0, rad1, cube.DeviceRadiatorThermostat, 1, []cube.DeviceMeta{meta(contact, cube.DeviceShutterContact),
		meta(other, cube.DeviceShutterContact), meta(rad1, cube.DeviceRadiatorThermostat)}, true, false)
	if len(got) != 2 || got[0].String() != cube.CmdLinkPartner(cube.Target{RFAddress: contact}, true, rad1, cube.DeviceRadiatorThermostat).String() ||
		got[1].String() != cube.CmdLinkPartner(cube.Target{RFAddress: other}, true, rad1, cube.DeviceRadiatorThermostat).String() {
		t.Fatalf("link with two contacts, no other thermostat: %v", got)
	}
	// A move or a placement (both): the room's radiogram, the contact's
	// own, then the device's own for each partner, as the original's
	// captures of 2026-11-04.
	got = linkLines(rad1, rad2, cube.DeviceRadiatorThermostat, 1, room, true, true)
	wantBoth := []cube.Message{
		cube.CmdLinkPartner(cube.Target{RFAddress: rad1, RoomID: 1, Room: true}, true, rad2, cube.DeviceRadiatorThermostat),
		cube.CmdLinkPartner(cube.Target{RFAddress: contact}, true, rad2, cube.DeviceRadiatorThermostat),
		cube.CmdLinkPartner(cube.Target{RFAddress: rad2}, true, rad1, cube.DeviceRadiatorThermostat),
		cube.CmdLinkPartner(cube.Target{RFAddress: rad2}, true, wall, cube.DeviceWallThermostat),
		cube.CmdLinkPartner(cube.Target{RFAddress: rad2}, true, contact, cube.DeviceShutterContact),
	}
	if len(got) != len(wantBoth) {
		t.Fatalf("link into a room of three, both sides: %v", got)
	}
	for i := range got {
		if got[i].String() != wantBoth[i].String() {
			t.Fatalf("line %d: %s, want %s", i, got[i], wantBoth[i])
		}
	}
	if got := linkLines(0, contact, cube.DeviceShutterContact, 1, []cube.DeviceMeta{meta(other, cube.DeviceShutterContact)}, false, true); got != nil {
		t.Fatalf("two contacts are no partners: %v", got)
	}
	if got := linkLines(rad1, rad1, cube.DeviceRadiatorThermostat, 1, room[:1], false, true); got != nil {
		t.Fatalf("a device alone in its room: %v", got)
	}
}
