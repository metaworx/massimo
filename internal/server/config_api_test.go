// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

func do(t *testing.T, ts *httptest.Server, method, path, body string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// sLines returns the s: commands the fake Cube received, decoded.
func sLines(fc *fakecube.Server) [][]byte {
	var out [][]byte
	for _, m := range fc.Received() {
		if m.Type == 's' {
			raw, _ := base64.StdEncoding.DecodeString(m.Data)
			out = append(out, raw)
		}
	}
	return out
}

func TestConfigEndpoints(t *testing.T) {
	fc, ts, s := start(t, "")
	rt := s.cubes["keq0000100"]
	snap := rt.client.Snapshot()
	const rf = "0a010d" // radiator thermostat in room 6 of the fixture
	dev := cube.RFAddress(0x0a010d)

	// Rename a room and a device: metadata writes, mirrored into the snapshot.
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6", `{"name":"Renamed"}`); code != 200 {
		t.Fatalf("rename room: %d", code)
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/"+rf, `{"name":"New name"}`); code != 200 {
		t.Fatalf("rename device: %d", code)
	}
	if countType(fc, 'm') != 2 {
		t.Fatalf("metadata writes: %d", countType(fc, 'm'))
	}
	md := snap.Metadata()
	okRoom, okDev := false, false
	for _, r := range md.Rooms {
		okRoom = okRoom || (r.ID == 6 && r.Name == "Renamed")
	}
	for _, d := range md.Devices {
		okDev = okDev || (d.RFAddress == dev && d.Name == "New name")
	}
	if !okRoom || !okDev {
		t.Fatalf("snapshot after renames: %+v", md)
	}
	for _, c := range []struct{ path, body string }{
		{"/rooms/6", `{"name":""}`},
		{"/rooms/6", `{"name":"` + strings.Repeat("x", 33) + `"}`},
		{"/rooms/42", `{"name":"x"}`},
		{"/devices/" + rf, `{"room_id":0}`},
	} {
		if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100"+c.path, c.body); code != 400 && code != 404 {
			t.Fatalf("PUT %s %s: %d", c.path, c.body, code)
		}
	}

	// Device configuration: wakeup of the room, one 0x11 with the merged
	// values, then a c: refresh.
	before := len(fc.Received())
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/"+rf+"/config", `{"eco_temp":16.5}`); code != 200 || out["ok"] != true {
		t.Fatalf("device config: %d %v", code, out)
	}
	got := fc.Received()[before:]
	var kinds []string
	for _, m := range got {
		kinds = append(kinds, m.String())
	}
	if len(got) != 3 || got[0].String() != "z:1e,G,06" || got[1].Type != 's' || got[2].String() != "c:"+rf {
		t.Fatalf("device config sent %v", kinds)
	}
	cfg := snap.Config(dev).Thermostat
	want, _ := cube.CmdSetTemperatures(cube.Target{RFAddress: dev}, cube.TemperatureSettings{Comfort: cfg.ComfortTemp, Eco: 16.5, Max: cfg.MaxSetpoint,
		Min: cfg.MinSetpoint, Offset: cfg.TempOffset, WindowOpen: cfg.WindowOpenTemp, WindowOpenMinutes: cfg.WindowOpenMins})
	if got[1].String() != want.String() {
		t.Fatalf("0x11 line %s, want %s", got[1], want)
	}
	// Validation and device type checks.
	for _, body := range []string{`{"eco_temp":40}`, `{"temp_offset":1.2}`, `{"boost_minutes":7}`, `{"decalc_day":"funday"}`, `{"display_actual":true}`, `{}`} {
		if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/"+rf+"/config", body); code != 400 {
			t.Fatalf("config %s: %d, want 400", body, code)
		}
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6/config", `{"temp_offset":1}`); code != 400 {
		t.Fatalf("room config with offset: %d", code)
	}

	// Room configuration: every thermostat of the room, per-device results.
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6/config", `{"boost_minutes":10}`); code != 200 || out["ok"] != true || len(out["devices"].([]any)) != 1 {
		t.Fatalf("room config: %d %v", code, out)
	}
	// GET room config and programme.
	if code, out := do(t, ts, "GET", "/api/v1/cubes/KEQ0000100/rooms/6/config", ""); code != 200 || out["source"] != rf || len(out["devices"].([]any)) != 1 {
		t.Fatalf("room config view: %d %v", code, out)
	}
	if code, out := do(t, ts, "GET", "/api/v1/cubes/KEQ0000100/rooms/6/program", ""); code != 200 || out["source"] != rf || out["monday"] == nil {
		t.Fatalf("room programme view: %d %v", code, out)
	}

	// Room week programme: a group radiogram per day (flags 04, room id).
	nS := len(sLines(fc))
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6/program", `{"monday":[{"temp":17,"until":"07:00"},{"temp":21,"until":"24:00"}]}`); code != 200 || out["ok"] != true {
		t.Fatalf("room programme: %d %v", code, out)
	}
	lines := sLines(fc)
	if len(lines) != nS+1 {
		t.Fatalf("room programme sent %d radiograms", len(lines)-nS)
	}
	l := lines[nS]
	if l[1] != 0x04 || l[2] != cube.RFSetWeekProgram || l[9] != 6 || l[10] != byte(cube.Monday) {
		t.Fatalf("room programme line % x", l)
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6/program", `{"monday":[{"temp":17,"until":"07:00"}]}`); code != 400 {
		t.Fatalf("programme not ending at 24:00 accepted: %d", code)
	}
	// Device week programme: direct, two days.
	nS = len(sLines(fc))
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/"+rf+"/program", `{"saturday":[{"temp":18,"until":"24:00"}],"sunday":[{"temp":18,"until":"24:00"}]}`); code != 200 {
		t.Fatalf("device programme: %d", code)
	}
	lines = sLines(fc)
	if len(lines) != nS+2 || lines[nS][1] != 0 || lines[nS][9] != 0 || lines[nS][10] != byte(cube.Saturday) {
		t.Fatalf("device programme lines % x", lines[nS:])
	}

	// Move the device to room 1: leave (group remove), join (links with the
	// room's radiator both ways, group set), metadata; the copy of the
	// room's settings, which follows by default, has tests of its own
	// (TestAddCopiesTheRoomsSettings and those after it).
	nS = len(sLines(fc))
	nM := countType(fc, 'm')
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/"+rf, `{"room_id":1,"copy":false}`); code != 200 {
		t.Fatalf("move: %d %v", code, out)
	}
	lines = sLines(fc)
	var cmds []byte
	for _, l := range lines[nS:] {
		cmds = append(cmds, l[2])
	}
	if string(cmds) != string([]byte{cube.RFRemoveGroup, cube.RFAddLinkPartner, cube.RFAddLinkPartner, cube.RFSetGroupAddress}) || countType(fc, 'm') != nM+1 {
		t.Fatalf("move sent % x, metadata writes %d", cmds, countType(fc, 'm')-nM)
	}
	for _, d := range snap.Metadata().Devices {
		if d.RFAddress == dev && d.RoomID != 1 {
			t.Fatalf("device still in room %d", d.RoomID)
		}
	}
}

