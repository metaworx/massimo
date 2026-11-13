// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package engine

import (
	"context"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// A queue command sent while the connection's poll waits for a slow
// reply waits for it and is sent then, instead of failing with
// cubelan.ErrBusy (AP SendTurn).
func TestQueueCommandWaitsForAPollInFlight(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &fakecube.Server{Greeting: greeting}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := cubelan.New(cubelan.Options{Addr: addr, PollInterval: 50 * time.Millisecond, ReplyTimeout: 5 * time.Second})
	go c.Run(ctx)
	select {
	case <-c.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("client not ready")
	}
	q := NewQueue(c, Options{Gap: time.Millisecond})
	go q.Run(ctx)

	count := func(typ byte) int {
		n := 0
		for _, m := range srv.Received() {
			if m.Type == typ {
				n++
			}
		}
		return n
	}
	polls := count('l')
	srv.SetSilent(true)
	for deadline := time.Now().Add(5 * time.Second); count('l') == polls; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("no poll while the Cube is silent")
		}
	}

	// A setpoint, as a user's write: the poll's l: holds the line.
	m, err := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a010d, RoomID: 6, Room: true}, 19, cube.ModeManual, cube.Until{})
	if err != nil {
		t.Fatal(err)
	}
	ticket := q.Enqueue(Command{Lines: []cube.Message{m}})
	select {
	case res := <-ticket.Done():
		t.Fatalf("the command ended while the poll waited for its reply: %v", res.Err)
	case <-time.After(150 * time.Millisecond):
	}
	if n := count('s'); n != 0 {
		t.Fatalf("%d s: lines written while the poll waited for its reply", n)
	}

	srv.SetSilent(false)
	wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
	defer wcancel()
	res := ticket.Wait(wctx)
	if res.Err != nil || len(res.Replies) != 1 || res.Replies[0].Type != 'S' {
		t.Fatalf("the command after the poll: %v %v", res.Replies, res.Err)
	}
	if n := count('s'); n != 1 {
		t.Errorf("%d s: lines, want 1", n)
	}
}
