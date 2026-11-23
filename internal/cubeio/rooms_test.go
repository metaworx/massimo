// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"encoding/base64"
	"slices"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
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

// The addressee in the owner's order (RoomMajority.D9), none dead: the
// master where it is a thermostat of the room, else the wall
// thermostat, else the first radiator thermostat; never the device that
// leaves or joins.
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
		if got, ok := Addressee(nil, c.master, c.members, c.leave); got != c.want || ok != c.ok {
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
	got := linkLines(nil, rad1, rad2, cube.DeviceRadiatorThermostat, 1, room, false, false)
	want := cube.CmdLinkPartner(cube.Target{RFAddress: rad1, RoomID: 1, Room: true}, false, rad2, cube.DeviceRadiatorThermostat)
	if len(got) != 1 || got[0].String() != want.String() {
		t.Fatalf("unlink from a room of three: %v, want %s", got, want)
	}
	got = linkLines(nil, 0, rad1, cube.DeviceRadiatorThermostat, 1, []cube.DeviceMeta{meta(contact, cube.DeviceShutterContact),
		meta(other, cube.DeviceShutterContact), meta(rad1, cube.DeviceRadiatorThermostat)}, true, false)
	if len(got) != 2 || got[0].String() != cube.CmdLinkPartner(cube.Target{RFAddress: contact}, true, rad1, cube.DeviceRadiatorThermostat).String() ||
		got[1].String() != cube.CmdLinkPartner(cube.Target{RFAddress: other}, true, rad1, cube.DeviceRadiatorThermostat).String() {
		t.Fatalf("link with two contacts, no other thermostat: %v", got)
	}
	// A move or a placement (both): the room's radiogram, the contact's
	// own, then the device's own for each partner, as the original's
	// captures of 2026-11-04.
	got = linkLines(nil, rad1, rad2, cube.DeviceRadiatorThermostat, 1, room, true, true)
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
	if got := linkLines(nil, 0, contact, cube.DeviceShutterContact, 1, []cube.DeviceMeta{meta(other, cube.DeviceShutterContact)}, false, true); got != nil {
		t.Fatalf("two contacts are no partners: %v", got)
	}
	if got := linkLines(nil, rad1, rad1, cube.DeviceRadiatorThermostat, 1, room[:1], false, true); got != nil {
		t.Fatalf("a device alone in its room: %v", got)
	}
}

