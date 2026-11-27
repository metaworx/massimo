// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"fmt"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/view"
)

const confirmBase = "/api/v1/cubes/KEQ0000100"

// confirming is the start of the confirmations' tests: the fake Cube's
// rooms fixture (rooms), which applies the commands it takes after
// applyAfter, polled often, a write's devices given within to show it
// (AP RoomConfirm v1.0, Block 1), and a written C line re-read after
// follow, nil for the defaults, without reconnects.
func confirming(within, applyAfter time.Duration, follow ...time.Duration) startOpt {
	return func(c *startConfig) {
		rooms()(c)
		withFake(func(fc *fakecube.Server) { fc.ApplyAfter = applyAfter })(c)
		editOptions(func(o *Options) {
			o.PollInterval, o.FollowUpDelays, o.FollowUpReconnectGap, o.confirmWithin = 50*time.Millisecond, follow, time.Hour, within
		})(c)
	}
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

// unconfirmedOf matches the log's unconfirmed events of an outcome.
func unconfirmedOf(outcome string) func(event) bool {
	return func(e event) bool { return logLine("unconfirmed", "")(e) && e.Summary["outcome"] == outcome }
}

// unconfirmedEvents are the log's unconfirmed events of an outcome,
// oldest first.
func unconfirmedEvents(t *testing.T, ts *httptest.Server, outcome string) []event {
	t.Helper()
	return logLines(t, logStart(ts), unconfirmedOf(outcome))
}

// waitUnconfirmed waits for the log's first unconfirmed event of an
// outcome.
func waitUnconfirmed(t *testing.T, ts *httptest.Server, outcome string) event {
	t.Helper()
	ev, _ := logAfter(t, logStart(ts), "an unconfirmed event, "+outcome, unconfirmedOf(outcome))
	return ev
}

// openWrites is how many writes the runtime has open.
func openWrites(rt *runtime) int {
	rt.confirms.mu.Lock()
	defer rt.confirms.mu.Unlock()
	return len(rt.confirms.writes)
}

// dueNow makes the deadline of room's open write now and wakes the
// follow-up, which judges it at once: a test of what comes at a write's
// deadline gives its writes one it never reaches (confirmWithin of an
// hour) and sets the moment itself, once it has checked what holds while
// the write is pending ("A write's deadline", docs/testing.md). It waits
// for a write to the room whose deadline is still to come, such as a
// re-send's once the write before it is past its own.
func (r *rig) dueNow(room uint8) {
	r.t.Helper()
	c := &r.rt.confirms
	testkit.Eventually(r.t, fmt.Sprintf("a write to room %d before its deadline", room), func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		now, found := time.Now(), false
		for _, w := range c.writes {
			if w.room == room && !w.late && !w.byConfig {
				w.due, found = now, true
			}
		}
		return found
	})
	select {
	case r.rt.follow.wake <- struct{}{}:
	default: // a wake is pending already
	}
}

// A room command its members take is pending, then done once every
// member shows it, with no line: pending while the fake Cube holds its
// apply, done after the release.
func TestRoomCommandConfirmed(t *testing.T) {
	t.Parallel()
	r := startServer(t, confirming(testkit.Scale(3*time.Second), 0))
	ts, rt := r.ts, r.rt
	release := r.fc.HoldApply()
	t.Cleanup(release)
	code, out := do(t, ts, "PUT", confirmBase+"/rooms/2/setpoint", `{"mode":"manual","temp":22}`)
	if code != 200 || out["confirmation"] != "pending" {
		t.Fatalf("setpoint: %d %v", code, out)
	}
	release()
	testkit.Eventually(t, "room 2 confirmed", func() bool { return confirmRoom(t, ts, 2).Confirmation == nil && openWrites(rt) == 0 })
	if evs := unconfirmedEvents(t, ts, "not_confirmed"); len(evs) != 0 {
		t.Fatalf("lines %+v", evs)
	}
}

