// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/view"
)

// A wall thermostat that comes back after a silence on another mode is
// a wall_reset in the log, as after new batteries (AP LegacyLogs v1.0,
// Block 12); one that comes back on its mode is not.
//
// Each step waits for the pump's state event, not for the API: the API
// shows the client's snapshot, which the pump reads only when it comes
// to the event, so a pump behind a poll would take two steps' states
// for one and never see the step between them.
func TestWallReset(t *testing.T) {
	old := wallSilence.Load()
	wallSilence.Store(0)
	t.Cleanup(func() { wallSilence.Store(old) })
	fc, ts, s, _ := startRooms(t)
	const wall = 0x0a0201
	s.mu.RLock()
	rt := s.cubes["keq0000100"]
	s.mu.RUnlock()
	stream := make(chan event, 1024)
	rt.subsMu.Lock()
	rt.subs[stream] = &subscriber{gone: make(chan struct{})}
	rt.subsMu.Unlock()
	t.Cleanup(func() {
		rt.subsMu.Lock()
		delete(rt.subs, stream)
		rt.subsMu.Unlock()
	})
	var resets []event // the wall thermostat's wall_resets, as the stream had them
	next := func(what string, ok func(event) bool) {
		t.Helper()
		timeout := time.After(testkit.Scale(5 * time.Second))
		for {
			select {
			case ev := <-stream:
				if ev.Kind == "wall_reset" && ev.Device == "0a0201" {
					resets = append(resets, ev)
				}
				if ok(ev) {
					return
				}
			case <-timeout:
				t.Fatalf("timed out waiting for %s", what)
			}
		}
	}
	// reported sets the wall thermostat's state on the fake Cube and
	// waits until the pump has handled a view of it with the valid bit
	// want and, while reported, the mode and setpoint.
	reported := func(want bool, st fakecube.State, mode cube.Mode, setpoint float64) {
		t.Helper()
		st.Reported = &want
		if err := fc.SetState(wall, st); err != nil {
			t.Fatal(err)
		}
		next("the wall thermostat's state", func(ev event) bool {
			dv, ok := ev.State.(view.Device)
			if ev.Kind != "state" || ev.Device != "0a0201" || !ok || dv.Reported != want {
				return false
			}
			return !want || dv.State != nil && dv.State.Mode == mode && dv.State.Setpoint != nil && *dv.State.Setpoint == setpoint
		})
	}
	manual, auto := "manual", "auto"
	six, low := 6.0, 5.5
	reported(true, fakecube.State{Mode: &manual, Setpoint: &six}, cube.ModeManual, 6)
	reported(false, fakecube.State{}, 0, 0)
	reported(true, fakecube.State{}, cube.ModeManual, 6)
	// The pump emits a return's wall_reset right after its state event,
	// so the next silence's state event comes after it.
	reported(false, fakecube.State{}, 0, 0)
	if len(resets) != 0 {
		t.Fatalf("a wall_reset for a return on the same mode: %+v", resets[0].Summary)
	}
	reported(true, fakecube.State{Mode: &auto, Setpoint: &low}, cube.ModeAuto, 5.5)
	next("the wall_reset", func(event) bool { return len(resets) > 0 })
	if sum := resets[0].Summary; sum["mode"] != "auto" || sum["was_mode"] != "manual" || sum["setpoint"] != 5.5 || sum["was_setpoint"] != 6.0 {
		t.Fatalf("wall_reset summary %v", sum)
	}
	testkit.Eventually(t, "the wall_reset in the log", func() bool { return hasKind(logKinds(t, ts, 0), "wall_reset:0a0201") })
}
