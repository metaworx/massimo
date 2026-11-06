// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"net"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
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
	conn.SetDeadline(time.Now().Add(5 * time.Second))
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
	greeting, err := LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	const gone, kept cube.RFAddress = 0x14d08e, 0x1514b6
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
	if listed(t, l, gone) || !listed(t, l, kept) || !listed(t, l, 0x14ce3d) {
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
	greeting, err := LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	const rf cube.RFAddress = 0x0a0b0c
	pair, err := PairingFrom(greeting, 0x1514b6, rf, "TEQ0000001", time.Millisecond)
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
