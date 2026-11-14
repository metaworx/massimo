// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

package sharedport

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// Control sets SO_REUSEADDR, which Linux needs on both sockets, and
// SO_REUSEPORT, which macOS and the BSDs need on both for a wildcard
// port (and which macOS' mDNSResponder and a newer avahi set).
func Control(network, address string, c syscall.RawConn) error {
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
