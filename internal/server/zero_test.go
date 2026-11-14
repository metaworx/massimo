// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/base64"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// withZeroEntry adds to a greeting what one of the owner's Cubes sent on
// firmware 1.1.3 (docs/cube-protocol.md): a C line at radio address
// 000000 too short for a Cube's configuration (its other bytes zero
// here) and an L entry at 000000 without flags.
func withZeroEntry(t *testing.T, g []cube.Message) []cube.Message {
	t.Helper()
	c := make([]byte, 207)
	c[0] = 0xce
	entry := []byte{0x0b, 0, 0, 0, 0x09, 0, 0, 0, 0, 0, 0, 0}
	var out []cube.Message
	for _, m := range g {
		if m.Type == 'L' {
			raw, err := base64.StdEncoding.DecodeString(m.Data)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, cube.Message{Type: 'C', Data: "000000," + base64.StdEncoding.EncodeToString(c)})
			m = cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(append(raw, entry...))}
		}
		out = append(out, m)
	}
	return out
}

// A Cube's entry at 000000 is no device: no list of the API names it,
// every device route answers 400 for it, and nothing reaches the Cube.
func TestZeroEntryIsNoDevice(t *testing.T) {
	fc, ts, _ := startCopyCube(t, func(fc *fakecube.Server) { fc.Greeting = withZeroEntry(t, fc.Greeting) })
	const base = "/api/v1/cubes/KEQ0000100"
	for _, path := range []string{base + "/devices", base + "/rooms"} {
		code, body := callAPI(t, ts, "GET", path, "", "")
		if code != 200 || strings.Contains(body, `"000000"`) {
			t.Fatalf("GET %s: %d, names 000000: %s", path, code, body)
		}
	}
	sent := len(fc.Received())
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/devices/000000", ""},
		{"GET", "/devices/000000/program", ""},
		{"PUT", "/devices/000000/setpoint", `{"mode":"manual","temp":21.5}`},
		{"PUT", "/devices/000000/lock", `{"locked":true}`},
		{"PUT", "/devices/000000", `{"room_id":1}`},
		{"PUT", "/devices/000000/config", `{}`},
		{"PUT", "/devices/000000/program", `{}`},
		{"DELETE", "/devices/000000", ""},
	} {
		code, body := callAPI(t, ts, c.method, base+c.path, "", c.body)
		if code != 400 || !strings.Contains(body, "000000 is no device") {
			t.Errorf("%s %s: %d %s, want 400", c.method, c.path, code, body)
		}
	}
	if got := fc.Received()[sent:]; len(got) != 0 {
		t.Fatalf("sent to the Cube: %v", lineStrings(got))
	}
}
