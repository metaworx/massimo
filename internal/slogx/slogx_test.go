// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package slogx

import (
	"bytes"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
)

// AtDebug writes an Info and a Warn at debug, with what With adds, and
// nothing where the logger takes info and above only.
func TestAtDebug(t *testing.T) {
	var buf bytes.Buffer
	l := AtDebug(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})).With("source", "cube"))
	l.Info("cube connected", "addr", "127.0.0.1:62910")
	l.WithGroup("g").Warn("cube connection lost", "error", "eof")
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `level=DEBUG msg="cube connected" source=cube addr=127.0.0.1:62910`) ||
		!strings.Contains(lines[1], `level=DEBUG msg="cube connection lost" source=cube g.error=eof`) {
		t.Fatalf("at debug:\n%s", buf.String())
	}

	buf.Reset()
	quiet := AtDebug(slog.New(slog.NewTextHandler(&buf, nil)))
	if quiet.Enabled(t.Context(), slog.LevelError) {
		t.Error("enabled at info and above")
	}
	quiet.Warn("cube connection lost")
	if buf.Len() != 0 {
		t.Fatalf("written at info and above: %s", buf.String())
	}
}

// WithFunc reads its value on each record: none while it is empty, then
// the value learned after the logger was made, and a new one once it
// changes; each line as l.With(key, value) would write it, with what
// With and WithGroup add after, through AtDebug too, and only where l
// takes the level.
func TestWithFunc(t *testing.T) {
	var serial atomic.Value
	serial.Store("")
	value := func() string { return serial.Load().(string) }
	// text writes a logger's lines without their time, at debug and up
	// or at info and up.
	text := func(buf *bytes.Buffer, debug bool) *slog.Logger {
		opts := &slog.HandlerOptions{ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		}}
		if debug {
			opts.Level = slog.LevelDebug
		}
		return slog.New(slog.NewTextHandler(buf, opts)).With("source", "cube")
	}
	var got bytes.Buffer
	l := WithFunc(text(&got, true), "cube", value)
	grouped := l.With("addr", "127.0.0.1:62910").WithGroup("g")
	quiet := AtDebug(l)
	// write logs the same lines through the logger made over l and
	// through its parts.
	write := func(l, grouped, quiet *slog.Logger) {
		l.Info("cube reconnecting", "retry_in", "1s")
		grouped.Warn("poll failed", "error", "eof")
		quiet.Info("cube connected")
	}

	write(l, grouped, quiet)
	if s := got.String(); strings.Contains(s, "cube=") {
		t.Fatalf("a cube before its serial is known:\n%s", s)
	}
	for _, sn := range []string{"KEQ0000100", "KEQ0000101"} {
		serial.Store(sn)
		got.Reset()
		write(l, grouped, quiet)
		var want bytes.Buffer
		static := text(&want, true).With("cube", sn)
		write(static, static.With("addr", "127.0.0.1:62910").WithGroup("g"), AtDebug(static))
		if got.String() != want.String() || strings.Count(got.String(), " cube=") != 3 {
			t.Fatalf("with the serial %s:\n%s\nwant:\n%s", sn, got.String(), want.String())
		}
	}

	got.Reset()
	l = WithFunc(text(&got, false), "cube", value)
	if l.Enabled(t.Context(), slog.LevelDebug) || !l.Enabled(t.Context(), slog.LevelInfo) {
		t.Error("the levels are not the logger's")
	}
	AtDebug(l).Warn("cube connection lost")
	if got.Len() != 0 {
		t.Fatalf("written below the logger's level: %s", got.String())
	}
}
