// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
)

// The repairs' tests run over copyGreeting (config_api_test.go): room 1
// holds the radiator thermostat 0a010a, its master, and the wall
// thermostat copyWall; copyNew, copyNew2 and copyContact are in no room
// until a test puts them somewhere.

// withMetadata changes the metadata of the greeting's M line.
func withMetadata(t *testing.T, greeting []cube.Message, change func(md *cube.Metadata)) {
	t.Helper()
	for i, g := range greeting {
		if g.Type != 'M' {
			continue
		}
		p, err := cube.ParseMetadataPart(g.Data)
		if err != nil {
			t.Fatal(err)
		}
		md, err := cube.ParseMetadata([]cube.MetadataPart{p})
		if err != nil {
			t.Fatal(err)
		}
		change(md)
		enc, err := md.Encode()
		if err != nil {
			t.Fatal(err)
		}
		greeting[i] = cube.Message{Type: 'M', Data: "00,01," + base64.StdEncoding.EncodeToString(enc)}
		return
	}
	t.Fatal("no M line")
}

// listIn lists rf, of type typ, in room in the greeting's metadata.
func listIn(t *testing.T, greeting []cube.Message, rf cube.RFAddress, typ cube.DeviceType, serial string, room uint8) {
	t.Helper()
	withMetadata(t, greeting, func(md *cube.Metadata) {
		for i := range md.Devices {
			if md.Devices[i].RFAddress == rf {
				md.Devices[i].RoomID = room
				return
			}
		}
		md.Devices = append(md.Devices, cube.DeviceMeta{Type: typ, RFAddress: rf, Serial: serial, Name: "Dev " + rf.String(), RoomID: room})
	})
}

// withContactInRoom1 puts the shutter contact into room 1, by its C line
// and by the metadata.
func withContactInRoom1(t *testing.T, greeting []cube.Message) {
	t.Helper()
	withConfigRoom(t, greeting, copyContact, 1)
	listIn(t, greeting, copyContact, cube.DeviceShutterContact, "IEQ0000003", 1)
}

// startRepairCube is startCopyCube with the resets read back at once.
func startRepairCube(t *testing.T, mod func(*fakecube.Server)) (*fakecube.Server, *httptest.Server, *Server) {
	t.Helper()
	fc := &fakecube.Server{Greeting: copyGreeting(t)}
	if mod != nil {
		mod(fc)
	}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond}, ResetSettle: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	waitFor(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})
	return fc, ts, s
}

// sentSince is what the fake Cube received from the n-th line on, the
// polls' l: left out.
func sentSince(fc *fakecube.Server, n int) []cube.Message {
	var out []cube.Message
	for _, m := range fc.Received()[n:] {
		if m.Type != 'l' {
			out = append(out, m)
		}
	}
	return out
}

// previewOf GETs a repair's preview.
func previewOf(t *testing.T, ts *httptest.Server, path string) (int, repairPlan, string) {
	t.Helper()
	code, body := callAPI(t, ts, "GET", path, "", "")
	var p repairPlan
	if code == 200 {
		if err := json.Unmarshal([]byte(body), &p); err != nil {
			t.Fatal(err)
		}
	}
	return code, p, body
}

// runRepairOf previews a repair and runs it with the preview's plan.
func runRepairOf(t *testing.T, ts *httptest.Server, path, query string) (repairPlan, int, map[string]any) {
	t.Helper()
	code, p, body := previewOf(t, ts, path+query)
	if code != 200 {
		t.Fatalf("GET %s: %d %s", path+query, code, body)
	}
	sep := "?"
	if query != "" {
		sep = "&"
	}
	code, out := do(t, ts, "POST", path+query+sep+"plan="+p.Plan, "")
	return p, code, out
}

// linkPartner is a 0x20 (add) or 0x21 to target naming partner.
func linkPartner(target cube.Target, add bool, partner cube.RFAddress, typ cube.DeviceType) string {
	return cube.CmdLinkPartner(target, add, partner, typ).String()
}

// room1 is room 1's member rf addressed with the room flag.
func room1(rf cube.RFAddress) cube.Target { return cube.Target{RFAddress: rf, RoomID: 1, Room: true} }

// direct addresses rf alone.
func direct(rf cube.RFAddress) cube.Target { return cube.Target{RFAddress: rf} }

