// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !darwin

package main

import (
	"log/slog"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/server"
)

// serveBrewUpdate leaves the manager's reason: the web UI updates
// through brew only where brew services runs the server under launchd.
func serveBrewUpdate(log *slog.Logger, upd *server.UpdateOptions, m install.Manager, listen string, tls bool) {
}
