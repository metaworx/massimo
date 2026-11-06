// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"go.metaworx.ch/massimo/internal/update"
)

func TestHelperEnv(t *testing.T) {
	got := helperEnv([]string{"Path=C:\\Windows", "MASSIMO_API_TOKEN=x", "massimo_cube_KEQ0000000_KEY=y", "MASSIMO_GITHUB_TOKEN=t", "MASSIMO_UPDATE_API=http://127.0.0.1:1", "MASSIMO_ENV_FILE=C:\\ProgramData\\Massimo\\env", "SystemRoot=C:\\Windows"})
	want := []string{"Path=C:\\Windows", "MASSIMO_GITHUB_TOKEN=t", "MASSIMO_UPDATE_API=http://127.0.0.1:1", "SystemRoot=C:\\Windows", "MASSIMO_ENV_FILE=" + noEnvFile}
	if !slices.Equal(got, want) {
		t.Fatalf("%q", got)
	}
}

func TestJobDecision(t *testing.T) {
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
	for i := 0; ; i++ {
		h.mu.Lock()
		running := h.running
		h.mu.Unlock()
		if !running {
			break
		}
		if i > 100 {
			t.Fatal("the helper did not exit")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
