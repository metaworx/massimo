// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package announce

import "syscall"

// shareAddress lets the socket share its address and port with Windows'
// own mDNS: SO_REUSEADDR, which on Windows is what SO_REUSEPORT is
// elsewhere.
func shareAddress(network, address string, c syscall.RawConn) error {
	var err error
	if cerr := c.Control(func(fd uintptr) {
		err = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	}); cerr != nil {
		return cerr
	}
	return err
}
