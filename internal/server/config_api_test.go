// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
	"go.metaworx.ch/massimo/internal/view"
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

// radioBytes decodes s: lines, as a watch of the fake Cube's lines gives
// them (cubewatch.Type('s')).
func radioBytes(lines []cube.Message) [][]byte {
	var out [][]byte
	for _, m := range lines {
		raw, _ := base64.StdEncoding.DecodeString(m.Data)
		out = append(out, raw)
	}
	return out
}

func TestConfigEndpoints(t *testing.T) {
	t.Parallel()
	rg := startServer(t)
	fc, ts, s := rg.fc, rg.ts, rg.s
	received := cubewatch.Lines(fc)
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
	if n := received.Count(0, cubewatch.Type('m')); n != 2 {
		t.Fatalf("metadata writes: %d", n)
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
	before := received.Mark()
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/"+rf+"/config", `{"eco_temp":16.5}`); code != 200 || out["ok"] != true {
		t.Fatalf("device config: %d %v", code, out)
	}
	got := received.Since(before, cubewatch.Except('l'))
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
	sent := received.Mark()
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6/program", `{"monday":[{"temp":17,"until":"07:00"},{"temp":21,"until":"24:00"}]}`); code != 200 || out["ok"] != true {
		t.Fatalf("room programme: %d %v", code, out)
	}
	lines := radioBytes(received.Since(sent, cubewatch.Type('s')))
	if len(lines) != 1 {
		t.Fatalf("room programme sent %d radiograms", len(lines))
	}
	l := lines[0]
	if l[1] != 0x04 || l[2] != cube.RFSetWeekProgram || l[9] != 6 || l[10] != byte(cube.Monday) {
		t.Fatalf("room programme line % x", l)
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6/program", `{"monday":[{"temp":17,"until":"07:00"}]}`); code != 400 {
		t.Fatalf("programme not ending at 24:00 accepted: %d", code)
	}
	// Device week programme: direct, two days.
	sent = received.Mark()
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/"+rf+"/program", `{"saturday":[{"temp":18,"until":"24:00"}],"sunday":[{"temp":18,"until":"24:00"}]}`); code != 200 {
		t.Fatalf("device programme: %d", code)
	}
	lines = radioBytes(received.Since(sent, cubewatch.Type('s')))
	if len(lines) != 2 || lines[0][1] != 0 || lines[0][9] != 0 || lines[0][10] != byte(cube.Saturday) {
		t.Fatalf("device programme lines % x", lines)
	}

	// Move the device to room 1: leave (group remove), join (one link to
	// the room with its flag, one to the device, group set), metadata; the copy of the
	// room's settings, which follows by default, has tests of its own
	// (TestAddCopiesTheRoomsSettings and those after it).
	sent = received.Mark()
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/"+rf, `{"room_id":1,"copy":false}`); code != 200 {
		t.Fatalf("move: %d %v", code, out)
	}
	var cmds []byte
	for _, l := range radioBytes(received.Since(sent, cubewatch.Type('s'))) {
		cmds = append(cmds, l[2])
	}
	if nM := received.Count(sent, cubewatch.Type('m')); string(cmds) != string([]byte{cube.RFRemoveGroup, cube.RFAddLinkPartner, cube.RFAddLinkPartner, cube.RFSetGroupAddress}) || nM != 1 {
		t.Fatalf("move sent % x, metadata writes %d", cmds, nM)
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
// and a shutter contact in no room. The Cube holds a valid state for
// each, and for 0a010a, the thermostats in auto at 21 °C, which the
// programme of room 1 has at the fixture's Tuesday 19:51; the
// fixture's other devices hold none, as recorded.
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

// validEntry is an L entry with a valid state (the valid and the
// initialized bit of its first flags byte), laid out as the fake Cube's
// fixtures lay theirs out: a thermostat in auto at setpoint, a window
// contact closed.
func validEntry(t *testing.T, rf cube.RFAddress, typ cube.DeviceType, setpoint float64) []byte {
	t.Helper()
	a := rf.Bytes()
	var e []byte
	switch {
	case typ == cube.DeviceShutterContact:
		return []byte{0x06, a[0], a[1], a[2], 0x00, 0x12, 0x18}
	case typ == cube.DeviceWallThermostat:
		e = []byte{0x0c, a[0], a[1], a[2], 0x00, 0x12, 0x18, 0x01, 0, 0, 0, 0, 0}
	default:
		e = []byte{0x0b, a[0], a[1], a[2], 0x00, 0x12, 0x18, 0, 0, 0, 0, 0}
	}
	sp, err := cube.EncodeTemp(setpoint)
	if err != nil {
		t.Fatal(err)
	}
	e[8] = sp
	return e
}

// withValidStates gives the devices of states a valid state in the
// greeting's L line (validEntry, at their setpoints), in place of the
// entry they have, else as a new one; the type is their C line's.
func withValidStates(t *testing.T, lines []cube.Message, states map[cube.RFAddress]float64) []cube.Message {
	t.Helper()
	out := append([]cube.Message(nil), lines...)
	types := map[cube.RFAddress]cube.DeviceType{}
	for rf := range states {
		types[rf] = cube.DeviceType(rawConfig(t, lines, rf)[4])
	}
	for i, m := range out {
		if m.Type != 'L' {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(m.Data)
		if err != nil {
			t.Fatal(err)
		}
		var list []byte
		done := map[cube.RFAddress]bool{}
		for j := 0; j < len(raw); j += int(raw[j]) + 1 {
			e := raw[j : j+int(raw[j])+1]
			rf := cube.RFAddressFromBytes(e[1:4])
			if sp, ok := states[rf]; ok {
				e, done[rf] = validEntry(t, rf, types[rf], sp), true
			}
			list = append(list, e...)
		}
		for _, rf := range slices.Sorted(maps.Keys(states)) {
			if !done[rf] {
				list = append(list, validEntry(t, rf, types[rf], states[rf])...)
			}
		}
		out[i].Data = base64.StdEncoding.EncodeToString(list)
	}
	return out
}

// copyGreeting is the fixture with the copy tests' devices.
func copyGreeting(t *testing.T) []cube.Message {
	t.Helper()
	greeting := recorded(t)
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
	return withValidStates(t, out, map[cube.RFAddress]float64{0x0a010a: 21, copyWall: 21, copyNew: 21, copyContact: 0, copyNew2: 21})
}

// lineStrings writes protocol lines as the Cube received them.
func lineStrings(lines []cube.Message) []string {
	var out []string
	for _, l := range lines {
		out = append(out, l.String())
	}
	return out
}

// lineIs matches the line that reads as line ("c:0a010a").
func lineIs(line string) func(cube.Message) bool {
	return func(m cube.Message) bool { return m.String() == line }
}

// sameLines fails the test where the lines the fake Cube received from
// the cursor on, the polls left out (they come whenever the poll's clock
// says), differ from want; a want of "m:" stands for any metadata write.
func sameLines(t *testing.T, received *testkit.Watch[cube.Message], from testkit.Cursor, want []string) {
	t.Helper()
	got := received.Since(from, cubewatch.Except('l'))
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
// the room's state, its thermostats being on their programme, 0x11 to
// the room's three thermostats, 0x12, and the five days that differ
// followed by 0x11 again; the radiator thermostat 0a010a, the master,
// gives the temperatures, which the wall thermostat does not share
// (RoomMajority.D9), and the valve settings, which it alone has.
func TestAddCopiesTheRoomsSettings(t *testing.T) {
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)))
	fc, ts, s := r.fc, r.ts, r.s
	const base = "/api/v1/cubes/KEQ0000100"
	received := cubewatch.Lines(fc)
	before := received.Mark()
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
	sameLines(t, received, before, want)
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
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)))
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	before := received.Mark()
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0b0c", `{"room_id":1,"copy":false}`)
	if code != 200 || out["copied"] != nil {
		t.Fatalf("add without copy: %d %v", code, out)
	}
	sameLines(t, received, before, []string{"z:1e,G,01",
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, true, copyNew, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyNew}, true, 0x0a010a, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyNew}, true, copyWall, cube.DeviceWallThermostat).String(),
		cube.CmdSetGroup(copyNew, 1, false).String(), "m:", "c:0a0b0c"})
}

