// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

// The copy of a room's state and configuration to a thermostat put into
// it, and the one rule of a room's configuration (AP RoomMajority,
// Block 2: D5-A, D8-A, D9, D16-A), over the rooms fixture
// (fakecube.RoomsFixture, roomsSnapshot): room 1 its wall thermostat
// 0a0201, the master, and the radiator thermostats 0a0202 and 0a0203;
// room 2 the radiator thermostats 0a0204 to 0a0206, 0a0204 its master;
// room 3 the window contact 0a0207 alone.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// stepLines are the lines of steps' commands as they go to the Cube,
// one slice per step.
func stepLines(steps []CopyStep) [][]string {
	var out [][]string
	for _, st := range steps {
		var ls []string
		for _, l := range st.Command.Lines {
			ls = append(ls, l.String())
		}
		out = append(out, ls)
	}
	return out
}

// setTemperature is a 0x40 to t, failing the test where it cannot be
// built.
func setTemperature(t *testing.T, to cube.Target, temp float64, mode cube.Mode, until cube.Until) string {
	t.Helper()
	m, err := cube.CmdSetTemperature(to, temp, mode, until)
	if err != nil {
		t.Fatal(err)
	}
	return m.String()
}

// setTemperatures is a 0x11 to rf with the temperatures of c, a radiator
// thermostat's or a wall thermostat's, the window-open minutes and the
// offset given.
func setTemperatures(t *testing.T, rf cube.RFAddress, c Temperatures, minutes int, offset float64) string {
	t.Helper()
	m, err := cube.CmdSetTemperatures(cube.Target{RFAddress: rf}, cube.TemperatureSettings{Comfort: c.Comfort, Eco: c.Eco, Max: c.Max, Min: c.Min,
		Offset: offset, WindowOpen: c.WindowOpen, WindowOpenMinutes: minutes})
	if err != nil {
		t.Fatal(err)
	}
	return m.String()
}

// The copy's 0x40 carries the room's state from before the change, to
// the device directly, without a Target (the Cube discarded a 0x40 with
// one to a device in a room on 2026-11-04): manual 5 °C as on
// 2026-11-23, auto on the programme, auto with a temperature set by
// hand, a holiday; auto with temperature 0 where there is no state, or
// the room has no other thermostat (0a0204 into room 3, a window
// contact's alone). The room's state follows as a room command to the
// room's addressee, the step CopySettings runs only after a discarded
// 0x40 (D16-A), and none without a state. A window contact gets
// nothing.
func TestCopyCommandsTakeTheRoomsState(t *testing.T) {
	snap := roomsSnapshot(t)
	holiday := cube.Until{Year: 2026, Month: 11, Day: 10, Minutes: 12 * 60}
	direct := cube.Target{RFAddress: 0x0a0204}
	room1 := cube.Target{RFAddress: 0x0a0201, RoomID: 1, Room: true}
	for _, c := range []struct {
		name  string
		room  uint8
		state *RoomState
		mode  string // the 0x40
		room0 string // the room command, "" for none
	}{
		{"manual 5 °C", 1, &RoomState{Mode: cube.ModeManual, Temp: 5}, setTemperature(t, direct, 5, cube.ModeManual, cube.Until{}),
			setTemperature(t, room1, 5, cube.ModeManual, cube.Until{})},
		{"auto on the programme", 1, &RoomState{}, setTemperature(t, direct, 0, cube.ModeAuto, cube.Until{}),
			setTemperature(t, room1, 0, cube.ModeAuto, cube.Until{})},
		{"auto with a temperature", 1, &RoomState{Mode: cube.ModeAuto, Temp: 23}, setTemperature(t, direct, 23, cube.ModeAuto, cube.Until{}),
			setTemperature(t, room1, 23, cube.ModeAuto, cube.Until{})},
		{"a holiday", 1, &RoomState{Mode: cube.ModeVacation, Temp: 12, Until: holiday}, setTemperature(t, direct, 12, cube.ModeVacation, holiday),
			setTemperature(t, room1, 12, cube.ModeVacation, holiday)},
		{"no state", 1, nil, setTemperature(t, direct, 0, cube.ModeAuto, cube.Until{}), ""},
		{"no other thermostat", 3, &RoomState{Mode: cube.ModeManual, Temp: 5}, setTemperature(t, direct, 0, cube.ModeAuto, cube.Until{}), ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			steps, err := CopyCommands(snap, 0x0a0204, cube.DeviceRadiatorThermostat, c.room, nil, c.state)
			if err != nil {
				t.Fatal(err)
			}
			mode := steps[0]
			if !mode.Mode || mode.Command.Target != 0 || mode.Command.Key != "copy-mode:0a0204" || len(mode.Command.Lines) != 1 || mode.Command.Lines[0].String() != c.mode {
				t.Fatalf("the 0x40 %+v, want %s without a Target", mode, c.mode)
			}
			last := steps[len(steps)-1]
			switch {
			case c.room0 == "" && slices.ContainsFunc(steps, func(st CopyStep) bool { return st.RoomMode }):
				t.Fatalf("a room command without a state: %v", stepLines(steps))
			case c.room0 != "" && (!last.RoomMode || len(last.Command.Lines) != 1 || last.Command.Lines[0].String() != c.room0 || last.Command.Target != 0):
				t.Fatalf("the room command %+v, want %s last", last, c.room0)
			}
		})
	}
	if steps, err := CopyCommands(snap, 0x0a0207, cube.DeviceShutterContact, 1, nil, &RoomState{}); err != nil || steps != nil {
		t.Fatalf("a contact: %+v %v", steps, err)
	}
}

