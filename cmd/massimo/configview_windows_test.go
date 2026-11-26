// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package main

import (
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
)

// On Windows, which takes a name in any case, massimo config and GET
// /api/v1/config list massimo_listen as the start reads it: known, with
// its value, named as it is spelled; massimo config names
// massimo_cube_key, a Cube key of the portal commands, as ignored (AP
// StartConfiguration, D53-A).
func TestConfigKnowsANameInAnyCase(t *testing.T) {
	unsetForTest(t, "MASSIMO_LISTEN", env.Key)
	t.Setenv("massimo_listen", ":8099")
	t.Setenv("massimo_cube_key", "placeholder-key-1")
	var out strings.Builder
	printVariables(&out)
	listed, removed := false, false
	for _, line := range strings.Split(out.String(), "\n") {
		switch f := strings.Fields(line); {
		case len(f) > 1 && f[0] == "massimo_listen":
			listed = f[1] == ":8099" && !strings.Contains(line, "unknown")
		case len(f) > 1 && f[0] == "massimo_cube_key":
			removed = strings.Contains(line, "<- ignored: the portal commands were removed")
		}
	}
	if !listed || !removed {
		t.Errorf("massimo config's variables:\n%s", out.String())
	}
	for _, cv := range configView(":8099", "off").Variables {
		if cv.Name == "massimo_listen" {
			if cv.Unknown || cv.Value != ":8099" {
				t.Errorf("GET /api/v1/config's massimo_listen: %+v", cv)
			}
			return
		}
	}
	t.Errorf("GET /api/v1/config does not list massimo_listen")
}
