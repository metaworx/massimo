// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !unix

package certs

import "os/exec"

// ownGroup leaves the hook as it is: without process groups only the hook
// itself is ended when its time is up, and WaitDelay stops the wait for
// the children that keep its output open.
func ownGroup(*exec.Cmd) {}
