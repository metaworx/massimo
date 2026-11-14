// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package proxy

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
)

// fakePacket is a datagram a fakeConn hands to its reader.
type fakePacket struct {
	b    []byte
	from *net.UDPAddr
}

// fakeWrite is a datagram written to a fakeConn.
type fakeWrite struct {
	at time.Time
	b  []byte
	to net.Addr
}

// fakeConn is a PacketConn whose reads are scripted and whose writes
// are recorded, so that the responder's tests need no network.
type fakeConn struct {
	local *net.UDPAddr
	in    chan fakePacket
	done  chan struct{}
	once  sync.Once

	mu     sync.Mutex
	writes []fakeWrite
}

func newFakeConn() *fakeConn {
	return &fakeConn{local: &net.UDPAddr{IP: net.IPv4zero, Port: cubelan.DiscoveryPort}, in: make(chan fakePacket, 16), done: make(chan struct{})}
}

func (c *fakeConn) ReadFrom(b []byte) (int, net.Addr, error) {
	select {
	case p := <-c.in:
		return copy(b, p.b), p.from, nil
	case <-c.done:
		return 0, nil, net.ErrClosed
	}
}

func (c *fakeConn) WriteTo(b []byte, to net.Addr) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes = append(c.writes, fakeWrite{time.Now(), append([]byte(nil), b...), to})
	return len(b), nil
}

func (c *fakeConn) Close() error {
	c.once.Do(func() { close(c.done) })
	return nil
}

func (c *fakeConn) LocalAddr() net.Addr              { return c.local }
func (c *fakeConn) SetDeadline(time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(time.Time) error { return nil }

// written returns the datagrams written so far.
func (c *fakeConn) written() []fakeWrite {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]fakeWrite(nil), c.writes...)
}

// identify is the search request, as the original software sends it.
var identify = append([]byte("eQ3Max*\x00**********"), 'I')

// hostAddr is the test host's address, on hostNets' network.
var hostAddr = net.IPv4(192, 0, 2, 10).To4()

// fakeResponder serves a responder on a fakeConn, the host on
// 192.0.2.10/24.
func fakeResponder(t *testing.T) (*Responder, *fakeConn) {
	t.Helper()
	return fakeResponderLog(t, nil)
}

// fakeResponderLog is fakeResponder logging to log.
func fakeResponderLog(t *testing.T, log *slog.Logger) (*Responder, *fakeConn) {
	t.Helper()
	fc := newFakeConn()
	r := NewResponder(log)
	r.hostNets = func() []*net.IPNet {
		return []*net.IPNet{{IP: hostAddr, Mask: net.CIDRMask(24, 32)}}
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go r.Serve(ctx, fc)
	return r, fc
}

// others is the count of other programs' searches the responder saw.
func (r *Responder) othersSeen() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.others
}

// waitFor fails the test unless cond holds within 5 s.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !cond(); time.Sleep(2 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no %s within 5 s", what)
		}
	}
}

// A search from this host and the responder's port is another
// program's, unless it is the responder's own come back; one from
// another port or another host is not.
func TestResponderTellsAnotherProgramsSearch(t *testing.T) {
	r, fc := fakeResponder(t)
	fc.in <- fakePacket{identify, &net.UDPAddr{IP: hostAddr, Port: 5000}}
	fc.in <- fakePacket{identify, &net.UDPAddr{IP: net.IPv4(192, 0, 2, 50), Port: cubelan.DiscoveryPort}}
	ownSends{fc, r}.WriteTo(identify, &net.UDPAddr{IP: net.IPv4bcast, Port: cubelan.DiscoveryPort})
	fc.in <- fakePacket{identify, &net.UDPAddr{IP: hostAddr, Port: cubelan.DiscoveryPort}}
	// The responder reads in turn: once the next one is noted, the ones
	// before it were looked at.
	waitFor(t, "own echo's end", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return time.Since(r.ownSend) > defaultOwnEcho
	})
	fc.in <- fakePacket{identify, &net.UDPAddr{IP: hostAddr, Port: cubelan.DiscoveryPort}}
	waitFor(t, "other program's search", func() bool { return r.othersSeen() > 0 })
	if n := r.othersSeen(); n != 1 {
		t.Fatalf("%d other programs' searches, want 1", n)
	}
}

// Discover waits until another program on this host has not searched
// for the search window, at most searchWaitMax.
func TestDiscoverWaitsForAnotherProgramsSearch(t *testing.T) {
	for _, tc := range []struct {
		name          string
		window, limit time.Duration
		want          time.Duration
	}{
		{"the window", 300 * time.Millisecond, 0, 300 * time.Millisecond},
		{"the limit", time.Hour, 200 * time.Millisecond, 200 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, fc := fakeResponder(t)
			r.mu.Lock()
			r.searchWindow, r.searchWaitMax = tc.window, tc.limit
			r.mu.Unlock()
			fc.in <- fakePacket{identify, &net.UDPAddr{IP: hostAddr, Port: cubelan.DiscoveryPort}}
			waitFor(t, "other program's search", func() bool { return r.othersSeen() > 0 })
			r.mu.Lock()
			seen := r.otherSearch
			r.mu.Unlock()
			called := time.Now()
			to := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 255), Port: cubelan.DiscoveryPort}
			if _, err := r.Discover(context.Background(), cubelan.SearchOptions{Targets: []*net.UDPAddr{to}, Wait: 10 * time.Millisecond, MAC: func(net.IP) string { return "" }}); err != nil {
				t.Fatal(err)
			}
			w := fc.written()
			if len(w) == 0 {
				t.Fatal("Discover sent nothing")
			}
			from := seen
			if tc.limit > 0 {
				from = called
			}
			if early := from.Add(tc.want).Sub(w[0].at); early > 5*time.Millisecond {
				t.Errorf("the search went out %v early", early)
			}
			if tc.limit > 0 && w[0].at.Sub(called) > tc.want+time.Second {
				t.Errorf("the search waited %v, the limit is %v", w[0].at.Sub(called), tc.want)
			}
		})
	}
}

