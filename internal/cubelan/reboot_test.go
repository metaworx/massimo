// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"net"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// Reboot sends R to the Cube and returns its answer, status 2.
func TestRebootIsAnswered(t *testing.T) {
	d, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000100", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	res, err := Reboot(context.Background(), pc, nil, d.Addr(), "KEQ0000100")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Answered || res.Status != 2 || d.Reboots() != 1 {
		t.Errorf("result %+v, %d reboots", res, d.Reboots())
	}
	// Another serial's Cube does not restart, and nothing answers.
	res, err = Reboot(context.Background(), pc, nil, d.Addr(), "KEQ0000999")
	if err != nil || res.Answered || d.Reboots() != 1 {
		t.Errorf("another serial: %+v, %v, %d reboots", res, err, d.Reboots())
	}
}

// A reboot closes the client's connection without a loss warning, the
// link is rebooting until the greeting after it, and connected then.
func TestRebootingIsNoLoss(t *testing.T) {
	srv, addr := startFake(t)
	var logs logLines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, Backoff: 10 * time.Millisecond, Logger: logs.logger()})
	evs := countEvents(ctx, c)
	go c.Run(ctx)
	eventually(t, "first greeting", func() bool { return evs.get(EventConnected) == 1 })

	srv.SetSilent(true) // the Cube restarts: no greeting for a while
	c.Rebooting(time.Minute)
	if l := c.Link(); l.State != LinkRebooting || l.Since.IsZero() || l.Until.IsZero() {
		t.Errorf("link %+v, want rebooting", l)
	}
	eventually(t, "the next connection", func() bool { return srv.Connections() == 2 })
	if l := c.Link(); l.State != LinkRebooting {
		t.Errorf("link %+v before the greeting, want rebooting", l)
	}
	srv.SetSilent(false)
	eventually(t, "the greeting after the reboot", func() bool { return evs.get(EventConnected) == 2 })
	if l := c.Link(); l.State != LinkConnected {
		t.Errorf("link %+v after the greeting", l)
	}
	if n := logs.count("WARN", "cube connection lost"); n != 0 || c.Lost() {
		t.Errorf("%d loss lines, lost %v", n, c.Lost())
	}
	if n := logs.count("INFO", "cube rebooting"); n != 1 {
		t.Errorf("%d rebooting lines", n)
	}
}
