// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.metaworx.ch/massimo/internal/env"
)

// logLevel is the level of every logger newLogger makes, one for the
// process, so that a setting could change it while serve runs.
var logLevel slog.LevelVar

// logLevelNames are the levels -log-level and MASSIMO_LOG_LEVEL take.
const logLevelNames = "debug, info, warn or error"

// parseLogLevel reads a level name in any case; empty is info.
func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("unknown log level %q, want %s", s, logLevelNames)
}

// setLogLevelVar checks a level given with the global -log-level and
// puts it into MASSIMO_LOG_LEVEL, where it wins over the settings
// file's line, as -data does over MASSIMO_DATA's.
func setLogLevelVar(v string) error {
	if _, err := parseLogLevel(v); err != nil {
		return err
	}
	return os.Setenv(env.LogLevel, v)
}

// newLogger returns a text logger on w at the level of MASSIMO_LOG_LEVEL,
// else info; an unknown level is an error.
func newLogger(w io.Writer) (*slog.Logger, error) {
	return newLoggerAt(w, os.Getenv(env.LogLevel), env.LogLevel)
}

// newLoggerAt is newLogger at level, which what names in an error: serve
// takes it from its -log-level, whose default is MASSIMO_LOG_LEVEL.
func newLoggerAt(w io.Writer, level, what string) (*slog.Logger, error) {
	l, err := parseLogLevel(level)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	logLevel.Set(l)
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: &logLevel})), nil
}
