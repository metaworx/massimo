// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/base64"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// A state change the Cube reports in the answer to a later l: poll is a
// state event in the log, after the connected event.
func TestPolledStateChangesReachTheLog(t *testing.T) {
	t.Parallel()
	// The start has the greeting through the pump: a pump still at the
	// greeting when the list changes reads the new state for the
	// greeting's line, and the change has none of its own.
	r := startServer(t, withOptions(Options{PollInterval: 100 * time.Millisecond}))
	fc, ts, greeting := r.fc, r.ts, recorded(t)
	// 0a010d reports: manual, 21 °C, valve 30 %.
	l := greeting[len(greeting)-1]
	raw, _ := base64.StdEncoding.DecodeString(l.Data)
	copy(raw, []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12, 0x01, 30, 42, 0, 0, 0})
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	// stateAfter is the log's state events of 0a010d after its connected
	// event, oldest first: the ring, which has no ids, in its order.
	stateAfter := func() []event {
		var evs, out []event
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log", &evs)
		connected := false
		for _, e := range evs {
			if e.Kind == "connected" {
				connected = true
			}
			if connected && e.Kind == "state" && e.Device == "0a010d" {
				out = append(out, e)
			}
		}
		return out
	}
	var changed event
	testkit.Eventually(t, "a state event after connected", func() bool {
		evs := stateAfter()
		if len(evs) > 0 {
			changed = evs[0]
		}
		return len(evs) > 0
	})
	// The device had not reported before: the event says so, and what it
	// reports now.
	c := changed.Changes
	if c["reported"].From != false || c["reported"].To != true || c["mode"].To != "manual" || c["setpoint"].To != 21.0 || c["valve_pct"].To != 30.0 {
		t.Fatalf("changes %+v", c)
	}
	// The first state events, at the connect, have no changes.
	var evs []event
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log", &evs)
	for _, e := range evs {
		if e.Kind == "connected" {
			break
		}
		if e.Kind == "state" && e.Changes != nil {
			t.Fatalf("first state event with changes %+v", e)
		}
	}
	// A later change of one value is an event with that value only.
	raw[7] = 50
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	var next []event
	testkit.Eventually(t, "the valve change", func() bool {
		next = stateAfter()
		return len(next) > 1
	})
	if c := next[1].Changes; len(c) != 1 || c["valve_pct"].From != 30.0 || c["valve_pct"].To != 50.0 {
		t.Fatalf("the valve change's event: changes %+v, want valve_pct 30 to 50 alone", c)
	}
}