// A placement without a name lists the device under the default a
// repair's listing gives (D200-A), "<room name> <type> <n>", n the
// lowest the room's names of that form leave free; the metadata written
// is exactly the Cube's with the device's entry added. Where the whole
// would pass 32 bytes, the room's name is dropped (D201): nobody is
// there to shorten it. A name given is taken as it is. The copy cube's
// room 1 is "Wohnzimmer".
func TestAddWithoutANameTakesTheDefault(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		body  string
		room  string                    // room 1 renamed
		names map[cube.RFAddress]string // room 1's devices renamed
		want  string
	}{
		{"the default", `{"room_id":1,"copy":false}`, "", nil, "Wohnzimmer radiator thermostat 1"},
		{"a taken n", `{"room_id":1,"copy":false}`, "", map[cube.RFAddress]string{0x0a010a: "Wohnzimmer radiator thermostat 1"},
			"Wohnzimmer radiator thermostat 2"},
		{"a room's name too long", `{"room_id":1,"copy":false}`, "Wohnzimmer Süd", nil, "radiator thermostat 1"},
		{"a name given", `{"name":" Neu ","room_id":1,"copy":false}`, "", nil, "Neu"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := startServer(t, withGreeting(copyGreeting(t)), withFake(func(fc *fakecube.Server) {
				withMetadata(t, fc.Greeting, func(md *cube.Metadata) {
					for i := range md.Devices {
						if n, ok := c.names[md.Devices[i].RFAddress]; ok {
							md.Devices[i].Name = n
						}
					}
					for i := range md.Rooms {
						if md.Rooms[i].ID == 1 && c.room != "" {
							md.Rooms[i].Name = c.room
						}
					}
				})
			}))
			fc, ts, s := r.fc, r.ts, r.s
			md := *s.cubes["keq0000100"].client.Snapshot().Metadata()
			received := cubewatch.Lines(fc)
			before := received.Mark()
			if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0b0c", c.body); code != 200 {
				t.Fatalf("add to room 1: %d %v", code, out)
			}
			md.Devices = append(append([]cube.DeviceMeta(nil), md.Devices...),
				cube.DeviceMeta{Type: cube.DeviceRadiatorThermostat, RFAddress: copyNew, Serial: "TEQ0000001", Name: c.want, RoomID: 1})
			lines, err := cube.WriteMetadataCommands(&md)
			if err != nil || len(lines) != 1 {
				t.Fatalf("metadata %v %v", lines, err)
			}
			sameLines(t, received, before, []string{"z:1e,G,01",
				cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, true, copyNew, cube.DeviceRadiatorThermostat).String(),
				cube.CmdLinkPartner(cube.Target{RFAddress: copyNew}, true, 0x0a010a, cube.DeviceRadiatorThermostat).String(),
				cube.CmdLinkPartner(cube.Target{RFAddress: copyNew}, true, copyWall, cube.DeviceWallThermostat).String(),
				cube.CmdSetGroup(copyNew, 1, false).String(), lines[0].String(), "c:0a0b0c"})
			if code, d := do(t, ts, "GET", "/api/v1/cubes/KEQ0000100/devices/0a0b0c", ""); code != 200 || d["name"] != c.want {
				t.Fatalf("device after the add: %d %v", code, d)
			}
		})
	}
}

// A shutter contact has no settings: it is linked and listed, nothing
// is copied.
func TestAddContactCopiesNothing(t *testing.T) {
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)))
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	before := received.Mark()
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0d0e0f", `{"name":"Fenster","room_id":1}`)
	if code != 200 || out["copied"] != nil {
		t.Fatalf("add a contact: %d %v", code, out)
	}
	sameLines(t, received, before, []string{"z:1e,G,01",
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, true, copyContact, cube.DeviceShutterContact).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyContact}, true, 0x0a010a, cube.DeviceRadiatorThermostat).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: copyContact}, true, copyWall, cube.DeviceWallThermostat).String(),
		cube.CmdSetGroup(copyContact, 1, false).String(), "m:", "c:0d0e0f"})
}

// A move copies the new room's settings after the move's own lines:
// 0a010d from room 6 into room 1 gets 0x40, 0x11 goes to the room's
// three thermostats, then 0x12, every day of room 1's programme (none is
// 0a010d's) and 0x11 again.
func TestMoveCopiesTheNewRoomsSettings(t *testing.T) {
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)))
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	before := received.Mark()
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a010d", `{"room_id":1}`)
	if code != 200 {
		t.Fatalf("move: %d %v", code, out)
	}
	got := received.Since(before, cubewatch.Except('l'))
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
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)), withFake(func(fc *fakecube.Server) {
		fc.DiscardIf = func(m cube.Message) bool {
			rc, err := cube.ParseRadioCommand(m.Data)
			return m.Type == 's' && err == nil && rc.Command == cube.RFSetTemperatures
		}
	}))
	fc, ts := r.fc, r.ts
	const base = "/api/v1/cubes/KEQ0000100"
	code, out := do(t, ts, "PUT", base+"/devices/0a0b0c", `{"room_id":1}`)
	if msg, _ := out["error"].(string); code != 502 || !strings.Contains(msg, "device 0a0b0c is in room 1, but copying the room's settings stopped") {
		t.Fatalf("a discarded 0x11: %d %v", code, out)
	}
	for _, l := range radioBytes(cubewatch.Lines(fc).Since(0, cubewatch.Type('s'))) {
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
	t.Parallel()
	// The start has the greeting through the pump: its metadata is what
	// the removal's metadata line is measured against.
	r := startServer(t, withGreeting(copyGreeting(t)))
	fc, ts, s := r.fc, r.ts, r.s
	const base = "/api/v1/cubes/KEQ0000100"
	var before cubeView
	getJSON(t, ts, "", base, &before)
	m := logMark(t, ts)
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	if code, out := do(t, ts, "DELETE", base+"/devices/0a010a", ""); code != 200 || out["ok"] != true || out["budget"] == nil {
		t.Fatalf("remove: %d %v", code, out)
	}
	// As the original software removed one on 2026-11-15: one 0x21 with
	// the room flag naming the device, to the wall thermostat, the master
	// being the device itself; no 0x23 and nothing to the device.
	sameLines(t, received, sent, []string{"z:1e,G,01",
		cube.CmdLinkPartner(cube.Target{RFAddress: copyWall, RoomID: 1, Room: true}, false, 0x0a010a, cube.DeviceRadiatorThermostat).String(),
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
	// The list written without the device is a metadata line of that one
	// device (AP Logging v1.1, Block 4).
	logAfter(t, m, "the metadata line", logLine("metadata", "0a010a"))
	logAfter(t, m, "device_forgotten", logLine("device_forgotten", "0a010a"))
}

// A device in no room by the metadata and by its C line gets t: alone.
func TestRemoveUnlistedDevice(t *testing.T) {
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)))
	fc, ts := r.fc, r.ts
	m := logMark(t, ts)
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0a0b0c", ""); code != 200 {
		t.Fatalf("remove: %d %v", code, out)
	}
	sameLines(t, received, sent, []string{"t:01,1,CgsM"})
	if device, _ := listedRF(t, ts, "0a0b0c"); device {
		t.Fatal("still among the devices")
	}
	logAfter(t, m, "device_forgotten", logLine("device_forgotten", "0a0b0c"))
}

