// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// A device whose entry the Cube marks out of date, the valid bit cleared
// and 0x02 kept, is not reported and has no state, but its last report's
// values as last_known, marked out of date, with its last seen (AP
// LegacyLogs v1.0, Block 13).
func TestOutOfDateDevice(t *testing.T) {
	t.Parallel()
	r := startServer(t, storedRooms())
	fc, ts := r.fc, r.ts
	const wall = 0x0a0201
	var d deviceDetail
	testkit.Eventually(t, "the wall thermostat's state", func() bool {
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/0a0201", &d) == 200 && d.Reported && d.State != nil
	})
	setpoint := *d.State.Setpoint
	no := false
	if err := fc.SetState(wall, fakecube.State{Reported: &no}); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "out of date", func() bool {
		d = deviceDetail{} // a field the answer leaves out is not kept from the last one
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/0a0201", &d) == 200 && !d.Reported
	})
	if d.State != nil || !d.OutOfDate || d.LastKnown == nil || d.LastKnown.Setpoint == nil || *d.LastKnown.Setpoint != setpoint || d.LastSeen == nil {
		t.Fatalf("out of date: %+v", d.Device)
	}
}
