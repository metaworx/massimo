// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/store"
)

// Who reboots: the Windows service itself; systemd's and launchd's
// services through their helper, which is there once its mark is;
// nobody for Homebrew's service, a terminal, or Linux without systemd.
func TestRebootBy(t *testing.T) {
	root := t.TempDir()
	unit := processInfo{goos: "linux", getenv: envOf(map[string]string{"INVOCATION_ID": "0123abcd", "SYSTEMD_EXEC_PID": "4242"}), pid: 4242, ppid: 1}
	daemon := processInfo{goos: "darwin", getenv: envOf(map[string]string{"XPC_SERVICE_NAME": install.Label}), pid: 4242, ppid: 1}
	for _, c := range []struct {
		name    string
		p       processInfo
		by      string
		blocked string
		helper  bool
	}{
		{"windows service", processInfo{goos: "windows", getenv: envOf(nil), service: true}, byWindows, "", false},
		{"systemd", unit, bySystemd, "", true},
		{"launchd", daemon, byLaunchd, "", true},
		{"homebrew", processInfo{goos: "darwin", getenv: envOf(map[string]string{"XPC_SERVICE_NAME": homebrewLabel}), pid: 4242, ppid: 1}, "", "Homebrew's service", false},
		{"homebrew alpha", processInfo{goos: "darwin", getenv: envOf(map[string]string{"XPC_SERVICE_NAME": homebrewLabel + "-alpha"}), pid: 4242, ppid: 1}, "", "Homebrew's service", false},
		{"terminal", processInfo{goos: "linux", getenv: envOf(nil), pid: 4242, ppid: 4000}, "", "only a server the system runs as a service", false},
		{"windows terminal", processInfo{goos: "windows", getenv: envOf(nil), pid: 4242, ppid: 4000}, "", "only a server the system runs as a service", false},
		{"a user's systemd shell", processInfo{goos: "linux", getenv: envOf(map[string]string{"INVOCATION_ID": "0123abcd", "SYSTEMD_EXEC_PID": "17"}), pid: 4242, ppid: 4000}, "", "only a server", false},
	} {
		by, blocked, helper := rebootBy(c.p, root)
		if by != c.by || !strings.HasPrefix(blocked, c.blocked) || (blocked == "") != (c.blocked == "") || (helper != nil) != c.helper {
			t.Errorf("%s: %q %q helper %v", c.name, by, blocked, helper != nil)
		}
	}
	_, _, linux := rebootBy(unit, root)
	_, _, mac := rebootBy(daemon, root)
	if linux() != noRebootHelper || mac() != noRebootHelper {
		t.Fatalf("no marks: %q %q", linux(), mac())
	}
	for _, p := range []string{install.RebootPathWant, install.PlistPath(install.RebootLabel)} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if linux() != "" || mac() != "" {
		t.Fatalf("with the marks: %q %q", linux(), mac())
	}
}

