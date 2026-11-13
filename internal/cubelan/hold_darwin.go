// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"net"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// keepAliveFor is the keepalive of a held connection on macOS, which
// has no timeout that governs the probes: one every 15 s after 15 s of
// silence, as many as cover the hold (at most 255). Without a hold it
// is keepAlive.
func keepAliveFor(hold time.Duration) net.KeepAliveConfig { return keepAliveCovering(hold) }

// holdControl sets TCP_RXT_CONNDROPTIME, after how long the kernel drops
// a connection whose retransmissions go unanswered, to the hold in
// seconds.
func holdControl(hold time.Duration) func(network, address string, c syscall.RawConn) error {
	if hold <= 0 {
		return nil
	}
	return func(_, _ string, c syscall.RawConn) error {
		var serr error
		if err := c.Control(func(fd uintptr) {
			serr = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_RXT_CONNDROPTIME, holdSeconds(hold))
		}); err != nil {
			return err
		}
		return serr
	}
}

// keepaliveSilent is Linux's alone (TCP_INFO): elsewhere the missed
// polls mark a loss.
var keepaliveSilent func(net.Conn) bool
