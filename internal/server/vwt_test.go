// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/base64"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
	"go.metaworx.ch/massimo/internal/view"
)

// storedRooms is the start of the rooms' tests: the fake Cube's rooms
// fixture (rooms), the Cube a row of a store, polled often, so that a
// state the fake is given arrives. A test of several writes in a row
// adds quiet: a poll waiting for its answer makes a write fail meanwhile
// (cubelan.ErrBusy).
func storedRooms() startOpt {
	return func(c *startConfig) {
		rooms()(c)
		stored()(c)
		editOptions(func(o *Options) { o.PollInterval = 50 * time.Millisecond })(c)
	}
}

// fastFollowUps re-reads a written device's configuration within a few
// tenths of a second.
func fastFollowUps() startOpt {
	return editOptions(func(o *Options) {
		o.FollowUpDelays = []time.Duration{100 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond}
	})
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
	r := startServer(t, storedRooms())
	ts, st := r.ts, r.st
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
	ts := startServer(t, storedRooms()).ts
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
	r := startServer(t, storedRooms())
	fc, ts, s := r.fc, r.ts, r.s
	rt := s.cubes["keq0000100"]
	stream := func(kind string) *testkit.Watch[testkit.Event] {
		url := ts.URL + "/api/v1/cubes/KEQ0000100/events?devices=" + kind
		return readStreamURL(t, url)
	}
	all, real, virtual := stream("all"), stream("real"), stream("virtual")
	b, atAll := all.AwaitN(t, 0, "the burst of all", 10, isEvent)
	if !strings.Contains(b[9].Data, `"virtual":true`) {
		t.Fatalf("the burst of all ends with %s", b[9].Data)
	}
	_, atReal := real.AwaitN(t, 0, "the burst of real", 7, isEvent)
	_, atVirtual := virtual.AwaitN(t, 0, "the burst of virtual", 9, isEvent)

	temp := 21.0 // 19.0, 21.0, 23.5: the median moves to 21.0
	if err := fc.SetState(0x0a0205, fakecube.State{ActualTemp: &temp}); err != nil {
		t.Fatal(err)
	}
	// The pump publishes the rooms' events from the snapshot as it stands
	// at each event it handles: one still busy with the greeting can send
	// a room's new temperature before the member's own event. So each
	// stream is read until every event wanted has come, in any order, a
	// room's older events passed over.
	is := func(device, has string) func(testkit.Event) bool {
		return testkit.Kind("state", `"device":"`+device+`"`, has)
	}
	all.AwaitAll(t, atAll, "the member's and room 2's state events", is("0a0205", ""), is("000002", `"actual_temp":21`))
	wall := 20.1
	if err := fc.SetState(0x0a0201, fakecube.State{ActualTemp: &wall}); err != nil {
		t.Fatal(err)
	}
	room1, atVirtual := virtual.AwaitAll(t, atVirtual, "room 1's state event", is("000001", `"actual_temp":20.1`))
	if !strings.Contains(room1[0].Data, `"changes"`) {
		t.Fatalf("room 1's event: %s", room1[0].Data)
	}
	_, atReal = real.AwaitAll(t, atReal, "the real wall thermostat's state event", is("0a0201", ""))
	// Once the pump has handled every event the client sent (pumpSync), a
	// test event ends the checks: what the views would carry of them was
	// published before it, a pump still behind included.
	rt.pumpSync(t)
	mark(rt, 1)
	real.Never(t, atReal, "a virtual wall thermostat's event on the real view", func(e testkit.Event) bool {
		return strings.Contains(e.Data, `"virtual":true`)
	}, real.Seen(atReal, testkit.Kind("test")))
	virtual.Never(t, atVirtual, "the real wall thermostat's event on the virtual view", func(e testkit.Event) bool {
		return strings.Contains(e.Data, `"device":"0a0201"`)
	}, virtual.Seen(atVirtual, testkit.Kind("test")))
	rt.subsMu.Lock()
	defer rt.subsMu.Unlock()
	if len(rt.replay) == 0 {
		t.Fatal("nothing kept for replay")
	}
}

