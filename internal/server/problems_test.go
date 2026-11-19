// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/base64"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// A Cube's devices with a problem are in its entry of the Cubes' list,
// each with its room's name: a transmit error and a low battery; a
// device without one is not, nor a hidden one.
func TestCubeProblems(t *testing.T) {
	t.Parallel()
	r := startServer(t, stored(), withOptions(Options{PollInterval: 100 * time.Millisecond}))
	fc, ts := r.fc, r.ts
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"hidden":["0a0108"]}`); code != 200 {
		t.Fatalf("hide: %d %v", code, out)
	}
	greeting := recorded(t)
	raw, _ := base64.StdEncoding.DecodeString(greeting[len(greeting)-1].Data)
	// 0a010d: valid, a transmit error, manual, its battery low;
	// 0a0108: valid, a link error, but hidden.
	copy(raw, []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12 | 0x08, 0x01 | 0x80, 30, 42, 0, 0, 0})
	copy(raw[12:], []byte{0x0b, 0x0a, 0x01, 0x08, 0x09, 0x12, 0x40, 30, 42, 0, 0, 0})
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	var c cubeView
	testkit.Eventually(t, "the problems", func() bool {
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", &c) == 200 && len(c.Problems) > 0
	})
	if len(c.Problems) != 1 {
		t.Fatalf("problems %+v, want 0a010d alone", c.Problems)
	}
	p := c.Problems[0]
	if p.RFAddress != "0a010d" || p.State == nil || !p.State.Error || !p.State.BatteryLow || p.Room == "" || p.Serial == "" {
		t.Fatalf("problem %+v", p)
	}
	var all []cubeView
	getJSON(t, ts, "", "/api/v1/cubes", &all)
	if len(all) != 1 || len(all[0].Problems) != 1 {
		t.Fatalf("the list: %+v", all)
	}
}
