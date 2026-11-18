// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"errors"
	"net"
	"runtime"
	"syscall"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/testkit"
)

// startFake starts a fake with the recorded greeting on a listener the
// test holds.
func startFake(t *testing.T) (*Server, string) {
	t.Helper()
	g, err := LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{Greeting: g}
	addr := srv.Serve(testkit.Listen(t))
	t.Cleanup(func() { srv.Close() })
	return srv, addr
}

// paused reports whether srv is paused.
func paused(srv *Server) bool {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return srv.paused
}

// refusedAsPaused checks that a client is refused as a paused fake
// refuses it: on Linux its dial is refused, as where nothing listens;
// elsewhere its connection ends before any line.
func refusedAsPaused(t *testing.T, addr string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		endsBeforeAnyLine(t, addr, "while paused")
		return
	}
	conn, err := net.DialTimeout("tcp", addr, testkit.Scale(time.Second))
	if !errors.Is(err, syscall.ECONNREFUSED) {
		if conn != nil {
			conn.Close()
		}
		t.Fatalf("dial while paused: %v, want connection refused", err)
	}
}

// endsBeforeAnyLine checks that a client's connection ends, by a reset
// or a close, before the fake sends a line. A reset can come before the
// dial has seen its connection made, and fails the dial then.
func endsBeforeAnyLine(t *testing.T, addr, when string) {
	t.Helper()
	var ne net.Error
	conn, err := net.DialTimeout("tcp", addr, testkit.Scale(time.Second))
	if err != nil {
		if errors.As(err, &ne) && ne.Timeout() {
			t.Fatalf("dial %s: %v, want a connection that ends", when, err)
		}
		return
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(testkit.Scale(5 * time.Second)))
	n, err := conn.Read(make([]byte, 64))
	if n > 0 || err == nil || errors.As(err, &ne) && ne.Timeout() {
		t.Fatalf("read %s: %d bytes, %v; want the connection's end", when, n, err)
	}
}

// refusedWhileTaken checks that a client is refused as a SingleClient
// fake refuses one while its slot is taken: on Linux its dial times
// out, as against the Cube; elsewhere its connection ends before any
// line.
func refusedWhileTaken(t *testing.T, addr string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		endsBeforeAnyLine(t, addr, "while the slot is taken")
		return
	}
	conn, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		if conn != nil {
			conn.Close()
		}
		t.Fatalf("dial while the slot is taken: %v, want a timeout", err)
	}
}

// keepsItsPort checks that nothing else can listen on the fake's
// address.
func keepsItsPort(t *testing.T, addr string) {
	t.Helper()
	if ln, err := net.Listen("tcp", addr); err == nil {
		ln.Close()
		t.Fatalf("another listener took %s", addr)
	}
}

// A paused fake drops its client, refuses the next and keeps its port;
// resumed, it greets again on the same address, a second time too.
func TestPauseAndResume(t *testing.T) {
	srv, addr := startFake(t)
	first := dial(t, addr)
	for round := range 2 {
		srv.Pause()
		srv.Pause() // a second does nothing
		first.conn.SetReadDeadline(time.Now().Add(testkit.Scale(5 * time.Second)))
		if _, err := first.r.Read(); err == nil {
			t.Fatalf("round %d: a client of the paused fake read a line", round)
		}
		refusedAsPaused(t, addr)
		keepsItsPort(t, addr)
		if err := srv.Resume(); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		if err := srv.Resume(); err != nil { // a second does nothing
			t.Fatalf("round %d, second Resume: %v", round, err)
		}
		first = dial(t, addr)
	}
	// Three clients greeted, none of those the pause refused.
	if n := srv.Connections(); n != 3 {
		t.Errorf("%d connections, want 3", n)
	}
}

// A fake paused before it serves starts paused, and greets once
// resumed; a paused fake closes without an error, and a Resume after
// the Close does nothing.
func TestPausedFromTheStart(t *testing.T) {
	g, err := LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{Greeting: g}
	srv.Pause()
	addr := srv.Serve(testkit.Listen(t))
	refusedAsPaused(t, addr)
	keepsItsPort(t, addr)
	if err := srv.Resume(); err != nil {
		t.Fatal(err)
	}
	dial(t, addr)
	srv.Pause()
	if err := srv.Close(); err != nil {
		t.Fatalf("Close while paused: %v", err)
	}
	if err := srv.Resume(); err != nil || !paused(srv) {
		t.Fatalf("Resume after Close: %v, paused %v", err, paused(srv))
	}
}

// wrapped is a listener holdPort does not know, so that a paused fake
// keeps it open and resets what it accepts, as outside Linux, and
// setBacklog cannot cut its backlog.
type wrapped struct{ net.Listener }

// A fake whose port cannot be held while its listener is closed keeps
// the listener: paused, it resets each connection before any line;
// resumed, it greets again. A SingleClient one refuses a second client
// by a close then.
func TestPauseKeepingTheListener(t *testing.T) {
	for _, single := range []bool{false, true} {
		t.Run(map[bool]string{false: "any clients", true: "single client"}[single], func(t *testing.T) {
			g, err := LoadGreeting(fixture)
			if err != nil {
				t.Fatal(err)
			}
			srv := &Server{Greeting: g, SingleClient: single}
			addr := srv.Serve(wrapped{testkit.Listen(t)})
			t.Cleanup(func() { srv.Close() })
			first := dial(t, addr)
			srv.Pause()
			first.conn.SetReadDeadline(time.Now().Add(testkit.Scale(5 * time.Second)))
			if _, err := first.r.Read(); err == nil {
				t.Fatal("a client of the paused fake read a line")
			}
			endsBeforeAnyLine(t, addr, "while paused")
			endsBeforeAnyLine(t, addr, "while paused, the second")
			keepsItsPort(t, addr)
			if err := srv.Resume(); err != nil {
				t.Fatal(err)
			}
			dial(t, addr)
			if single {
				endsBeforeAnyLine(t, addr, "beside the connected client")
			}
		})
	}
}

// A SingleClient fake that holds a dead session keeps it across a pause:
// it refuses while paused as any paused fake does, and once resumed as
// it did before the pause, until the session is released.
func TestPauseKeepsADeadSession(t *testing.T) {
	srv, addr := startSingle(t, time.Hour)
	first := dial(t, addr)
	srv.SetSilent(true)
	first.conn.Close()
	testkit.Eventually(t, "the dead session", srv.HeldDead)
	srv.SetSilent(false)
	refusedWhileTaken(t, addr)
	srv.Pause()
	refusedAsPaused(t, addr)
	if err := srv.Resume(); err != nil {
		t.Fatal(err)
	}
	if !srv.HeldDead() {
		t.Fatal("the pause ended the dead session")
	}
	refusedWhileTaken(t, addr)
	srv.Release()
	testkit.Eventually(t, "the slot freed", func() bool { return !refused(addr) })
	dial(t, addr)
}
