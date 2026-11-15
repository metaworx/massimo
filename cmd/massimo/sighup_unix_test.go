// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !windows

package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// lockedWriter collects what a logger writes from another goroutine.
type lockedWriter struct {
	mu sync.Mutex
	b  strings.Builder
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *lockedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

// catchSighup catches a real SIGHUP to the process: logged before
// ready, the reload after it.
func TestCatchSighup(t *testing.T) {
	var out lockedWriter
	h, stop := catchSighup(slog.New(slog.NewTextHandler(&out, nil)))
	defer stop()
	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	for end := time.Now().Add(5 * time.Second); !strings.Contains(out.String(), "SIGHUP during the start, ignored"); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatalf("no warning for the SIGHUP before ready: %q", out.String())
		}
	}
	ran := make(chan struct{}, 1)
	h.ready(func() { ran <- struct{}{} })
	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("the SIGHUP after ready ran no reload")
	}
}

// A SIGHUP while serve starts, with -sighup reload, once the store is
// created, while it migrates: serve does not end, and logs the SIGHUP as
// ignored; a SIGHUP once it runs is a reload, with the log's rotation.
func TestServeSighupDuringTheStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serve.log")
	s := launchBuiltServe(t, []string{"MASSIMO_CUBE_ADDRESS=" + startFakeCube(t)}, "-log", path, "-proxy", "off", "-sighup", "reload")
	s.waitFor(t, "store created")
	if err := s.cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	if a := s.waitFor(t, "SIGHUP during the start, ignored"); a["level"] != "WARN" {
		t.Errorf("the warning: %v", a)
	}
	s.waitFor(t, "server started")
	if err := s.cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	s.waitForAttr(t, "log rotated", "reason", "sighup")
	s.stop(t)
}
