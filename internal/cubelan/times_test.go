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

// entry is an L line with one radiator thermostat entry for 1514b6:
// valid, manual, setpoint and valve as given, measured temperature in
// tenths (0 = not reported).
func entry(setpoint float64, valve int, actualTenths int) cube.Message {
	b := []byte{0x0b, 0x15, 0x14, 0xb6, 0x09, 0x12, 0x01, byte(valve), byte(setpoint * 2), byte(actualTenths >> 8), byte(actualTenths), 0}
	return cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(b)}
}

func TestStateTimes(t *testing.T) {
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
	tm, ok := s.Times(0x1514b6)
	if !ok || tm.Changed != t0 || len(tm.Fields) != 0 {
		t.Fatalf("before the report %+v %v", tm, ok)
	}
	t1 := t0.Add(time.Hour)
	s.Apply(entry(21, 30, 205), t1)
	tm, _ = s.Times(0x1514b6)
	for _, k := range []string{"mode", "setpoint", "valve_pct", "actual_temp", "battery_low"} {
		if tm.Fields[k] != t1 {
			t.Errorf("%s at %v", k, tm.Fields[k])
		}
	}
	// The same entry again moves nothing.
	t2 := t1.Add(time.Hour)
	s.Apply(entry(21, 30, 205), t2)
	if tm, _ = s.Times(0x1514b6); tm.Changed != t1 || tm.Fields["setpoint"] != t1 {
		t.Fatalf("repeated entry %+v", tm)
	}
	// The valve moves and the temperature is not reported: only the
	// valve is new, the measured temperature disappears.
	t3 := t2.Add(time.Hour)
	s.Apply(entry(21, 50, 0), t3)
	tm, _ = s.Times(0x1514b6)
	if tm.Changed != t3 || tm.Fields["valve_pct"] != t3 || tm.Fields["setpoint"] != t1 || tm.Fields["mode"] != t1 {
		t.Fatalf("valve change %+v", tm)
	}
	if _, ok := tm.Fields["actual_temp"]; ok {
		t.Fatal("actual_temp kept although the entry has none")
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
	// Replace 1514b6's entry, the first in the fixture's L line.
	raw = append(mine, raw[12:]...)
	s.Apply(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)}, t3.Add(time.Hour))
	if tm, _ = s.Times(0x1514b6); tm.Changed != t3 || tm.Fields["valve_pct"] != t3 {
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
			// 1514b6, the first entry, as reported, so that it has values.
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
	if tm, _ := s.Times(0x1514b6); tm.Changed != t1 || tm.Fields["valve_pct"] != t1 || tm.Fields["setpoint"] != t0 {
		t.Fatalf("later device list %+v", tm)
	}
}
