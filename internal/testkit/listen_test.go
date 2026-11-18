// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"testing"
	"time"
)

// Listen's listener takes a connection, holds its port against another
// listener while the test runs, and is closed at the test's end.
func TestListen(t *testing.T) {
	var ln net.Listener
	t.Run("held", func(t *testing.T) {
		ln = Listen(t)
		if other, err := net.Listen("tcp", ln.Addr().String()); err == nil {
			other.Close()
			t.Fatalf("a second listener took %s", ln.Addr())
		}
		conn, err := net.DialTimeout("tcp", ln.Addr().String(), Scale(time.Second))
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		got, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		got.Close()
	})
	if _, err := ln.Accept(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Accept after the test's end: %v, want the listener closed", err)
	}
}

// ListenPacket's socket takes a datagram, holds its port against another
// socket while the test runs, and is closed at the test's end.
func TestListenPacket(t *testing.T) {
	var pc net.PacketConn
	t.Run("held", func(t *testing.T) {
		pc = ListenPacket(t)
		if other, err := net.ListenPacket("udp4", pc.LocalAddr().String()); err == nil {
			other.Close()
			t.Fatalf("a second socket took %s", pc.LocalAddr())
		}
		send, err := net.Dial("udp4", pc.LocalAddr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer send.Close()
		if _, err := send.Write([]byte("ping")); err != nil {
			t.Fatal(err)
		}
		pc.SetReadDeadline(time.Now().Add(Scale(5 * time.Second)))
		buf := make([]byte, 8)
		if n, _, err := pc.ReadFrom(buf); err != nil || string(buf[:n]) != "ping" {
			t.Fatalf("read %q, %v", buf[:n], err)
		}
	})
	if _, _, err := pc.ReadFrom(make([]byte, 8)); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("ReadFrom after the test's end: %v, want the socket closed", err)
	}
}

// bindRecorder is a recorder that records a skip too: its Skipf ends the
// goroutine it runs in, as the real one does.
type bindRecorder struct {
	recorder
	skipped string
}

func (r *bindRecorder) Skipf(format string, args ...any) {
	r.skipped = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// BindOrSkip skips a test whose listen met an address the platform
// lacks, fails one whose listen failed otherwise, as at a port taken,
// and ends nothing without an error.
func TestBindOrSkip(t *testing.T) {
	held := Listen(t)
	_, inUse := net.Listen("tcp", held.Addr().String())
	if inUse == nil {
		t.Fatalf("a second listener took %s", held.Addr())
	}
	// 192.0.2.1 (TEST-NET-1) is no interface's address.
	lacked, lacking := net.ListenPacket("udp4", "192.0.2.1:0")
	if lacking == nil {
		lacked.Close()
		t.Skip("this host has 192.0.2.1")
	}
	windows := &net.OpError{Op: "listen", Net: "udp4", Err: os.NewSyscallError("bind", wsaeAddrNotAvail)}
	for _, c := range []struct {
		name            string
		err             error
		skipped, failed bool
	}{
		{"an address no interface has", lacking, true, false},
		{"Windows' address not available", windows, true, false},
		{"a port taken", inUse, false, true},
		{"no error", nil, false, false},
	} {
		r := &bindRecorder{recorder: recorder{TB: t}}
		done := make(chan struct{})
		go func() {
			defer close(done)
			BindOrSkip(r, c.err)
		}()
		<-done
		if (r.skipped != "") != c.skipped || (r.failed != "") != c.failed {
			t.Errorf("%s (%v): skipped %q, failed %q", c.name, c.err, r.skipped, r.failed)
		}
	}
}
