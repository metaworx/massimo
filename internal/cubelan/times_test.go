// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"encoding/base64"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// entry is an L line with one radiator thermostat entry for 0a010d:
// valid, manual, setpoint and valve as given, measured temperature in
// tenths (0 = not reported).
func entry(setpoint float64, valve int, actualTenths int) cube.Message {
	b := []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12, 0x01, byte(valve), byte(setpoint * 2), byte(actualTenths >> 8), byte(actualTenths), 0}
	return cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(b)}
}

func TestStateTimes(t *testing.T) {
	t.Parallel()
	lines, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)
	s := newSnapshot()
	for _, m := range lines {
		if err := s.Apply(m, t0); err != nil {
			t.Fatal(err)
		}
	}
	if s.Since() != t0 {
		t.Fatalf("since %v", s.Since())
	}
	// Not reported yet: a time for the entry, none for values.
	tm, ok := s.Times(0x0a010d)
	if !ok || tm.Changed != t0 || len(tm.Fields) != 0 {
		t.Fatalf("before the report %+v %v", tm, ok)
	}
	t1 := t0.Add(time.Hour)
	s.Apply(entry(21, 30, 205), t1)
	tm, _ = s.Times(0x0a010d)
	for _, k := range []string{"mode", "setpoint", "valve_pct", "actual_temp", "battery_low"} {
		if tm.Fields[k] != t1 {
			t.Errorf("%s at %v", k, tm.Fields[k])
		}
	}
	// The same entry again moves nothing.
	t2 := t1.Add(time.Hour)
	s.Apply(entry(21, 30, 205), t2)
	if tm, _ = s.Times(0x0a010d); tm.Changed != t1 || tm.Fields["setpoint"] != t1 {
		t.Fatalf("repeated entry %+v", tm)
	}
	// The valve moves and the temperature is not reported: only the
	// valve is new; the measured temperature stays, with its time.
	t3 := t2.Add(time.Hour)
	s.Apply(entry(21, 50, 0), t3)
	tm, _ = s.Times(0x0a010d)
	if tm.Changed != t3 || tm.Fields["valve_pct"] != t3 || tm.Fields["setpoint"] != t1 || tm.Fields["mode"] != t1 {
		t.Fatalf("valve change %+v", tm)
	}
	if st, _ := s.State(0x0a010d); st.ActualTemp == nil || *st.ActualTemp != 20.5 || tm.Fields["actual_temp"] != t1 {
		t.Fatalf("the last temperature is not kept: %+v at %v", st.ActualTemp, tm.Fields["actual_temp"])
	}
	// A reconnect keeps the times of unchanged values.
	s.beginGreeting()
	for _, m := range lines {
		if m.Type != 'L' {
			s.Apply(m, t3.Add(time.Hour))
		}
	}
	l := lines[len(lines)-1]
	raw, _ := base64.StdEncoding.DecodeString(l.Data)
	mine, _ := base64.StdEncoding.DecodeString(entry(21, 50, 0).Data)
	// Replace 0a010d's entry, the first in the fixture's L line.
	raw = append(mine, raw[12:]...)
	s.Apply(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)}, t3.Add(time.Hour))
	if tm, _ = s.Times(0x0a010d); tm.Changed != t3 || tm.Fields["valve_pct"] != t3 {
		t.Fatalf("after the reconnect %+v", tm)
	}
	if _, ok := s.Times(0xabcdef); ok {
		t.Fatal("times for an unknown device")
	}
}

// The greeting's lines arrive one after the other, the L line last; its
// values still carry exactly the time of the first line, the Cube's
// observed_since, while a later L line carries its own time.
func TestFirstDeviceListCarriesObservedSince(t *testing.T) {
	t.Parallel()
	lines, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)
	s := newSnapshot()
	s.beginGreeting()
	at := t0
	for _, m := range lines {
		if m.Type == 'L' {
			// 0a010d, the first entry, as reported, so that it has values.
			raw, _ := base64.StdEncoding.DecodeString(m.Data)
			mine, _ := base64.StdEncoding.DecodeString(entry(21, 30, 205).Data)
			m.Data = base64.StdEncoding.EncodeToString(append(mine, raw[12:]...))
		}
		if _, err := s.apply(m, at); err != nil {
			t.Fatal(err)
		}
		at = at.Add(300 * time.Microsecond)
	}
	if s.Since() != t0 {
		t.Fatalf("since %v", s.Since())
	}
	withValues := 0
	for rf := range s.states {
		tm, ok := s.Times(rf)
		if !ok || tm.Changed != t0 {
			t.Errorf("%s changed at %v", rf, tm.Changed)
		}
		for k, v := range tm.Fields {
			if v != t0 {
				t.Errorf("%s %s at %v", rf, k, v)
			}
		}
		if len(tm.Fields) > 0 {
			withValues++
		}
	}
	if withValues == 0 {
		t.Fatal("the fixture has no device with values")
	}
	t1 := t0.Add(time.Hour)
	s.Apply(entry(21, 50, 205), t1)
	if tm, _ := s.Times(0x0a010d); tm.Changed != t1 || tm.Fields["valve_pct"] != t1 || tm.Fields["setpoint"] != t0 {
		t.Fatalf("later device list %+v", tm)
	}
}

