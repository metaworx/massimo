// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package slogx holds what the standard log/slog lacks for Massimo: a
// logger that writes every record at debug, for the lines another part
// writes as well (AP OneLog, Blocks 5 to 7), and a logger whose attribute
// is read as each record is written, for a value learned after the
// logger is made (AP StartConfiguration, Block 20). A part chooses its
// logger once, and its lines keep their own levels in the code, so that
// they read as plain Info and Warn calls.
package slogx

import (
	"context"
	"log/slog"
	"slices"
	"sync/atomic"
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

// WithFunc is l with the attribute key on each record, its value read
// from value as the record is written and the attribute left out while
// value returns "": for a value learned after the logger is made, such
// as the serial a Cube reports once it is connected. A record reads as
// one of l.With(key, v) would, v the value at that moment, whatever With
// and WithGroup add later.
func WithFunc(l *slog.Logger, key string, value func() string) *slog.Logger {
	h := l.Handler()
	return slog.New(&withFunc{base: h, key: key, value: value, plain: h})
}

// withFunc is the handler of WithFunc. Its handler with the value is
// built as the value changes, from base and what its With and WithGroup
// added since (after), and kept for the next record (last).
type withFunc struct {
	base  slog.Handler
	key   string
	value func() string
	after []func(slog.Handler) slog.Handler
	plain slog.Handler // base and after, for a record without the value
	last  atomic.Pointer[valued]
}

// valued is the handler of withFunc for the value v.
type valued struct {
	v string
	h slog.Handler
}

func (w *withFunc) Enabled(ctx context.Context, level slog.Level) bool {
	return w.plain.Enabled(ctx, level)
}

func (w *withFunc) Handle(ctx context.Context, r slog.Record) error {
	v := w.value()
	if v == "" {
		return w.plain.Handle(ctx, r)
	}
	c := w.last.Load()
	if c == nil || c.v != v {
		h := w.base.WithAttrs([]slog.Attr{slog.String(w.key, v)})
		for _, f := range w.after {
			h = f(h)
		}
		c = &valued{v: v, h: h}
		w.last.Store(c)
	}
	return c.h.Handle(ctx, r)
}

func (w *withFunc) WithAttrs(attrs []slog.Attr) slog.Handler {
	return w.then(func(h slog.Handler) slog.Handler { return h.WithAttrs(attrs) })
}

func (w *withFunc) WithGroup(name string) slog.Handler {
	if name == "" {
		return w
	}
	return w.then(func(h slog.Handler) slog.Handler { return h.WithGroup(name) })
}

// then is w with f applied after it, as With and WithGroup add.
func (w *withFunc) then(f func(slog.Handler) slog.Handler) slog.Handler {
	return &withFunc{base: w.base, key: w.key, value: w.value, after: append(slices.Clip(w.after), f), plain: f(w.plain)}
}