// A device in no room by the metadata whose C line names room 1, as one
// paired long ago, is unlinked from room 1's members before t:, by one
// 0x21 to the room's master; no metadata is written for it.
func TestRemoveDeviceInARoomByItsConfig(t *testing.T) {
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)), withFake(func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyContact, 1) }))
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0d0e0f", ""); code != 200 {
		t.Fatalf("remove: %d %v", code, out)
	}
	sameLines(t, received, sent, []string{"z:1e,G,01",
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, false, copyContact, cube.DeviceShutterContact).String(),
		"t:01,1,DQ4P"})
}

// withErrorFlag sets the error flag (byte 5's 0x08) of rf's entry in the
// greeting's device list, with byte 4 as the failed radiogram.
func withErrorFlag(t *testing.T, greeting []cube.Message, rf cube.RFAddress, failed byte) {
	t.Helper()
	for i, g := range greeting {
		if g.Type != 'L' {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(g.Data)
		if err != nil {
			t.Fatal(err)
		}
		for off := 0; off < len(raw); off += int(raw[off]) + 1 {
			if cube.RFAddressFromBytes(raw[off+1:off+4]) == rf {
				raw[off+4], raw[off+5] = failed, raw[off+5]|0x08
				greeting[i] = cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)}
				return
			}
		}
	}
	t.Fatalf("no entry for %s", rf)
}

// A removal resets every flagged member of the room first (D161-B), then
// sends its one 0x21 to the room's master, which names the wall
// thermostat leaving.
func TestRemoveResetsFlaggedMembers(t *testing.T) {
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)), withFake(func(fc *fakecube.Server) { withErrorFlag(t, fc.Greeting, 0x0a010a, cube.RFRemoveLink) }))
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0c0d0e", ""); code != 200 {
		t.Fatalf("remove the wall thermostat: %d %v", code, out)
	}
	sameLines(t, received, sent, []string{"z:1e,G,01",
		cube.CmdResetError(0x0a010a).String(),
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, false, copyWall, cube.DeviceWallThermostat).String(),
		cube.CmdDeleteDevices(true, copyWall).String(), "m:"})
}

// A room setpoint resets every flagged member of the room first, by the
// members' C lines (AP RadioHealth v1.2, Block 2, D110-A): 0a0109, put
// into room 1 by its C line, and 0a010a, the room's master, which the
// command names; the wall thermostat, unflagged, is left alone.
func TestRoomSetpointResetsFlaggedMembers(t *testing.T) {
	t.Parallel()
	rg := startServer(t, withGreeting(copyGreeting(t)), withFake(func(fc *fakecube.Server) {
		withConfigRoom(t, fc.Greeting, 0x0a0109, 1)
		withErrorFlag(t, fc.Greeting, 0x0a0109, 0x03)
		withErrorFlag(t, fc.Greeting, 0x0a010a, 0x03)
	}))
	fc, ts := rg.fc, rg.ts
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/1/setpoint", `{"mode":"auto"}`); code != 200 {
		t.Fatalf("room setpoint: %d %v", code, out)
	}
	m, err := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, 0, cube.ModeAuto, cube.Until{})
	if err != nil {
		t.Fatal(err)
	}
	radio := lineStrings(received.Since(sent, cubewatch.Type('r', 's')))
	if want := []string{cube.CmdResetError(0x0a0109).String(), cube.CmdResetError(0x0a010a).String(), m.String()}; !slices.Equal(radio, want) {
		t.Fatalf("radio lines\n%s\nwant\n%s", strings.Join(radio, "\n"), strings.Join(want, "\n"))
	}
}

// A step's result gives each line's answer, and the line the Cube
// discarded: here the removal's 0x21.
func TestStepNamesTheDiscardedLine(t *testing.T) {
	t.Parallel()
	discard := func(m cube.Message) bool {
		rc, err := cube.ParseRadioCommand(m.Data)
		return m.Type == 's' && err == nil && rc.Command == cube.RFRemoveLink
	}
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")), withGreeting(copyGreeting(t)),
		withFake(func(fc *fakecube.Server) { fc.DiscardIf = discard }))
	ts := r.ts
	keepStore(t, ts, r.st)
	if code, text := callAPI(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0a010a", "secret", ""); code != 502 {
		t.Fatalf("remove with the 0x21 discarded: %d %s", code, text)
	}
	unlink := cube.CmdLinkPartner(cube.Target{RFAddress: copyWall, RoomID: 1, Room: true}, false, 0x0a010a, cube.DeviceRadiatorThermostat).String()
	rows := auditRows(t, ts, "secret", "?limit=1")
	if len(rows) != 1 || rows[0].CommandID == 0 {
		t.Fatalf("audit row %+v", rows)
	}
	c := commandOfRow(t, ts, "secret", rows[0])
	for _, step := range c.Steps {
		var r struct {
			Answers       []string `json:"answers"`
			DiscardedLine string   `json:"discarded_line"`
		}
		if json.Unmarshal(step.Result, &r) != nil || r.DiscardedLine == "" {
			continue
		}
		if r.DiscardedLine != unlink || len(r.Answers) != 1 || !strings.HasPrefix(r.Answers[0], "S:") || !strings.Contains(r.Answers[0], ",1,") {
			t.Fatalf("step result %s, want the discarded %s", step.Result, unlink)
		}
		return
	}
	t.Fatalf("no step names the discarded line: %+v", c.Steps)
}

// A hidden device is not found, the Cube itself and a bad address are
// refused, and nothing reaches the Cube.
func TestRemoveRefused(t *testing.T) {
	t.Parallel()
	r := startServer(t)
	fc, ts, s := r.fc, r.ts, r.s
	s.cubes["keq0000100"].setConfig(func(c *CubeConfig) { c.Hidden = []cube.RFAddress{0x0a010d} })
	for path, want := range map[string]int{"0a010d": 404, "0a0107": 400, "zz": 400, "0f0f0f": 404} {
		if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/"+path, ""); code != want {
			t.Fatalf("DELETE %s: %d %v, want %d", path, code, out, want)
		}
	}
	if got := cubewatch.Lines(fc).Since(0, cubewatch.Except('l')); len(got) != 0 {
		t.Fatalf("cube received %v", got)
	}
}

// The Cube discarding the copy's 0x40 ends nothing: the rest of the
// copy follows, the answer says the mode was not set, and no error
// reset is sent for it. The room command of the room's state that then
// follows (RoomMajority.D16-A), discarded here too, ends nothing either.
func TestCopyAfterADiscardedMode(t *testing.T) {
	t.Parallel()
	rg := startServer(t, withGreeting(copyGreeting(t)), withFake(func(fc *fakecube.Server) {
		fc.DiscardIf = func(m cube.Message) bool {
			rc, err := cube.ParseRadioCommand(m.Data)
			return m.Type == 's' && err == nil && rc.Command == cube.RFSetTemperature
		}
	}))
	fc, ts := rg.fc, rg.ts
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0b0c", `{"room_id":1}`)
	copied, _ := out["copied"].(map[string]any)
	if code != 200 || copied["mode_not_set"] != true || copied["room_mode"] != nil || copied["valve"] != true || copied["days"] != float64(5) {
		t.Fatalf("add with a discarded 0x40: %d %v", code, out)
	}
	if room := setTemp(t, room1(0x0a010a), 0, cube.ModeAuto, cube.Until{}); !slices.Contains(lineStrings(cubewatch.Lines(fc).Since(0, cubewatch.Type('s'))), room) {
		t.Fatalf("no room command %s after the discard", room)
	}
	if n := cubewatch.Lines(fc).Count(0, cubewatch.Type('r')); n != 0 {
		t.Fatalf("%d error resets sent", n)
	}
}