// A thermostat keeps its last measured temperature and its time while its
// entries report none, on vacation too, until a new one comes; a device
// forgotten forgets it.
func TestLastTemperatureKept(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)
	s := newSnapshot()
	lines, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range lines {
		s.Apply(m, t0)
	}
	actual := func() (*float64, time.Time) {
		st, _ := s.State(0x0a010d)
		tm, _ := s.Times(0x0a010d)
		return st.ActualTemp, tm.Fields["actual_temp"]
	}
	t1 := t0.Add(time.Hour)
	s.Apply(entry(21, 30, 0), t1)
	if v, _ := actual(); v != nil {
		t.Fatalf("a temperature before any was reported: %v", *v)
	}
	t2 := t1.Add(time.Hour)
	s.Apply(entry(21, 30, 198), t2)
	// Vacation: the bytes carry the end date, 2027-01-06 10:00.
	vac := []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12, 0x02, 30, 30, 0x06, 0x1b, 20}
	s.Apply(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(vac)}, t2.Add(time.Hour))
	st, _ := s.State(0x0a010d)
	if v, at := actual(); v == nil || *v != 19.8 || at != t2 || st.Mode != cube.ModeVacation || st.Until == nil {
		t.Fatalf("on vacation: %v at %v, %+v", v, at, st)
	}
	t4 := t2.Add(3 * time.Hour)
	s.Apply(entry(21, 30, 211), t4)
	if v, at := actual(); v == nil || *v != 21.1 || at != t4 {
		t.Fatalf("a new value: %v at %v", v, at)
	}
	s.Apply(entry(21, 40, 0), t4.Add(time.Hour))
	if v, at := actual(); v == nil || *v != 21.1 || at != t4 {
		t.Fatalf("kept after the new value: %v at %v", v, at)
	}
	s.Forget(0x0a010d)
	if _, ok := s.lastTemp[0x0a010d]; ok {
		t.Fatal("a forgotten device's temperature is kept")
	}
}

