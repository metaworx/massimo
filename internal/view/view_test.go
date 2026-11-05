// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package view

import (
	"encoding/base64"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

const fixture = "../protocol/cube/testdata/cube-fw0113-greeting.txt"

// silentDevice is the only device of room 6 in the fixture. The fixture
// was recorded right after a Cube restart: no device has reported yet.
const silentDevice cube.RFAddress = 0x1514b6

// reportedLine is an L line in which silentDevice reports: manual,
// 21 °C, valve 0 %.
var reportedLine = cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString([]byte{0x0b, 0x15, 0x14, 0xb6, 0x09, 0x12, 0x01, 0x00, 42, 0, 0, 0})}

func snapshot(t *testing.T, extra ...cube.Message) *cubelan.Snapshot {
	t.Helper()
	lines, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	lines = append(lines, extra...)
	snap, err := cubelan.SnapshotFromLines(lines)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestRoomsAndReady(t *testing.T) {
	snap := snapshot(t)
	rooms := Rooms(snap, nil)
	if len(rooms) != 6 {
		t.Fatalf("rooms %d", len(rooms))
	}
	for _, r := range rooms {
		if r.Ready || len(r.Devices) == 0 {
			t.Errorf("room %d ready %v with %d devices", r.ID, r.Ready, len(r.Devices))
		}
		for _, d := range r.Devices {
			if d.RoomID != r.ID || d.Unlisted || d.Reported != (d.State != nil) {
				t.Errorf("room %d device %+v", r.ID, d)
			}
		}
	}
	if s := Silent(snap, 6, nil); len(s) != 1 || s[0] != silentDevice {
		t.Fatalf("silent %v", s)
	}
	if s := Silent(snap, 6, Set([]cube.RFAddress{silentDevice})); len(s) != 0 {
		t.Fatalf("hidden device counted as silent: %v", s)
	}
	snap = snapshot(t, reportedLine)
	for _, r := range Rooms(snap, nil) {
		if r.Ready != (r.ID == 6) {
			t.Errorf("after the report: room %d ready %v", r.ID, r.Ready)
		}
		if r.ID == 6 && (r.Devices[0].State == nil || r.Devices[0].State.Setpoint == nil || *r.Devices[0].State.Setpoint != 21) {
			t.Errorf("room 6 state %+v", r.Devices[0].State)
		}
	}
}

func TestHiddenRoomsAndDevices(t *testing.T) {
	snap := snapshot(t)
	md := snap.Metadata()
	h := NewHidden([]cube.RFAddress{silentDevice}, md, false)
	if !h.Rooms(md)[6] || len(h.Rooms(md)) != 1 {
		t.Fatalf("hidden rooms %v", h.Rooms(md))
	}
	if _, ok := h.Room(md, 6); ok {
		t.Fatal("room 6 visible")
	}
	if _, ok := h.Room(md, 1); !ok {
		t.Fatal("room 1 not visible")
	}
	rooms, devices := Counts(snap, h)
	allRooms, allDevices := Counts(snap, nil)
	if rooms != allRooms-1 || devices != allDevices-1 {
		t.Fatalf("counts %d/%d of %d/%d", rooms, devices, allRooms, allDevices)
	}
	for _, d := range Devices(snap, h) {
		if d.RFAddress == silentDevice.String() {
			t.Fatal("hidden device listed")
		}
		if d.Type == cube.DeviceCube {
			t.Fatal("the cube itself listed")
		}
	}
	for _, r := range Rooms(snap, h) {
		if r.ID == 6 {
			t.Fatal("hidden room listed")
		}
	}
}

func TestGhostRoomIsHidden(t *testing.T) {
	md := &cube.Metadata{
		Rooms:   []cube.RoomMeta{{ID: 1, Name: "a"}, {ID: cube.GhostRoom, Name: "parked"}},
		Devices: []cube.DeviceMeta{{RFAddress: 0x000001, RoomID: 1}, {RFAddress: 0x000002, RoomID: cube.GhostRoom}},
	}
	h := NewHidden(nil, md, false)
	if !h[0x000002] || h[0x000001] {
		t.Fatalf("hidden %v", h)
	}
	if v := h.VisibleRooms(md); len(v) != 1 || v[0].ID != 1 {
		t.Fatalf("visible %v", v)
	}
	if h := NewHidden([]cube.RFAddress{0x000001}, md, true); len(h) != 0 {
		t.Fatalf("show: hidden %v", h)
	}
}

func TestPending(t *testing.T) {
	snap := snapshot(t)
	if n := Pending(snap, cubelan.Budget{}); n != 0 {
		t.Fatalf("pending from the greeting %d", n)
	}
	b := cubelan.Budget{FreeMemorySlots: 47, MaxSlots: 50}
	b.UpdatedAt = b.UpdatedAt.AddDate(2026, 0, 0)
	if n := Pending(snap, b); n != 3 {
		t.Fatalf("pending %d", n)
	}
}
