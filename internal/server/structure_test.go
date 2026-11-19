// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// structured is the start of the structure changes' tests:
// copyGreeting's Cube, the changes read back within a fraction of a
// second, and reconnects for the budget allowed (AP DeviceUnPairingFix
// v1.3, Block 3).
func structured() startOpt {
	return func(c *startConfig) {
		withGreeting(copyGreeting(c.t))(c)
		editOptions(func(o *Options) {
			o.RefreshInterval, o.FollowUpDelays, o.FollowUpReconnectGap = time.Hour, []time.Duration{10 * time.Millisecond}, time.Hour
			o.StructureChecks = []time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 300 * time.Millisecond}
		})(c)
	}
}

// structureEvent waits for the structure event of device in the Cube's
// log and returns it.
func structureEvent(t *testing.T, ts *httptest.Server, device string) event {
	t.Helper()
	var found event
	testkit.Eventually(t, "the structure event of "+device, func() bool {
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
	t.Parallel()
	ts := startServer(t, structured()).ts
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

// A room's repair names its rooms in its structure line as numbers, not
// as the base64 string JSON makes of a []uint8 ("BQ==" for room 5, seen
// on hardware 2026-11-16).
func TestStructureRoomsOfARoomRepair(t *testing.T) {
	t.Parallel()
	ts := startServer(t, structured(), withFake(func(fc *fakecube.Server) { withContactInRoom1(t, fc.Greeting) })).ts
	const path = "/api/v1/cubes/KEQ0000100/rooms/1/repair/relink"
	if _, p, body := previewOf(t, ts, path); p.Plan == "" {
		t.Fatalf("preview: %s", body)
	} else if code, out := do(t, ts, "POST", path+"?plan="+p.Plan, ""); code != 200 || out["ok"] != true {
		t.Fatalf("run: %d %v", code, out)
	}
	ev := structureEvent(t, ts, "")
	rooms, ok := ev.Summary["rooms"].([]any)
	if ev.Summary["action"] != "relink" || !ok || len(rooms) != 1 || rooms[0] != float64(1) {
		t.Fatalf("event %+v", ev)
	}
}

// A move whose radiograms the Cube still holds after the last check is
// not confirmed, with the slots named, and a member the move left
// flagged is named too.
func TestStructureNotConfirmed(t *testing.T) {
	t.Parallel()
	r := startServer(t, structured())
	fc, ts := r.fc, r.ts
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
