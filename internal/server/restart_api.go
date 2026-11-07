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

import (
	"net"
	"net/http"
	"sync/atomic"

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
// why not; open is set when the reason is an open server.
func (s *Server) restartState(r *http.Request) (v restartView, open bool) {
	o := s.opts.Restart
	switch {
	case o.Restart == nil || o.By == "":
		v.Blocked = o.Blocked
		if v.Blocked == "" {
			v.Blocked = "no service manager would start this server again"
		}
		return v, false
	case s.restartGate.asked.Load():
		v.Blocked = "a restart is under way"
	case s.updateRunning():
		v.Blocked = "an update is running, and it restarts the server itself"
	case !s.authenticated(r.Context()) && !fromTheBox(r):
		v.Blocked, open = openRestartRefused, true
	}
	v.By, v.Available = o.By, v.Blocked == ""
	return v, open
}

func (s *Server) getRestart(w http.ResponseWriter, r *http.Request) error {
	v, _ := s.restartState(r)
	v.Pending = s.pendingRestart()
	writeJSON(w, http.StatusOK, v)
	return nil
}

func (s *Server) startRestart(w http.ResponseWriter, r *http.Request) error {
	v, open := s.restartState(r)
	switch {
	case open:
		return fail(http.StatusForbidden, "%s", v.Blocked)
	case !v.Available:
		return fail(http.StatusConflict, "%s", v.Blocked)
	case !s.restartGate.asked.CompareAndSwap(false, true):
		return fail(http.StatusConflict, "a restart is under way")
	}
	attrs := []any{"by", v.By, "remote", s.clientIP(r)}
	if p := principalOf(r); p.named() {
		attrs = append(attrs, "user", p.user.Name)
	}
	s.log.Info("restart requested", attrs...)
	// The stop waits for this request, its answer and its audit row
	// (cmd/massimo, serveHTTP), so the restart may begin before the
	// answer is written.
	if err := s.opts.Restart.Restart(); err != nil {
		s.restartGate.asked.Store(false)
		s.log.Error("restart not started", "error", err)
		return fail(http.StatusInternalServerError, "starting the restart: %v", err)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "by": v.By})
	return nil
}
