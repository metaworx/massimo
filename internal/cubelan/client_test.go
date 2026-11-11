// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
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
	if cfg := s.Config(0x0a010d); cfg == nil || cfg.Thermostat == nil || cfg.Thermostat.ComfortTemp != 21.5 {
		t.Fatalf("config: %+v", cfg)
	}
	if st, ok := s.State(0x0a010d); !ok || st.Valid {
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
	m, _ := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a010d, RoomID: 6, Room: true}, 19, cube.ModeManual, cube.Until{})
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
	gone, ok := c.Snapshot().ConfigLine(0x0a010d)
	if !ok {
		t.Fatal("no C line for 0a010d")
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
	if c.Snapshot().Config(0x0a010d) == nil {
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
	const rf cube.RFAddress = 0x0a010d
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
	if len(s.Devices()) != 5 || s.Config(0x0a010a) == nil {
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

// The event of a forgotten device carries what the snapshot knew of it
// before it was dropped: its type, serial, name and room from the room
// and device list; and the serial finds the device until then, in any
// case (AP Logging v1.1, Block 1).
func TestForgetDeviceCarriesLastView(t *testing.T) {
	_, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr})
	go c.Run(ctx)
	waitEvent(t, c, EventConnected)
	const rf cube.RFAddress = 0x0a010d
	s := c.Snapshot()
	var want cube.DeviceMeta
	for _, d := range s.Metadata().Devices {
		if d.RFAddress == rf {
			want = d
		}
	}
	if want.Serial == "" || want.Name == "" {
		t.Fatalf("the greeting's entry for %s: %+v", rf, want)
	}
	if got := s.Serial(rf); got != want.Serial {
		t.Fatalf("Serial(%s) = %q, want %q", rf, got, want.Serial)
	}
	if got, ok := s.BySerial(strings.ToLower(want.Serial)); !ok || got != rf {
		t.Fatalf("BySerial(%q) = %s %v", want.Serial, got, ok)
	}
	c.ForgetDevice(rf)
	e := waitEvent(t, c, EventForgotten)
	if e.Gone == nil {
		t.Fatal("no Gone on the event")
	}
	if g := *e.Gone; g.Type != want.Type || g.Serial != want.Serial || g.Name != want.Name || g.RoomID != want.RoomID {
		t.Fatalf("Gone %+v, want the list's %+v", g, want)
	}
	if got := s.Serial(rf); got != "" {
		t.Fatalf("Serial after Forget: %q", got)
	}
	if _, ok := s.BySerial(want.Serial); ok {
		t.Fatal("BySerial finds a forgotten device")
	}

	// A removal writes the room and device list without the device
	// first, then forgets it: the event still names it.
	const other cube.RFAddress = 0x0a010a
	md := *s.Metadata()
	var kept []cube.DeviceMeta
	var was cube.DeviceMeta
	for _, d := range md.Devices {
		if d.RFAddress == other {
			was = d
			continue
		}
		kept = append(kept, d)
	}
	md.Devices = kept
	cmds, err := cube.WriteMetadataCommands(&md)
	if err != nil {
		t.Fatal(err)
	}
	c.ApplyMetadataWrite(cmds)
	for _, d := range s.Metadata().Devices {
		if d.RFAddress == other {
			t.Fatal("the list written without the device still names it")
		}
	}
	c.ForgetDevice(other)
	e = waitEvent(t, c, EventForgotten)
	if e.Device != other || e.Gone == nil || e.Gone.Name != was.Name || e.Gone.Serial != was.Serial || e.Gone.RoomID != was.RoomID {
		t.Fatalf("after the list without it: %+v, want %+v", e.Gone, was)
	}
}

// Send returns once an L or C reply is in the snapshot, also when the
// reader is slow to apply it, so that a read right after sees the reply.
func TestSendReturnsWithTheReplyApplied(t *testing.T) {
	_, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr})
	// Every L and C line waits before it goes into the snapshot: a Send
	// that returned before the apply would find its reply missing.
	c.beforeApply = func(m cube.Message) {
		if m.Type == 'L' || m.Type == 'C' {
			time.Sleep(50 * time.Millisecond)
		}
	}
	go c.Run(ctx)
	waitEvent(t, c, EventConnected)
	const rf cube.RFAddress = 0x0a010d
	c.ForgetDevice(rf)
	waitEvent(t, c, EventForgotten)
	s := c.Snapshot()
	if s.Config(rf) != nil {
		t.Fatal("config kept after ForgetDevice")
	}
	if line, err := c.Send(ctx, cube.Message{Type: 'c', Data: rf.String()}); err != nil || line.Type != 'C' {
		t.Fatalf("c: %s %v", line, err)
	}
	if s.Config(rf) == nil {
		t.Fatal("Send returned before its C reply was in the snapshot")
	}
	if _, err := c.Send(ctx, cube.CmdList); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.State(rf); !ok {
		t.Fatal("Send returned before its L reply was in the snapshot")
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
	del := cube.CmdDeleteDevices(true, 0x0a0108, 0x0a0109, 0x0a010b)
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

// logLines captures a client's log, Debug included, for the tests that
// count its lines.
type logLines struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logLines) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logLines) logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(l, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// lines are the lines of msg at level, as slog's text handler writes them.
func (l *logLines) lines(level, msg string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, line := range strings.Split(l.buf.String(), "\n") {
		if strings.Contains(line, " level="+level+" ") && strings.Contains(line, ` msg="`+msg+`"`) {
			out = append(out, line)
		}
	}
	return out
}

func (l *logLines) count(level, msg string) int { return len(l.lines(level, msg)) }

// eventCount counts a client's events by kind, reading them as they
// come so that none is dropped.
type eventCount struct {
	mu sync.Mutex
	n  map[EventKind]int
}

func countEvents(ctx context.Context, c *Client) *eventCount {
	ec := &eventCount{n: map[EventKind]int{}}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-c.Events():
				ec.mu.Lock()
				ec.n[e.Kind]++
				ec.mu.Unlock()
			}
		}
	}()
	return ec
}

