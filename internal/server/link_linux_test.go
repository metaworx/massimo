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
	"go.metaworx.ch/massimo/internal/testkit"
)

// A Cube that answers discovery from its address while another client
// holds it, so that the server's dials time out (the fake's refusal is
// Linux's), is listed with link busy: discovery's answer is what tells
// it from a Cube that is off.
func TestABusyCubeIsListedAsBusy(t *testing.T) {
	// The fake is the test's own, at 127.0.0.2 and held by another
	// client before the server starts: the start waits for no Cube.
	fc := &fakecube.Server{Greeting: recorded(t), SingleClient: true}
	addr, err := fc.Start("127.0.0.2:0")
	if err != nil {
		testkit.BindOrSkip(t, err)
	}
	defer fc.Close()
	other, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	// The dial returns once the kernel has queued the connection; the
	// fake refuses others only once it has taken this one, and a dial of
	// the server's before that would get in and wait for a greeting.
	testkit.Eventually(t, "the other client taken", func() bool { return fc.Connections() == 1 })
	at, err := fakecube.StartDiscovery("127.0.0.2:0", "KEQ0000100", 0x0a0107)
	if err != nil {
		testkit.BindOrSkip(t, err)
	}
	defer at.Close()

	quick := func(ctx context.Context, a string) (net.Conn, error) {
		d := net.Dialer{Timeout: 150 * time.Millisecond}
		return d.DialContext(ctx, "tcp", a)
	}
	ts := startServer(t, discovering(Options{DiscoverInterval: time.Hour, DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{at.Addr()},
		cubeOptions: func(o *cubelan.Options) { o.Dialer, o.Backoff = quick, 50*time.Millisecond }}),
		withCubes(CubeConfig{Serial: "KEQ0000100", Addr: addr})).ts
	link := func() map[string]any {
		var cubes []map[string]any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		if len(cubes) == 0 {
			return nil
		}
		l, _ := cubes[0]["link"].(map[string]any)
		return l
	}
	testkit.Eventually(t, "busy", func() bool { l := link(); return l != nil && l["state"] == "busy" })
	if l := link(); l["since"] == nil {
		t.Errorf("busy link %v: want since", l)
	}
	other.Close()
	testkit.Eventually(t, "connected once the other client left", func() bool { l := link(); return l != nil && l["state"] == "connected" })
}