// The devices the copy tests add to the fixture: a wall thermostat in
// room 1 beside 0a010a, a radiator thermostat paired but in no room,
// whose Saturday and Sunday are room 1's already, a second one like it,
// and a shutter contact in no room.
const (
	copyWall    cube.RFAddress = 0x0c0d0e
	copyNew     cube.RFAddress = 0x0a0b0c
	copyNew2    cube.RFAddress = 0x0b0c0d
	copyContact cube.RFAddress = 0x0d0e0f
)

// rawConfig decodes a C line of the greeting.
func rawConfig(t *testing.T, greeting []cube.Message, rf cube.RFAddress) []byte {
	t.Helper()
	for _, g := range greeting {
		if g.Type == 'C' && strings.HasPrefix(g.Data, rf.String()+",") {
			raw, err := base64.StdEncoding.DecodeString(g.Data[7:])
			if err != nil {
				t.Fatal(err)
			}
			return raw
		}
	}
	t.Fatalf("no C line for %s", rf)
	return nil
}

// configLine is a C line for raw with the header's address, room and
// serial set.
func configLine(raw []byte, rf cube.RFAddress, room uint8, serial string) cube.Message {
	raw = append([]byte(nil), raw...)
	b := rf.Bytes()
	copy(raw[1:4], b[:])
	raw[5] = room
	copy(raw[8:18], serial)
	return cube.Message{Type: 'C', Data: rf.String() + "," + base64.StdEncoding.EncodeToString(raw)}
}

