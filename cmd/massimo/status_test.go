// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

func TestStatusFromSnapshot(t *testing.T) {
	lines, err := fakecube.LoadGreeting("../../internal/protocol/cube/testdata/cube-fw0113-greeting.txt")
	if err != nil {
		t.Fatal(err)
	}
	snap, err := cubelan.SnapshotFromLines(lines)
	if err != nil {
		t.Fatal(err)
	}
	// 1514b6 is the only device of room 6: hiding it hides the room.
	rep := statusFromSnapshot(snap, cubelan.Budget{}, view.Set([]cube.RFAddress{0x1514b6}))
	if rep.Serial != "KEQ0000100" || len(rep.Rooms) != 5 || len(rep.Unlisted) != 0 {
		t.Fatalf("report %+v", rep)
	}
	for _, r := range rep.Rooms {
		if r.ID == 6 || r.Ready || len(r.Devices) == 0 {
			t.Fatalf("room %+v", r)
		}
		for _, d := range r.Devices {
			if d.Room != r.Name || d.Reported {
				t.Fatalf("device %+v", d)
			}
		}
	}
}
