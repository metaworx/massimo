// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"context"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// A pairing whose n: is still queued when its caller gives up is taken
// out of the queue: the n: is never sent, so no inclusion runs on the
// Cube that the client counts as ended.
func TestTimedOutPairingIsWithdrawn(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &fakecube.Server{Greeting: greeting, Duty: 100}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The duty cycle "resets" 600 ms after a command waits for it.
	c := cubelan.New(cubelan.Options{Addr: addr})
	q := engine.NewQueue(c, engine.Options{Gap: time.Millisecond, ResetAt: func(now time.Time) time.Time { return now.Add(600 * time.Millisecond) }})
	go c.Run(ctx)
	go q.Run(ctx)
	<-c.Ready()
	w := Writer{Client: c, Queue: q}
	radio := cube.Message{Type: 's', Data: "AARAAAAACgENBms="}
	// The first radio command reports the duty cycle exhausted; the
	// second waits for the reset and holds the queue.
	if res := w.Do(ctx, engine.Command{Lines: []cube.Message{radio}}); res.Err != nil {
		t.Fatal(res.Err)
	}
	held := q.Enqueue(engine.Command{Lines: []cube.Message{radio}})
	time.Sleep(100 * time.Millisecond)
	pctx, stop := context.WithTimeout(ctx, 200*time.Millisecond)
	defer stop()
	if _, _, err := w.StartPairing(pctx, time.Minute); err == nil {
		t.Fatal("a pairing behind a held queue started")
	}
	if c.Inclusion().Active {
		t.Fatal("the client counts a withdrawn pairing as running")
	}
	// The queue moves on once the held command is sent.
	if res := held.Wait(ctx); res.Err != nil {
		t.Fatal(res.Err)
	}
	time.Sleep(200 * time.Millisecond)
	for _, m := range srv.Received() {
		if m.Type == 'n' {
			t.Fatalf("the n: of the pairing given up reached the Cube: %v", m)
		}
	}
}
