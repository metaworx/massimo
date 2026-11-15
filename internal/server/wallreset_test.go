// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// A wall thermostat that comes back after a silence on another mode is
// a wall_reset in the log, as after new batteries (AP LegacyLogs v1.0,
// Block 12); one that comes back on its mode is not.
func TestWallReset(t *testing.T) {
	old := wallSilence.Load()
	wallSilence.Store(0)
	t.Cleanup(func() { wallSilence.Store(old) })
	fc, ts, _, _ := startRooms(t)
	const wall = 0x0a0201
	set := func(st fakecube.State) {
		t.Helper()
		if err := fc.SetState(wall, st); err != nil {
			t.Fatal(err)
		}
	}
	reported := func(want bool) {
		t.Helper()
		waitFor(t, "the wall thermostat's state", func() bool {
			var d deviceDetail
			return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/0a0201", &d) == 200 && d.Reported == want
		})
	}
	yes, no := true, false
	manual, auto := "manual", "auto"
	six, low := 6.0, 5.5
	set(fakecube.State{Mode: &manual, Setpoint: &six})
	waitFor(t, "manual 6", func() bool {
		var d deviceDetail
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/0a0201", &d) == 200 && d.State != nil && d.State.Setpoint != nil && *d.State.Setpoint == 6
	})
	set(fakecube.State{Reported: &no})
	reported(false)
	set(fakecube.State{Reported: &yes})
	reported(true)
	if hasKind(logKinds(t, ts, 0), "wall_reset:0a0201") {
		t.Fatal("a wall_reset for a return on the same mode")
	}
	set(fakecube.State{Reported: &no})
	reported(false)
	set(fakecube.State{Reported: &yes, Mode: &auto, Setpoint: &low})
	waitFor(t, "the wall_reset", func() bool { return hasKind(logKinds(t, ts, 0), "wall_reset:0a0201") })
}
