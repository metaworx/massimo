// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"net"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// The fake's refusal is the kernel's (Linux): a refused client's dial
// times out, as against a Cube. These tests dial with a short timeout.

// singleFake starts a fake Cube that serves one client at a time.
func singleFake(t *testing.T) (*fakecube.Server, string) {
	t.Helper()
	g, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &fakecube.Server{Greeting: g, SingleClient: true, HoldDead: time.Hour}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	return srv, addr
}

// quickDial dials with a timeout short enough for a test.
func quickDial(ctx context.Context, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 150 * time.Millisecond}
	return d.DialContext(ctx, "tcp", addr)
}

// After an outage longer than the hold, the Cube still holds serve's
// old connection: dials time out while discovery hears the Cube, which
// is stale, logged once; once the Cube lets go the client connects and
// the link is connected again.
func TestAStaleCubeAfterTheHold(t *testing.T) {
	srv, addr := singleFake(t)
	var logs logLines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, PollInterval: 100 * time.Millisecond, ReplyTimeout: 50 * time.Millisecond, Backoff: 10 * time.Millisecond,
		Hold: 200 * time.Millisecond, Logger: logs.logger(), Dialer: quickDial})
	go c.Run(ctx)
	eventually(t, "first greeting", c.Connected)
	if l := c.Link(); l.State != LinkConnected {
		t.Fatalf("link %+v, want connected", l)
	}

	srv.SetSilent(true)
	eventually(t, "the hold", func() bool { return c.Link().State == LinkHeld })
	if l := c.Link(); l.Since.IsZero() || !l.Until.After(l.Since) {
		t.Errorf("held link %+v: want since and until", l)
	}
	eventually(t, "the dead session", srv.HeldDead)
	srv.SetSilent(false) // the network is back, the Cube still holds the session
	eventually(t, "two dials timed out", func() bool { return logs.count("DEBUG", "cube connection attempt failed") >= 2 })
	if l := c.Link(); l.State != LinkOffline {
		t.Errorf("link %+v before discovery heard the Cube, want offline", l)
	}
	c.Heard()
	if l := c.Link(); l.State != LinkStale || l.Since.IsZero() {
		t.Fatalf("link %+v, want stale", l)
	}
	c.Heard()
	if n := logs.count("WARN", "cube holds serve's old connection"); n != 1 {
		t.Errorf("%d stale lines, want 1", n)
	}

	srv.Release()
	eventually(t, "the connection after the release", func() bool { return c.Link().State == LinkConnected })
	// The line follows the state, written right after the greeting.
	eventually(t, "the line of the refusal's end", func() bool { return logs.count("INFO", "cube no longer refuses") == 1 })
}

// A Cube that another client holds, with no connection of serve's left
// behind, is busy.
func TestABusyCube(t *testing.T) {
	srv, addr := singleFake(t)
	other, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	eventually(t, "the other client served", func() bool { return srv.Connections() == 1 })

	var logs logLines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, PollInterval: time.Minute, Backoff: 10 * time.Millisecond, Logger: logs.logger(), Dialer: quickDial})
	go c.Run(ctx)
	eventually(t, "two dials timed out", func() bool { return logs.count("DEBUG", "cube connection attempt failed") >= 1 })
	eventually(t, "the busy link", func() bool { c.Heard(); return c.Link().State == LinkBusy })
	if n := logs.count("WARN", "cube busy with another client"); n != 1 {
		t.Errorf("%d busy lines, want 1", n)
	}

	other.Close()
	eventually(t, "the connection once the other left", func() bool { return c.Link().State == LinkConnected })
}
