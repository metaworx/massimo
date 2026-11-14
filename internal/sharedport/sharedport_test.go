// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd || windows

package sharedport

import (
	"net"
	"testing"
)

// shared opens a shared socket on the loopback, at port "0" or one given.
func shared(t *testing.T, addr string) net.PacketConn {
	t.Helper()
	pc, err := ListenPacket("udp4", addr)
	if err != nil {
		t.Fatalf("shared bind %s: %v", addr, err)
	}
	t.Cleanup(func() { pc.Close() })
	return pc
}

// Two shared sockets bind one port.
func TestTwoSharedSocketsBindOnePort(t *testing.T) {
	first := shared(t, "127.0.0.1:0")
	shared(t, first.LocalAddr().String())
}

// A plain socket cannot join a shared port: the command line's binds
// stay plain, and fall back to another port beside serve.
func TestPlainAfterSharedIsRefused(t *testing.T) {
	first := shared(t, "127.0.0.1:0")
	if pc, err := net.ListenPacket("udp4", first.LocalAddr().String()); err == nil {
		pc.Close()
		t.Fatal("a plain socket bound a shared port")
	}
}

// A port bound plain stays its socket's: serve then has no discovery
// port, as before.
func TestSharedAfterPlainIsRefused(t *testing.T) {
	first, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if pc, err := ListenPacket("udp4", first.LocalAddr().String()); err == nil {
		pc.Close()
		t.Fatal("a shared socket bound a port held plain")
	}
}
