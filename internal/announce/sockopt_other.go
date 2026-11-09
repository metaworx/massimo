// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !(linux || darwin || dragonfly || freebsd || netbsd || openbsd || windows)

package announce

import "syscall"

// shareAddress does nothing where the socket options are not known: the
// port is then the server's alone, or not had.
func shareAddress(network, address string, c syscall.RawConn) error { return nil }
