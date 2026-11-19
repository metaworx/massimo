// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !windows

package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/testkit"
)

// catchSighup catches a real SIGHUP to the process: logged before
// ready, the reload after it.
func TestCatchSighup(t *testing.T) {
	log, w := testkit.LogWatch(nil)
	h, stop := catchSighup(log)
	defer stop()
	m := w.Mark()
	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	w.Await(t, m, "the warning for the SIGHUP before ready", testkit.Msg("SIGHUP during the start, ignored"))
	ran := make(chan struct{}, 1)
	h.ready(func() { ran <- struct{}{} })
	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ran:
	case <-time.After(testkit.Scale(5 * time.Second)):
		t.Fatal("the SIGHUP after ready ran no reload")
	}
}

// A SIGHUP while serve starts, with -sighup reload, once the store is
// created, while it migrates: serve does not end, and logs the SIGHUP as
// ignored; a SIGHUP once it runs is a reload, with the log's rotation.
func TestServeSighupDuringTheStart(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "serve.log")
	s := launchBuiltServe(t, []string{"MASSIMO_CUBE_ADDRESS=" + startFakeCube(t)}, "-log", path, "-proxy", "off", "-sighup", "reload")
	s.log.Await(t, 0, "store created", testkit.Msg("store created"), serveWait)
	m := s.log.Mark()
	if err := s.cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.log.Await(t, m, "the warning for the SIGHUP", testkit.Msg("SIGHUP during the start, ignored"), serveWait); r.Level != slog.LevelWarn {
		t.Errorf("the warning: %v", r)
	}
	s.log.Await(t, 0, "server started", testkit.Msg("server started"), serveWait)
	m = s.log.Mark()
	if err := s.cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	s.log.Await(t, m, "the SIGHUP's rotation", testkit.Msg("log rotated", "reason", "sighup"), serveWait)
	s.stop(t)
}
