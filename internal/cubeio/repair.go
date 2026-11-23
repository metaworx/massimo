// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

// The repairs of a Cube's rooms and devices (AP RadioHealth v1.2, Blocks
// 11 and 12, D163): the commands each sends, built from the snapshot so
// that a preview shows exactly what a run sends. A room is its members by
// their C lines, the Cube's own idea of it, as RoomAddressee takes it;
// the server refuses a re-link of a room whose C lines and metadata
// differ (RoomDisagreement), which the placement repairs first.

import (
	"fmt"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// Flagged lists the devices whose error flag (byte 5's 0x08, the
// communication error) is set, in the Cube's order: those whose C line
// names room, or every device for room 0. The Cube itself and the
// devices in skip are left out.
func Flagged(snap *cubelan.Snapshot, room uint8, skip view.Hidden) []cube.RFAddress {
	var out []cube.RFAddress
	for _, rf := range snap.Devices() {
		c := snap.Config(rf)
		if c == nil || skip[rf] || c.Type == cube.DeviceCube || room != 0 && c.RoomID != room {
			continue
		}
		if st, ok := snap.State(rf); ok && st.Error {
			out = append(out, rf)
		}
	}
	return out
}

// ResetCommand is one r: for each of devices, which clears the Cube's
// error flag of each at no radio cost (docs/cube-protocol.md, the r:
// findings): no wakeup and no target, so that the queue adds nothing.
func ResetCommand(key string, devices []cube.RFAddress) engine.Command {
	cmd := engine.Command{Key: key, Priority: engine.PriorityHigh}
	for _, rf := range devices {
		cmd.Lines = append(cmd.Lines, cube.CmdResetError(rf))
	}
	return cmd
}

// ConfigMembers lists the devices whose C line names room, the Cube and
// the devices in skip left out, in the Cube's order.
func ConfigMembers(snap *cubelan.Snapshot, room uint8, skip view.Hidden) []cube.DeviceMeta {
	var members []cube.DeviceMeta
	for _, rf := range snap.Devices() {
		c := snap.Config(rf)
		if c == nil || skip[rf] || c.RoomID != room || c.Type == cube.DeviceCube {
			continue
		}
		members = append(members, cube.DeviceMeta{Type: c.Type, RFAddress: rf, RoomID: room})
	}
	return members
}

// RoomDisagreement lists the devices that the metadata places in room
// and whose C line names another room or none, and those whose C line
// names room and that the metadata places elsewhere or does not list,
// the devices in skip left out: a room whose list and Cube disagree.
func RoomDisagreement(snap *cubelan.Snapshot, room uint8, skip view.Hidden) []cube.RFAddress {
	byConfig := map[cube.RFAddress]bool{}
	for _, d := range ConfigMembers(snap, room, skip) {
		byConfig[d.RFAddress] = true
	}
	var out []cube.RFAddress
	listed := map[cube.RFAddress]bool{}
	for _, d := range RoomMembers(snap, room, skip) {
		listed[d.RFAddress] = true
		if !byConfig[d.RFAddress] && snap.Config(d.RFAddress) != nil {
			out = append(out, d.RFAddress)
		}
	}
	for _, d := range ConfigMembers(snap, room, skip) {
		if !listed[d.RFAddress] {
			out = append(out, d.RFAddress)
		}
	}
	return out
}

// RelinkCommands builds the re-link of room, adding only (D163-A): for
// each member that takes part in links (Linkable), after a wakeup of the
// room, the placement's link half (AP DeviceUnPairingFix v1.3, D170-A):
// one 0x20 with the room flag naming it, to the room's addressee, one
// to each window contact among its partners, one to the member for each
// partner, then 0x22, its group. Each is a command of its own with the
// member its target, so that the queue resets it where flagged and
// retries a discard once; the first command resets the room's other
// flagged members (D161-B), at no radio cost. Nothing is unlinked. The
// members are the devices whose C line names room, the devices in skip
// left out.
func RelinkCommands(snap *cubelan.Snapshot, room uint8, skip view.Hidden) []engine.Command {
	return relink(snap, room, skip, true)
}

// relink is RelinkCommands; reset says that its first command resets
// the room's flagged members.
func relink(snap *cubelan.Snapshot, room uint8, skip view.Hidden, reset bool) []engine.Command {
	members := ConfigMembers(snap, room, skip)
	master := RoomMaster(snap, room)
	var out []engine.Command
	for _, d := range members {
		if !Linkable(d.Type) {
			continue
		}
		var lines []cube.Message
		if reset && len(out) == 0 {
			lines = flaggedResets(snap, members, d.RFAddress)
		}
		lines = append(lines, linkLines(snap, master, d.RFAddress, d.Type, room, members, true, true)...)
		lines = append(lines, cube.CmdSetGroup(d.RFAddress, room, false))
		out = append(out, engine.Command{Key: "repair-relink:" + d.RFAddress.String(), Target: d.RFAddress, RoomID: room, Wakeup: true, Lines: lines})
	}
	return out
}

// UnlinkRelinkCommands builds "unlink all and re-link" of room (D163-B,
// a step of its own behind a warning): for each member that takes part
// in links, after a wakeup of the room, the 0x21s of a move's unlink
// half without its 0x23 (one with the room flag naming it, to the room's
// addressee, one to each window contact among its partners, one to the
// member for each partner), which leaves the member in its room, its
// group kept, with no partner; then the re-link of every member
// (RelinkCommands), whose 0x22 sets the group again. The first command
// resets the room's other flagged members. Not yet run on hardware.
func UnlinkRelinkCommands(snap *cubelan.Snapshot, room uint8, skip view.Hidden) []engine.Command {
	members := ConfigMembers(snap, room, skip)
	master := RoomMaster(snap, room)
	var out []engine.Command
	for _, d := range members {
		if !Linkable(d.Type) {
			continue
		}
		var lines []cube.Message
		if len(out) == 0 {
			lines = flaggedResets(snap, members, d.RFAddress)
		}
		lines = append(lines, linkLines(snap, master, d.RFAddress, d.Type, room, members, false, true)...)
		if len(lines) == 0 {
			continue // no partner: nothing to unlink
		}
		out = append(out, engine.Command{Key: "repair-unlink:" + d.RFAddress.String(), Target: d.RFAddress, RoomID: room, Wakeup: true, Lines: lines})
	}
	return append(out, relink(snap, room, skip, len(out) == 0)...)
}

// MasterCandidate is the thermostat a room's metadata should name as its
// master: the first of the room's order with the master itself left
// aside (RoomOrder, RoomMajority.D9), its wall thermostat, else its
// first radiator thermostat, by their C lines, the dead passed over
// where one is alive, the devices in skip left out (Addressee); false
// for a room without a thermostat.
func MasterCandidate(snap *cubelan.Snapshot, room uint8, skip view.Hidden) (cube.RFAddress, bool) {
	return Addressee(snap, 0, ConfigMembers(snap, room, skip), 0)
}

// MasterValid reports whether the metadata's master of room is a
// thermostat whose C line names the room, which a room command may
// name (D167).
func MasterValid(snap *cubelan.Snapshot, room uint8, skip view.Hidden) bool {
	m := RoomMaster(snap, room)
	if m == 0 {
		return false
	}
	for _, d := range ConfigMembers(snap, room, skip) {
		if d.RFAddress == m {
			return d.Type.IsThermostat()
		}
	}
	return false
}

// Temperatures are the settings of a 0x11 that a room's thermostats
// share: comfort, eco, maximum, minimum and window-open temperatures.
// The offset is each device's own; the window-open minutes, which a wall
// thermostat does not store, are not among them (RoomSettings).
type Temperatures struct {
	Comfort    float64 `json:"comfort_temp"`
	Eco        float64 `json:"eco_temp"`
	Max        float64 `json:"max_setpoint"`
	Min        float64 `json:"min_setpoint"`
	WindowOpen float64 `json:"window_open_temp"`
}

// RoomAgreement is the configuration of room that rf's is re-sent from
// (D163-A, as AP RoomMajority's D9 changes it): RoomConfig over every
// thermostat whose C line names the room, rf included, the devices in
// skip left out, the room's master the metadata's, the reference the
// server's report of members that differ judges rf by, so that the
// repair sends what the report named. others counts the room's other
// thermostats the Cube holds a valid state for; false, and nothing to
// re-send, where it has none.
func RoomAgreement(snap *cubelan.Snapshot, rf cube.RFAddress, room uint8, skip view.Hidden) (s RoomSettings, others int, ok bool) {
	members := ConfigMembers(snap, room, skip)
	for _, d := range members {
		if d.RFAddress != rf && d.Type.IsThermostat() && snap.Config(d.RFAddress) != nil && !Dead(snap, d.RFAddress) {
			others++
		}
	}
	if others == 0 {
		return RoomSettings{}, 0, false
	}
	s, ok = RoomConfig(snap, RoomMaster(snap, room), members, 0, nil)
	return s, others, ok
}

// temperatures are the shared temperatures of a thermostat's C line,
// zero for a configuration without them.
func temperatures(c *cube.DeviceConfig) Temperatures {
	t, _ := temperaturesOf(c)
	return t
}

// temperaturesOf reads the shared temperatures of a thermostat's C line;
// false for none, or another device's.
func temperaturesOf(c *cube.DeviceConfig) (Temperatures, bool) {
	switch {
	case c == nil:
	case c.Thermostat != nil:
		t := c.Thermostat
		return Temperatures{t.ComfortTemp, t.EcoTemp, t.MaxSetpoint, t.MinSetpoint, t.WindowOpenTemp}, true
	case c.Wall != nil:
		w := c.Wall
		return Temperatures{w.ComfortTemp, w.EcoTemp, w.MaxSetpoint, w.MinSetpoint, w.WindowOpenTemp}, true
	}
	return Temperatures{}, false
}

// NoMajorityError says why a device's configuration cannot be re-sent
// from its room: the room has no other thermostat the Cube holds a valid
// state for (RoomAgreement, RoomMajority.D9); Dead counts the others
// passed over for that.
type NoMajorityError struct {
	Room uint8
	Dead int
}

func (e *NoMajorityError) Error() string {
	if e.Dead == 0 {
		return fmt.Sprintf("room %d has no other thermostat to take the configuration from", e.Room)
	}
	return fmt.Sprintf("room %d has no other thermostat the cube holds a valid state for to take the configuration from: its %d others have not reported since the cube's last restart or are out of date", e.Room, e.Dead)
}

// Resent says what a re-sent configuration sends: whether the
// temperatures, and which days of the week programme.
type Resent struct {
	Temperatures bool           `json:"temperatures"`
	Days         []cube.Weekday `json:"days"`
}

// ResendCommands builds the re-sent configuration of rf, a thermostat
// of room, from its room's (RoomAgreement: D163-A as AP RoomMajority's
// D9 changes it): after a wakeup of the room, the days of the room's
// week programme that differ from its own, then 0x11 with the room's
// temperatures, its own offset and window-open minutes kept, where its
// own temperatures or a day differ, as a copy of a room's settings sends
// them (CopyCommands). others counts the room's other thermostats the
// Cube holds a valid state for. The error is a NoMajorityError where the
// room has none; nothing is built where rf has the room's configuration
// already.
func ResendCommands(snap *cubelan.Snapshot, rf cube.RFAddress, room uint8, skip view.Hidden) (cmds []engine.Command, sent Resent, a RoomSettings, others int, err error) {
	c := snap.Config(rf)
	if c == nil {
		return nil, Resent{}, RoomSettings{}, 0, fmt.Errorf("device %s has no configuration to re-send", rf)
	}
	if _, ok := temperaturesOf(c); !ok {
		return nil, Resent{}, RoomSettings{}, 0, fmt.Errorf("device %s is a %s and has no configuration to re-send", rf, c.Type)
	}
	a, others, ok := RoomAgreement(snap, rf, room, skip)
	if !ok {
		dead := 0
		for _, d := range ConfigMembers(snap, room, skip) {
			if d.RFAddress != rf && d.Type.IsThermostat() && Dead(snap, d.RFAddress) {
				dead++
			}
		}
		return nil, Resent{}, a, 0, &NoMajorityError{Room: room, Dead: dead}
	}
	minutes := RoomRadiator(snap, room, rf).WindowOpenMins
	if c.Thermostat != nil {
		minutes = c.Thermostat.WindowOpenMins
	}
	t := a.Temps
	temps, err := cube.CmdSetTemperatures(cube.Target{RFAddress: rf}, cube.TemperatureSettings{Comfort: t.Comfort, Eco: t.Eco,
		Max: t.Max, Min: t.Min, Offset: ownOffset(c), WindowOpen: t.WindowOpen, WindowOpenMinutes: minutes})
	if err != nil {
		return nil, Resent{}, a, others, fmt.Errorf("the room's temperatures for %s: %w", rf, err)
	}
	sent = Resent{Temperatures: !a.SameTemperatures(c)}
	var lines []cube.Message
	for _, d := range a.Days(c) {
		ls, err := cube.CmdSetDayProgram(cube.Target{RFAddress: rf}, d, a.Program[d])
		if err != nil {
			return nil, Resent{}, a, others, fmt.Errorf("the room's programme for %s: %w", d, err)
		}
		lines = append(lines, ls...)
		sent.Days = append(sent.Days, d)
	}
	if len(lines) > 0 || sent.Temperatures {
		// The 0x11 after the days, as the copy sends it, carries the
		// temperatures too.
		lines = append(lines, temps)
		sent.Temperatures = true
	}
	if len(lines) == 0 {
		return nil, sent, a, others, nil
	}
	return []engine.Command{{Key: "repair-config:" + rf.String(), Target: rf, RoomID: room, Wakeup: true, Lines: lines}}, sent, a, others, nil
}
