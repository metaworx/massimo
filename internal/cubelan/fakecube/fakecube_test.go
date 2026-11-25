// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

const fixture = "../../protocol/cube/testdata/cube-fw0113-greeting.txt"

// client is a raw connection to the fake that has read the greeting.
type client struct {
	t     *testing.T
	conn  net.Conn
	r     *cube.Reader
	greet []cube.Message
}

func dial(t *testing.T, addr string) *client {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetDeadline(time.Now().Add(testkit.Scale(5 * time.Second)))
	c := &client{t: t, conn: conn, r: cube.NewReader(conn)}
	for {
		m := c.read()
		c.greet = append(c.greet, m)
		if m.Type == 'L' {
			return c
		}
	}
}

func (c *client) read() cube.Message {
	c.t.Helper()
	m, err := c.r.Read()
	if err != nil {
		c.t.Fatal(err)
	}
	return m
}

func (c *client) send(m cube.Message) cube.Message {
	c.t.Helper()
	if _, err := c.conn.Write([]byte(m.Wire())); err != nil {
		c.t.Fatal(err)
	}
	return c.read()
}

// listed reports whether an L line holds rf's entry.
func listed(t *testing.T, l cube.Message, rf cube.RFAddress) bool {
	t.Helper()
	states, err := cube.ParseDeviceList(l.Data, map[cube.RFAddress]cube.DeviceType{})
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range states {
		if st.RFAddress == rf {
			return true
		}
	}
	return false
}

// t: answers A: per address and deletes the devices: their C lines and
// L entries are gone, from a new connection's greeting too; a device in
// Keep stays, as one that never answers does on hardware.
func TestDeleteDevices(t *testing.T) {
	t.Parallel()
	greeting, err := LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	const gone, kept cube.RFAddress = 0x0a010a, 0x0a010d
	s := &Server{Greeting: greeting, Keep: map[cube.RFAddress]bool{kept: true}}
	addr, err := s.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	c := dial(t, addr)
	if a := c.send(cube.CmdDeleteDevices(true, gone, kept)); a.Type != 'A' {
		t.Fatalf("t: answered %s", a)
	}
	if a := c.read(); a.Type != 'A' {
		t.Fatalf("second answer %s, want A per address", a)
	}
	if got := c.send(cube.Message{Type: 'c', Data: gone.String()}); got.Type != 'A' {
		t.Fatalf("c: for the deleted device answered %s", got)
	}
	if got := c.send(cube.Message{Type: 'c', Data: kept.String()}); got.Type != 'C' {
		t.Fatalf("c: for the kept device answered %s", got)
	}
	l := c.send(cube.CmdList)
	if listed(t, l, gone) || !listed(t, l, kept) || !listed(t, l, 0x0a0108) {
		t.Fatalf("L after t: %s", l)
	}
	fresh := dial(t, addr)
	for _, m := range fresh.greet {
		if m.Type == 'C' && strings.HasPrefix(m.Data, gone.String()+",") {
			t.Fatal("the deleted device's C line in a new greeting")
		}
		if m.Type == 'L' && (listed(t, m, gone) || !listed(t, m, kept)) {
			t.Fatalf("L in a new greeting %s", m)
		}
	}
}

// The Pair device, deleted with t:, joins again with the next n:.
func TestDeletedPairDeviceJoinsAgain(t *testing.T) {
	t.Parallel()
	greeting, err := LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	const rf cube.RFAddress = 0x0a0b0c
	pair, err := PairingFrom(greeting, 0x0a010d, rf, "TEQ0000001", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Greeting: greeting, Pair: pair}
	addr, err := s.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	c := dial(t, addr)
	for round := 0; round < 2; round++ {
		if n := c.send(cube.CmdInclusion(time.Minute)); n.String() != NewDeviceLine(pair.Device).String() {
			t.Fatalf("round %d: %s, want the N line", round, n)
		}
		if got := c.send(cube.Message{Type: 'c', Data: rf.String()}); got.Type != 'C' {
			t.Fatalf("round %d: c: answered %s", round, got)
		}
		if n := c.send(cube.CmdCancelInclusion); n.Type != 'N' || n.Data != "" {
			t.Fatalf("round %d: x: answered %s", round, n)
		}
		if a := c.send(cube.CmdDeleteDevices(true, rf)); a.Type != 'A' {
			t.Fatalf("round %d: t: answered %s", round, a)
		}
		if got := c.send(cube.Message{Type: 'c', Data: rf.String()}); got.Type != 'A' {
			t.Fatalf("round %d: c: after t: answered %s", round, got)
		}
	}
}