// A hidden device in the room is neither linked with the new device nor
// given the room's temperatures, nor taken for the room's reference.
func TestCopyKeepsAwayFromHiddenDevices(t *testing.T) {
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)))
	fc, ts, s := r.fc, r.ts, r.s
	s.cubes["keq0000100"].setConfig(func(c *CubeConfig) { c.Hidden = []cube.RFAddress{copyWall} })
	received := cubewatch.Lines(fc)
	before := received.Mark()
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0b0c", `{"room_id":1}`)
	if code != 200 {
		t.Fatalf("add: %d %v", code, out)
	}
	w := copyWall.Bytes()
	for _, m := range received.Since(before, cubewatch.Except('l')) {
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
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)))
	fc, ts, s := r.fc, r.ts, r.s
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
	for _, l := range radioBytes(cubewatch.Lines(fc).Since(0, cubewatch.Type('s'))) {
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
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)), withFake(func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyContact, cube.HiddenRoom) }))
	fc, ts, s := r.fc, r.ts, r.s
	s.cubes["keq0000100"].setConfig(func(c *CubeConfig) { c.Hidden = []cube.RFAddress{copyWall} })
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0a010a", ""); code != 200 {
		t.Fatalf("remove 0a010a: %d %v", code, out)
	}
	sameLines(t, received, sent, []string{"z:1e,G,01", "t:01,1,CgEK", "m:"})
	sent = received.Mark()
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0d0e0f", ""); code != 200 {
		t.Fatalf("remove 0d0e0f: %d %v", code, out)
	}
	sameLines(t, received, sent, []string{"t:01,1,DQ4P"})
}

// A device only the metadata lists, without a C line, loses its entry;
// nothing is sent by radio.
func TestRemoveListedOnly(t *testing.T) {
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)), withFake(func(fc *fakecube.Server) {
		kept := fc.Greeting[:0]
		for _, g := range fc.Greeting {
			if g.Type != 'C' || !strings.HasPrefix(g.Data, "0a010c,") {
				kept = append(kept, g)
			}
		}
		fc.Greeting = kept
	}))
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/devices/0a010c", ""); code != 200 {
		t.Fatalf("remove: %d %v", code, out)
	}
	sameLines(t, received, sent, []string{"m:"})
	if device, inRoom := listedRF(t, ts, "0a010c"); device || inRoom {
		t.Fatalf("still listed: device %v, in a room %v", device, inRoom)
	}
}

// A device the Cube keeps after t:, as one that never answers: the
// Cube's next device list does not bring it back, the next greeting does.
func TestRemovedDeviceTheCubeKeeps(t *testing.T) {
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)), withFake(func(fc *fakecube.Server) { fc.Keep = map[cube.RFAddress]bool{0x0a010a: true} }))
	ts, s := r.ts, r.s
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
	testkit.Eventually(t, "the device after the greeting", func() bool { return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/0a010a", nil) == 200 })
}

// A removal and an add at once: each reads the metadata the other wrote,
// so the added device is listed and the removed one is not.
func TestRemoveAndAddAtOnce(t *testing.T) {
	t.Parallel()
	r := startServer(t, withGreeting(copyGreeting(t)))
	ts, s := r.ts, r.s
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

// withRoomMaster sets a room's address in the greeting's metadata, the
// device the Cube's metadata names as the room's master.
func withRoomMaster(t *testing.T, greeting []cube.Message, room uint8, rf cube.RFAddress) {
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
		for j := range md.Rooms {
			if md.Rooms[j].ID == room {
				md.Rooms[j].RFAddress = rf
			}
		}
		enc, err := md.Encode()
		if err != nil {
			t.Fatal(err)
		}
		greeting[i] = cube.Message{Type: 'M', Data: "00,01," + base64.StdEncoding.EncodeToString(enc)}
		return
	}
	t.Fatal("no M line")
}

// A room command names a thermostat of the room, the first of its order
// (RoomMajority.D9, in place of D167; one function, D169-A): the
// metadata's master where it is one, else the room's wall thermostat,
// else its first radiator thermostat, never a device of another room,
// which would take the command too (docs/cube-protocol.md, "Set temperature (0x40)
// addressing"); a room without a thermostat is refused and nothing is
// sent. Room 1 holds 0a010a and the wall thermostat, room 6 0a010d.
func TestRoomCommandAddress(t *testing.T) {
	t.Parallel()
	auto := func(rf cube.RFAddress) string {
		m, err := cube.CmdSetTemperature(cube.Target{RFAddress: rf, RoomID: 1, Room: true}, 0, cube.ModeAuto, cube.Until{})
		if err != nil {
			t.Fatal(err)
		}
		return m.String()
	}
	for _, c := range []struct {
		name   string
		master cube.RFAddress
		moved  []cube.RFAddress // into room 6 by their C lines
		want   string           // the line sent, "" for a refusal
	}{
		{"no master", 0, nil, auto(copyWall)},
		{"master of another room", 0x0a010d, nil, auto(copyWall)},
		{"the radiator thermostat as master", 0x0a010a, nil, auto(0x0a010a)},
		{"only a radiator thermostat", 0, []cube.RFAddress{copyWall}, auto(0x0a010a)},
		{"no thermostat", 0x0a010a, []cube.RFAddress{0x0a010a, copyWall}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := startServer(t, withGreeting(copyGreeting(t)), withFake(func(fc *fakecube.Server) {
				withRoomMaster(t, fc.Greeting, 1, c.master)
				for _, rf := range c.moved {
					withConfigRoom(t, fc.Greeting, rf, 6)
				}
			}))
			fc, ts := r.fc, r.ts
			received := cubewatch.Lines(fc)
			sent := received.Mark()
			code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/1/setpoint", `{"mode":"auto"}`)
			got := lineStrings(received.Since(sent, cubewatch.Type('s')))
			if c.want == "" {
				if code != http.StatusConflict || len(got) != 0 {
					t.Fatalf("%d %v, sent %v; want 409 and nothing sent", code, out, got)
				}
				return
			}
			if code != 200 || len(got) != 1 || got[0] != c.want {
				t.Fatalf("%d %v, sent %v; want %s", code, out, got, c.want)
			}
		})
	}
}

// The copy takes the room's state from before a newcomer and the room's
// configuration by the one rule (AP RoomMajority, Block 2: D5-A, D8-A,
// D9, D15-A, D16-A).

// newcomer is the rooms fixture's wall thermostat, which startNewcomer
// takes out of its room.
const newcomer cube.RFAddress = 0x0a0201

// startNewcomer is a driftRig over the rooms fixture with room 1 holding
// its radiator thermostat 0a0202 alone, its master (0a0203 moved into no
// room), and the wall thermostat 0a0201 in no room: its C line names
// none, and the list has it in listed, out of the list for 0. change
// changes the greeting after that, fake the fake Cube. The structure
// changes are read back within a fraction of a second. The fake Cube
// does not take a group (0x22) itself; as the Cube does once the device
// acknowledged, it answers 0a0201's C line with the room its 0x22 named
// from then on.
func startNewcomer(t *testing.T, listed uint8, change func([]cube.Message) []cube.Message, fake func(*fakecube.Server)) *driftRig {
	t.Helper()
	clock := &checkClock{}
	opts := []startOpt{rooms(), stored(),
		withOptions(Options{PollInterval: 50 * time.Millisecond, driftClock: clock.now, driftEvery: -1,
			StructureChecks: []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}}),
		changeGreeting(func(lines []cube.Message) []cube.Message {
			lines = withDeviceIn(t, lines, 0x0a0203, 0)
			withConfigRoom(t, lines, newcomer, 0)
			withMetadata(t, lines, func(md *cube.Metadata) {
				md.Devices = slices.DeleteFunc(md.Devices, func(d cube.DeviceMeta) bool { return d.RFAddress == newcomer && listed == 0 })
				for i := range md.Devices {
					if md.Devices[i].RFAddress == newcomer {
						md.Devices[i].RoomID = listed
					}
				}
				for i := range md.Rooms {
					if md.Rooms[i].ID == 1 {
						md.Rooms[i].RFAddress = 0x0a0202
					}
				}
			})
			if change != nil {
				lines = change(lines)
			}
			return lines
		}),
		withFake(func(fc *fakecube.Server) {
			raw := rawConfig(t, fc.Greeting, newcomer)
			fc.OnReceive(func(m cube.Message) {
				rc, err := cube.ParseRadioCommand(m.Data)
				if m.Type == 's' && err == nil && rc.Command == cube.RFSetGroupAddress && rc.Target.RFAddress == newcomer && len(rc.Payload) > 0 {
					fc.SetConfig(configLine(raw, newcomer, rc.Payload[0], string(raw[8:18])))
				}
			})
		}),
	}
	if fake != nil {
		opts = append(opts, withFake(fake))
	}
	return &driftRig{rig: startServer(t, opts...), clock: clock}
}