// roomsSnapshot is the snapshot of the rooms fixture
// (fakecube.RoomsFixture: room 1 its wall thermostat 0a0201, the room's
// master, and the radiator thermostats 0a0202 and 0a0203; room 2 the
// radiator thermostats 0a0204 to 0a0206; room 3 the window contact
// 0a0207), the devices in dead without valid state: their L entries'
// valid bit (0x10 of the first flags byte) clear, as for a device not
// heard since the Cube's restart.
func roomsSnapshot(t *testing.T, dead ...cube.RFAddress) *cubelan.Snapshot {
	t.Helper()
	greeting, err := fakecube.RoomsFixture()
	if err != nil {
		t.Fatal(err)
	}
	for i, m := range greeting {
		if m.Type != 'L' {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(m.Data)
		if err != nil {
			t.Fatal(err)
		}
		for j := 0; j < len(raw); j += int(raw[j]) + 1 {
			if slices.Contains(dead, cube.RFAddressFromBytes(raw[j+1:j+4])) {
				raw[j+5] &^= 0x10
			}
		}
		greeting[i].Data = base64.StdEncoding.EncodeToString(raw)
	}
	snap, err := cubelan.SnapshotOf(greeting, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, rf := range dead {
		if !Dead(snap, rf) {
			t.Fatalf("%s not dead in the snapshot", rf)
		}
	}
	return snap
}

// The one order of a room (RoomMajority.D9): the master where it is a
// thermostat of the room, the wall thermostat, the radiator thermostats
// in the Cube's order, a dead one passed over at every step and the
// device that leaves never listed; the addressee is its first, and
// where every thermostat but the one leaving is dead, the first of the
// same order with the dead in it. A window contact as the master is no
// thermostat.
func TestRoomOrder(t *testing.T) {
	const (
		wallT cube.RFAddress = 0x0a0201
		radA  cube.RFAddress = 0x0a0202
		radB  cube.RFAddress = 0x0a0203
		win   cube.RFAddress = 0x0a0207
	)
	// The room's members as the Cube may list them: a window contact
	// among them, or the wall thermostat after the radiator thermostats.
	withContact := func(snap *cubelan.Snapshot) []cube.DeviceMeta {
		return append(ConfigMembers(snap, 1, nil), cube.DeviceMeta{Type: cube.DeviceShutterContact, RFAddress: win, RoomID: 1})
	}
	wallLast := func(snap *cubelan.Snapshot) []cube.DeviceMeta {
		ms := ConfigMembers(snap, 1, nil)
		return append(ms[1:], ms[0])
	}
	for _, c := range []struct {
		name          string
		dead          []cube.RFAddress
		master, leave cube.RFAddress
		members       func(*cubelan.Snapshot) []cube.DeviceMeta // nil for room 1's by their C lines
		order         []cube.RFAddress
		addressee     cube.RFAddress
	}{
		{"the wall thermostat as master", nil, wallT, 0, nil, []cube.RFAddress{wallT, radA, radB}, wallT},
		{"a radiator thermostat as master", nil, radB, 0, nil, []cube.RFAddress{radB, wallT, radA}, radB},
		{"no master, the wall thermostat listed last", nil, 0, 0, wallLast, []cube.RFAddress{wallT, radA, radB}, wallT},
		{"a dead master", []cube.RFAddress{radB}, radB, 0, nil, []cube.RFAddress{wallT, radA}, wallT},
		{"a dead wall thermostat, no master", []cube.RFAddress{wallT}, 0, 0, nil, []cube.RFAddress{radA, radB}, radA},
		{"a dead wall thermostat as master", []cube.RFAddress{wallT}, wallT, 0, nil, []cube.RFAddress{radA, radB}, radA},
		{"a dead first radiator thermostat", []cube.RFAddress{wallT, radA}, wallT, 0, nil, []cube.RFAddress{radB}, radB},
		{"every thermostat dead: the order with the dead", []cube.RFAddress{wallT, radA, radB}, radB, 0, nil, nil, radB},
		{"every one but the leaving dead", []cube.RFAddress{wallT, radA}, radB, radB, nil, nil, wallT},
		{"the master leaving", nil, radB, radB, nil, []cube.RFAddress{wallT, radA}, wallT},
		{"a window contact as master", nil, win, 0, withContact, []cube.RFAddress{wallT, radA, radB}, wallT},
		{"a window contact as master, the wall thermostat dead", []cube.RFAddress{wallT}, win, 0, withContact, []cube.RFAddress{radA, radB}, radA},
	} {
		snap := roomsSnapshot(t, c.dead...)
		members := ConfigMembers(snap, 1, nil)
		if c.members != nil {
			members = c.members(snap)
		}
		if got := RoomOrder(snap, c.master, members, c.leave); !slices.Equal(got, c.order) {
			t.Errorf("%s: the order %v, want %v", c.name, got, c.order)
		}
		if got, ok := Addressee(snap, c.master, members, c.leave); got != c.addressee || !ok {
			t.Errorf("%s: the addressee %s %v, want %s", c.name, got, ok, c.addressee)
		}
	}

	// The room as the Cube has it: its master the metadata's, 0a0201.
	snap := roomsSnapshot(t, wallT)
	if got, ok := RoomAddressee(snap, 1, nil); got != radA || !ok {
		t.Errorf("RoomAddressee with the master dead: %s %v, want %s", got, ok, radA)
	}
	if got, ok := MasterCandidate(snap, 1, nil); got != radA || !ok {
		t.Errorf("MasterCandidate with the wall thermostat dead: %s %v, want %s", got, ok, radA)
	}
	if got, ok := MasterCandidate(roomsSnapshot(t), 1, nil); got != wallT || !ok {
		t.Errorf("MasterCandidate: %s %v, want %s", got, ok, wallT)
	}
	if got, ok := RoomAddressee(snap, 3, nil); ok {
		t.Errorf("a room of a window contact has an addressee: %s", got)
	}
}

// A move's link half into a room whose wall thermostat, its master, is
// dead names the room's first radiator thermostat in its radiogram with
// the room flag (RoomMajority.D9): a dead addressee would not pass it on.
func TestMoveIntoARoomWithADeadWallThermostat(t *testing.T) {
	const moved cube.RFAddress = 0x0a0204
	snap := roomsSnapshot(t, 0x0a0201)
	_, in := MoveCommands(snap, moved, cube.DeviceRadiatorThermostat, 2, 1, RoomMembers(snap, 1, nil), nil)
	want := []string{
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a0202, RoomID: 1, Room: true}, true, moved, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: moved}, true, 0x0a0201, cube.DeviceWallThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: moved}, true, 0x0a0202, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: moved}, true, 0x0a0203, cube.DeviceRadiatorThermostat).String(),
		cube.CmdSetGroup(moved, 1, false).String(),
	}
	var got []string
	for _, l := range in.Lines {
		got = append(got, l.String())
	}
	if !slices.Equal(got, want) {
		t.Fatalf("the link half:\n%v\nwant\n%v", got, want)
	}
}

