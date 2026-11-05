// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"fmt"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// StateTimes says how old a device's state values are. The Cube repeats
// every value in each L line and keeps no times itself, so the times
// are those at which the snapshot saw a value appear: a value that
// changed, or that was there when the server first connected. A device
// that reports the same value again leaves its time alone, and a time
// from the first connection means the value may be older. A reconnect
// keeps the times of values that did not change.
type StateTimes struct {
	// Changed is when the device's L entry last changed in any way: the
	// closest a client gets to the device's last transmission. The Cube
	// also changes the setpoint in the entry itself right after a
	// set-temperature command.
	Changed time.Time
	// Fields is, per value (named as in the API's state object: mode,
	// setpoint, actual_temp, valve_pct, window_open, until, battery_low,
	// error, link_error, panel_locked, display_actual), when its current
	// value appeared. Only the values the device has are present.
	Fields map[string]time.Time
}

// stateValues renders the values of a state for comparison, keyed by
// their API names; an entry without valid state has none.
func stateValues(st cube.DeviceState) map[string]string {
	v := map[string]string{}
	if !st.Valid {
		return v
	}
	v["mode"] = st.Mode.String()
	v["battery_low"] = fmt.Sprint(st.BatteryLow)
	v["error"] = fmt.Sprint(st.Error)
	v["link_error"] = fmt.Sprint(st.LinkError)
	v["panel_locked"] = fmt.Sprint(st.PanelLocked)
	if st.Setpoint != nil {
		v["setpoint"] = fmt.Sprint(*st.Setpoint)
	}
	if st.ActualTemp != nil {
		v["actual_temp"] = fmt.Sprint(*st.ActualTemp)
	}
	if st.ValvePct != nil {
		v["valve_pct"] = fmt.Sprint(*st.ValvePct)
	}
	if st.WindowOpen != nil {
		v["window_open"] = fmt.Sprint(*st.WindowOpen)
	}
	if st.Until != nil {
		v["until"] = st.Until.String()
	}
	if st.DisplayActual != nil {
		v["display_actual"] = fmt.Sprint(*st.DisplayActual)
	}
	return v
}

// noteTimesLocked updates the times of a device from its previous
// state (had says whether there was one) to its new one.
func (s *Snapshot) noteTimesLocked(old cube.DeviceState, had bool, st cube.DeviceState, now time.Time) {
	t, ok := s.times[st.RFAddress]
	if !ok {
		t = &StateTimes{Fields: map[string]time.Time{}}
		s.times[st.RFAddress] = t
	}
	if had && ok && sameState(old, st) {
		return
	}
	t.Changed = now
	var before map[string]string
	if had {
		before = stateValues(old)
	}
	after := stateValues(st)
	for k, v := range after {
		if _, seen := t.Fields[k]; !seen || before[k] != v {
			t.Fields[k] = now
		}
	}
	for k := range t.Fields {
		if _, ok := after[k]; !ok {
			delete(t.Fields, k)
		}
	}
}

// Times returns the times of a device's state values, or false when the
// Cube sent no state for it.
func (s *Snapshot) Times(rf cube.RFAddress) (StateTimes, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.times[rf]
	if !ok {
		return StateTimes{}, false
	}
	cp := StateTimes{Changed: t.Changed, Fields: make(map[string]time.Time, len(t.Fields))}
	for k, v := range t.Fields {
		cp.Fields[k] = v
	}
	return cp, true
}

// Since is when the snapshot first heard from the Cube: a time equal to
// it in StateTimes means the value was already there and may be older.
func (s *Snapshot) Since() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.since
}