func (ec *eventCount) get(k EventKind) int {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.n[k]
}

// eventually waits up to five seconds for cond.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !cond(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no %s within 5 s", what)
		}
	}
}

// A Cube that goes away is reported once, however often the client
// retries, and its return once, with how long it was down; the next
// loss is reported again.
func TestALostCubeIsReportedOnce(t *testing.T) {
	srv, addr := startFake(t)
	var logs logLines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, Backoff: 10 * time.Millisecond, Logger: logs.logger()})
	evs := countEvents(ctx, c)
	go c.Run(ctx)
	eventually(t, "first greeting", func() bool { return evs.get(EventConnected) == 1 })

	srv.Close()
	eventually(t, "three failed retries", func() bool { return logs.count("DEBUG", "cube connection attempt failed") >= 3 })
	srv2 := &fakecube.Server{Greeting: srv.Greeting}
	if _, err := srv2.Start(addr); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv2.Close() })
	eventually(t, "greeting after the loss", func() bool { return evs.get(EventConnected) == 2 })
	// The client logs the return before the event (AP LostOrder).
	if n := logs.count("WARN", "cube connection lost"); n != 1 {
		t.Errorf("%d cube connection lost lines, want 1", n)
	}
	if n := evs.get(EventDisconnected); n != 1 {
		t.Errorf("%d disconnected events, want 1", n)
	}
	if back := logs.lines("INFO", "cube connection re-established"); len(back) != 1 || !strings.Contains(back[0], " down_for=") {
		t.Errorf("re-established lines: %q", back)
	}
	if n := logs.count("INFO", "cube connected"); n != 1 {
		t.Errorf("%d cube connected lines, want the first connection's only", n)
	}
	if n := logs.count("WARN", "cube still unreachable"); n != 0 {
		t.Errorf("%d reminders within a day", n)
	}

	srv2.Close()
	eventually(t, "second disconnected event", func() bool { return evs.get(EventDisconnected) == 2 })
	if n := logs.count("WARN", "cube connection lost"); n != 2 {
		t.Errorf("%d cube connection lost lines after the second loss, want 2", n)
	}
}

