// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// startRooms starts a server with a store over the fake Cube's rooms
// fixture: room 1 with a wall thermostat and two radiator thermostats,
// room 2 with three radiator thermostats, room 3 with a contact alone.
// The server polls often, so that a state the fake is given arrives.
func startRooms(t *testing.T) (*fakecube.Server, *httptest.Server, *Server, *store.Store) {
	t.Helper()
	greeting, err := fakecube.RoomsFixture()
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting, Apply: true}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	st := testStore(t, "")
	row, err := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := CubeFromRow(row)
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Cubes: []CubeConfig{cfg}, Store: st, Queue: engine.Options{Gap: time.Millisecond}, PollInterval: 50 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	waitConnected(t, ts, "KEQ0000100")
	eventually(t, "the rooms' uids", func() bool {
		var rooms []view.Room
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/rooms", &rooms)
		return len(rooms) == 3 && rooms[2].UID != ""
	})
	return fc, ts, s, st
}

// devicesIn is a room's devices in a view, by radio address.
func devicesIn(t *testing.T, ts *httptest.Server, kind string, room int) []view.Device {
	t.Helper()
	var rooms []view.Room
	if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/rooms?devices="+kind, &rooms); code != 200 {
		t.Fatalf("rooms?devices=%s: %d", kind, code)
	}
	for _, r := range rooms {
		if int(r.ID) == room {
			return r.Devices
		}
	}
	t.Fatalf("no room %d", room)
	return nil
}

// The four views of the rooms and devices, the rooms' uids from the
// store, and the room's temperature.
func TestVirtualWallThermostatViews(t *testing.T) {
	_, ts, _, st := startRooms(t)
	addrs := func(ds []view.Device) string {
		var out []string
		for _, d := range ds {
			out = append(out, d.RFAddress)
		}
		return strings.Join(out, ",")
	}
	for _, c := range []struct {
		kind         string
		room1, room2 string
	}{
		{"", "0a0201,0a0202,0a0203", "000002,0a0204,0a0205,0a0206"},
		{"default", "0a0201,0a0202,0a0203", "000002,0a0204,0a0205,0a0206"},
		{"real", "0a0201,0a0202,0a0203", "0a0204,0a0205,0a0206"},
		{"virtual", "000001,0a0202,0a0203", "000002,0a0204,0a0205,0a0206"},
		{"all", "000001,0a0201,0a0202,0a0203", "000002,0a0204,0a0205,0a0206"},
	} {
		if got := addrs(devicesIn(t, ts, c.kind, 1)); got != c.room1 {
			t.Errorf("%q room 1: %s, want %s", c.kind, got, c.room1)
		}
		if got := addrs(devicesIn(t, ts, c.kind, 2)); got != c.room2 {
			t.Errorf("%q room 2: %s, want %s", c.kind, got, c.room2)
		}
	}
	if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/rooms?devices=some", nil); code != 400 {
		t.Fatalf("an unknown view: %d", code)
	}
	var devs []view.Device
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices?devices=virtual", &devs)
	if len(devs) != 9 {
		t.Fatalf("devices in the virtual view: %d", len(devs))
	}

	vwt := devicesIn(t, ts, "all", 1)[0]
	if !vwt.Virtual || vwt.Serial != "VWT0000001" || vwt.Type != cube.DeviceWallThermostat || *vwt.State.ActualTemp != 21.4 {
		t.Fatalf("room 1's: %+v", vwt)
	}
	if v := devicesIn(t, ts, "all", 2)[0]; *v.State.ActualTemp != 19.5 || len(v.Sources["actual_temp"]) != 3 {
		t.Fatalf("room 2's median: %+v", v)
	}

	rows, err := st.Rooms(context.Background(), 1)
	if err != nil || len(rows) != 3 {
		t.Fatalf("store rooms %+v %v", rows, err)
	}
	var rooms []view.Room
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/rooms", &rooms)
	if rooms[0].UID != rows[0].UUID || devicesIn(t, ts, "all", 1)[0].RoomUID != rows[0].UUID {
		t.Fatalf("room 1's uid %q, store %q", rooms[0].UID, rows[0].UUID)
	}
}

