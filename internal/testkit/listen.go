// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"net"
	"testing"
)

// Listen returns a TCP listener on the loopback, at a port the system
// chose, open until the test ends, for the test to hand on: to a fake
// Cube (fakecube's Serve), to a product's listen hook. The port is the
// test's while the listener is open, so that nothing else takes it, as
// another program can between a listener closed and one opened on its
// port. A product that closes the listener it was handed frees the port;
// the test's end closes it where it did not.
func Listen(t testing.TB) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("testkit: listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln
}

// ListenPacket returns a UDP socket on the loopback's IPv4 address, at a
// port the system chose, open until the test ends: an address that
// takes datagrams and answers none, held for the test, or a socket to
// hand on, as Listen's listener.
func ListenPacket(t testing.TB) net.PacketConn {
	t.Helper()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("testkit: listen: %v", err)
	}
	t.Cleanup(func() { pc.Close() })
	return pc
}