// Lost holds from the loss Run reports to the next complete greeting;
// a deliberate Reconnect is none.
func TestLostFromALossToTheNextGreeting(t *testing.T) {
	srv, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, Backoff: 10 * time.Millisecond})
	if c.Lost() {
		t.Fatal("lost before Run")
	}
	evs := countEvents(ctx, c)
	go c.Run(ctx)
	eventually(t, "first greeting", func() bool { return evs.get(EventConnected) == 1 })
	if c.Lost() {
		t.Fatal("lost after the greeting")
	}

	var sawLost atomic.Bool
	done := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		for {
			select {
			case <-done:
				return
			default:
			}
			if c.Lost() {
				sawLost.Store(true)
			}
			time.Sleep(100 * time.Microsecond)
		}
	}()
	if err := c.Reconnect(ctx); err != nil {
		t.Fatal(err)
	}
	close(done)
	<-sampled
	if sawLost.Load() {
		t.Error("lost during a deliberate reconnect")
	}

	srv.Close()
	eventually(t, "the loss", c.Lost)
	// The event follows Lost, and countEvents reads it on its own.
	eventually(t, "the disconnected event", func() bool { return evs.get(EventDisconnected) >= 1 })
	if n := evs.get(EventDisconnected); n != 1 {
		t.Errorf("%d disconnected events, want 1", n)
	}
	srv2 := &fakecube.Server{Greeting: srv.Greeting}
	if _, err := srv2.Start(addr); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv2.Close() })
	eventually(t, "the greeting after the loss", func() bool { return evs.get(EventConnected) == 3 })
	if c.Lost() || !c.Connected() {
		t.Errorf("after the greeting: lost %v, connected %v", c.Lost(), c.Connected())
	}
}

// pollsReceived counts the l: lines a fake Cube has read.
func pollsReceived(srv *fakecube.Server) int {
	n := 0
	for _, m := range srv.Received() {
		if m.Type == 'l' {
			n++
		}
	}
	return n
}

// A Cube that answers nothing, as one whose cable is out, is lost after
// two unanswered polls, long before the read deadline of three polls and
// 30 s, and connected again once it answers.
func TestSilentCubeIsLostAfterTwoPolls(t *testing.T) {
	srv, addr := startFake(t)
	var logs logLines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, PollInterval: 200 * time.Millisecond, ReplyTimeout: 100 * time.Millisecond, Backoff: 10 * time.Millisecond, Logger: logs.logger()})
	evs := countEvents(ctx, c)
	go c.Run(ctx)
	eventually(t, "first greeting", func() bool { return evs.get(EventConnected) == 1 })

	srv.SetSilent(true)
	eventually(t, "the loss", c.Lost)
	if lost := logs.lines("WARN", "cube connection lost"); len(lost) != 1 || !strings.Contains(lost[0], "2 polls in a row") {
		t.Errorf("cube connection lost lines: %q", lost)
	}
	srv.SetSilent(false)
	eventually(t, "the greeting after the loss", func() bool { return evs.get(EventConnected) == 2 })
	if n := evs.get(EventDisconnected); n != 1 {
		t.Errorf("%d disconnected events, want 1", n)
	}
}

// holdLost is a log handler that holds the first "cube connection lost"
// record until release is closed, and drops every other.
type holdLost struct {
	once    *sync.Once
	held    chan struct{}
	release chan struct{}
}

func (h holdLost) Enabled(context.Context, slog.Level) bool { return true }
func (h holdLost) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h holdLost) WithGroup(string) slog.Handler            { return h }
func (h holdLost) Handle(_ context.Context, r slog.Record) error {
	if r.Message == "cube connection lost" {
		h.once.Do(func() { close(h.held); <-h.release })
	}
	return nil
}