// copyGreeting is the fixture with the copy tests' devices.
func copyGreeting(t *testing.T) []cube.Message {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	ref := rawConfig(t, greeting, 0x0a010a)  // room 1's radiator thermostat
	tmpl := rawConfig(t, greeting, 0x0a010d) // room 6's, one programme every day
	// A wall thermostat: comfort 21, eco 17, maximum 30.5, minimum 4.5,
	// 0a010a's week programme, offset -0.5, window open 12.0, boost 5 min 80 %.
	wall := append([]byte{0xce, 0, 0, 0, byte(cube.DeviceWallThermostat), 0, 0x10, 0xff}, make([]byte, 10)...)
	wall = append(wall, 42, 34, 61, 9)
	wall = append(wall, ref[0x1d:0x1d+cube.WeekProgramSize]...)
	wall = append(wall, 6, 24, 0x30)
	// The new device: 0a010d's settings, room 1's Saturday and Sunday.
	fresh := append([]byte(nil), tmpl...)
	copy(fresh[0x1d:0x1d+2*2*cube.DaySlots], ref[0x1d:])
	contact := append([]byte{0x11, 0, 0, 0, byte(cube.DeviceShutterContact), 0, 0x10, 0xa0}, make([]byte, 10)...)
	var out []cube.Message
	for _, g := range greeting {
		switch g.Type {
		case 'M':
			p, err := cube.ParseMetadataPart(g.Data)
			if err != nil {
				t.Fatal(err)
			}
			md, err := cube.ParseMetadata([]cube.MetadataPart{p})
			if err != nil {
				t.Fatal(err)
			}
			md.Devices = append(md.Devices, cube.DeviceMeta{Type: cube.DeviceWallThermostat, RFAddress: copyWall, Serial: "NEQ0000002", Name: "Wall", RoomID: 1})
			enc, err := md.Encode()
			if err != nil {
				t.Fatal(err)
			}
			g = cube.Message{Type: 'M', Data: "00,01," + base64.StdEncoding.EncodeToString(enc)}
		case 'L':
			out = append(out, configLine(wall, copyWall, 1, "NEQ0000002"), configLine(fresh, copyNew, 0, "TEQ0000001"),
				configLine(contact, copyContact, 0, "IEQ0000003"), configLine(fresh, copyNew2, 0, "TEQ0000004"))
		}
		out = append(out, g)
	}
	return out
}

// startCopyCube starts a server over a fake Cube with copyGreeting; mod
// changes the fake before it starts.
func startCopyCube(t *testing.T, mod func(*fakecube.Server)) (*fakecube.Server, *httptest.Server, *Server) {
	t.Helper()
	fc := &fakecube.Server{Greeting: copyGreeting(t)}
	if mod != nil {
		mod(fc)
	}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond}})
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

// lineStrings writes protocol lines as the Cube received them.
func lineStrings(lines []cube.Message) []string {
	var out []string
	for _, l := range lines {
		out = append(out, l.String())
	}
	return out
}

// sameLines fails the test where got and want differ; a want of "m:"
// stands for any metadata write.
func sameLines(t *testing.T, got []cube.Message, want []string) {
	t.Helper()
	g := lineStrings(got)
	if len(g) != len(want) {
		t.Fatalf("cube received %d lines, want %d:\n%s", len(g), len(want), strings.Join(g, "\n"))
	}
	for i := range want {
		if want[i] == "m:" && got[i].Type == 'm' {
			continue
		}
		if g[i] != want[i] {
			t.Fatalf("line %d: %s, want %s", i, g[i], want[i])
		}
	}
}

// roomTemps is room 1's 0x11 for one device: 0a010a's values with the
// device's own offset.
func roomTemps(t *testing.T, rf cube.RFAddress, offset float64) string {
	t.Helper()
	m, err := cube.CmdSetTemperatures(cube.Target{RFAddress: rf}, cube.TemperatureSettings{Comfort: 22, Eco: 15, Max: 30.5, Min: 4.5, Offset: offset,
		WindowOpen: 5, WindowOpenMinutes: 15})
	if err != nil {
		t.Fatal(err)
	}
	return m.String()
}

