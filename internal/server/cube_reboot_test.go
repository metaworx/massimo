// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// A Cube's reboot: refused while a radiogram is pending and the request
// does not accept it, then R to the Cube's discovery port, its answer,
// the restart and the connection after it.
func TestRebootACube(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer fc.Close()
	at, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000100", 0x0a0107)
	if err != nil {
		t.Fatal(err)
	}
	defer at.Close()
	at.SetOnRestart(fc.Restart)
	ts, _ := startDiscovering(t, Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}},
		DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{at.Addr()}})
	link := func() string {
		var cubes []map[string]any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		if len(cubes) == 0 {
			return ""
		}
		l, _ := cubes[0]["link"].(map[string]any)
		s, _ := l["state"].(string)
		return s
	}
	eventually(t, "connected", func() bool { return link() == "connected" })

	// The fake's S reply leaves one radiogram pending (49 free slots).
	if code, body := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a010d/setpoint", `{"mode":"manual","temp":21.5}`); code != 200 {
		t.Fatalf("setpoint: %d %v", code, body)
	}
	if code, body := do(t, ts, "POST", "/api/v1/cubes/KEQ0000100/reboot", `{}`); code != 409 || !strings.Contains(body["error"].(string), "1 radiograms pending") {
		t.Fatalf("reboot with a radiogram pending: %d %v", code, body)
	}
	if at.Reboots() != 0 {
		t.Fatal("R sent though refused")
	}
	code, body := do(t, ts, "POST", "/api/v1/cubes/KEQ0000100/reboot", `{"pending":1}`)
	if code != 202 || body["sent"] != true || body["answered"] != true || body["status"] != float64(2) {
		t.Fatalf("reboot: %d %v", code, body)
	}
	if at.Reboots() != 1 {
		t.Errorf("%d R received", at.Reboots())
	}
	eventually(t, "the restart", func() bool { return fc.Restarts() == 1 })
	eventually(t, "connected after the reboot", func() bool { return link() == "connected" && fc.Connections() >= 2 })
	if code, _ := do(t, ts, "POST", "/api/v1/cubes/KEQ0000999/reboot", `{}`); code != 404 {
		t.Errorf("another Cube: %d", code)
	}
}

// While the reboot is waited for, the link says rebooting.
func TestARebootingCubeIsListedAsRebooting(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer fc.Close()
	at, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000100", 0x0a0107)
	if err != nil {
		t.Fatal(err)
	}
	defer at.Close()
	ts, s := startDiscovering(t, Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}},
		DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{at.Addr()}})
	eventually(t, "connected", func() bool { return s.runtimes()[0].client.Connected() })
	fc.SetSilent(true) // the Cube restarts: no greeting for a while
	defer fc.SetSilent(false)
	if code, body := do(t, ts, "POST", "/api/v1/cubes/KEQ0000100/reboot", `{}`); code != 202 {
		t.Fatalf("reboot: %d %v", code, body)
	}
	var cubes []map[string]any
	getJSON(t, ts, "", "/api/v1/cubes", &cubes)
	l, _ := cubes[0]["link"].(map[string]any)
	if l["state"] != "rebooting" || l["until"] == nil {
		t.Errorf("link %v, want rebooting", l)
	}
}
