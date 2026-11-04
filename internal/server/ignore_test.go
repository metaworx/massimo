// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// In the fixture 1514b6 is the only device of room 6.
func TestIgnoredDevicesInvisible(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0537249", Addr: addr, Ignore: []cube.RFAddress{0x1514b6}}}, Queue: engine.Options{Gap: time.Millisecond}})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	var c cubeView
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249", &c) == 200 && c.Connected {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !c.Connected || c.Rooms != 5 || c.Devices != 5 || len(c.RoomList) != 5 {
		t.Fatalf("cube %+v", c)
	}
	var devs []deviceState
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/devices", &devs)
	for _, d := range devs {
		if d.RFAddress == "1514b6" {
			t.Fatal("ignored device listed")
		}
	}
	if len(devs) != 5 {
		t.Fatalf("devices %d", len(devs))
	}
	var rooms []roomView
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/rooms", &rooms)
	for _, r := range rooms {
		if r.ID == 6 {
			t.Fatal("hidden room listed")
		}
	}
	if getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/devices/1514b6", nil) != 404 {
		t.Fatal("ignored device not 404")
	}
	for _, c := range []struct{ path, body string }{
		{"/api/v1/cubes/KEQ0537249/devices/1514b6/setpoint", `{"mode":"auto"}`},
		{"/api/v1/cubes/KEQ0537249/devices/1514b6/lock", `{"locked":false}`},
		{"/api/v1/cubes/KEQ0537249/rooms/6/setpoint", `{"mode":"auto"}`},
	} {
		req, _ := http.NewRequest("PUT", ts.URL+c.path, strings.NewReader(c.body))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Fatalf("%s: %d, want 404", c.path, resp.StatusCode)
		}
	}
	if len(fc.Received()) != 0 {
		t.Fatalf("commands reached the cube: %v", fc.Received())
	}
	resp, _ := http.Get(ts.URL + "/api/v1/cubes/KEQ0537249/backup?format=json")
	body := make([]byte, 64<<10)
	n, _ := resp.Body.Read(body)
	resp.Body.Close()
	if resp.StatusCode != 200 || strings.Contains(string(body[:n]), `"rf_address": "1514b6"`) {
		t.Fatalf("backup %d lists the ignored device", resp.StatusCode)
	}
}