// relinkLines are the re-link of room 1 with 0a010a, the wall
// thermostat and the contact, for add (0x20 and 0x22) or not (0x21, no
// 0x22): each member named to the room's addressee, the master 0a010a
// but for itself, where the wall thermostat takes its place; the contact
// directly where a thermostat is named; the member told of each partner.
func relinkLines(add bool) []string {
	const rad = cube.DeviceRadiatorThermostat
	const wall = cube.DeviceWallThermostat
	const contact = cube.DeviceShutterContact
	var out []string
	part := func(lines ...string) { out = append(out, lines...) }
	group := func(rf cube.RFAddress) {
		if add {
			part(cube.CmdSetGroup(rf, 1, false).String())
		}
	}
	part("z:1e,G,01",
		linkPartner(room1(copyWall), add, 0x0a010a, rad),
		linkPartner(direct(copyContact), add, 0x0a010a, rad),
		linkPartner(direct(0x0a010a), add, copyWall, wall),
		linkPartner(direct(0x0a010a), add, copyContact, contact))
	group(0x0a010a)
	part("z:1e,G,01",
		linkPartner(room1(0x0a010a), add, copyWall, wall),
		linkPartner(direct(copyContact), add, copyWall, wall),
		linkPartner(direct(copyWall), add, 0x0a010a, rad),
		linkPartner(direct(copyWall), add, copyContact, contact))
	group(copyWall)
	part("z:1e,G,01",
		linkPartner(room1(0x0a010a), add, copyContact, contact),
		linkPartner(direct(copyContact), add, 0x0a010a, rad),
		linkPartner(direct(copyContact), add, copyWall, wall))
	group(copyContact)
	return out
}

// A re-link of room 1 adds every member's links and group again, one
// command per member after the room's wakeup, and unlinks nothing; the
// preview names those lines and their cost and sends nothing, and the
// run sends exactly them, then reads the slots back.
func TestRepairRelink(t *testing.T) {
	fc, ts, _ := startRepairCube(t, func(fc *fakecube.Server) { withContactInRoom1(t, fc.Greeting) })
	const path = "/api/v1/cubes/KEQ0000100/rooms/1/repair/relink"
	sent := len(fc.Received())
	code, p, body := previewOf(t, ts, path)
	if code != 200 || p.Action != "relink" || p.Plan == "" || len(p.Steps) != 3 || p.Refused != "" {
		t.Fatalf("preview: %d %s", code, body)
	}
	// 14 radiograms, three of them with the room flag to a room of three:
	// two more slots each, held until the members' next wakeup.
	if p.Radiograms != 14 || p.DutyCostPct != 14*radioCostPct || p.Slots != 20 || p.Held != 6 || p.Wakeups != 3 || p.Resets != 0 || p.Metadata {
		t.Fatalf("preview's cost: %s", body)
	}
	var shown []string
	for _, st := range p.Steps {
		shown = append(shown, "z:1e,G,01")
		shown = append(shown, st.Lines...)
	}
	want := relinkLines(true)
	if strings.Join(shown, "\n") != strings.Join(want, "\n") {
		t.Fatalf("preview's lines:\n%s\nwant\n%s", strings.Join(shown, "\n"), strings.Join(want, "\n"))
	}
	if got := sentSince(fc, sent); len(got) != 0 {
		t.Fatalf("the preview sent %v", lineStrings(got))
	}
	code, out := do(t, ts, "POST", path+"?plan="+p.Plan, "")
	if code != 200 || out["ok"] != true || out["confirmation"] != "pending" {
		t.Fatalf("run: %d %v", code, out)
	}
	sameLines(t, sentSince(fc, sent), want)
	steps, _ := out["steps"].([]any)
	if len(steps) != 3 {
		t.Fatalf("steps %v", out["steps"])
	}
	for _, st := range steps {
		if m := st.(map[string]any); m["accepted"] != m["lines"] || m["error"] != nil {
			t.Fatalf("step %v", m)
		}
	}
}

