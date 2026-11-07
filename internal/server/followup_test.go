// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"net"
	"net/http/httptest"
	"strings"
	"sync"
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
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond},
		RefreshInterval: time.Hour, FollowUpDelays: []time.Duration{40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond},
		FollowUpReconnectGap: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})
	const base = "/api/v1/cubes/KEQ0000100"
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
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond},
		FollowUpDelays: []time.Duration{20 * time.Millisecond, 20 * time.Millisecond}, FollowUpReconnectGap: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/14d08e/config", `{"eco_temp":17}`); code != 200 {
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

// syncBuffer collects log output from several goroutines.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

// proxyClient connects to a proxy and reads its greeting.
func proxyClient(t *testing.T, addr string) (net.Conn, *cube.Reader) {
	t.Helper()
	var conn net.Conn
	eventually(t, "the proxy", func() bool {
		var err error
		conn, err = net.DialTimeout("tcp", addr, time.Second)
		return err == nil
	})
	t.Cleanup(func() { conn.Close() })
	conn.SetDeadline(time.Now().Add(10 * time.Second))
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
	if m, err := r.Read(); err != nil || m.Type != 'S' {
		t.Fatalf("s:%s answered %v %v", data, m, err)
	}
}

// A write that a client of the LAN proxy sends is followed up as the
// API's own: a configuration write has the device's C line re-read until
// it changes, then the budget reconnect; a setpoint gets the budget
// reconnect only.
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
	var logs syncBuffer
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond},
		RefreshInterval: time.Hour, FollowUpDelays: []time.Duration{40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond},
		FollowUpReconnectGap: time.Millisecond, Proxy: []ProxyListener{{Addr: proxyAddr}},
		Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})
	sent := func() (rereads, others int) {
		for _, m := range fc.Received() {
			switch {
			case m.String() == "c:14d08e":
				rereads++
			case m.Type == 'c':
				others++
			}
		}
		return rereads, others
	}

	// The original software's 0x11 (preset temperatures) to 14d08e.
	conn, r := proxyClient(t, proxyAddr)
	proxyWrite(t, conn, r, "AAARAAAAFNCOACwePQkDCgM=")
	eventually(t, "a re-read", func() bool { n, _ := sent(); return n >= 1 })
	fc.SetConfig(withEco(t, greeting, 0x14d08e, 17))
	eventually(t, "the device's new configuration", func() bool {
		return strings.Contains(logs.String(), "device configuration taken over by the device")
	})
	_, d := do(t, ts, "GET", "/api/v1/cubes/KEQ0000100/devices/14d08e", "")
	if th, _ := d["thermostat"].(map[string]any); th["eco_temp"] != float64(17) {
		t.Fatalf("eco after the follow-up %v", th["eco_temp"])
	}
	// The fake's S reply left one radiogram pending.
	eventually(t, "the budget reconnect", func() bool { return fc.Connections() == 2 })
	eventually(t, "the budget refresh logged", func() bool { return strings.Contains(logs.String(), "budget refreshed after writes") })
	time.Sleep(200 * time.Millisecond)
	before, others := sent()
	if before > 5 || others != 0 {
		t.Fatalf("%d re-reads of the device, %d of others", before, others)
	}

	// A room setpoint re-reads nothing, but wants the budget again.
	conn, r = proxyClient(t, proxyAddr)
	proxyWrite(t, conn, r, "AARAAAAAFRS2BgA=")
	eventually(t, "the second budget reconnect", func() bool { return fc.Connections() == 3 })
	if after, others := sent(); after != before || others != 0 {
		t.Fatalf("a setpoint re-read: %d re-reads of the device (%d before), %d of others", after, before, others)
	}
}