// The request is in place once written, and its writer returns once the
// helper took it; one the helper did not take is removed after the wait,
// and a directory that is missing is an error at once.
func TestRebootRequester(t *testing.T) {
	take, poll := rebootTake, rebootPoll
	t.Cleanup(func() { rebootTake, rebootPoll = take, poll })
	rebootTake, rebootPoll = 300*time.Millisecond, 10*time.Millisecond
	path := filepath.Join(t.TempDir(), "reboot-request")
	taken := make(chan string, 1)
	go func() {
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
			if b, err := os.ReadFile(path); err == nil {
				os.Remove(path)
				taken <- string(b)
				return
			}
		}
		taken <- ""
	}()
	if err := rebootRequester(path)(); err != nil {
		t.Fatalf("taken: %v", err)
	}
	if line := <-taken; !strings.HasPrefix(line, "massimo: reboot requested at ") {
		t.Fatalf("the request: %q", line)
	}
	err := rebootRequester(path)()
	if err == nil || !strings.Contains(err.Error(), "did not take the request within 300ms") {
		t.Fatalf("not taken: %v", err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("the request is left: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 0 {
		t.Fatalf("left in the directory: %v", entries)
	}
	if err := rebootRequester(filepath.Join(t.TempDir(), "missing", "reboot-request"))(); err == nil || !strings.Contains(err.Error(), "writing the reboot request") {
		t.Fatalf("a missing directory: %v", err)
	}
	var log bytes.Buffer
	os.WriteFile(path, []byte("x"), 0o600)
	removeStaleReboot(path, slog.New(slog.NewTextHandler(&log, nil)))
	if _, err := os.Lstat(path); !os.IsNotExist(err) || !strings.Contains(log.String(), "a reboot request left from before removed") {
		t.Fatalf("the stale request: %v, %s", err, log.String())
	}
}

// Windows reboots at once through shutdown.exe of its system directory,
// with a planned reason and without forcing programs to close; its
// failure names the command and what it printed.
func TestWindowsRebooter(t *testing.T) {
	name, args := windowsShutdown(`D:\Win`)
	if name != `D:\Win\System32\shutdown.exe` || strings.Join(args, " ") != "/r /t 0 /d p:4:1 /c Massimo: reboot requested from the web UI" {
		t.Fatalf("%s %q", name, args)
	}
	if name, _ := windowsShutdown(""); name != `C:\Windows\System32\shutdown.exe` {
		t.Fatalf("without SystemRoot: %s", name)
	}
	run := runCommand
	t.Cleanup(func() { runCommand = run })
	var got []string
	runCommand = func(name string, args ...string) ([]byte, error) {
		got = append([]string{name}, args...)
		return []byte("Access is denied.(5)\r\n"), os.ErrPermission
	}
	t.Setenv("SystemRoot", `C:\Windows`)
	err := windowsRebooter()()
	if len(got) == 0 || got[0] != `C:\Windows\System32\shutdown.exe` || err == nil || !strings.Contains(err.Error(), "Access is denied.(5)") {
		t.Fatalf("%q: %v", got, err)
	}
}

// The settings file's MASSIMO_REBOOT applies at once, both ways, and is
// never listed as waiting for the next start; with -allow-reboot given,
// the file's value does not count.
func TestReloadAppliesTheRebootSwitch(t *testing.T) {
	unsetForTest(t, rebootVar)
	p := filepath.Join(t.TempDir(), "env")
	os.WriteFile(p, nil, 0o600)
	t.Cleanup(func() { os.WriteFile(p, nil, 0o600); env.ReloadFile(p) })
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := store.Open(t.TempDir(), store.Options{Version: "test", Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	opts := server.RebootOptions{By: bySystemd, Reboot: func() error { t.Error("rebooted"); return nil }}
	srv := server.New(server.Options{Store: st, Logger: quiet, Reboot: opts})
	available := func(srv *server.Server) bool {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/reboot", nil)
		req.RemoteAddr = "127.0.0.1:5000"
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("GET /api/v1/reboot: %d %s", rec.Code, rec.Body.String())
		}
		var v struct {
			Available bool `json:"available"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v.Available
	}
	r := newSettingsReloader(p, quiet, srv, settings(), map[string]bool{})
	if available(srv) {
		t.Fatal("available before the switch")
	}
	for _, c := range []struct {
		line string
		want bool
	}{{rebootVar + "=on\n", true}, {rebootVar + "=maybe\n", false}, {rebootVar + "=yes\n", true}, {"", false}} {
		os.WriteFile(p, []byte(c.line), 0o600)
		r.reload(t.Context(), "test")
		if available(srv) != c.want || len(r.pending()) != 0 {
			t.Fatalf("%q: available %v, pending %v", c.line, !c.want, r.pending())
		}
	}
	opts.Allowed = true
	srv2 := server.New(server.Options{Store: st, Logger: quiet, Reboot: opts})
	r2 := newSettingsReloader(p, quiet, srv2, settings(), map[string]bool{"allow-reboot": true})
	os.WriteFile(p, []byte(rebootVar+"=off\n"), 0o600)
	r2.reload(t.Context(), "test")
	if !available(srv2) || len(r2.pending()) != 0 {
		t.Fatalf("with -allow-reboot: available %v, pending %v", available(srv2), r2.pending())
	}
}