// Which values' times are a change from an earlier value made by
// someone (StateTimes.Changes, AP RoomMajority D3-A): not a first
// sighting, nor a value back after the entry had none, nor the mode and
// setpoint with which a boost ends or a holiday ends at its until; a
// holiday left by hand before its end is. Fields' times are the same
// either way.
func TestStateTimesChanges(t *testing.T) {
	t.Parallel()
	const rf cube.RFAddress = 0x0a010d
	s := newSnapshot()
	state := func(mode cube.Mode, set float64, until *cube.Until) cube.DeviceState {
		return cube.DeviceState{RFAddress: rf, Type: cube.DeviceRadiatorThermostat, Valid: true, Mode: mode, Setpoint: &set, Until: until}
	}
	var last cube.DeviceState
	had, n := false, 0
	note := func(st cube.DeviceState, at time.Time) StateTimes {
		n++
		st.Raw = []byte{byte(n)} // an entry that differs from the one before
		s.noteTimesLocked(last, had, st, at)
		last, had = st, true
		tm, _ := s.Times(rf)
		return tm
	}
	// want checks the changes of the values named, and the time of
	// those that are a change.
	want := func(step string, tm StateTimes, at time.Time, changes map[string]bool) {
		t.Helper()
		for k, c := range changes {
			if tm.Changes[k] != c {
				t.Errorf("%s: %s a change %v, want %v (%+v)", step, k, tm.Changes[k], c, tm.Changes)
			}
			if c && tm.Fields[k] != at {
				t.Errorf("%s: %s at %v, want %v", step, k, tm.Fields[k], at)
			}
		}
	}
	t0 := time.Date(2026, 11, 23, 8, 0, 0, 0, time.Local)
	at := func(h int) time.Time { return t0.Add(time.Duration(h) * time.Hour) }
	tm := note(state(cube.ModeManual, 21, nil), at(0))
	want("the first sighting", tm, at(0), map[string]bool{"mode": false, "setpoint": false, "valve_pct": false})
	if tm.Fields["mode"] != at(0) || tm.Fields["setpoint"] != at(0) {
		t.Fatalf("a first sighting's times: %+v", tm.Fields)
	}
	want("a setpoint set", note(state(cube.ModeManual, 22, nil), at(1)), at(1), map[string]bool{"mode": false, "setpoint": true})
	want("auto set", note(state(cube.ModeAuto, 22, nil), at(2)), at(2), map[string]bool{"mode": true})
	want("a boost pressed", note(state(cube.ModeBoost, 22, nil), at(3)), at(3), map[string]bool{"mode": true})
	tm = note(state(cube.ModeAuto, 22, nil), at(4))
	want("the boost's end", tm, at(4), map[string]bool{"mode": false})
	if tm.Fields["mode"] != at(4) {
		t.Fatalf("the boost's end moved no time: %+v", tm.Fields)
	}
	end := cube.UntilFromTime(at(6))
	want("a holiday set", note(state(cube.ModeVacation, 15, &end), at(5)), at(5), map[string]bool{"mode": true, "setpoint": true, "until": false})
	tm = note(state(cube.ModeAuto, 21, nil), at(6).Add(2*time.Minute))
	want("the holiday's end at its until", tm, at(6), map[string]bool{"mode": false, "setpoint": false})
	if _, ok := tm.Changes["until"]; ok {
		t.Fatalf("an until gone keeps its change: %+v", tm.Changes)
	}
	later := cube.UntilFromTime(at(30))
	note(state(cube.ModeVacation, 15, &later), at(7))
	want("a holiday left by hand", note(state(cube.ModeManual, 20, nil), at(8)), at(8), map[string]bool{"mode": true, "setpoint": true})
	tm = note(cube.DeviceState{RFAddress: rf, Type: cube.DeviceRadiatorThermostat}, at(9)) // the Cube restarted
	if len(tm.Fields) != 0 || len(tm.Changes) != 0 {
		t.Fatalf("an entry without values: %+v", tm)
	}
	want("values back", note(state(cube.ModeManual, 19, nil), at(10)), at(10), map[string]bool{"mode": false, "setpoint": false})

	// A boost whose entry shows another setpoint ends with the one from
	// before it: no one's; a boost ended by turning the wheel to a new
	// setpoint is a change.
	want("a boost pressed again", note(state(cube.ModeBoost, 30, nil), at(11)), at(11), map[string]bool{"mode": true, "setpoint": true})
	want("its end at the setpoint from before", note(state(cube.ModeManual, 19, nil), at(12)), at(12), map[string]bool{"mode": false, "setpoint": false})
	note(state(cube.ModeBoost, 19, nil), at(13))
	want("a boost ended at the wheel", note(state(cube.ModeManual, 24, nil), at(14)), at(14), map[string]bool{"mode": false, "setpoint": true})

	// With the Cube's time known, a holiday's end is read on its clock,
	// here 2026-11-03 19:51 at the H line, the host's clock days later:
	// a holiday whose end has passed for the host but not for the Cube
	// is left by hand; one whose end has passed for the Cube ends by
	// itself.
	s.hello, s.helloAt = &cube.Hello{CubeDate: "2026-11-03", CubeTime: "19:51"}, at(15)
	hostPast := cube.Until{Year: 2026, Month: 11, Day: 20, Minutes: 12 * 60}
	note(state(cube.ModeVacation, 15, &hostPast), at(15))
	want("a holiday left before its end on the Cube's clock", note(state(cube.ModeManual, 20, nil), at(16)), at(16), map[string]bool{"mode": true, "setpoint": true})
	cubePast := cube.Until{Year: 2026, Month: 11, Day: 3, Minutes: 22 * 60}
	note(state(cube.ModeVacation, 15, &cubePast), at(17))
	want("a holiday's end on the Cube's clock", note(state(cube.ModeAuto, 21, nil), at(18)), at(18), map[string]bool{"mode": false, "setpoint": false})
}
