// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"errors"
	"net"
	"syscall"
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

// BindOrSkip ends a test whose listen failed with err: it skips the test
// where the platform lacks the address, as macOS has 127.0.0.1 alone of
// the loopback's addresses, and fails it on any other error, such as a
// port another program holds, where the platform has what the test
// needs: a skip there would leave the run green with the test not run.
// A nil err ends nothing.
func BindOrSkip(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		return
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && (errno == syscall.EADDRNOTAVAIL || errno == wsaeAddrNotAvail) {
		t.Skipf("testkit: the platform lacks the address: %v", err)
	}
	t.Fatalf("testkit: listen: %v", err)
}

// wsaeAddrNotAvail is Windows' WSAEADDRNOTAVAIL, which package syscall
// does not name there.
const wsaeAddrNotAvail = syscall.Errno(10049)
