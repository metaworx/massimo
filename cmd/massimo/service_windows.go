// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package main

import (
	"os"

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
func init() {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		return
	}
	go func() {
		err := svc.Run(install.ServiceName, serviceHandler{})
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}()
}

type serviceHandler struct{}

func (serviceHandler) Execute(_ []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	s <- svc.Status{State: svc.StartPending}
	s <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			s <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			s <- svc.Status{State: svc.StopPending}
			return false, 0
		}
	}
	return false, 0
}