// The view virtual leaves out every event of the real wall thermostat,
// not only the state events the pump marks: a mismatch of its
// configuration too, which a stream opened before the pump came to it
// would otherwise carry; the views that show it carry the event.
func TestVirtualViewLeavesOutTheRealWallsEvents(t *testing.T) {
	// The start has the greeting's mismatches through the pump before
	// the streams open: a pump slow to come to the greeting (armv6 under
	// QEMU) would publish them on the streams, beside the test's own.
	r := startServer(t, storedRooms())
	ts, s := r.ts, r.s
	rt := s.cubes["keq0000100"]
	stream := func(kind string, burst int) (*testkit.Watch[testkit.Event], testkit.Cursor) {
		evs := readStreamURL(t, ts.URL+"/api/v1/cubes/KEQ0000100/events?devices="+kind)
		_, after := evs.AwaitN(t, 0, "the burst of "+kind, burst, isEvent)
		return evs, after
	}
	all, atAll := stream("all", 10)
	virtual, atVirtual := stream("virtual", 9)
	mismatch := func(device string) event {
		return event{Kind: "mismatch", Source: "cube", Level: "warn", Device: device, Time: time.Now(), Summary: map[string]any{"finding": "test"}}
	}
	rt.send(mismatch("0a0201")) // the real wall thermostat
	rt.send(mismatch("0a0204")) // a radiator thermostat, in every view
	// upTo reads a stream up to the test's mismatch of device: the
	// devices of the test's mismatches, and whether any event named the
	// real wall thermostat.
	upTo := func(evs *testkit.Watch[testkit.Event], c testkit.Cursor, device string) (got []string, wall bool) {
		t.Helper()
		for {
			var ev testkit.Event
			ev, c = evs.Await(t, c, "the test's mismatch of "+device, isEvent)
			wall = wall || strings.Contains(ev.Data, `"device":"0a0201"`)
			if ev.Kind != "mismatch" {
				continue
			}
			for _, d := range []string{"0a0201", "0a0204"} {
				if strings.Contains(ev.Data, `"device":"`+d+`"`) {
					got = append(got, d)
				}
			}
			if strings.Contains(ev.Data, `"device":"`+device+`"`) {
				return got, wall
			}
		}
	}
	if got, wall := upTo(virtual, atVirtual, "0a0204"); !slices.Equal(got, []string{"0a0204"}) || wall {
		t.Fatalf("the virtual view's mismatches: %v, the real wall thermostat's event %v", got, wall)
	}
	if got, _ := upTo(all, atAll, "0a0204"); !slices.Equal(got, []string{"0a0201", "0a0204"}) {
		t.Fatalf("the view all's mismatches: %v", got)
	}
}

// lastRadio is the last radio command among the lines a fake Cube
// received.
func lastRadio(t *testing.T, received *testkit.Watch[cube.Message]) cube.RadioCommand {
	t.Helper()
	got := received.Since(0, cubewatch.Type('s'))
	if len(got) == 0 {
		t.Fatal("no radio command")
	}
	rc, err := cube.ParseRadioCommand(got[len(got)-1].Data)
	if err != nil {
		t.Fatal(err)
	}
	return rc
}

