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
)

const fixture = "../protocol/cube/testdata/cube-fw0113-greeting.txt"

func start(t *testing.T, token string) (*fakecube.Server, *httptest.Server) {
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
			return fc, ts
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("cube never connected")
	return nil, nil
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
	fc, ts := start(t, "")
	var c cubeView
	if getJSON(t, ts, "", "/api/v1/cubes/keq0537249", &c) != 200 || c.Rooms != 6 || c.Devices != 6 || c.Firmware != "1.1.3" || len(c.RoomList) != 6 {
		t.Fatalf("cube %+v", c)
	}
	var devs []deviceState
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/devices", &devs)
	if len(devs) != 6 || devs[0].Name == "" {
		t.Fatalf("devices %+v", devs)
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
	// Bad requests.
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/cubes/KEQ0537249/devices/1514b6/setpoint", strings.NewReader(`{"mode":"boost"}`))
	resp, _ := http.DefaultClient.Do(req)
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
	_, ts := start(t, "secret")
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
