// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// -pair names a copy of a device of the greeting, joining after two
// seconds unless a delay is given; anything else is refused.
func TestParsePair(t *testing.T) {
	greeting, err := fakecube.LoadGreeting("../../internal/protocol/cube/testdata/cube-fw0113-greeting.txt")
	if err != nil {
		t.Fatal(err)
	}
	p, err := parsePair(greeting, "0a010d,0abcde,TEQ0000001")
	if err != nil || p.Device.RFAddress != 0x0abcde || p.Device.Serial != "TEQ0000001" || p.Device.Type != cube.DeviceRadiatorThermostat || p.Delay != 2*time.Second {
		t.Fatalf("%+v %v", p, err)
	}
	if p, err := parsePair(greeting, "0a010d,0abcde,TEQ0000001,500ms"); err != nil || p.Delay != 500*time.Millisecond {
		t.Fatalf("with a delay: %+v %v", p, err)
	}
	for _, bad := range []string{"0a010d,0abcde", "0a010d,0abcde,TEQ0000001,soon", "0f0f0f,0abcde,TEQ0000001", "0a010d,xyz,TEQ0000001", "0a010d,0abcde,TEQ00000012"} {
		if _, err := parsePair(greeting, bad); err == nil {
			t.Errorf("-pair %s accepted", bad)
		}
	}
}
