// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/logfile"
	"go.metaworx.ch/massimo/internal/server"
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

// logFormat is how serve writes its log to one output.
type logFormat string

// The formats: none writes nothing, and only stderr may be given it (the
// Mac's launchd, whose stderr file then catches what the Go runtime
// prints itself).
const (
	formatPlain logFormat = "plain"
	formatJSON  logFormat = "json"
	formatNone  logFormat = "none"
)

// logFormatNames are the formats -log-format and -log-file-format take.
const logFormatNames = "json or plain"

// parseLogFormat reads a format name in any case; empty is "", not
// given. none is a format only where noneOK.
func parseLogFormat(s string, noneOK bool) (logFormat, error) {
	switch f := logFormat(strings.ToLower(strings.TrimSpace(s))); f {
	case "", formatPlain, formatJSON:
		return f, nil
	case formatNone:
		if noneOK {
			return f, nil
		}
	}
	names := logFormatNames
	if noneOK {
		names = "json, plain or none"
	}
	return "", fmt.Errorf("unknown log format %q, want %s", s, names)
}

// logFormatFlags are serve's three format settings as given, each by its
// flag or else its variable.
type logFormatFlags struct {
	general, file, process string
}

// formats resolves the format of the log file and of stderr: the more
// specific setting wins over -log-format, and that over the default,
// JSON in the file and plain on stderr. An unknown value is an error
// naming its flag and variable.
func (g logFormatFlags) formats() (file, process logFormat, err error) {
	general, err := parseLogFormat(g.general, false)
	if err != nil {
		return "", "", fmt.Errorf("-log-format or %s: %w", env.LogFormat, err)
	}
	if file, err = parseLogFormat(g.file, false); err != nil {
		return "", "", fmt.Errorf("-log-file-format or %s: %w", env.LogFileFormat, err)
	}
	if process, err = parseLogFormat(g.process, true); err != nil {
		return "", "", fmt.Errorf("-log-process-format or %s: %w", env.LogProcessFormat, err)
	}
	file = cmp.Or(file, general, formatJSON)
	process = cmp.Or(process, general, formatPlain)
	return file, process, nil
}

// logHandler is a handler writing format to w at the shared level.
func logHandler(w io.Writer, format logFormat) slog.Handler {
	opts := &slog.HandlerOptions{Level: &logLevel}
	if format == formatJSON {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}

// newServeLogger returns serve's logger at level (what names it in an
// error): one record written to stderr in its format, unless none, and
// to the log file, when there is one, in the file's.
func newServeLogger(stderr, file io.Writer, fileFormat, processFormat logFormat, level, what string) (*slog.Logger, error) {
	l, err := parseLogLevel(level)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	logLevel.Set(l)
	var hs []slog.Handler
	if processFormat != formatNone {
		hs = append(hs, logHandler(stderr, processFormat))
	}
	if file != nil {
		hs = append(hs, logHandler(file, fileFormat))
	}
	switch len(hs) {
	case 0:
		return slog.New(slog.DiscardHandler), nil
	case 1:
		return slog.New(hs[0]), nil
	}
	return slog.New(slog.NewMultiHandler(hs...)), nil
}

// parseLogSize reads a size of -log-rotate-size and -log-keep-min: bytes,
// or with k, M or G (powers of 1024, with or without a trailing B or
// iB); empty is def.
func parseLogSize(s string, def int64) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return def, nil
	}
	l := strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(s), "b"), "i")
	mult := int64(1)
	if i := len(l) - 1; i >= 0 {
		if m, ok := map[byte]int64{'k': 1 << 10, 'm': 1 << 20, 'g': 1 << 30}[l[i]]; ok {
			mult, l = m, l[:i]
		}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(l), 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("unknown size %q, want bytes or a number with k, M or G", s)
	}
	return n * mult, nil
}

// logRotation is how serve rotates its log file: the flags as given, each
// by its flag or else its variable.
type logRotation struct {
	size, keep, keepMin string
}

// options returns the rotation's logfile options; an unknown value is an
// error naming its flag and variable.
func (g logRotation) options() (size int64, keep int, keepMin int64, err error) {
	if size, err = parseLogSize(g.size, logfile.DefaultSize); err != nil {
		return 0, 0, 0, fmt.Errorf("-log-rotate-size or %s: %w", env.LogRotateSize, err)
	}
	keep = logfile.DefaultKeep
	if v := strings.TrimSpace(g.keep); v != "" {
		if keep, err = strconv.Atoi(v); err != nil || keep < 0 {
			return 0, 0, 0, fmt.Errorf("-log-keep or %s: %q is no number of files", env.LogKeep, g.keep)
		}
	}
	if keepMin, err = parseLogSize(g.keepMin, logfile.DefaultKeepMin); err != nil {
		return 0, 0, 0, fmt.Errorf("-log-keep-min or %s: %w", env.LogKeepMin, err)
	}
	return size, keep, keepMin, nil
}

// logRotated writes a rotation of the log file on stderr, as the new
// file's first line, which the file wrote itself, says it; at warn where
// a step failed.
func logRotated(stderr *slog.Logger, r logfile.Rotation) {
	level := slog.LevelInfo
	if r.Err != nil {
		level = slog.LevelWarn
	}
	stderr.LogAttrs(context.Background(), level, "log rotated", r.Attrs()...)
}

// newStderrLogger is serve's logger without the file, for the lines the
// file writes itself: stderr in its format, at the shared level.
func newStderrLogger(stderr io.Writer, processFormat logFormat) *slog.Logger {
	if processFormat == formatNone {
		return slog.New(slog.DiscardHandler)
	}
	return slog.New(logHandler(stderr, processFormat))
}

// rotationSink hands the log file's rotations to the server's log, those
// before the server runs kept until it does.
type rotationSink struct {
	mu      sync.Mutex
	srv     *server.Server
	pending []logfile.Rotation
}

func (k *rotationSink) add(r logfile.Rotation) {
	k.mu.Lock()
	srv := k.srv
	if srv == nil {
		k.pending = append(k.pending, r)
	}
	k.mu.Unlock()
	if srv != nil {
		srv.LogRotated(r.Reason, r.Previous, r.Kept, r.Removed, r.Err)
	}
}

func (k *rotationSink) set(srv *server.Server) {
	k.mu.Lock()
	k.srv = srv
	pending := k.pending
	k.pending = nil
	k.mu.Unlock()
	for _, r := range pending {
		srv.LogRotated(r.Reason, r.Previous, r.Kept, r.Removed, r.Err)
	}
}
