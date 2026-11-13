// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// received counts the lines of type typ a fake Cube has read.
func received(srv *fakecube.Server, typ byte) int {
	n := 0
	for _, m := range srv.Received() {
		if m.Type == typ {
			n++
		}
	}
	return n
}

type sent struct {
	reply cube.Message
	err   error
}

// sendAsync runs Send in a goroutine and hands over its outcome.
func sendAsync(ctx context.Context, c *Client, m cube.Message) <-chan sent {
	ch := make(chan sent, 1)
	go func() {
		reply, err := c.Send(ctx, m)
		ch <- sent{reply, err}
	}()
	return ch
}

// outcome waits up to five seconds for a Send started with sendAsync.
func outcome(t *testing.T, what string, ch <-chan sent) sent {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(5 * time.Second):
		t.Fatalf("%s: Send never returned", what)
		return sent{}
	}
}

// startTurnClient starts a client without polls on a fake Cube and
// waits for its greeting.
func startTurnClient(t *testing.T, opts Options) (*fakecube.Server, *Client, context.Context) {
	t.Helper()
	srv, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts.Addr = addr
	c := New(opts)
	go c.Run(ctx)
	eventually(t, "first greeting", c.Connected)
	return srv, c, ctx
}

// A command sent while another waits for its reply waits its turn: it is
// not written until the first is answered, and then answered itself
// (AP SendTurn).
func TestSendWaitsForTheCommandInFlight(t *testing.T) {
	srv, c, ctx := startTurnClient(t, Options{ReplyTimeout: 5 * time.Second})

	srv.SetSilent(true)
	first := sendAsync(ctx, c, cube.CmdGetNTP)
	eventually(t, "the first f: read", func() bool { return received(srv, 'f') == 1 })
	second := sendAsync(ctx, c, cube.CmdGetNTP)
	time.Sleep(100 * time.Millisecond)
	select {
	case s := <-second:
		t.Fatalf("the second Send returned while the first waited: %v %v", s.reply, s.err)
	default:
	}
	if n := received(srv, 'f'); n != 1 {
		t.Fatalf("%d f: lines written while the first waited, want 1", n)
	}

	srv.SetSilent(false)
	for _, w := range []struct {
		what string
		ch   <-chan sent
	}{{"first", first}, {"second", second}} {
		if s := outcome(t, w.what, w.ch); s.err != nil || s.reply.Type != 'F' {
			t.Errorf("%s: %v %v", w.what, s.reply, s.err)
		}
	}
	if n := received(srv, 'f'); n != 2 {
		t.Errorf("%d f: lines, want 2", n)
	}
}

// A Send waiting its turn ends with its context, and its line is never
// written.
func TestWaitingSendEndsWithItsContext(t *testing.T) {
	srv, c, ctx := startTurnClient(t, Options{ReplyTimeout: 5 * time.Second})

	srv.SetSilent(true)
	first := sendAsync(ctx, c, cube.CmdGetNTP)
	eventually(t, "the first f: read", func() bool { return received(srv, 'f') == 1 })
	wctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Send(wctx, cube.Message{Type: 'c', Data: "0a010d"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting Send: %v, want the context's deadline", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("waiting Send returned after %v, its context ended after 50 ms", d)
	}
	if n := received(srv, 'c'); n != 0 {
		t.Errorf("%d c: lines written, want none", n)
	}
	srv.SetSilent(false)
	if s := outcome(t, "first", first); s.err != nil {
		t.Errorf("first: %v", s.err)
	}
}

// A Send waiting its turn ends with ErrNotConnected when the connection
// drops, as the one in flight does.
func TestWaitingSendEndsWhenTheConnectionDrops(t *testing.T) {
	srv, c, ctx := startTurnClient(t, Options{ReplyTimeout: 5 * time.Second, Backoff: time.Minute})

	srv.SetSilent(true)
	first := sendAsync(ctx, c, cube.CmdGetNTP)
	eventually(t, "the first f: read", func() bool { return received(srv, 'f') == 1 })
	second := sendAsync(ctx, c, cube.Message{Type: 'c', Data: "0a010d"})
	time.Sleep(50 * time.Millisecond)
	srv.Drop()
	for _, w := range []struct {
		what string
		ch   <-chan sent
	}{{"first", first}, {"second", second}} {
		if s := outcome(t, w.what, w.ch); !errors.Is(s.err, ErrNotConnected) {
			t.Errorf("%s: %v %v, want ErrNotConnected", w.what, s.reply, s.err)
		}
	}
	if n := received(srv, 'c'); n != 0 {
		t.Errorf("%d c: lines written, want none", n)
	}
}

// A poll that finds another command waiting for its reply sends nothing
// and counts no miss: polls go on once the command is answered, on the
// same connection.
func TestPollSkipsABusyLine(t *testing.T) {
	var logs logLines
	srv, c, ctx := startTurnClient(t, Options{PollInterval: 50 * time.Millisecond, ReplyTimeout: 2 * time.Second, Logger: logs.logger()})

	srv.SetSilent(true)
	held := sendAsync(ctx, c, cube.CmdGetNTP)
	eventually(t, "the f: read", func() bool { return received(srv, 'f') == 1 })
	// A poll that took the turn first is held as well, and ends with the
	// reply timeout; the polls after the f: are the ones that matter.
	polls, warned := pollsReceived(srv), logs.count("WARN", "poll failed")
	time.Sleep(300 * time.Millisecond)
	if n := pollsReceived(srv); n != polls {
		t.Errorf("%d polls written while a command waited for its reply", n-polls)
	}
	srv.SetSilent(false)
	if s := outcome(t, "held f:", held); s.err != nil {
		t.Fatalf("held f: %v", s.err)
	}
	eventually(t, "polls after the reply", func() bool { return pollsReceived(srv) >= polls+2 })
	if c.Lost() || srv.Connections() != 1 {
		t.Errorf("lost %v, %d connections", c.Lost(), srv.Connections())
	}
	if n := logs.count("WARN", "poll failed") - warned; n != 0 {
		t.Errorf("%d poll failed warnings", n)
	}
}
