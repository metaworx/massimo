// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// scripted is a Gateway that answers every line, but for the radio line
// failRadio and the plain line of type failPlain, which fail, and the
// radio line discard, which the Cube discards; radio lines are counted
// from the first sent, a prelude's r: included, -1 for none.
type scripted struct {
	snap      *cubelan.Snapshot
	failRadio int
	discard   int
	failPlain byte

	mu    sync.Mutex
	radio int // radio lines sent
}

func (g *scripted) Send(_ context.Context, m cube.Message) (cube.Message, error) {
	if m.Type == g.failPlain {
		return cube.Message{}, errors.New("scripted: no answer")
	}
	return cube.Message{Type: 'A'}, nil
}

func (g *scripted) SendRadio(_ context.Context, m cube.Message) (*cube.SendResult, error) {
	g.mu.Lock()
	i := g.radio
	g.radio++
	g.mu.Unlock()
	if i == g.failRadio {
		return nil, errors.New("scripted: connection lost")
	}
	return &cube.SendResult{Discarded: i == g.discard, FreeMemorySlots: 49}, nil
}

func (g *scripted) Snapshot() *cubelan.Snapshot { return g.snap }

func (g *scripted) Budget() cubelan.Budget { return cubelan.Budget{} }

// Result.Sent counts the lines the queue began to send, the one that
// failed included: an error of the gateway on line k gives k+1, as does
// a discard on line k; a wakeup or a prelude that fails sends no line
// of the command's, 0.
func TestResultSent(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := cubelan.SnapshotOf(withError(t, greeting, 0x0a010d), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	three := []cube.Message{cube.CmdSetGroup(0x0a0108, 3, false), cube.CmdSetGroup(0x0a0109, 2, false), cube.CmdSetGroup(0x0a010b, 4, false)}
	for _, c := range []struct {
		name    string
		gw      *scripted
		cmd     Command
		sent    int
		replies int
	}{
		{"all answered", &scripted{failRadio: -1, discard: -1}, Command{Lines: three}, 3, 3},
		{"the gateway fails on the second line", &scripted{failRadio: 1, discard: -1}, Command{Lines: three}, 2, 1},
		{"the gateway fails on the first line", &scripted{failRadio: 0, discard: -1}, Command{Lines: three}, 1, 0},
		{"the second line discarded", &scripted{failRadio: -1, discard: 1}, Command{Lines: three}, 2, 2},
		{"the wakeup fails", &scripted{failRadio: -1, discard: -1, failPlain: 'z'}, Command{RoomID: 3, Wakeup: true, Lines: three}, 0, 0},
		{"the prelude's reset fails", &scripted{failRadio: 0, discard: -1}, Command{Target: 0x0a010d, Lines: three}, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.gw.snap = snap
			q := NewQueue(c.gw, Options{Gap: time.Millisecond})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go q.Run(ctx)
			res := q.Enqueue(c.cmd).Wait(ctx)
			if res.Sent != c.sent || len(res.Replies) != c.replies {
				t.Fatalf("sent %d with %d replies (%v), want %d and %d", res.Sent, len(res.Replies), res.Err, c.sent, c.replies)
			}
		})
	}
}