// The new radiator thermostat put into room 1 (a radiator and a wall
// thermostat): the links and the group, the metadata, the re-read, then
// the room's settings as the original software sends them: 0x40 auto,
// 0x11 to the room's three thermostats, 0x12, and the five days that
// differ followed by 0x11 again.
func TestAddCopiesTheRoomsSettings(t *testing.T) {
	fc, ts, s := startCopyCube(t, nil)
	const base = "/api/v1/cubes/KEQ0000100"
	before := len(fc.Received())
	code, out := do(t, ts, "PUT", base+"/devices/0a0b0c", `{"name":"Neu","room_id":1}`)
	if code != 200 {
		t.Fatalf("add to room 1: %d %v", code, out)
	}
	mode, _ := cube.CmdSetTemperature(cube.Target{RFAddress: copyNew}, 0, cube.ModeAuto, cube.Until{})
	valve, _ := cube.CmdSetValveConfig(cube.Target{RFAddress: copyNew}, cube.ValveSettings{BoostMinutes: 5, BoostValvePct: 80, DecalcDay: cube.Saturday,
		DecalcHour: 12, MaxValvePct: 100})
	want := []string{"z:1e,G,01",
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, true, copyNew, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyNew}, true, 0x0a010a, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyWall}, true, copyNew, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyNew}, true, copyWall, cube.DeviceWallThermostat).String(),
		cube.CmdSetGroup(copyNew, 1, false).String(),
		"m:", "c:0a0b0c",
		mode.String(),
		"z:1e,G,01", roomTemps(t, 0x0a010a, 0), roomTemps(t, copyNew, 0), roomTemps(t, copyWall, -0.5),
		"z:1e,G,01", valve.String(),
		"z:1e,G,01"}
	prog := s.cubes["keq0000100"].client.Snapshot().Config(0x0a010a).Thermostat.WeekProgram
	for _, d := range []cube.Weekday{cube.Monday, cube.Tuesday, cube.Wednesday, cube.Thursday, cube.Friday} {
		ls, err := cube.CmdSetDayProgram(cube.Target{RFAddress: copyNew}, d, prog[d])
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, lineStrings(ls)...)
	}
	want = append(want, roomTemps(t, copyNew, 0))
	sameLines(t, fc.Received()[before:], want)
	copied, _ := out["copied"].(map[string]any)
	if temps, _ := copied["temperatures"].([]any); len(temps) != 3 || temps[0] != "0a010a" || temps[1] != "0a0b0c" || temps[2] != "0c0d0e" ||
		copied["valve"] != true || copied["days"] != float64(5) {
		t.Fatalf("copied %v", out["copied"])
	}
	if code, d := do(t, ts, "GET", base+"/devices/0a0b0c", ""); code != 200 || d["unlisted"] == true || d["room_id"] != float64(1) {
		t.Fatalf("device after the add: %d %v", code, d)
	}
}

// "copy": false puts the device into the room as before: the links, the
// group, the metadata and the re-read, nothing of the room's settings.
func TestAddWithoutCopy(t *testing.T) {
	fc, ts, _ := startCopyCube(t, nil)
	before := len(fc.Received())
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0b0c", `{"room_id":1,"copy":false}`)
	if code != 200 || out["copied"] != nil {
		t.Fatalf("add without copy: %d %v", code, out)
	}
	sameLines(t, fc.Received()[before:], []string{"z:1e,G,01",
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, true, copyNew, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyNew}, true, 0x0a010a, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyWall}, true, copyNew, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyNew}, true, copyWall, cube.DeviceWallThermostat).String(),
		cube.CmdSetGroup(copyNew, 1, false).String(), "m:", "c:0a0b0c"})
}

// A shutter contact has no settings: it is linked and listed, nothing
// is copied.
func TestAddContactCopiesNothing(t *testing.T) {
	fc, ts, _ := startCopyCube(t, nil)
	before := len(fc.Received())
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0d0e0f", `{"name":"Fenster","room_id":1}`)
	if code != 200 || out["copied"] != nil {
		t.Fatalf("add a contact: %d %v", code, out)
	}
	sameLines(t, fc.Received()[before:], []string{"z:1e,G,01",
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, true, copyContact, cube.DeviceShutterContact).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyContact}, true, 0x0a010a, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyWall}, true, copyContact, cube.DeviceShutterContact).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyContact}, true, copyWall, cube.DeviceWallThermostat).String(),
		cube.CmdSetGroup(copyContact, 1, false).String(), "m:", "c:0d0e0f"})
}

