// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
)

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

// outcome waits up to five seconds, scaled, for a Send started with
// sendAsync.
func outcome(t *testing.T, what string, ch <-chan sent) sent {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(testkit.Scale(5 * time.Second)):
		t.Fatalf("%s: Send never returned", what)
		return sent{}
	}
}

// startTurnClient starts a client without polls on a fake Cube and
// waits for its greeting; it returns the fake and the watch of its
// lines.
func startTurnClient(t *testing.T, opts Options) (*fakecube.Server, *testkit.Watch[cube.Message], *Client, context.Context) {
	t.Helper()
	srv, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts.Addr = addr
	c := New(opts)
	go c.Run(ctx)
	testkit.Eventually(t, "first greeting", c.Connected)
	return srv, cubewatch.Lines(srv), c, ctx
}

// A command sent while another waits for its reply waits its turn: it is
// not written until the first is answered, and then answered itself
// (AP SendTurn).
func TestSendWaitsForTheCommandInFlight(t *testing.T) {
	srv, lines, c, ctx := startTurnClient(t, Options{ReplyTimeout: 5 * time.Second})

	srv.SetSilent(true)
	from := lines.Mark()
	first := sendAsync(ctx, c, cube.CmdGetNTP)
	lines.Await(t, from, "the first f: read", cubewatch.Type('f'))
	second := sendAsync(ctx, c, cube.CmdGetNTP)
	// No end to wait for: a Send waiting its turn writes nothing.
	time.Sleep(100 * time.Millisecond)
	select {
	case s := <-second:
		t.Fatalf("the second Send returned while the first waited: %v %v", s.reply, s.err)
	default:
	}
	if n := lines.Count(0, cubewatch.Type('f')); n != 1 {
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
	if n := lines.Count(0, cubewatch.Type('f')); n != 2 {
		t.Errorf("%d f: lines, want 2", n)
	}
}

// A Send waiting its turn ends with its context, and its line is never
// written.
func TestWaitingSendEndsWithItsContext(t *testing.T) {
	srv, lines, c, ctx := startTurnClient(t, Options{ReplyTimeout: 5 * time.Second})

	srv.SetSilent(true)
	from := lines.Mark()
	first := sendAsync(ctx, c, cube.CmdGetNTP)
	lines.Await(t, from, "the first f: read", cubewatch.Type('f'))
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
	if n := lines.Count(0, cubewatch.Type('c')); n != 0 {
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
	srv, lines, c, ctx := startTurnClient(t, Options{ReplyTimeout: 5 * time.Second, Backoff: time.Minute})

	srv.SetSilent(true)
	from := lines.Mark()
	first := sendAsync(ctx, c, cube.CmdGetNTP)
	lines.Await(t, from, "the first f: read", cubewatch.Type('f'))
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
	if n := lines.Count(0, cubewatch.Type('c')); n != 0 {
		t.Errorf("%d c: lines written, want none", n)
	}
}

// A poll that finds another command waiting for its reply sends nothing
// and counts no miss: polls go on once the command is answered, on the
// same connection.
func TestPollSkipsABusyLine(t *testing.T) {
	logger, logs := testkit.LogWatch(slog.LevelDebug)
	srv, lines, c, ctx := startTurnClient(t, Options{PollInterval: 50 * time.Millisecond, ReplyTimeout: 2 * time.Second, Logger: logger})

	srv.SetSilent(true)
	from := lines.Mark()
	held := sendAsync(ctx, c, cube.CmdGetNTP)
	lines.Await(t, from, "the f: read", cubewatch.Type('f'))
	// A poll that took the turn first is held as well, and ends with the
	// reply timeout; the polls after the f: are the ones that matter. No
	// end to wait for: a poll that finds the line busy writes nothing.
	polls, warned := lines.Mark(), logs.Mark()
	time.Sleep(300 * time.Millisecond)
	if n := lines.Count(polls, cubewatch.Type('l')); n != 0 {
		t.Errorf("%d polls written while a command waited for its reply", n)
	}
	srv.SetSilent(false)
	if s := outcome(t, "held f:", held); s.err != nil {
		t.Fatalf("held f: %v", s.err)
	}
	lines.AwaitN(t, polls, "polls after the reply", 2, cubewatch.Type('l'))
	if c.Lost() || srv.Connections() != 1 {
		t.Errorf("lost %v, %d connections", c.Lost(), srv.Connections())
	}
	if n := logs.Count(warned, pollFailed); n != 0 {
		t.Errorf("%d poll failed warnings", n)
	}
}