// nothingOf asserts that a repair's preview has nothing to send, why
// naming want, and that its run is refused with 409.
func nothingOf(t *testing.T, ts *httptest.Server, path, want string) repairPlan {
	t.Helper()
	code, p, body := previewOf(t, ts, path)
	if code != 200 || p.Plan != "" || len(p.Steps) != 0 || !strings.Contains(p.Nothing, want) {
		t.Fatalf("GET %s: %d %s, want nothing to send for %q", path, code, body, want)
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	if code, out := do(t, ts, "POST", path+sep+"plan=0", ""); code != http.StatusConflict {
		t.Fatalf("POST %s: %d %v, want 409", path, code, out)
	}
	return p
}

// A room whose list and Cube disagree is not re-linked: the placement
// repairs it first.
func TestRepairRelinkRefusesADisagreeingRoom(t *testing.T) {
	fc, ts, _ := startRepairCube(t, func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyContact, 1) })
	sent := len(fc.Received())
	nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/rooms/1/repair/relink", "0d0e0f")
	if got := sentSince(fc, sent); len(got) != 0 {
		t.Fatalf("sent %v", lineStrings(got))
	}
}

// A run needs the preview's plan, and one whose plan differs from the
// repair as it is now is refused: nothing is sent.
func TestRepairNeedsItsPlan(t *testing.T) {
	fc, ts, _ := startRepairCube(t, func(fc *fakecube.Server) { withContactInRoom1(t, fc.Greeting) })
	const path = "/api/v1/cubes/KEQ0000100/rooms/1/repair/relink"
	sent := len(fc.Received())
	if code, out := do(t, ts, "POST", path, ""); code != http.StatusBadRequest {
		t.Fatalf("without a plan: %d %v", code, out)
	}
	if code, out := do(t, ts, "POST", path+"?plan=0000", ""); code != http.StatusConflict || !strings.Contains(out["error"].(string), "preview it again") {
		t.Fatalf("with another plan: %d %v", code, out)
	}
	if got := sentSince(fc, sent); len(got) != 0 {
		t.Fatalf("sent %v", lineStrings(got))
	}
}

// A repair that would leave fewer free slots than the queue waits for
// is refused before it starts, in its preview and its run.
func TestRepairRefusedForTheSlots(t *testing.T) {
	fc, ts, _ := startRepairCube(t, func(fc *fakecube.Server) {
		withContactInRoom1(t, fc.Greeting)
		fc.SetFreeSlots(8)
	})
	const path = "/api/v1/cubes/KEQ0000100/rooms/1/repair/relink"
	sent := len(fc.Received())
	code, p, body := previewOf(t, ts, path)
	if code != 200 || !strings.Contains(p.Refused, "memory slots") {
		t.Fatalf("preview with 8 slots free: %d %s", code, body)
	}
	if code, out := do(t, ts, "POST", path+"?plan="+p.Plan, ""); code != http.StatusConflict {
		t.Fatalf("run: %d %v", code, out)
	}
	if got := sentSince(fc, sent); len(got) != 0 {
		t.Fatalf("sent %v", lineStrings(got))
	}
}

// clearOnReset makes the fake Cube clear a device's error flag at its
// r:, as the Cube does (docs/cube-protocol.md, the r: findings).
func clearOnReset(t *testing.T, fc *fakecube.Server) {
	fc.DiscardIf = func(m cube.Message) bool {
		if m.Type != 'r' {
			return false
		}
		_, b64, _ := strings.Cut(m.Data, ",")
		raw, err := base64.StdEncoding.DecodeString(b64)
		for i := 0; err == nil && i+3 <= len(raw); i += 3 {
			off := false
			fc.SetState(cube.RFAddressFromBytes(raw[i:i+3]), fakecube.State{Error: &off})
		}
		return false
	}
}