// A move copies the new room's settings after the move's own lines:
// 0a010d from room 6 into room 1 gets 0x40, 0x11 goes to the room's
// three thermostats, then 0x12, every day of room 1's programme (none is
// 0a010d's) and 0x11 again.
func TestMoveCopiesTheNewRoomsSettings(t *testing.T) {
	fc, ts, _ := startCopyCube(t, nil)
	before := len(fc.Received())
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a010d", `{"room_id":1}`)
	if code != 200 {
		t.Fatalf("move: %d %v", code, out)
	}
	got := fc.Received()[before:]
	reread := -1
	for i, m := range got {
		if m.String() == "c:0a010d" {
			reread = i
		}
	}
	if reread < 0 || got[reread-1].Type != 'm' {
		t.Fatalf("no metadata and re-read before the copy: %v", lineStrings(got))
	}
	var kinds []string
	for _, m := range got[reread+1:] {
		if m.Type != 's' {
			kinds = append(kinds, m.String())
			continue
		}
		rc, err := cube.ParseRadioCommand(m.Data)
		if err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, fmt.Sprintf("%s:%02x", rc.Target.RFAddress, rc.Command))
	}
	want := "0a010d:40 z:1e,G,01 0a010d:11 0a010a:11 0c0d0e:11 z:1e,G,01 0a010d:12 z:1e,G,01 " + strings.Repeat("0a010d:10 ", 7) + "0a010d:11"
	if strings.Join(kinds, " ") != want {
		t.Fatalf("copy after the move:\n%s\nwant\n%s", strings.Join(kinds, " "), want)
	}
	if copied, _ := out["copied"].(map[string]any); copied["days"] != float64(7) || copied["valve"] != true {
		t.Fatalf("copied %v", out["copied"])
	}
}

// A discarded 0x11 ends the copy: the answer is the error, which says
// the device is in the room, and nothing after it is sent.
func TestCopyStopsAtADiscard(t *testing.T) {
	fc, ts, _ := startCopyCube(t, func(fc *fakecube.Server) {
		fc.DiscardIf = func(m cube.Message) bool {
			rc, err := cube.ParseRadioCommand(m.Data)
			return m.Type == 's' && err == nil && rc.Command == cube.RFSetTemperatures
		}
	})
	const base = "/api/v1/cubes/KEQ0000100"
	code, out := do(t, ts, "PUT", base+"/devices/0a0b0c", `{"room_id":1}`)
	if msg, _ := out["error"].(string); code != 502 || !strings.Contains(msg, "device 0a0b0c is in room 1, but copying the room's settings stopped") {
		t.Fatalf("a discarded 0x11: %d %v", code, out)
	}
	for _, l := range sLines(fc) {
		if l[2] == cube.RFSetValveConfig || l[2] == cube.RFSetWeekProgram {
			t.Fatalf("sent after the discard: % x", l)
		}
	}
	if code, d := do(t, ts, "GET", base+"/devices/0a0b0c", ""); code != 200 || d["unlisted"] == true || d["room_id"] != float64(1) {
		t.Fatalf("device after the failed copy: %d %v", code, d)
	}
}

// withConfigRoom sets the room byte of a C line of the greeting.
func withConfigRoom(t *testing.T, greeting []cube.Message, rf cube.RFAddress, room uint8) {
	t.Helper()
	for i, g := range greeting {
		if g.Type == 'C' && strings.HasPrefix(g.Data, rf.String()+",") {
			raw := rawConfig(t, greeting, rf)
			greeting[i] = configLine(raw, rf, room, string(raw[8:18]))
			return
		}
	}
	t.Fatalf("no C line for %s", rf)
}

// logKinds returns the kinds of the Cube's log events from the n-th on.
func logKinds(t *testing.T, ts *httptest.Server, n int) []string {
	t.Helper()
	var evs []event
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log?limit=500", &evs)
	var kinds []string
	for i, e := range evs {
		if i >= n {
			kinds = append(kinds, e.Kind+":"+e.Device)
		}
	}
	return kinds
}

