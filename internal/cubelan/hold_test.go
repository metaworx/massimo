// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// heldClient starts a client against a fake with short polls, so that
// two missed polls begin a hold quickly.
func heldClient(t *testing.T, opts Options) (*Client, *fakecube.Server, *logLines, *eventCount) {
	t.Helper()
	srv, addr := startFake(t)
	logs := &logLines{}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts.Addr = addr
	if opts.PollInterval == 0 {
		opts.PollInterval = 100 * time.Millisecond
	}
	opts.ReplyTimeout = 50 * time.Millisecond
	opts.Backoff = 10 * time.Millisecond
	opts.Logger = logs.logger()
	c := New(opts)
	evs := countEvents(ctx, c)
	go c.Run(ctx)
	eventually(t, "first greeting", func() bool { return evs.get(EventConnected) == 1 })
	return c, srv, logs, evs
}

// A Cube that goes silent is lost after two polls, but its connection
// is held, and it resumes on the same connection once the Cube answers
// again: one loss, one return, no second connection. While held, Send
// and Reconnect answer ErrNotConnected at once.
func TestAHeldConnectionResumes(t *testing.T) {
	c, srv, logs, evs := heldClient(t, Options{Hold: time.Minute})
	srv.SetSilent(true)
	eventually(t, "the loss", func() bool { return c.Lost() && c.Held() })
	if c.Connected() {
		t.Error("connected while held")
	}
	if u := c.HeldUntil(); u.Before(time.Now().Add(50*time.Second)) || u.After(time.Now().Add(time.Minute)) {
		t.Errorf("held until %v, want about a minute from now", u)
	}
	ctx := context.Background()
	if _, err := c.Send(ctx, cube.CmdList); !errors.Is(err, ErrNotConnected) {
		t.Errorf("Send while held: %v", err)
	}
	if err := c.Reconnect(ctx); !errors.Is(err, ErrNotConnected) {
		t.Errorf("Reconnect while held: %v", err)
	}
	n := pollsReceived(srv)
	time.Sleep(300 * time.Millisecond)
	if m := pollsReceived(srv); m != n {
		t.Errorf("%d polls written while held", m-n)
	}

	srv.SetSilent(false)
	eventually(t, "the resumption", func() bool { return c.Connected() && !c.Lost() && !c.Held() })
	if srv.Connections() != 1 {
		t.Errorf("%d connections, want the held one only", srv.Connections())
	}
	if lost := logs.lines("WARN", "cube connection lost"); len(lost) != 1 || !strings.Contains(lost[0], "held_until=") {
		t.Errorf("loss lines: %q", lost)
	}
	if back := logs.lines("INFO", "cube connection re-established"); len(back) != 1 || !strings.Contains(back[0], "held=true") {
		t.Errorf("return lines: %q", back)
	}
	eventually(t, "the return's event", func() bool { return evs.get(EventConnected) == 2 })
	if n := evs.get(EventDisconnected); n != 1 {
		t.Errorf("%d disconnected events, want 1", n)
	}
	if _, err := c.Send(ctx, cube.CmdList); err != nil {
		t.Errorf("Send after the resumption: %v", err)
	}
}

// A hold ends at Options.Hold; the client then dials again, with no
// second loss warning, and the next greeting says the Cube is back.
func TestAHoldEndsAtItsTime(t *testing.T) {
	c, srv, logs, evs := heldClient(t, Options{Hold: 300 * time.Millisecond})
	srv.SetSilent(true)
	eventually(t, "the loss", c.Held)
	eventually(t, "the hold's end", func() bool { return logs.count("WARN", "cube held connection ended") == 1 })
	if !c.Lost() || c.Held() {
		t.Errorf("after the hold: lost %v, held %v", c.Lost(), c.Held())
	}
	srv.SetSilent(false)
	eventually(t, "the greeting after the hold", func() bool { return evs.get(EventConnected) == 2 })
	if srv.Connections() != 2 {
		t.Errorf("%d connections, want a second after the hold", srv.Connections())
	}
	if n := logs.count("WARN", "cube connection lost"); n != 1 {
		t.Errorf("%d loss lines, want 1", n)
	}
}

// A hold ends once discovery has heard the Cube for HoldHeard without
// the connection resuming.
func TestAHoldEndsOnceTheCubeIsHeard(t *testing.T) {
	c, srv, logs, _ := heldClient(t, Options{Hold: time.Hour, HoldHeard: 200 * time.Millisecond})
	srv.SetSilent(true)
	defer srv.SetSilent(false)
	eventually(t, "the loss", c.Held)
	time.Sleep(300 * time.Millisecond)
	if !c.Held() {
		t.Fatal("the hold ended though discovery heard nothing")
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(20 * time.Millisecond):
				c.Heard()
			}
		}
	}()
	eventually(t, "the hold's end", func() bool { return logs.count("WARN", "cube held connection ended") == 1 })
	if l := logs.lines("WARN", "cube held connection ended")[0]; !strings.Contains(l, "answered discovery") {
		t.Errorf("hold's end: %q", l)
	}
}

