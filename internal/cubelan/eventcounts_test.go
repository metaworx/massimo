// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// The events a client sends out are numbered in the order of Events, and
// one dropped to make room is the oldest: what is left of a burst no one
// read is its newest, numbered on, and the count is the last number.
func TestEventSeqAcrossDrops(t *testing.T) {
	c := New(Options{})
	const n = 100
	for i := 1; i <= n; i++ {
		c.emit(Event{Kind: EventState, Device: cube.RFAddress(i)})
	}
	if handling, sent := c.EventCounts(); handling != 0 || sent != n {
		t.Fatalf("handling %d, sent %d, want 0 and %d", handling, sent, n)
	}
	kept := cap(c.evs)
	for want := n - kept + 1; want <= n; want++ {
		e := <-c.Events()
		if e.Seq() != uint64(want) || e.Device != cube.RFAddress(want) {
			t.Fatalf("event %d (device %d), want %d", e.Seq(), e.Device, want)
		}
	}
	if (Event{}).Seq() != 0 {
		t.Fatal("an event the client did not send has a number")
	}
}

// Events sent from several goroutines at once come in the order of their
// numbers, those dropped left out, and the newest is never dropped.
func TestEventSeqUnderConcurrentSends(t *testing.T) {
	c := New(Options{})
	const senders, each = 4, 200
	var wg sync.WaitGroup
	for range senders {
		wg.Go(func() {
			for range each {
				c.emit(Event{Kind: EventState})
			}
		})
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	var last uint64
	read := func(e Event) {
		if e.Seq() <= last {
			t.Errorf("event %d after %d", e.Seq(), last)
		}
		last = e.Seq()
	}
	for running := true; running; {
		select {
		case e := <-c.Events():
			read(e)
		case <-done:
			running = false
		}
	}
	for len(c.evs) > 0 {
		read(<-c.Events())
	}
	if _, sent := c.EventCounts(); sent != senders*each || last != sent {
		t.Fatalf("sent %d, the last read %d, want both %d", sent, last, senders*each)
	}
}

// onLine is a log handler that calls f at each record of message msg,
// in the goroutine that writes it, and drops every record.
type onLine struct {
	msg string
	f   func()
}

func (h onLine) Enabled(context.Context, slog.Level) bool { return true }
func (h onLine) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h onLine) WithGroup(string) slog.Handler            { return h }
func (h onLine) Handle(_ context.Context, r slog.Record) error {
	if r.Message == h.msg {
		h.f()
	}
	return nil
}

// Once a client is connected and handles no line, its connected event,
// sent after the greeting's last line made it connected, is counted: a
// test that saw the connection may count on it. The client writes the
// line "cube connected" once it is connected and before it sends the
// event: there, the one moment the event is not counted yet, the line
// it handles still is.
func TestEventCountsCoverTheGreeting(t *testing.T) {
	_, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type counts struct {
		connected bool
		handling  int64
		sent      uint64
	}
	atLine := make(chan counts, 1)
	var c *Client
	c = New(Options{Addr: addr, Logger: slog.New(onLine{msg: "cube connected", f: func() {
		handling, sent := c.EventCounts()
		select {
		case atLine <- counts{c.Connected(), handling, sent}:
		default:
		}
	}})})
	evs := events(t, c)
	go c.Run(ctx)
	got, _ := evs.Await(t, 0, "the connected event", kind(EventConnected))
	var line counts
	select {
	case line = <-atLine:
	case <-time.After(testkit.Scale(testkit.Wait)):
		t.Fatal("no cube connected line")
	}
	if !line.connected || line.handling == 0 {
		t.Fatalf("at the cube connected line: connected %v, %d lines handled, %d events sent; the connected event is number %d",
			line.connected, line.handling, line.sent, got.Seq())
	}
	testkit.Eventually(t, "the greeting's lines handled", func() bool {
		handling, _ := c.EventCounts()
		return handling == 0
	})
	if _, sent := c.EventCounts(); got.Seq() == 0 || got.Seq() > sent {
		t.Fatalf("the connected event is number %d, %d sent before", got.Seq(), sent)
	}
	for i, e := range evs.Since(0, nil) {
		if e.Seq() != uint64(i+1) {
			t.Fatalf("event %d (%s) is number %d", i+1, eventText(e), e.Seq())
		}
	}
}