// hasKind reports whether kinds holds kind.
func hasKind(kinds []string, kind string) bool {
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// listedRF reports whether the device list, or a room's, names rf.
func listedRF(t *testing.T, ts *httptest.Server, rf string) (device, inRoom bool) {
	t.Helper()
	var devs []map[string]any
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices", &devs)
	for _, d := range devs {
		device = device || d["rf_address"] == rf
	}
	var rooms []map[string]any
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/rooms", &rooms)
	for _, r := range rooms {
		for _, d := range r["devices"].([]any) {
			inRoom = inRoom || d.(map[string]any)["rf_address"] == rf
		}
	}
	return device, inRoom
}

// A listed thermostat is deleted as the original software deletes it:
// after a wakeup of its room it is unlinked from the room's wall
// thermostat in both directions and its group removed, then t: with
// the force flag, then the metadata without it; it is gone from the
// devices, the rooms and the counts, and the metadata event came.
func TestRemoveListedDevice(t *testing.T) {
	fc, ts, s := startCopyCube(t, nil)
	const base = "/api/v1/cubes/KEQ0000100"
	var before cubeView
	getJSON(t, ts, "", base, &before)
	events := len(logKinds(t, ts, 0))
	sent := len(fc.Received())
	if code, out := do(t, ts, "DELETE", base+"/devices/0a010a", ""); code != 200 || out["ok"] != true || out["budget"] == nil {
		t.Fatalf("remove: %d %v", code, out)
	}
	sameLines(t, fc.Received()[sent:], []string{"z:1e,G,01",
		cube.CmdLinkPartner(cube.Target{RFAddress: copyWall}, false, 0x0a010a, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a}, false, copyWall, cube.DeviceWallThermostat).String(),
		cube.CmdSetGroup(0x0a010a, 1, true).String(),
		"t:01,1,CgEK", "m:"})
	for _, d := range s.cubes["keq0000100"].client.Snapshot().Metadata().Devices {
		if d.RFAddress == 0x0a010a {
			t.Fatal("the metadata still lists the device")
		}
	}
	if device, inRoom := listedRF(t, ts, "0a010a"); device || inRoom {
		t.Fatalf("still listed: device %v, in a room %v", device, inRoom)
	}
	if code := getJSON(t, ts, "", base+"/devices/0a010a", nil); code != 404 {
		t.Fatalf("GET the removed device: %d", code)
	}
	var after cubeView
	getJSON(t, ts, "", base, &after)
	if after.Devices != before.Devices-1 || after.Rooms != before.Rooms {
		t.Fatalf("counts %d devices %d rooms, before %d and %d", after.Devices, after.Rooms, before.Devices, before.Rooms)
	}
	waitFor(t, "the events", func() bool {
		kinds := logKinds(t, ts, events)
		return hasKind(kinds, "metadata:") && hasKind(kinds, "device_forgotten:0a010a")
	})
}

// A device in no room by the metadata and by its C line gets t: alone.
func TestRemoveUnlistedDevice(t *testing.T) {
	fc, ts, _ := startCopyCube(t, nil)
	events := len(logKinds(t, ts, 0))
	sent := len(fc.Received())
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0a0b0c", ""); code != 200 {
		t.Fatalf("remove: %d %v", code, out)
	}
	sameLines(t, fc.Received()[sent:], []string{"t:01,1,CgsM"})
	if device, _ := listedRF(t, ts, "0a0b0c"); device {
		t.Fatal("still among the devices")
	}
	waitFor(t, "device_forgotten", func() bool { return hasKind(logKinds(t, ts, events), "device_forgotten:0a0b0c") })
}

// A device in no room by the metadata whose C line names room 1, as one
// paired long ago, is unlinked from room 1's members before t:; no
// metadata is written for it.
func TestRemoveDeviceInARoomByItsConfig(t *testing.T) {
	fc, ts, _ := startCopyCube(t, func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyContact, 1) })
	sent := len(fc.Received())
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0d0e0f", ""); code != 200 {
		t.Fatalf("remove: %d %v", code, out)
	}
	sameLines(t, fc.Received()[sent:], []string{"z:1e,G,01",
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, false, copyContact, cube.DeviceShutterContact).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyContact}, false, 0x0a010a, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyWall}, false, copyContact, cube.DeviceShutterContact).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyContact}, false, copyWall, cube.DeviceWallThermostat).String(),
		cube.CmdSetGroup(copyContact, 1, true).String(),
		"t:01,1,DQ4P"})
}

