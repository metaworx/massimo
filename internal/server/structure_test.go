// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// startStructureCube is startCopyCube with the structure changes read
// back within a fraction of a second, and reconnects for the budget
// allowed (AP DeviceUnPairingFix v1.3, Block 3).
func startStructureCube(t *testing.T, mod func(*fakecube.Server)) (*fakecube.Server, *httptest.Server) {
	t.Helper()
	fc := &fakecube.Server{Greeting: copyGreeting(t)}
	if mod != nil {
		mod(fc)
	}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond},
		RefreshInterval: time.Hour, FollowUpDelays: []time.Duration{10 * time.Millisecond}, FollowUpReconnectGap: time.Hour,
		StructureChecks: []time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 300 * time.Millisecond}})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	waitFor(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})
	return fc, ts
}

// structureEvent waits for the structure event of device in the Cube's
// log and returns it.
func structureEvent(t *testing.T, ts *httptest.Server, device string) event {
	t.Helper()
	var found event
	waitFor(t, "the structure event of "+device, func() bool {
		var evs []event
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log?limit=500", &evs)
		for _, e := range evs {
			if e.Kind == "structure" && e.Device == device {
				found = e
				return true
			}
		}
		return false
	})
	return found
}

// A removal answers pending, and is done once the Cube's free slots
// are back at their count before it: the S replies say 49 free, the
// greeting of the reconnect 50, as before the removal.
func TestStructureDone(t *testing.T) {
	_, ts := startStructureCube(t, nil)
	code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0a010a", "")
	if code != 200 || out["confirmation"] != "pending" {
		t.Fatalf("remove: %d %v", code, out)
	}
	ev := structureEvent(t, ts, "0a010a")
	if ev.Level != "info" || ev.Summary["outcome"] != "done" || ev.Summary["action"] != "remove" ||
		ev.Summary["before"] != float64(50) || ev.Summary["free_slots"] != float64(50) {
		t.Fatalf("event %+v", ev)
	}
}

// A move whose radiograms the Cube still holds after the last check is
// not confirmed, with the slots named, and a member the move left
// flagged is named too.
func TestStructureNotConfirmed(t *testing.T) {
	fc, ts := startStructureCube(t, nil)
	fc.SetFreeSlots(47)
	flagged := copyGreeting(t)
	withErrorFlag(t, flagged, 0x0a010a, cube.RFAddLinkPartner)
	fc.SetGreeting(flagged)
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a010d", `{"room_id":1,"copy":false}`)
	if code != 200 || out["confirmation"] != "pending" {
		t.Fatalf("move: %d %v", code, out)
	}
	ev := structureEvent(t, ts, "0a010d")
	flags, _ := ev.Summary["flagged"].([]any)
	if ev.Level != "warn" || ev.Summary["outcome"] != "not_confirmed" || ev.Summary["action"] != "move" || ev.Summary["held"] != float64(3) ||
		len(flags) != 1 || flags[0] != "0a010a" {
		t.Fatalf("event %+v", ev)
	}
}
