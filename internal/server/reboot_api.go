// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The reboot of the computer the server runs on: GET /api/v1/reboot says
// whether the server can reboot it from here and who does, POST
// /api/v1/reboot asks for it after a delay, DELETE /api/v1/reboot takes
// back one that still waits. The server waits the delay itself and then
// asks RebootOptions.Reboot, which cmd/massimo provides: a request file
// for the root helper on Linux and macOS, shutdown.exe from the Windows
// service. The owner's switch (MASSIMO_REBOOT, -allow-reboot) and, where
// the system needs one, the installed helper come before anything else;
// an open server, with no users or without a token, takes a request only
// from the box itself, as the restart does.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
)

// The delay of a reboot: the default, and the longest a request may ask.
const (
	DefaultRebootDelay = 60 * time.Second
	MaxRebootDelay     = time.Hour
)

// rebootStuck is how long a server still runs after its reboot was taken
// before it counts the reboot as failed: the system stops the server
// within seconds of a reboot that started. A variable for the tests.
var rebootStuck = 2 * time.Minute

// RebootOptions say whether and how the server reboots the computer it
// runs on; cmd/massimo finds them.
type RebootOptions struct {
	// By names who reboots: "systemd" or "launchd" (the root helper they
	// start), "windows" (the service itself); "" when nobody can, and
	// Blocked says why.
	By      string
	Blocked string
	// Allowed is the owner's switch at the start (MASSIMO_REBOOT,
	// -allow-reboot); SetRebootAllowed changes it.
	Allowed bool
	// Helper says, at every request, why the helper cannot reboot the
	// computer, "" when it can; nil where none is needed.
	Helper func() string
	// Reboot asks for the reboot once the delay is over, and returns once
	// the helper or the system has taken it, or why not.
	Reboot func() error
}

// The reasons a reboot is refused that are not the system's own.
const (
	rebootSwitchedOff = "the reboot is switched off: set MASSIMO_REBOOT=on in the settings file, or start serve with -allow-reboot"
	rebootWaiting     = "a reboot is waiting"
	rebootBegun       = "the reboot has begun"
	openRebootRefused = "this server is open: reboot the computer from the box itself, or create a user or set MASSIMO_API_TOKEN"
)

// rebootPlan is the reboot that waits, if any.
type rebootPlan struct {
	mu      sync.Mutex
	allowed bool
	timer   *time.Timer
	// gen counts the reboots asked for and taken back, so that a timer
	// that fires after its reboot was taken back does nothing.
	gen   uint64
	at    time.Time // when the waiting reboot is asked for; zero: none waits
	begun bool      // the delay is over and Reboot was called
	err   string    // why the last one failed
}

type rebootView struct {
	Available bool       `json:"available"`
	By        string     `json:"by,omitempty"`
	Blocked   string     `json:"blocked,omitempty"`
	At        *time.Time `json:"at,omitempty"`
	Begun     bool       `json:"begun,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// SetRebootAllowed changes the owner's switch (cmd/massimo, the settings
// file's reload); switching it off takes back a reboot that waits.
func (s *Server) SetRebootAllowed(on bool) {
	p := &s.reboot
	p.mu.Lock()
	p.allowed = on
	cancelled := !on && p.takeBack()
	p.mu.Unlock()
	if cancelled {
		s.log.Warn("reboot taken back: switched off")
		s.noteStop("")
		s.logServer(systemEvent("reboot_cancelled", 0, map[string]any{"why": "switched off"}))
	}
}

// takeBack stops a reboot that waits and has not begun; p.mu is held.
func (p *rebootPlan) takeBack() bool {
	if p.at.IsZero() || p.begun {
		return false
	}
	p.gen++
	p.timer.Stop()
	p.timer, p.at = nil, time.Time{}
	return true
}

// rebootWaits reports whether a reboot waits or has begun, for the
// restart.
func (s *Server) rebootWaits() bool {
	p := &s.reboot
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.at.IsZero()
}

// rebootState says whether the request could reboot the computer, and
// why not; open is set when the reason is an open server.
func (s *Server) rebootState(r *http.Request) (v rebootView, open bool) {
	o := s.opts.Reboot
	p := &s.reboot
	p.mu.Lock()
	allowed, at, begun := p.allowed, p.at, p.begun
	v.Error, v.Begun = p.err, p.begun
	p.mu.Unlock()
	if !at.IsZero() {
		v.At = &at
	}
	if o.Reboot == nil || o.By == "" {
		v.Blocked = o.Blocked
		if v.Blocked == "" {
			v.Blocked = "this server does not reboot the computer it runs on"
		}
		return v, false
	}
	helper := ""
	if o.Helper != nil && allowed {
		helper = o.Helper()
	}
	switch {
	case !allowed:
		v.Blocked = rebootSwitchedOff
	case helper != "":
		v.Blocked = helper
	case begun:
		v.Blocked = rebootBegun
	case !at.IsZero():
		v.Blocked = rebootWaiting
	case s.restartGate.asked.Load():
		v.Blocked = "a restart is under way"
	case s.updateRunning():
		v.Blocked = "an update is running"
	case !s.authenticated(r.Context()) && !fromTheBox(r):
		v.Blocked, open = openRebootRefused, true
	}
	v.By, v.Available = o.By, v.Blocked == ""
	return v, open
}

func (s *Server) getReboot(w http.ResponseWriter, r *http.Request) error {
	v, _ := s.rebootState(r)
	writeJSON(w, http.StatusOK, v)
	return nil
}

func (s *Server) startReboot(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Delay *int `json:"delay"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	delay := DefaultRebootDelay
	if req.Delay != nil {
		if *req.Delay < 0 || *req.Delay > int(MaxRebootDelay/time.Second) {
			return fail(http.StatusBadRequest, "delay: want 0 to %d seconds", int(MaxRebootDelay/time.Second))
		}
		delay = time.Duration(*req.Delay) * time.Second
	}
	v, open := s.rebootState(r)
	switch {
	case open:
		return fail(http.StatusForbidden, "%s", v.Blocked)
	case !v.Available:
		return fail(http.StatusConflict, "%s", v.Blocked)
	}
	at, ok := s.planReboot(delay)
	if !ok {
		return fail(http.StatusConflict, rebootWaiting)
	}
	attrs := []any{"by", v.By, "delay", delay.String(), "at", at.Format(time.RFC3339), "remote", s.clientIP(r)}
	if p := principalOf(r); p.named() {
		attrs = append(attrs, "user", p.user.Name)
	}
	s.log.Info("reboot requested", attrs...)
	s.noteStop("reboot")
	s.logServer(systemEvent("reboot", userIDOf(r), map[string]any{"by": v.By, "at": at.UTC().Format(time.RFC3339)}))
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "by": v.By, "at": at})
	return nil
}