// A loss is reported in the order Lost promises: while the warning is
// being written, Lost is false and no EventDisconnected is sent; after
// it, both follow (AP LostOrder).
func TestLostAfterItsWarning(t *testing.T) {
	srv, addr := startFake(t)
	h := holdLost{once: new(sync.Once), held: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, Backoff: 10 * time.Millisecond, Logger: slog.New(h)})
	go c.Run(ctx)
	eventually(t, "first greeting", c.Connected)

	srv.Close()
	select {
	case <-h.held:
	case <-time.After(10 * time.Second):
		close(h.release)
		t.Fatal("no cube connection lost warning")
	}
	lost := c.Lost()
	var disconnected int
	for drained := false; !drained; {
		select {
		case e := <-c.Events():
			if e.Kind == EventDisconnected {
				disconnected++
			}
		default:
			drained = true
		}
	}
	close(h.release)
	if lost || disconnected != 0 {
		t.Errorf("while the warning is written: lost %v, %d disconnected events", lost, disconnected)
	}
	eventually(t, "the loss after the warning", c.Lost)
}

// One poll the Cube answers late, or not at all, followed by one it
// answers, keeps the connection.
func TestOneMissedPollKeepsTheConnection(t *testing.T) {
	srv, addr := startFake(t)
	var logs logLines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, PollInterval: 200 * time.Millisecond, ReplyTimeout: 50 * time.Millisecond, Backoff: 10 * time.Millisecond, Logger: logs.logger()})
	evs := countEvents(ctx, c)
	go c.Run(ctx)
	eventually(t, "first greeting", func() bool { return evs.get(EventConnected) == 1 })

	n := pollsReceived(srv)
	srv.SetSilent(true)
	eventually(t, "a poll held", func() bool { return pollsReceived(srv) > n })
	eventually(t, "the poll missed", func() bool { return logs.count("WARN", "poll failed") >= 1 })
	srv.SetSilent(false)
	m := pollsReceived(srv)
	eventually(t, "two polls more", func() bool { return pollsReceived(srv) >= m+2 })
	if c.Lost() || evs.get(EventDisconnected) != 0 || srv.Connections() != 1 {
		t.Errorf("lost %v, %d disconnected events, %d connections", c.Lost(), evs.get(EventDisconnected), srv.Connections())
	}
}

// Polls the Cube leaves unanswered while pairing runs count nothing,
// since whether the Cube answers l: then is unconfirmed; once pairing
// ends they do.
func TestSilentCubeWhilePairingIsKept(t *testing.T) {
	srv, addr := startFake(t)
	var logs logLines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, PollInterval: 100 * time.Millisecond, ReplyTimeout: 30 * time.Millisecond, Backoff: 10 * time.Millisecond, Logger: logs.logger()})
	evs := countEvents(ctx, c)
	go c.Run(ctx)
	eventually(t, "first greeting", func() bool { return evs.get(EventConnected) == 1 })

	if err := c.StartInclusion(time.Minute); err != nil {
		t.Fatal(err)
	}
	srv.SetSilent(true)
	eventually(t, "three missed polls", func() bool { return logs.count("WARN", "poll failed") >= 3 })
	if c.Lost() || evs.get(EventDisconnected) != 0 {
		t.Fatal("lost while pairing runs")
	}
	c.EndInclusion()
	eventually(t, "the loss once pairing ended", c.Lost)
}

