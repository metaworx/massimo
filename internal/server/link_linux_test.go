// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// A Cube that answers discovery from its address while another client
// holds it, so that the server's dials time out (the fake's refusal is
// Linux's), is listed with link busy: discovery's answer is what tells
// it from a Cube that is off.
func TestABusyCubeIsListedAsBusy(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting, SingleClient: true}
	addr, err := fc.Start("127.0.0.2:0")
	if err != nil {
		t.Skipf("cannot listen on 127.0.0.2: %v", err)
	}
	defer fc.Close()
	other, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	at, err := fakecube.StartDiscovery("127.0.0.2:0", "KEQ0000100", 0x0a0107)
	if err != nil {
		t.Skip(err)
	}
	defer at.Close()

	quick := func(ctx context.Context, a string) (net.Conn, error) {
		d := net.Dialer{Timeout: 150 * time.Millisecond}
		return d.DialContext(ctx, "tcp", a)
	}
	ts, _ := startDiscovering(t, Options{Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}, DiscoverInterval: time.Hour,
		DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{at.Addr()},
		cubeOptions: func(o *cubelan.Options) { o.Dialer, o.Backoff = quick, 50*time.Millisecond }})
	link := func() map[string]any {
		var cubes []map[string]any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		if len(cubes) == 0 {
			return nil
		}
		l, _ := cubes[0]["link"].(map[string]any)
		return l
	}
	eventually(t, "busy", func() bool { l := link(); return l != nil && l["state"] == "busy" })
	if l := link(); l["since"] == nil {
		t.Errorf("busy link %v: want since", l)
	}
	other.Close()
	eventually(t, "connected once the other client left", func() bool { l := link(); return l != nil && l["state"] == "connected" })
}
