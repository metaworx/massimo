// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// SIGHUP, on Unix: under a service manager it asks a daemon to read its
// configuration again, which systemctl reload sends; in a terminal it
// says that the terminal closed. serve -sighup says which it is: a reload
// reads the settings file again (reload.go) and checkpoints the store, a
// stop stops serve as SIGTERM does. Windows has no SIGHUP.

import "fmt"

// sighupVar is the variable behind serve's -sighup.
const sighupVar = "MASSIMO_SIGHUP"

// The values of -sighup.
const (
	sighupAuto   = "auto"
	sighupReload = "reload"
	sighupStop   = "stop"
)

// sighupAction is what SIGHUP does under the -sighup mode: auto reloads
// where a service manager runs serve, or where SIGHUP was ignored at the
// start (nohup), and stops elsewhere, in a terminal.
func sighupAction(mode string, managed, ignored bool) (string, error) {
	switch mode {
	case "", sighupAuto:
		if managed || ignored {
			return sighupReload, nil
		}
		return sighupStop, nil
	case sighupReload, sighupStop:
		return mode, nil
	}
	return "", fmt.Errorf("serve: -sighup %q: want auto, reload or stop", mode)
}
