// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !windows

package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"go.metaworx.ch/massimo/internal/update"
)

// noEnvFile is a settings file that does not exist, so that a helper
// the server starts loads none (a missing file is not an error).
const noEnvFile = "/nonexistent/.env"

// brewHelperStarter returns the function that starts the brew helper
// (update_brew.go) for the request and status files: this binary in its
// helper mode, as the server's own user, in a session of its own, so
// that it outlives the restart of the job it was started from (launchd
// stops the job's process group, not another session), with the
// server's /healthz as health, restart the job to restart, and its
// output appended to logPath.
func brewHelperStarter(log *slog.Logger, exe, request, status, restart, health, logPath string) func() error {
	h := &sessionHelper{
		log:     log,
		exe:     exe,
		args:    []string{"update", "-request", request, "-status", status, "-restart", restart, "-health", health},
		dir:     filepath.Dir(status),
		logPath: logPath,
	}
	return h.start
}

// sessionHelper starts a helper in a session of its own, one at a time.
type sessionHelper struct {
	log          *slog.Logger
	exe          string
	args         []string
	dir, logPath string

	mu      sync.Mutex
	running bool // a helper started here has not exited
}

// start starts the helper unless one started here is still running.
func (h *sessionHelper) start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		return update.ErrHelperRunning
	}
	if err := os.MkdirAll(filepath.Dir(h.logPath), 0o755); err != nil {
		return fmt.Errorf("update log: %w", err)
	}
	out, err := os.OpenFile(h.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("update log: %w", err)
	}
	cmd := exec.Command(h.exe, h.args...)
	cmd.Dir = h.dir
	cmd.Env = helperEnv(os.Environ())
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	err = cmd.Start()
	out.Close() // the child has its own descriptor
	if err != nil {
		return fmt.Errorf("start %s: %w", h.exe, err)
	}
	h.running = true
	h.log.Info("update helper started", "pid", cmd.Process.Pid, "log", h.logPath)
	go func() {
		// Usually never seen: the helper restarts this server first.
		err := cmd.Wait()
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
		if err != nil {
			h.log.Warn("update helper exited", "pid", cmd.Process.Pid, "error", err)
		} else {
			h.log.Info("update helper exited", "pid", cmd.Process.Pid)
		}
	}()
	return nil
}
