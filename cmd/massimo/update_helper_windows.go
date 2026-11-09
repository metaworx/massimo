// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/update"
)

// On Windows nothing watches the update request file, and the service
// cannot restart itself: once it has stopped, nobody is left to start
// it. So the server starts the update helper itself, as a detached
// process that outlives it: the installed binary in its helper mode
// (massimo update -request … -status … -restart massimo -health …),
// which takes the request, installs the release, stops and starts the
// service and rolls back when the new version does not come up. Both
// run as LocalSystem; see docs/update-windows.md.

// isProcessInJob is not wrapped by x/sys/windows.
var isProcessInJob = windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")

// noEnvFile is a settings file that does not exist, so that the helper
// loads none (a missing file is not an error).
const noEnvFile = `C:\nonexistent\.env`

// updateHelperStarter returns the function that starts the helper for
// the request and status files, with the server's /healthz as health and
// the helper's output appended to logPath; or, when this process cannot
// start one that would outlive it, nil and the reason.
func updateHelperStarter(log *slog.Logger, exe, request, status, health, logPath string) (start func() error, blocked string) {
	flags, blocked, err := jobFlags()
	if err != nil {
		// The check failed: start the helper anyway, as outside a job.
		log.Warn("update helper: cannot check the job object", "error", err)
	}
	if blocked != "" {
		return nil, blocked
	}
	h := &helperStarter{
		log:     log,
		exe:     exe,
		args:    []string{"update", "-request", request, "-status", status, "-restart", install.ServiceName, "-health", health},
		dir:     filepath.Dir(status),
		logPath: logPath,
		flags:   windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP | flags,
	}
	return h.start, ""
}

// restartHelperStarter returns the function that starts massimo restart
// for the service, detached as the update helper is and with its output
// appended to service.log: it stops the service through the service
// manager, which stops this server as every stop does, and starts it
// again. Or, when this process cannot start one that would outlive it,
// nil and the reason.
func restartHelperStarter(log *slog.Logger, exe string) (start func() error, blocked string) {
	h, blocked := newRestartHelper(log, exe)
	if h == nil {
		return nil, blocked
	}
	return h.start, ""
}

// newRestartHelper is restartHelperStarter's starter, unstarted.
func newRestartHelper(log *slog.Logger, exe string) (*helperStarter, string) {
	logPath := serviceLogPath()
	flags, blocked, err := jobFlags()
	if err != nil {
		log.Warn("restart helper: cannot check the job object", "error", err)
	}
	if blocked != "" {
		return nil, "the server runs in a job object that would end massimo restart with it; restart the service from an administrator prompt (massimo restart)"
	}
	return &helperStarter{
		name:    "restart helper",
		log:     log,
		exe:     exe,
		args:    []string{"restart", "-service", install.ServiceName},
		dir:     filepath.Dir(logPath),
		logPath: logPath,
		flags:   windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP | flags,
	}, ""
}

type helperStarter struct {
	name    string // for the log; "" is the update helper
	log     *slog.Logger
	exe     string
	args    []string
	dir     string
	logPath string
	flags   uint32

	mu      sync.Mutex
	running bool // a helper started here has not exited
}

// start starts the helper unless one started here is still running.
func (h *helperStarter) start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		return update.ErrHelperRunning
	}
	out, err := os.OpenFile(h.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("update log: %w", err)
	}
	cmd := exec.Command(h.exe, h.args...)
	cmd.Dir = h.dir
	cmd.Env = helperEnv(os.Environ())
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: h.flags, HideWindow: true}
	err = cmd.Start()
	out.Close() // the child has its own handle
	if err != nil {
		return fmt.Errorf("start %s: %w", h.exe, err)
	}
	h.running = true
	name := h.name
	if name == "" {
		name = "update helper"
	}
	h.log.Info(name+" started", "pid", cmd.Process.Pid, "log", h.logPath)
	go func() {
		// Usually never seen: the helper stops this server first.
		err := cmd.Wait()
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
		if err != nil {
			h.log.Warn(name+" exited", "pid", cmd.Process.Pid, "error", err)
		} else {
			h.log.Info(name+" exited", "pid", cmd.Process.Pid)
		}
	}()
	return nil
}

// jobFlags checks whether this process is in a job object whose
// processes die with it (JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE): the
// service manager puts none around a service, but a helper started in
// such a job would die with the server it stops. It returns the creation
// flag that breaks the child away where the job allows it, or the reason
// no helper can be started.
func jobFlags() (flags uint32, blocked string, err error) {
	var in int32
	if r, _, err := isProcessInJob.Call(uintptr(windows.CurrentProcess()), 0, uintptr(unsafe.Pointer(&in))); r == 0 {
		return 0, "", fmt.Errorf("IsProcessInJob: %w", err)
	}
	if in == 0 {
		return 0, "", nil
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	// A null handle names the job of the calling process.
	if err := windows.QueryInformationJobObject(0, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil); err != nil {
		return 0, "", fmt.Errorf("QueryInformationJobObject: %w", err)
	}
	flags, blocked = jobDecision(info.BasicLimitInformation.LimitFlags)
	return flags, blocked, nil
}

// jobDecision is jobFlags' choice for the limit flags of the job this
// process is in.
func jobDecision(limits uint32) (flags uint32, blocked string) {
	switch {
	case limits&windows.JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK != 0:
		return 0, "" // children are outside the job anyway
	case limits&windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK != 0:
		return windows.CREATE_BREAKAWAY_FROM_JOB, ""
	case limits&windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE != 0:
		return 0, "the server runs in a job object that would end the update helper with it; update from an administrator prompt (massimo update -restart massimo)"
	}
	return 0, "" // a job that does not kill its processes: the helper outlives the server
}
