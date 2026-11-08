// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import "testing"

// configView, GET /api/v1/config's settings in effect, marks a key, an
// IV and the token as set and leaves their values out, and shows the
// others with where they came from.
func TestConfigViewHidesSecrets(t *testing.T) {
	t.Setenv("MASSIMO_API_TOKEN", "token-value-1")
	t.Setenv("MASSIMO_CUBE_KEQ0000100_KEY", "00112233445566778899aabbccddeeff")
	t.Setenv("MASSIMO_CUBE_KEQ0000100_IV", "ffeeddccbbaa99887766554433221100")
	t.Setenv("MASSIMO_LISTEN", ":8099")
	v := configView(":8099", "off")
	if v.Listen != ":8099" || v.TLS != "off" || v.File == "" {
		t.Fatalf("view %+v", v)
	}
	seen := map[string]bool{}
	for _, cv := range v.Variables {
		seen[cv.Name] = true
		switch cv.Name {
		case "MASSIMO_API_TOKEN", "MASSIMO_CUBE_KEQ0000100_KEY", "MASSIMO_CUBE_KEQ0000100_IV":
			if !cv.Secret || cv.Value != "" {
				t.Errorf("%s shown: %+v", cv.Name, cv)
			}
		case "MASSIMO_LISTEN":
			if cv.Secret || cv.Value != ":8099" || cv.Source == "" {
				t.Errorf("MASSIMO_LISTEN: %+v", cv)
			}
		}
	}
	for _, k := range []string{"MASSIMO_API_TOKEN", "MASSIMO_CUBE_KEQ0000100_KEY", "MASSIMO_LISTEN"} {
		if !seen[k] {
			t.Errorf("%s not listed", k)
		}
	}
}
