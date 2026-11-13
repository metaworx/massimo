// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !linux && !windows && !darwin

package cubelan

import (
	"net"
	"syscall"
	"time"
)

// keepAliveFor covers the hold with the keepalive's probes where the
// platform has no other limit to set.
func keepAliveFor(hold time.Duration) net.KeepAliveConfig { return keepAliveCovering(hold) }

// holdControl sets nothing here: the kernel's own retransmission limit
// may end a held connection before the hold does.
func holdControl(time.Duration) func(network, address string, c syscall.RawConn) error {
	return nil
}

// keepaliveSilent is Linux's alone.
var keepaliveSilent func(net.Conn) bool
