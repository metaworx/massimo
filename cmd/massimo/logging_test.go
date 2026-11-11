// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
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
// settings file's line as the environment does (env.LoadFile, tested
// there), and refuses an unknown level.
func TestGlobalLogLevel(t *testing.T) {
	resetLogLevel(t)
	rest, err := globalOptions([]string{"-log-level", "debug", "watch", "-addr", "x"})
	if err != nil || len(rest) != 3 || rest[0] != "watch" {
		t.Fatalf("rest %v err %v", rest, err)
	}
	if v := os.Getenv(env.LogLevel); v != "debug" {
		t.Fatalf("%s = %q, want debug", env.LogLevel, v)
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

// A MASSIMO_LOG_LEVEL the settings file changes while serve runs waits
// for the next start (GET /api/v1/restart lists it), unless serve's
// -log-level fixed the level.
func TestReloadListsTheLogLevel(t *testing.T) {
	resetLogLevel(t)
	unsetForTest(t, env.LogLevel)
	p := filepath.Join(t.TempDir(), "env")
	os.WriteFile(p, nil, 0o600)
	t.Cleanup(func() { os.WriteFile(p, nil, 0o600); env.ReloadFile(p) }) // the env package forgets the test's names
	start := settings()
	free := newSettingsReloader(p, quietLog(), nil, start, map[string]bool{})
	fixed := newSettingsReloader(p, quietLog(), nil, start, map[string]bool{"log-level": true})
	os.WriteFile(p, []byte(env.LogLevel+"=debug\n"), 0o600)
	if _, err := env.ReloadFile(p); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(free.pending(), ","); got != env.LogLevel {
		t.Errorf("pending %q, want %s", got, env.LogLevel)
	}
	if got := fixed.pending(); len(got) != 0 {
		t.Errorf("with -log-level, pending %v", got)
	}
}

func TestParseLogFormat(t *testing.T) {
	for in, want := range map[string]logFormat{"": "", "json": formatJSON, " JSON ": formatJSON, "plain": formatPlain, "Plain": formatPlain} {
		if got, err := parseLogFormat(in, false); err != nil || got != want {
			t.Errorf("parseLogFormat(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if got, err := parseLogFormat("none", true); err != nil || got != formatNone {
		t.Errorf("none where it may be: %q, %v", got, err)
	}
	for _, in := range []string{"none", "text", "logfmt"} {
		if _, err := parseLogFormat(in, false); err == nil || !strings.Contains(err.Error(), logFormatNames) {
			t.Errorf("parseLogFormat(%q): %v, want an error naming the formats", in, err)
		}
	}
	if _, err := parseLogFormat("text", true); err == nil || !strings.Contains(err.Error(), "none") {
		t.Errorf("stderr's unknown format: %v, want an error naming none too", err)
	}
}

// The more specific setting wins over -log-format, and that over the
// default: JSON in the file, plain on stderr.
func TestLogFormatPrecedence(t *testing.T) {
	for _, c := range []struct {
		given         logFormatFlags
		file, process logFormat
	}{
		{logFormatFlags{}, formatJSON, formatPlain},
		{logFormatFlags{general: "plain"}, formatPlain, formatPlain},
		{logFormatFlags{general: "json"}, formatJSON, formatJSON},
		{logFormatFlags{general: "plain", file: "json"}, formatJSON, formatPlain},
		{logFormatFlags{general: "json", process: "plain"}, formatJSON, formatPlain},
		{logFormatFlags{file: "plain", process: "json"}, formatPlain, formatJSON},
		{logFormatFlags{general: "plain", process: "none"}, formatPlain, formatNone},
	} {
		file, process, err := c.given.formats()
		if err != nil || file != c.file || process != c.process {
			t.Errorf("%+v: file %q, stderr %q, %v; want %q, %q", c.given, file, process, err, c.file, c.process)
		}
	}
	for given, names := range map[logFormatFlags]string{
		{general: "none"}: "-log-format or " + env.LogFormat,
		{file: "none"}:    "-log-file-format or " + env.LogFileFormat,
		{process: "xml"}:  "-log-process-format or " + env.LogProcessFormat,
	} {
		if _, _, err := given.formats(); err == nil || !strings.Contains(err.Error(), names) {
			t.Errorf("%+v: %v, want an error naming %s", given, err, names)
		}
	}
}

// One record goes to stderr and to the file, each in its format, at the
// one level; none writes nothing on stderr.
func TestServeLoggerFormats(t *testing.T) {
	resetLogLevel(t)
	var stderr, file bytes.Buffer
	log, err := newServeLogger(&stderr, &file, formatJSON, formatPlain, "info", "level")
	if err != nil {
		t.Fatal(err)
	}
	log.Debug("hidden")
	log.Info("cube connected", "serial", "KEQ0000000")
	if got := stderr.String(); !strings.HasPrefix(got, "time=") || !strings.Contains(got, `msg="cube connected" serial=KEQ0000000`) || strings.Contains(got, "hidden") {
		t.Errorf("stderr:\n%s", got)
	}
	var line map[string]any
	if err := json.Unmarshal(file.Bytes(), &line); err != nil {
		t.Fatalf("file: %v\n%s", err, file.String())
	}
	if line["msg"] != "cube connected" || line["serial"] != "KEQ0000000" || line["level"] != "INFO" {
		t.Errorf("file: %v", line)
	}

	stderr.Reset()
	log, err = newServeLogger(&stderr, nil, formatJSON, formatNone, "info", "level")
	if err != nil {
		t.Fatal(err)
	}
	log.Error("lost")
	if stderr.Len() != 0 {
		t.Errorf("none on stderr wrote:\n%s", stderr.String())
	}
	if _, err := newServeLogger(&stderr, nil, formatJSON, formatPlain, "loud", "-log-level"); err == nil || !strings.Contains(err.Error(), "-log-level") {
		t.Errorf("unknown level: %v", err)
	}
}

// The log file is made 0640, and one others may read more widely is
// narrowed to it; a narrower one stays as it is.
func TestOpenLogFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows keeps the file's ACLs")
	}
	dir := t.TempDir()
	modeOf := func(p string) os.FileMode {
		t.Helper()
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		return fi.Mode().Perm()
	}
	for _, c := range []struct {
		name       string
		before     os.FileMode // 0: no file yet
		want, mask os.FileMode
	}{
		{"new.log", 0, 0o640, 0o640},
		{"wide.log", 0o644, 0o640, 0o777},
		{"open.log", 0o666, 0o640, 0o777},
		{"narrow.log", 0o600, 0o600, 0o777},
	} {
		p := filepath.Join(dir, c.name)
		if c.before != 0 {
			if err := os.WriteFile(p, []byte("time=x\n"), c.before); err != nil {
				t.Fatal(err)
			}
			os.Chmod(p, c.before) // past the umask
		}
		f, narrowErr, err := openLogFile(p)
		if err != nil || narrowErr != nil {
			t.Fatalf("%s: %v, %v", c.name, err, narrowErr)
		}
		f.Close()
		// A new file's mode is 0640 less the umask.
		if got := modeOf(p); got&^c.mask != 0 || (c.before != 0 && got != c.want) || got&0o600 != 0o600 {
			t.Errorf("%s: mode %#o, want %#o", c.name, got, c.want)
		}
	}
}

// serve writes plain lines on stderr and JSON ones into its -log file,
// made 0640.
func TestServeLogFileIsJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serve.log")
	s := startBuiltServe(t, []string{"MASSIMO_CUBE_ADDRESS=" + startFakeCube(t)}, "-log", path, "-proxy", "off")
	s.waitFor(t, "api listening")
	s.stop(t)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var line map[string]any
		if err := json.Unmarshal([]byte(l), &line); err != nil {
			t.Fatalf("not JSON: %v\n%s", err, l)
		}
		found = found || line["msg"] == "api listening"
	}
	if !found {
		t.Errorf("no api listening in the file:\n%s", b)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm()&^0o640 != 0 {
		t.Errorf("mode %v, %v; want 0640 at most", fi.Mode(), err)
	}
}