// A member that does not take a room command keeps the room pending,
// then not confirmed at the deadline: one warning names it, what it
// shows and its flags; once the command's re-send is not confirmed
// either (resend.go), the room and the device say so until it shows it,
// which a line at info says. The test sets each deadline (dueNow).
func TestRoomCommandNotConfirmed(t *testing.T) {
	t.Parallel()
	r := startServer(t, confirming(time.Hour, 0))
	fc, ts := r.fc, r.ts
	fc.SetDeaf(0x0a0205, true)
	on := true
	if err := fc.SetState(0x0a0205, fakecube.State{Error: &on}); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the error flag", func() bool {
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

	testkit.Eventually(t, "the members that hear showing it", func() bool {
		c := confirmRoom(t, ts, 2).Confirmation
		return c != nil && reflect.DeepEqual(c.Devices, []string{"0a0205"})
	})
	r.dueNow(2)
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
	r.dueNow(2) // the re-send's
	waitUnconfirmed(t, ts, "resend_not_confirmed")
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
	testkit.Eventually(t, "room 2 confirmed", func() bool { return confirmRoom(t, ts, 2).Confirmation == nil })
	if evs := unconfirmedEvents(t, ts, "not_confirmed"); len(evs) != 1 {
		t.Fatalf("%d warnings", len(evs))
	}
}

// A newer setpoint of a room replaces the one before it: one write open,
// one warning, for the newer, at its deadline, and no second one up to
// the report of its re-send (resend.go), not confirmed either, which the
// room then says. The test sets each deadline (dueNow).
func TestNewerWriteReplaces(t *testing.T) {
	t.Parallel()
	r := startServer(t, confirming(time.Hour, 0))
	fc, ts, rt := r.fc, r.ts, r.rt
	fc.SetDeaf(0x0a0205, true)
	for _, body := range []string{`{"mode":"manual","temp":22}`, `{"mode":"manual","temp":23}`} {
		if code, out := do(t, ts, "PUT", confirmBase+"/rooms/2/setpoint", body); code != 200 || out["confirmation"] != "pending" {
			t.Fatalf("setpoint %s: %d %v", body, code, out)
		}
	}
	if n := openWrites(rt); n != 1 {
		t.Fatalf("%d writes open", n)
	}
	r.dueNow(2)
	ev, next := logAfter(t, logStart(ts), "the warning", unconfirmedOf("not_confirmed"))
	if e := ev.Summary["expected"].(map[string]any); e["setpoint"] != float64(23) {
		t.Fatalf("expected %v", e)
	}
	r.dueNow(2) // the re-send's
	logAfter(t, next, "the re-send's report", unconfirmedOf("resend_not_confirmed"))
	if c := confirmRoom(t, ts, 2).Confirmation; c == nil || c.State != "unconfirmed" {
		t.Fatalf("room after the re-send's report: %+v", c)
	}
	// From the warning on, not from the report: the lines between them too.
	if more := logLines(t, next, unconfirmedOf("not_confirmed")); len(more) != 0 {
		t.Fatalf("%d warnings after the first: %+v", len(more), more)
	}
}

