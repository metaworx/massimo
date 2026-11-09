// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The S/N in the log and in the device routes (AP Logging v1.1,
// Block 1): every device event carries the device's serial, the Cube's
// own configuration is the Cube's, a removed device's event names it as
// it was, and a device is reached by its serial as by its radio address.

import (
	"net/http/httptest"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
)

// serialsOf maps the Cube's devices' radio addresses to their serials,
// as GET …/devices lists them.
func serialsOf(t *testing.T, ts *httptest.Server, token string) map[string]string {
	t.Helper()
	var devs []map[string]any
	if code := getJSON(t, ts, token, "/api/v1/cubes/KEQ0000100/devices", &devs); code != 200 || len(devs) == 0 {
		t.Fatalf("devices: %d %v", code, devs)
	}
	out := map[string]string{}
	for _, d := range devs {
		rf, _ := d["rf_address"].(string)
		sn, _ := d["serial"].(string)
		if rf == "" || sn == "" {
			t.Fatalf("a device without its radio address or serial: %v", d)
		}
		out[rf] = sn
	}
	return out
}

// Every device event of the greeting carries the device's serial and
// the source "device", a configuration also before the device has a
// state; the Cube's own C line is the Cube's, under its serial.
func TestLogEventsCarrySerial(t *testing.T) {
	_, ts, _, _ := startStored(t, Options{})
	serials := serialsOf(t, ts, "")
	var cubeRF string
	if v := waitConnected(t, ts, "KEQ0000100"); v != nil {
		cubeRF, _ = v["rf_address"].(string)
	}
	var evs []event
	eventually(t, "a config event per device and the Cube's own", func() bool {
		_, evs = logOf(t, ts, "?kind=config&limit=1000")
		return len(evs) >= len(serials)+1
	})
	_, states := logOf(t, ts, "?kind=state&limit=1000")
	own := 0
	for _, e := range append(evs, states...) {
		switch {
		case e.Device == "":
			t.Fatalf("a %s event without its device", e.Kind)
		case e.Source == "cube":
			own++
			if e.Kind != "config" || e.Serial != "KEQ0000100" || cubeRF != "" && e.Device != cubeRF {
				t.Fatalf("the Cube's own event: %+v, the Cube at %s", e, cubeRF)
			}
		case e.Source != "device" || e.Serial != serials[e.Device]:
			t.Fatalf("%s of %s: source %q serial %q, want device %q", e.Kind, e.Device, e.Source, e.Serial, serials[e.Device])
		}
	}
	if own == 0 {
		t.Fatal("no config event of the Cube itself")
	}
}

// A device removed by its serial is deleted as by its radio address, and
// its event names it as it was: its serial, type, name and room, though
// the snapshot no longer has it.
func TestLogForgottenDeviceHasSerial(t *testing.T) {
	fc, ts, _ := startCopyCube(t, nil)
	const base = "/api/v1/cubes/KEQ0000100"
	var dev map[string]any
	if code := getJSON(t, ts, "", base+"/devices/0a010a", &dev); code != 200 {
		t.Fatalf("the device: %d", code)
	}
	serial, _ := dev["serial"].(string)
	name, _ := dev["name"].(string)
	if serial == "" || name == "" {
		t.Fatalf("the device without serial or name: %v", dev)
	}
	sent := len(fc.Received())
	if code, out := do(t, ts, "DELETE", base+"/devices/"+strings.ToLower(serial), ""); code != 200 || out["ok"] != true {
		t.Fatalf("remove by serial: %d %v", code, out)
	}
	if got := lineStrings(fc.Received()[sent:]); len(got) == 0 || !containsLine(got, "t:01,1,CgEK") {
		t.Fatalf("the removal sent %v, want its t:", got)
	}
	var gone *event
	eventually(t, "device_forgotten in the log", func() bool {
		_, evs := logOf(t, ts, "?kind=device_forgotten")
		for i := range evs {
			if evs[i].Device == "0a010a" {
				gone = &evs[i]
			}
		}
		return gone != nil
	})
	if gone.Serial != serial || gone.Source != "device" {
		t.Fatalf("the event: serial %q source %q, want %q", gone.Serial, gone.Source, serial)
	}
	raw := rawLog(t, ts, "?kind=device_forgotten")
	st, _ := raw[len(raw)-1]["state"].(map[string]any)
	if st["serial"] != serial || st["name"] != name || st["type"] != dev["type"] || st["room_id"] != dev["room_id"] {
		t.Fatalf("the event's view %v, want the device as it was %v", st, dev)
	}
	if code := getJSON(t, ts, "", base+"/devices/"+serial, nil); code != 404 {
		t.Fatalf("GET the removed device by serial: %d", code)
	}
}

func containsLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

