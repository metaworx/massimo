// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

package announce

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// shareAddress lets the socket share its address and port with the
// system's responder: SO_REUSEADDR, and SO_REUSEPORT, which macOS'
// mDNSResponder and a newer avahi set.
func shareAddress(network, address string, c syscall.RawConn) error {
	var err error
	if cerr := c.Control(func(fd uintptr) {
		if err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err == nil {
			err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		}
	}); cerr != nil {
		return cerr
	}
	return err
}