// A search of another program on this host while Discover runs cuts
// it short: Discover returns its answers with ErrSearchShared.
func TestDiscoverCutShortByAnotherProgramsSearch(t *testing.T) {
	r, fc := fakeResponder(t)
	r.mu.Lock()
	r.ownEcho = 20 * time.Millisecond
	r.mu.Unlock()
	to := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 255), Port: cubelan.DiscoveryPort}
	done := make(chan error, 1)
	go func() {
		_, err := r.Discover(context.Background(), cubelan.SearchOptions{Targets: []*net.UDPAddr{to}, Wait: 600 * time.Millisecond, MAC: func(net.IP) string { return "" }})
		done <- err
	}()
	// Search sends twice, 50 ms apart; another program's search comes
	// well after the second.
	waitFor(t, "the search's two requests", func() bool { return len(fc.written()) >= 2 })
	time.Sleep(100 * time.Millisecond)
	fc.in <- fakePacket{identify, &net.UDPAddr{IP: hostAddr, Port: cubelan.DiscoveryPort}}
	select {
	case err := <-done:
		if !errors.Is(err, ErrSearchShared) {
			t.Fatalf("Discover: %v, want ErrSearchShared", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Discover did not end")
	}
	// Without another program's search, the next run is complete.
	r.mu.Lock()
	r.searchWindow = time.Millisecond
	r.mu.Unlock()
	if _, err := r.Discover(context.Background(), cubelan.SearchOptions{Targets: []*net.UDPAddr{to}, Wait: 10 * time.Millisecond, MAC: func(net.IP) string { return "" }}); err != nil {
		t.Fatalf("the next Discover: %v", err)
	}
}

// syncBuffer is a log's buffer that the test reads while the responder
// writes it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

// lines returns the lines logged at level that contain s.
func (b *syncBuffer) lines(level, s string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, l := range strings.Split(b.b.String(), "\n") {
		if strings.Contains(l, "level="+level) && strings.Contains(l, s) {
			out = append(out, l)
		}
	}
	return out
}

// A Cube's answer that no Discover or Reboot asked for is logged at
// Warn, once per source and opcode a minute; not one from this host, an
// identify answer, nor one during or just after an operation of the
// responder's own.
func TestResponderWarnsOfAnswersItDidNotAskFor(t *testing.T) {
	var buf syncBuffer
	r, fc := fakeResponderLog(t, slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	cube := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 23), Port: cubelan.DiscoveryPort}
	answer := func(op byte) []byte { return cubelan.UDPReply("KEQ0000000", op, []byte{0}) }
	const msg = "answered another program on this host"
	warned := func() int { return len(buf.lines("WARN", msg)) }

	fc.in <- fakePacket{answer('c'), cube}
	waitFor(t, "the first warning", func() bool { return warned() == 1 })
	fc.in <- fakePacket{answer('c'), cube}                                    // again within the minute
	fc.in <- fakePacket{answer('c'), &net.UDPAddr{IP: hostAddr, Port: 23272}} // this host
	fc.in <- fakePacket{answer('I'), cube}                                    // identify
	fc.in <- fakePacket{answer('C'), cube}                                    // another opcode
	waitFor(t, "the second warning", func() bool { return warned() == 2 })
	if l := buf.lines("WARN", msg); !strings.Contains(l[0], "serial=KEQ0000000") || !strings.Contains(l[0], "op=c") || !strings.Contains(l[1], "op=C") {
		t.Errorf("warnings %q", l)
	}
	waitFor(t, "the identify answer at Debug", func() bool { return len(buf.lines("DEBUG", "identify answer")) == 1 })

	// During an operation of the responder's own, and within the grace
	// after it, an answer is that operation's.
	ch, _ := r.startOp()
	fc.in <- fakePacket{answer('h'), cube}
	<-ch
	r.endOp(ch)
	fc.in <- fakePacket{answer('H'), cube}
	// The responder reads in turn: once the search after it is counted,
	// the answer within the grace was looked at.
	fc.in <- fakePacket{identify, &net.UDPAddr{IP: hostAddr, Port: cubelan.DiscoveryPort}}
	waitFor(t, "the search after it", func() bool { return r.othersSeen() == 1 })
	r.mu.Lock()
	r.opGrace = time.Millisecond
	r.mu.Unlock()
	time.Sleep(10 * time.Millisecond)
	fc.in <- fakePacket{answer('R'), cube}
	waitFor(t, "the warning after the grace", func() bool { return warned() == 3 })
	if l := buf.lines("WARN", msg); !strings.Contains(l[2], "op=R") {
		t.Errorf("third warning %q, want the one after the grace", l[2])
	}
}