// putNewcomer puts 0a0201 into room with PUT …/devices/0a0201 and
// returns the answer, failing the test where it is not 200.
func (r *driftRig) putNewcomer(room uint8) map[string]any {
	r.t.Helper()
	code, out := do(r.t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0201", fmt.Sprintf(`{"room_id":%d}`, room))
	if code != 200 {
		r.t.Fatalf("0a0201 into room %d: %d %v", room, code, out)
	}
	return out
}

// settled waits until no structure change is read back any more
// (structure.go), so that the room checks take the rooms again.
func (r *driftRig) settled() {
	r.t.Helper()
	testkit.Eventually(r.t, "the structure changes read back", func() bool { return len(r.rt.structureRooms(r.rt.client.Snapshot())) == 0 })
}

// heldIn is the state held for room (holdReference), false for none.
func (r *driftRig) heldIn(room uint8) (resend, bool) {
	d := &r.rt.drift
	d.mu.Lock()
	defer d.mu.Unlock()
	if h := d.holds[room]; h != nil {
		return h.state, true
	}
	return resend{}, false
}

// newcomerLink is 0a0201's link half into room 1 beside 0a0202, the
// metadata and the re-read, as the fake Cube receives them.
func newcomerLink() []string {
	return []string{"z:1e,G,01",
		linkPartner(room1(0x0a0202), true, newcomer, cube.DeviceWallThermostat),
		linkPartner(direct(newcomer), true, 0x0a0202, cube.DeviceRadiatorThermostat),
		cube.CmdSetGroup(newcomer, 1, false).String(), "m:", "c:0a0201"}
}

// setTemp is a 0x40 to t.
func setTemp(t *testing.T, to cube.Target, temp float64, mode cube.Mode, until cube.Until) string {
	t.Helper()
	m, err := cube.CmdSetTemperature(to, temp, mode, until)
	if err != nil {
		t.Fatal(err)
	}
	return m.String()
}

// copyOf is the copy's configuration for 0a0201 put into room after its
// 0x40, from ref, the room's configuration as RoomConfig decides it:
// after a wakeup 0x11 to every thermostat of the room, radiator
// thermostats first (radiators) and 0a0201 last, each with its own
// offset (0 in the fixture); 0x12 to 0a0201; every day of ref's
// programme, the wall thermostat's own differing every day, then 0x11
// again.
func copyOf(t *testing.T, room uint8, radiators []cube.RFAddress, ref cube.ThermostatConfig) []string {
	t.Helper()
	temps := func(rf cube.RFAddress) string {
		m, err := cube.CmdSetTemperatures(direct(rf), cube.TemperatureSettings{Comfort: ref.ComfortTemp, Eco: ref.EcoTemp, Max: ref.MaxSetpoint,
			Min: ref.MinSetpoint, WindowOpen: ref.WindowOpenTemp, WindowOpenMinutes: ref.WindowOpenMins})
		if err != nil {
			t.Fatal(err)
		}
		return m.String()
	}
	wakeup := fmt.Sprintf("z:1e,G,%02x", room)
	out := []string{wakeup}
	for _, rf := range append(slices.Clone(radiators), newcomer) {
		out = append(out, temps(rf))
	}
	valve, err := cube.CmdSetValveConfig(direct(newcomer), cube.ValveSettings{BoostMinutes: ref.BoostMinutes, BoostValvePct: ref.BoostValvePct,
		DecalcDay: ref.DecalcDay, DecalcHour: ref.DecalcHour, MaxValvePct: ref.MaxValvePct, ValveOffsetPct: ref.ValveOffsetPct})
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, wakeup, valve.String(), wakeup)
	for _, d := range []cube.Weekday{cube.Monday, cube.Tuesday, cube.Wednesday, cube.Thursday, cube.Friday, cube.Saturday, cube.Sunday} {
		ls, err := cube.CmdSetDayProgram(direct(newcomer), d, ref.WeekProgram[d])
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, lineStrings(ls)...)
	}
	return append(out, temps(newcomer))
}

// The owner's case of 2026-11-23: a room of one radiator thermostat in
// manual 5 °C, set at the device, and a wall thermostat put into it. The
// copy's 0x40 is manual 5 °C, to the wall thermostat directly (D8-A), not
// auto, after which that room went to its programme; the rest is the
// radiator thermostat's configuration. Where the radiator thermostat
// then shows auto, as the wall thermostat's own setpoint did reach it
// that day, the room checks send the room manual 5 °C, the state from
// before the change (before_change), not the newest change.
func TestCopyKeepsTheRoomsManualSetpoint(t *testing.T) {
	t.Parallel()
	r := startNewcomer(t, 0, nil, nil)
	r.set(0x0a0202, "manual", 5)
	sent := r.lines.Mark()
	out := r.putNewcomer(1)
	want := append(newcomerLink(), setTemp(t, direct(newcomer), 5, cube.ModeManual, cube.Until{}))
	sameLines(t, r.lines, sent, append(want, copyOf(t, 1, []cube.RFAddress{0x0a0202}, *r.rt.client.Snapshot().Config(0x0a0202).Thermostat)...))
	if copied, _ := out["copied"].(map[string]any); copied["days"] != float64(7) || copied["valve"] != true || copied["mode_not_set"] != nil {
		t.Fatalf("copied %v", out["copied"])
	}
	r.settled()
	r.waitSetting(newcomer, "manual", 5)
	r.set(0x0a0202, "auto", 21)
	sent = r.lines.Mark()
	r.disagreeAndResend()
	r.wantResendOf(sent, 0x0a0202, 1, 5, cube.ModeManual, "majority", true)
}

// stateList is the L line of snap's states with rf's mode and setpoint
// changed, as the Cube sends it once rf reported them.
func stateList(t *testing.T, snap *cubelan.Snapshot, rf cube.RFAddress, mode cube.Mode, setpoint float64) cube.Message {
	t.Helper()
	sp, err := cube.EncodeTemp(setpoint)
	if err != nil {
		t.Fatal(err)
	}
	var list []byte
	for _, dev := range snap.Devices() {
		st, ok := snap.State(dev)
		if !ok {
			continue
		}
		e := slices.Clone(st.Raw)
		if dev == rf {
			e[6] = e[6]&^0x03 | byte(mode)
			e[8] = e[8]&0xc0 | sp
		}
		list = append(list, e...)
	}
	return cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(list)}
}