// The resets of a room and of the Cube: one r: per flagged device, by
// their C lines, free; the device list read after them shows the flags
// gone, or names those still set.
func TestRepairReset(t *testing.T) {
	flag := func(fc *fakecube.Server) {
		withErrorFlag(t, fc.Greeting, 0x0a010a, cube.RFRemoveLink)
		withErrorFlag(t, fc.Greeting, 0x0a010d, 0x03)
	}
	t.Run("room", func(t *testing.T) {
		fc, ts, _ := startRepairCube(t, func(fc *fakecube.Server) { flag(fc); clearOnReset(t, fc) })
		sent := len(fc.Received())
		p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/rooms/1/repair/reset", "")
		if p.Radiograms != 0 || p.Resets != 1 || p.DutyCostPct != 0 || p.Wakeups != 0 || p.Refused != "" {
			t.Fatalf("preview %+v", p)
		}
		if code != 200 || out["ok"] != true || out["confirmation"] != nil {
			t.Fatalf("run: %d %v", code, out)
		}
		if still, _ := out["still_flagged"].([]any); len(still) != 0 {
			t.Fatalf("still flagged: %v", still)
		}
		sameLines(t, sentSince(fc, sent), []string{cube.CmdResetError(0x0a010a).String()})
	})
	t.Run("cube", func(t *testing.T) {
		fc, ts, _ := startRepairCube(t, flag)
		sent := len(fc.Received())
		p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/repair/reset", "")
		if p.Resets != 2 || p.Radiograms != 0 {
			t.Fatalf("preview %+v", p)
		}
		still, _ := out["still_flagged"].([]any)
		if code != 200 || len(still) != 2 {
			t.Fatalf("run, the fake keeping the flags: %d %v", code, out)
		}
		sameLines(t, sentSince(fc, sent), []string{cube.CmdResetError(0x0a010d).String(), cube.CmdResetError(0x0a010a).String()})
	})
	t.Run("nothing flagged", func(t *testing.T) {
		_, ts, _ := startRepairCube(t, nil)
		nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/rooms/1/repair/reset", "no device of room 1 has a communication error")
		nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/repair/reset", "no device of the cube")
	})
}

// withRoom1Thermostats puts the two new radiator thermostats into room
// 1 beside 0a010a and the wall thermostat, by their C lines and the
// metadata: copyNew keeps its own settings (0a010d's, with room 1's
// Saturday and Sunday), copyNew2 gets 0a010a's where same is set.
func withRoom1Thermostats(t *testing.T, greeting []cube.Message, same bool) {
	t.Helper()
	ref := rawConfig(t, greeting, 0x0a010a)
	withConfigRoom(t, greeting, copyNew, 1)
	listIn(t, greeting, copyNew, cube.DeviceRadiatorThermostat, "TEQ0000001", 1)
	if !same {
		return
	}
	for i, g := range greeting {
		if g.Type == 'C' && strings.HasPrefix(g.Data, copyNew2.String()+",") {
			greeting[i] = configLine(ref, copyNew2, 1, "TEQ0000004")
		}
	}
	listIn(t, greeting, copyNew2, cube.DeviceRadiatorThermostat, "TEQ0000004", 1)
}

// A thermostat's configuration is re-sent from what most of its room's
// other thermostats share by their C lines: 0a010a and copyNew2 share
// temperatures that the wall thermostat does not, all three 0a010a's
// week programme; copyNew gets the five days of it that differ, then the
// shared temperatures with its own offset and window-open minutes, and
// its C line is read again.
func TestRepairResendConfig(t *testing.T) {
	fc, ts, s := startRepairCube(t, func(fc *fakecube.Server) { withRoom1Thermostats(t, fc.Greeting, true) })
	snap := s.cubes["keq0000100"].client.Snapshot()
	refProg := snap.Config(0x0a010a).Thermostat.WeekProgram
	own := snap.Config(copyNew).Thermostat
	var want []string
	want = append(want, "z:1e,G,01")
	for _, d := range []cube.Weekday{cube.Monday, cube.Tuesday, cube.Wednesday, cube.Thursday, cube.Friday} {
		ls, err := cube.CmdSetDayProgram(direct(copyNew), d, refProg[d])
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, lineStrings(ls)...)
	}
	temps, err := cube.CmdSetTemperatures(direct(copyNew), cube.TemperatureSettings{Comfort: 22, Eco: 15, Max: 30.5, Min: 4.5,
		Offset: own.TempOffset, WindowOpen: 5, WindowOpenMinutes: own.WindowOpenMins})
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, temps.String(), "c:"+copyNew.String())
	sent := len(fc.Received())
	p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0a0b0c/repair/config", "")
	if p.Action != "config" || p.Details["others"] != float64(3) || p.Details["temperatures_by"] != float64(2) || p.Details["program_by"] != float64(3) {
		t.Fatalf("preview %+v", p)
	}
	if code != 200 || out["ok"] != true {
		t.Fatalf("run: %d %v", code, out)
	}
	sameLines(t, sentSince(fc, sent), want)
}

// Without a majority, nothing is re-sent and the answer says why: the
// two other thermostats of room 1 differ in their temperatures; a device
// alone in its room has nothing to take them from.
func TestRepairResendRefused(t *testing.T) {
	fc, ts, _ := startRepairCube(t, func(fc *fakecube.Server) { withRoom1Thermostats(t, fc.Greeting, false) })
	sent := len(fc.Received())
	nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0a0b0c/repair/config", "share no temperatures")
	nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0a010d/repair/config", "no other thermostat")
	if got := sentSince(fc, sent); len(got) != 0 {
		t.Fatalf("sent %v", lineStrings(got))
	}
}

