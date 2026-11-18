// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !linux

package fakecube

import (
	"errors"
	"net"
)

// dropsSYN is false: outside Linux the fake accepts a refused client
// and closes the connection at once.
const dropsSYN = false

// fullBacklog is unused outside Linux.
const fullBacklog = 0

// setBacklog is Linux's alone.
func setBacklog(net.Listener, int) error { return errors.ErrUnsupported }

// holdPort is Linux's alone: elsewhere a paused fake keeps its listener
// and resets what it accepts (Pause), since whether a socket bound
// without listening refuses a dial there, rather than letting it time
// out, has not been tried.
func holdPort(net.Listener) (portHold, error) { return nil, errors.ErrUnsupported }