// planReboot sets the timer of a reboot after delay, unless one waits.
func (s *Server) planReboot(delay time.Duration) (time.Time, bool) {
	p := &s.reboot
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.at.IsZero() {
		return time.Time{}, false
	}
	p.gen++
	gen := p.gen
	p.at, p.err = time.Now().Add(delay).UTC(), ""
	p.timer = time.AfterFunc(delay, func() { s.rebootNow(gen) })
	return p.at, true
}

// rebootNow asks for the reboot at the end of the delay, unless it was
// taken back meanwhile.
func (s *Server) rebootNow(gen uint64) {
	p := &s.reboot
	p.mu.Lock()
	if p.gen != gen || p.at.IsZero() {
		p.mu.Unlock()
		return
	}
	p.begun = true
	p.mu.Unlock()
	s.log.Info("rebooting the computer", "by", s.opts.Reboot.By)
	err := s.opts.Reboot.Reboot()
	if err != nil {
		s.log.Error("reboot not started", "error", err)
		s.noteStop("")
		failed := systemEvent("reboot", 0, map[string]any{"by": s.opts.Reboot.By})
		failed.Level, failed.Error = "warn", err.Error()
		s.logServer(failed)
		p.failed(gen, err.Error())
		return
	}
	// The helper took it; a server still here later did not go down
	// with the computer, and the next reboot may be asked for.
	time.AfterFunc(rebootStuck, func() {
		if p.failed(gen, "the computer has not rebooted within "+rebootStuck.String()+"; the reboot helper's log says why") {
			s.log.Error("the computer has not rebooted", "after", rebootStuck.String())
		}
	})
}

// failed ends the reboot gen with its reason, unless another has
// followed it; it reports whether it did.
func (p *rebootPlan) failed(gen uint64, why string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gen != gen || !p.begun {
		return false
	}
	p.begun, p.at, p.timer, p.err = false, time.Time{}, nil, why
	return true
}

// cancelReboot takes back a reboot that waits. An open server takes it
// only from the box, as the request.
func (s *Server) cancelReboot(w http.ResponseWriter, r *http.Request) error {
	if !s.authenticated(r.Context()) && !fromTheBox(r) {
		return fail(http.StatusForbidden, openRebootRefused)
	}
	p := &s.reboot
	p.mu.Lock()
	begun, waits := p.begun, !p.at.IsZero()
	taken := p.takeBack()
	p.mu.Unlock()
	switch {
	case begun:
		return fail(http.StatusConflict, rebootBegun)
	case !waits || !taken:
		return fail(http.StatusConflict, "no reboot is waiting")
	}
	attrs := []any{"remote", s.clientIP(r)}
	if pr := principalOf(r); pr.named() {
		attrs = append(attrs, "user", pr.user.Name)
	}
	s.log.Info("reboot taken back", attrs...)
	s.noteStop("")
	s.logServer(systemEvent("reboot_cancelled", userIDOf(r), nil))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}