// The room's state is read before the links (D8-A): the radiator
// thermostat shows auto on its programme from the moment the fake Cube
// takes the link radiogram that names the wall thermostat, which the
// server's snapshot shows before the radiogram is answered; the copy
// still gives the wall thermostat manual 5 °C, the state from before,
// and the room checks hold it.
func TestCopyReadsTheRoomBeforeTheLinks(t *testing.T) {
	t.Parallel()
	r := startNewcomer(t, 0, nil, nil)
	r.set(0x0a0202, "manual", 5)
	later := stateList(t, r.rt.client.Snapshot(), 0x0a0202, cube.ModeAuto, 21)
	var once sync.Once
	r.fc.OnReceive(func(m cube.Message) {
		rc, err := cube.ParseRadioCommand(m.Data)
		if m.Type != 's' || err != nil || rc.Command != cube.RFAddLinkPartner || !rc.Target.Room || len(rc.Payload) < 3 ||
			cube.RFAddressFromBytes(rc.Payload[:3]) != newcomer {
			return
		}
		once.Do(func() {
			mode, temp := "auto", 21.0
			if err := r.fc.SetState(0x0a0202, fakecube.State{Mode: &mode, Setpoint: &temp}); err != nil {
				t.Error(err)
			}
			r.rt.client.ApplyLocal(later)
		})
	})
	sent := r.lines.Mark()
	r.putNewcomer(1)
	if st, ok := r.rt.client.Snapshot().State(0x0a0202); !ok || st.Mode != cube.ModeAuto {
		t.Fatalf("0a0202 not in auto by the copy: %+v", st)
	}
	want := append(newcomerLink(), setTemp(t, direct(newcomer), 5, cube.ModeManual, cube.Until{}))
	sameLines(t, r.lines, sent, append(want, copyOf(t, 1, []cube.RFAddress{0x0a0202}, *r.rt.client.Snapshot().Config(0x0a0202).Thermostat)...))
	if held, ok := r.heldIn(1); !ok || held.mode != cube.ModeManual || held.temp != 5 || held.from != fromMajority {
		t.Fatalf("held for room 1: %+v %v", held, ok)
	}
}

// The copy's 0x40 is the room's state: auto on the programme as auto
// with temperature 0, auto with a temperature set by hand as such, a
// holiday with its end; in a room with no other thermostat (room 3, a
// window contact's alone) auto with 0, and nothing is held. The answer's
// copied names the state and where it came from, and in room 3 that no
// configuration was sent, its list of temperatures empty, not null.
func TestCopyGivesTheRoomsState(t *testing.T) {
	t.Parallel()
	holiday := cube.Until{Year: 2026, Month: 11, Day: 10, Minutes: 12 * 60}
	for _, c := range []struct {
		name  string
		room  uint8
		state func(r *driftRig)
		want  string
	}{
		{"auto on the programme", 1, func(r *driftRig) { r.set(0x0a0202, "auto", 21) }, setTemp(t, direct(newcomer), 0, cube.ModeAuto, cube.Until{})},
		{"auto with a temperature", 1, func(r *driftRig) { r.set(0x0a0202, "auto", 23) }, setTemp(t, direct(newcomer), 23, cube.ModeAuto, cube.Until{})},
		{"a holiday", 1, func(r *driftRig) {
			m, err := cube.CmdSetTemperature(direct(0x0a0202), 12, cube.ModeVacation, holiday)
			if err != nil {
				r.t.Fatal(err)
			}
			if res := r.rt.queue.Enqueue(engine.Command{Lines: []cube.Message{m}}).Wait(r.t.Context()); res.Err != nil {
				r.t.Fatal(res.Err)
			}
			testkit.Eventually(r.t, "0a0202 on holiday", func() bool {
				st, ok := r.rt.client.Snapshot().State(0x0a0202)
				return ok && st.Mode == cube.ModeVacation && st.Until != nil && *st.Until == holiday
			})
		}, setTemp(t, direct(newcomer), 12, cube.ModeVacation, holiday)},
		{"no other thermostat", 3, func(r *driftRig) { r.set(0x0a0202, "manual", 5) }, setTemp(t, direct(newcomer), 0, cube.ModeAuto, cube.Until{})},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := startNewcomer(t, 0, nil, nil)
			c.state(r)
			sent := r.lines.Mark()
			out := r.putNewcomer(c.room)
			got := lineStrings(r.lines.Since(sent, cubewatch.Except('l')))
			at := slices.Index(got, "c:0a0201")
			if at < 0 || at+1 >= len(got) || got[at+1] != c.want {
				t.Fatalf("the copy's 0x40, after the re-read, in %v; want %s", got, c.want)
			}
			if _, ok := r.heldIn(c.room); ok == (c.room == 3) {
				t.Fatalf("held for room %d: %v", c.room, ok)
			}
			copied, _ := out["copied"].(map[string]any)
			temps, list := copied["temperatures"].([]any)
			from, skipped := "room", ""
			if c.room == 3 {
				from, skipped = "default", "no_other_thermostat"
			}
			if !list || copied["state_from"] != from || (copied["config_skipped"] != nil || skipped != "") && copied["config_skipped"] != skipped ||
				c.room == 3 && len(temps) != 0 {
				t.Fatalf("copied %#v", copied)
			}
		})
	}
}

// A copy whose 0x40 the Cube discards sends the room's state once more
// as a room command, after the programme (D16-A), which counts as
// Massimo's own setpoint to the room and keeps the room's hold; the
// discard is not retried, the device's flag being clear.
func TestCopyAfterADiscardedModeSendsTheRoom(t *testing.T) {
	t.Parallel()
	r := startNewcomer(t, 0, nil, func(fc *fakecube.Server) {
		fc.DiscardIf = func(m cube.Message) bool {
			rc, err := cube.ParseRadioCommand(m.Data)
			return m.Type == 's' && err == nil && rc.Command == cube.RFSetTemperature && !rc.Target.Room && rc.Target.RFAddress == newcomer
		}
	})
	r.set(0x0a0202, "manual", 5)
	sent := r.lines.Mark()
	out := r.putNewcomer(1)
	want := append(newcomerLink(), setTemp(t, direct(newcomer), 5, cube.ModeManual, cube.Until{}))
	want = append(want, copyOf(t, 1, []cube.RFAddress{0x0a0202}, *r.rt.client.Snapshot().Config(0x0a0202).Thermostat)...)
	sameLines(t, r.lines, sent, append(want, setTemp(t, room1(0x0a0202), 5, cube.ModeManual, cube.Until{})))
	if copied, _ := out["copied"].(map[string]any); copied["mode_not_set"] != true || copied["room_mode"] != true {
		t.Fatalf("copied %v", out["copied"])
	}
	d := &r.rt.drift
	d.mu.Lock()
	in, intended := d.intent[1]
	d.mu.Unlock()
	if !intended || in.mode != cube.ModeManual || in.temp != 5 {
		t.Fatalf("Massimo's setpoint to room 1: %+v %v", in, intended)
	}
	if held, ok := r.heldIn(1); !ok || held.mode != cube.ModeManual || held.temp != 5 {
		t.Fatalf("held for room 1: %+v %v", held, ok)
	}
}

// The repair's placement holds the room's state too (D15-A), before its
// link half, and copies nothing: a wall thermostat the list has in room
// 1 and its C line in none is placed there, and once it shows auto, a
// newer change than the radiator thermostat's manual 5 °C, the room
// checks send manual 5 °C, the state from before (before_change).
func TestRepairPlaceHoldsTheRoomsState(t *testing.T) {
	t.Parallel()
	r := startNewcomer(t, 1, nil, nil)
	r.set(0x0a0202, "manual", 5)
	sent := r.lines.Mark()
	_, code, out := runRepairOf(t, r.ts, "/api/v1/cubes/KEQ0000100/devices/0a0201/repair/place", "")
	if code != 200 {
		t.Fatalf("the placement: %d %v", code, out)
	}
	sameLines(t, r.lines, sent, []string{"z:1e,G,01",
		linkPartner(room1(0x0a0202), true, newcomer, cube.DeviceWallThermostat),
		linkPartner(direct(newcomer), true, 0x0a0202, cube.DeviceRadiatorThermostat),
		cube.CmdSetGroup(newcomer, 1, false).String(), "c:0a0201"})
	if held, ok := r.heldIn(1); !ok || held.mode != cube.ModeManual || held.temp != 5 {
		t.Fatalf("held for room 1: %+v %v", held, ok)
	}
	r.settled()
	r.set(newcomer, "auto", 21)
	sent = r.lines.Mark()
	r.disagreeAndResend()
	r.wantResendOf(sent, 0x0a0202, 1, 5, cube.ModeManual, "majority", true)
}

