// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"errors"
	"net"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// startSingle starts a SingleClient fake with the recorded greeting.
func startSingle(t *testing.T, holdDead time.Duration) (*Server, string) {
	t.Helper()
	g, err := LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{Greeting: g, SingleClient: true, HoldDead: holdDead}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	return srv, addr
}

// refused reports whether a client is refused: its dial fails, as on
// Linux, or the connection ends without a greeting, as elsewhere.
func refused(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return true
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, err = cube.NewReader(conn).Read()
	return err != nil
}

// dialWhenFree dials addr until the fake serves the connection, its
// greeting read to the L line, and keeps that connection as the client.
// A probe that found the slot free (refused) holds it until the fake
// has seen it closed, and on Windows, where the fake ends a connection
// the taken slot cannot serve, a dial right after the probe can end
// before its greeting.
func dialWhenFree(t *testing.T, addr, what string) *client {
	t.Helper()
	var c *client
	testkit.Eventually(t, what, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err != nil {
			return false
		}
		conn.SetDeadline(time.Now().Add(testkit.Scale(5 * time.Second)))
		r := cube.NewReader(conn)
		var greet []cube.Message
		for {
			m, err := r.Read()
			if err != nil {
				conn.Close()
				return false
			}
			greet = append(greet, m)
			if m.Type == 'L' {
				break
			}
		}
		t.Cleanup(func() { conn.Close() })
		c = &client{t: t, conn: conn, r: r, greet: greet}
		return true
	})
	return c
}

// A SingleClient fake refuses a second client while one is connected,
// and takes the next once that one has left.
func TestSingleClientRefusesASecond(t *testing.T) {
	t.Parallel()
	srv, addr := startSingle(t, 0)
	first := dial(t, addr)
	if !refused(addr) {
		t.Fatal("a second client was served while the first is connected")
	}
	first.conn.Close()
	testkit.Eventually(t, "the slot freed", func() bool { return !refused(addr) })
	if srv.HeldDead() {
		t.Error("a client that left while the fake spoke left a dead session")
	}
}

// A client that leaves while the fake is silent leaves a dead session:
// the fake refuses every client until HoldDead has passed.
func TestADeadSessionIsHeldUntilHoldDead(t *testing.T) {
	t.Parallel()
	srv, addr := startSingle(t, 300*time.Millisecond)
	first := dial(t, addr)
	srv.SetSilent(true)
	first.conn.Close()
	testkit.Eventually(t, "the dead session", srv.HeldDead)
	srv.SetSilent(false)
	if !refused(addr) {
		t.Fatal("a client was served while the dead session is held")
	}
	testkit.Eventually(t, "the end of the dead session", func() bool { return !srv.HeldDead() })
	dial(t, addr)
}

// Release and Restart, the fake's answer to an R, forget a dead session
// at once; Restart also ends the connections.
func TestReleaseAndRestartFreeTheCube(t *testing.T) {
	t.Parallel()
	srv, addr := startSingle(t, time.Hour)
	c := dial(t, addr)
	for _, free := range []func(){srv.Release, srv.Restart} {
		srv.SetSilent(true)
		c.conn.Close()
		testkit.Eventually(t, "the dead session", srv.HeldDead)
		srv.SetSilent(false)
		free()
		if srv.HeldDead() {
			t.Fatal("the dead session is still held")
		}
		// the client served is the next round's, and the last one's the
		// restart's below
		c = dialWhenFree(t, addr, "a client served again")
	}
	srv.Restart()
	// The read ends with the close; one that times out found the
	// connection open.
	c.conn.SetReadDeadline(time.Now().Add(testkit.Scale(testkit.Wait)))
	var ne net.Error
	if _, err := c.r.Read(); err == nil || errors.As(err, &ne) && ne.Timeout() {
		t.Errorf("the connection outlived the restart: %v", err)
	}
	if srv.HeldDead() || srv.Restarts() != 2 {
		t.Errorf("after the restart: dead %v, %d restarts", srv.HeldDead(), srv.Restarts())
	}
}

// The discovery responder's R restarts the TCP fake it is joined to.
func TestAnRRestartsTheJoinedFake(t *testing.T) {
	t.Parallel()
	srv, addr := startSingle(t, time.Hour)
	d, err := StartDiscovery("127.0.0.1:0", "KEQ0000100", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.SetOnRestart(srv.Restart)
	c := dial(t, addr)
	srv.SetSilent(true)
	c.conn.Close()
	testkit.Eventually(t, "the dead session", srv.HeldDead)
	srv.SetSilent(false)

	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	to, _ := net.ResolveUDPAddr("udp4", d.Addr())
	frame := append([]byte("eQ3Max*\x00KEQ0000100"), 'R')
	if _, err := pc.WriteTo(frame, to); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the restart", func() bool { return srv.Restarts() == 1 && !srv.HeldDead() })
	if d.Reboots() != 1 {
		t.Errorf("%d reboots counted", d.Reboots())
	}
	dial(t, addr)
}