// A virtual wall thermostat is found by its room, its serial and its
// address, with its room's settings and week programme.
func TestVirtualWallThermostatDevice(t *testing.T) {
	_, ts, _, _ := startRooms(t)
	for _, name := range []string{"room-2", "ROOM-2", "VWT0000002", "000002"} {
		var d vwtDetail
		if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/"+name, &d); code != 200 {
			t.Fatalf("%s: %d", name, code)
		}
		if !d.Virtual || d.RoomID != 2 || d.Wall == nil || len(d.Wall.WeekProgram[cube.Monday]) == 0 || d.Settings.TempMethod != "median" {
			t.Fatalf("%s: %+v", name, d)
		}
	}
	if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/room-9", nil); code != 404 {
		t.Fatalf("room-9: %d", code)
	}
	// A real device's address is the real device.
	var d deviceDetail
	if getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/0a0201", &d) != 200 || d.Virtual || d.Serial != "NEQ0000201" {
		t.Fatalf("the real wall thermostat %+v", d)
	}
}

// A member's report that changes the room's temperature is a state event
// of the virtual wall thermostat too, on the streams whose view shows it;
// the virtual view leaves the real wall thermostat's events out.
func TestVirtualWallThermostatEvents(t *testing.T) {
	fc, ts, s, _ := startRooms(t)
	rt := s.cubes["keq0000100"]
	stream := func(kind string) <-chan sseEvent {
		url := ts.URL + "/api/v1/cubes/KEQ0000100/events?devices=" + kind
		return readStreamURL(t, url)
	}
	burstOf := func(evs <-chan sseEvent, n int) []sseEvent {
		var out []sseEvent
		for i := 0; i < n; i++ {
			out = append(out, next(t, evs))
		}
		return out
	}
	all, real, virtual := stream("all"), stream("real"), stream("virtual")
	if b := burstOf(all, 10); !strings.Contains(b[9].data, `"virtual":true`) {
		t.Fatalf("the burst of all ends with %s", b[9].data)
	}
	burstOf(real, 7)
	burstOf(virtual, 9)

	temp := 21.0 // 19.0, 21.0, 23.5: the median moves to 21.0
	if err := fc.SetState(0x0a0205, fakecube.State{ActualTemp: &temp}); err != nil {
		t.Fatal(err)
	}
	wait := func(evs <-chan sseEvent, device string) sseEvent {
		t.Helper()
		for {
			ev := next(t, evs)
			if ev.kind == "state" && strings.Contains(ev.data, `"device":"`+device+`"`) {
				return ev
			}
		}
	}
	wait(all, "0a0205")
	if ev := wait(all, "000002"); !strings.Contains(ev.data, `"actual_temp":21`) {
		t.Fatalf("room 2's event: %s", ev.data)
	}
	wall := 20.1
	if err := fc.SetState(0x0a0201, fakecube.State{ActualTemp: &wall}); err != nil {
		t.Fatal(err)
	}
	ev := wait(virtual, "000001")
	if !strings.Contains(ev.data, `"actual_temp":20.1`) || !strings.Contains(ev.data, `"changes"`) {
		t.Fatalf("room 1's event: %s", ev.data)
	}
	wait(real, "0a0201")
	for {
		select {
		case e := <-real:
			if strings.Contains(e.data, `"virtual":true`) {
				t.Fatalf("the real view got %s", e.data)
			}
			continue
		case e := <-virtual:
			if strings.Contains(e.data, `"device":"0a0201"`) {
				t.Fatalf("the virtual view got the real wall thermostat: %s", e.data)
			}
			continue
		case <-time.After(300 * time.Millisecond):
		}
		break
	}
	rt.subsMu.Lock()
	defer rt.subsMu.Unlock()
	if len(rt.replay) == 0 {
		t.Fatal("nothing kept for replay")
	}
}