// A device the metadata lists in room 1 whose C line names no room is
// placed there with the placement's link half, the metadata left as it
// is; one whose C line names room 6 is moved from there.
func TestRepairPlace(t *testing.T) {
	t.Run("from no room", func(t *testing.T) {
		fc, ts, _ := startRepairCube(t, func(fc *fakecube.Server) {
			listIn(t, fc.Greeting, copyContact, cube.DeviceShutterContact, "IEQ0000003", 1)
		})
		sent := len(fc.Received())
		p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0d0e0f/repair/place", "")
		if p.Action != "move" || len(p.Steps) != 1 || p.Radiograms != 4 {
			t.Fatalf("preview %+v", p)
		}
		if code != 200 || out["confirmation"] != "pending" {
			t.Fatalf("run: %d %v", code, out)
		}
		sameLines(t, sentSince(fc, sent), []string{"z:1e,G,01",
			linkPartner(room1(0x0a010a), true, copyContact, cube.DeviceShutterContact),
			linkPartner(direct(copyContact), true, 0x0a010a, cube.DeviceRadiatorThermostat),
			linkPartner(direct(copyContact), true, copyWall, cube.DeviceWallThermostat),
			cube.CmdSetGroup(copyContact, 1, false).String(),
			"c:" + copyContact.String()})
	})
	t.Run("from room 6", func(t *testing.T) {
		fc, ts, _ := startRepairCube(t, func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyWall, 6) })
		sent := len(fc.Received())
		_, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0c0d0e/repair/place", "")
		if code != 200 {
			t.Fatalf("run: %d %v", code, out)
		}
		sameLines(t, sentSince(fc, sent), []string{"z:1e,G,06",
			cube.CmdSetGroup(copyWall, 6, true).String(),
			linkPartner(cube.Target{RFAddress: 0x0a010d, RoomID: 6, Room: true}, false, copyWall, cube.DeviceWallThermostat),
			linkPartner(direct(copyWall), false, 0x0a010d, cube.DeviceRadiatorThermostat),
			"z:1e,G,01",
			linkPartner(room1(0x0a010a), true, copyWall, cube.DeviceWallThermostat),
			linkPartner(direct(copyWall), true, 0x0a010a, cube.DeviceRadiatorThermostat),
			cube.CmdSetGroup(copyWall, 1, false).String(),
			"c:" + copyWall.String()})
	})
	t.Run("in its room", func(t *testing.T) {
		_, ts, _ := startRepairCube(t, nil)
		nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0a010a/repair/place", "in room 1 already")
	})
}

// listedRoom is the room the server's metadata lists rf in, false where
// it does not list it.
func listedRoom(s *Server, rf cube.RFAddress) (uint8, bool) {
	for _, d := range s.cubes["keq0000100"].client.Snapshot().Metadata().Devices {
		if d.RFAddress == rf {
			return d.RoomID, true
		}
	}
	return 0, false
}

// A device is listed where its C line has it, metadata only: moved in
// the list from room 1 to room 6; one the list does not have, in a room
// it does not have, is listed there once the room has a name, the
// device its master.
func TestRepairList(t *testing.T) {
	t.Run("another room", func(t *testing.T) {
		fc, ts, s := startRepairCube(t, func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyWall, 6) })
		sent := len(fc.Received())
		p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0c0d0e/repair/list", "")
		if p.Radiograms != 0 || !p.Metadata || p.DutyCostPct != 0 {
			t.Fatalf("preview %+v", p)
		}
		if code != 200 || out["ok"] != true {
			t.Fatalf("run: %d %v", code, out)
		}
		for _, m := range sentSince(fc, sent) {
			if m.Type != 'm' {
				t.Fatalf("sent %s", m)
			}
		}
		if room, ok := listedRoom(s, copyWall); !ok || room != 6 {
			t.Fatalf("listed in %d %v", room, ok)
		}
	})
	t.Run("a new room", func(t *testing.T) {
		_, ts, s := startRepairCube(t, func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyNew, 7) })
		const path = "/api/v1/cubes/KEQ0000100/devices/0a0b0c/repair/list"
		if p := nothingOf(t, ts, path, "give room_name"); p.Details["code"] != "room_name_needed" || p.Details["room"] != float64(7) {
			t.Fatalf("without the room's name: %+v", p)
		}
		if _, code, out := runRepairOf(t, ts, path, "?room_name="+url.QueryEscape("Attic")); code != 200 {
			t.Fatalf("run: %d %v", code, out)
		}
		if room, ok := listedRoom(s, copyNew); !ok || room != 7 {
			t.Fatalf("listed in %d %v", room, ok)
		}
		for _, r := range s.cubes["keq0000100"].client.Snapshot().Metadata().Rooms {
			if r.ID == 7 && (r.Name != "Attic" || r.RFAddress != copyNew) {
				t.Fatalf("room 7: %+v", r)
			}
		}
	})
}

