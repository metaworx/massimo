// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package view

import (
	"encoding/base64"
	"testing"
	"time"

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
	rooms := Rooms(snap, nil, 0)
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
	for _, r := range Rooms(snap, nil, 0) {
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
	for _, d := range Devices(snap, h, 0) {
		if d.RFAddress == silentDevice.String() {
			t.Fatal("hidden device listed")
		}
		if d.Type == cube.DeviceCube {
			t.Fatal("the cube itself listed")
		}
	}
	for _, r := range Rooms(snap, h, 0) {
		if r.ID == 6 {
			t.Fatal("hidden room listed")
		}
	}
}

func TestRoom255IsHidden(t *testing.T) {
	md := &cube.Metadata{
		Rooms:   []cube.RoomMeta{{ID: 1, Name: "a"}, {ID: cube.HiddenRoom, Name: "parked"}},
		Devices: []cube.DeviceMeta{{RFAddress: 0x000001, RoomID: 1}, {RFAddress: 0x000002, RoomID: cube.HiddenRoom}},
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

func TestAgesAndStale(t *testing.T) {
	t0 := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)
	lines, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := cubelan.SnapshotFromLines(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range append(lines, reportedLine) {
		snap.Apply(m, t0)
	}
	defer func() { now = time.Now }()
	now = func() time.Time { return t0.Add(time.Hour) }
	d := DeviceOf(snap, silentDevice, 0)
	if d.LastSeen == nil || !d.LastSeen.Equal(t0) || d.Stale || !d.Received["setpoint"].Equal(t0) || !d.Received["valve_pct"].Equal(t0) {
		t.Fatalf("device %+v", d)
	}
	// Devices without state have no times.
	if d := DeviceOf(snap, 0x14d08e, 0); d.LastSeen != nil || d.Received != nil || d.Stale {
		t.Fatalf("unreported device %+v", d)
	}
	now = func() time.Time { return t0.Add(StaleAfter + time.Minute) }
	if d := DeviceOf(snap, silentDevice, 0); !d.Stale {
		t.Fatal("not stale after StaleAfter")
	}
	for _, r := range Rooms(snap, nil, 0) {
		if r.Stale != (r.ID == 6) {
			t.Errorf("room %d stale %v", r.ID, r.Stale)
		}
	}
}

func TestChanges(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	b := func(v bool) *bool { return &v }
	before := &State{Mode: cube.ModeAuto, Setpoint: f(18), ActualTemp: f(19.9), WindowOpen: b(false)}
	after := &State{Mode: cube.ModeManual, Setpoint: f(21), WindowOpen: b(true)}
	c := Changes(before, after)
	want := map[string]Change{
		"mode":        {From: "auto", To: "manual"},
		"setpoint":    {From: 18.0, To: 21.0},
		"actual_temp": {From: 19.9, To: nil},
		"window_open": {From: false, To: true},
	}
	if len(c) != len(want) {
		t.Fatalf("changes %v", c)
	}
	for k, w := range want {
		if !sameValue(c[k], w) {
			t.Errorf("%s: %v, want %v", k, c[k], w)
		}
	}
	if c := Changes(after, after); c != nil {
		t.Fatalf("no change gave %v", c)
	}
	if c := Changes(nil, after); !sameValue(c["reported"], Change{From: false, To: true}) || !sameValue(c["mode"], Change{From: nil, To: "manual"}) {
		t.Fatalf("first report %v", c)
	}
	if c := Changes(after, nil); len(c) != 1+len(stateFields(after)) || !sameValue(c["reported"], Change{From: true, To: false}) {
		t.Fatalf("state lost %v", c)
	}
}
