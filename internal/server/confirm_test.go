// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

const confirmBase = "/api/v1/cubes/KEQ0000100"

// startConfirmCube starts a server over the fake Cube's rooms fixture
// (fakecube.RoomsFixture), which applies the commands it takes after
// applyAfter, polled often, a write's devices given within to show it
// (AP RoomConfirm v1.0, Block 1).
func startConfirmCube(t *testing.T, within, applyAfter time.Duration) (*fakecube.Server, *httptest.Server, *runtime) {
	t.Helper()
	greeting, err := fakecube.RoomsFixture()
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting, Apply: true, ApplyAfter: applyAfter}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond},
		PollInterval: 50 * time.Millisecond, RefreshInterval: time.Hour, FollowUpReconnectGap: time.Hour, confirmWithin: within})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	waitConnected(t, ts, "KEQ0000100")
	return fc, ts, s.cubes["keq0000100"]
}

// confirmRoom is a room of the API's list.
func confirmRoom(t *testing.T, ts *httptest.Server, id uint8) view.Room {
	t.Helper()
	var rooms []view.Room
	getJSON(t, ts, "", confirmBase+"/rooms", &rooms)
	for _, r := range rooms {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no room %d in %+v", id, rooms)
	return view.Room{}
}

// confirmDevice is a device of the API.
func confirmDevice(t *testing.T, ts *httptest.Server, rf string) view.Device {
	t.Helper()
	var d view.Device
	if code := getJSON(t, ts, "", confirmBase+"/devices/"+rf, &d); code != 200 {
		t.Fatalf("device %s: %d", rf, code)
	}
	return d
}

// unconfirmedEvents are the log's unconfirmed events of an outcome,
// newest first.
func unconfirmedEvents(t *testing.T, ts *httptest.Server, outcome string) []event {
	t.Helper()
	var evs, out []event
	getJSON(t, ts, "", confirmBase+"/log?limit=500", &evs)
	for _, e := range evs {
		if e.Kind == "unconfirmed" && e.Summary["outcome"] == outcome {
			out = append(out, e)
		}
	}
	return out
}

// waitUnconfirmed waits for the first unconfirmed event of an outcome.
func waitUnconfirmed(t *testing.T, ts *httptest.Server, outcome string) event {
	t.Helper()
	var found []event
	waitFor(t, "an unconfirmed event, "+outcome, func() bool {
		found = unconfirmedEvents(t, ts, outcome)
		return len(found) > 0
	})
	return found[len(found)-1]
}

// openWrites is how many writes the runtime has open.
func openWrites(rt *runtime) int {
	rt.confirms.mu.Lock()
	defer rt.confirms.mu.Unlock()
	return len(rt.confirms.writes)
}

// A room command its members take is pending, then done once every
// member shows it, with no line.
func TestRoomCommandConfirmed(t *testing.T) {
	_, ts, rt := startConfirmCube(t, 3*time.Second, 100*time.Millisecond)
	code, out := do(t, ts, "PUT", confirmBase+"/rooms/2/setpoint", `{"mode":"manual","temp":22}`)
	if code != 200 || out["confirmation"] != "pending" {
		t.Fatalf("setpoint: %d %v", code, out)
	}
	eventually(t, "room 2 confirmed", func() bool { return confirmRoom(t, ts, 2).Confirmation == nil && openWrites(rt) == 0 })
	if evs := unconfirmedEvents(t, ts, "not_confirmed"); len(evs) != 0 {
		t.Fatalf("lines %+v", evs)
	}
}

// A member that does not take a room command keeps the room pending,
// then not confirmed at the deadline: one warning names it, what it
// shows and its flags; the room and the device say so until it shows
// it, which a line at info says.
func TestRoomCommandNotConfirmed(t *testing.T) {
	fc, ts, _ := startConfirmCube(t, 600*time.Millisecond, 0)
	fc.SetDeaf(0x0a0205, true)
	on := true
	if err := fc.SetState(0x0a0205, fakecube.State{Error: &on}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the error flag", func() bool {
		d := confirmDevice(t, ts, "0a0205")
		return d.State != nil && d.State.Error
	})
	code, out := do(t, ts, "PUT", confirmBase+"/rooms/2/setpoint", `{"mode":"manual","temp":22}`)
	if code != 200 || out["confirmation"] != "pending" {
		t.Fatalf("setpoint: %d %v", code, out)
	}
	if c := confirmRoom(t, ts, 2).Confirmation; c == nil || c.State != "pending" || !reflect.DeepEqual(c.Fields, []string{"mode", "setpoint"}) {
		t.Fatalf("pending room: %+v", c)
	}

	ev := waitUnconfirmed(t, ts, "not_confirmed")
	if ev.Level != "warn" || ev.Source != "device" || ev.Device != "" || ev.Summary["write"] != "setpoint" || ev.Summary["target"] != "room:2" ||
		ev.Summary["room"] != float64(2) || ev.Summary["source"] != "api" || ev.Summary["confirmed"] != float64(2) {
		t.Fatalf("event %+v", ev)
	}
	if want := map[string]any{"mode": "manual", "setpoint": float64(22)}; !reflect.DeepEqual(ev.Summary["expected"], want) {
		t.Fatalf("expected %v", ev.Summary["expected"])
	}
	want := []any{map[string]any{"rf_address": "0a0205", "serial": "MEQ0000205", "fields": []any{"mode", "setpoint"},
		"shows": map[string]any{"mode": "auto", "setpoint": float64(20)}, "error": true, "link_error": false, "out_of_date": false}}
	if !reflect.DeepEqual(ev.Summary["devices"], want) {
		t.Fatalf("devices %v", ev.Summary["devices"])
	}
	c := confirmRoom(t, ts, 2).Confirmation
	if c == nil || c.State != "unconfirmed" || !reflect.DeepEqual(c.Devices, []string{"0a0205"}) || !reflect.DeepEqual(c.Fields, []string{"mode", "setpoint"}) {
		t.Fatalf("room: %+v", c)
	}
	if c := confirmDevice(t, ts, "0a0205").Confirmation; c == nil || c.State != "unconfirmed" {
		t.Fatalf("device: %+v", c)
	}
	if c := confirmDevice(t, ts, "0a0204").Confirmation; c != nil {
		t.Fatalf("a member that took it: %+v", c)
	}

	manual, temp := "manual", 22.0
	if err := fc.SetState(0x0a0205, fakecube.State{Mode: &manual, Setpoint: &temp}); err != nil {
		t.Fatal(err)
	}
	late := waitUnconfirmed(t, ts, "confirmed_late")
	if late.Level != "info" || late.Device != "0a0205" || late.Serial != "MEQ0000205" || late.Summary["target"] != "room:2" {
		t.Fatalf("late %+v", late)
	}
	eventually(t, "room 2 confirmed", func() bool { return confirmRoom(t, ts, 2).Confirmation == nil })
	if evs := unconfirmedEvents(t, ts, "not_confirmed"); len(evs) != 1 {
		t.Fatalf("%d warnings", len(evs))
	}
}