// A hidden device is not found, the Cube itself and a bad address are
// refused, and nothing reaches the Cube.
func TestRemoveRefused(t *testing.T) {
	fc, ts, s := start(t, "")
	s.cubes["keq0000100"].setConfig(func(c *CubeConfig) { c.Hidden = []cube.RFAddress{0x0a010d} })
	for path, want := range map[string]int{"0a010d": 404, "0a0107": 400, "zz": 400, "0f0f0f": 404} {
		if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/"+path, ""); code != want {
			t.Fatalf("DELETE %s: %d %v, want %d", path, code, out, want)
		}
	}
	if got := fc.Received(); len(got) != 0 {
		t.Fatalf("cube received %v", lineStrings(got))
	}
}

// The Cube discarding the copy's 0x40 ends nothing: the rest of the
// copy follows, the answer says the mode was not set, and no error
// reset is sent for it.
func TestCopyAfterADiscardedMode(t *testing.T) {
	fc, ts, _ := startCopyCube(t, func(fc *fakecube.Server) {
		fc.DiscardIf = func(m cube.Message) bool {
			rc, err := cube.ParseRadioCommand(m.Data)
			return m.Type == 's' && err == nil && rc.Command == cube.RFSetTemperature
		}
	})
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0b0c", `{"room_id":1}`)
	copied, _ := out["copied"].(map[string]any)
	if code != 200 || copied["mode_not_set"] != true || copied["valve"] != true || copied["days"] != float64(5) {
		t.Fatalf("add with a discarded 0x40: %d %v", code, out)
	}
	if n := countType(fc, 'r'); n != 0 {
		t.Fatalf("%d error resets sent", n)
	}
}

// A hidden device in the room is neither linked with the new device nor
// given the room's temperatures, nor taken for the room's reference.
func TestCopyKeepsAwayFromHiddenDevices(t *testing.T) {
	fc, ts, s := startCopyCube(t, nil)
	s.cubes["keq0000100"].setConfig(func(c *CubeConfig) { c.Hidden = []cube.RFAddress{copyWall} })
	before := len(fc.Received())
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0b0c", `{"room_id":1}`)
	if code != 200 {
		t.Fatalf("add: %d %v", code, out)
	}
	w := copyWall.Bytes()
	for _, m := range fc.Received()[before:] {
		raw, _ := base64.StdEncoding.DecodeString(m.Data)
		if m.Type == 's' && len(raw) >= 10 && cube.RFAddressFromBytes(raw[6:9]) == copyWall || strings.Contains(string(raw), string(w[:])) {
			t.Fatalf("a line for the hidden device: %s", m)
		}
	}
	if temps, _ := out["copied"].(map[string]any)["temperatures"].([]any); len(temps) != 2 || temps[0] != "0a010a" || temps[1] != "0a0b0c" {
		t.Fatalf("copied %v", out["copied"])
	}
}

// Two thermostats put into one room at once: each is linked, listed in
// the room and given its own copy, the second request reading the
// metadata the first one wrote.
func TestConcurrentAddsIntoOneRoom(t *testing.T) {
	fc, ts, s := startCopyCube(t, nil)
	answers := make(chan map[string]any, 2)
	for _, rf := range []string{"0a0b0c", "0b0c0d"} {
		go func() {
			code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/"+rf, `{"room_id":1}`)
			if code != 200 {
				out = map[string]any{"code": code, "answer": out}
			}
			answers <- out
		}()
	}
	for range 2 {
		if out := <-answers; out["ok"] != true || out["copied"].(map[string]any)["valve"] != true {
			t.Fatalf("an add: %v", out)
		}
	}
	listed := 0
	for _, d := range s.cubes["keq0000100"].client.Snapshot().Metadata().Devices {
		if (d.RFAddress == copyNew || d.RFAddress == copyNew2) && d.RoomID == 1 {
			listed++
		}
	}
	valves := map[cube.RFAddress]int{}
	for _, l := range sLines(fc) {
		if l[2] == cube.RFSetValveConfig {
			valves[cube.RFAddressFromBytes(l[6:9])]++
		}
	}
	if listed != 2 || valves[copyNew] != 1 || valves[copyNew2] != 1 {
		t.Fatalf("%d of 2 listed in room 1, valve settings sent %v", listed, valves)
	}
}