// A Cube never reached since the start is reported once, at the first
// failed attempt, and the first greeting says it is back.
func TestAnUnreachableCubeIsReportedOnce(t *testing.T) {
	addr := freeAddr(t)
	var logs logLines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, Backoff: 10 * time.Millisecond, Logger: logs.logger()})
	evs := countEvents(ctx, c)
	go c.Run(ctx)
	eventually(t, "four failed retries", func() bool { return logs.count("DEBUG", "cube connection attempt failed") >= 4 })
	if lost := logs.lines("WARN", "cube connection lost"); len(lost) != 1 || !strings.Contains(lost[0], " retry_in=") {
		t.Errorf("cube connection lost lines: %q", lost)
	}

	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &fakecube.Server{Greeting: greeting}
	if _, err := srv.Start(addr); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	eventually(t, "greeting", func() bool { return evs.get(EventConnected) == 1 })
	// The client logs the return before the event (AP LostOrder).
	if n := evs.get(EventDisconnected); n != 1 {
		t.Errorf("%d disconnected events, want 1", n)
	}
	if n := logs.count("WARN", "cube connection lost"); n != 1 {
		t.Errorf("%d cube connection lost lines, want 1", n)
	}
	if n := logs.count("INFO", "cube connection re-established"); n != 1 {
		t.Errorf("%d re-established lines, want 1", n)
	}
	if n := logs.count("INFO", "cube connected"); n != 0 {
		t.Errorf("%d cube connected lines after a loss", n)
	}
}

// A Cube that stays unreachable is warned of again every RemindEvery,
// with how long it has been down, and without another event.
func TestAReminderWhileTheCubeStaysDown(t *testing.T) {
	addr := freeAddr(t)
	var logs logLines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, Backoff: 10 * time.Millisecond, RemindEvery: 100 * time.Millisecond, Logger: logs.logger()})
	evs := countEvents(ctx, c)
	go c.Run(ctx)
	eventually(t, "two reminders", func() bool { return logs.count("WARN", "cube still unreachable") >= 2 })
	for _, line := range logs.lines("WARN", "cube still unreachable") {
		if !strings.Contains(line, " down_for=") {
			t.Errorf("reminder without down_for: %s", line)
		}
	}
	if n := logs.count("WARN", "cube connection lost"); n != 1 {
		t.Errorf("%d cube connection lost lines, want 1", n)
	}
	eventually(t, "disconnected event", func() bool { return evs.get(EventDisconnected) > 0 })
	if n := evs.get(EventDisconnected); n != 1 {
		t.Errorf("%d disconnected events, want 1", n)
	}
}

// A deliberate Reconnect whose dial then fails is a loss like any
// other: reported once, and its end once.
func TestAFailedReconnectIsALoss(t *testing.T) {
	_, addr := startFake(t)
	var refuse atomic.Bool
	dial := func(ctx context.Context, a string) (net.Conn, error) {
		if refuse.Load() {
			return nil, errors.New("refused by the test")
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp", a)
	}
	var logs logLines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, Dialer: dial, Backoff: 10 * time.Millisecond, Logger: logs.logger()})
	evs := countEvents(ctx, c)
	go c.Run(ctx)
	eventually(t, "first greeting", func() bool { return evs.get(EventConnected) == 1 })

	refuse.Store(true)
	rctx, stop := context.WithTimeout(ctx, 200*time.Millisecond)
	defer stop()
	if err := c.Reconnect(rctx); err == nil {
		t.Fatal("Reconnect succeeded with every dial refused")
	}
	eventually(t, "two failed retries", func() bool { return logs.count("DEBUG", "cube connection attempt failed") >= 2 })
	refuse.Store(false)
	eventually(t, "greeting after the reconnect", func() bool { return evs.get(EventConnected) == 2 })
	if n := logs.count("INFO", "cube reconnecting"); n != 1 {
		t.Errorf("%d cube reconnecting lines, want 1", n)
	}
	if n := logs.count("WARN", "cube connection lost"); n != 1 {
		t.Errorf("%d cube connection lost lines, want 1", n)
	}
	if n := evs.get(EventDisconnected); n != 1 {
		t.Errorf("%d disconnected events, want 1", n)
	}
	if n := logs.count("INFO", "cube connection re-established"); n != 1 {
		t.Errorf("%d re-established lines, want 1", n)
	}
}
