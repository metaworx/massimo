// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package roomorder

import (
	"slices"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// The steps of State and what each gives: the member whose state it is,
// the rule, and the members it came from.
func TestState(t *testing.T) {
	t0 := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	m := func(rf cube.RFAddress, typ cube.DeviceType, mode cube.Mode, set float64, follows bool, changed int) Member {
		sp := set
		x := Member{RF: rf, Type: typ, State: cube.DeviceState{RFAddress: rf, Valid: true, Mode: mode, Setpoint: &sp}, Follows: follows}
		if changed > 0 {
			x.ModeAt = t0.Add(time.Duration(changed) * time.Minute)
		}
		return x
	}
	const wall, radA, radB cube.RFAddress = 0x0a0201, 0x0a0202, 0x0a0203
	for _, c := range []struct {
		name   string
		master cube.RFAddress
		ms     []Member
		index  int
		rule   Rule
		from   []cube.RFAddress
	}{
		{"more than half", wall, []Member{m(wall, cube.DeviceWallThermostat, cube.ModeManual, 23, false, 5),
			m(radA, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, false, 0), m(radB, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, false, 0)},
			1, ByMajority, []cube.RFAddress{radA, radB}},
		{"two on their programmes as one", radB, []Member{m(wall, cube.DeviceWallThermostat, cube.ModeAuto, 21, true, 0),
			m(radA, cube.DeviceRadiatorThermostat, cube.ModeAuto, 19, true, 0), m(radB, cube.DeviceRadiatorThermostat, cube.ModeManual, 21, false, 0)},
			0, ByMajority, []cube.RFAddress{wall, radA}},
		{"the newest change", wall, []Member{m(wall, cube.DeviceWallThermostat, cube.ModeManual, 23, false, 1),
			m(radA, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, false, 2)}, 1, ByNewest, []cube.RFAddress{radA}},
		{"the master", radA, []Member{m(wall, cube.DeviceWallThermostat, cube.ModeManual, 23, false, 0),
			m(radA, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, false, 0)}, 1, ByMaster, []cube.RFAddress{radA}},
		{"the wall thermostat", 0, []Member{m(radA, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, false, 0),
			m(wall, cube.DeviceWallThermostat, cube.ModeManual, 23, false, 0)}, 1, ByWall, []cube.RFAddress{wall}},
		{"the first radiator thermostat", 0, []Member{m(radB, cube.DeviceRadiatorThermostat, cube.ModeManual, 19, false, 0),
			m(radA, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, false, 0)}, 0, ByRadiator, []cube.RFAddress{radB}},
	} {
		p, ok := State(c.master, c.ms)
		if !ok || p.Index != c.index || p.Rule != c.rule || !slices.Equal(p.From, c.from) {
			t.Errorf("%s: %+v %v, want %d %s %v", c.name, p, ok, c.index, c.rule, c.from)
		}
	}
	if p, ok := State(wall, nil); ok {
		t.Errorf("no member: %+v", p)
	}
}

// A thermostat is judged by its programme only in auto, with a setpoint,
// a programme and the Cube's time; it follows it where its setpoint is
// the programme's temperature within Grace.
func TestFollowing(t *testing.T) {
	var w cube.WeekProgram
	for d := range w {
		w[d] = cube.DayProgram{{Temp: 17, Until: 6 * 60}, {Temp: 21, Until: 24 * 60}}
	}
	at := time.Date(2026, 11, 24, 5, 57, 0, 0, time.UTC) // three minutes before the switch point to 21 °C
	sp := func(v float64) *float64 { return &v }
	for _, c := range []struct {
		name            string
		st              cube.DeviceState
		w               *cube.WeekProgram
		clock           bool
		judged, follows bool
	}{
		{"auto at the programme's", cube.DeviceState{Mode: cube.ModeAuto, Setpoint: sp(17)}, &w, true, true, true},
		{"auto at the next switch point's, within Grace", cube.DeviceState{Mode: cube.ModeAuto, Setpoint: sp(21)}, &w, true, true, true},
		{"auto off its programme", cube.DeviceState{Mode: cube.ModeAuto, Setpoint: sp(19)}, &w, true, true, false},
		{"manual", cube.DeviceState{Mode: cube.ModeManual, Setpoint: sp(17)}, &w, true, false, false},
		{"the Cube's time not known", cube.DeviceState{Mode: cube.ModeAuto, Setpoint: sp(17)}, &w, false, false, false},
		{"no programme", cube.DeviceState{Mode: cube.ModeAuto, Setpoint: sp(17)}, nil, true, false, false},
	} {
		if judged, follows := Following(c.st, c.w, at, c.clock); judged != c.judged || follows != c.follows {
			t.Errorf("%s: judged %v follows %v, want %v %v", c.name, judged, follows, c.judged, c.follows)
		}
	}
}
