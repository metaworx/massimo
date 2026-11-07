// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package engine

import (
	"context"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
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
	case <-time.After(5 * time.Second):
		t.Fatal("client not ready")
	}
	q := NewQueue(c, Options{Gap: time.Millisecond})
	go q.Run(ctx)
	reconnect := func() {
		t.Helper()
		wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
		defer wcancel()
		if res := q.Enqueue(Command{Key: "refresh", Priority: PriorityNormal, Reconnect: true, Moved: true}).Wait(wctx); res.Err != nil {
			t.Fatal(res.Err)
		}
	}

	// Still connected to the old address: dropped, the new one dialled.
	c.SetAddress(newAddr)
	reconnect()
	for deadline := time.Now().Add(5 * time.Second); !(c.Connected() && moved.Connections() == 1); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("not connected to the new address: %d connections", moved.Connections())
		}
	}
	// Connected to the new address already: nothing to drop.
	reconnect()
	time.Sleep(300 * time.Millisecond)
	if n := moved.Connections(); n != 1 {
		t.Fatalf("the reconnect for the move dropped the connection to the new address: %d connections", n)
	}
}