// metadataOf is the room and device list of a greeting's M lines.
func metadataOf(t *testing.T, greeting []cube.Message) *cube.Metadata {
	t.Helper()
	var parts []cube.MetadataPart
	for _, m := range greeting {
		if m.Type == 'M' {
			p, err := cube.ParseMetadataPart(m.Data)
			if err != nil {
				t.Fatal(err)
			}
			parts = append(parts, p)
		}
	}
	md, err := cube.ParseMetadata(parts)
	if err != nil {
		t.Fatal(err)
	}
	return md
}

// greeted reports whether a greeting holds rf's C line and its entry in
// the L line.
func greeted(t *testing.T, greeting []cube.Message, rf cube.RFAddress) (config, entry bool) {
	t.Helper()
	for _, m := range greeting {
		switch m.Type {
		case 'C':
			config = config || strings.HasPrefix(m.Data, rf.String()+",")
		case 'L':
			entry = listed(t, m, rf)
		}
	}
	return config, entry
}

// The fake keeps what a Cube keeps across connections: a device that
// joined is in a new connection's greeting, its C line and its L entry,
// in no room, until a t: deletes it; an m: the fake acknowledged is the
// greeting's room and device list from then on, of one block or of
// several, until SetGreeting gives another.
func TestGreetingKeepsTheJoinedDeviceAndTheWrittenList(t *testing.T) {
	t.Parallel()
	greeting, err := LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	const rf cube.RFAddress = 0x0a0b0c
	pair, err := PairingFrom(greeting, 0x0a010d, rf, "TEQ0000001", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Greeting: greeting, Pair: pair}
	addr, err := s.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	c := dial(t, addr)
	if config, entry := greeted(t, c.greet, rf); config || entry {
		t.Fatalf("the device in the greeting before it joined: C line %v, L entry %v", config, entry)
	}
	if n := c.send(cube.CmdInclusion(time.Minute)); n.String() != NewDeviceLine(pair.Device).String() {
		t.Fatalf("n: brought %s, want the N line", n)
	}
	if n := c.send(cube.CmdCancelInclusion); n.Type != 'N' || n.Data != "" {
		t.Fatalf("x: answered %s", n)
	}
	if l := c.send(cube.CmdList); !listed(t, l, rf) {
		t.Fatalf("l: after the join: %s", l)
	}

	// A new connection: the device that joined, in no room.
	again := dial(t, addr)
	if config, entry := greeted(t, again.greet, rf); !config || !entry {
		t.Fatalf("the joined device in a new greeting: C line %v, L entry %v", config, entry)
	}
	if cfg := again.config(rf); cfg == nil || cfg.RoomID != 0 || cfg.Serial != "TEQ0000001" {
		t.Fatalf("its C line %+v", cfg)
	}
	md := metadataOf(t, again.greet)
	for _, d := range md.Devices {
		if d.RFAddress == rf {
			t.Fatal("the joined device in the recorded list")
		}
	}

	// An m: puts it into room 1: the next greeting has the list written.
	md.Devices = append(md.Devices, cube.DeviceMeta{Type: pair.Device.Type, RFAddress: rf, Serial: "TEQ0000001", Name: "Neu", RoomID: 1})
	cmds, err := cube.WriteMetadataCommands(md)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range cmds {
		if a := again.send(m); a.Type != 'A' {
			t.Fatalf("m:%s answered %s", m.Data[:2], a)
		}
	}
	written := dial(t, addr)
	got := metadataOf(t, written.greet)
	if len(got.Devices) != len(md.Devices) || got.Devices[len(got.Devices)-1] != md.Devices[len(md.Devices)-1] {
		t.Fatalf("the list after the m: %+v, want %+v", got.Devices, md.Devices)
	}
	if config, entry := greeted(t, written.greet, rf); !config || !entry {
		t.Fatalf("the device after the m:: C line %v, L entry %v", config, entry)
	}

	// A list of two blocks: both are the greeting's M lines.
	long := *md
	long.Devices = append([]cube.DeviceMeta(nil), md.Devices...)
	for i := range 50 {
		long.Devices = append(long.Devices, cube.DeviceMeta{Type: cube.DeviceShutterContact, RFAddress: cube.RFAddress(0x0a2000 + i),
			Serial: "IEQ0000001", Name: "Fenster " + strings.Repeat("x", 10), RoomID: 2})
	}
	if cmds, err = cube.WriteMetadataCommands(&long); err != nil || len(cmds) != 2 {
		t.Fatalf("%d blocks, %v; want 2", len(cmds), err)
	}
	for _, m := range cmds {
		if a := written.send(m); a.Type != 'A' {
			t.Fatalf("m:%s answered %s", m.Data[:2], a)
		}
	}
	two := dial(t, addr)
	var ms []string
	for _, m := range two.greet {
		if m.Type == 'M' {
			ms = append(ms, m.Data[:5])
		}
	}
	if strings.Join(ms, " ") != "00,02 01,02" || len(metadataOf(t, two.greet).Devices) != len(long.Devices) {
		t.Fatalf("the M lines of two blocks: %v", ms)
	}

	// t: deletes the device: it is gone from the greeting, and SetGreeting
	// brings back the recorded list.
	if a := two.send(cube.CmdDeleteDevices(true, rf)); a.Type != 'A' {
		t.Fatalf("t: answered %s", a)
	}
	gone := dial(t, addr)
	if config, entry := greeted(t, gone.greet, rf); config || entry {
		t.Fatalf("the deleted device in a new greeting: C line %v, L entry %v", config, entry)
	}
	s.SetGreeting(greeting)
	if got := metadataOf(t, dial(t, addr).greet); len(got.Devices) != len(metadataOf(t, greeting).Devices) {
		t.Fatalf("the list after SetGreeting: %d devices", len(got.Devices))
	}
}

