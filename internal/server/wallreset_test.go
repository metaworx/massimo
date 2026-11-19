// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"fmt"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/view"
)

// eventText is an event of the server's stream as a failure shows it.
func eventText(ev event) string {
	return fmt.Sprintf("%s %s %v", ev.Kind, ev.Device, ev.Summary)
}

// A wall thermostat that comes back after a silence on another mode is
// a wall_reset in the log, as after new batteries (AP LegacyLogs v1.0,
// Block 12); one that comes back on its mode is not.
//
// Each step waits for the API and then for the pump (pumpSync): the API
// shows the client's snapshot, which the pump reads only when it comes
// to the event, so a pump behind a poll would take two steps' states
// for one and never see the step between them.
func TestWallReset(t *testing.T) {
	swapping(t)
	old := wallSilence.Load()
	wallSilence.Store(0)
	t.Cleanup(func() { wallSilence.Store(old) })
	r := startServer(t, storedRooms())
	fc, ts, s := r.fc, r.ts, r.s
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
	// seen is what the stream carried, taken after each pumpSync: the
	// pump publishes in its turn, so every event of the steps so far is
	// in the channel by then.
	var seen []event
	drain := func() {
		for {
			select {
			case ev := <-stream:
				seen = append(seen, ev)
			default:
				return
			}
		}
	}
	// isReset matches the wall thermostat's wall_reset.
	isReset := func(ev event) bool { return ev.Kind == "wall_reset" && ev.Device == "0a0201" }
	// shows reports whether a view of the wall thermostat has the valid
	// bit want and, while reported, the mode and setpoint.
	shows := func(dv view.Device, want bool, mode cube.Mode, setpoint float64) bool {
		if dv.Reported != want {
			return false
		}
		return !want || dv.State != nil && dv.State.Mode == mode && dv.State.Setpoint != nil && *dv.State.Setpoint == setpoint
	}
	// reported sets the wall thermostat's state on the fake Cube, waits
	// until the snapshot shows it and the pump has handled it, and checks
	// that the pump's last state event of it shows it too.
	reported := func(want bool, st fakecube.State, mode cube.Mode, setpoint float64) {
		t.Helper()
		st.Reported = &want
		if err := fc.SetState(wall, st); err != nil {
			t.Fatal(err)
		}
		testkit.Eventually(t, "the wall thermostat's state in the snapshot", func() bool {
			return shows(view.DeviceOf(rt.client.Snapshot(), wall, s.staleness()), want, mode, setpoint)
		})
		rt.pumpSync(t)
		drain()
		var last *event
		for i := range seen {
			if seen[i].Kind == "state" && seen[i].Device == "0a0201" {
				last = &seen[i]
			}
		}
		if last == nil {
			t.Fatal("no state event of the wall thermostat")
		}
		if dv, ok := last.State.(view.Device); !ok || !shows(dv, want, mode, setpoint) {
			t.Fatalf("the wall thermostat's last state event: %s %+v", eventText(*last), last.State)
		}
	}
	manual, auto := "manual", "auto"
	six, low := 6.0, 5.5
	reported(true, fakecube.State{Mode: &manual, Setpoint: &six}, cube.ModeManual, 6)
	reported(false, fakecube.State{}, 0, 0)
	reported(true, fakecube.State{}, cube.ModeManual, 6)
	reported(false, fakecube.State{}, 0, 0)
	for _, ev := range seen {
		if isReset(ev) {
			t.Fatalf("a wall_reset for a return on the same mode: %+v", ev.Summary)
		}
	}
	before := len(seen)
	reported(true, fakecube.State{Mode: &auto, Setpoint: &low}, cube.ModeAuto, 5.5)
	var reset *event
	for i := before; i < len(seen); i++ {
		if isReset(seen[i]) {
			reset = &seen[i]
			break
		}
	}
	if reset == nil {
		t.Fatal("no wall_reset for a return on another mode")
	}
	if sum := reset.Summary; sum["mode"] != "auto" || sum["was_mode"] != "manual" || sum["setpoint"] != 5.5 || sum["was_setpoint"] != 6.0 {
		t.Fatalf("wall_reset summary %v", sum)
	}
	testkit.Eventually(t, "the wall_reset in the log", func() bool { return hasKind(logKinds(t, ts, 0), "wall_reset:0a0201") })
}
