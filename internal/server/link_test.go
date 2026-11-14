// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// A Cube whose connection is held is listed with link held, since the
// loss and until the hold's end, and connected again once it resumes.
func TestAHeldCubeIsListedAsHeld(t *testing.T) {
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
	ts, _ := startDiscovering(t, Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, PollInterval: 100 * time.Millisecond,
		CubeHold: time.Minute, cubeOptions: func(o *cubelan.Options) { o.ReplyTimeout = 50 * time.Millisecond }})
	link := func() map[string]any {
		var cubes []map[string]any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		if len(cubes) == 0 {
			return nil
		}
		l, _ := cubes[0]["link"].(map[string]any)
		return l
	}
	eventually(t, "connected", func() bool { l := link(); return l != nil && l["state"] == "connected" })
	fc.SetSilent(true)
	eventually(t, "held", func() bool { l := link(); return l != nil && l["state"] == "held" })
	if l := link(); l["since"] == nil || l["until"] == nil {
		t.Errorf("held link %v: want since and until", l)
	}
	fc.SetSilent(false)
	eventually(t, "connected again", func() bool { l := link(); return l != nil && l["state"] == "connected" })
	if fc.Connections() != 1 {
		t.Errorf("%d connections, want the held one only", fc.Connections())
	}
}