// The copy's configuration is the room's per group and per day
// (D5-A, D9): in room 2, its master 0a0206 here, 0a0204 has another
// comfort temperature and Monday, 0a0205 another Tuesday, 0a0206 another
// Wednesday and other valve settings; the wall thermostat put into it
// takes what two of the three share in each, a week none of them has,
// and its own offset. The report after the copy finds the three other
// days, and nothing of the wall thermostat's, nor any temperatures, the
// copy's 0x11 having gone to every thermostat of the room.
func TestCopyTakesTheRoomsConfigurationPerGroup(t *testing.T) {
	t.Parallel()
	r := startNewcomer(t, 0, func(lines []cube.Message) []cube.Message {
		edit := func(rf cube.RFAddress, change func(raw []byte)) {
			raw := rawConfig(t, lines, rf)
			change(raw)
			for i, g := range lines {
				if g.Type == 'C' && strings.HasPrefix(g.Data, rf.String()+",") {
					lines[i] = configLine(raw, rf, raw[5], string(raw[8:18]))
				}
			}
		}
		// A radiator thermostat's programme starts at 0x1d, a day of it
		// 2 bytes per switch point; a first byte's 0x02 is half a degree
		// of the first switch point's temperature.
		const program, day = 0x1d, 2 * cube.DaySlots
		edit(0x0a0204, func(raw []byte) { raw[0x12] = 46; raw[program+day*int(cube.Monday)] ^= 0x02 }) // comfort 23
		edit(0x0a0205, func(raw []byte) { raw[program+day*int(cube.Tuesday)] ^= 0x02 })
		edit(0x0a0206, func(raw []byte) { raw[0x19] = 0x52; raw[program+day*int(cube.Wednesday)] ^= 0x02 }) // boost 10 min at 90 %
		withRoomMaster(t, lines, 2, 0x0a0206)
		return lines
	}, nil)
	snap := r.rt.client.Snapshot()
	base := *snap.Config(0x0a0202).Thermostat // the fixture's radiator thermostat, as none of room 2's is now
	sent := r.lines.Mark()
	r.putNewcomer(2)
	want := []string{"z:1e,G,02",
		linkPartner(cube.Target{RFAddress: 0x0a0206, RoomID: 2, Room: true}, true, newcomer, cube.DeviceWallThermostat),
		linkPartner(direct(newcomer), true, 0x0a0204, cube.DeviceRadiatorThermostat),
		linkPartner(direct(newcomer), true, 0x0a0205, cube.DeviceRadiatorThermostat),
		linkPartner(direct(newcomer), true, 0x0a0206, cube.DeviceRadiatorThermostat),
		cube.CmdSetGroup(newcomer, 2, false).String(), "m:", "c:0a0201",
		setTemp(t, direct(newcomer), 20, cube.ModeAuto, cube.Until{})} // the three in auto at 20 °C, off their programme's 21
	sameLines(t, r.lines, sent, append(want, copyOf(t, 2, []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206}, base)...))

	for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206, newcomer} {
		r.rt.writer().Reread(t.Context(), rf) // the C lines as the fake Cube took the copy
	}
	snap = r.rt.client.Snapshot()
	var found []string
	mismatches, _ := findMismatches(snap, snap.Metadata(), view.NewHidden(nil, snap.Metadata(), false), nil)
	for _, m := range mismatches {
		if m.Room == 2 && (m.Finding == view.MismatchProgramme || m.Finding == view.MismatchTemperatures) {
			found = append(found, m.Finding+" "+m.Device)
		}
	}
	if want := []string{"programme 0a0204", "programme 0a0205", "programme 0a0206"}; !slices.Equal(found, want) {
		t.Fatalf("room 2's members that differ after the copy: %v, want %v", found, want)
	}
}

// The device being added is left out of the room's state where its C
// line names the room already, as after a retry whose link half went
// through: 0a0201, in room 1 by its C line and not in the list, shows
// manual 18 °C, a newer change than the radiator thermostat's manual
// 5 °C, and the copy's 0x40 is the room's manual 5 °C, which is held.
func TestCopyLeavesTheNewcomerOutOfTheRoomsState(t *testing.T) {
	t.Parallel()
	r := startNewcomer(t, 0, func(lines []cube.Message) []cube.Message {
		withConfigRoom(t, lines, newcomer, 1)
		return lines
	}, nil)
	r.set(0x0a0202, "manual", 5)
	r.set(newcomer, "manual", 18)
	sent := r.lines.Mark()
	r.putNewcomer(1)
	got := lineStrings(r.lines.Since(sent, cubewatch.Except('l')))
	want := setTemp(t, direct(newcomer), 5, cube.ModeManual, cube.Until{})
	if at := slices.Index(got, "c:0a0201"); at < 0 || at+1 >= len(got) || got[at+1] != want {
		t.Fatalf("the copy's 0x40, after the re-read, in %v; want %s", got, want)
	}
	if held, ok := r.heldIn(1); !ok || held.mode != cube.ModeManual || held.temp != 5 {
		t.Fatalf("held for room 1: %+v %v", held, ok)
	}
}

// A move of a thermostat into a room takes the room's state as an add
// does (D8-A): 0a0204 of room 2 moved into room 1, whose radiator
// thermostat shows manual 5 °C, gets manual 5 °C, to it directly after
// the move's re-read, and the room's state is held.
func TestMoveGivesTheRoomsState(t *testing.T) {
	t.Parallel()
	r := startNewcomer(t, 0, nil, nil)
	r.set(0x0a0202, "manual", 5)
	sent := r.lines.Mark()
	code, out := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0204", `{"room_id":1}`)
	if code != 200 {
		t.Fatalf("the move: %d %v", code, out)
	}
	got := lineStrings(r.lines.Since(sent, cubewatch.Except('l')))
	want := setTemp(t, direct(0x0a0204), 5, cube.ModeManual, cube.Until{})
	if at := slices.Index(got, "c:0a0204"); at < 0 || at+1 >= len(got) || got[at+1] != want {
		t.Fatalf("the copy's 0x40, after the re-read, in %v; want %s", got, want)
	}
	if held, ok := r.heldIn(1); !ok || held.mode != cube.ModeManual || held.temp != 5 {
		t.Fatalf("held for room 1: %+v %v", held, ok)
	}
	if copied, _ := out["copied"].(map[string]any); fmt.Sprint(copied["state"]) != "map[mode:manual setpoint:5]" {
		t.Fatalf("copied %v", out["copied"])
	}
}

