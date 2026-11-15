// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// SIGHUP, on Unix: under a service manager it asks a daemon to read its
// configuration again, which systemctl reload sends; in a terminal it
// says that the terminal closed. serve -sighup says which it is: a reload
// reads the settings file again (reload.go) and checkpoints the store, a
// stop stops serve as SIGTERM does. Windows has no SIGHUP.

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

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

// hupReload is SIGHUP as a reload, caught from serve's start (AP
// EarlyHup): until serve hands it the reload (ready), a SIGHUP is
// logged at Warn and dropped, where Go's default would end serve, which
// systemd counts as a clean stop and does not start again. There is
// nothing for it to do then: the settings were just read and the log
// file just opened.
type hupReload struct {
	log *slog.Logger
	mu  sync.Mutex
	run func()
}

// catchSighup catches SIGHUP for a reload; stop ends that.
func catchSighup(log *slog.Logger) (h *hupReload, stop func()) {
	h = &hupReload{log: log}
	hups := make(chan os.Signal, 1)
	signal.Notify(hups, syscall.SIGHUP)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-hups:
				h.handle()
			}
		}
	}()
	return h, func() {
		signal.Stop(hups)
		close(done)
	}
}

// ready hands over the reload a SIGHUP runs from now on.
func (h *hupReload) ready(run func()) {
	h.mu.Lock()
	h.run = run
	h.mu.Unlock()
}

// handle runs the reload, or logs a SIGHUP that came before it was ready.
func (h *hupReload) handle() {
	h.mu.Lock()
	run := h.run
	h.mu.Unlock()
	if run == nil {
		h.log.Warn("SIGHUP during the start, ignored", "reason", "the settings were just read and the log file opened")
		return
	}
	run()
}
