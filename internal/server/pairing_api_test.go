// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
)

const newDevice cube.RFAddress = 0x0a0b0c

// pairing is the start of the pairing's tests: a fake Cube in which a
// radiator thermostat, a copy of 0a010d at newDevice, joins delay after
// n:.
func pairing(delay time.Duration) startOpt {
	return func(c *startConfig) {
		withFake(func(fc *fakecube.Server) {
			pair, err := fakecube.PairingFrom(fc.Greeting, 0x0a010d, newDevice, "TEQ0000001", delay)
			if err != nil {
				c.t.Fatal(err)
			}
			fc.Pair = pair
		})(c)
	}
}

func TestPairingAndAddingToARoom(t *testing.T) {
	t.Parallel()
	rg := startServer(t, pairing(50*time.Millisecond))
	fc, ts := rg.fc, rg.ts
	received := cubewatch.Lines(fc)
	const base = "/api/v1/cubes/KEQ0000100"
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
	// n: gets no answer, so the fake may hold it only after the API's.
	// The device joins 50 ms after it, and the server reads its C line,
	// which a busy machine can show by now: that line is the join's.
	received.Await(t, 0, "the n: at the Cube", cubewatch.Type('n'))
	notJoin := func(m cube.Message) bool { return m.Type != 'l' && m.String() != "c:0a0b0c" }
	if got := received.Since(0, notJoin); len(got) != 1 || got[0].String() != "n:001e" {
		t.Fatalf("cube received %v", got)
	}
	// The device joins: it is in the state, the log has the event, and
	// the server fetched its C line, so the device list has it.
	testkit.Eventually(t, "the paired device", func() bool {
		_, st = do(t, ts, "GET", base+"/pairing", "")
		p, _ := st["paired"].([]any)
		return len(p) == 1
	})
	if p := st["paired"].([]any)[0].(map[string]any); p["rf_address"] != "0a0b0c" || p["serial"] != "TEQ0000001" || p["type"] != "radiator_thermostat" {
		t.Fatalf("paired %v", p)
	}
	testkit.Eventually(t, "the C line", func() bool {
		code, d := do(t, ts, "GET", base+"/devices/0a0b0c", "")
		return code == 200 && d["unlisted"] == true && d["thermostat"] != nil
	})
	// The pump logs the paired event after it has asked for the C line,
	// so the API can show the line before the log has the event: the
	// log is read once the pump has handled it (docs/testing.md, "The
	// pump").
	pumpSync(t, rg.s)
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
	if got := received.Since(0, cubewatch.Except('l')); got[len(got)-1].Type != 'x' {
		t.Fatalf("last line %v", got[len(got)-1])
	}

	// Without a room it cannot be renamed alone.
	if code, _ := do(t, ts, "PUT", base+"/devices/0a0b0c", `{"name":"New"}`); code != 409 {
		t.Fatalf("rename of an unlisted device: %d", code)
	}
	before := received.Mark()
	if code, out := do(t, ts, "PUT", base+"/devices/0a0b0c", `{"name":"Neu","room_id":1,"copy":false}`); code != 200 {
		t.Fatalf("add to room 1: %d %v", code, out)
	}
	// Room 1 has one radiator thermostat, 0a010a. As the original
	// software: wakeup, a link in each direction, the group, then the
	// metadata and a re-read of the C line, the device woken itself after
	// its room, whose group it is not in yet; the copy of the room's
	// settings, which follows by default, has tests of its own
	// (TestAddCopiesTheRoomsSettings and those after it).
	partner := cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, true, newDevice, cube.DeviceRadiatorThermostat)
	own := cube.CmdLinkPartner(cube.Target{RFAddress: newDevice}, true, 0x0a010a, cube.DeviceRadiatorThermostat)
	group := cube.CmdSetGroup(newDevice, 1, false)
	got := received.Since(before, cubewatch.Except('l'))
	want := []string{"z:1e,G,01", "z:1e,D,0a0b0c", partner.String(), own.String(), group.String()}
	if len(got) != 7 {
		t.Fatalf("cube received %v", got)
	}
	for i, w := range want {
		if got[i].String() != w {
			t.Fatalf("line %d: %s, want %s", i, got[i], w)
		}
	}
	if got[5].Type != 'm' || got[6].String() != "c:0a0b0c" {
		t.Fatalf("metadata and re-read: %v", got[5:])
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
	t.Parallel()
	r := startServer(t, pairing(time.Hour))
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	const base = "/api/v1/cubes/KEQ0000100"
	if code, _ := do(t, ts, "POST", base+"/pairing", `{"seconds":1}`); code != 200 {
		t.Fatal(code)
	}
	testkit.Eventually(t, "the end", func() bool {
		_, st := do(t, ts, "GET", base+"/pairing", "")
		return st["active"] == false && st["ended"] != nil
	})
	if got := received.Since(0, cubewatch.Except('l')); got[0].String() != "n:0001" {
		t.Fatalf("received %v", got)
	}
}

// The API asks for at most 255 s, the longest a Cube keeps the mode:
// 256 is refused before anything is sent, 255 goes to the Cube as n:00ff.
func TestPairingTimeLimit(t *testing.T) {
	t.Parallel()
	r := startServer(t, pairing(time.Hour))
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	const base = "/api/v1/cubes/KEQ0000100"
	for _, body := range []string{`{"seconds":256}`, `{"seconds":600}`} {
		if code, out := do(t, ts, "POST", base+"/pairing", body); code != 400 || out["error"] != "seconds must be 1..255" {
			t.Fatalf("%s: %d %v", body, code, out)
		}
	}
	if got := received.Since(0, cubewatch.Except('l')); len(got) != 0 {
		t.Fatalf("a refused pairing sent %v", got)
	}
	code, st := do(t, ts, "POST", base+"/pairing", `{"seconds":255}`)
	if code != 200 {
		t.Fatalf("255 s: %d %v", code, st)
	}
	started, _ := time.Parse(time.RFC3339Nano, st["started"].(string))
	until, _ := time.Parse(time.RFC3339Nano, st["until"].(string))
	if until.Sub(started) != 255*time.Second {
		t.Fatalf("started %v, until %v", st["started"], st["until"])
	}
	// The fake records a line once it read it, which can be after the
	// answer.
	received.Await(t, 0, "the n: at the Cube", cubewatch.Type('n'))
	if got := received.Since(0, cubewatch.Except('l')); len(got) != 1 || got[0].String() != "n:00ff" {
		t.Fatalf("cube received %v", got)
	}
	if code, _ := do(t, ts, "DELETE", base+"/pairing", ""); code != 200 {
		t.Fatalf("stop: %d", code)
	}
}
