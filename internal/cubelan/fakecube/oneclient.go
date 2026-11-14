// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"net"
	"strconv"
	"time"
)

// DefaultHoldDead is how long a Cube kept a session whose end it never
// saw: 41½ minutes on 2026-11-13 (firmware 1.1.3), so 41 minutes.
const DefaultHoldDead = 41 * time.Minute

// The one-client rule of SingleClient: a Cube serves one client at a
// time and refuses every other while it has one, a session it still
// believes open included. On hardware it answers the second client's
// SYN with a reset that Linux ignores, so that client's dial times out
// (docs/cube-protocol.md, "Lessons from the hardware days"). The fake
// does the same on Linux: while the slot is taken it stops accepting,
// cuts its listener's backlog to 0 and parks a connection of its own in
// the accept queue, so the kernel drops the next SYN. Elsewhere it
// accepts the second client and closes it at once.

// occupiedLocked reports whether the slot is taken: a client is
// connected, or a dead session is held. s.mu is held.
func (s *Server) occupiedLocked() bool {
	return len(s.open) > 0 || !s.deadUntil.IsZero()
}

// changedLocked follows a change of the slot: once it is free again the
// listener takes connections again, and whoever waits in waitFree is
// woken. s.mu is held.
func (s *Server) changedLocked() {
	if s.refusing && !s.occupiedLocked() {
		s.refusing = false
		setBacklog(s.ln, fullBacklog)
		if s.park != nil {
			s.park.Close()
			s.park = nil
		}
	}
	if s.freed != nil {
		close(s.freed)
	}
	s.freed = make(chan struct{})
}

// refuseLocked makes the listener drop new clients' SYNs while the slot
// is taken, where the platform allows it (dropsSYN). s.mu is held.
func (s *Server) refuseLocked() {
	if !dropsSYN || s.refusing {
		return
	}
	if err := setBacklog(s.ln, 0); err != nil {
		return
	}
	s.refusing = true
	// One connection of our own in the accept queue fills it: with a
	// backlog of 0 the kernel then drops every further SYN. A client
	// that got in first fills it as well, and the dial fails then.
	for _, a := range dialable(s.ln.Addr()) {
		if c, err := net.DialTimeout("tcp", a, 500*time.Millisecond); err == nil {
			s.park, s.parkAddr = c, c.LocalAddr().String()
			return
		}
	}
}

// waitFree waits until the slot is free; false once the server closed.
func (s *Server) waitFree() bool {
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return false
		}
		if !s.occupiedLocked() {
			s.mu.Unlock()
			return true
		}
		if s.freed == nil {
			s.freed = make(chan struct{})
		}
		ch := s.freed
		s.mu.Unlock()
		<-ch
	}
}

// holdDeadLocked keeps the session that just ended as the Cube keeps
// one whose FIN the outage lost: the slot stays taken for HoldDead, or
// until Release or Restart. s.mu is held.
func (s *Server) holdDeadLocked() {
	d := s.HoldDead
	if d <= 0 {
		d = DefaultHoldDead
	}
	if s.deadTimer != nil {
		s.deadTimer.Stop()
	}
	s.deadUntil = time.Now().Add(d)
	s.deadTimer = time.AfterFunc(d, s.Release)
}

// Release forgets a dead session the fake holds, as the Cube lets go of
// one after its own timeout, and takes clients again.
func (s *Server) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deadTimer != nil {
		s.deadTimer.Stop()
		s.deadTimer = nil
	}
	if s.deadUntil.IsZero() {
		return
	}
	s.deadUntil = time.Time{}
	s.changedLocked()
}

// Restart behaves as the Cube does on a UDP R: every connection ends
// and a dead session is forgotten (DiscoveryResponder.SetOnRestart).
func (s *Server) Restart() {
	s.mu.Lock()
	for c := range s.open {
		c.Close()
	}
	s.restarts++
	s.mu.Unlock()
	s.Release()
}

// HeldDead reports whether the fake holds a dead session.
func (s *Server) HeldDead() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.deadUntil.IsZero()
}

// Restarts counts the restarts (Restart) the fake has taken.
func (s *Server) Restarts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restarts
}

// isParkLocked reports whether conn is the fake's own parked connection,
// accepted once the backlog was restored. s.mu is held.
func (s *Server) isParkLocked(conn net.Conn) bool {
	return s.parkAddr != "" && conn.RemoteAddr().String() == s.parkAddr
}

// dialable are the addresses to dial a listener at: its own, or for an
// unspecified IP the IPv4 loopback, which a dual-stack listener on [::]
// takes too, then the IPv6 one, which a container may lack.
func dialable(a net.Addr) []string {
	t, ok := a.(*net.TCPAddr)
	if !ok {
		return []string{a.String()}
	}
	port := strconv.Itoa(t.Port)
	if t.IP == nil || t.IP.IsUnspecified() {
		return []string{net.JoinHostPort("127.0.0.1", port), net.JoinHostPort("::1", port)}
	}
	return []string{net.JoinHostPort(t.IP.String(), port)}
}