// A virtual wall thermostat's commands reach its room, or its room's real
// wall thermostat; its settings the store; it is neither renamed, moved
// nor deleted.
func TestVirtualWallThermostatCommands(t *testing.T) {
	r := startServer(t, storedRooms(), quiet())
	fc, ts, s, st := r.fc, r.ts, r.s, r.st
	received := cubewatch.Lines(fc)
	path := "/api/v1/cubes/KEQ0000100/devices/"
	room := func(rc cube.RadioCommand, cmd byte, id uint8) bool {
		return rc.Command == cmd && rc.Target.Room && rc.Target.RoomID == id
	}

	if code, body := do(t, ts, "PUT", path+"room-2/setpoint", `{"mode":"manual","temp":21.5}`); code != 200 || !room(lastRadio(t, received), cube.RFSetTemperature, 2) {
		t.Fatalf("setpoint: %d %v %+v", code, body, lastRadio(t, received))
	}
	if code, body := do(t, ts, "PUT", path+"000002/program", `{"monday":[{"temp":17,"until":"06:00"},{"temp":21,"until":"24:00"}]}`); code != 200 || !room(lastRadio(t, received), cube.RFSetWeekProgram, 2) {
		t.Fatalf("program: %d %v", code, body)
	}

	// The room's settings and the virtual wall thermostat's in one body.
	code, body := do(t, ts, "PUT", path+"VWT0000002/config", `{"comfort_temp":22.5,"temp_method":"mean"}`)
	if code != 200 || body["room"] == nil || body["settings"] == nil {
		t.Fatalf("config: %d %v", code, body)
	}
	if rc := lastRadio(t, received); rc.Command != cube.RFSetTemperatures {
		t.Fatalf("config sent %+v", rc)
	}
	rows, _ := st.Rooms(context.Background(), 1)
	if rows[1].TempMethod != "mean" {
		t.Fatalf("the store's room 2: %+v", rows[1])
	}
	var d vwtDetail
	getJSON(t, ts, "", path+"room-2", &d)
	if d.Settings.TempMethod != "mean" || *d.State.ActualTemp != 20.7 {
		t.Fatalf("after the settings: %+v %v", d.Settings, *d.State.ActualTemp)
	}
	// The settings alone answer with them.
	if code, body := do(t, ts, "PUT", path+"room-2/config", `{"temp_method":"median"}`); code != 200 || body["settings"] == nil || body["room"] != nil {
		t.Fatalf("settings alone: %d %v", code, body)
	}

	// The real wall thermostat's own: room 1 has one, room 2 none.
	if code, _ := do(t, ts, "PUT", path+"room-2/config", `{"temp_offset":1}`); code != 400 {
		t.Fatalf("an offset without a wall thermostat: %d", code)
	}
	if code, body := do(t, ts, "PUT", path+"room-1/config", `{"display_actual":true}`); code != 200 {
		t.Fatalf("display: %d %v", code, body)
	} else if rc := lastRadio(t, received); rc.Command != cube.RFSetDisplayMode || rc.Target.RFAddress != 0x0a0201 {
		t.Fatalf("display sent %+v", rc)
	}
	if code, _ := do(t, ts, "PUT", path+"room-2/lock", `{"locked":true}`); code != 409 {
		t.Fatalf("a lock without a wall thermostat: %d", code)
	}
	if code, body := do(t, ts, "PUT", path+"room-1/lock", `{"locked":true}`); code != 200 {
		t.Fatalf("lock: %d %v", code, body)
	} else if rc := lastRadio(t, received); rc.Command != cube.RFLockControls || rc.Target.RFAddress != 0x0a0201 {
		t.Fatalf("lock sent %+v", rc)
	}

	// Its address: the owner's, unless a device or another room has it.
	for addr, want := range map[string]int{"0a0201": 409, "000001": 409, "zz": 400, "000150": 200} {
		if code, _ := do(t, ts, "PUT", path+"room-2/config", `{"rf_address":"`+addr+`"}`); code != want {
			t.Fatalf("rf_address %s: %d, want %d", addr, code, want)
		}
	}
	getJSON(t, ts, "", path+"room-2", &d)
	if d.RFAddress != "000150" || d.Settings.RFAddress != "000150" {
		t.Fatalf("after the address: %s %+v", d.RFAddress, d.Settings)
	}
	if code, _ := do(t, ts, "PUT", path+"000150/setpoint", `{"mode":"auto"}`); code != 200 {
		t.Fatalf("by its new address: %d", code)
	}

	for _, c := range []struct{ method, path, body string }{
		{"PUT", "room-1", `{"name":"Neu"}`},
		{"PUT", "room-1", `{"room_id":2}`},
		{"DELETE", "room-1", ""},
	} {
		if code, _ := do(t, ts, c.method, path+c.path, c.body); code != 409 {
			t.Fatalf("%s %s: %d", c.method, c.path, code)
		}
	}
	_ = s
}

