// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"

	"go.metaworx.ch/massimo/internal/install"
)

// Started by the Windows service manager (massimo install), the process
// must answer it, or the manager gives up on the service after 30
// seconds. The service's command line is an ordinary one
// ("massimo -env … serve …"), which main runs as always; this answers the
// manager next to it: running once started, and on stop or shutdown it
// ends serve's context (serviceContext) and waits for serve to return,
// which writes the log's pending events and closes the store, then
// reports stopped and ends the process. When serve fails by itself the
// process ends without reporting, and the manager's recovery actions
// restart it.
// What happens between the manager and the process is noted in
// service.log next to the settings, since a service has no console.
func init() {
	isService, err := svc.IsWindowsService()
	if err != nil {
		serviceLog("cannot tell whether this runs as a service: %v", err)
		return
	}
	if !isService {
		return
	}
	runningAsService = true
	// A service has no console: what main reports on stderr, such as the
	// error that ends serve, goes to service.log as well.
	if f, err := os.OpenFile(serviceLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
		os.Stderr = f
	}
	go func() {
		serviceLog("started as a service: %q", os.Args)
		if err := svc.Run(install.ServiceName, serviceHandler{}); err != nil {
			serviceLog("answering the service manager failed: %v", err)
			os.Exit(1)
		}
		serviceLog("stopped")
		os.Exit(0)
	}()
}

// runningAsService is set when the service manager started the process.
var runningAsService bool

// serviceDataDir is %ProgramData%\Massimo for a service started without
// -data: its working directory is System32, so the current directory
// will not do (docs/store.md section 3).
func serviceDataDir() string {
	if !runningAsService {
		return ""
	}
	return install.WindowsPaths(os.Getenv("ProgramFiles"), os.Getenv("ProgramData")).DataDir
}

func serviceLogPath() string {
	return install.WindowsPaths(os.Getenv("ProgramFiles"), os.Getenv("ProgramData")).DataDir + `\service.log`
}

func serviceLog(format string, args ...any) {
	f, err := os.OpenFile(serviceLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

// stopWait is how long a stop waits for serve, and the wait the manager
// is told to allow: serve's listener takes up to 5 seconds to shut down
// and the server's run up to 10 (runServe); the update helper waits a
// minute for the service to stop.
const stopWait = 20 * time.Second

// serviceRun links the manager's stop with the command main runs.
type serviceRun struct {
	stop chan struct{} // closed when the manager asks the service to stop
	done chan struct{} // closed when main's command returned after that
}

func newServiceRun() *serviceRun {
	return &serviceRun{stop: make(chan struct{}), done: make(chan struct{})}
}

// theService is the process's; tests replace it.
var theService = newServiceRun()

// serviceContext is ctx, ended also when the service manager asks the
// service to stop.
func serviceContext(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	stop := theService.stop
	go func() {
		select {
		case <-stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// serviceEnd is called by main when its command returned. After a stop
// the manager asked for it lets Execute report the service stopped, and
// init's goroutine ends the process; without one it returns at once,
// and the process ends unreported, which the manager's recovery actions
// answer with a restart.
func serviceEnd() {
	run := theService
	select {
	case <-run.stop:
	default:
		return
	}
	close(run.done)
	time.Sleep(stopWait) // init's goroutine ends the process first
}

type serviceHandler struct{}

func (serviceHandler) Execute(_ []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	s <- svc.Status{State: svc.StartPending}
	s <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	serviceLog("running")
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			s <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			serviceLog("stop requested")
			theService.stopServe(r, s)
			return false, 0
		}
	}
	return false, 0
}

// stopServe ends serve's context and waits until main's command
// returned, at most stopWait, answering the manager meanwhile.
func (run *serviceRun) stopServe(r <-chan svc.ChangeRequest, s chan<- svc.Status) {
	pending := svc.Status{State: svc.StopPending, WaitHint: uint32(stopWait / time.Millisecond)}
	s <- pending
	close(run.stop)
	timeout := time.NewTimer(stopWait)
	defer timeout.Stop()
	for {
		select {
		case <-run.done:
			serviceLog("serve stopped")
			return
		case <-timeout.C:
			serviceLog("serve did not stop within %s", stopWait)
			return
		case c, ok := <-r:
			if !ok {
				r = nil
			} else if c.Cmd == svc.Interrogate {
				s <- pending
			}
		}
	}
}
