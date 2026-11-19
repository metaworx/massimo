// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"context"
	"net"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// listenSharing listens on addr with SO_REUSEPORT, as a socket of the
// same user that asks to share a port does.
func listenSharing(addr string) (net.Listener, error) {
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		if err := c.Control(func(fd uintptr) { serr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1) }); err != nil {
			return err
		}
		return serr
	}}
	return lc.Listen(context.Background(), "tcp", addr)
}

// Once resumed, the fake's port is its listener's alone, as before the
// pause: a listener that asks to share it with SO_REUSEPORT cannot join,
// as it can while the pause holds the port (holdPort), a second time
// too.
func TestResumedPortIsNotShared(t *testing.T) {
	t.Parallel()
	srv, addr := startFake(t)
	for round := range 2 {
		srv.Pause()
		if ln, err := listenSharing(addr); err != nil {
			t.Fatalf("round %d: the paused port refused a sharing listener: %v; the check below would prove nothing", round, err)
		} else {
			ln.Close()
		}
		if err := srv.Resume(); err != nil {
			t.Fatal(err)
		}
		if ln, err := listenSharing(addr); err == nil {
			ln.Close()
			t.Fatalf("round %d: a sharing listener joined %s after the Resume", round, addr)
		}
		keepsItsPort(t, addr)
		dial(t, addr)
	}
}
