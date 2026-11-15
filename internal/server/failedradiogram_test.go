// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
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
	var buf syncBuffer
	fc, ts, _, _ := startStored(t, Options{PollInterval: 100 * time.Millisecond, Logger: slog.New(slog.NewJSONHandler(&buf, nil))})
	greeting, _ := fakecube.LoadGreeting(fixture)
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
	// stateLine is a state line of 0a010d whose change of key is want.
	stateLine := func(key, want string) *event {
		var evs []event
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log?kind=state&limit=500", &evs)
		for i := range evs {
			if evs[i].Device == "0a010d" && change(&evs[i], key) == want {
				return &evs[i]
			}
		}
		return nil
	}

	list(0x09, false)
	waitFor(t, "the report", func() bool { st := state("0a010d"); return st != nil && st["failed_radiogram"] == nil })
	list(0x03, true)
	waitFor(t, "the communication error", func() bool { return state("0a010d")["error"] == true })
	if got := failed(state("0a010d")); got != `{"code":3,"name":"TimeInformation"}` {
		t.Fatalf("failed_radiogram %s", got)
	}
	if st := state("0a0108"); st == nil || st["failed_radiogram"] != nil {
		t.Fatalf("0a0108 without the flag: %v", st)
	}
	var line *event
	waitFor(t, "the line that set the flag", func() bool { line = stateLine("error", `{"from":false,"to":true}`); return line != nil })
	if change(line, "failed_radiogram") != `{"from":null,"to":{"code":3,"name":"TimeInformation"}}` {
		t.Fatalf("the line's changes %v", line.Changes)
	}
	var logLine map[string]any
	waitFor(t, "the process log's line", func() bool {
		for _, l := range jsonLines(t, buf.String()) {
			if l["kind"] == "state" && l["device"] == "0a010d" && l["error"] == true {
				logLine = l
				return true
			}
		}
		return false
	})
	if logLine["failed_radiogram"] != "TimeInformation" || !strings.Contains(logLine["changed"].(string), "failed_radiogram") {
		t.Fatalf("the process log's line %v", logLine)
	}
	var alerts logAlertsView
	waitFor(t, "the flag's alert", func() bool {
		return getJSON(t, ts, "", "/api/v1/log/alerts", &alerts) == 200 && len(alerts.Alerts) == 1
	})
	if a := alerts.Alerts[0]; a.Flag != "error" || a.FailedRadiogram == nil || *a.FailedRadiogram != cube.RadiogramTimeInformation {
		t.Fatalf("the alert %+v", a)
	}

	// Another radiogram fails while the flag stays; its code is not in
	// the list.
	list(0xf6, true)
	waitFor(t, "the unknown code", func() bool { return failed(state("0a010d")) == `{"code":246}` })
	waitFor(t, "its line", func() bool {
		line = stateLine("failed_radiogram", `{"from":{"code":3,"name":"TimeInformation"},"to":{"code":246}}`)
		return line != nil
	})
	if _, ok := line.Changes["error"]; ok {
		t.Fatalf("the flag stayed, yet %v", line.Changes)
	}
	// The alert names the most severe of the run's radiograms, of two as
	// severe the oldest (D181): the time push still, then a setpoint's.
	alertNames := func(what string, want cube.RadiogramType) {
		t.Helper()
		var got *cube.RadiogramType
		waitFor(t, what, func() bool {
			if getJSON(t, ts, "", "/api/v1/log/alerts", &alerts) != 200 || len(alerts.Alerts) != 1 {
				return false
			}
			got = alerts.Alerts[0].FailedRadiogram
			return got != nil && *got == want
		})
	}
	alertNames("the time push, as severe as the unknown code and older", cube.RadiogramTimeInformation)
	list(0x40, true)
	alertNames("the setpoint's, more severe", cube.RadiogramControlMode)
	list(0xf6, true)
	waitFor(t, "the unknown code again", func() bool { return failed(state("0a010d")) == `{"code":246}` })
	alertNames("the setpoint's still", cube.RadiogramControlMode)

	// The flag cleared, byte 4 kept: no failed radiogram.
	list(0xf6, false)
	waitFor(t, "the flag cleared", func() bool { return state("0a010d")["error"] == false })
	if st := state("0a010d"); st["failed_radiogram"] != nil {
		t.Fatalf("cleared, yet %v", st)
	}
	waitFor(t, "the line that cleared it", func() bool {
		line = stateLine("error", `{"from":true,"to":false}`)
		return line != nil
	})
	if change(line, "failed_radiogram") != `{"from":{"code":246},"to":null}` {
		t.Fatalf("the clearing line's changes %v", line.Changes)
	}
}

// A virtual wall thermostat's L entry for the proxy's clients stays as
// it was: the wall thermostat's communication error and failed
// radiogram are not in it.
func TestVirtualEntryLeavesTheCommunicationErrorOut(t *testing.T) {
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
