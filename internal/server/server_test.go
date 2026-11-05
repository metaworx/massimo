// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

const fixture = "../protocol/cube/testdata/cube-fw0113-greeting.txt"

func countType(fc *fakecube.Server, typ byte) int {
	n := 0
	for _, m := range fc.Received() {
		if m.Type == typ {
			n++
		}
	}
	return n
}

func start(t *testing.T, token string) (*fakecube.Server, *httptest.Server, *Server) {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0537249", Addr: addr}}, Token: token, Queue: engine.Options{Gap: time.Millisecond}})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	// Wait for the greeting.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var cubes []cubeView
		getJSON(t, ts, token, "/api/v1/cubes", &cubes)
		if len(cubes) == 1 && cubes[0].Connected {
			return fc, ts, s
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("cube never connected")
	return nil, nil, nil
}

func getJSON(t *testing.T, ts *httptest.Server, token, path string, v any) int {
	t.Helper()
	req, _ := http.NewRequest("GET", ts.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if v != nil && resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

func TestServerReadsAndSetpoints(t *testing.T) {
	fc, ts, s := start(t, "")
	var c cubeView
	if getJSON(t, ts, "", "/api/v1/cubes/keq0537249", &c) != 200 || c.Rooms != 6 || c.Devices != 6 || c.Firmware != "1.1.3" || len(c.RoomList) != 6 {
		t.Fatalf("cube %+v", c)
	}
	var devs []view.Device
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/devices", &devs)
	if len(devs) != 6 || devs[0].Name == "" {
		t.Fatalf("devices %+v", devs)
	}
	var rooms []view.Room
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/rooms", &rooms)
	if len(rooms) != 6 {
		t.Fatalf("rooms %+v", rooms)
	}
	for _, r := range rooms {
		for _, d := range r.Devices {
			if d.Reported != (d.State != nil) || (!d.Reported && r.Ready) {
				t.Fatalf("room %d ready=%v device %+v", r.ID, r.Ready, d)
			}
		}
	}
	var d deviceDetail
	if getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/devices/1514b6", &d) != 200 || d.Thermostat == nil || d.Thermostat.ComfortTemp != 21.5 || d.RoomID != 6 {
		t.Fatalf("device %+v", d)
	}
	if getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/devices/abcdef", nil) != 404 {
		t.Fatal("unknown device not 404")
	}
	// Setpoint on a device, then on a room.
	for _, c := range []struct{ path, body, want string }{
		{"/api/v1/cubes/KEQ0537249/devices/1514b6/setpoint", `{"mode":"manual","temp":21.5}`, "s:AARAAAAAFRS2Bms="},
		{"/api/v1/cubes/KEQ0537249/rooms/6/setpoint", `{"mode":"auto"}`, "s:AARAAAAAFRS2BgA="},
	} {
		req, _ := http.NewRequest("PUT", ts.URL+c.path, strings.NewReader(c.body))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var res map[string]any
		json.NewDecoder(resp.Body).Decode(&res)
		resp.Body.Close()
		if resp.StatusCode != 200 || res["ok"] != true {
			t.Fatalf("%s: %d %v", c.path, resp.StatusCode, res)
		}
		var got []string
		for _, m := range fc.Received() {
			if m.Type == 's' {
				got = append(got, m.String())
			}
		}
		if got[len(got)-1] != c.want {
			t.Fatalf("%s sent %v, want last %s", c.path, got, c.want)
		}
	}
	// Key lock: wakeup of the room, then radiogram 0x80 to the device.
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/cubes/KEQ0537249/devices/1514b6/lock", strings.NewReader(`{"locked":false}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("lock: %d", resp.StatusCode)
	}
	var lines []string
	for _, m := range fc.Received() {
		lines = append(lines, m.String())
	}
	if n := len(lines); n < 2 || lines[n-1] != "s:AACAAAAAFRS2AAA=" || !strings.HasPrefix(lines[n-2], "z:1e,G,06") {
		t.Fatalf("lock sent %v", lines)
	}
	// Deleting rooms: occupied ones are refused, unknown ones are 404,
	// an empty one goes to the cube as a metadata write.
	for _, c := range []struct {
		path string
		want int
	}{{"/api/v1/cubes/KEQ0537249/rooms/6", 409}, {"/api/v1/cubes/KEQ0537249/rooms/42", 404}} {
		req, _ = http.NewRequest("DELETE", ts.URL+c.path, nil)
		resp, _ = http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Fatalf("DELETE %s: %d, want %d", c.path, resp.StatusCode, c.want)
		}
	}
	rt := s.cubes["keq0537249"]
	withRoom := *rt.client.Snapshot().Metadata()
	withRoom.Rooms = append(append([]cube.RoomMeta(nil), withRoom.Rooms...), cube.RoomMeta{ID: 7, Name: "Empty"})
	cmds, _ := cube.WriteMetadataCommands(&withRoom)
	_, data, _ := strings.Cut(cmds[0].Data, ",")
	rt.client.ApplyLocal(cube.Message{Type: 'M', Data: "00,01," + data})
	before := countType(fc, 'm')
	req, _ = http.NewRequest("DELETE", ts.URL+"/api/v1/cubes/KEQ0537249/rooms/7", nil)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 200 || countType(fc, 'm') != before+1 || len(rt.client.Snapshot().Metadata().Rooms) != 6 {
		t.Fatalf("DELETE empty room: %d, m lines %d→%d, rooms %d", resp.StatusCode, before, countType(fc, 'm'), len(rt.client.Snapshot().Metadata().Rooms))
	}
	// Bad requests.
	req, _ = http.NewRequest("PUT", ts.URL+"/api/v1/cubes/KEQ0537249/devices/1514b6/setpoint", strings.NewReader(`{"mode":"boost"}`))
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("boost accepted: %d", resp.StatusCode)
	}
	// Backup download.
	resp, _ = http.Get(ts.URL + "/api/v1/cubes/KEQ0537249/backup?format=yaml")
	body := make([]byte, 200)
	n, _ := resp.Body.Read(body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body[:n]), "serial: KEQ0537249") {
		t.Fatalf("backup %d %q", resp.StatusCode, body[:n])
	}
}

func TestServerToken(t *testing.T) {
	_, ts, _ := start(t, "secret")
	if getJSON(t, ts, "", "/api/v1/cubes", nil) != 401 {
		t.Fatal("no token accepted")
	}
	if getJSON(t, ts, "wrong", "/api/v1/cubes", nil) != 401 {
		t.Fatal("wrong token accepted")
	}
	if getJSON(t, ts, "", "/healthz", nil) != 200 {
		t.Fatal("healthz needs a token")
	}
}

func TestPeriodicRefreshReconnects(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0537249", Addr: addr}}, RefreshInterval: 50 * time.Millisecond, Queue: engine.Options{Gap: time.Millisecond}})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	t.Cleanup(func() { cancel(); fc.Close() })
	deadline := time.Now().Add(5 * time.Second)
	for fc.Connections() < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if fc.Connections() < 3 {
		t.Fatalf("connections %d", fc.Connections())
	}
}
