// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
)

// heldClient starts a client against a fake with short polls, so that
// two missed polls begin a hold quickly, and waits for its greeting; it
// returns the watches of the client's log, Debug included, and of its
// events.
func heldClient(t *testing.T, opts Options) (*Client, *fakecube.Server, *testkit.Watch[testkit.Record], *testkit.Watch[Event]) {
	t.Helper()
	srv, addr := startFake(t)
	logger, logs := testkit.LogWatch(slog.LevelDebug)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts.Addr = addr
	if opts.PollInterval == 0 {
		opts.PollInterval = 100 * time.Millisecond
	}
	opts.ReplyTimeout = 50 * time.Millisecond
	opts.Backoff = 10 * time.Millisecond
	opts.Logger = logger
	c := New(opts)
	evs := events(t, c)
	go c.Run(ctx)
	evs.Await(t, 0, "first greeting", kind(EventConnected))
	return c, srv, logs, evs
}

// A Cube that goes silent is lost after two polls, but its connection
// is held, and it resumes on the same connection once the Cube answers
// again: one loss, one return, no second connection. While held, Send
// and Reconnect answer ErrNotConnected at once.
func TestAHeldConnectionResumes(t *testing.T) {
	t.Parallel()
	c, srv, logs, evs := heldClient(t, Options{Hold: time.Minute})
	lines := cubewatch.Lines(srv)
	at := evs.Mark()
	srv.SetSilent(true)
	testkit.Eventually(t, "the loss", func() bool { return c.Lost() && c.Held() })
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
	// No end to wait for: a held client writes nothing and logs nothing
	// at its polls' ticks.
	whileHeld := lines.Mark()
	time.Sleep(300 * time.Millisecond)
	if n := lines.Count(whileHeld, cubewatch.Type('l')); n != 0 {
		t.Errorf("%d polls written while held", n)
	}

	srv.SetSilent(false)
	testkit.Eventually(t, "the resumption", func() bool { return c.Connected() && !c.Lost() && !c.Held() })
	// The return's line comes after the state the wait saw, and before
	// the return's event.
	evs.Await(t, at, "the return's event", kind(EventConnected))
	if srv.Connections() != 1 {
		t.Errorf("%d connections, want the held one only", srv.Connections())
	}
	if lost := logs.Since(0, connLost); len(lost) != 1 || lost[0].Attrs["held_until"] == "" {
		t.Errorf("loss lines: %q", lost)
	}
	if back := logs.Since(0, reEstablished); len(back) != 1 || back[0].Attrs["held"] != "true" {
		t.Errorf("return lines: %q", back)
	}
	if n := evs.Count(0, kind(EventDisconnected)); n != 1 {
		t.Errorf("%d disconnected events, want 1", n)
	}
	if _, err := c.Send(ctx, cube.CmdList); err != nil {
		t.Errorf("Send after the resumption: %v", err)
	}
}

// A hold ends at Options.Hold; the client then dials again, with no
// second loss warning, and the next greeting says the Cube is back.
func TestAHoldEndsAtItsTime(t *testing.T) {
	t.Parallel()
	c, srv, logs, evs := heldClient(t, Options{Hold: 300 * time.Millisecond})
	at := evs.Mark()
	srv.SetSilent(true)
	testkit.Eventually(t, "the loss", c.Held)
	logs.Await(t, 0, "the hold's end", holdEnded)
	if !c.Lost() || c.Held() {
		t.Errorf("after the hold: lost %v, held %v", c.Lost(), c.Held())
	}
	srv.SetSilent(false)
	evs.Await(t, at, "the greeting after the hold", kind(EventConnected))
	if srv.Connections() != 2 {
		t.Errorf("%d connections, want a second after the hold", srv.Connections())
	}
	if n := logs.Count(0, connLost); n != 1 {
		t.Errorf("%d loss lines, want 1", n)
	}
}

// A hold ends once discovery has heard the Cube for HoldHeard without
// the connection resuming.
func TestAHoldEndsOnceTheCubeIsHeard(t *testing.T) {
	t.Parallel()
	c, srv, logs, _ := heldClient(t, Options{Hold: time.Hour, HoldHeard: 200 * time.Millisecond})
	srv.SetSilent(true)
	defer srv.SetSilent(false)
	testkit.Eventually(t, "the loss", c.Held)
	// No end to wait for: the hold's ticks leave no trace while it holds.
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
	if r, _ := logs.Await(t, 0, "the hold's end", holdEnded); !strings.Contains(r.Attrs["error"], "answered discovery") {
		t.Errorf("hold's end: %q", r)
	}
}

// The Cube's close of a held connection ends the hold at once.
func TestTheCubesCloseEndsAHold(t *testing.T) {
	t.Parallel()
	c, srv, logs, evs := heldClient(t, Options{Hold: time.Hour})
	at := evs.Mark()
	srv.SetSilent(true)
	testkit.Eventually(t, "the loss", c.Held)
	srv.Drop()
	logs.Await(t, 0, "the hold's end", holdEnded)
	srv.SetSilent(false)
	evs.Await(t, at, "the greeting after the hold", kind(EventConnected))
}

// A Cube found elsewhere ends the hold: ReconnectIfMoved drops the held
// connection and the client connects at the new address.
func TestAMovedCubeEndsAHold(t *testing.T) {
	t.Parallel()
	c, srv, _, _ := heldClient(t, Options{Hold: time.Hour})
	srv.SetSilent(true)
	defer srv.SetSilent(false)
	testkit.Eventually(t, "the loss", c.Held)
	srv2, addr2 := startFake(t)
	c.SetAddress(addr2)
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Scale(5*time.Second))
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
	t.Parallel()
	srv, addr := startFake(t)
	logger, logs := testkit.LogWatch(slog.LevelDebug)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, PollInterval: time.Hour, Hold: time.Minute, Backoff: 10 * time.Millisecond, Logger: logger})
	var silent atomic.Bool
	c.probeSilent = func(net.Conn) bool { return silent.Load() }
	c.probeEvery = 20 * time.Millisecond
	go c.Run(ctx)
	testkit.Eventually(t, "first greeting", c.Connected)

	lines := cubewatch.Lines(srv)
	at := lines.Mark()
	srv.SetSilent(true)
	silent.Store(true)
	testkit.Eventually(t, "the loss", c.Held)
	silent.Store(false)
	if lost := logs.Since(0, connLost); len(lost) != 1 || !strings.Contains(lost[0].Attrs["error"], "probes unanswered") {
		t.Errorf("loss lines: %q", lost)
	}
	lines.Await(t, at, "the hold's poll", cubewatch.Type('l'))
	srv.SetSilent(false)
	testkit.Eventually(t, "the resumption", func() bool { return c.Connected() && !c.Held() })
	if srv.Connections() != 1 {
		t.Errorf("%d connections", srv.Connections())
	}
}

// How long a connection is held: the default, a value of its own, none
// when asked or without polling.
func TestHoldFor(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
