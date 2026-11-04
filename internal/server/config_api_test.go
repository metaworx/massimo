// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
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
	rt := s.cubes["keq0537249"]
	snap := rt.client.Snapshot()
	const rf = "1514b6" // radiator thermostat in room 6 of the fixture
	dev := cube.RFAddress(0x1514b6)

	// Rename a room and a device: metadata writes, mirrored into the snapshot.
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/rooms/6", `{"name":"Renamed"}`); code != 200 {
		t.Fatalf("rename room: %d", code)
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/devices/"+rf, `{"name":"New name"}`); code != 200 {
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
		if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249"+c.path, c.body); code != 400 && code != 404 {
			t.Fatalf("PUT %s %s: %d", c.path, c.body, code)
		}
	}

	// Device configuration: wakeup of the room, one 0x11 with the merged
	// values, then a c: refresh.
	before := len(fc.Received())
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/devices/"+rf+"/config", `{"eco_temp":16.5}`); code != 200 || out["ok"] != true {
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
		if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/devices/"+rf+"/config", body); code != 400 {
			t.Fatalf("config %s: %d, want 400", body, code)
		}
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/rooms/6/config", `{"temp_offset":1}`); code != 400 {
		t.Fatalf("room config with offset: %d", code)
	}

	// Room configuration: every thermostat of the room, per-device results.
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/rooms/6/config", `{"boost_minutes":10}`); code != 200 || out["ok"] != true || len(out["devices"].([]any)) != 1 {
		t.Fatalf("room config: %d %v", code, out)
	}
	// GET room config and programme.
	if code, out := do(t, ts, "GET", "/api/v1/cubes/KEQ0537249/rooms/6/config", ""); code != 200 || out["source"] != rf || len(out["devices"].([]any)) != 1 {
		t.Fatalf("room config view: %d %v", code, out)
	}
	if code, out := do(t, ts, "GET", "/api/v1/cubes/KEQ0537249/rooms/6/program", ""); code != 200 || out["source"] != rf || out["monday"] == nil {
		t.Fatalf("room programme view: %d %v", code, out)
	}

	// Room week programme: a group radiogram per day (flags 04, room id).
	nS := len(sLines(fc))
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/rooms/6/program", `{"monday":[{"temp":17,"until":"07:00"},{"temp":21,"until":"24:00"}]}`); code != 200 || out["ok"] != true {
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
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/rooms/6/program", `{"monday":[{"temp":17,"until":"07:00"}]}`); code != 400 {
		t.Fatalf("programme not ending at 24:00 accepted: %d", code)
	}
	// Device week programme: direct, two days.
	nS = len(sLines(fc))
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/devices/"+rf+"/program", `{"saturday":[{"temp":18,"until":"24:00"}],"sunday":[{"temp":18,"until":"24:00"}]}`); code != 200 {
		t.Fatalf("device programme: %d", code)
	}
	lines = sLines(fc)
	if len(lines) != nS+2 || lines[nS][1] != 0 || lines[nS][9] != 0 || lines[nS][10] != byte(cube.Saturday) {
		t.Fatalf("device programme lines % x", lines[nS:])
	}

	// Move the device to room 1: leave (group remove), join (links with the
	// room's radiator both ways, group set), metadata.
	nS = len(sLines(fc))
	nM := countType(fc, 'm')
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/devices/"+rf, `{"room_id":1}`); code != 200 {
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