// The copy's configuration into room 1, by RoomConfig: the radiator
// thermostats' temperatures, which two of the three share, with each
// device's own offset to every thermostat of the room, radiator
// thermostats first; their valve settings; no day, since 0a0204's
// programme is theirs already. With both radiator thermostats dead, as
// after a power cut they have not reported since, the wall thermostat
// alone speaks for the room: its temperatures, its boost settings with
// the original software's descaling and valve limits, the original's
// window-open minutes, and every day of its programme, which differs
// from 0a0204's, then 0x11 again; the 0x11 goes to 0a0204 and the wall
// thermostat alone, not to the dead, whose configuration did not count.
// With every other member dead, nothing of the configuration is sent,
// and the 0x40's step says why, as it does in a room of no other
// thermostat.
func TestCopyCommandsTakeTheRoomsConfiguration(t *testing.T) {
	snap := roomsSnapshot(t)
	steps, err := CopyCommands(snap, 0x0a0204, cube.DeviceRadiatorThermostat, 1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	rad := temperatures(snap.Config(0x0a0202))
	valve, _ := cube.CmdSetValveConfig(cube.Target{RFAddress: 0x0a0204}, cube.ValveSettings{BoostMinutes: 5, BoostValvePct: 80,
		DecalcDay: cube.Saturday, DecalcHour: 12, MaxValvePct: 100})
	want := [][]string{
		{setTemperature(t, cube.Target{RFAddress: 0x0a0204}, 0, cube.ModeAuto, cube.Until{})},
		{setTemperatures(t, 0x0a0202, rad, 15, 0), setTemperatures(t, 0x0a0203, rad, 15, 0), setTemperatures(t, 0x0a0204, rad, 15, 0),
			setTemperatures(t, 0x0a0201, rad, 15, 0)},
		{valve.String()},
	}
	if got := stepLines(steps); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("the copy into room 1:\n%v\nwant\n%v", got, want)
	}
	if st := steps[1].Command; !st.Wakeup || st.RoomID != 1 || st.Target != 0 || steps[2].Command.Target != 0x0a0204 {
		t.Fatalf("the steps' wakeups and targets: %+v", steps)
	}

	snap = roomsSnapshot(t, 0x0a0202, 0x0a0203)
	steps, err = CopyCommands(snap, 0x0a0204, cube.DeviceRadiatorThermostat, 1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := snap.Config(0x0a0201).Wall
	walls := temperatures(snap.Config(0x0a0201))
	valve, _ = cube.CmdSetValveConfig(cube.Target{RFAddress: 0x0a0204}, cube.ValveSettings{BoostMinutes: w.BoostMinutes, BoostValvePct: w.BoostValvePct,
		DecalcDay: cube.Saturday, DecalcHour: 12, MaxValvePct: 100})
	want = [][]string{
		{setTemperature(t, cube.Target{RFAddress: 0x0a0204}, 0, cube.ModeAuto, cube.Until{})},
		{setTemperatures(t, 0x0a0204, walls, 15, 0), setTemperatures(t, 0x0a0201, walls, 15, 0)},
		{valve.String()},
	}
	var days []string
	for _, d := range copyDayOrder {
		ls, err := cube.CmdSetDayProgram(cube.Target{RFAddress: 0x0a0204}, d, w.WeekProgram[d])
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range ls {
			days = append(days, l.String())
		}
	}
	want = append(want, append(days, setTemperatures(t, 0x0a0204, walls, 15, 0)))
	if got := stepLines(steps); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("the copy into room 1, its radiator thermostats dead:\n%v\nwant\n%v", got, want)
	}
	if steps[3].Copies.Days != 7 || steps[0].Copies.ConfigSkipped != "" || !slices.Equal(steps[1].Copies.Temperatures, []cube.RFAddress{0x0a0204, 0x0a0201}) {
		t.Fatalf("copied %+v", steps)
	}

	for _, c := range []struct {
		name string
		snap *cubelan.Snapshot
		room uint8
		why  string
	}{
		{"every other member dead", roomsSnapshot(t, 0x0a0201, 0x0a0202, 0x0a0203), 1, SkipNoState},
		{"no other thermostat", roomsSnapshot(t), 3, SkipNoOther},
	} {
		steps, err := CopyCommands(c.snap, 0x0a0204, cube.DeviceRadiatorThermostat, c.room, nil, nil)
		if err != nil || len(steps) != 1 || !steps[0].Mode || steps[0].Copies.ConfigSkipped != c.why {
			t.Fatalf("%s: %+v %v, want the 0x40 alone and %q", c.name, steps, err, c.why)
		}
	}
}

