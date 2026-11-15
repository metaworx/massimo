// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/battery"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// A device's battery flag makes it weak (AP LegacyLogs v1.0, Block 11):
// its view says so, its log has the change as a battery event, and a
// runtime started anew over the same store rebuilds the level from the
// log's state rows.
func TestBatteryLevel(t *testing.T) {
	fc, ts, s, st := startStored(t, Options{PollInterval: 100 * time.Millisecond})
	var d deviceDetail
	// 0a010d has no state in the fixture, and so no level.
	if getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/0a010d", &d) != 200 || d.Battery != "" {
		t.Fatalf("before its report: %+v", d.Device)
	}
	on := true
	if err := fc.SetState(0x0a010d, fakecube.State{Reported: &on, BatteryLow: &on}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the weak level", func() bool {
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/0a010d", &d) == 200 && d.Battery == "weak"
	})
	waitFor(t, "the battery event", func() bool { return hasKind(logKinds(t, ts, 0), "battery:0a010d") })
	var devs []deviceDetail
	if getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices", &devs) != 200 {
		t.Fatal("device list")
	}
	for _, dv := range devs {
		if dv.RFAddress == "0a010d" && dv.Battery != "weak" {
			t.Fatalf("the list's device %+v", dv.Device)
		}
	}

	if err := st.FlushEvents(context.Background()); err != nil {
		t.Fatal(err)
	}
	rt := s.runtimes()[0]
	again := s.newRuntime(rt.cfg, "")
	s.rebuildBattery(context.Background(), again)
	if l := again.battery.Level("0a010d"); l != battery.Weak {
		t.Fatalf("rebuilt from the log: %s", l)
	}
}
