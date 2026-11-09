// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Software updates: GET /api/v1/update says whether a newer signed release
// is out on the update channel, POST /api/v1/update asks the update
// helper to install it (internal/update): through a request file that a
// systemd path unit (packaging/systemd) or launchd (packaging/launchd)
// watches, and on Windows by starting the helper itself (Start). The
// server only checks and asks; the helper replaces the binary, or, where
// the Debian package owns it, has apt upgrade it (packaging/deb).

import (
	"context"
	"errors"
	"net/http"
	"os"
	goruntime "runtime"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/update"
)

// UpdateOptions configure the update endpoints.
type UpdateOptions struct {
	Version string // the build version, "v0.1.0-alpha.9", without the Go details
	Channel string // the update channel (a branch name)
	// Request is the file that asks the root helper for an update; empty
	// means no helper is installed and updates cannot be started here.
	Request string
	Status  string        // the helper's status file
	Finder  ReleaseFinder // where releases come from; nil: GitHub
	// Start starts the helper once the request is written; nil where the
	// operating system does that (the systemd path unit, launchd's
	// WatchPaths). update.ErrHelperRunning from it means a helper it
	// started is still at work.
	Start func() error
	// Blocked is a reason the helper cannot be started on this machine,
	// found by whoever provides Start; it is reported like the others.
	Blocked string
	// Managed is set where a package manager owns the binary
	// (install.Packaged) and updates are not installed from here: the
	// reason, given ahead of every other and shown beside the newer
	// version.
	Managed string
	// Manager is the package manager that owns the binary, "apt", shown
	// as such; where Managed is empty, the helper installs through it.
	Manager string
	// Helper says why the helper is not there, asked at each request
	// (the Debian package's, which the owner can turn off); nil where
	// Request alone says it.
	Helper func() string
}

// ReleaseFinder finds the newest signed release on a channel; nil, nil
// when there is none.
type ReleaseFinder interface {
	Latest(ctx context.Context, channel string) (*update.Release, error)
}

// checkEvery is how long a release check is reused.
const checkEvery = 10 * time.Minute

type updateCheck struct {
	mu      sync.Mutex
	at      time.Time
	release *update.Release
	err     error
}

func (s *Server) finder() ReleaseFinder {
	if s.opts.Update.Finder != nil {
		return s.opts.Update.Finder
	}
	return update.NewClient()
}

// latest returns the newest release on the channel, checked at most every
// checkEvery unless refresh is set.
func (s *Server) latest(ctx context.Context, refresh bool) (*update.Release, time.Time, error) {
	c := &s.updates
	c.mu.Lock()
	defer c.mu.Unlock()
	if refresh || c.at.IsZero() || time.Since(c.at) > checkEvery {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		c.release, c.err = s.finder().Latest(ctx, s.opts.Update.Channel)
		c.at = time.Now()
	}
	return c.release, c.at, c.err
}

// updateBlocked says why an update cannot be started from the API, or "".
func (s *Server) updateBlocked(ctx context.Context) string {
	switch {
	case s.opts.Update.Managed != "":
		return s.opts.Update.Managed
	case !s.authenticated(ctx):
		return "set MASSIMO_API_TOKEN or create a user to start updates from the API"
	case s.opts.Update.Request == "" && goruntime.GOOS == "windows":
		return "start updates from the installed service (massimo install)"
	case s.opts.Update.Request == "":
		return "no update helper installed (see packaging/README.md)"
	case s.opts.Update.Helper != nil && s.opts.Update.Helper() != "":
		return s.opts.Update.Helper()
	case s.opts.Update.Blocked != "":
		return s.opts.Update.Blocked
	case !update.ValidChannel(s.opts.Update.Channel):
		return "no update channel: set MASSIMO_UPDATE_CHANNEL"
	}
	if v, ok := update.ParseVersion(s.opts.Update.Version); !ok || v.Dirty {
		return "this is a development or modified build; update it by hand (massimo update -force)"
	}
	return ""
}

