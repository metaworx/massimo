// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"go.metaworx.ch/massimo/internal/battery"
	"go.metaworx.ch/massimo/internal/view"
)

// cubeProblem is a device of a Cube with a problem, as GET /api/v1/cubes
// lists it: the device as the device list shows it, with its room's name.
type cubeProblem struct {
	view.Device
	Room string `json:"room,omitempty"`
}

// problemsOf lists a Cube's visible devices with a problem, in the order
// the Cube reported them: a communication or link error, a battery
// problem (batteryProblem), a window open (with its times, view.Device
// SetWindow). A device that has not reported since the Cube's restart
// has none yet. The state is the server's last, also while the Cube is
// not connected.
func (s *Server) problemsOf(rt *runtime) []cubeProblem {
	return s.devicesWhere(rt, func(d view.Device) bool {
		st := d.State
		open := st.WindowOpen != nil && *st.WindowOpen
		return st.Error || st.LinkError || batteryProblem(d) || open
	})
}

// flaggedOf lists a Cube's visible devices with a flag of deviceFlags
// set, as problemsOf does, whatever their battery level: the log's
// alerts go by the Cube's flags (logalerts.go).
func (s *Server) flaggedOf(rt *runtime) []cubeProblem {
	return s.devicesWhere(rt, func(d view.Device) bool {
		return d.State.Error || d.State.LinkError || d.State.BatteryLow
	})
}

// devicesWhere lists a Cube's visible devices that have reported since
// the Cube's restart and that keep takes, in the order the Cube reported
// them, each with its room's name.
func (s *Server) devicesWhere(rt *runtime, keep func(view.Device) bool) []cubeProblem {
	snap := rt.client.Snapshot()
	rooms := map[uint8]string{}
	if md := snap.Metadata(); md != nil {
		for _, r := range md.Rooms {
			rooms[r.ID] = r.Name
		}
	}
	var out []cubeProblem
	for _, d := range rt.withBattery(s.withWindow(view.Devices(snap, rt.hidden(), s.staleness()))) {
		if !d.Reported || d.State == nil || !keep(d) {
			continue
		}
		out = append(out, cubeProblem{Device: d, Room: rooms[d.RoomID]})
	}
	return out
}

// batteryProblem reports whether a device's battery is a problem: its
// level is weak or low, the level the server keeps (internal/battery)
// and the device rows and tiles show, whatever the Cube's flag says
// now; for a device the server keeps no level for, the flag, as the
// web UI's battery mark does (ui/src/model/device.ts).
func batteryProblem(d view.Device) bool {
	switch battery.Level(d.Battery) {
	case battery.Weak, battery.Low:
		return true
	case "":
		return d.State != nil && d.State.BatteryLow
	}
	return false
}
