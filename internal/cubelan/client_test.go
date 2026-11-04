// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

const fixture = "../protocol/cube/testdata/cube-fw0113-greeting.txt"

func startFake(t *testing.T) (*fakecube.Server, string) {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &fakecube.Server{Greeting: greeting}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	return srv, addr
}

func waitEvent(t *testing.T, c *Client, kind EventKind) Event {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-c.Events():
			if e.Kind == kind {
				return e
			}
		case <-deadline:
			t.Fatalf("no %s event", kind)
		}
	}
}

func TestClientGreetingAndSnapshot(t *testing.T) {
	_, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr})
	go c.Run(ctx)

	select {
	case <-c.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("client never became ready")
	}
	s := c.Snapshot()
	if s.Hello() == nil || s.Hello().Serial != "KEQ0537249" {
		t.Fatalf("hello: %+v", s.Hello())
	}
	if len(s.Metadata().Rooms) != 6 || len(s.Devices()) != 6 {
		t.Fatalf("metadata: %d rooms, %d devices", len(s.Metadata().Rooms), len(s.Devices()))
	}
	if cfg := s.Config(0x1514b6); cfg == nil || cfg.Thermostat == nil || cfg.Thermostat.ComfortTemp != 21.5 {
		t.Fatalf("config: %+v", cfg)
	}
	if st, ok := s.State(0x1514b6); !ok || st.Valid {
		t.Fatalf("state: %+v %v", st, ok)
	}

	// The snapshot can be rendered back into the same lines.
	lines := s.Lines()
	orig, _ := fakecube.LoadGreeting(fixture)
	if len(lines) != len(orig) {
		t.Fatalf("lines: got %d want %d", len(lines), len(orig))
	}
	for i := range orig {
		if lines[i].String() != orig[i].String() {
			t.Errorf("line %d differs:\n got %s\nwant %s", i, lines[i], orig[i])
		}
	}
}

func TestClientSendAndPoll(t *testing.T) {
	srv, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, PollInterval: 50 * time.Millisecond})
	go c.Run(ctx)
	<-c.Ready()

	// A radio command gets an S reply.
	m, _ := cube.CmdSetTemperature(cube.Target{RFAddress: 0x1514b6, RoomID: 6, Room: true}, 19, cube.ModeManual, cube.Until{})
	res, err := c.SendRadio(ctx, m)
	if err != nil || res.Discarded || res.FreeMemorySlots != 0x31 {
		t.Fatalf("send: %+v %v", res, err)
	}
	// NTP round trip.
	if _, err := c.Send(ctx, cube.CmdSetNTP("a.example", "b.example")); err != nil {
		t.Fatal(err)
	}
	reply, err := c.Send(ctx, cube.CmdGetNTP)
	if err != nil || reply.Data != "a.example,b.example" {
		t.Fatalf("ntp: %v %v", reply, err)
	}
	// Polling sends l commands.
	time.Sleep(200 * time.Millisecond)
	var polls int
	for _, r := range srv.Received() {
		if r.Type == 'l' {
			polls++
		}
	}
	if polls < 2 {
		t.Fatalf("want polls, got %d", polls)
	}
}

func TestClientReconnects(t *testing.T) {
	srv, addr := startFake(t)
	var mu sync.Mutex
	current := addr
	dial := func(ctx context.Context, _ string) (net.Conn, error) {
		mu.Lock()
		a := current
		mu.Unlock()
		return (&net.Dialer{}).DialContext(ctx, "tcp", a)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, Dialer: dial})
	go c.Run(ctx)
	<-c.Ready()
	waitEvent(t, c, EventConnected)

	// Kill the server side; the client must notice and reconnect to the
	// replacement server.
	greeting, _ := fakecube.LoadGreeting(fixture)
	srv2 := &fakecube.Server{Greeting: greeting}
	addr2, err := srv2.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv2.Close()
	mu.Lock()
	current = addr2
	mu.Unlock()
	srv.Close()
	waitEvent(t, c, EventDisconnected)
	waitEvent(t, c, EventConnected)
	if srv2.Connections() != 1 {
		t.Fatalf("want 1 connection on the new server, got %d", srv2.Connections())
	}
}
