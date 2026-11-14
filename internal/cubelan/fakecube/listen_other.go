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