type updateView struct {
	Current    string         `json:"current"`
	Channel    string         `json:"channel"`
	Manager    string         `json:"manager,omitempty"`
	Latest     string         `json:"latest,omitempty"`
	ReleaseURL string         `json:"release_url,omitempty"`
	Available  bool           `json:"available"`
	CheckedAt  time.Time      `json:"checked_at"`
	CheckError string         `json:"check_error,omitempty"`
	CanUpdate  bool           `json:"can_update"`
	Blocked    string         `json:"blocked,omitempty"`
	Helper     *update.Status `json:"helper,omitempty"`
}

func (s *Server) updateState(ctx context.Context, refresh bool) updateView {
	u := s.opts.Update
	v := updateView{Current: u.Version, Channel: u.Channel, Manager: u.Manager, Blocked: s.updateBlocked(ctx)}
	if update.ValidChannel(u.Channel) {
		rel, at, err := s.latest(ctx, refresh)
		v.CheckedAt = at
		if err != nil {
			v.CheckError = err.Error()
		}
		if rel != nil {
			v.Latest, v.ReleaseURL = rel.Tag, rel.URL
			cur, ok := update.ParseVersion(u.Version)
			v.Available = !ok || update.Compare(rel.Version, cur) > 0
		}
	}
	if u.Status != "" {
		if st, err := update.ReadStatus(u.Status); err == nil && !st.Time.IsZero() {
			v.Helper = &st
		}
	}
	v.CanUpdate = v.Blocked == "" && v.Available && (v.Helper == nil || !v.Helper.Running())
	return v
}

func (s *Server) getUpdate(w http.ResponseWriter, r *http.Request) error {
	writeJSON(w, http.StatusOK, s.updateState(r.Context(), r.URL.Query().Get("refresh") != ""))
	return nil
}

func (s *Server) startUpdate(w http.ResponseWriter, r *http.Request) error {
	// Replacing the binary is never open to an unauthenticated API, even
	// where reading and setting temperatures are.
	if why := s.updateBlocked(r.Context()); why != "" {
		return fail(http.StatusForbidden, "%s", why)
	}
	v := s.updateState(r.Context(), true)
	switch {
	case v.Helper != nil && v.Helper.Running():
		return fail(http.StatusConflict, "an update is already running (%s)", v.Helper.State)
	case v.Latest == "":
		if v.CheckError != "" {
			return fail(http.StatusBadGateway, "release check failed: %s", v.CheckError)
		}
		return fail(http.StatusConflict, "no release on channel %s", v.Channel)
	case !v.Available:
		return fail(http.StatusConflict, "already up to date (%s)", v.Current)
	}
	// One request at a time: a second POST waits for the first's write
	// and start, and the helper that start left at work answers it.
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	if err := update.WriteRequest(s.opts.Update.Request, v.Channel); err != nil {
		return fail(http.StatusInternalServerError, "asking the update helper: %v", err)
	}
	s.log.Info("update requested", "from", v.Current, "to", v.Latest, "channel", v.Channel)
	asked := map[string]any{"from": v.Current, "to": v.Latest, "channel": v.Channel}
	if start := s.opts.Update.Start; start != nil {
		switch err := start(); {
		case errors.Is(err, update.ErrHelperRunning):
			// The running helper takes the request or has taken its own;
			// a leftover one names the same channel and goes to the next.
			return fail(http.StatusConflict, "%v (starting)", err)
		case err != nil:
			os.Remove(s.opts.Update.Request)
			s.log.Error("update helper not started", "error", err)
			failed := systemEvent("update", userIDOf(r), asked)
			failed.Level, failed.Error = "warn", err.Error()
			s.logServer(failed)
			return fail(http.StatusInternalServerError, "starting the update helper: %v", err)
		}
	}
	s.noteStop("update")
	s.logServer(systemEvent("update", userIDOf(r), asked))
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "requested": v.Latest, "channel": v.Channel})
	return nil
}
