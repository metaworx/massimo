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
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
)

// A Cube found at a new address: the reconnect queued for it drops a
// connection to the old address, but not one the client has made to the
// new address since (SetAddress wakes its dial loop), which would open a
// second connection to the Cube.
func TestMovedReconnectKeepsTheNewConnection(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	old := &fakecube.Server{Greeting: greeting}
	oldAddr, err := old.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	moved := &fakecube.Server{Greeting: greeting}
	newAddr, err := moved.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer moved.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := cubelan.New(cubelan.Options{Addr: oldAddr})
	go c.Run(ctx)
	select {
	case <-c.Ready():
	case <-time.After(testkit.Scale(5 * time.Second)):
		t.Fatal("client not ready")
	}
	q := NewQueue(c, Options{Gap: time.Millisecond})
	go q.Run(ctx)
	reconnect := func() {
		t.Helper()
		wctx, wcancel := context.WithTimeout(ctx, testkit.Scale(5*time.Second))
		defer wcancel()
		if res := q.Enqueue(Command{Key: "refresh", Priority: PriorityNormal, Reconnect: true, Moved: true}).Wait(wctx); res.Err != nil {
			t.Fatal(res.Err)
		}
	}

	// Still connected to the old address: dropped, the new one dialled.
	c.SetAddress(newAddr)
	reconnect()
	if !testkit.Within(5*time.Second, func() bool { return c.Connected() && moved.Connections() == 1 }) {
		t.Fatalf("not connected to the new address: %d connections", moved.Connections())
	}
	// Connected to the new address already: nothing to drop. The queue
	// runs one command at a time: by the next command's line at the Cube,
	// a drop for the move would have come.
	lines := cubewatch.Lines(moved)
	reconnect()
	next := lines.Mark()
	q.Enqueue(Command{Lines: []cube.Message{cube.CmdGetNTP}})
	testkit.Never(t, "the reconnect for the move dropping the connection to the new address",
		func() bool { return moved.Connections() != 1 }, lines.Seen(next, cubewatch.Type('f')))
}
