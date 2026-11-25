// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"flag"
	"io"
	"net"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// -pair names a copy of a device of the greeting, joining after two
// seconds unless a delay is given; anything else is refused.
func TestParsePair(t *testing.T) {
	t.Parallel()
	greeting, err := fakecube.LoadGreeting("../../internal/protocol/cube/testdata/cube-fw0113-greeting.txt")
	if err != nil {
		t.Fatal(err)
	}
	p, err := parsePair(greeting, "0a010d,0abcde,TEQ0000001")
	if err != nil || p.Device.RFAddress != 0x0abcde || p.Device.Serial != "TEQ0000001" || p.Device.Type != cube.DeviceRadiatorThermostat || p.Delay != 2*time.Second {
		t.Fatalf("%+v %v", p, err)
	}
	if p, err := parsePair(greeting, "0a010d,0abcde,TEQ0000001,500ms"); err != nil || p.Delay != 500*time.Millisecond {
		t.Fatalf("with a delay: %+v %v", p, err)
	}
	for _, bad := range []string{"0a010d,0abcde", "0a010d,0abcde,TEQ0000001,soon", "0f0f0f,0abcde,TEQ0000001", "0a010d,xyz,TEQ0000001", "0a010d,0abcde,TEQ00000012"} {
		if _, err := parsePair(greeting, bad); err == nil {
			t.Errorf("-pair %s accepted", bad)
		}
	}
}

// -free-slots gives the fake the free memory slots of its S replies and
// of its greeting's H line; without it the S replies say 0x31 and the H
// line the greeting's own (50 in this greeting); a value that is no
// byte is refused.
func TestFreeSlotsFlag(t *testing.T) {
	t.Parallel()
	greeting, err := fakecube.LoadGreeting("../../internal/protocol/cube/testdata/cube-fw0113-greeting.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args         []string
		hello, reply int
	}{
		{nil, 50, 0x31},
		{[]string{"-free-slots", "33"}, 33, 33},
		{[]string{"-free-slots=50"}, 50, 50},
		{[]string{"-free-slots", "0"}, 0, 0},
	} {
		fs := flag.NewFlagSet("fakecube", flag.ContinueOnError)
		set := freeSlotsFlag(fs)
		if err := fs.Parse(c.args); err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		srv := &fakecube.Server{Greeting: greeting}
		set(srv)
		addr, err := srv.Start("127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { srv.Close() })
		hello, reply := freeSlotsSaid(t, addr)
		if hello != c.hello || reply != c.reply {
			t.Errorf("%v: the H line says %d free, the S reply %d; want %d and %d", c.args, hello, reply, c.hello, c.reply)
		}
	}
	for _, bad := range []string{"-1", "256", "many"} {
		fs := flag.NewFlagSet("fakecube", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		freeSlotsFlag(fs)
		if err := fs.Parse([]string{"-free-slots", bad}); err == nil {
			t.Errorf("-free-slots %s accepted", bad)
		}
	}
}

// freeSlotsSaid connects to the fake at addr and returns the free memory
// slots its greeting's H line says and those of its S reply to an r:.
func freeSlotsSaid(t *testing.T, addr string) (hello, reply int) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(testkit.Scale(5 * time.Second)))
	r := cube.NewReader(conn)
	hello = -1
	for {
		m, err := r.Read()
		if err != nil {
			t.Fatal(err)
		}
		if m.Type == 'H' {
			h, err := cube.ParseHello(m.Data)
			if err != nil {
				t.Fatal(err)
			}
			hello = h.FreeMemorySlots
		}
		if m.Type == 'L' {
			break
		}
	}
	if _, err := conn.Write([]byte(cube.CmdResetError(0x0a010d).Wire())); err != nil {
		t.Fatal(err)
	}
	m, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	s, err := cube.ParseSendResult(m.Data)
	if err != nil || m.Type != 'S' {
		t.Fatalf("the reply to r: %v %v", m, err)
	}
	return hello, s.FreeMemorySlots
}
