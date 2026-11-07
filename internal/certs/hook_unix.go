// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build unix

package certs

import (
	"os/exec"
	"syscall"
)

// ownGroup runs the hook in a process group of its own and ends the whole
// group when its time is up, so that a child the hook started (a curl
// waiting on a stalled API) goes with it.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
