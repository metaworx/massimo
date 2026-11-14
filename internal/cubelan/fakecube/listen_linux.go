// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"errors"
	"net"

	"golang.org/x/sys/unix"
)

// dropsSYN says that a full accept queue makes the kernel drop a SYN
// without an answer, so that a refused client's dial times out as it
// does against a Cube that holds another session.
const dropsSYN = true

// fullBacklog is the backlog restored once the slot is free; the kernel
// caps it at net.core.somaxconn.
const fullBacklog = 4096

// setBacklog changes the backlog of a listening socket: Linux applies a
// second listen() to a socket that listens already.
func setBacklog(ln net.Listener, n int) error {
	tl, ok := ln.(*net.TCPListener)
	if !ok {
		return errors.ErrUnsupported
	}
	raw, err := tl.SyscallConn()
	if err != nil {
		return err
	}
	var lerr error
	if err := raw.Control(func(fd uintptr) { lerr = unix.Listen(int(fd), n) }); err != nil {
		return err
	}
	return lerr
}
