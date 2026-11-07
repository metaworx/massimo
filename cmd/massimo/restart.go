// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// The restart from the API (POST /api/v1/restart): who starts the server
// again, and how serve ends for it. A server under systemd, launchd or
// Homebrew stops as on SIGTERM and exits with exitRestart, which those
// answer with a start; a Windows service starts `massimo restart`, which
// stops the service through the service manager, so that serve stops as
// on every stop, and starts it again. A server nobody would start again,
// such as one in a terminal, does not offer the restart.

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"

	"go.metaworx.ch/massimo/internal/install"
)

// exitRestart is the exit status of a serve that stopped for a restart:
// the systemd unit forces a restart on it (RestartForceExitStatus) and
// counts it as a success, launchd and Homebrew restart on any exit but 0.
// 75 is EX_TEMPFAIL of sysexits.h.
const exitRestart = 75

// errRestart ends runServe after a stop for a restart; main exits with
// exitRestart.
var errRestart = errors.New("stopped for a restart")

// restartVar is the variable behind serve's -restart.
const restartVar = "MASSIMO_RESTART"

// The values of -restart.
const (
	restartAuto = "auto" // detect the service manager
	restartExit = "exit" // exit with exitRestart for whatever runs serve
	restartOff  = "off"  // never offer the restart
)

// Who starts the server again, as GET /api/v1/restart names it.
const (
	byWindows = "windows" // the Windows service manager, through massimo restart
	bySystemd = "systemd"
	byLaunchd = "launchd"
	byExit    = "exit" // whatever runs serve, told by exitRestart
)

// homebrewLabel is the label brew services gives the server's job.
const homebrewLabel = "homebrew.mxcl.massimo"

// processInfo is what restartBy looks at, gathered by currentProcess.
type processInfo struct {
	goos    string
	getenv  func(string) string
	ppid    int
	service bool // a Windows service, started by its manager
}

func currentProcess() processInfo {
	return processInfo{goos: runtime.GOOS, getenv: os.Getenv, ppid: os.Getppid(), service: runningAsService}
}

// restartBy says who starts this server again after a restart from the
// API under the -restart mode, or, with by empty, why nobody does.
//
//   - A Windows service: the service manager, through massimo restart.
//   - systemd sets INVOCATION_ID for every service it starts; the unit
//     must restart on failure, as the installed one does.
//   - launchd sets XPC_SERVICE_NAME to the job's label, and is the parent
//     of the processes it starts: the installed LaunchDaemon and
//     Homebrew's job both restart on an exit other than 0.
//   - Anything else, a terminal, Docker, a unit massimo does not know: only
//     with -restart exit, which trusts it to start serve again.
func restartBy(mode string, p processInfo) (by, blocked string, err error) {
	switch mode {
	case "", restartAuto:
	case restartExit:
		return byExit, "", nil
	case restartOff:
		return "", "the restart is switched off (-restart off)", nil
	default:
		return "", "", fmt.Errorf("serve: -restart %q: want auto, exit or off", mode)
	}
	switch {
	case p.service:
		return byWindows, "", nil
	case p.goos == "linux" && p.getenv("INVOCATION_ID") != "":
		return bySystemd, "", nil
	case p.goos == "darwin" && (p.ppid == 1 || p.getenv("XPC_SERVICE_NAME") == install.Label || p.getenv("XPC_SERVICE_NAME") == homebrewLabel):
		return byLaunchd, "", nil
	}
	return "", "no service manager would start this server again (in a terminal, or under one massimo does not know: -restart exit if it restarts serve)", nil
}

// restarter carries out a restart: helper, where set, asks the service
// manager for it (Windows); otherwise serve's context ends, and runServe
// returns errRestart once everything has stopped.
type restarter struct {
	by     string
	cancel context.CancelFunc
	helper func() error
	asked  atomic.Bool
}

func (r *restarter) restart() error {
	if r.helper != nil {
		return r.helper()
	}
	r.asked.Store(true)
	r.cancel()
	return nil
}

// result is what runServe returns: errRestart after a stop the restart
// asked for, else err.
func (r *restarter) result(err error) error {
	if r != nil && r.asked.Load() && err == nil {
		return errRestart
	}
	return err
}

// exitCode is the process's exit status for a command's error.
func exitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errRestart):
		return exitRestart
	}
	return 1
}

// defaultServiceName is the installed service massimo restart restarts.
func defaultServiceName() string {
	switch runtime.GOOS {
	case "windows":
		return install.ServiceName
	case "darwin":
		return install.Label
	}
	return "massimo.service"
}

// runRestart restarts the installed service: the systemd unit, the
// LaunchDaemon or the Windows service. A Windows service runs it as a
// detached process for a restart from the API.
func runRestart(args []string) error {
	fs := flag.NewFlagSet("restart", flag.ContinueOnError)
	name := fs.String("service", defaultServiceName(), "the service to restart: a systemd unit, on macOS a LaunchDaemon's label, on Windows a service's name")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo restart [-service <name>]\n\nRestart the installed server: systemctl restart on Linux, launchctl\nkickstart -k on macOS, a stop and a start through the service manager on\nWindows. Needs root (an administrator on Windows). The server's own\nrestart from the API and the web UI does not need this command, except on\nWindows, where it runs it itself.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		fs.Usage()
		return fmt.Errorf("restart: unexpected argument %q", fs.Arg(0))
	}
	return restartService(*name)
}