// The room's state is held only once a radiogram of the link half went
// through, which may have brought the newcomer's state to the room's
// members: an add whose link radiogram the Cube discards, its first,
// holds nothing and leaves a hold of an earlier change as it was, its
// state and its time; one whose 0x22, the last, is discarded holds the
// room's state, the 0x20s having gone through. So do a move and the
// repair's placement.
func TestHoldOnlyOnceTheLinkWentThrough(t *testing.T) {
	t.Parallel()
	discardOf := func(command byte) func(*fakecube.Server) {
		return func(fc *fakecube.Server) {
			fc.DiscardIf = func(m cube.Message) bool {
				rc, err := cube.ParseRadioCommand(m.Data)
				return m.Type == 's' && err == nil && rc.Command == command
			}
		}
	}
	t.Run("nothing went through", func(t *testing.T) {
		t.Parallel()
		r := startNewcomer(t, 0, nil, discardOf(cube.RFAddLinkPartner))
		r.set(0x0a0202, "manual", 5)
		if code, out := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0201", `{"room_id":1}`); code != http.StatusBadGateway {
			t.Fatalf("the add: %d %v, want 502", code, out)
		}
		if held, ok := r.heldIn(1); ok {
			t.Fatalf("held for room 1: %+v", held)
		}
		earlier := r.s.roomNow().Add(-time.Minute)
		r.rt.holdReference(1, resend{mode: cube.ModeManual, temp: 7, from: fromMajority}, earlier)
		d := &r.rt.drift
		d.mu.Lock()
		d.holds[1].judged = true
		d.mu.Unlock()
		if code, out := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0201", `{"room_id":1}`); code != http.StatusBadGateway {
			t.Fatalf("the add again: %d %v, want 502", code, out)
		}
		d.mu.Lock()
		h := d.holds[1]
		d.mu.Unlock()
		if h == nil || h.state.temp != 7 || !h.at.Equal(earlier) || !h.judged {
			t.Fatalf("the earlier hold after the failed add: %+v", h)
		}
	})
	t.Run("the link went through", func(t *testing.T) {
		t.Parallel()
		r := startNewcomer(t, 0, nil, discardOf(cube.RFSetGroupAddress))
		r.set(0x0a0202, "manual", 5)
		if code, out := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0201", `{"room_id":1}`); code != http.StatusBadGateway {
			t.Fatalf("the add: %d %v, want 502", code, out)
		}
		if held, ok := r.heldIn(1); !ok || held.mode != cube.ModeManual || held.temp != 5 {
			t.Fatalf("held for room 1: %+v %v", held, ok)
		}
	})
	t.Run("a move whose link radiogram is discarded", func(t *testing.T) {
		t.Parallel()
		r := startNewcomer(t, 0, nil, discardOf(cube.RFAddLinkPartner))
		r.set(0x0a0202, "manual", 5)
		if code, out := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0204", `{"room_id":1}`); code != http.StatusBadGateway {
			t.Fatalf("the move: %d %v, want 502", code, out)
		}
		if held, ok := r.heldIn(1); ok {
			t.Fatalf("held for room 1: %+v", held)
		}
	})
	t.Run("a placement whose link radiogram is discarded", func(t *testing.T) {
		t.Parallel()
		r := startNewcomer(t, 1, nil, discardOf(cube.RFAddLinkPartner))
		r.set(0x0a0202, "manual", 5)
		if _, code, out := runRepairOf(t, r.ts, "/api/v1/cubes/KEQ0000100/devices/0a0201/repair/place", ""); code != http.StatusBadGateway {
			t.Fatalf("the placement: %d %v, want 502", code, out)
		}
		if held, ok := r.heldIn(1); ok {
			t.Fatalf("held for room 1: %+v", held)
		}
	})
}

// A room setpoint of Massimo's taken while an add's link half runs is the
// newer wish (RoomMajority.D8-A, D4-A first): the copy gives the device
// that setpoint, manual 21 °C, not the radiator thermostat's manual 5 °C
// read before the links, both in its direct 0x40, which the Cube
// discards here, and in the room command that follows (D16-A), which
// Massimo's intent then records; the state read before is not held. The
// setpoint is taken while the first link radiogram waits for its
// answer, and runs after the link half, before the metadata.
func TestCopyTakesANewerSetpoint(t *testing.T) {
	t.Parallel()
	r := startNewcomer(t, 0, nil, func(fc *fakecube.Server) {
		fc.DiscardIf = func(m cube.Message) bool {
			rc, err := cube.ParseRadioCommand(m.Data)
			return m.Type == 's' && err == nil && rc.Command == cube.RFSetTemperature && !rc.Target.Room && rc.Target.RFAddress == newcomer
		}
	})
	r.set(0x0a0202, "manual", 5)
	var once sync.Once
	setpoint := make(chan int, 1)
	r.fc.OnReceive(func(m cube.Message) {
		rc, err := cube.ParseRadioCommand(m.Data)
		switch {
		case m.Type == 's' && err == nil && rc.Command == cube.RFAddLinkPartner:
			once.Do(func() {
				go func() {
					code, _ := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/1/setpoint", `{"mode":"manual","temp":21}`)
					setpoint <- code
				}()
				// the setpoint in the queue before the link half ends
				if !testkit.Within(testkit.Wait, func() bool {
					return r.rt.queue.Pending(func(c engine.Command) bool { return c.Key == "setpoint:room:1" })
				}) {
					t.Error("the room setpoint never queued")
				}
			})
		case m.Type == 'm':
			// the setpoint's intent recorded before the metadata is answered
			if !testkit.Within(testkit.Wait, func() bool { _, ok := r.rt.intentAfter(1, time.Time{}); return ok }) {
				t.Error("the room setpoint's intent never recorded")
			}
		}
	})
	sent := r.lines.Mark()
	out := r.putNewcomer(1)
	if code := <-setpoint; code != 200 {
		t.Fatalf("the room setpoint: %d", code)
	}
	link := newcomerLink()
	want := append(append([]string(nil), link[:4]...), setTemp(t, room1(0x0a0202), 21, cube.ModeManual, cube.Until{}))
	want = append(want, link[4:]...)
	want = append(want, setTemp(t, direct(newcomer), 21, cube.ModeManual, cube.Until{}))
	want = append(want, copyOf(t, 1, []cube.RFAddress{0x0a0202}, *r.rt.client.Snapshot().Config(0x0a0202).Thermostat)...)
	sameLines(t, r.lines, sent, append(want, setTemp(t, room1(0x0a0202), 21, cube.ModeManual, cube.Until{})))
	if copied, _ := out["copied"].(map[string]any); copied["mode_not_set"] != true || copied["room_mode"] != true ||
		fmt.Sprint(copied["state"]) != "map[mode:manual setpoint:21]" {
		t.Fatalf("copied %v", out["copied"])
	}
	if in, ok := r.rt.intentAfter(1, time.Time{}); !ok || in.mode != cube.ModeManual || in.temp != 21 {
		t.Fatalf("Massimo's setpoint to room 1: %+v %v", in, ok)
	}
	if held, ok := r.heldIn(1); ok {
		t.Fatalf("held for room 1 over the newer setpoint: %+v", held)
	}
}

// A link command whose wait ends with the request's context while the
// queue still sends it has an unknown fate (cubeio.Reached): the Cube
// answers its 0x22 a while after it came.
func TestReachedAfterTheContextEnds(t *testing.T) {
	t.Parallel()
	r := startNewcomer(t, 0, nil, func(fc *fakecube.Server) {
		fc.DiscardIf = func(m cube.Message) bool {
			if rc, err := cube.ParseRadioCommand(m.Data); m.Type == 's' && err == nil && rc.Command == cube.RFSetGroupAddress {
				time.Sleep(300 * time.Millisecond)
			}
			return false
		}
	})
	cmd := engine.Command{Key: "link-late", Lines: []cube.Message{cube.CmdSetGroup(newcomer, 1, false)}}
	from := r.lines.Mark()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	res := r.rt.writer().Do(ctx, cmd)
	if !errors.Is(res.Err, context.DeadlineExceeded) || !cubeio.Reached(cmd, res) {
		t.Fatalf("the result %+v, reached %v", res, cubeio.Reached(cmd, res))
	}
	r.lines.Await(t, from, "the 0x22 sent", func(m cube.Message) bool { return m.String() == cmd.Lines[0].String() })
}