// A newer setpoint of a room replaces the one before it: one warning, for
// the newer, at its deadline.
func TestNewerWriteReplaces(t *testing.T) {
	fc, ts, rt := startConfirmCube(t, 600*time.Millisecond, 0)
	fc.SetDeaf(0x0a0205, true)
	for _, body := range []string{`{"mode":"manual","temp":22}`, `{"mode":"manual","temp":23}`} {
		if code, out := do(t, ts, "PUT", confirmBase+"/rooms/2/setpoint", body); code != 200 || out["confirmation"] != "pending" {
			t.Fatalf("setpoint %s: %d %v", body, code, out)
		}
	}
	if n := openWrites(rt); n != 1 {
		t.Fatalf("%d writes open", n)
	}
	ev := waitUnconfirmed(t, ts, "not_confirmed")
	if e := ev.Summary["expected"].(map[string]any); e["setpoint"] != float64(23) {
		t.Fatalf("expected %v", e)
	}
	time.Sleep(300 * time.Millisecond)
	if evs := unconfirmedEvents(t, ts, "not_confirmed"); len(evs) != 1 {
		t.Fatalf("%d warnings", len(evs))
	}
}

// A setpoint a client of the LAN proxy sent to a room is confirmed by
// the room's thermostats as the API's are: here none takes it.
func TestProxyWriteConfirmed(t *testing.T) {
	_, ts, rt := startConfirmCube(t, 300*time.Millisecond, 0)
	m, err := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0204, RoomID: 2, Room: true}, 21.5, cube.ModeManual, cube.Until{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := cube.ParseRadioCommand(m.Data)
	if err != nil {
		t.Fatal(err)
	}
	rt.proxyWrote(c) // as if the Cube had taken it
	ev := waitUnconfirmed(t, ts, "not_confirmed")
	devices, _ := ev.Summary["devices"].([]any)
	if ev.Summary["source"] != "proxy" || ev.Summary["target"] != "room:2" || ev.Summary["confirmed"] != float64(0) || len(devices) != 3 {
		t.Fatalf("event %+v", ev)
	}
	if e := ev.Summary["expected"].(map[string]any); e["mode"] != "manual" || e["setpoint"] != 21.5 {
		t.Fatalf("expected %v", e)
	}
}

