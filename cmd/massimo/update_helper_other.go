// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !windows

package main

import "log/slog"

// updateHelperStarter returns nil: on Linux a systemd path unit and on
// macOS launchd start the update helper when the request file appears.
func updateHelperStarter(log *slog.Logger, exe, request, status, health, logPath string) (start func() error, blocked string) {
	return nil, ""
}

// restartHelperStarter returns nil: outside Windows the service manager
// starts the server again once it exits for a restart (restart.go).
func restartHelperStarter(log *slog.Logger, exe string) (start func() error, blocked string) {
	return nil, ""
}