// A setpoint a client of the LAN proxy sent to a room is confirmed by
// the room's thermostats as the API's are: here none takes it.
func TestProxyWriteConfirmed(t *testing.T) {
	t.Parallel()
	r := startServer(t, confirming(300*time.Millisecond, 0))
	ts, rt := r.ts, r.rt
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
// display write is no C line's to follow: pending while the fake Cube
// holds its apply, confirmed after the release.
func TestLockAndDisplayConfirmed(t *testing.T) {
	t.Parallel()
	r := startServer(t, confirming(testkit.Scale(3*time.Second), 0))
	ts, rt := r.ts, r.rt
	release := r.fc.HoldApply()
	t.Cleanup(release)
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
	release()
	testkit.Eventually(t, "both confirmed", func() bool { return openWrites(rt) == 0 })
	if d := confirmDevice(t, ts, "0a0204"); d.Confirmation != nil || !d.State.PanelLocked {
		t.Fatalf("locked: %+v", d)
	}
}

// What a 0x40 write is confirmed by: the mode always, the setpoint in
// manual and vacation, a vacation's end.
func TestSetpointWant(t *testing.T) {
	t.Parallel()
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

// shortFollow re-reads a written C line three times within a fraction of
// a second.
var shortFollow = []time.Duration{50 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond}

// A configuration write the C line shows at once, as the writer re-reads
// it, is confirmed in the answer.
func TestConfigWriteShownAtOnce(t *testing.T) {
	t.Parallel()
	r := startServer(t, confirming(time.Minute, 0, shortFollow...))
	ts, rt := r.ts, r.rt
	if code, out := do(t, ts, "PUT", confirmBase+"/devices/0a0206/config", `{"eco_temp":16}`); code != 200 || out["confirmation"] != "confirmed" {
		t.Fatalf("config: %d %v", code, out)
	}
	if n := openWrites(rt); n != 0 {
		t.Fatalf("%d writes open", n)
	}
}

// A configuration write the C line shows is pending, then done: pending
// while the fake Cube holds its apply, and done at the first re-read
// after the release. The follow-up re-reads every 100 ms for a minute,
// so that a re-read comes after the release on any runner; it ends at
// the first that shows the write.
func TestConfigWriteConfirmed(t *testing.T) {
	t.Parallel()
	r := startServer(t, confirming(time.Minute, 0, slices.Repeat([]time.Duration{100 * time.Millisecond}, 600)...))
	ts, rt := r.ts, r.rt
	release := r.fc.HoldApply() // the device asleep: the write stays on its way
	t.Cleanup(release)
	code, out := do(t, ts, "PUT", confirmBase+"/devices/0a0204/config", `{"eco_temp":16.5,"boost_minutes":10}`)
	if code != 200 || out["confirmation"] != "pending" {
		t.Fatalf("config: %d %v", code, out)
	}
	release() // it wakes and takes it
	var lost []event
	testkit.Eventually(t, "the C line shows it", func() bool {
		lost = unconfirmedEvents(t, ts, "not_confirmed")
		return len(lost) > 0 || openWrites(rt) == 0
	})
	if len(lost) != 0 {
		t.Fatalf("not confirmed at a re-read after the release: %+v", lost)
	}
}

// A configuration write the device does not take is not confirmed after
// the follow-up's last re-read, the fields that differ named.
func TestConfigWriteNotConfirmed(t *testing.T) {
	t.Parallel()
	r := startServer(t, confirming(time.Minute, 0, shortFollow...))
	fc, ts := r.fc, r.ts
	fc.SetDeaf(0x0a0205, true)
	code, out := do(t, ts, "PUT", confirmBase+"/devices/0a0205/config", `{"eco_temp":16.5,"boost_minutes":10}`)
	if code != 200 || out["confirmation"] != "pending" {
		t.Fatalf("config: %d %v", code, out)
	}
	ev := waitUnconfirmed(t, ts, "not_confirmed")
	devices, _ := ev.Summary["devices"].([]any)
	if ev.Device != "0a0205" || ev.Summary["write"] != "config" || ev.Summary["target"] != "device:0a0205" || len(devices) != 1 {
		t.Fatalf("event %+v", ev)
	}
	d := devices[0].(map[string]any)
	shows, _ := d["shows"].(map[string]any)
	if !reflect.DeepEqual(d["fields"], []any{"boost_minutes", "eco_temp"}) || shows["eco_temp"] == 16.5 || shows["boost_minutes"] == float64(10) || shows["eco_temp"] == nil {
		t.Fatalf("device %v", d)
	}
	if c := confirmDevice(t, ts, "0a0205").Confirmation; c == nil || c.State != "unconfirmed" || !reflect.DeepEqual(c.Fields, []string{"boost_minutes", "eco_temp"}) {
		t.Fatalf("device: %+v", c)
	}
}

// A room's week programme is confirmed by each thermostat's C line: the
// one that does not take the day is named with it.
func TestProgramDayNotConfirmed(t *testing.T) {
	t.Parallel()
	r := startServer(t, confirming(time.Minute, 0, shortFollow...))
	fc, ts, rt := r.fc, r.ts, r.rt
	fc.SetDeaf(0x0a0205, true)
	body := `{"monday": [{"temp": 17, "until": "07:00"}, {"temp": 21, "until": "20:30"}, {"temp": 17, "until": "24:00"}]}`
	if code, out := do(t, ts, "PUT", confirmBase+"/rooms/2/program", body); code != 200 || out["confirmation"] != "pending" {
		t.Fatalf("program: %d %v", code, out)
	}
	ev := waitUnconfirmed(t, ts, "not_confirmed")
	devices, _ := ev.Summary["devices"].([]any)
	if ev.Device != "0a0205" || ev.Summary["write"] != "program" || ev.Summary["target"] != "room:2" || len(devices) != 1 ||
		!reflect.DeepEqual(devices[0].(map[string]any)["fields"], []any{"program.monday"}) {
		t.Fatalf("event %+v", ev)
	}
	testkit.Eventually(t, "the others confirmed", func() bool { return openWrites(rt) == 1 })
	if c := confirmRoom(t, ts, 2).Confirmation; c == nil || !reflect.DeepEqual(c.Devices, []string{"0a0205"}) || !reflect.DeepEqual(c.Fields, []string{"program.monday"}) {
		t.Fatalf("room: %+v", c)
	}
}

// A LAN proxy client's 0x11 to a room is compared with each thermostat's
// C line: here none takes it.
func TestProxyConfigWriteConfirmed(t *testing.T) {
	t.Parallel()
	r := startServer(t, confirming(time.Minute, 0, shortFollow...))
	ts, rt := r.ts, r.rt
	m, err := cube.CmdSetTemperatures(cube.Target{RFAddress: 0x0a0204, RoomID: 2, Room: true},
		cube.TemperatureSettings{Comfort: 22.5, Eco: 16.5, Max: 30.5, Min: 4.5, WindowOpen: 12, WindowOpenMinutes: 15})
	if err != nil {
		t.Fatal(err)
	}
	c, err := cube.ParseRadioCommand(m.Data)
	if err != nil {
		t.Fatal(err)
	}
	rt.proxyWrote(c) // as if the Cube had taken it
	evs, next := logAfterN(t, logStart(ts), "three warnings", 3, unconfirmedOf("not_confirmed"))
	if more := logLines(t, next, unconfirmedOf("not_confirmed")); len(more) != 0 {
		t.Fatalf("%d warnings more than three: %+v", len(more), more)
	}
	for _, ev := range evs {
		if ev.Summary["source"] != "proxy" || ev.Summary["write"] != "config" || ev.Summary["target"] != "room:2" || ev.Device == "" {
			t.Fatalf("event %+v", ev)
		}
	}
}

// What a C line shows of each field, and a newer write's field replaces
// an older one's.
func TestMissingConfig(t *testing.T) {
	t.Parallel()
	day := cube.DayProgram{{Temp: 17, Until: 420}, {Temp: 21, Until: 1230}, {Temp: 17, Until: 1440}}
	var week cube.WeekProgram
	week[cube.Monday] = day
	c := &cube.DeviceConfig{Thermostat: &cube.ThermostatConfig{EcoTemp: 16.5, BoostMinutes: 10, DecalcDay: cube.Saturday, WeekProgram: week}}
	want := map[string]any{"eco_temp": 16.5, "boost_minutes": 5, "decalc_day": cube.Saturday,
		"program.monday":  cube.DayProgramPart{Day: cube.Monday, Points: day, Last: true},
		"program.tuesday": cube.DayProgramPart{Day: cube.Tuesday, Points: day, Last: true}}
	if m := missingConfig(c, want); !reflect.DeepEqual(m, []string{"boost_minutes", "program.tuesday"}) {
		t.Errorf("missing %v", m)
	}
	if m := missingConfig(nil, map[string]any{"eco_temp": 16.5}); len(m) != 1 {
		t.Errorf("without a C line: %v", m)
	}
	wall := &cube.DeviceConfig{Wall: &cube.WallConfig{EcoTemp: 16.5}}
	if m := missingConfig(wall, map[string]any{"eco_temp": 16.5, "decalc_day": cube.Saturday}); !reflect.DeepEqual(m, []string{"decalc_day"}) {
		t.Errorf("a wall thermostat's %v", m)
	}

	var cs confirmations
	old := &confirmWrite{what: writeConfig, byConfig: true, want: map[string]any{"eco_temp": 16.5, "boost_minutes": 5},
		open: map[cube.RFAddress][]string{0x0a0204: {"boost_minutes", "eco_temp"}}}
	cs.writes = []*confirmWrite{old}
	cs.replaceLocked(&confirmWrite{what: writeConfig, byConfig: true, want: map[string]any{"eco_temp": 17}}, 0x0a0204)
	if len(cs.writes) != 1 || !reflect.DeepEqual(old.open[0x0a0204], []string{"boost_minutes"}) {
		t.Errorf("after a newer eco: %v", old.open)
	}
	cs.replaceLocked(&confirmWrite{what: writeConfig, byConfig: true, want: map[string]any{"boost_minutes": 10}}, 0x0a0204)
	if len(cs.writes) != 0 {
		t.Errorf("after a newer boost: %d writes", len(cs.writes))
	}
}
