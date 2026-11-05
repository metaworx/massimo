// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// withEco returns rf's C line from the greeting with another eco
// temperature, as the Cube reports it once the device took the write.
func withEco(t *testing.T, greeting []cube.Message, rf cube.RFAddress, eco float64) cube.Message {
	t.Helper()
	for _, g := range greeting {
		if g.Type == 'C' && strings.HasPrefix(g.Data, rf.String()+",") {
			raw, err := base64.StdEncoding.DecodeString(g.Data[7:])
			if err != nil {
				t.Fatal(err)
			}
			raw[0x13] = byte(eco * 2)
			return cube.Message{Type: 'C', Data: rf.String() + "," + base64.StdEncoding.EncodeToString(raw)}
		}
	}
	t.Fatalf("no C line for %s", rf)
	return cube.Message{}
}

func TestFollowUpAfterAWrite(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0537249", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond},
		RefreshInterval: time.Hour, FollowUpDelays: []time.Duration{40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond},
		FollowUpReconnectGap: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249", &c) == 200 && c.Connected
	})
	const base = "/api/v1/cubes/KEQ0537249"
	eco := func() any {
		_, d := do(t, ts, "GET", base+"/devices/14d08e", "")
		th, _ := d["thermostat"].(map[string]any)
		return th["eco_temp"]
	}
	if e := eco(); e != float64(15) {
		t.Fatalf("eco before %v", e)
	}
	if code, out := do(t, ts, "PUT", base+"/devices/14d08e/config", `{"eco_temp":17}`); code != 200 {
		t.Fatalf("write %d %v", code, out)
	}
	// The immediate re-read still sees the old line; the Cube changes it
	// a little later, and the follow-up picks that up.
	if e := eco(); e != float64(15) {
		t.Fatalf("eco right after the write %v", e)
	}
	fc.SetConfig(withEco(t, greeting, 0x14d08e, 17))
	eventually(t, "the new eco temperature", func() bool { return eco() == float64(17) })
	// The fake's S reply left one radiogram pending (49 of 50 slots
	// free), so the follow-up reconnects for a fresh H line.
	eventually(t, "the budget reconnect", func() bool { return fc.Connections() == 2 })
	var evs []event
	getJSON(t, ts, "", base+"/log", &evs)
	config := false
	for _, e := range evs {
		config = config || (e.Kind == "config" && e.Device == "14d08e")
	}
	if !config {
		t.Fatal("no config event for the device")
	}
	// The re-reads stop once the line changed: no c: for 14d08e after it.
	time.Sleep(200 * time.Millisecond)
	var cs int
	for _, m := range fc.Received() {
		if m.String() == "c:14d08e" {
			cs++
		}
	}
	if cs > 4 {
		t.Fatalf("%d re-reads", cs)
	}
}

func TestFollowUpGivesUpAndRespectsTheRefreshSwitch(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	// No RefreshInterval: no reconnects for the budget.
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0537249", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond},
		FollowUpDelays: []time.Duration{20 * time.Millisecond, 20 * time.Millisecond}, FollowUpReconnectGap: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249", &c) == 200 && c.Connected
	})
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/devices/14d08e/config", `{"eco_temp":17}`); code != 200 {
		t.Fatal(code)
	}
	time.Sleep(300 * time.Millisecond)
	var cs int
	for _, m := range fc.Received() {
		if m.String() == "c:14d08e" {
			cs++
		}
	}
	// One re-read right after the write, then one per follow-up delay.
	if cs != 3 || fc.Connections() != 1 {
		t.Fatalf("%d re-reads, %d connections", cs, fc.Connections())
	}
}

// eventually polls cond until it holds or five seconds passed.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}