// A removal's unlink, a re-link and "unlink all and re-link" pass over
// a dead addressee too (RoomMajority.D9): room 1's wall thermostat, its
// master, is dead, so the radiogram with the room flag of the removal
// of 0a0202 names 0a0203, and each member's unlink and re-link the
// first other one alive.
func TestUnlinkAndRelinkPassOverTheDead(t *testing.T) {
	const (
		wallT cube.RFAddress = 0x0a0201
		radA  cube.RFAddress = 0x0a0202
		radB  cube.RFAddress = 0x0a0203
	)
	snap := roomsSnapshot(t, wallT)
	strs := func(ms []cube.Message) []string {
		var out []string
		for _, m := range ms {
			out = append(out, m.String())
		}
		return out
	}
	room := func(to, rf cube.RFAddress, typ cube.DeviceType, add bool) string {
		return cube.CmdLinkPartner(cube.Target{RFAddress: to, RoomID: 1, Room: true}, add, rf, typ).String()
	}
	unlink, _, ok := RemoveCommands(snap, radA, cube.DeviceRadiatorThermostat, nil)
	if want := []string{room(radB, radA, cube.DeviceRadiatorThermostat, false)}; !ok || !slices.Equal(strs(unlink.Lines), want) {
		t.Fatalf("the removal's unlink: %v %v, want %v", strs(unlink.Lines), ok, want)
	}
	cmds := RelinkCommands(snap, 1, nil)
	if len(cmds) != 3 {
		t.Fatalf("the re-link: %d commands", len(cmds))
	}
	for i, c := range []struct {
		rf  cube.RFAddress
		typ cube.DeviceType
		to  cube.RFAddress
	}{{wallT, cube.DeviceWallThermostat, radA}, {radA, cube.DeviceRadiatorThermostat, radB}, {radB, cube.DeviceRadiatorThermostat, radA}} {
		if got := strs(cmds[i].Lines); len(got) == 0 || got[0] != room(c.to, c.rf, c.typ, true) {
			t.Errorf("the re-link of %s: %v, want first %s", c.rf, got, room(c.to, c.rf, c.typ, true))
		}
	}
	want := []string{room(radB, radA, cube.DeviceRadiatorThermostat, true),
		cube.CmdLinkPartner(cube.Target{RFAddress: radA}, true, wallT, cube.DeviceWallThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: radA}, true, radB, cube.DeviceRadiatorThermostat).String(),
		cube.CmdSetGroup(radA, 1, false).String()}
	if got := strs(cmds[1].Lines); !slices.Equal(got, want) {
		t.Fatalf("the re-link of %s:\n%v\nwant\n%v", radA, got, want)
	}
	// Unlink all and re-link: each member's unlink first, then the
	// re-link, each radiogram with the room flag to the first other
	// member alive.
	cmds = UnlinkRelinkCommands(snap, 1, nil)
	if len(cmds) != 6 {
		t.Fatalf("unlink all and re-link: %d commands", len(cmds))
	}
	for i, c := range []struct {
		rf  cube.RFAddress
		typ cube.DeviceType
		to  cube.RFAddress
	}{{wallT, cube.DeviceWallThermostat, radA}, {radA, cube.DeviceRadiatorThermostat, radB}, {radB, cube.DeviceRadiatorThermostat, radA}} {
		if got := strs(cmds[i].Lines); len(got) == 0 || got[0] != room(c.to, c.rf, c.typ, false) {
			t.Errorf("the unlink of %s: %v, want first %s", c.rf, got, room(c.to, c.rf, c.typ, false))
		}
		if got := strs(cmds[3+i].Lines); len(got) == 0 || got[0] != room(c.to, c.rf, c.typ, true) {
			t.Errorf("the re-link of %s after the unlinks: %v, want first %s", c.rf, got, room(c.to, c.rf, c.typ, true))
		}
	}
}