// A key lock and a display mode are confirmed by the L entry, and the
// display write is no C line's to follow.
func TestLockAndDisplayConfirmed(t *testing.T) {
	_, ts, rt := startConfirmCube(t, 3*time.Second, 100*time.Millisecond)
	if code, out := do(t, ts, "PUT", confirmBase+"/devices/0a0204/lock", `{"locked":true}`); code != 200 || out["confirmation"] != "pending" {
		t.Fatalf("lock: %d %v", code, out)
	}
	if code, out := do(t, ts, "PUT", confirmBase+"/devices/0a0201/config", `{"display_actual":true}`); code != 200 || out["confirmation"] != "pending" {
		t.Fatalf("display: %d %v", code, out)
	}
	rt.follow.mu.Lock()
	_, followed := rt.follow.devices[0x0a0201]
	rt.follow.mu.Unlock()
	if followed {
		t.Fatal("the display write's C line followed")
	}
	if c := confirmDevice(t, ts, "0a0201").Confirmation; c == nil || !reflect.DeepEqual(c.Fields, []string{"display_actual"}) {
		t.Fatalf("display pending: %+v", c)
	}
	eventually(t, "both confirmed", func() bool { return openWrites(rt) == 0 })
	if d := confirmDevice(t, ts, "0a0204"); d.Confirmation != nil || !d.State.PanelLocked {
		t.Fatalf("locked: %+v", d)
	}
}

// What a 0x40 write is confirmed by: the mode always, the setpoint in
// manual and vacation, a vacation's end.
func TestSetpointWant(t *testing.T) {
	end := cube.Until{Year: 2026, Month: 11, Day: 20, Minutes: 600}
	for _, tc := range []struct {
		st   cube.SetTemperature
		want map[string]any
	}{
		{cube.SetTemperature{Mode: cube.ModeManual, Setpoint: 21}, map[string]any{"mode": cube.ModeManual, "setpoint": 21.0}},
		{cube.SetTemperature{Mode: cube.ModeAuto, Setpoint: 19}, map[string]any{"mode": cube.ModeAuto}},
		{cube.SetTemperature{Mode: cube.ModeBoost, Setpoint: 20}, map[string]any{"mode": cube.ModeBoost}},
		{cube.SetTemperature{Mode: cube.ModeVacation, Setpoint: 12, Until: end}, map[string]any{"mode": cube.ModeVacation, "setpoint": 12.0, "until": end}},
	} {
		if got := setpointWant(tc.st); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%+v: %v, want %v", tc.st, got, tc.want)
		}
	}
	// An entry out of date shows nothing.
	sp := 21.0
	st := cube.DeviceState{Mode: cube.ModeManual, Setpoint: &sp, Initialized: true, OutOfDate: true}
	if m := missing(st, true, map[string]any{"mode": cube.ModeManual, "setpoint": 21.0}); len(m) != 2 {
		t.Errorf("out of date: %v", m)
	}
	st.Valid, st.OutOfDate = true, false
	if m := missing(st, true, map[string]any{"mode": cube.ModeManual, "setpoint": 21.0}); len(m) != 0 {
		t.Errorf("valid: %v", m)
	}
}
