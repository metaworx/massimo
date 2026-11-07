// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"encoding/base64"
	"errors"
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
	if s.Hello() == nil || s.Hello().Serial != "KEQ0000100" {
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

func TestReconnectRefreshesTheSnapshot(t *testing.T) {
	srv, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr})
	go c.Run(ctx)
	select {
	case <-c.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("client never became ready")
	}
	// A device the Cube no longer has (deleted by another client, say)
	// lingers in the snapshot until the next greeting.
	gone, ok := c.Snapshot().ConfigLine(0x1514b6)
	if !ok {
		t.Fatal("no C line for 1514b6")
	}
	raw, err := base64.StdEncoding.DecodeString(gone.Data[7:])
	if err != nil {
		t.Fatal(err)
	}
	raw[1], raw[2], raw[3] = 0x0a, 0x0b, 0x0c
	c.ApplyLocal(cube.Message{Type: 'C', Data: "0a0b0c," + base64.StdEncoding.EncodeToString(raw)})
	if c.Snapshot().Config(0x0a0b0c) == nil {
		t.Fatal("extra device not applied")
	}
	rctx, rcancel := context.WithTimeout(ctx, 5*time.Second)
	defer rcancel()
	if err := c.Reconnect(rctx); err != nil {
		t.Fatal(err)
	}
	if srv.Connections() != 2 {
		t.Fatalf("connections %d", srv.Connections())
	}
	if c.Snapshot().Config(0x0a0b0c) != nil {
		t.Fatal("device the greeting no longer lists kept")
	}
	for _, rf := range c.Snapshot().Devices() {
		if rf == 0x0a0b0c {
			t.Fatal("device still listed")
		}
	}
	if c.Snapshot().Config(0x1514b6) == nil {
		t.Fatal("real device dropped")
	}
	// No disconnected event for a reconnect on purpose.
	for {
		select {
		case e := <-c.Events():
			if e.Kind == EventDisconnected {
				t.Fatal("disconnected event")
			}
			continue
		default:
		}
		break
	}
}

func TestConnectedFollowsTheConnection(t *testing.T) {
	srv, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr})
	if c.Connected() {
		t.Fatal("connected before Run")
	}
	go c.Run(ctx)
	waitEvent(t, c, EventConnected)
	if !c.Connected() || c.Snapshot().Metadata() == nil {
		t.Fatal("not connected after the greeting")
	}
	srv.Close()
	waitEvent(t, c, EventDisconnected)
	if c.Connected() {
		t.Fatal("still connected after the cube went away")
	}
}

func TestClientWaitsForItsAddress(t *testing.T) {
	srv, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{})
	go c.Run(ctx)
	time.Sleep(100 * time.Millisecond)
	if srv.Connections() != 0 || c.Address() != "" {
		t.Fatalf("connected without an address: %d %q", srv.Connections(), c.Address())
	}
	if !c.SetAddress(addr) || c.SetAddress(addr) {
		t.Fatal("SetAddress did not report the change once")
	}
	waitEvent(t, c, EventConnected)
	if got := New(Options{Addr: "10.0.0.1"}).Address(); got != "10.0.0.1:62910" {
		t.Fatalf("default port: %q", got)
	}
}

// ForgetDevice drops a deleted device from the snapshot, its metadata
// entry included, and says so once with a device_forgotten event; the
// other devices stay.
func TestForgetDevice(t *testing.T) {
	_, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr})
	go c.Run(ctx)
	waitEvent(t, c, EventConnected)
	const rf cube.RFAddress = 0x1514b6
	c.ForgetDevice(rf)
	if e := waitEvent(t, c, EventForgotten); e.Device != rf || e.Kind.String() != "device_forgotten" {
		t.Fatalf("event %+v", e)
	}
	s := c.Snapshot()
	if _, ok := s.State(rf); ok || s.Config(rf) != nil {
		t.Fatal("config or state kept")
	}
	for _, d := range s.Devices() {
		if d == rf {
			t.Fatal("still among the devices")
		}
	}
	for _, d := range s.Metadata().Devices {
		if d.RFAddress == rf {
			t.Fatal("still in the metadata")
		}
	}
	if len(s.Devices()) != 5 || s.Config(0x14d08e) == nil {
		t.Fatalf("the other devices: %v", s.Devices())
	}
	l, ok := s.DeviceListLine()
	if !ok {
		t.Fatal("no L line")
	}
	states, err := cube.ParseDeviceList(l.Data, map[cube.RFAddress]cube.DeviceType{})
	if err != nil || len(states) != 5 {
		t.Fatalf("L line %s: %d entries, %v", l, len(states), err)
	}
	for _, st := range states {
		if st.RFAddress == rf {
			t.Fatal("still in the L line")
		}
	}
	c.ForgetDevice(rf)
	select {
	case e := <-c.Events():
		if e.Kind == EventForgotten {
			t.Fatal("a second event for a device already forgotten")
		}
	case <-time.After(50 * time.Millisecond):
	}
	// The Cube's next device list, which still has it, does not bring it
	// back; a C line for it does, and its state follows.
	if _, err := c.Send(ctx, cube.CmdList); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.State(rf); ok {
		t.Fatal("the device list brought the forgotten device back")
	}
	line, err := c.Send(ctx, cube.Message{Type: 'c', Data: rf.String()})
	if err != nil || line.Type != 'C' {
		t.Fatalf("c: %s %v", line, err)
	}
	c.ApplyLocal(line)
	if _, err := c.Send(ctx, cube.CmdList); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.State(rf); !ok || s.Config(rf) == nil {
		t.Fatal("not back after its C line")
	}
}