// The Cube's close of a held connection ends the hold at once.
func TestTheCubesCloseEndsAHold(t *testing.T) {
	c, srv, logs, evs := heldClient(t, Options{Hold: time.Hour})
	srv.SetSilent(true)
	eventually(t, "the loss", c.Held)
	srv.Drop()
	eventually(t, "the hold's end", func() bool { return logs.count("WARN", "cube held connection ended") == 1 })
	srv.SetSilent(false)
	eventually(t, "the greeting after the hold", func() bool { return evs.get(EventConnected) == 2 })
}

// A Cube found elsewhere ends the hold: ReconnectIfMoved drops the held
// connection and the client connects at the new address.
func TestAMovedCubeEndsAHold(t *testing.T) {
	c, srv, _, _ := heldClient(t, Options{Hold: time.Hour})
	srv.SetSilent(true)
	defer srv.SetSilent(false)
	eventually(t, "the loss", c.Held)
	srv2, addr2 := startFake(t)
	c.SetAddress(addr2)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ReconnectIfMoved(ctx); err != nil {
		t.Fatal(err)
	}
	if !c.Connected() || c.Lost() || c.Held() || srv2.Connections() != 1 {
		t.Errorf("after the move: connected %v, lost %v, held %v, %d connections", c.Connected(), c.Lost(), c.Held(), srv2.Connections())
	}
}

// On Linux the kernel's count of unanswered keepalive probes begins a
// hold between polls, about 30 s after the Cube went; the hold writes
// a poll, whose answer resumes it.
func TestUnansweredKeepaliveProbesBeginAHold(t *testing.T) {
	srv, addr := startFake(t)
	var logs logLines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, PollInterval: time.Hour, Hold: time.Minute, Backoff: 10 * time.Millisecond, Logger: logs.logger()})
	var silent atomic.Bool
	c.probeSilent = func(net.Conn) bool { return silent.Load() }
	c.probeEvery = 20 * time.Millisecond
	go c.Run(ctx)
	eventually(t, "first greeting", c.Connected)

	n := pollsReceived(srv)
	srv.SetSilent(true)
	silent.Store(true)
	eventually(t, "the loss", c.Held)
	silent.Store(false)
	if lost := logs.lines("WARN", "cube connection lost"); len(lost) != 1 || !strings.Contains(lost[0], "probes unanswered") {
		t.Errorf("loss lines: %q", lost)
	}
	eventually(t, "the hold's poll", func() bool { return pollsReceived(srv) == n+1 })
	srv.SetSilent(false)
	eventually(t, "the resumption", func() bool { return c.Connected() && !c.Held() })
	if srv.Connections() != 1 {
		t.Errorf("%d connections", srv.Connections())
	}
}

// How long a connection is held: the default, a value of its own, none
// when asked or without polling.
func TestHoldFor(t *testing.T) {
	for _, tc := range []struct {
		o    Options
		want time.Duration
	}{
		{Options{PollInterval: time.Minute}, DefaultHold},
		{Options{PollInterval: time.Minute, Hold: 2 * time.Minute}, 2 * time.Minute},
		{Options{PollInterval: time.Minute, Hold: -1}, 0},
		{Options{Hold: time.Hour}, 0},
	} {
		if got := tc.o.holdFor(); got != tc.want {
			t.Errorf("%+v: hold %v, want %v", tc.o, got, tc.want)
		}
	}
}

// Where only the probe count keeps the kernel from ending a held
// connection, the count covers the hold, at most 255 probes.
func TestKeepAliveCovering(t *testing.T) {
	if k := keepAliveCovering(0); k != keepAlive {
		t.Errorf("no hold: %+v", k)
	}
	if k := keepAliveCovering(DefaultHold); k.Interval != 15*time.Second || k.Count != 180 {
		t.Errorf("45 min: %+v", k)
	}
	if k := keepAliveCovering(24 * time.Hour); k.Count != 255 {
		t.Errorf("a day: %+v", k)
	}
	if s := holdSeconds(1500 * time.Millisecond); s != 2 {
		t.Errorf("1.5 s in seconds: %d", s)
	}
}

// The kernel lets a held connection live for the hold and the time the
// loss takes to be marked before it, from the first byte left
// unacknowledged: two polls and their reply timeouts, or the keepalive's
// silence and probes, whichever is longer, and a probe's interval.
func TestKernelHold(t *testing.T) {
	for _, tc := range []struct {
		o    Options
		want time.Duration
	}{
		{Options{PollInterval: time.Minute, ReplyTimeout: 10 * time.Second}, DefaultHold + 140*time.Second + 5*time.Second},
		{Options{PollInterval: 2 * time.Second, ReplyTimeout: 10 * time.Second, Hold: 20 * time.Second}, 20*time.Second + 30*time.Second + 5*time.Second},
		{Options{PollInterval: time.Minute, Hold: -1}, 0},
	} {
		if got := tc.o.kernelHold(); got != tc.want {
			t.Errorf("%+v: kernel hold %v, want %v", tc.o, got, tc.want)
		}
	}
}
