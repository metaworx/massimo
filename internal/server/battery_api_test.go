// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/battery"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// replacedBody is the one body of new batteries marked.
const replacedBody = `{"replaced": true}`

// flagGone matches the state line of device whose battery flag is off.
func flagGone(device string) func(event) bool {
	return func(e event) bool {
		dv, _ := e.State.(map[string]any)
		st, _ := dv["state"].(map[string]any)
		return e.Kind == "state" && e.Device == device && st != nil && st["battery_low"] == false
	}
}

// answerOf reads a JSON answer.
func answerOf(t *testing.T, text string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("answer %q: %v", text, err)
	}
	return m
}

// New batteries marked (AP RefreshBatteries v1.0, Block 3): refused
// without configure, for another body, a virtual wall thermostat, the
// Cube, a device that is not there, one without a current report and
// one whose flag is on; once the flag is off, a weak device is ok at
// once, its battery event in the log with the cause and who marked it,
// and its change line; marked again, nothing changes; a restart keeps
// it ok, the rebuild replaying the mark; and a flag after the mark makes
// a rebuilt tracker weak, the mark replayed in its time's place.
func TestNewBatteries(t *testing.T) {
	t.Parallel()
	r := startServer(t, stored(), withOptions(Options{PollInterval: 100 * time.Millisecond}))
	fc, ts, st := r.fc, r.ts, r.st
	operator := userToken(t, st, "operator1", store.RoleOperator)
	viewer := userToken(t, st, "viewer1", store.RoleViewer)
	devices := "/api/v1/cubes/KEQ0000100/devices/"
	path := devices + "0a010d/battery"
	levelOf := func(dev string) string {
		var d deviceDetail
		if code := getJSON(t, r.ts, operator, devices+dev, &d); code != 200 {
			t.Fatalf("GET %s: %d", dev, code)
		}
		return d.Battery
	}

	// 0a010d has no state in the fixture.
	if code, text := callAPI(t, ts, "POST", path, operator, replacedBody); code != 409 || !strings.Contains(errorOf(text), "no current report of device 0a010d") ||
		refusalIn(t, text).Code != "battery_no_report" || !reflect.DeepEqual(refusalIn(t, text).Params, map[string]any{"device": "0a010d"}) {
		t.Fatalf("without a report: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "POST", path, viewer, replacedBody); code != 403 || errorOf(text) != `capability "configure" required` {
		t.Fatalf("as viewer: %d %s", code, text)
	}
	for _, body := range []string{"", "{}", "null", "[true]", `{"replaced": false}`, `{"replaced": "true"}`, `{"replaced": 1}`,
		`{"Replaced": true}`, `{"replaced": true, "at": "now"}`, `{"replaced": true} {}`, `{"replaced": true}}`} {
		if code, text := callAPI(t, ts, "POST", path, operator, body); code != 400 || errorOf(text) != `body must be {"replaced": true}` {
			t.Errorf("body %q: %d %s", body, code, text)
		}
	}
	for _, c := range []struct {
		device string
		code   int
		error  string
		refuse string // the code, "" for none
	}{
		{"room-1", 400, "the virtual wall thermostat of room 1 has no batteries", ""},
		{"VWT0000001", 400, "the virtual wall thermostat of room 1 has no batteries", ""},
		{"room-9", 404, "no room 9", "room_unknown"},
		{"0a0107", 400, "device 0a0107 is the Cube, which has no batteries", ""},
		{"0a0fff", 404, "no device 0a0fff", "device_unknown"},
		{"MEQ0000999", 404, "no device MEQ0000999", "device_unknown"},
		{"xyz", 400, `bad device "xyz": a radio address of six hex digits, or a serial number`, ""},
	} {
		if code, text := callAPI(t, ts, "POST", devices+c.device+"/battery", operator, replacedBody); code != c.code || errorOf(text) != c.error || refusalIn(t, text).Code != c.refuse {
			t.Errorf("%s: %d %s, want %d %q %s", c.device, code, text, c.code, c.error, c.refuse)
		}
	}

	on, off := true, false
	if err := fc.SetState(0x0a010d, fakecube.State{Reported: &on, BatteryLow: &on}); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the weak level", func() bool { return levelOf("0a010d") == "weak" })
	if code, text := callAPI(t, ts, "POST", path, operator, replacedBody); code != 409 ||
		errorOf(text) != "device 0a010d still reports its batteries low: wait for its next report" || refusalIn(t, text).Code != "battery_still_low" {
		t.Fatalf("with the flag on: %d %s", code, text)
	}
	m := logMarkAs(t, ts, operator)
	if err := fc.SetState(0x0a010d, fakecube.State{BatteryLow: &off}); err != nil {
		t.Fatal(err)
	}
	_, m = logAfter(t, m, "the flag gone", flagGone("0a010d"))
	if l := levelOf("0a010d"); l != "weak" {
		t.Fatalf("the flag gone: %s, want weak until the mark", l)
	}

	// By its serial, as the other device routes take it.
	code, text := callAPI(t, ts, "POST", devices+"MEQ0000106/battery", operator, replacedBody)
	if want := map[string]any{"battery": "ok", "from": "weak", "changed": true}; code != 200 || !reflect.DeepEqual(answerOf(t, text), want) {
		t.Fatalf("marked: %d %s, want %v", code, text, want)
	}
	if l := levelOf("0a010d"); l != "ok" {
		t.Fatalf("after the mark: %s", l)
	}
	ev, _ := logAfter(t, m, "the battery event", logLine("battery", "0a010d"))
	if s := ev.Summary; s["level"] != "ok" || s["from"] != "weak" || s["cause"] != "replaced" || ev.Level != "info" {
		t.Errorf("the battery event: %s %v", ev.Level, s)
	}
	if ev.By == nil || ev.By.Name != "operator1" || ev.Serial != "MEQ0000106" || ev.Command == 0 {
		t.Errorf("the battery event's user, serial and command: %+v %q %d", ev.By, ev.Serial, ev.Command)
	}
	ch, _ := logAfter(t, m, "the change line", logLine("change", ""))
	if ch.Summary["action"] != "device.battery" || ch.Summary["target"] != "device:MEQ0000106" || ch.By == nil || ch.By.Name != "operator1" || ch.Command != ev.Command {
		t.Errorf("the change line: %v %+v %d", ch.Summary, ch.By, ch.Command)
	}

	m = logMarkAs(t, ts, operator)
	code, text = callAPI(t, ts, "POST", path, operator, replacedBody)
	if want := map[string]any{"battery": "ok", "from": "ok", "changed": false}; code != 200 || !reflect.DeepEqual(answerOf(t, text), want) {
		t.Fatalf("marked again: %d %s, want %v", code, text, want)
	}
	logAfter(t, m, "the second change line", logLine("change", ""))
	if evs := logLines(t, m, logLine("battery", "")); len(evs) != 0 {
		t.Errorf("marked again: battery events %+v", evs)
	}

	// A restart: without the mark replayed, the flag's history alone
	// would make the device weak again.
	r.stop()
	r = startServer(t, again(r), withOptions(Options{PollInterval: 100 * time.Millisecond}))
	if l := r.rt.battery.Level("0a010d"); l != battery.OK {
		t.Fatalf("after the restart: %s", l)
	}
	if l := levelOf("0a010d"); l != "ok" {
		t.Fatalf("after the restart, the API: %s", l)
	}

	// The mark replays in its time's place: a flag after it makes the
	// device weak again, which a tracker rebuilt keeps.
	m = logMarkAs(t, r.ts, operator)
	if err := r.fc.SetState(0x0a010d, fakecube.State{BatteryLow: &on}); err != nil {
		t.Fatal(err)
	}
	_, m = logAfter(t, m, "the flag again", logLine("battery", "0a010d"))
	if err := r.fc.SetState(0x0a010d, fakecube.State{BatteryLow: &off}); err != nil {
		t.Fatal(err)
	}
	logAfter(t, m, "the flag gone again", flagGone("0a010d"))
	if err := r.st.FlushEvents(t.Context()); err != nil {
		t.Fatal(err)
	}
	rebuilt := r.s.newRuntime(r.rt.cfg, "")
	r.s.rebuildBattery(t.Context(), rebuilt)
	if l := rebuilt.battery.Level("0a010d"); l != battery.Weak {
		t.Fatalf("rebuilt after a flag that followed the mark: %s", l)
	}
}

// New batteries marked make the level ok, which drops a battery_weak
// still waiting its 15 minutes; another device's goes out.
func TestNewBatteriesDropTheWaitingNotification(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(time.Now())
	opts := storeOptions(t, t.TempDir(), "secret")
	var logs *testkit.Watch[testkit.Record]
	opts.Logger, logs = testkit.LogWatch(nil)
	opts.PollInterval = 100 * time.Millisecond
	opts.notifyClock = kitClock{clock}
	rg := startServer(t, withOptions(opts))
	fc, ts := rg.fc, rg.ts
	if code, text := callAPI(t, ts, "POST", "/api/v1/me/push", "secret", subscription(t, "https://push.example.net/send/a", "phone")); code != 201 {
		t.Fatalf("subscribe: %d %s", code, text)
	}
	testkit.Eventually(t, "the rules' scan", func() bool { return clock.Pending() == 1 })

	on, off := true, false
	for _, rf := range []cube.RFAddress{0x0a010d, 0x0a010a} {
		if err := fc.SetState(rf, fakecube.State{Reported: &on, BatteryLow: &on}); err != nil {
			t.Fatal(err)
		}
	}
	testkit.Eventually(t, "the two battery_weak waiting", func() bool { return clock.Pending() == 3 })
	at := logs.Mark()
	if err := fc.SetState(0x0a010d, fakecube.State{BatteryLow: &off}); err != nil {
		t.Fatal(err)
	}
	// The pump writes the state line once the tracker has read it.
	logs.Await(t, at, "the flag gone", func(r testkit.Record) bool {
		_, low := r.Attrs["battery_low"]
		return r.Msg == "device state" && r.Attrs["device"] == "0a010d" && !low
	})
	if code, text := callAPI(t, ts, "POST", "/api/v1/cubes/KEQ0000100/devices/0a010d/battery", "secret", replacedBody); code != 200 {
		t.Fatalf("the mark: %d %s", code, text)
	}
	testkit.Eventually(t, "0a010d's battery_weak dropped", func() bool { return clock.Pending() == 2 })
	clock.Add(notify.BatteryWeakDelay)
	logs.Await(t, at, "0a010a's battery_weak", testkit.Msg("notification", "kind", "battery_weak", "device", "0a010a"))
	if got := logs.Since(0, testkit.Msg("notification", "kind", "battery_weak", "device", "0a010d")); len(got) != 0 {
		t.Fatalf("0a010d's battery_weak after its new batteries: %v", got)
	}
}
