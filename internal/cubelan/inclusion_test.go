// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

func TestInclusionDeliversNewDevices(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := fakecube.PairingFrom(greeting, 0x1514b6, 0x0a0b0c, "TEQ0000001", 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	srv := &fakecube.Server{Greeting: greeting, Pair: pair}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr})
	go c.Run(ctx)
	waitEvent(t, c, EventConnected)

	if err := c.StartInclusion(30 * time.Second); err != nil {
		t.Fatal(err)
	}
	if err := c.StartInclusion(30 * time.Second); err != ErrInclusionActive {
		t.Fatalf("second start: %v", err)
	}
	lines, stop := c.WatchInclusion()
	defer stop()
	// n: has no immediate reply: Send returns after the write.
	if _, err := c.Send(ctx, cube.CmdInclusion(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-lines:
		nd, err := cube.ParseNewDevice(m.Data)
		if err != nil || nd.RFAddress != 0x0a0b0c || nd.Serial != "TEQ0000001" || nd.Type != cube.DeviceRadiatorThermostat {
			t.Fatalf("N line %v: %+v %v", m, nd, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no N line")
	}
	if e := waitEvent(t, c, EventPaired); e.Device != 0x0a0b0c || e.New == nil {
		t.Fatalf("paired event %+v", e)
	}
	if st := c.Inclusion(); !st.Active || len(st.Paired) != 1 || st.Paired[0].RFAddress != 0x0a0b0c {
		t.Fatalf("state %+v", st)
	}
	// x: ends it; the Cube's empty N line is relayed, then the watch ends.
	if _, err := c.Send(ctx, cube.CmdCancelInclusion); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, c, EventPairingEnded)
	if m, ok := <-lines; !ok || m.Data != "" {
		t.Fatalf("end line %v %v", m, ok)
	}
	if _, ok := <-lines; ok {
		t.Fatal("watch still open")
	}
	if st := c.Inclusion(); st.Active || st.Ended.IsZero() || len(st.Paired) != 1 {
		t.Fatalf("state after the end %+v", st)
	}
	// Without an inclusion, a watch is closed at once.
	if _, ok := <-func() <-chan cube.Message { ch, _ := c.WatchInclusion(); return ch }(); ok {
		t.Fatal("watch without inclusion")
	}
}

func TestInclusionEndsOnItsOwn(t *testing.T) {
	_, addr := startFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr})
	go c.Run(ctx)
	waitEvent(t, c, EventConnected)
	c.StartInclusion(time.Second)
	c.Send(ctx, cube.CmdInclusion(time.Second)) // the fake ends it after a second with N:
	waitEvent(t, c, EventPairingEnded)
	if c.Inclusion().Active {
		t.Fatal("still active")
	}
}
