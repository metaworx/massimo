// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !windows

package install

import "errors"

var errNotWindows = errors.New("only on Windows")

// InstallWindows installs massimo as a Windows service; only on Windows.
func InstallWindows(Options) error { return errNotWindows }

// RemoveWindows removes the Windows service; only on Windows.
func RemoveWindows(Options, bool) error { return errNotWindows }

// RestartService restarts a Windows service; only on Windows.
func RestartService(string) error { return errNotWindows }
