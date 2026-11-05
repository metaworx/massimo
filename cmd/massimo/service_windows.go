// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package main

import (
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
// reports stopped and ends the process, which closes the Cube
// connections and the listeners. When serve fails by itself the process
// ends without reporting, and the manager's recovery actions restart it.
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
			s <- svc.Status{State: svc.StopPending}
			return false, 0
		}
	}
	return false, 0
}