// A t: naming several devices is answered with an A line for each, and
// Send returns only after the last, so that the next command's reply is
// its own and not a late A of the deletion.
func TestSendWaitsForEveryAOfADelete(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &fakecube.Server{Greeting: greeting, ReplyGap: 100 * time.Millisecond}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr})
	go c.Run(ctx)
	<-c.Ready()
	del := cube.CmdDeleteDevices(true, 0x14ce3d, 0x14ce58, 0x151486)
	start := time.Now()
	if _, err := c.Send(ctx, del); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < 180*time.Millisecond {
		t.Fatalf("Send of a t: for three devices returned after %s, before the Cube's third A", took)
	}
}

// freeAddr is an address nothing listens on.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

// A new address, as discovery finds it after a DHCP lease, is dialled at
// once, not after the backoff the failed dials to the old one built up.
func TestSetAddressCutsTheBackoffShort(t *testing.T) {
	_, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: freeAddr(t)})
	go c.Run(ctx)
	time.Sleep(1300 * time.Millisecond) // two failed dials: the client waits 2 s now
	start := time.Now()
	c.SetAddress(addr)
	select {
	case <-c.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("never connected")
	}
	if took := time.Since(start); took > 700*time.Millisecond {
		t.Fatalf("connected %s after the new address", took)
	}
}

// Reconnect while the client is between connections dials at once and
// waits for the greeting, rather than failing with ErrNotConnected.
func TestReconnectWhileDisconnected(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr})
	go c.Run(ctx)
	time.Sleep(1300 * time.Millisecond)
	srv := &fakecube.Server{Greeting: greeting}
	if _, err := srv.Start(addr); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	rctx, stop := context.WithTimeout(ctx, 700*time.Millisecond)
	defer stop()
	if err := c.Reconnect(rctx); err != nil {
		t.Fatalf("Reconnect while disconnected: %v", err)
	}
	if !c.Connected() {
		t.Fatal("not connected after Reconnect")
	}
}

// A re-read c: sent while the greeting is still coming takes one of its
// C lines as its reply; that line is applied all the same, so the
// device it describes is still there when the greeting ends.
func TestGreetingCLineAnsweringASend(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	server, client := net.Pipe()
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: "192.0.2.10:62910", Dialer: func(context.Context, string) (net.Conn, error) { return client, nil }})
	go c.Run(ctx)
	var first cube.RFAddress
	for _, m := range greeting {
		if m.Type == 'C' && first == 0 {
			cfg, err := cube.ParseConfig(m.Data)
			if err != nil {
				t.Fatal(err)
			}
			first = cfg.RFAddress
		}
	}
	go func() {
		for _, m := range greeting {
			if m.Type != 'C' && m.Type != 'L' {
				server.Write([]byte(m.Wire()))
			}
		}
	}()
	// The re-read goes out mid-greeting and the Cube's first C answers it.
	replied := make(chan error, 1)
	go func() {
		for {
			_, err := c.Send(ctx, cube.Message{Type: 'c', Data: first.String()})
			if !errors.Is(err, ErrNotConnected) {
				replied <- err
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	r := cube.NewReader(server)
	for {
		m, err := r.Read()
		if err != nil {
			t.Fatal(err)
		}
		if m.Type == 'c' {
			break
		}
	}
	go func() {
		for _, m := range greeting {
			if m.Type == 'C' || m.Type == 'L' {
				server.Write([]byte(m.Wire()))
			}
		}
		for { // the rest the client sends
			if _, err := r.Read(); err != nil {
				return
			}
		}
	}()
	if err := <-replied; err != nil {
		t.Fatal(err)
	}
	select {
	case <-c.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("no greeting")
	}
	if c.Snapshot().Config(first) == nil {
		t.Fatalf("device %s dropped at the end of the greeting", first)
	}
}
