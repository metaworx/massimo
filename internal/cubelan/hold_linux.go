// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"net"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// keepAliveFor is keepAlive on Linux, whatever the hold:
// TCP_USER_TIMEOUT (holdControl) then decides when the kernel gives up,
// and ignores the probe count, so the probes go on every 5 s, and a Cube
// that restarted answers the next one with a reset.
func keepAliveFor(time.Duration) net.KeepAliveConfig { return keepAlive }

// holdControl sets TCP_USER_TIMEOUT to the hold: how long data or
// keepalive probes may go unacknowledged before the kernel ends the
// connection; Linux's own limit for data is about 15 minutes.
func holdControl(hold time.Duration) func(network, address string, c syscall.RawConn) error {
	if hold <= 0 {
		return nil
	}
	return func(_, _ string, c syscall.RawConn) error {
		var serr error
		if err := c.Control(func(fd uintptr) {
			serr = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, int(hold.Milliseconds()))
		}); err != nil {
			return err
		}
		return serr
	}
}

// keepaliveSilent reports whether the connection's keepalive probes
// went unanswered, keepAlive.Count of them: the kernel's count of
// unanswered probes (TCP_INFO's tcpi_probes), which falls back to 0
// when the Cube acknowledges anything. It marks a loss at about 30 s
// on an idle connection, as the keepalive's end of it did before the
// hold.
func keepaliveSilent(conn net.Conn) bool {
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		return false
	}
	raw, err := tc.SyscallConn()
	if err != nil {
		return false
	}
	var info *unix.TCPInfo
	if err := raw.Control(func(fd uintptr) {
		info, _ = unix.GetsockoptTCPInfo(int(fd), unix.IPPROTO_TCP, unix.TCP_INFO)
	}); err != nil || info == nil {
		return false
	}
	return int(info.Probes) >= keepAlive.Count
}
