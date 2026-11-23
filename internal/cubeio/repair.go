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
// The offset is each device's own, and the window-open minutes, which a
// wall thermostat does not store, the device's too.
type Temperatures struct {
	Comfort    float64 `json:"comfort_temp"`
	Eco        float64 `json:"eco_temp"`
	Max        float64 `json:"max_setpoint"`
	Min        float64 `json:"min_setpoint"`
	WindowOpen float64 `json:"window_open_temp"`
}

// Agreement is what most of a room's other thermostats share by their C
// lines, against which a device's configuration is re-sent (D163-A):
// Others counts them; Temps and Program are the shared values, nil
// where no value is shared by more than half of them, with how many
// share each (TempsBy, ProgramBy).
type Agreement struct {
	Others    int               `json:"others"`
	Temps     *Temperatures     `json:"temperatures,omitempty"`
	TempsBy   int               `json:"temperatures_by"`
	Program   *cube.WeekProgram `json:"-"`
	ProgramBy int               `json:"program_by"`
}

// RoomAgreement reads what most of the thermostats of room other than rf
// share by their C lines, the devices in skip left out.
func RoomAgreement(snap *cubelan.Snapshot, rf cube.RFAddress, room uint8, skip view.Hidden) Agreement {
	var a Agreement
	var temps []Temperatures
	var progs []cube.WeekProgram
	for _, d := range ConfigMembers(snap, room, skip) {
		c := snap.Config(d.RFAddress)
		if d.RFAddress == rf || c == nil {
			continue
		}
		t, ok := temperaturesOf(c)
		if !ok {
			continue
		}
		temps = append(temps, t)
		progs = append(progs, ownProgram(c))
	}
	a.Others = len(temps)
	if i, n := majority(len(temps), func(i, j int) bool { return temps[i] == temps[j] }); i >= 0 {
		a.Temps, a.TempsBy = &temps[i], n
	}
	if i, n := majority(len(progs), func(i, j int) bool { return sameWeek(progs[i], progs[j]) }); i >= 0 {
		a.Program, a.ProgramBy = &progs[i], n
	}
	return a
}

// majority finds a value of n that more than half of them share, by
// same; it returns its index and how many share it, or -1.
func majority(n int, same func(i, j int) bool) (int, int) {
	for i := 0; i < n; i++ {
		count := 0
		for j := 0; j < n; j++ {
			if same(i, j) {
				count++
			}
		}
		if 2*count > n {
			return i, count
		}
	}
	return -1, 0
}

// temperaturesOf reads the shared temperatures of a thermostat's C line.
func temperaturesOf(c *cube.DeviceConfig) (Temperatures, bool) {
	switch {
	case c.Thermostat != nil:
		t := c.Thermostat
		return Temperatures{t.ComfortTemp, t.EcoTemp, t.MaxSetpoint, t.MinSetpoint, t.WindowOpenTemp}, true
	case c.Wall != nil:
		w := c.Wall
		return Temperatures{w.ComfortTemp, w.EcoTemp, w.MaxSetpoint, w.MinSetpoint, w.WindowOpenTemp}, true
	}
	return Temperatures{}, false
}

// sameWeek reports whether two week programmes have the same switch
// points every day.
func sameWeek(a, b cube.WeekProgram) bool {
	for d := range a {
		if !sameDay(a[d], b[d]) {
			return false
		}
	}
	return true
}

// NoMajorityError says why a device's configuration cannot be re-sent
// from its room: no other thermostat, or none of its values shared by
// more than half of them.
type NoMajorityError struct {
	Room   uint8
	Others int
	What   string // "temperatures" or "week programme"; "" for no other thermostat
}

func (e *NoMajorityError) Error() string {
	if e.What == "" {
		return fmt.Sprintf("room %d has no other thermostat to take the configuration from", e.Room)
	}
	return fmt.Sprintf("the %d other thermostats of room %d share no %s by a majority: re-send it to the room instead", e.Others, e.Room, e.What)
}

// Resent says what a re-sent configuration sends: whether the
// temperatures, and which days of the week programme.
type Resent struct {
	Temperatures bool           `json:"temperatures"`
	Days         []cube.Weekday `json:"days"`
}

// ResendCommands builds the re-sent configuration of rf, a thermostat
// of room, from its room's agreeing members (RoomAgreement, D163-A):
// after a wakeup of the room, 0x11 with the shared temperatures, its own
// offset and window-open minutes kept, where its own differ; then the
// days of the shared week programme that differ from its own, followed
// by 0x11 once more, as a copy of a room's settings sends them
// (CopyCommands). The error is a NoMajorityError where the room has no
// other thermostat or its others share no temperatures or no week
// programme; nothing is built where rf agrees already.
func ResendCommands(snap *cubelan.Snapshot, rf cube.RFAddress, room uint8, skip view.Hidden) ([]engine.Command, Resent, Agreement, error) {
	c := snap.Config(rf)
	if c == nil {
		return nil, Resent{}, Agreement{}, fmt.Errorf("device %s has no configuration to re-send", rf)
	}
	own, ok := temperaturesOf(c)
	if !ok {
		return nil, Resent{}, Agreement{}, fmt.Errorf("device %s is a %s and has no configuration to re-send", rf, c.Type)
	}
	a := RoomAgreement(snap, rf, room, skip)
	switch {
	case a.Others == 0:
		return nil, Resent{}, a, &NoMajorityError{Room: room}
	case a.Temps == nil:
		return nil, Resent{}, a, &NoMajorityError{Room: room, Others: a.Others, What: "temperatures"}
	case a.Program == nil:
		return nil, Resent{}, a, &NoMajorityError{Room: room, Others: a.Others, What: "week programme"}
	}
	minutes := RoomRadiator(snap, room, rf).WindowOpenMins
	if c.Thermostat != nil {
		minutes = c.Thermostat.WindowOpenMins
	}
	t := a.Temps
	temps, err := cube.CmdSetTemperatures(cube.Target{RFAddress: rf}, cube.TemperatureSettings{Comfort: t.Comfort, Eco: t.Eco,
		Max: t.Max, Min: t.Min, Offset: ownOffset(c), WindowOpen: t.WindowOpen, WindowOpenMinutes: minutes})
	if err != nil {
		return nil, Resent{}, a, fmt.Errorf("the room's temperatures for %s: %w", rf, err)
	}
	sent := Resent{Temperatures: own != *t}
	mine := ownProgram(c)
	var lines []cube.Message
	for _, d := range copyDayOrder {
		if len(a.Program[d]) == 0 || sameDay(a.Program[d], mine[d]) {
			continue
		}
		ls, err := cube.CmdSetDayProgram(cube.Target{RFAddress: rf}, d, a.Program[d])
		if err != nil {
			return nil, Resent{}, a, fmt.Errorf("the room's programme for %s: %w", d, err)
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
		return nil, sent, a, nil
	}
	return []engine.Command{{Key: "repair-config:" + rf.String(), Target: rf, RoomID: room, Wakeup: true, Lines: lines}}, sent, a, nil
}