// roomConfigs is snap's configurations, those of edits changed: a radiator
// thermostat's each, copied first.
func roomConfigs(snap *cubelan.Snapshot, edits map[cube.RFAddress]func(*cube.ThermostatConfig)) func(cube.RFAddress) *cube.DeviceConfig {
	cfgs := map[cube.RFAddress]*cube.DeviceConfig{}
	for rf, edit := range edits {
		c := *snap.Config(rf)
		th := *c.Thermostat
		edit(&th)
		c.Thermostat = &th
		cfgs[rf] = &c
	}
	return func(rf cube.RFAddress) *cube.DeviceConfig {
		if c, ok := cfgs[rf]; ok {
			return c
		}
		return snap.Config(rf)
	}
}

// One rule for a room's configuration (RoomMajority.D9, D5-A), each group
// on its own: in room 2, 0a0204 has another comfort temperature and
// Monday, 0a0205 another Tuesday, and 0a0206, the master here, other
// valve settings; each group and each day is what two of the three
// share, from no one device. On a tie, the master's; a dead member is
// passed over, and with every member dead there is none. In room 1 the
// valve settings and the window-open minutes are the radiator
// thermostats', the first of theirs on a tie, though the master is the
// wall thermostat, and in room 2 the minutes most of them share, not the
// master's; with only the wall thermostat alive, its boost settings and
// the original software's defaults.
func TestRoomConfig(t *testing.T) {
	snap := roomsSnapshot(t)
	room2 := []cube.DeviceMeta{meta(0x0a0204, cube.DeviceRadiatorThermostat), meta(0x0a0205, cube.DeviceRadiatorThermostat),
		meta(0x0a0206, cube.DeviceRadiatorThermostat)}
	base := *snap.Config(0x0a0204).Thermostat
	monday := cube.DayProgram{{Temp: 19, Until: 1440}}
	tuesday := cube.DayProgram{{Temp: 18, Until: 1440}}
	edits := map[cube.RFAddress]func(*cube.ThermostatConfig){
		0x0a0204: func(c *cube.ThermostatConfig) { c.ComfortTemp = 23; c.WeekProgram[cube.Monday] = monday },
		0x0a0205: func(c *cube.ThermostatConfig) { c.WeekProgram[cube.Tuesday] = tuesday },
		0x0a0206: func(c *cube.ThermostatConfig) { c.BoostMinutes, c.BoostValvePct = 10, 90 },
	}
	config := roomConfigs(snap, edits)
	s, ok := RoomConfig(snap, 0x0a0206, room2, 0, config)
	switch {
	case !ok || s.Others != 3:
		t.Fatalf("room 2: %+v %v", s, ok)
	case s.Temps.Comfort != base.ComfortTemp || s.TempsBy != 2:
		t.Errorf("the temperatures %+v by %d, want 0a0205's and 0a0206's", s.Temps, s.TempsBy)
	case s.Valve.BoostMinutes != base.BoostMinutes || s.Valve.BoostValvePct != base.BoostValvePct:
		t.Errorf("the valve settings %+v, want 0a0204's and 0a0205's", s.Valve)
	case !sameDay(s.Program[cube.Monday], base.WeekProgram[cube.Monday]) || s.DaysBy[cube.Monday] != 2:
		t.Errorf("Monday %v by %d, want 0a0205's and 0a0206's", s.Program[cube.Monday], s.DaysBy[cube.Monday])
	case !sameDay(s.Program[cube.Tuesday], base.WeekProgram[cube.Tuesday]) || s.DaysBy[cube.Tuesday] != 2:
		t.Errorf("Tuesday %v by %d, want 0a0204's and 0a0206's", s.Program[cube.Tuesday], s.DaysBy[cube.Tuesday])
	case s.DaysBy[cube.Friday] != 3 || s.ProgramBy() != 2 || s.WindowOpenMins != base.WindowOpenMins:
		t.Errorf("Friday by %d, the programme by %d, minutes %d", s.DaysBy[cube.Friday], s.ProgramBy(), s.WindowOpenMins)
	}
	if got := s.Days(config(0x0a0204)); !slices.Equal(got, []cube.Weekday{cube.Monday}) {
		t.Errorf("0a0204's days that differ: %v", got)
	}
	if s.SameTemperatures(config(0x0a0204)) || !s.SameTemperatures(config(0x0a0205)) {
		t.Errorf("the temperatures compared wrongly")
	}

	// 0a0204 and 0a0205, 0a0206 left: they tie on the comfort temperature,
	// which the master's decides, whichever of them it is.
	for _, c := range []struct {
		master  cube.RFAddress
		comfort float64
	}{{0x0a0204, 23}, {0x0a0205, base.ComfortTemp}} {
		if s, ok := RoomConfig(snap, c.master, room2, 0x0a0206, config); !ok || s.Temps.Comfort != c.comfort || s.TempsBy != 1 || s.Others != 2 {
			t.Errorf("a tie, master %s: %+v %v, want comfort %v", c.master, s, ok, c.comfort)
		}
	}
	// The master 0a0204 dead: passed over, 0a0205's.
	dead := roomsSnapshot(t, 0x0a0204)
	if s, ok := RoomConfig(dead, 0x0a0204, room2, 0x0a0206, roomConfigs(dead, edits)); !ok || s.Temps.Comfort != base.ComfortTemp || s.Others != 1 {
		t.Errorf("the master dead: %+v %v", s, ok)
	}
	allDead := roomsSnapshot(t, 0x0a0204, 0x0a0205, 0x0a0206)
	if s, ok := RoomConfig(allDead, 0x0a0204, room2, 0, nil); ok {
		t.Errorf("every member dead: %+v", s)
	}

	room1 := []cube.DeviceMeta{meta(0x0a0201, cube.DeviceWallThermostat), meta(0x0a0202, cube.DeviceRadiatorThermostat),
		meta(0x0a0203, cube.DeviceRadiatorThermostat)}
	tie := roomConfigs(snap, map[cube.RFAddress]func(*cube.ThermostatConfig){
		0x0a0202: func(c *cube.ThermostatConfig) { c.BoostMinutes, c.BoostValvePct, c.WindowOpenMins = 10, 90, 20 },
	})
	s, ok = RoomConfig(snap, 0x0a0201, room1, 0, tie)
	if !ok || s.Valve.BoostMinutes != 10 || s.Valve.BoostValvePct != 90 || s.WindowOpenMins != 20 || s.Valve.MaxValvePct != base.MaxValvePct {
		t.Errorf("room 1's valve settings and minutes: %+v %v, want 0a0202's, the first radiator thermostat's", s, ok)
	}
	// Room 2's window-open minutes by the majority of its radiator
	// thermostats, not its first one's: its master 0a0204 has 20, the
	// others 15.
	minutes := roomConfigs(snap, map[cube.RFAddress]func(*cube.ThermostatConfig){
		0x0a0204: func(c *cube.ThermostatConfig) { c.WindowOpenMins = 20 },
	})
	if s, ok := RoomConfig(snap, 0x0a0204, room2, 0, minutes); !ok || s.WindowOpenMins != base.WindowOpenMins || base.WindowOpenMins == 20 {
		t.Errorf("room 2's window-open minutes: %+v %v, want %d, the majority's", s, ok, base.WindowOpenMins)
	}
	walls := roomsSnapshot(t, 0x0a0202, 0x0a0203)
	w := walls.Config(0x0a0201).Wall
	s, ok = RoomConfig(walls, 0x0a0201, room1, 0, nil)
	if want := (cube.ValveSettings{BoostMinutes: w.BoostMinutes, BoostValvePct: w.BoostValvePct, DecalcDay: cube.Saturday, DecalcHour: 12, MaxValvePct: 100}); !ok ||
		s.Valve != want || s.WindowOpenMins != 15 || s.Temps.Comfort != w.ComfortTemp || s.Others != 1 {
		t.Errorf("room 1, the wall thermostat alone: %+v %v, want %+v", s, ok, want)
	}
}

