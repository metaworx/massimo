// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/base64"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
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
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond},
		RefreshInterval: time.Hour, FollowUpDelays: []time.Duration{40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond},
		FollowUpReconnectGap: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	testkit.Eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})
	const base = "/api/v1/cubes/KEQ0000100"
	eco := func() any {
		_, d := do(t, ts, "GET", base+"/devices/0a010a", "")
		th, _ := d["thermostat"].(map[string]any)
		return th["eco_temp"]
	}
	if e := eco(); e != float64(15) {
		t.Fatalf("eco before %v", e)
	}
	if code, out := do(t, ts, "PUT", base+"/devices/0a010a/config", `{"eco_temp":17}`); code != 200 {
		t.Fatalf("write %d %v", code, out)
	}
	// The immediate re-read still sees the old line; the Cube changes it
	// a little later, and the follow-up picks that up.
	if e := eco(); e != float64(15) {
		t.Fatalf("eco right after the write %v", e)
	}
	fc.SetConfig(withEco(t, greeting, 0x0a010a, 17))
	testkit.Eventually(t, "the new eco temperature", func() bool { return eco() == float64(17) })
	// The fake's S reply left one radiogram pending (49 of 50 slots
	// free), so the follow-up reconnects for a fresh H line.
	testkit.Eventually(t, "the budget reconnect", func() bool { return fc.Connections() == 2 })
	var evs []event
	getJSON(t, ts, "", base+"/log", &evs)
	config := false
	for _, e := range evs {
		config = config || (e.Kind == "config" && e.Device == "0a010a")
	}
	if !config {
		t.Fatal("no config event for the device")
	}
	// The re-reads stop once the line changed: no c: for 0a010a after it.
	time.Sleep(200 * time.Millisecond)
	if cs := cubewatch.Lines(fc).Count(0, lineIs("c:0a010a")); cs > 4 {
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
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond},
		FollowUpDelays: []time.Duration{20 * time.Millisecond, 20 * time.Millisecond}, FollowUpReconnectGap: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	testkit.Eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a010a/config", `{"eco_temp":17}`); code != 200 {
		t.Fatal(code)
	}
	time.Sleep(300 * time.Millisecond)
	// One re-read right after the write, then one per follow-up delay.
	if cs := cubewatch.Lines(fc).Count(0, lineIs("c:0a010a")); cs != 3 || fc.Connections() != 1 {
		t.Fatalf("%d re-reads, %d connections", cs, fc.Connections())
	}
}

// proxyClient connects to a proxy and reads its greeting.
func proxyClient(t *testing.T, addr string) (net.Conn, *cube.Reader) {
	t.Helper()
	var conn net.Conn
	testkit.Eventually(t, "the proxy", func() bool {
		var err error
		conn, err = net.DialTimeout("tcp", addr, time.Second)
		return err == nil
	})
	t.Cleanup(func() { conn.Close() })
	conn.SetDeadline(time.Now().Add(testkit.Scale(10 * time.Second)))
	r := cube.NewReader(conn)
	for {
		m, err := r.Read()
		if err != nil {
			t.Fatal(err)
		}
		if m.Type == 'L' {
			return conn, r
		}
	}
}

// proxyWrite sends one s: line through a proxy client and reads the S
// reply.
func proxyWrite(t *testing.T, conn net.Conn, r *cube.Reader, data string) {
	t.Helper()
	conn.Write([]byte("s:" + data + "\r\n"))
	// The reply is the next line: the original software reads it so,
	// and anything the proxy sent unasked would be taken for it.
	if m, err := r.Read(); err != nil || m.Type != 'S' {
		t.Fatalf("s:%s answered %v %v", data, m, err)
	}
}

// A write that a client of the LAN proxy sends is followed up as the
// API's own: a configuration write has the device's C line re-read until
// it shows what was written, then the budget reconnect; a setpoint gets
// the budget reconnect only.
func TestFollowUpAfterAProxyClientsWrite(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxyAddr := ln.Addr().String()
	ln.Close()
	log, logs := testkit.LogWatch(nil)
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond},
		RefreshInterval: time.Hour, FollowUpDelays: []time.Duration{40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond},
		FollowUpReconnectGap: time.Millisecond, Proxy: []ProxyListener{{Addr: proxyAddr}},
		Logger: log})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	testkit.Eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})
	received := cubewatch.Lines(fc)
	sent := func() (rereads, others int) {
		return received.Count(0, lineIs("c:0a010a")), received.Count(0, func(m cube.Message) bool { return m.Type == 'c' && m.String() != "c:0a010a" })
	}

	// A 0x11 (preset temperatures) to 0a010a, as the original software
	// sends it, with eco 17 and the device's other presets.
	changed := withEco(t, greeting, 0x0a010a, 17)
	cfg, err := cube.ParseConfig(changed.Data)
	if err != nil {
		t.Fatal(err)
	}
	th := cfg.Thermostat
	m, err := cube.CmdSetTemperatures(cube.Target{RFAddress: 0x0a010a}, cube.TemperatureSettings{Comfort: th.ComfortTemp, Eco: th.EcoTemp,
		Max: th.MaxSetpoint, Min: th.MinSetpoint, Offset: th.TempOffset, WindowOpen: th.WindowOpenTemp, WindowOpenMinutes: th.WindowOpenMins})
	if err != nil {
		t.Fatal(err)
	}
	conn, r := proxyClient(t, proxyAddr)
	proxyWrite(t, conn, r, m.Data)
	received.Await(t, 0, "a re-read", lineIs("c:0a010a"))
	took := logs.Mark()
	fc.SetConfig(changed)
	logs.Await(t, took, "the device's new configuration", testkit.Msg("device configuration taken over by the device"))
	_, d := do(t, ts, "GET", "/api/v1/cubes/KEQ0000100/devices/0a010a", "")
	if th, _ := d["thermostat"].(map[string]any); th["eco_temp"] != float64(17) {
		t.Fatalf("eco after the follow-up %v", th["eco_temp"])
	}
	// The fake's S reply left one radiogram pending.
	testkit.Eventually(t, "the budget reconnect", func() bool { return fc.Connections() == 2 })
	logs.Await(t, 0, "the budget refresh logged", testkit.Msg("budget refreshed after writes"))
	time.Sleep(200 * time.Millisecond)
	before, others := sent()
	if before > 5 || others != 0 {
		t.Fatalf("%d re-reads of the device, %d of others", before, others)
	}

	// A room setpoint re-reads nothing, but wants the budget again.
	conn, r = proxyClient(t, proxyAddr)
	proxyWrite(t, conn, r, "AARAAAAACgENBgA=")
	testkit.Eventually(t, "the second budget reconnect", func() bool { return fc.Connections() == 3 })
	if after, others := sent(); after != before || others != 0 {
		t.Fatalf("a setpoint re-read: %d re-reads of the device (%d before), %d of others", after, before, others)
	}
}
