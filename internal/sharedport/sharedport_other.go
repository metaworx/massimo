// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !(linux || darwin || dragonfly || freebsd || netbsd || openbsd || windows)

package sharedport

import "syscall"

// Control does nothing where the socket options are not known: the port
// is then the socket's alone, or not had.
func Control(network, address string, c syscall.RawConn) error { return nil }
