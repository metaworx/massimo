// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package backup

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

func TestIgnoredDevicesAreHiddenButKept(t *testing.T) {
	_, c, _ := connect(t)
	// 1514b6 is the only device of room 6 in the fixture.
	f, err := Take(c.Snapshot(), nil, "test", time.Now(), IgnoreSet([]cube.RFAddress{0x1514b6}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.Device(0x1514b6); ok {
		t.Fatal("ignored device in the backup")
	}
	if len(f.Rooms) != 5 || hasRoom(f.Rooms, 6) {
		t.Fatalf("rooms %+v", f.Rooms)
	}
	if len(f.Devices) != 5 {
		t.Fatalf("devices %d", len(f.Devices))
	}
	// The raw lines keep everything.
	raw, _ := os.ReadFile(fixture)
	want, _ := cube.SplitLines(string(raw))
	if len(f.Lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(f.Lines), len(want))
	}
	// A restore from that file must leave the device and its room on the
	// Cube: the plan is empty and does not mention the device.
	p, err := NewPlan(f, Current{Snapshot: c.Snapshot()}, PlanOptions{Ignore: []cube.RFAddress{0x1514b6}})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Empty() {
		t.Fatalf("plan not empty: %+v", p.Steps)
	}
	for _, n := range p.Notes {
		if strings.Contains(n, "1514b6") {
			t.Fatalf("note about the ignored device: %s", n)
		}
	}
	// Even a renamed room elsewhere keeps the hidden room and device in
	// the metadata the plan writes.
	f.Rooms[0].Name = "Renamed"
	p, err = NewPlan(f, Current{Snapshot: c.Snapshot()}, PlanOptions{Ignore: []cube.RFAddress{0x1514b6}})
	if err != nil {
		t.Fatal(err)
	}
	var mline string
	for _, st := range p.Steps {
		for _, l := range st.Command.Lines {
			if l.Type == 'm' {
				mline = l.Data
			}
		}
	}
	if mline == "" {
		t.Fatalf("no metadata step: %+v", p.Steps)
	}
	_, data, _ := strings.Cut(mline, ",")
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatal(err)
	}
	md, err := cube.DecodeMetadata(b)
	if err != nil {
		t.Fatal(err)
	}
	if !hasRoom(md.Rooms, 6) {
		t.Fatalf("room 6 dropped from the metadata: %+v", md.Rooms)
	}
	found := false
	for _, d := range md.Devices {
		if d.RFAddress == 0x1514b6 && d.RoomID == 6 {
			found = true
		}
	}
	if !found {
		t.Fatalf("ignored device dropped or moved: %+v", md.Devices)
	}
	// An old backup that still lists the device does not touch it either.
	f2, _ := Take(c.Snapshot(), nil, "test", time.Now(), nil)
	for i := range f2.Devices {
		if f2.Devices[i].RFAddress == 0x1514b6 {
			f2.Devices[i].Name, f2.Devices[i].RoomID = "Moved", 1
			f2.Devices[i].Thermostat.ComfortTemp = 25
		}
	}
	p, _ = NewPlan(f2, Current{Snapshot: c.Snapshot()}, PlanOptions{Ignore: []cube.RFAddress{0x1514b6}})
	if !p.Empty() {
		t.Fatalf("plan touches the ignored device: %+v", p.Steps)
	}
}
