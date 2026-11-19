// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/testkit"
)

// A Cube whose connection is held is listed with link held, since the
// loss and until the hold's end, and connected again once it resumes.
func TestAHeldCubeIsListedAsHeld(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(Options{PollInterval: 100 * time.Millisecond, CubeHold: time.Minute,
		cubeOptions: func(o *cubelan.Options) { o.ReplyTimeout = 50 * time.Millisecond }}))
	fc, ts := r.fc, r.ts
	link := func() map[string]any {
		var cubes []map[string]any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		if len(cubes) == 0 {
			return nil
		}
		l, _ := cubes[0]["link"].(map[string]any)
		return l
	}
	testkit.Eventually(t, "connected", func() bool { l := link(); return l != nil && l["state"] == "connected" })
	fc.SetSilent(true)
	testkit.Eventually(t, "held", func() bool { l := link(); return l != nil && l["state"] == "held" })
	if l := link(); l["since"] == nil || l["until"] == nil {
		t.Errorf("held link %v: want since and until", l)
	}
	fc.SetSilent(false)
	testkit.Eventually(t, "connected again", func() bool { l := link(); return l != nil && l["state"] == "connected" })
	if fc.Connections() != 1 {
		t.Errorf("%d connections, want the held one only", fc.Connections())
	}
}
