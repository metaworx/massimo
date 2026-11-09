// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !windows

package main

import (
	"context"
	"os"
	"syscall"
)

// runningAsService is false: only a Windows service is started by a
// manager that the process must answer.
const runningAsService = false

// euid is os.Geteuid; a variable so that a test runs as another user.
var euid = os.Geteuid

// serviceUser reports whether this process runs as the service user:
// not root, and the owner of envPath, the service's settings file, which
// install and the Debian package give to the service account. A missing
// file, or one this user cannot see, is no service user.
func serviceUser(envPath string) bool {
	uid := euid()
	if uid == 0 {
		return false
	}
	fi, err := os.Stat(envPath)
	if err != nil {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int64(st.Uid) == int64(uid)
}

// serviceDataDir is the data directory of a Windows service started
// without -data; elsewhere the installers always pass -data.
func serviceDataDir() string { return "" }

// serviceContext is ctx: only a Windows service is stopped by its
// manager rather than by a signal.
func serviceContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctx, func() {}
}

// serviceEnd does nothing: only a Windows service reports its stop.
func serviceEnd() {}
