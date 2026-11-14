// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"net"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
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

// within waits up to five seconds for cond.
func within(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

// A SingleClient fake refuses a second client while one is connected,
// and takes the next once that one has left.
func TestSingleClientRefusesASecond(t *testing.T) {
	srv, addr := startSingle(t, 0)
	first := dial(t, addr)
	if !refused(addr) {
		t.Fatal("a second client was served while the first is connected")
	}
	first.conn.Close()
	within(t, "the slot freed", func() bool { return !refused(addr) })
	if srv.HeldDead() {
		t.Error("a client that left while the fake spoke left a dead session")
	}
}

// A client that leaves while the fake is silent leaves a dead session:
// the fake refuses every client until HoldDead has passed.
func TestADeadSessionIsHeldUntilHoldDead(t *testing.T) {
	srv, addr := startSingle(t, 300*time.Millisecond)
	first := dial(t, addr)
	srv.SetSilent(true)
	first.conn.Close()
	within(t, "the dead session", srv.HeldDead)
	srv.SetSilent(false)
	if !refused(addr) {
		t.Fatal("a client was served while the dead session is held")
	}
	within(t, "the end of the dead session", func() bool { return !srv.HeldDead() })
	dial(t, addr)
}

// Release and Restart, the fake's answer to an R, forget a dead session
// at once; Restart also ends the connections.
func TestReleaseAndRestartFreeTheCube(t *testing.T) {
	srv, addr := startSingle(t, time.Hour)
	for _, free := range []func(){srv.Release, srv.Restart} {
		c := dial(t, addr)
		srv.SetSilent(true)
		c.conn.Close()
		within(t, "the dead session", srv.HeldDead)
		srv.SetSilent(false)
		free()
		if srv.HeldDead() {
			t.Fatal("the dead session is still held")
		}
		within(t, "a client served again", func() bool { return !refused(addr) })
	}
	c := dial(t, addr)
	srv.Restart()
	c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.r.Read(); err == nil {
		t.Error("the connection outlived the restart")
	}
	if srv.HeldDead() || srv.Restarts() != 2 {
		t.Errorf("after the restart: dead %v, %d restarts", srv.HeldDead(), srv.Restarts())
	}
}

// The discovery responder's R restarts the TCP fake it is joined to.
func TestAnRRestartsTheJoinedFake(t *testing.T) {
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
	within(t, "the dead session", srv.HeldDead)
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
	within(t, "the restart", func() bool { return srv.Restarts() == 1 && !srv.HeldDead() })
	if d.Reboots() != 1 {
		t.Errorf("%d reboots counted", d.Reboots())
	}
	dial(t, addr)
}
