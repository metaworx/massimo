// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"sync/atomic"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// A wall thermostat whose batteries were out starts again in auto mode,
// on its week programme: on 2026-11-15 one that had been on manual 6 °C
// came back on auto 5.5 °C, and its room's radiator thermostat stayed on
// manual (docs/cube-protocol.md, "How devices report"). So when a wall
// thermostat reports again after wallSilence without a state, on another
// mode than it last had, the log says so at warn, as wall_reset (AP
// LegacyLogs v1.0, Block 12, D147-A): a battery change returns a room to
// its programme without anyone asking.

// wallSilence is how long a wall thermostat has to be without a state
// for its return to count, in nanoseconds; the Cube clears its valid bit
// after about five minutes of silence. Atomic, for the tests.
var wallSilence atomic.Int64

func init() { wallSilence.Store(int64(5 * time.Minute)) }

// wallMemo is what a wall thermostat last reported, and since when it
// has no state.
type wallMemo struct {
	mode     cube.Mode
	setpoint *float64
	silent   time.Time // zero while it has a state
}

// wallReset takes a wall thermostat's view at the time at and returns
// the wall_reset event where it came back on another mode; nil
// otherwise. Only the pump calls it.
func (rt *runtime) wallReset(dv view.Device, at time.Time) *event {
	if dv.Type != cube.DeviceWallThermostat || dv.Virtual {
		return nil
	}
	rf, err := cube.ParseRFAddress(dv.RFAddress)
	if err != nil {
		return nil
	}
	if rt.walls == nil {
		rt.walls = map[cube.RFAddress]*wallMemo{}
	}
	m := rt.walls[rf]
	if !dv.Reported || dv.State == nil {
		if m != nil && m.silent.IsZero() {
			m.silent = at
		}
		return nil
	}
	var ev *event
	if m != nil && !m.silent.IsZero() && at.Sub(m.silent) >= time.Duration(wallSilence.Load()) && dv.State.Mode != m.mode {
		summary := map[string]any{"mode": dv.State.Mode.String(), "was_mode": m.mode.String(), "silent_min": int(at.Sub(m.silent).Minutes())}
		if dv.State.Setpoint != nil {
			summary["setpoint"] = *dv.State.Setpoint
		}
		if m.setpoint != nil {
			summary["was_setpoint"] = *m.setpoint
		}
		ev = &event{Kind: "wall_reset", Source: "device", Device: dv.RFAddress, Serial: dv.Serial, Level: "warn", Time: at, State: dv, Summary: summary}
	}
	rt.walls[rf] = &wallMemo{mode: dv.State.Mode, setpoint: dv.State.Setpoint}
	return ev
}
