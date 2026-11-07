// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/packaging"
)

// envOf is a getenv over a map.
func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// Who starts the server again: the service manager that runs it, found
// by its traces, or whatever -restart exit trusts; nobody in a terminal.
func TestRestartBy(t *testing.T) {
	for _, c := range []struct {
		name, mode string
		p          processInfo
		by         string
		blocked    bool
	}{
		{"systemd", "", processInfo{goos: "linux", getenv: envOf(map[string]string{"INVOCATION_ID": "0123abcd"}), ppid: 1}, bySystemd, false},
		{"systemd, auto", "auto", processInfo{goos: "linux", getenv: envOf(map[string]string{"INVOCATION_ID": "0123abcd"}), ppid: 1}, bySystemd, false},
		{"linux terminal", "", processInfo{goos: "linux", getenv: envOf(nil), ppid: 4242}, "", true},
		{"linux, parent 1 without systemd", "", processInfo{goos: "linux", getenv: envOf(nil), ppid: 1}, "", true},
		{"launchd daemon", "", processInfo{goos: "darwin", getenv: envOf(map[string]string{"XPC_SERVICE_NAME": install.Label}), ppid: 1}, byLaunchd, false},
		{"brew services", "", processInfo{goos: "darwin", getenv: envOf(map[string]string{"XPC_SERVICE_NAME": homebrewLabel}), ppid: 1}, byLaunchd, false},
		{"launchd, parent only", "", processInfo{goos: "darwin", getenv: envOf(nil), ppid: 1}, byLaunchd, false},
		{"macOS terminal", "", processInfo{goos: "darwin", getenv: envOf(map[string]string{"XPC_SERVICE_NAME": "application.com.apple.Terminal.0000"}), ppid: 4242}, "", true},
		{"windows service", "", processInfo{goos: "windows", getenv: envOf(nil), service: true}, byWindows, false},
		{"windows console", "", processInfo{goos: "windows", getenv: envOf(nil), ppid: 4242}, "", true},
		{"exit in a terminal", "exit", processInfo{goos: "linux", getenv: envOf(nil), ppid: 4242}, byExit, false},
		{"exit as a service", "exit", processInfo{goos: "windows", getenv: envOf(nil), service: true}, byExit, false},
		{"off under systemd", "off", processInfo{goos: "linux", getenv: envOf(map[string]string{"INVOCATION_ID": "0123abcd"}), ppid: 1}, "", true},
	} {
		by, blocked, err := restartBy(c.mode, c.p)
		if err != nil || by != c.by || (blocked != "") != c.blocked {
			t.Errorf("%s: by %q, blocked %q, %v; want by %q, blocked %v", c.name, by, blocked, err, c.by, c.blocked)
		}
	}
	if _, _, err := restartBy("always", processInfo{goos: "linux", getenv: envOf(nil)}); err == nil || !strings.Contains(err.Error(), "want auto, exit or off") {
		t.Fatalf("a bad -restart: %v", err)
	}
}

// A restart ends serve's context and turns its end into errRestart, which
// exits 75; a Windows helper is started instead of ending anything.
func TestRestarter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &restarter{by: bySystemd, cancel: cancel}
	if err := r.result(nil); err != nil {
		t.Fatalf("before a restart: %v", err)
	}
	if err := r.restart(); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() == nil {
		t.Fatal("the restart did not end serve's context")
	}
	if err := r.result(nil); !errors.Is(err, errRestart) {
		t.Fatalf("after a restart: %v", err)
	}
	failed := errors.New("listen: address in use")
	if err := r.result(failed); err != failed {
		t.Fatalf("a failure is reported as it is: %v", err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	started := 0
	w := &restarter{by: byWindows, cancel: cancel, helper: func() error { started++; return nil }}
	if err := w.restart(); err != nil || started != 1 || ctx.Err() != nil || w.result(nil) != nil {
		t.Fatalf("windows: started %d, context %v", started, ctx.Err())
	}
	cancel()

	for err, want := range map[error]int{nil: 0, errRestart: exitRestart, fmt.Errorf("serve: %w", errRestart): exitRestart, failed: 1} {
		if got := exitCode(err); got != want {
			t.Errorf("exitCode(%v) = %d, want %d", err, got, want)
		}
	}
}

// The installed unit restarts on the exit status of a restart and does
// not count it as a failure.
func TestUnitRestartsOnExitRestart(t *testing.T) {
	b, err := packaging.Files.ReadFile("systemd/massimo.service")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"Restart=on-failure", fmt.Sprintf("RestartForceExitStatus=%d", exitRestart), fmt.Sprintf("SuccessExitStatus=%d", exitRestart)} {
		if !strings.Contains(string(b), "\n"+line+"\n") {
			t.Errorf("massimo.service lacks %s", line)
		}
	}
}
