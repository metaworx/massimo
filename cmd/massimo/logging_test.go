// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/env"
)

// resetLogLevel clears MASSIMO_LOG_LEVEL for the test, and puts the
// shared level back to info afterwards.
func resetLogLevel(t *testing.T) {
	t.Helper()
	t.Setenv(env.LogLevel, "")
	t.Cleanup(func() { logLevel.Set(slog.LevelInfo) })
}

func TestParseLogLevel(t *testing.T) {
	for in, want := range map[string]slog.Level{
		"debug": slog.LevelDebug, "DEBUG": slog.LevelDebug, " Debug ": slog.LevelDebug,
		"info": slog.LevelInfo, "": slog.LevelInfo,
		"warn": slog.LevelWarn, "Warn": slog.LevelWarn,
		"error": slog.LevelError,
	} {
		if got, err := parseLogLevel(in); err != nil || got != want {
			t.Errorf("parseLogLevel(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"verbose", "warning", "debug-4", "0"} {
		if _, err := parseLogLevel(in); err == nil || !strings.Contains(err.Error(), logLevelNames) {
			t.Errorf("parseLogLevel(%q): %v, want an error naming the levels", in, err)
		}
	}
}

// newLogger logs at the level of MASSIMO_LOG_LEVEL, info without it, and
// refuses an unknown one.
func TestNewLoggerLevel(t *testing.T) {
	resetLogLevel(t)
	for _, c := range []struct {
		level   string
		written []string
		skipped []string
	}{
		{"", []string{"msg=info"}, []string{"msg=debug"}},
		{"debug", []string{"msg=debug", "msg=info"}, nil},
		{"warn", []string{"msg=warn"}, []string{"msg=info"}},
		{"error", []string{"msg=error"}, []string{"msg=warn"}},
	} {
		t.Setenv(env.LogLevel, c.level)
		var buf bytes.Buffer
		log, err := newLogger(&buf)
		if err != nil {
			t.Fatalf("%q: %v", c.level, err)
		}
		log.Debug("debug")
		log.Info("info")
		log.Warn("warn")
		log.Error("error")
		for _, w := range c.written {
			if !strings.Contains(buf.String(), w) {
				t.Errorf("at %q, %s not written:\n%s", c.level, w, buf.String())
			}
		}
		for _, s := range c.skipped {
			if strings.Contains(buf.String(), s+"\n") {
				t.Errorf("at %q, %s written:\n%s", c.level, s, buf.String())
			}
		}
	}
	t.Setenv(env.LogLevel, "verbose")
	if _, err := newLogger(&bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), env.LogLevel) {
		t.Errorf("unknown level: %v, want an error naming %s", err, env.LogLevel)
	}
}

// The global -log-level sets MASSIMO_LOG_LEVEL, which then wins over the
// settings file's line, and refuses an unknown level.
func TestGlobalLogLevel(t *testing.T) {
	resetLogLevel(t)
	rest, err := globalOptions([]string{"-log-level", "debug", "watch", "-addr", "x"})
	if err != nil || len(rest) != 3 || rest[0] != "watch" {
		t.Fatalf("rest %v err %v", rest, err)
	}
	if v := os.Getenv(env.LogLevel); v != "debug" {
		t.Fatalf("%s = %q, want debug", env.LogLevel, v)
	}
	file := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(file, []byte(env.LogLevel+"=error\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := env.LoadFile(file); err != nil {
		t.Fatal(err)
	}
	if v := os.Getenv(env.LogLevel); v != "debug" {
		t.Fatalf("after the settings file, %s = %q, want the option's debug", env.LogLevel, v)
	}
	if _, err := globalOptions([]string{"--log-level=loud", "watch"}); err == nil || !strings.Contains(err.Error(), "-log-level") {
		t.Fatalf("unknown level: %v", err)
	}
}

// An unknown level stops serve at the start, from the flag or from the
// variable, before it opens anything.
func TestServeRefusesAnUnknownLogLevel(t *testing.T) {
	clearCubeEnv(t)
	resetLogLevel(t)
	args := []string{"-addr", "127.0.0.1:1", "-listen", "127.0.0.1:0", "-proxy", "off"}
	if err := runServe(append([]string{"-log-level", "loud"}, args...)); err == nil || !strings.Contains(err.Error(), "loud") {
		t.Errorf("-log-level loud: %v", err)
	}
	t.Setenv(env.LogLevel, "verbose")
	if err := runServe(args); err == nil || !strings.Contains(err.Error(), env.LogLevel) {
		t.Errorf("%s=verbose: %v", env.LogLevel, err)
	}
}

// serve's -log-level wins over MASSIMO_LOG_LEVEL: at error the start's
// Info lines would be missing, and at debug the proxy logs a malformed
// line from its client.
func TestServeLogLevelFlagWins(t *testing.T) {
	proxyAddr := freeAddr(t)
	s := startBuiltServe(t, []string{"MASSIMO_CUBE_ADDRESS=" + startFakeCube(t), env.LogLevel + "=error"}, "-log-level", "debug", "-proxy", proxyAddr)
	s.waitFor(t, "cube proxy listening")
	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("zz\r\n")); err != nil {
		t.Fatal(err)
	}
	if a := s.waitFor(t, "proxy: ignoring malformed line"); a["level"] != "DEBUG" {
		t.Errorf("logged at %q, want DEBUG", a["level"])
	}
	s.stop(t)
}