// The device that joins after n: does not join once x: ended the
// inclusion before its delay, and a second n: makes it join once.
func TestPairDeviceJoinsOnlyWhileIncluding(t *testing.T) {
	t.Parallel()
	greeting, err := LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := PairingFrom(greeting, 0x0a010d, 0x0a0b0c, "TEQ0000001", 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Greeting: greeting, Pair: pair}
	addr, err := s.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	c := dial(t, addr)
	c.conn.Write([]byte(cube.CmdInclusion(time.Minute).Wire()))
	if n := c.send(cube.CmdCancelInclusion); n.Type != 'N' || n.Data != "" {
		t.Fatalf("x: answered %s", n)
	}
	// Within the delay and after: nothing more comes.
	c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if m, err := c.r.Read(); err == nil {
		t.Fatalf("after x: the fake sent %s", m)
	}
	c.conn.SetDeadline(time.Now().Add(testkit.Scale(5 * time.Second)))
	c.r = cube.NewReader(c.conn)
	// Two n: in a row: the device joins once.
	c.conn.Write([]byte(cube.CmdInclusion(time.Minute).Wire()))
	c.conn.Write([]byte(cube.CmdInclusion(time.Minute).Wire()))
	if n := c.read(); n.String() != NewDeviceLine(pair.Device).String() {
		t.Fatalf("n: brought %s, want the N line", n)
	}
	c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if m, err := c.r.Read(); err == nil {
		t.Fatalf("a second join: %s", m)
	}
}

// The fake reads n:'s time as a real Cube does, by its low byte: n:0101
// ends after a second with an empty N line, not after 257.
func TestInclusionTimeIsTheLowByte(t *testing.T) {
	greeting, err := LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Greeting: greeting}
	addr, err := s.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	c := dial(t, addr)
	start := time.Now()
	c.conn.Write([]byte("n:0101\r\n"))
	if n := c.read(); n.Type != 'N' || n.Data != "" {
		t.Fatalf("n:0101 brought %s, want the empty N line", n)
	}
	if took := time.Since(start); took < 900*time.Millisecond || took > 3*time.Second {
		t.Fatalf("the inclusion ended after %s", took)
	}
}

// OnReceive hands each hook the lines received before it, then each as
// it comes, in the order Received has them, before the fake answers the
// line; the lines of two clients at once come one at a time.
func TestOnReceive(t *testing.T) {
	t.Parallel()
	greeting, err := LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Greeting: greeting}
	addr, err := s.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	first, second := dial(t, addr), dial(t, addr)
	first.send(cube.CmdList)
	var mu sync.Mutex
	var a, b []string
	s.OnReceive(func(m cube.Message) { mu.Lock(); a = append(a, m.String()); mu.Unlock() })
	s.OnReceive(func(m cube.Message) { mu.Lock(); b = append(b, m.String()); mu.Unlock() })
	hooked := func() ([]string, []string) {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(a), slices.Clone(b)
	}
	if a, b := hooked(); !slices.Equal(a, []string{"l:"}) || !slices.Equal(b, a) {
		t.Fatalf("the line before the hooks: %q, %q", a, b)
	}
	// The answer comes after the hooks: no wait.
	first.send(cube.Message{Type: 'c', Data: "0a0101"})
	if a, _ := hooked(); !slices.Equal(a, []string{"l:", "c:0a0101"}) {
		t.Fatalf("at the answer: %q", a)
	}
	var wg sync.WaitGroup
	for _, c := range []*client{first, second} {
		wg.Go(func() {
			for range 20 {
				if _, err := c.conn.Write([]byte(cube.CmdList.Wire())); err != nil {
					t.Error(err)
					return
				}
				if _, err := c.r.Read(); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	var received []string
	for _, m := range s.Received() {
		received = append(received, m.String())
	}
	if a, b := hooked(); len(a) != 42 || !slices.Equal(a, received) || !slices.Equal(b, received) {
		t.Fatalf("hooked %d and %d lines, received %d, in another order", len(a), len(b), len(received))
	}
}
