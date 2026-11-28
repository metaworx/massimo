// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The restart from the API: GET /api/v1/restart says whether the server
// can be restarted from here and who starts it again, POST
// /api/v1/restart asks for it (RestartOptions.Restart, which cmd/massimo
// provides): the server stops as on SIGTERM once this request is
// answered, and its service manager starts it again. An open server,
// with no users or without a token, takes the POST only from the box
// itself, so that nobody on the LAN can restart it; the cross-origin
// protection of the handler keeps other sites' pages out (server.go).
//
// POST /api/v1/reload reads the settings file again at once, as SIGHUP
// does, with what cmd/massimo hands the server (SetReloader); the server
// reads it by itself within seconds of a change anyway. GET
// /api/v1/restart names the file and when it was last read, and both
// answer what the last read found the next start refuses (SetRefused),
// which blocks nothing: the restart still goes (AP ReloadChecks, D8-B).
// All three take restart, which update and reboot include, since each
// restarts the server (auth.go, includes; AP StartConfiguration, D37).

import (
	"context"
	"net"
	"net/http"
	"slices"
	"sync/atomic"
	"time"

	"go.metaworx.ch/massimo/internal/update"
)

// restartGate keeps a second restart out while one is under way.
type restartGate struct{ asked atomic.Bool }

type restartView struct {
	Available bool   `json:"available"`
	By        string `json:"by,omitempty"`
	Blocked   string `json:"blocked,omitempty"`
	// Pending names the settings that changed since the start and
	// apply only at the next one.
	Pending []string `json:"pending,omitempty"`
	// File is the settings file, ReadAt when it was last read: at the
	// start, or again since, also where it did not parse, or was there
	// but could not be read (the log's settings_read).
	File   string     `json:"file,omitempty"`
	ReadAt *time.Time `json:"read_at,omitempty"`
	// Refused is what the last read found the next start refuses
	// (SetRefused).
	Refused []Refusal `json:"refused,omitempty"`
}

// Refusal is one thing the next start of serve refuses, as a read of the
// settings file found it (cmd/massimo, startchecks.go and reload.go): the
// settings that changed since the start and that the refusing check of
// the start reads, by name, and the start's own text, which may quote a
// value, a secret's named by its variable; no settings where the file
// itself does not parse, or is there but cannot be read, the text then
// naming the file and the line by its number.
type Refusal struct {
	Settings []string `json:"settings,omitempty"`
	Error    string   `json:"error"`
}

// ReloadResult is what reading the settings file again found: the
// variables that changed (names, never values), those that now differ
// from the start and apply only at the next one, what the next start
// refuses of them (SetRefused), and when it was read.
type ReloadResult struct {
	Changed []string  `json:"changed"`
	Pending []string  `json:"pending"`
	Refused []Refusal `json:"refused"`
	ReadAt  time.Time `json:"read_at"`
}

// SetReloader hands the server the settings file and what reads it again
// as SIGHUP does (cmd/massimo); without one POST /api/v1/reload answers
// 409. It also counts as the file's read at the start.
func (s *Server) SetReloader(file string, reload func(context.Context) (ReloadResult, error)) {
	s.pendingMu.Lock()
	s.settingsFile, s.reloader = file, reload
	s.pendingMu.Unlock()
}

// SetSettingsRead says when the settings file was last read.
func (s *Server) SetSettingsRead(at time.Time) {
	s.pendingMu.Lock()
	s.settingsRead = at
	s.pendingMu.Unlock()
}

func (s *Server) settingsState() (file string, at *time.Time, reload func(context.Context) (ReloadResult, error)) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	if !s.settingsRead.IsZero() {
		t := s.settingsRead
		at = &t
	}
	return s.settingsFile, at, s.reloader
}

// reloadSettings reads the settings file again at once and answers what
// changed; it sends nothing to a Cube, so a retry is harmless.
func (s *Server) reloadSettings(w http.ResponseWriter, r *http.Request) error {
	_, _, reload := s.settingsState()
	if reload == nil {
		return refuse(http.StatusConflict, codeReloadNoSettingsFile)
	}
	res, err := reload(r.Context())
	if err != nil {
		return fail(http.StatusInternalServerError, "reading the settings file: %v", err)
	}
	writeJSON(w, http.StatusOK, res)
	return nil
}

// SetPendingRestart says which settings changed since the start and
// apply only at the next one: names, never values (cmd/massimo, the
// settings file's reload).
func (s *Server) SetPendingRestart(names []string) {
	s.pendingMu.Lock()
	s.pending = append([]string(nil), names...)
	s.pendingMu.Unlock()
}

