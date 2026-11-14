// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// A proxy client gets only what it asks for: a change of the Cube's state
// sends it nothing, and its next l: shows the change. The original
// software takes the next line it reads for the reply to its command, so
// an unasked L line makes every later reply one line late ("S: expected").
func TestProxySendsNothingUnasked(t *testing.T) {
	greeting, err := fakecube.RoomsFixture()
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
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond},
		PollInterval: 50 * time.Millisecond, RefreshInterval: time.Hour, Proxy: []ProxyListener{{Addr: proxyAddr}}})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})

	conn, r := proxyClient(t, proxyAddr)
	setpoint := 23.5
	if err := fc.SetState(0x0a0202, fakecube.State{Setpoint: &setpoint}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the new setpoint", func() bool {
		_, d := do(t, ts, "GET", "/api/v1/cubes/KEQ0000100/devices/0a0202", "")
		st, _ := d["state"].(map[string]any)
		return st["setpoint"] == setpoint
	})
	// The server has seen the change; give an unasked line time to come.
	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if m, err := r.Read(); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("the proxy sent %v unasked (%v)", m, err)
	}

	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("l:\r\n")); err != nil {
		t.Fatal(err)
	}
	l, err := r.Read()
	if err != nil || l.Type != 'L' {
		t.Fatalf("l: answered %v %v", l, err)
	}
	if got := setpointIn(t, l, 0x0a0202); got != setpoint {
		t.Fatalf("the L line's setpoint of 0a0202 is %v, want %v", got, setpoint)
	}
}

// setpointIn is rf's setpoint in an L line, -1 when rf has no entry with
// one.
func setpointIn(t *testing.T, l cube.Message, rf cube.RFAddress) float64 {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(l.Data)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+4 <= len(raw); i += 1 + int(raw[i]) {
		if cube.RFAddressFromBytes(raw[i+1:i+4]) == rf && int(raw[i]) >= 8 && i+8 < len(raw) {
			return float64(raw[i+8]&0x3f) / 2
		}
	}
	return -1
}
