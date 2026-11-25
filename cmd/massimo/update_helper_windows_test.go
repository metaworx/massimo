// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/update"
)

func TestHelperEnv(t *testing.T) {
	t.Parallel()
	got := helperEnv([]string{"Path=C:\\Windows", "MASSIMO_API_TOKEN=x", "massimo_cube_KEQ0000000_KEY=y", "MASSIMO_GITHUB_TOKEN=t", "MASSIMO_UPDATE_API=http://127.0.0.1:1", "MASSIMO_ENV_FILE=C:\\ProgramData\\Massimo\\env", "SystemRoot=C:\\Windows"})
	want := []string{"Path=C:\\Windows", "MASSIMO_GITHUB_TOKEN=t", "MASSIMO_UPDATE_API=http://127.0.0.1:1", "SystemRoot=C:\\Windows", "MASSIMO_ENV_FILE=" + noEnvFile}
	if !slices.Equal(got, want) {
		t.Fatalf("%q", got)
	}
}

func TestJobDecision(t *testing.T) {
	t.Parallel()
	if f, b := jobDecision(0); f != 0 || b != "" {
		t.Fatal("a plain job")
	}
	if f, b := jobDecision(windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK); f != windows.CREATE_BREAKAWAY_FROM_JOB || b != "" {
		t.Fatal("breakaway allowed")
	}
	if f, b := jobDecision(windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK); f != 0 || b != "" {
		t.Fatal("silent breakaway")
	}
	if _, b := jobDecision(windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE); b == "" {
		t.Fatal("killed with the job, not blocked")
	}
	// Whatever this test runs in, the check itself must work.
	if _, _, err := jobFlags(); err != nil {
		t.Fatal(err)
	}
}

// The starter runs a process and refuses a second start until it exits.
func TestHelperStarterGuard(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := &helperStarter{
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		exe:     filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"),
		args:    []string{"/c", "ping -n 3 127.0.0.1 >NUL"},
		dir:     dir,
		logPath: filepath.Join(dir, "update.log"),
		flags:   windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	if err := h.start(); err != nil {
		t.Fatal(err)
	}
	if err := h.start(); err != update.ErrHelperRunning {
		t.Fatalf("second start: %v", err)
	}
	// Once it has exited (and let go of update.log), the guard is released.
	testkit.Eventually(t, "the helper's exit", func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return !h.running
	}, testkit.For(10*time.Second), testkit.Every(100*time.Millisecond))
}

// With -update-request, a listen address the update helper cannot ask
// /healthz at no longer stops the start (AP StartConfiguration,
// D21-A): serve listens on port 0, and the update API says why updates
// cannot be started, naming the setting, in GET's blocked and POST's
// 403. serve runs in the test's process, stopped as the manager's stop
// does, with a data directory and a %ProgramData% of its own; nothing
// asks GitHub.
func TestUpdateHelperListenKeepsTheStart(t *testing.T) {
	testkit.Swapping(t)
	clearCubeEnv(t)
	tempServiceLog(t)
	t.Setenv(update.APIVar, "http://127.0.0.1:1") // never GitHub
	defer func(old *serviceRun) { theService = old }(theService)
	theService = newServiceRun()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "serve.log")
	t.Cleanup(func() {
		if b, err := os.ReadFile(logPath); t.Failed() && err == nil {
			t.Logf("serve's log:\n%s", b)
		}
	})
	served := make(chan error, 1)
	go func() {
		served <- runServe([]string{"-listen", "127.0.0.1:0", "-discover-interval", "0", "-discover-listen", "127.0.0.1:0", "-proxy", "off", "-token", "test-token",
			"-log", logPath, "-log-process-format", "none", "-update-request", filepath.Join(dir, "update-request"), "-update-status", filepath.Join(dir, "update-status.json")})
	}()
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		close(theService.stop) // what the manager's stop does
		select {
		case err := <-served:
			if err != nil {
				t.Errorf("serve ended on the stop with %v", err)
			}
		case <-time.After(stopWait):
			t.Errorf("serve did not end within %s", stopWait)
		}
	}
	defer stop()

	// The address is the log's: "api listening" says which port it got.
	logs := func() *testkit.Watch[testkit.Record] {
		w := testkit.NewWatch[testkit.Record](nil)
		if f, err := os.Open(logPath); err == nil {
			testkit.ReadLog(w, f)
			f.Close()
		}
		return w
	}
	var addr string
	testkit.Eventually(t, "serve's api listening", func() bool {
		select {
		case err := <-served:
			stopped = true
			t.Fatalf("serve ended at the start: %v", err)
		default:
		}
		if r := logs().Since(0, testkit.Msg("api listening")); len(r) > 0 {
			addr = r[0].Attrs["addr"]
		}
		return addr != ""
	}, testkit.For(15*time.Second), testkit.Every(50*time.Millisecond))

	want := `-listen or MASSIMO_LISTEN: the update helper cannot ask "127.0.0.1:0" for /healthz: bad port`
	logs().Has(t, 0, "the reason in the log", testkit.MsgAt(slog.LevelWarn, "updates cannot be started from the API", "reason", want))
	ask := func(method string) (int, map[string]any) {
		req, err := http.NewRequest(method, "http://"+addr+"/api/v1/update", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer test-token")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("%s /api/v1/update: %s, %v", method, resp.Status, err)
		}
		return resp.StatusCode, out
	}
	if code, out := ask(http.MethodGet); code != http.StatusOK || out["can_update"] != false || out["blocked"] != want {
		t.Errorf("GET /api/v1/update: %d %v; want blocked %q", code, out, want)
	}
	if code, out := ask(http.MethodPost); code != http.StatusForbidden || out["error"] != want {
		t.Errorf("POST /api/v1/update: %d %v; want 403 %q", code, out, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "update-request")); !os.IsNotExist(err) {
		t.Errorf("the request file was written: %v", err)
	}
	stop()
}

// The restart helper is this binary's massimo restart for the service,
// detached, writing to service.log in the data directory. It is never
// started here: on the owner's box that would restart the real service.
func TestRestartHelper(t *testing.T) {
	t.Setenv("ProgramData", t.TempDir()) // service.log is the real service's otherwise
	exe := `C:\Program Files\Massimo\massimo.exe`
	h, blocked := newRestartHelper(slog.New(slog.NewTextHandler(io.Discard, nil)), exe)
	if h == nil {
		t.Fatalf("blocked: %s", blocked)
	}
	if h.exe != exe || strings.Join(h.args, " ") != "restart -service "+install.ServiceName || h.logPath != serviceLogPath() || h.dir != filepath.Dir(serviceLogPath()) || h.name != "restart helper" {
		t.Fatalf("helper %+v", h)
	}
	if !strings.HasPrefix(h.logPath, os.Getenv("ProgramData")) {
		t.Fatalf("log %s outside the test's ProgramData", h.logPath)
	}
	if h.flags&windows.DETACHED_PROCESS == 0 {
		t.Fatal("not detached")
	}
}