// A removal keeps away from hidden devices: a hidden partner is not
// unlinked, and a device whose C line names room 255 is not unlinked
// at all.
func TestRemoveLeavesHiddenDevicesAlone(t *testing.T) {
	fc, ts, s := startCopyCube(t, func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyContact, cube.HiddenRoom) })
	s.cubes["keq0000100"].setConfig(func(c *CubeConfig) { c.Hidden = []cube.RFAddress{copyWall} })
	sent := len(fc.Received())
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0a010a", ""); code != 200 {
		t.Fatalf("remove 0a010a: %d %v", code, out)
	}
	sameLines(t, fc.Received()[sent:], []string{"z:1e,G,01", cube.CmdSetGroup(0x0a010a, 1, true).String(), "t:01,1,CgEK", "m:"})
	sent = len(fc.Received())
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0d0e0f", ""); code != 200 {
		t.Fatalf("remove 0d0e0f: %d %v", code, out)
	}
	sameLines(t, fc.Received()[sent:], []string{"t:01,1,DQ4P"})
}

// A device only the metadata lists, without a C line, loses its entry;
// nothing is sent by radio.
func TestRemoveListedOnly(t *testing.T) {
	fc, ts, _ := startCopyCube(t, func(fc *fakecube.Server) {
		kept := fc.Greeting[:0]
		for _, g := range fc.Greeting {
			if g.Type != 'C' || !strings.HasPrefix(g.Data, "0a010c,") {
				kept = append(kept, g)
			}
		}
		fc.Greeting = kept
	})
	sent := len(fc.Received())
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0a010c", ""); code != 200 {
		t.Fatalf("remove: %d %v", code, out)
	}
	sameLines(t, fc.Received()[sent:], []string{"m:"})
	if device, inRoom := listedRF(t, ts, "0a010c"); device || inRoom {
		t.Fatalf("still listed: device %v, in a room %v", device, inRoom)
	}
}

// A device the Cube keeps after t:, as one that never answers: the
// Cube's next device list does not bring it back, the next greeting does.
func TestRemovedDeviceTheCubeKeeps(t *testing.T) {
	_, ts, s := startCopyCube(t, func(fc *fakecube.Server) { fc.Keep = map[cube.RFAddress]bool{0x0a010a: true} })
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0a010a", ""); code != 200 {
		t.Fatalf("remove: %d %v", code, out)
	}
	rt := s.cubes["keq0000100"]
	if _, err := rt.client.Send(t.Context(), cube.CmdList); err != nil {
		t.Fatal(err)
	}
	if device, _ := listedRF(t, ts, "0a010a"); device {
		t.Fatal("the device list brought it back")
	}
	if res := rt.queue.Enqueue(engine.Command{Reconnect: true}).Wait(t.Context()); res.Err != nil {
		t.Fatal(res.Err)
	}
	waitFor(t, "the device after the greeting", func() bool { return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/0a010a", nil) == 200 })
}

// A removal and an add at once: each reads the metadata the other wrote,
// so the added device is listed and the removed one is not.
func TestRemoveAndAddAtOnce(t *testing.T) {
	_, ts, s := startCopyCube(t, nil)
	codes := make(chan int, 2)
	go func() {
		code, _ := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0a010a", "")
		codes <- code
	}()
	go func() {
		code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0b0c", `{"room_id":1,"copy":false}`)
		codes <- code
	}()
	for range 2 {
		if code := <-codes; code != 200 {
			t.Fatalf("a request answered %d", code)
		}
	}
	added, removed := false, true
	for _, d := range s.cubes["keq0000100"].client.Snapshot().Metadata().Devices {
		added = added || d.RFAddress == copyNew && d.RoomID == 1
		removed = removed && d.RFAddress != 0x0a010a
	}
	if !added || !removed {
		t.Fatalf("metadata: added %v, removed %v", added, removed)
	}
}
