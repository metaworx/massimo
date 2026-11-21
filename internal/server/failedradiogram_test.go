// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/view"
)

// A device's communication error names the radiogram that failed (AP
// RadioHealth v1.2, Block 1): the device's state gives failed_radiogram
// with the flag alone; the state line that set the flag has it in its
// changes, in the store and in the process log; the flag's alert has it,
// and of the radiograms that failed while the flag stayed, the most
// severe (D181); a code the list lacks stays a number; the flag cleared
// takes it away.
func TestFailedRadiogram(t *testing.T) {
	t.Parallel()
	log, logs := testkit.LogWatch(nil)
	rg := startServer(t, stored(), withOptions(Options{PollInterval: 100 * time.Millisecond, Logger: log}))
	fc, ts, s := rg.fc, rg.ts, rg.s
	greeting := recorded(t)
	clean := greeting[len(greeting)-1]
	list := func(b4 byte, flagged bool) {
		raw, _ := base64.StdEncoding.DecodeString(clean.Data)
		f1 := byte(0x12)
		if flagged {
			f1 |= 0x08
		}
		// 0a010d: valid, manual; 0a0108: byte 4 0x21 without the flag,
		// as an r: leaves it.
		copy(raw, []byte{0x0b, 0x0a, 0x01, 0x0d, b4, f1, 0x01, 30, 42, 0, 0, 0})
		copy(raw[12:], []byte{0x0b, 0x0a, 0x01, 0x08, 0x21, 0x12, 0x01, 30, 42, 0, 0, 0})
		fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	}
	state := func(rf string) map[string]any {
		var d map[string]any
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/"+rf, &d)
		st, _ := d["state"].(map[string]any)
		return st
	}
	failed := func(st map[string]any) string {
		raw, _ := json.Marshal(st["failed_radiogram"])
		return string(raw)
	}
	change := func(ev *event, key string) string {
		raw, _ := json.Marshal(ev.Changes[key])
		return string(raw)
	}
	// stateLines are the state lines of 0a010d, oldest first.
	stateLines := func() []event {
		var evs []event
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log?kind=state&device=0a010d", &evs)
		return evs
	}
	// changed matches a state line of 0a010d whose change of key is want.
	changed := func(key, want string) func(event) bool {
		return func(e event) bool { return logLine("state", "0a010d")(e) && change(&e, key) == want }
	}
	// shows reports whether a device's state has the flag as flagged
	// and the failed radiogram as want.
	shows := func(st map[string]any, flagged bool, want string) bool {
		return st != nil && st["error"] == flagged && failed(st) == want
	}
	// caughtUp waits until 0a010d's state shows the list set last - the
	// flag as flagged, the failed radiogram as want - and the pump has
	// handled it, then checks that the newest state line of 0a010d shows
	// it too. The device's state and the alert's radiogram of now are the
	// snapshot's, but a line is the pump's: it reads the snapshot when it
	// takes the event, which a loaded host can delay past the test's next
	// list. That list's change then comes in the line of the one before,
	// measured from the one before that, and a line the test waits for or
	// the alert reads is never written. So before it sets the next list,
	// the test waits for the pump (pumpSync).
	caughtUp := func(what string, flagged bool, want string) {
		t.Helper()
		testkit.Eventually(t, what+" in the device's state", func() bool { return shows(state("0a010d"), flagged, want) })
		pumpSync(t, s)
		evs := stateLines()
		if len(evs) == 0 {
			t.Fatalf("%s: no state line of 0a010d", what)
		}
		d, _ := evs[len(evs)-1].State.(map[string]any)
		if st, _ := d["state"].(map[string]any); !shows(st, flagged, want) {
			t.Fatalf("%s: the newest state line of 0a010d shows %v", what, st)
		}
	}

	list(0x09, false)
	testkit.Eventually(t, "the report", func() bool { st := state("0a010d"); return st != nil && st["failed_radiogram"] == nil })
	caughtUp("the report's line", false, "null")
	flagged, m := logs.Mark(), logMark(t, ts)
	list(0x03, true)
	testkit.Eventually(t, "the communication error", func() bool { return state("0a010d")["error"] == true })
	if got := failed(state("0a010d")); got != `{"code":3,"name":"TimeInformation"}` {
		t.Fatalf("failed_radiogram %s", got)
	}
	if st := state("0a0108"); st == nil || st["failed_radiogram"] != nil {
		t.Fatalf("0a0108 without the flag: %v", st)
	}
	line, _ := logAfter(t, m, "the line that set the flag", changed("error", `{"from":false,"to":true}`))
	if change(&line, "failed_radiogram") != `{"from":null,"to":{"code":3,"name":"TimeInformation"}}` {
		t.Fatalf("the line's changes %v", line.Changes)
	}
	printed, _ := logs.Await(t, flagged, "the process log's line", func(l testkit.Record) bool {
		return l.Attrs["kind"] == "state" && l.Attrs["device"] == "0a010d" && l.Attrs["error"] == "true"
	})
	if printed.Attrs["failed_radiogram"] != "TimeInformation" || !strings.Contains(printed.Attrs["changed"], "failed_radiogram") {
		t.Fatalf("the process log's line %v", printed)
	}
	var alerts logAlertsView
	testkit.Eventually(t, "the flag's alert", func() bool {
		return getJSON(t, ts, "", "/api/v1/log/alerts", &alerts) == 200 && len(alerts.Alerts) == 1
	})
	if a := alerts.Alerts[0]; a.Flag != "error" || a.FailedRadiogram == nil || *a.FailedRadiogram != cube.RadiogramTimeInformation {
		t.Fatalf("the alert %+v", a)
	}

	// Another radiogram fails while the flag stays; its code is not in
	// the list.
	m = logMark(t, ts)
	list(0xf6, true)
	testkit.Eventually(t, "the unknown code", func() bool { return failed(state("0a010d")) == `{"code":246}` })
	line, _ = logAfter(t, m, "its line", changed("failed_radiogram", `{"from":{"code":3,"name":"TimeInformation"},"to":{"code":246}}`))
	if _, ok := line.Changes["error"]; ok {
		t.Fatalf("the flag stayed, yet %v", line.Changes)
	}
	// The alert names the most severe of the run's radiograms, of two as
	// severe the oldest (D181): the time push still, then a setpoint's.
	alertNames := func(what string, want cube.RadiogramType) {
		t.Helper()
		var got *cube.RadiogramType
		testkit.Eventually(t, what, func() bool {
			if getJSON(t, ts, "", "/api/v1/log/alerts", &alerts) != 200 || len(alerts.Alerts) != 1 {
				return false
			}
			got = alerts.Alerts[0].FailedRadiogram
			return got != nil && *got == want
		})
	}
	alertNames("the time push, as severe as the unknown code and older", cube.RadiogramTimeInformation)
	list(0x40, true)
	caughtUp("the setpoint's line", true, `{"code":64,"name":"ControlMode"}`)
	alertNames("the setpoint's, more severe", cube.RadiogramControlMode)
	list(0xf6, true)
	testkit.Eventually(t, "the unknown code again", func() bool { return failed(state("0a010d")) == `{"code":246}` })
	caughtUp("its line again", true, `{"code":246}`)
	alertNames("the setpoint's still", cube.RadiogramControlMode)

	// The flag cleared, byte 4 kept: no failed radiogram.
	m = logMark(t, ts)
	list(0xf6, false)
	testkit.Eventually(t, "the flag cleared", func() bool { return state("0a010d")["error"] == false })
	if st := state("0a010d"); st["failed_radiogram"] != nil {
		t.Fatalf("cleared, yet %v", st)
	}
	line, _ = logAfter(t, m, "the line that cleared it", changed("error", `{"from":true,"to":false}`))
	if change(&line, "failed_radiogram") != `{"from":{"code":246},"to":null}` {
		t.Fatalf("the clearing line's changes %v", line.Changes)
	}
}

// A virtual wall thermostat's L entry for the proxy's clients stays as
// it was: the wall thermostat's communication error and failed
// radiogram are not in it.
func TestVirtualEntryLeavesTheCommunicationErrorOut(t *testing.T) {
	t.Parallel()
	sp, at := 21.0, 20.5
	failed := cube.RadiogramTimeInformation
	d := view.Device{RFAddress: "0a0101", Type: cube.DeviceWallThermostat, Reported: true,
		State: &view.State{Mode: cube.ModeAuto, Setpoint: &sp, ActualTemp: &at}}
	before := virtualEntry(0x0a0101, d, nil)
	d.State.Error, d.State.FailedRadiogram = true, &failed
	if after := virtualEntry(0x0a0101, d, nil); string(after) != string(before) || after[4] != 0 || after[5]&0x08 != 0 {
		t.Fatalf("% x, want % x", after, before)
	}
}