// A room takes one wall thermostat: adding or moving a second into it is
// refused before anything is sent, as the original software refuses it.
func TestSecondWallThermostatRefused(t *testing.T) {
	r := startServer(t, storedRooms(), changeGreeting(func(lines []cube.Message) []cube.Message {
		p, err := fakecube.PairingFrom(lines, 0x0a0201, 0x0a0299, "NEQ0000299", 0)
		if err != nil {
			t.Fatal(err)
		}
		out := append([]cube.Message(nil), lines[:len(lines)-1]...)
		return append(out, p.Config, lines[len(lines)-1])
	}), quiet())
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	before := received.Mark()
	code, body := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0299", `{"room_id":1,"name":"Zweiter"}`)
	if code != 409 || !strings.Contains(body["error"].(string), "wall thermostat already") {
		t.Fatalf("a second wall thermostat: %d %v", code, body)
	}
	if got := received.Since(before, cubewatch.Type('s', 'm')); len(got) != 0 {
		t.Fatalf("sent %v", got)
	}
}

// A boost goes over the air as the room's set-temperature command with
// mode 3, carrying the setpoint the room has; on a virtual wall
// thermostat too. The fake Cube applies it as the devices do.
func TestBoostOverTheAir(t *testing.T) {
	r := startServer(t, storedRooms(), quiet())
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	for _, path := range []string{"rooms/2/setpoint", "devices/0a0205/setpoint", "devices/room-2/setpoint"} {
		code, body := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/"+path, `{"mode":"boost"}`)
		if code != 200 {
			t.Fatalf("%s: %d %v", path, code, body)
		}
		got := received.Since(0, cubewatch.Except('l'))
		want, _ := cube.CmdSetTemperature(cube.Target{RFAddress: lastRadio(t, received).Target.RFAddress, RoomID: 2, Room: true}, 20, cube.ModeBoost, cube.Until{})
		if last := got[len(got)-1]; last.String() != want.String() {
			t.Fatalf("%s sent %s, want %s", path, last, want)
		}
	}
}

// The option proxy_virtual is kept per Cube; with it on, the proxy gets
// a virtual wall thermostat per room whose C line and L entry read as a
// wall thermostat's, with the room's programme and temperature.
func TestProxyVirtualOption(t *testing.T) {
	r := startServer(t, storedRooms(), quiet())
	ts, s, st := r.ts, r.s, r.st
	rt := s.cubes["keq0000100"]
	if vs := s.proxyVirtuals(rt); vs != nil {
		t.Fatalf("off by default: %d", len(vs))
	}
	if code, body := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_virtual":true}`); code != 200 || body["proxy_virtual"] != true {
		t.Fatalf("PUT: %d %v", code, body)
	}
	if c, _, _ := st.CubeByID(context.Background(), 1); !c.ProxyVirtual {
		t.Fatal("not kept in the store")
	}
	vs := s.proxyVirtuals(rt)
	if len(vs) != 3 {
		t.Fatalf("%d virtual wall thermostats", len(vs))
	}
	for _, v := range vs {
		cfg, err := cube.ParseConfig(v.RF.String() + "," + base64.StdEncoding.EncodeToString(v.Config))
		if err != nil || cfg.Type != cube.DeviceWallThermostat || cfg.RoomID != v.Room.ID || cfg.Serial != v.Serial {
			t.Fatalf("room %d's C line: %+v %v", v.Room.ID, cfg, err)
		}
		states, err := cube.ParseDeviceList(base64.StdEncoding.EncodeToString(v.Entry), map[cube.RFAddress]cube.DeviceType{v.RF: cube.DeviceWallThermostat})
		if err != nil || len(states) != 1 {
			t.Fatalf("room %d's L entry: %v", v.Room.ID, err)
		}
		switch v.Room.ID {
		case 1:
			if v.Wall != 0x0a0201 || states[0].ActualTemp == nil || *states[0].ActualTemp != 21.4 || len(cfg.Wall.WeekProgram[cube.Monday]) == 0 {
				t.Fatalf("room 1: %+v %+v", v, states[0])
			}
		case 2:
			if v.Wall != 0 || v.Member != 0x0a0204 || *states[0].ActualTemp != 19.5 {
				t.Fatalf("room 2: %+v %+v", v, states[0])
			}
		}
	}
	do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_virtual":false}`)
	if s.proxyVirtuals(rt) != nil {
		t.Fatal("still on")
	}
}
