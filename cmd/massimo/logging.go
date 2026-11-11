// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"cmp"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
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

// logFileMode is the log file's mode: its owner writes it, the admin
// group (the directory's, set-group-id, massimo install) reads it.
const logFileMode = 0o640

// openLogFile opens the log file for appending, made logFileMode, and
// narrows one that others may read more widely to logFileMode; the
// narrowing's error, as for a file of another user's, is returned beside
// the file, which is open all the same. Windows keeps its ACLs.
func openLogFile(path string) (f *os.File, narrowErr error, err error) {
	f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logFileMode)
	if err != nil {
		return nil, nil, err
	}
	if runtime.GOOS == "windows" {
		return f, nil, nil
	}
	fi, err := f.Stat()
	if err != nil {
		return f, err, nil
	}
	if perm := fi.Mode().Perm(); perm&^logFileMode != 0 {
		narrowErr = f.Chmod(perm & logFileMode)
	}
	return f, narrowErr, nil
}