// Each device route takes the serial, in any case, as it takes the radio
// address: the same lines reach the Cube, the same device is read; a
// serial the Cube does not have is a 404, neither form a 400.
func TestDeviceRoutesTakeSerial(t *testing.T) {
	fc, ts, _ := startCopyCube(t, nil)
	const base = "/api/v1/cubes/KEQ0000100/devices/"
	serial := serialsOf(t, ts, "")["0a010a"]
	var byRF, bySN map[string]any
	getJSON(t, ts, "", base+"0a010a", &byRF)
	if code := getJSON(t, ts, "", base+serial, &bySN); code != 200 || bySN["rf_address"] != "0a010a" || bySN["serial"] != byRF["serial"] {
		t.Fatalf("GET by serial: %d %v", code, bySN)
	}
	for _, c := range []struct{ method, path, body string }{
		{"PUT", "/setpoint", `{"mode":"manual","temp":21.5}`},
		{"PUT", "/lock", `{"locked":false}`},
		{"PUT", "/config", `{"eco_temp":17}`},
	} {
		sent := len(fc.Received())
		if code, out := do(t, ts, c.method, base+"0a010a"+c.path, c.body); code != 200 {
			t.Fatalf("%s by radio address: %d %v", c.path, code, out)
		}
		want := lineStrings(fc.Received()[sent:])
		sent = len(fc.Received())
		if code, out := do(t, ts, c.method, base+strings.ToLower(serial)+c.path, c.body); code != 200 {
			t.Fatalf("%s by serial: %d %v", c.path, code, out)
		}
		got := lineStrings(fc.Received()[sent:])
		if len(want) == 0 || strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("%s by serial sent\n%s\nby radio address\n%s", c.path, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
	sent := len(fc.Received())
	if code, out := do(t, ts, "PUT", base+serial, `{"name":"By serial"}`); code != 200 {
		t.Fatalf("rename by serial: %d %v", code, out)
	}
	if got := lineStrings(fc.Received()[sent:]); len(got) == 0 || !strings.HasPrefix(got[len(got)-1], "m:") {
		t.Fatalf("the rename sent %v", got)
	}
	var renamed map[string]any
	if getJSON(t, ts, "", base+"0a010a", &renamed); renamed["name"] != "By serial" {
		t.Fatalf("after the rename: %v", renamed)
	}
	for _, path := range []string{"", "/setpoint", "/lock", "/config", "/program"} {
		method := "PUT"
		if path == "" {
			method = "GET"
		}
		if code, _ := do(t, ts, method, base+"MEQ0000999"+path, `{}`); code != 404 {
			t.Errorf("%s %s of a serial the Cube does not have: %d", method, path, code)
		}
		if code, _ := do(t, ts, method, base+"MEQ00009"+path, `{}`); code != 400 {
			t.Errorf("%s %s of neither form: %d", method, path, code)
		}
	}
}

// The log's device= takes the serial, in any case, as the radio address;
// a serial the Cube does not have answers an empty page.
func TestLogDeviceFilterBySerial(t *testing.T) {
	_, ts, _, _ := startStored(t, Options{})
	serials := serialsOf(t, ts, "")
	var byRF []event
	eventually(t, "the device's events", func() bool {
		_, byRF = logOf(t, ts, "?device=0a010d")
		return len(byRF) > 0
	})
	for _, q := range []string{serials["0a010d"], strings.ToLower(serials["0a010d"])} {
		code, bySN := logOf(t, ts, "?device="+q)
		if code != 200 || len(bySN) != len(byRF) {
			t.Fatalf("device=%s: %d, %d events, by radio address %d", q, code, len(bySN), len(byRF))
		}
		for _, e := range bySN {
			if e.Device != "0a010d" || e.Serial != serials["0a010d"] {
				t.Fatalf("device=%s answered %+v", q, e)
			}
		}
	}
	if code, evs := logOf(t, ts, "?device=MEQ0000999"); code != 200 || len(evs) != 0 {
		t.Fatalf("a serial the Cube does not have: %d %d", code, len(evs))
	}
}

// The audit names a device by its serial, whichever form the path held.
func TestAuditTargetBySerial(t *testing.T) {
	_, ts, _ := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	serial := serialsOf(t, ts, "secret")["0a010d"]
	callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a010d/lock", "secret", `{"locked":false}`)
	callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/"+strings.ToLower(serial)+"/lock", "secret", `{"locked":true}`)
	rows := auditRows(t, ts, "secret", "")
	n := 0
	for _, r := range rows {
		if r.Action != "device.lock" {
			continue
		}
		n++
		if r.Target != "device:"+serial || r.Result != store.ResultOK {
			t.Fatalf("row %+v, want the target device:%s", r, serial)
		}
	}
	if n != 2 {
		t.Fatalf("%d lock rows", n)
	}
}
