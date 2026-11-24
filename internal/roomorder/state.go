// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package roomorder

import (
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Grace is how far from the Cube's time a week programme's temperature
// still counts as the one a thermostat follows: a switch point just
// past, or a clock a few minutes off.
const Grace = 5 * time.Minute

// Program is the week programme of a thermostat's configuration (its C
// line), nil for another device's or none.
func Program(c *cube.DeviceConfig) *cube.WeekProgram {
	switch {
	case c == nil:
	case c.Thermostat != nil:
		return &c.Thermostat.WeekProgram
	case c.Wall != nil:
		return &c.Wall.WeekProgram
	}
	return nil
}

// ProgramDay is w's programme of t's weekday; the devices' week starts
// on Saturday.
func ProgramDay(w *cube.WeekProgram, t time.Time) cube.DayProgram {
	return w[(int(t.Weekday())+1)%7]
}

// ProgramTemp is w's temperature at t, a time of the Cube's clock.
func ProgramTemp(w *cube.WeekProgram, t time.Time) (float64, bool) {
	m := t.Hour()*60 + t.Minute()
	for _, p := range ProgramDay(w, t) {
		if m < p.Until {
			return p.Temp, true
		}
	}
	return 0, false
}

// FollowsAround reports whether setpoint is w's temperature at a minute
// within grace of t.
func FollowsAround(w *cube.WeekProgram, t time.Time, setpoint float64, grace time.Duration) bool {
	for d := -grace; d <= grace; d += time.Minute {
		if temp, ok := ProgramTemp(w, t.Add(d)); ok && temp == setpoint {
			return true
		}
	}
	return false
}

// Following says how a thermostat whose entry is st stands to its week
// programme w at cubeNow, the Cube's time, where clock says it is known:
// judged where it is in auto with a setpoint and a programme and the
// Cube's time is known; follows where it is judged and its setpoint is
// the programme's temperature within Grace of cubeNow.
func Following(st cube.DeviceState, w *cube.WeekProgram, cubeNow time.Time, clock bool) (judged, follows bool) {
	if st.Mode != cube.ModeAuto || st.Setpoint == nil || w == nil || !clock {
		return false, false
	}
	return true, FollowsAround(w, cubeNow, *st.Setpoint, Grace)
}

// ChangedAt is when the value field of times changed to the one it has,
// zero where that is no one's change (cubelan.StateTimes.Changes): a
// value's first sighting, or the end of a boost or of a holiday at its
// until.
func ChangedAt(times cubelan.StateTimes, field string) time.Time {
	if !times.Changes[field] {
		return time.Time{}
	}
	return times.Fields[field]
}

// Member is a thermostat of a room as the room's state is decided over
// it: one the Cube holds a valid state for (not Dead), with a setpoint.
type Member struct {
	RF   cube.RFAddress
	Type cube.DeviceType
	// State is its entry, of which its mode, setpoint and until count.
	State cube.DeviceState
	// Follows says it is in auto at its week programme's temperature
	// (Following).
	Follows bool
	// ModeAt and SetpointAt are when its mode and its setpoint changed
	// to their values by someone's hand (ChangedAt), zero where that is
	// not known or no one's.
	ModeAt, SetpointAt time.Time
}

// SameSetting reports whether two entries agree on mode, setpoint and
// until: the rule of a virtual wall thermostat's Consistent and of the
// room checks' comparison of a room's members.
func SameSetting(a, b cube.DeviceState) bool {
	eqF := func(x, y *float64) bool { return x == nil && y == nil || x != nil && y != nil && *x == *y }
	eqU := func(x, y *cube.Until) bool { return x == nil && y == nil || x != nil && y != nil && *x == *y }
	return a.Mode == b.Mode && eqF(a.Setpoint, b.Setpoint) && eqU(a.Until, b.Until)
}

// Agree reports whether two members show one state: the same mode,
// setpoint and until (SameSetting), or both in auto, each following its
// own programme (RadioHealth.D188), whose difference is their
// configuration's.
func Agree(a, b Member) bool {
	return a.Follows && b.Follows || SameSetting(a.State, b.State)
}

// Rule is the step of State that decided a room's state, as the room
// checks' re-send names it (its summary's from).
type Rule string

// The steps of State, in the order it takes them.
const (
	// ByMajority is the state more than half of the members show.
	ByMajority Rule = "majority"
	// ByNewest is the state of the member whose mode or setpoint changed
	// last.
	ByNewest Rule = "newest"
	// ByMaster, ByWall and ByRadiator are the first of the room's order
	// (Of): its master, its wall thermostat, a radiator thermostat.
	ByMaster   Rule = "master"
	ByWall     Rule = "wall_thermostat"
	ByRadiator Rule = "radiator_thermostat"
)

// Pick is the state State decided: the member whose state it is, Index
// into the members it was given, the Rule that took it, and the members
// it came from, From: those that agree with it where more than half do,
// those of the newest change, else the one.
type Pick struct {
	Index int
	Rule  Rule
	From  []cube.RFAddress
}

// State is the state that speaks for a room by the one order
// (RoomMajority.D9; D3-A and D7-C for its second step), the first of
// these over ms, the room's thermostats with a state in the Cube's
// order, the dead and those in boost left out by the caller, master
// the metadata's master of the room:
//  1. what more than half of ms show (Majority), the members in auto
//     that each follow their own programme counting as one state;
//  2. the state of the member whose mode or setpoint changed last
//     (Newest);
//  3. the state of the first of the room's order among ms (ByOrder):
//     its master where it is one of them, its wall thermostat, its
//     radiator thermostats.
//
// False where ms is empty. Massimo's own command to the room and a
// state held for it before a structure change, which the server alone
// knows, come before it in the room checks' re-send
// (internal/server/drift.go, referenceState); the virtual wall
// thermostat takes it as it is (internal/view).
func State(master cube.RFAddress, ms []Member) (Pick, bool) {
	if p, ok := Majority(ms); ok {
		return p, true
	}
	if p, ok := Newest(ms); ok {
		return p, true
	}
	return ByOrder(master, ms)
}

// Majority is the state more than half of ms show (RadioHealth.D188):
// the first member's in ms' order with which more than half agree
// (Agree, itself counted), the members in auto that each follow their
// own programme as one state; From lists those that agree with it.
// False where no state has more than half.
func Majority(ms []Member) (Pick, bool) {
	for i, m := range ms {
		var from []cube.RFAddress
		for _, o := range ms {
			if Agree(m, o) {
				from = append(from, o.RF)
			}
		}
		if 2*len(from) > len(ms) {
			return Pick{Index: i, Rule: ByMajority, From: from}, true
		}
	}
	return Pick{}, false
}

// Newest is the state of the member whose mode or setpoint changed last
// (RoomMajority.D3-A, as D7-C counts it): a change made by hand at a
// device is the newest wish. A member's time is the later of its mode's
// and its setpoint's, those not known left out (ChangedAt); but where
// any of ms is in manual or on holiday, a member that follows its
// programme counts its mode's time alone, since in auto a setpoint set
// by hand holds only until the programme's next switch point, while in
// manual or on holiday the programme changes nothing. The newest time
// wins strictly: false where no member has a time, or where members
// that do not agree share it. Its state is the first of those members'
// in ms' order, and From lists them all.
func Newest(ms []Member) (Pick, bool) {
	fixed := false // a member in manual or on holiday
	for _, m := range ms {
		fixed = fixed || m.State.Mode == cube.ModeManual || m.State.Mode == cube.ModeVacation
	}
	var best []int
	var at time.Time
	for i, m := range ms {
		t := m.ModeAt
		if !(fixed && m.Follows) && m.SetpointAt.After(t) {
			t = m.SetpointAt
		}
		switch {
		case t.IsZero() || t.Before(at):
		case t.After(at):
			best, at = []int{i}, t
		default:
			best = append(best, i)
		}
	}
	if len(best) == 0 {
		return Pick{}, false
	}
	p := Pick{Index: best[0], Rule: ByNewest}
	for _, i := range best {
		if !Agree(ms[best[0]], ms[i]) {
			return Pick{}, false
		}
		p.From = append(p.From, ms[i].RF)
	}
	return p, true
}

// ByOrder is the state of the first of the room's order among ms (Of,
// RoomMajority.D9, without the dead, whom the caller left out), by the
// rule that took it: its master, its wall thermostat or a radiator
// thermostat; false for none.
func ByOrder(master cube.RFAddress, ms []Member) (Pick, bool) {
	metas := make([]cube.DeviceMeta, 0, len(ms))
	for _, m := range ms {
		metas = append(metas, cube.DeviceMeta{Type: m.Type, RFAddress: m.RF})
	}
	order := Of(nil, master, metas, 0)
	if len(order) == 0 {
		return Pick{}, false
	}
	for i, m := range ms {
		if m.RF != order[0] {
			continue
		}
		p := Pick{Index: i, Rule: ByRadiator, From: []cube.RFAddress{m.RF}}
		switch {
		case m.RF == master:
			p.Rule = ByMaster
		case m.Type == cube.DeviceWallThermostat:
			p.Rule = ByWall
		}
		return p, true
	}
	return Pick{}, false
}
