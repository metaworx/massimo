// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !windows

package main

import "context"

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
