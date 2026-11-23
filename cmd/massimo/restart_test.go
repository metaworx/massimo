// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/packaging"
)

// envOf is a getenv over a map.
func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// Who starts the server again: the service manager that runs it, found
// by its traces, or whatever -restart exit trusts; nobody in a terminal.
func TestRestartBy(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, mode string
		p          processInfo
		by         string
		blocked    bool
	}{
		{"systemd", "", processInfo{goos: "linux", getenv: envOf(map[string]string{"INVOCATION_ID": "0123abcd", "SYSTEMD_EXEC_PID": "4242"}), pid: 4242, ppid: 1}, bySystemd, false},
		{"systemd, auto", "auto", processInfo{goos: "linux", getenv: envOf(map[string]string{"INVOCATION_ID": "0123abcd", "SYSTEMD_EXEC_PID": "4242"}), pid: 4242, ppid: 1}, bySystemd, false},
		{"systemd before 248", "", processInfo{goos: "linux", getenv: envOf(map[string]string{"INVOCATION_ID": "0123abcd"}), pid: 4242, ppid: 1}, bySystemd, false},
		{"a terminal of a user service", "", processInfo{goos: "linux", getenv: envOf(map[string]string{"INVOCATION_ID": "0123abcd", "SYSTEMD_EXEC_PID": "1717"}), pid: 4242, ppid: 4000}, "", true},
		{"a child of a service, old systemd", "", processInfo{goos: "linux", getenv: envOf(map[string]string{"INVOCATION_ID": "0123abcd"}), pid: 4242, ppid: 4000}, "", true},
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
	if _, _, err := restartBy("always", processInfo{goos: "linux", getenv: envOf(nil)}); err == nil || err.Error() != `-restart or MASSIMO_RESTART "always": want auto, exit or off` {
		t.Fatalf("a bad -restart: %v", err)
	}
}

// How the server was installed: the package manager that owns the binary
// first, else the service manager that started it, else other.
func TestInstallKind(t *testing.T) {
	t.Parallel()
	systemd := processInfo{goos: "linux", getenv: envOf(map[string]string{"INVOCATION_ID": "0123abcd", "SYSTEMD_EXEC_PID": "4242"}), pid: 4242, ppid: 1}
	for _, c := range []struct {
		name     string
		m        install.Manager
		packaged bool
		p        processInfo
		want     string
	}{
		{"the deb under systemd", install.Manager{Name: "apt"}, true, systemd, "apt"},
		{"brew services", install.Manager{Name: "brew"}, true, processInfo{goos: "darwin", getenv: envOf(map[string]string{"XPC_SERVICE_NAME": homebrewLabel}), ppid: 1}, "brew"},
		{"massimo install's unit", install.Manager{}, false, systemd, server.InstallSystemd},
		{"massimo install's daemon", install.Manager{}, false, processInfo{goos: "darwin", getenv: envOf(map[string]string{"XPC_SERVICE_NAME": install.Label}), ppid: 1}, server.InstallLaunchd},
		{"the Windows service", install.Manager{}, false, processInfo{goos: "windows", getenv: envOf(nil), service: true}, server.InstallWindowsService},
		{"a terminal", install.Manager{}, false, processInfo{goos: "linux", getenv: envOf(nil), ppid: 4242}, server.InstallOther},
		{"a Windows console", install.Manager{}, false, processInfo{goos: "windows", getenv: envOf(nil), ppid: 4242}, server.InstallOther},
	} {
		if got := installKind(c.m, c.packaged, c.p); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// A restart ends serve's context and turns its end into errRestart, which
// exits 75; a Windows helper is started instead of ending anything.
func TestRestarter(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// A restart from the API, end to end: an open server asked from the box
// answers 202, stops, closes its store with the request's audit row in
// it, and exits with exitRestart for its service manager.
func TestServeRestartsThroughTheAPI(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("a Windows service restarts through massimo restart")
	}
	if testing.Short() {
		t.Skip("builds the binary")
	}
	data := t.TempDir()
	cmd := exec.Command(builtBinary(t), "-data", data, "serve", "-listen", "127.0.0.1:0", "-discover-interval", "0")
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "MASSIMO_") && !strings.HasPrefix(kv, "INVOCATION_ID=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, "MASSIMO_ENV_FILE="+os.DevNull, restartVar+"="+restartExit)
	s := runBuilt(t, cmd)
	r, _ := s.log.Await(t, 0, "the start, with -restart exit", testkit.Msg("api listening", "source", "system", "restart", "exit"), serveWait)
	base := "http://" + r.Attrs["addr"]
	m := s.log.Mark()
	resp, err := http.Post(base+"/api/v1/restart", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), `"by": "exit"`) {
		t.Fatalf("POST: %d %s", resp.StatusCode, body)
	}
	select {
	case <-s.exited:
		var ee *exec.ExitError
		if !errors.As(s.err, &ee) || ee.ExitCode() != exitRestart {
			t.Fatalf("serve ended with %v, want exit status %d", s.err, exitRestart)
		}
	case <-time.After(testkit.Scale(20 * time.Second)):
		t.Fatal("serve did not end for the restart")
	}
	// Once serve ended, its log is read whole.
	s.log.Has(t, m, "the stop for a restart", testkit.Msg("stopped for a restart"))
	s.log.HasNot(t, 0, "an error", testkit.Contains("error:"))
	if _, err := os.Stat(filepath.Join(data, "massimo.db-wal")); err == nil {
		t.Fatal("the store was not closed: its -wal file is left")
	}
	st, err := store.Open(data, store.Options{Version: "test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	rows, err := st.AuditList(t.Context(), time.Time{}, 10, "")
	if err != nil || len(rows) == 0 || rows[0].Action != "server.restart" {
		t.Fatalf("the audit after the restart: %+v %v", rows, err)
	}
}
