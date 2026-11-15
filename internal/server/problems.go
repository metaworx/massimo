// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import "go.metaworx.ch/massimo/internal/view"

// cubeProblem is a device of a Cube with a problem, as GET /api/v1/cubes
// lists it: the device as the device list shows it, with its room's name.
type cubeProblem struct {
	view.Device
	Room string `json:"room,omitempty"`
}

// problemsOf lists a Cube's visible devices with a problem, in the order
// the Cube reported them: a transmit or link error, a low battery, a
// window open (with its times, view.Device SetWindow). A device that has
// not reported since the Cube's restart has none yet. The state is the
// server's last, also while the Cube is not connected.
func (s *Server) problemsOf(rt *runtime) []cubeProblem {
	snap := rt.client.Snapshot()
	rooms := map[uint8]string{}
	if md := snap.Metadata(); md != nil {
		for _, r := range md.Rooms {
			rooms[r.ID] = r.Name
		}
	}
	var out []cubeProblem
	for _, d := range rt.withBattery(s.withWindow(view.Devices(snap, rt.hidden(), s.staleness()))) {
		st := d.State
		if !d.Reported || st == nil {
			continue
		}
		open := st.WindowOpen != nil && *st.WindowOpen
		if st.Error || st.LinkError || st.BatteryLow || open {
			out = append(out, cubeProblem{Device: d, Room: rooms[d.RoomID]})
		}
	}
	return out
}
