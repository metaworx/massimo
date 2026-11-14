// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd || windows

package sharedport

import (
	"net"
	"runtime"
	"strconv"
	"testing"
	"time"
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

// A dual-stack socket on a shared port leaves the unicast datagrams to
// an IPv4 socket beside it, whichever was bound first: serve's
// discovery socket is dual-stack, so that the
// original software's IPv4 socket gets the Cubes' answers while it
// searches (Windows and Linux; Linux with SO_REUSEPORT).
func TestDualStackLeavesUnicastToIPv4(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("measured on Linux and Windows only")
	}
	for _, dualFirst := range []bool{true, false} {
		var dual, v4 net.PacketConn
		var err error
		if dualFirst {
			if dual, err = ListenPacket("udp", "[::]:0"); err != nil {
				t.Skipf("no dual-stack socket: %v", err)
			}
			t.Cleanup(func() { dual.Close() })
			v4 = shared(t, "0.0.0.0:"+port(dual))
		} else {
			v4 = shared(t, "0.0.0.0:0")
			if dual, err = ListenPacket("udp", "[::]:"+port(v4)); err != nil {
				t.Skipf("no dual-stack socket: %v", err)
			}
			t.Cleanup(func() { dual.Close() })
		}
		if a, ok := dual.LocalAddr().(*net.UDPAddr); !ok || a.IP.To4() != nil {
			t.Skipf("no IPv6: %v", dual.LocalAddr())
		}
		send, err := net.ListenPacket("udp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		to := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: v4.LocalAddr().(*net.UDPAddr).Port}
		const n = 20
		for i := 0; i < n; i++ {
			if _, err := send.WriteTo([]byte{byte(i)}, to); err != nil {
				t.Fatal(err)
			}
		}
		send.Close()
		if got := count(v4); got != n {
			t.Errorf("dual-stack first %v: the IPv4 socket got %d of %d", dualFirst, got, n)
		}
		if got := count(dual); got != 0 {
			t.Errorf("dual-stack first %v: the dual-stack socket got %d", dualFirst, got)
		}
		dual.Close()
		v4.Close()
	}
}

// port is the port pc is bound to.
func port(pc net.PacketConn) string {
	return strconv.Itoa(pc.LocalAddr().(*net.UDPAddr).Port)
}

// count reads what pc has until it is quiet for 200 ms.
func count(pc net.PacketConn) int {
	n := 0
	buf := make([]byte, 16)
	for {
		pc.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		if _, _, err := pc.ReadFrom(buf); err != nil {
			return n
		}
		n++
	}
}
