// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"net"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The connection the default dialer makes carries keepAlive: on, 15 s
// idle, 5 s between probes, 3 probes, read back from the socket; with
// polling, TCP_USER_TIMEOUT covers the hold and the time the loss takes
// to be marked (Options.kernelHold), and without, none.
func TestDefaultDialerKeepAlive(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				c.Read(make([]byte, 1))
			}()
		}
	}()
	for _, tc := range []struct {
		name string
		o    Options
		user int
	}{
		// The hold, two polls and their reply timeouts before it, and a
		// probe's interval.
		{"polling", Options{PollInterval: time.Minute}, int((DefaultHold + 2*70*time.Second + 5*time.Second).Milliseconds())},
		{"a hold of its own", Options{PollInterval: time.Minute, Hold: 2 * time.Minute}, 265000},
		{"no polling", Options{}, 0},
	} {
		tc.o.Addr = ln.Addr().String()
		c := New(tc.o)
		conn, err := c.opts.Dialer(context.Background(), ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		got := sockopts(t, conn)
		conn.Close()
		want := map[string]int{"SO_KEEPALIVE": 1, "TCP_KEEPIDLE": 15, "TCP_KEEPINTVL": 5, "TCP_KEEPCNT": 3, "TCP_USER_TIMEOUT": tc.user}
		for name, v := range want {
			if got[name] != v {
				t.Errorf("%s: %s = %d, want %d", tc.name, name, got[name], v)
			}
		}
	}
}

// sockopts reads the keepalive's options and TCP_USER_TIMEOUT.
func sockopts(t *testing.T, conn net.Conn) map[string]int {
	t.Helper()
	raw, err := conn.(*net.TCPConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	var serr error
	if err := raw.Control(func(fd uintptr) {
		for name, opt := range map[string][2]int{
			"SO_KEEPALIVE":     {unix.SOL_SOCKET, unix.SO_KEEPALIVE},
			"TCP_KEEPIDLE":     {unix.IPPROTO_TCP, unix.TCP_KEEPIDLE},
			"TCP_KEEPINTVL":    {unix.IPPROTO_TCP, unix.TCP_KEEPINTVL},
			"TCP_KEEPCNT":      {unix.IPPROTO_TCP, unix.TCP_KEEPCNT},
			"TCP_USER_TIMEOUT": {unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT},
		} {
			v, err := unix.GetsockoptInt(int(fd), opt[0], opt[1])
			if err != nil {
				serr = err
				return
			}
			got[name] = v
		}
	}); err != nil {
		t.Fatal(err)
	}
	if serr != nil {
		t.Fatal(serr)
	}
	return got
}

// keepaliveSilent reads the kernel's count of unanswered probes: none on
// a connection whose peer is alive.
func TestKeepaliveSilentOnALiveConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			defer c.Close()
			c.Read(make([]byte, 1))
		}
	}()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if keepaliveSilent(conn) {
		t.Error("a live connection's probes counted as unanswered")
	}
}