// A room whose metadata names a device of another room as its master
// gets its wall thermostat as master, metadata only; a valid master is
// left alone.
func TestRepairMaster(t *testing.T) {
	fc, ts, s := startRepairCube(t, func(fc *fakecube.Server) { withRoomMaster(t, fc.Greeting, 1, 0x0a010d) })
	sent := len(fc.Received())
	p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/rooms/1/repair/master", "")
	if p.Details["new_master"] != copyWall.String() || p.Radiograms != 0 || !p.Metadata {
		t.Fatalf("preview %+v", p)
	}
	if code != 200 {
		t.Fatalf("run: %d %v", code, out)
	}
	for _, m := range sentSince(fc, sent) {
		if m.Type != 'm' {
			t.Fatalf("sent %s", m)
		}
	}
	for _, r := range s.cubes["keq0000100"].client.Snapshot().Metadata().Rooms {
		if r.ID == 1 && r.RFAddress != copyWall {
			t.Fatalf("room 1's master %s", r.RFAddress)
		}
	}
	nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/rooms/1/repair/master", "a thermostat of the room already")
}

// Every preview sends nothing to the Cube, whichever repair.
func TestRepairPreviewsSendNothing(t *testing.T) {
	fc, ts, _ := startRepairCube(t, func(fc *fakecube.Server) {
		withContactInRoom1(t, fc.Greeting)
		withErrorFlag(t, fc.Greeting, 0x0a010a, 0x03)
		withRoomMaster(t, fc.Greeting, 1, 0x0a010d)
		withConfigRoom(t, fc.Greeting, copyNew, 1)
	})
	sent := len(fc.Received())
	const base = "/api/v1/cubes/KEQ0000100"
	for _, path := range []string{"/repair/reset", "/rooms/1/repair/reset", "/rooms/1/repair/master", "/rooms/1/repair/relink",
		"/devices/0a0b0c/repair/config", "/devices/0a0b0c/repair/place", "/devices/0a0b0c/repair/list"} {
		if code, _, body := previewOf(t, ts, base+path); code >= 500 {
			t.Fatalf("GET %s: %d %s", path, code, body)
		}
	}
	if got := sentSince(fc, sent); len(got) != 0 {
		t.Fatalf("the previews sent %v", lineStrings(got))
	}
}

// A role without repair is refused every repair, its preview included,
// and sends nothing; one with repair alone is let through.
func TestRepairNeedsRepair(t *testing.T) {
	fc, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	st.AddRole(t.Context(), store.Role{Key: "builder", Capabilities: []string{"read", "configure", "structure", "pair"}}, 0)
	st.AddRole(t.Context(), store.Role{Key: "fixer", Capabilities: []string{"read", "repair"}}, 0)
	builder := userToken(t, st, "builder", "builder")
	fixer := userToken(t, st, "fixer", "fixer")
	sent := len(fc.Received())
	for _, r := range registeredRoutes(t) {
		if r.capability != "repair" {
			continue
		}
		code, text := callAPI(t, ts, r.method, fillRoute(r.path)+"?plan=0", builder, "")
		if code != http.StatusForbidden || errorOf(text) != `capability "repair" required` {
			t.Errorf("builder: %s %s: %d %s", r.method, r.path, code, text)
		}
		if code, text = callAPI(t, ts, r.method, fillRoute(r.path)+"?plan=0", fixer, ""); code == http.StatusForbidden {
			t.Errorf("fixer: %s %s: %d %s", r.method, r.path, code, text)
		}
	}
	if got := sentSince(fc, sent); len(got) != 0 {
		t.Fatalf("sent %v", lineStrings(got))
	}
}