func (s *Server) pendingRestart() []string {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	return append([]string(nil), s.pending...)
}

// SetRefused says what the last read of the settings file found the next
// start refuses, none for an empty list (cmd/massimo, the settings
// file's reload): a read that finds nothing refused clears it, and the
// start has none, since it would have refused it. It blocks nothing,
// neither the restart nor an update or a reboot (AP ReloadChecks, D8-B).
func (s *Server) SetRefused(refused []Refusal) {
	s.pendingMu.Lock()
	s.refused = cloneRefusals(refused)
	s.pendingMu.Unlock()
}

func (s *Server) refusedAtNextStart() []Refusal {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	return cloneRefusals(s.refused)
}

// cloneRefusals is a copy of refused that shares nothing with it, nil
// for none.
func cloneRefusals(refused []Refusal) []Refusal {
	var out []Refusal
	for _, f := range refused {
		out = append(out, Refusal{Settings: slices.Clone(f.Settings), Error: f.Error})
	}
	return out
}

// openRestartRefused is the reason an open server gives a request that
// does not come from the box itself.
const openRestartRefused = "this server is open: restart it from the box itself, or create a user or set MASSIMO_API_TOKEN"

// fromTheBox reports whether a request comes from the box itself: its
// TCP peer is a loopback address and it carries no forwarding header,
// which a reverse proxy on the box adds for a client elsewhere.
func fromTheBox(r *http.Request) bool {
	ip := net.ParseIP(peerIP(r.RemoteAddr))
	if ip == nil || !ip.IsLoopback() {
		return false
	}
	for _, h := range []string{"X-Forwarded-For", "X-Real-IP", "Forwarded"} {
		if len(r.Header.Values(h)) > 0 {
			return false
		}
	}
	return true
}

// updateRunning reports whether the update helper is at work: it stops
// and starts the server itself.
func (s *Server) updateRunning() bool {
	if s.opts.Update.Status == "" {
		return false
	}
	st, err := update.ReadStatus(s.opts.Update.Status)
	return err == nil && st.Running()
}

// restartState says whether the request could restart the server, and
// why not: the refusal a request for it meets, nil where it could, its
// text the view's blocked; a 403 for an open server's, else a 409.
func (s *Server) restartState(r *http.Request) (v restartView, why error) {
	o := s.opts.Restart
	switch {
	case o.Restart == nil || o.By == "":
		why = refuse(http.StatusConflict, codeRestartUnavailable)
		if o.Blocked != "" {
			why = refuseSaying(http.StatusConflict, o.Blocked, codeRestartUnavailable)
		}
		v.Blocked = why.Error()
		return v, why
	case s.restartGate.asked.Load():
		why = refuse(http.StatusConflict, codeRestartUnderWay)
	case s.updateRunning():
		why = refuseSaying(http.StatusConflict, "an update is running, and it restarts the server itself", codeUpdateRunning)
	case s.rebootWaits():
		why = refuseSaying(http.StatusConflict, "a reboot of the computer is waiting", codeRebootWaiting)
	case !s.authenticated(r.Context()) && !fromTheBox(r):
		why = refuse(http.StatusForbidden, codeRestartOpen)
	}
	if why != nil {
		v.Blocked = why.Error()
	}
	v.By, v.Available = o.By, why == nil
	return v, why
}

func (s *Server) getRestart(w http.ResponseWriter, r *http.Request) error {
	v, _ := s.restartState(r)
	v.Pending = s.pendingRestart()
	v.Refused = s.refusedAtNextStart()
	v.File, v.ReadAt, _ = s.settingsState()
	writeJSON(w, http.StatusOK, v)
	return nil
}

func (s *Server) startRestart(w http.ResponseWriter, r *http.Request) error {
	v, why := s.restartState(r)
	switch {
	case why != nil:
		return why
	case !s.restartGate.asked.CompareAndSwap(false, true):
		return refuse(http.StatusConflict, codeRestartUnderWay)
	}
	s.noteStop("restart")
	s.logServer(systemEvent("restart", userIDOf(r), map[string]any{"by": v.By}))
	// The stop waits for this request, its answer and its audit row
	// (cmd/massimo, serveHTTP), so the restart may begin before the
	// answer is written.
	if err := s.opts.Restart.Restart(); err != nil {
		s.restartGate.asked.Store(false)
		s.noteStop("")
		failed := systemEvent("restart", userIDOf(r), map[string]any{"by": v.By})
		failed.Level, failed.Error = "error", err.Error()
		s.logServer(failed)
		return fail(http.StatusInternalServerError, "starting the restart: %v", err)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "by": v.By})
	return nil
}
