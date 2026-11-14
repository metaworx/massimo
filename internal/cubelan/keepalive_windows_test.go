// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"net"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// With polling, the default dialer's connection retransmits for the
// hold and the time the loss takes to be marked (TCP_MAXRT, in seconds,
// Options.kernelHold), read back from the socket.
func TestDefaultDialerMaxRT(t *testing.T) {
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
	c := New(Options{Addr: ln.Addr().String(), PollInterval: time.Minute, Hold: 2 * time.Minute})
	conn, err := c.opts.Dialer(context.Background(), ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	raw, err := conn.(*net.TCPConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var got int
	var gerr error
	if err := raw.Control(func(fd uintptr) {
		got, gerr = windows.GetsockoptInt(windows.Handle(fd), windows.IPPROTO_TCP, windows.TCP_MAXRT)
	}); err != nil {
		t.Fatal(err)
	}
	if gerr != nil {
		t.Skipf("TCP_MAXRT cannot be read back here: %v", gerr)
	}
	if got != 265 { // the hold, two polls and their reply timeouts, a probe's interval
		t.Errorf("TCP_MAXRT = %d, want 265", got)
	}
}