// Reached, whether a radiogram of a command may have reached a device:
// an s: line the Cube accepted, also after a retry, or one sent whose
// answer did not come, and every line where the caller's context ended
// while the queue ran the command; not a line discarded, also after a
// retry, nor an r: line, nor a line never sent.
func TestReached(t *testing.T) {
	s1, s2, s3 := cube.CmdSetGroup(0x0a0201, 1, false), cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a0201}, true, 0x0a0202, cube.DeviceRadiatorThermostat),
		cube.CmdSetGroup(0x0a0202, 1, false)
	reset := cube.CmdResetError(0x0a0201)
	ok, discarded := cube.Message{Type: 'S', Data: "00,0,31"}, cube.Message{Type: 'S', Data: "00,1,31"}
	three := engine.Command{Lines: []cube.Message{s1, s2, s3}}
	for _, c := range []struct {
		name string
		cmd  engine.Command
		res  engine.Result
		want bool
	}{
		{"accepted", three, engine.Result{Sent: 3, Replies: []cube.Message{ok, ok, ok}}, true},
		{"the first line discarded", three, engine.Result{Sent: 1, Replies: []cube.Message{discarded}, Discarded: true, Err: engine.ErrDiscarded}, false},
		{"the last line discarded", three, engine.Result{Sent: 3, Replies: []cube.Message{ok, ok, discarded}, Discarded: true, Err: engine.ErrDiscarded}, true},
		{"no answer after a send error", three, engine.Result{Sent: 1, Err: errors.New("cubelan: connection lost")}, true},
		{"a retry accepted", three, engine.Result{Sent: 3, Replies: []cube.Message{ok, ok, ok}, Retried: true}, true},
		{"a retry discarded", three, engine.Result{Sent: 1, Replies: []cube.Message{discarded}, Retried: true, Discarded: true, Err: engine.ErrDiscarded}, false},
		{"r: lines accepted, the radiogram discarded", engine.Command{Lines: []cube.Message{reset, s1}},
			engine.Result{Sent: 2, Replies: []cube.Message{ok, discarded}, Discarded: true, Err: engine.ErrDiscarded}, false},
		{"r: lines alone", engine.Command{Lines: []cube.Message{reset}}, engine.Result{Sent: 1, Replies: []cube.Message{ok}}, false},
		{"nothing sent", three, engine.Result{Err: errors.New("wakeup: cubelan: not connected")}, false},
		{"the context ended", three, engine.Result{Err: context.DeadlineExceeded}, true},
		{"the context cancelled", three, engine.Result{Err: fmt.Errorf("wait: %w", context.Canceled)}, true},
	} {
		if got := Reached(c.cmd, c.res); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
