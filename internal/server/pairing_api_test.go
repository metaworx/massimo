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

const newDevice cube.RFAddress = 0x0a0b0c

// startPairingCube starts a server over a fake Cube in which a radiator
// thermostat, a copy of 1514b6 at newDevice, joins delay after n:.
func startPairingCube(t *testing.T, delay time.Duration) (*fakecube.Server, *httptest.Server, *Server) {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := fakecube.PairingFrom(greeting, 0x1514b6, newDevice, "TEQ0000001", delay)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting, Pair: pair}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0537249", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond}})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var c cubeView
		if getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249", &c) == 200 && c.Connected {
			return fc, ts, s
		}
	}
	t.Fatal("cube never connected")
	return nil, nil, nil
}

// waitFor polls cond until it holds or five seconds passed.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestPairingAndAddingToARoom(t *testing.T) {
	fc, ts, _ := startPairingCube(t, 50*time.Millisecond)
	const base = "/api/v1/cubes/KEQ0537249"
	if code, _ := do(t, ts, "POST", base+"/pairing", `{"seconds":0}`); code != 400 {
		t.Fatalf("0 seconds: %d", code)
	}
	code, st := do(t, ts, "POST", base+"/pairing", `{"seconds":30}`)
	if code != 200 || st["active"] != true || st["until"] == nil {
		t.Fatalf("start: %d %v", code, st)
	}
	if code, _ := do(t, ts, "POST", base+"/pairing", ""); code != 409 {
		t.Fatalf("second start: %d", code)
	}
	if got := fc.Received(); len(got) != 1 || got[0].String() != "n:001e" {
		t.Fatalf("cube received %v", got)
	}
	// The device joins: it is in the state, the log has the event, and
	// the server fetched its C line, so the device list has it.
	waitFor(t, "the paired device", func() bool {
		_, st = do(t, ts, "GET", base+"/pairing", "")
		p, _ := st["paired"].([]any)
		return len(p) == 1
	})
	if p := st["paired"].([]any)[0].(map[string]any); p["rf_address"] != "0a0b0c" || p["serial"] != "TEQ0000001" || p["type"] != "radiator_thermostat" {
		t.Fatalf("paired %v", p)
	}
	waitFor(t, "the C line", func() bool {
		code, d := do(t, ts, "GET", base+"/devices/0a0b0c", "")
		return code == 200 && d["unlisted"] == true && d["thermostat"] != nil
	})
	var evs []event
	getJSON(t, ts, "", base+"/log", &evs)
	kinds := map[string]bool{}
	for _, e := range evs {
		kinds[e.Kind] = true
		if e.Kind == "paired" && (e.Paired == nil || e.Paired.RFAddress != newDevice || e.Device != "0a0b0c") {
			t.Fatalf("paired event %+v", e)
		}
	}
	if !kinds["pairing_started"] || !kinds["paired"] {
		t.Fatalf("events %v", kinds)
	}
	// Stop: x: to the Cube.
	if code, st := do(t, ts, "DELETE", base+"/pairing", ""); code != 200 || st["active"] != false {
		t.Fatalf("stop: %d %v", code, st)
	}
	if got := fc.Received(); got[len(got)-1].Type != 'x' {
		t.Fatalf("last line %v", got[len(got)-1])
	}

	// Without a room it cannot be renamed alone.
	if code, _ := do(t, ts, "PUT", base+"/devices/0a0b0c", `{"name":"New"}`); code != 409 {
		t.Fatalf("rename of an unlisted device: %d", code)
	}
	before := len(fc.Received())
	if code, out := do(t, ts, "PUT", base+"/devices/0a0b0c", `{"name":"Neu","room_id":1,"copy":false}`); code != 200 {
		t.Fatalf("add to room 1: %d %v", code, out)
	}
	// Room 1 has one radiator thermostat, 14d08e. As the original
	// software: wakeup, a link in each direction, the group, then the
	// metadata and a re-read of the C line; the copy of the room's
	// settings, which follows by default, has tests of its own
	// (TestAddCopiesTheRoomsSettings and those after it).
	partner := cube.CmdLinkPartner(cube.Target{RFAddress: 0x14d08e, RoomID: 1, Room: true}, true, newDevice, cube.DeviceRadiatorThermostat)
	own := cube.CmdLinkPartner(cube.Target{RFAddress: newDevice}, true, 0x14d08e, cube.DeviceRadiatorThermostat)
	group := cube.CmdSetGroup(newDevice, 1, false)
	got := fc.Received()[before:]
	want := []string{"z:1e,G,01", partner.String(), own.String(), group.String()}
	if len(got) != 6 {
		t.Fatalf("cube received %v", got)
	}
	for i, w := range want {
		if got[i].String() != w {
			t.Fatalf("line %d: %s, want %s", i, got[i], w)
		}
	}
	if got[4].Type != 'm' || got[5].String() != "c:0a0b0c" {
		t.Fatalf("metadata and re-read: %v", got[4:])
	}
	_, d := do(t, ts, "GET", base+"/devices/0a0b0c", "")
	if d["unlisted"] == true || d["name"] != "Neu" || d["room_id"] != float64(1) {
		t.Fatalf("device after the add %v", d)
	}
	var rooms []map[string]any
	getJSON(t, ts, "", base+"/rooms", &rooms)
	found := false
	for _, r := range rooms {
		if r["id"] == float64(1) {
			for _, d := range r["devices"].([]any) {
				found = found || d.(map[string]any)["rf_address"] == "0a0b0c"
			}
		}
	}
	if !found {
		t.Fatal("device not in room 1")
	}
}

func TestPairingEndsWithTheCube(t *testing.T) {
	fc, ts, _ := startPairingCube(t, time.Hour)
	const base = "/api/v1/cubes/KEQ0537249"
	if code, _ := do(t, ts, "POST", base+"/pairing", `{"seconds":1}`); code != 200 {
		t.Fatal(code)
	}
	waitFor(t, "the end", func() bool {
		_, st := do(t, ts, "GET", base+"/pairing", "")
		return st["active"] == false && st["ended"] != nil
	})
	if got := fc.Received(); got[0].String() != "n:0001" {
		t.Fatalf("received %v", got)
	}
}
