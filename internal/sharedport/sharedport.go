// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package sharedport opens a UDP port that other programs can open too:
// the mDNS port, which the system's responder holds, and the Cubes'
// discovery port, which the original MAX! software opens beside serve.
// Both sockets must set the options for the port to be shared; a socket
// bound without them keeps the port to itself, and one that comes after
// it is refused. A datagram sent to the shared port's unicast address
// reaches one of the sockets only (Windows: the one bound first; Linux:
// the one bound last, or one chosen by a hash of the sender with
// SO_REUSEPORT); a broadcast or a multicast reaches all of them.
package sharedport

import (
	"context"
	"net"
)

// ListenPacket opens address on network with the port shared (Control).
func ListenPacket(network, address string) (net.PacketConn, error) {
	lc := net.ListenConfig{Control: Control}
	return lc.ListenPacket(context.Background(), network, address)
}
