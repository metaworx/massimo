// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"net"
	"testing"

	"golang.org/x/sys/unix"
)

// The connection the default dialer makes carries keepAlive: on, 15 s
// idle, 5 s between probes, 3 probes, read back from the socket.
func TestDefaultDialerKeepAlive(t *testing.T) {
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
	c := New(Options{Addr: ln.Addr().String()})
	conn, err := c.opts.Dialer(context.Background(), ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	raw, err := conn.(*net.TCPConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	var serr error
	if err := raw.Control(func(fd uintptr) {
		for name, opt := range map[string][2]int{
			"SO_KEEPALIVE":  {unix.SOL_SOCKET, unix.SO_KEEPALIVE},
			"TCP_KEEPIDLE":  {unix.IPPROTO_TCP, unix.TCP_KEEPIDLE},
			"TCP_KEEPINTVL": {unix.IPPROTO_TCP, unix.TCP_KEEPINTVL},
			"TCP_KEEPCNT":   {unix.IPPROTO_TCP, unix.TCP_KEEPCNT},
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
	want := map[string]int{"SO_KEEPALIVE": 1, "TCP_KEEPIDLE": 15, "TCP_KEEPINTVL": 5, "TCP_KEEPCNT": 3}
	for name, v := range want {
		if got[name] != v {
			t.Errorf("%s = %d, want %d", name, got[name], v)
		}
	}
}
