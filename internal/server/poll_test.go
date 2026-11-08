// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/base64"
	"net/http/httptest"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// A state change the Cube reports in the answer to a later l: poll is a
// state event in the log, after the connected event.
func TestPolledStateChangesReachTheLog(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, PollInterval: 100 * time.Millisecond, Queue: engine.Options{Gap: time.Millisecond}})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})
	// 0a010d reports: manual, 21 °C, valve 30 %.
	l := greeting[len(greeting)-1]
	raw, _ := base64.StdEncoding.DecodeString(l.Data)
	copy(raw, []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12, 0x01, 30, 42, 0, 0, 0})
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	var changed event
	eventually(t, "a state event after connected", func() bool {
		var evs []event
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log", &evs)
		connected := false
		for _, e := range evs {
			if e.Kind == "connected" {
				connected = true
			}
			if connected && e.Kind == "state" && e.Device == "0a010d" {
				changed = e
				return true
			}
		}
		return false
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
	eventually(t, "the valve change", func() bool {
		var evs []event
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log", &evs)
		last := evs[len(evs)-1]
		return last.Kind == "state" && len(last.Changes) == 1 && last.Changes["valve_pct"].From == 30.0 && last.Changes["valve_pct"].To == 50.0
	})
}
