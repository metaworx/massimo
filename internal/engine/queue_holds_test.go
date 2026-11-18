// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package engine

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
)

// The budget's holds and the commands of r: lines alone (AP RadioHealth
// v1.2, D183-B): an r: takes no memory slot and no air time.

// holdRecorder keeps the holds the queue reports.
type holdRecorder struct {
	mu    sync.Mutex
	holds []Hold
}

func (h *holdRecorder) note(x Hold) { h.mu.Lock(); h.holds = append(h.holds, x); h.mu.Unlock() }

func (h *holdRecorder) list() []Hold {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]Hold(nil), h.holds...)
}

// A command of r: lines alone, as the server's automatic reset, goes at
// once while the duty cycle is used up and few slots are free, and no
// hold is reported for it.
func TestQueueResetsPassTheHolds(t *testing.T) {
	var rec holdRecorder
	srv, q, _ := setup(t, Options{DutyWait: time.Minute, SlotWait: time.Minute, OnHold: rec.note})
	lines := cubewatch.Lines(srv)
	srv.Duty = 0x64 // 100 %
	srv.SetFreeSlots(2)
	m := roomSetpoint(t)
	// The first radio command learns the budget: 100 %, 2 slots free.
	q.Enqueue(Command{Lines: []cube.Message{m}}).Wait(context.Background())
	reset := cube.CmdResetError(0x0a010d)
	ctx, stop := context.WithTimeout(context.Background(), testkit.Scale(5*time.Second))
	defer stop()
	res := q.Enqueue(Command{Key: "error-reset:0a010d", Priority: PriorityLow, Lines: []cube.Message{reset}}).Wait(ctx)
	if res.Err != nil || res.Discarded {
		t.Fatalf("result %+v", res)
	}
	if h := rec.list(); len(h) != 0 {
		t.Fatalf("holds %+v, want none for r: lines alone", h)
	}
	sameSent(t, lines, []string{m.String(), reset.String()})
}

// A command with an s: line waits for free slots as before, also where
// it holds an r: too.
func TestQueueHoldsAResetWithARadiogram(t *testing.T) {
	var rec holdRecorder
	srv, q, _ := setup(t, Options{SlotWait: 200 * time.Millisecond, OnHold: rec.note})
	lines := cubewatch.Lines(srv)
	srv.SetFreeSlots(2)
	m := roomSetpoint(t)
	// The first radio command learns the budget: 2 slots free.
	q.Enqueue(Command{Lines: []cube.Message{m}}).Wait(context.Background())
	reset := cube.CmdResetError(0x0a010d)
	start := time.Now()
	if res := q.Enqueue(Command{Lines: []cube.Message{reset, m}}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
	if d := time.Since(start); d < 190*time.Millisecond {
		t.Fatalf("did not hold: %s", d)
	}
	if h := rec.list(); len(h) != 1 || h[0].Reason != HoldMemorySlots || h[0].FreeSlots != 2 {
		t.Fatalf("holds %+v, want one for the memory slots", h)
	}
	sameSent(t, lines, []string{m.String(), reset.String(), m.String()})
}

// A command whose Wanted says no once the hold has let it pass is
// dropped with ErrUnwanted, nothing sent: Wanted is asked after the
// hold, as for an automatic correction that a newer command made moot
// while it waited (AP RadioHealth v1.3, Block 8). One it wants goes.
func TestQueueDropsAnUnwantedCommand(t *testing.T) {
	var wanted atomic.Bool
	var asked atomic.Int32
	var once sync.Once
	srv, q, _ := setup(t, Options{SlotWait: 100 * time.Millisecond, OnHold: func(Hold) { once.Do(func() { wanted.Store(false) }) }})
	lines := cubewatch.Lines(srv)
	srv.SetFreeSlots(2)
	m := roomSetpoint(t)
	// The first radio command learns the budget: 2 slots free.
	q.Enqueue(Command{Lines: []cube.Message{m}}).Wait(context.Background())
	wanted.Store(true)
	still := func() bool { asked.Add(1); return wanted.Load() }
	res := q.Enqueue(Command{Priority: PriorityLow, Lines: []cube.Message{m}, Wanted: still}).Wait(context.Background())
	if !errors.Is(res.Err, ErrUnwanted) || asked.Load() != 1 {
		t.Fatalf("result %+v, asked %d times", res, asked.Load())
	}
	sameSent(t, lines, []string{m.String()})

	wanted.Store(true)
	if res := q.Enqueue(Command{Priority: PriorityLow, Lines: []cube.Message{m}, Wanted: still}).Wait(context.Background()); res.Err != nil {
		t.Fatalf("a wanted command: %+v", res)
	}
	sameSent(t, lines, []string{m.String(), m.String()})
}
