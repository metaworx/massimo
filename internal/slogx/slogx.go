// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package slogx holds what the standard log/slog lacks for Massimo: a
// logger that writes every record at debug, for the lines another part
// writes as well (AP OneLog, Blocks 5 to 7). A part chooses its logger
// once, and its lines keep their own levels in the code, so that they
// read as plain Info and Warn calls.
package slogx

import (
	"context"
	"log/slog"
)

// AtDebug is l writing every record at debug, whatever level it was
// logged at: for a line the caller's own log says already, such as a
// connection serve's server logs as its event.
func AtDebug(l *slog.Logger) *slog.Logger { return slog.New(atDebug{l.Handler()}) }

// atDebug is the handler of AtDebug.
type atDebug struct{ h slog.Handler }

func (a atDebug) Enabled(ctx context.Context, _ slog.Level) bool {
	return a.h.Enabled(ctx, slog.LevelDebug)
}

func (a atDebug) Handle(ctx context.Context, r slog.Record) error {
	r.Level = slog.LevelDebug
	return a.h.Handle(ctx, r)
}

func (a atDebug) WithAttrs(attrs []slog.Attr) slog.Handler { return atDebug{a.h.WithAttrs(attrs)} }

func (a atDebug) WithGroup(name string) slog.Handler { return atDebug{a.h.WithGroup(name)} }
