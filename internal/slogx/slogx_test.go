// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package slogx

import (
	"bytes"
	"log/slog"
	"strings"
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
