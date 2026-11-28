// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Software updates: GET /api/v1/update says whether a newer signed release
// is out on the update channel (updateChannel), as the server's own check
// does every six hours (updateLoop), and POST /api/v1/update asks the
// update helper to install it (internal/update): through a request file
// that a systemd path unit (packaging/systemd) or launchd
// (packaging/launchd) watches, and on Windows by starting the helper
// itself (Start), having taken a backup of the server first
// (update_backup.go). The server only checks, backs itself up and asks;
// the helper replaces the binary, or, where the Debian package owns it,
// has apt upgrade it (packaging/deb).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	goruntime "runtime"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/update"
)

// UpdateOptions configure the update endpoints.
type UpdateOptions struct {
	Version string // the build version, "v0.1.0-alpha.9", without the Go details
	// Flavour is what the build was released as (update.Flavours): the
	// channel the server follows unless Channel or the store's
	// update.channel says otherwise.
	Flavour string
	// Channel, where set, is the channel whatever the store says:
	// MASSIMO_UPDATE_CHANNEL, or the formula brew follows.
	Channel string
	// Suite, where set, reads the suite apt follows (the massimo source),
	// at each check, since the update helper switches it: the channel
	// where the store names none. A stored channel that differs is the one
	// the next update asks the helper to switch apt to.
	Suite func() string
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
	// Source, where set, reads the newest version the box's package
	// source has for channel, as a tag, "" for none: apt's index of the
	// suite the next update uses, or the formula brew follows. A release
	// is then available only where the source has it (AP AutoUpdate
	// D5-A); nil on a box massimo install set up, which takes GitHub's.
	Source func(ctx context.Context, channel string) (string, error)
}

// ReleaseFinder finds the newest signed release on a channel; nil, nil
// when there is none.
type ReleaseFinder interface {
	Latest(ctx context.Context, channel string) (*update.Release, error)
}

// checkEvery is how long a release check is reused, and how long the
// server's own check waits after one that failed.
const checkEvery = 10 * time.Minute

// selfCheckEvery is how often the server checks for a release by itself
// (updateLoop).
const selfCheckEvery = 6 * time.Hour

// releaseCheck is what one check found: GitHub's newest signed release
// on the channel, and, where the box has a package source and the
// release is newer than the running version, the newest the source had.
type releaseCheck struct {
	at      time.Time
	release *update.Release
	err     error
	// source is the source's newest at sourceAt, zero where it was not
	// read; sourceErr says why it could not be.
	source    string
	sourceAt  time.Time
	sourceErr error
}

type updateCheck struct {
	mu      sync.Mutex
	channel string // the channel checked
	releaseCheck
	held updateHold
	// start is an update being started and the last start that failed
	// (update_backup.go), under its own lock: mu is held through a
	// release check.
	start updateStart
}

// updateHold is a version the update helper answered up-to-date for,
// "apt has nothing newer yet", at since: held back from Update until a
// check made after since finds the box's source has it (AP AutoUpdate
// D5-B). It is kept in memory: a restart is followed by a check, which
// is the later check the hold waits for.
type updateHold struct {
	version string
	since   time.Time
}

// updateChannel is the channel updates come from: Channel where set,
// else the store's update.channel, else apt's suite, else the build's
// flavour.
func (s *Server) updateChannel(ctx context.Context) string {
	if c := s.opts.Update.Channel; c != "" {
		return c
	}
	if c := s.storedChannel(ctx); c != "" {
		return c
	}
	if c := s.aptSuite(); c != "" {
		return c
	}
	return s.opts.Update.Flavour
}

// aptSuite is the suite apt follows, "" where apt does not own massimo.
func (s *Server) aptSuite() string {
	if s.opts.Update.Suite == nil {
		return ""
	}
	return s.opts.Update.Suite()
}

// storedChannel is the store's update.channel, "" where unset or there
// is no store.
func (s *Server) storedChannel(ctx context.Context) string {
	if s.opts.Store == nil {
		return ""
	}
	c, err := s.opts.Store.SettingString(ctx, store.UpdateChannelKey)
	if err != nil {
		s.log.Warn("update channel not read", "error", err)
		return ""
	}
	return c
}

// keepChannel stores the channel the box follows before it installs a
// release of another flavour, where neither Channel nor the store names
// one: the new binary's flavour would otherwise become the channel, and a
// box on beta that installs a stable release would follow stable from
// then on.
func (s *Server) keepChannel(ctx context.Context, channel string, rel *update.Release) {
	if s.opts.Update.Channel != "" || s.opts.Update.Suite != nil || s.opts.Store == nil || rel == nil || rel.Flavour == s.opts.Update.Flavour || s.storedChannel(ctx) != "" {
		return
	}
	if err := s.opts.Store.SetSetting(ctx, store.UpdateChannelKey, channel); err != nil {
		s.log.Warn("update channel not kept", "channel", channel, "error", err)
		return
	}
	s.log.Info("update channel kept", "channel", channel, "flavour", s.opts.Update.Flavour, "release_flavour", rel.Flavour)
}

// finder is where the server looks for releases: the options' Finder,
// else defaultFinder's.
func (s *Server) finder() ReleaseFinder {
	if s.opts.Update.Finder != nil {
		return s.opts.Update.Finder
	}
	return defaultFinder()
}

// defaultFinder is the finder of a server whose options name none:
// GitHub's releases. The package's tests replace it with one that fails
// loudly, so that none of them reaches GitHub.
var defaultFinder = func() ReleaseFinder { return update.NewClient() }

// updateLoop checks for a release by itself (AP AutoUpdate D4-A),
// whatever the settings say, so that the notification update reaches
// people without an open page. Each minute's tick asks updateTick
// whether a check is due: the first tick's, a minute after the start,
// when a box that has just booted most likely has its network, then
// every selfCheckEvery. It ends with ctx.
func (s *Server) updateLoop(ctx context.Context) {
	tick := s.opts.selfCheckTick
	if tick <= 0 {
		tick = time.Minute
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		s.updateTick(ctx, time.Now())
	}
}

// updateTick checks for a release where the server's own check is due at
// now: the first at once, then selfCheckEvery after one that answered,
// checkEvery after one that failed, or whose package source could not be
// read. The check is updateRelease's, fresh, which pushes update where a
// newer release is out; the rules tell each version once. Then the night
// runs its automatic update (nightTick). Nothing is checked without a
// store, which the rules and the settings need, or without a valid
// channel: a channel set later is checked at the next tick.
func (s *Server) updateTick(ctx context.Context, now time.Time) {
	if s.opts.Store == nil || !update.ValidChannel(s.updateChannel(ctx)) {
		return
	}
	checked := !now.Before(s.updateNext)
	if checked {
		v, _ := s.updateRelease(ctx, true)
		if v.CheckError != "" || v.SourceError != "" {
			s.updateNext = now.Add(checkEvery)
			s.log.Debug("update check failed", "channel", v.Channel, "error", v.CheckError, "source_error", v.SourceError)
		} else {
			s.updateNext = now.Add(selfCheckEvery)
		}
	}
	s.nightTick(ctx, now, checked)
}

// latest returns the newest release on the channel and what the box's
// package source has, checked at most every checkEvery unless refresh is
// set. The source is read only where the release is newer than the
// running version: otherwise there is nothing to install either way.
func (s *Server) latest(ctx context.Context, channel string, refresh bool) releaseCheck {
	c := &s.updates
	c.mu.Lock()
	defer c.mu.Unlock()
	if refresh || c.at.IsZero() || time.Since(c.at) > checkEvery || c.channel != channel {
		fctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		rel, err := s.finder().Latest(fctx, channel)
		cancel()
		c.releaseCheck, c.channel = releaseCheck{at: time.Now(), release: rel, err: err}, channel
		if src := s.opts.Update.Source; src != nil && err == nil && rel != nil && s.newerThanRunning(rel.Tag) {
			sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			c.source, c.sourceErr = src(sctx, channel)
			cancel()
			c.sourceAt = time.Now()
		}
	}
	return c.releaseCheck
}

// newerThanRunning reports whether tag is newer than the running
// version; every release is, where the running one is no release's.
func (s *Server) newerThanRunning(tag string) bool {
	v, ok := update.ParseVersion(tag)
	if !ok {
		return false
	}
	cur, ok := update.ParseVersion(s.opts.Update.Version)
	return !ok || update.Compare(v, cur) > 0
}

// olderTag reports whether a is older than b, or no version at all.
func olderTag(a, b string) bool {
	av, ok := update.ParseVersion(a)
	if !ok {
		return true
	}
	bv, ok := update.ParseVersion(b)
	return ok && update.Compare(av, bv) < 0
}

// holdVersion holds version back from Update since the helper's status
// at since said it found nothing to install (updateHold).
func (s *Server) holdVersion(version string, since time.Time) {
	c := &s.updates
	c.mu.Lock()
	defer c.mu.Unlock()
	c.held = updateHold{version: version, since: since}
}

// heldBack is the version held back from offer (updateHold), "" where
// none is: none is held, offer is newer than the held one, or the box
// runs it already; and where what the check found in the box's source
// (GitHub's release where it has none) after the hold has it, which
// ends the hold.
func (s *Server) heldBack(rc releaseCheck, offer string) string {
	c := &s.updates
	c.mu.Lock()
	defer c.mu.Unlock()
	h := c.held
	if h.version == "" {
		return ""
	}
	has, at := "", rc.at
	if rc.release != nil {
		has = rc.release.Tag
	}
	if !rc.sourceAt.IsZero() {
		has, at = rc.source, rc.sourceAt
	}
	if at.After(h.since) && !olderTag(has, h.version) || !s.newerThanRunning(h.version) {
		c.held = updateHold{}
		return ""
	}
	if offer == "" || olderTag(h.version, offer) {
		return ""
	}
	return h.version
}

// updateBlocked says why an update cannot be started from the API: the
// 403 a request for one meets, its text the view's blocked; nil where it
// can be.
func (s *Server) updateBlocked(ctx context.Context) error {
	switch {
	case s.opts.Update.Managed != "":
		return refuseSaying(http.StatusForbidden, s.opts.Update.Managed, codeUpdateManaged)
	case !s.authenticated(ctx):
		return refuse(http.StatusForbidden, codeUpdateOpen)
	case s.opts.Update.Request == "" && goruntime.GOOS == "windows":
		return refuse(http.StatusForbidden, codeUpdateFromService)
	case s.opts.Update.Request == "":
		return refuse(http.StatusForbidden, codeUpdateNoHelper)
	case s.opts.Update.Helper != nil && s.opts.Update.Helper() != "":
		return refuseSaying(http.StatusForbidden, s.opts.Update.Helper(), codeUpdateHelperOff)
	case s.opts.Update.Blocked != "":
		return refuseSaying(http.StatusForbidden, s.opts.Update.Blocked, codeUpdateUnstartable)
	case !update.ValidChannel(s.updateChannel(ctx)):
		return refuse(http.StatusForbidden, codeUpdateNoChannel)
	}
	if v, ok := update.ParseVersion(s.opts.Update.Version); !ok || v.Dirty {
		return refuse(http.StatusForbidden, codeUpdateDevBuild)
	}
	return nil
}

type updateView struct {
	Current string `json:"current"`
	Flavour string `json:"flavour,omitempty"`
	Channel string `json:"channel"`
	Suite   string `json:"suite,omitempty"`
	Manager string `json:"manager,omitempty"`
	// Latest is the version an update would install: GitHub's newest
	// signed release, or on a box with a package source the newest the
	// source has, never newer than GitHub's (AP AutoUpdate D8).
	Latest string `json:"latest,omitempty"`
	// Released is GitHub's newest where the source has it not yet.
	Released string `json:"released,omitempty"`
	// SourceError says why the package source could not be read;
	// nothing is available then (D9).
	SourceError string `json:"source_error,omitempty"`
	// Held is a version the update helper found nothing to install for,
	// held back from Update until a later check finds it (D5-B).
	Held       string `json:"held,omitempty"`
	ReleaseURL string `json:"release_url,omitempty"`
	// Available is set where Latest is newer than the running version
	// and not held back.
	Available  bool           `json:"available"`
	CheckedAt  time.Time      `json:"checked_at"`
	CheckError string         `json:"check_error,omitempty"`
	CanUpdate  bool           `json:"can_update"`
	Blocked    string         `json:"blocked,omitempty"`
	Helper     *update.Status `json:"helper,omitempty"`
	// Auto is the automatic update at night (update_auto.go); absent
	// without a store.
	Auto *autoView `json:"auto,omitempty"`
	// History is the last updates, newest first (update_history.go).
	History []updateEntry `json:"history"`
	// Backup is the backup an update takes first (update_backup.go);
	// absent without a store.
	Backup *updateBackupView `json:"backup,omitempty"`
	// Starting is an update being started, its backup running, until the
	// helper is asked; StartError the last start that failed before it,
	// until an update starts (update_backup.go).
	Starting   *updateStartingView `json:"starting,omitempty"`
	StartError *updateStartError   `json:"start_error,omitempty"`
}

func (s *Server) updateState(ctx context.Context, refresh bool) updateView {
	v, _ := s.updateRelease(ctx, refresh)
	return v
}

// updateRelease is updateState with the release it found, if any.
func (s *Server) updateRelease(ctx context.Context, refresh bool) (updateView, *update.Release) {
	u := s.opts.Update
	v := updateView{Current: u.Version, Flavour: u.Flavour, Channel: s.updateChannel(ctx), Suite: s.aptSuite(), Manager: u.Manager}
	if why := s.updateBlocked(ctx); why != nil {
		v.Blocked = why.Error()
	}
	if u.Status != "" {
		if st, err := update.ReadStatus(u.Status); err == nil && !st.Time.IsZero() {
			v.Helper = &st
			// an update the helper ended without restarting the server
			// ends here, before the history is read and what is held
			// back is known
			s.noteHelperOutcome(ctx, &st)
		}
	}
	var found *update.Release
	if update.ValidChannel(v.Channel) {
		rc := s.latest(ctx, v.Channel, refresh)
		found = rc.release
		v.CheckedAt = rc.at
		if rc.err != nil {
			v.CheckError = rc.err.Error()
		}
		if rel := rc.release; rel != nil {
			offer := rel.Tag
			switch {
			case rc.sourceAt.IsZero():
			case rc.sourceErr != nil:
				v.SourceError, offer = rc.sourceErr.Error(), ""
			case olderTag(rc.source, rel.Tag):
				// GitHub's is out before the source has it
				v.Released, offer = rel.Tag, rc.source
			}
			v.Latest, v.ReleaseURL = offer, rel.URL
			v.Held = s.heldBack(rc, offer)
			v.Available = v.Held == "" && s.newerThanRunning(offer)
			if v.Available {
				s.pushNotify(notify.Event{Kind: "update", Values: map[string]any{"version": offer}})
			}
		}
	}
	v.Starting, v.StartError = s.startState()
	v.CanUpdate = v.Blocked == "" && v.Available && (v.Helper == nil || !v.Helper.Running()) && v.Starting == nil
	v.Auto = s.autoFor(ctx, v)
	if v.History = s.updateHistory(ctx); v.History == nil {
		v.History = []updateEntry{}
	}
	v.Backup = s.updateBackupState(ctx)
	return v, found
}

// sourceName names a box's package source in a reason: its package
// manager, apt or brew.
func sourceName(manager string) string {
	if manager == "" {
		return "the package source"
	}
	return manager
}

func (s *Server) getUpdate(w http.ResponseWriter, r *http.Request) error {
	writeJSON(w, http.StatusOK, s.updateState(r.Context(), r.URL.Query().Get("refresh") != ""))
	return nil
}

// startUpdate handles POST /api/v1/update, {} or {"no_backup": true}:
// the update, its backup first unless no_backup asks for none, which is
// an audit row of its own (update.no_backup). Where a backup is taken,
// 202 comes at once with starting, the update being started.
func (s *Server) startUpdate(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		NoBackup bool `json:"no_backup"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return fail(http.StatusBadRequest, "bad body: %v (want {} or {\"no_backup\": true})", err)
	}
	if req.NoBackup {
		auditAction(r, "update.no_backup")
	}
	v, err := s.beginUpdate(r.Context(), userIDOf(r), false, req.NoBackup)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusAccepted, acceptedUpdate(v))
	return nil
}

// acceptedUpdate is the answer of an update accepted, 202: the version
// asked for and its channel, and starting where its backup runs first.
func acceptedUpdate(v updateView) map[string]any {
	out := map[string]any{"ok": true, "requested": v.Latest, "channel": v.Channel}
	if v.Starting != nil {
		out["starting"] = v.Starting
	}
	return out
}

// beginUpdate asks the update helper to install the newest release, for
// POST /api/v1/update (userID, 0 for the owner's key) and for the night's
// automatic update (auto, nobody's): the checks, one request at a time,
// the server's backup unless noBackup (planBackup), the channel kept, the
// request, the helper started where the server starts it (Windows,
// brew), the history's entry and the log's line (askHelper). Where a
// backup is due, the update is being started until the helper is asked
// (claimStart): a request's answers at once with v.Starting set, its
// backup and the rest running on a context of their own (startInBackground);
// the night's runs them in the loop. A backup that fails stops it before
// the request. Its errors are fail's, which POST answers as they are.
func (s *Server) beginUpdate(ctx context.Context, userID int64, auto, noBackup bool) (updateView, error) {
	// Replacing the binary is never open to an unauthenticated API, even
	// where reading and setting temperatures are.
	if why := s.updateBlocked(ctx); why != nil {
		return updateView{}, why
	}
	v, rel := s.updateRelease(ctx, true)
	switch {
	case v.Helper != nil && v.Helper.Running():
		return v, refuseSaying(http.StatusConflict, fmt.Sprintf("an update is already running (%s)", v.Helper.State), codeUpdateRunning)
	case v.Starting != nil:
		return v, fail(http.StatusConflict, "an update to %s is being started: the server's backup runs", v.Starting.Version)
	case v.Latest == "" && v.CheckError != "":
		return v, fail(http.StatusBadGateway, "release check failed: %s", v.CheckError)
	case v.SourceError != "":
		return v, fail(http.StatusBadGateway, "reading what %s has failed: %s", sourceName(v.Manager), v.SourceError)
	case v.Held != "":
		return v, refuse(http.StatusConflict, codeUpdateHeld, v.Held, sourceName(v.Manager))
	case v.Released != "" && !v.Available:
		return v, refuse(http.StatusConflict, codeUpdateNotInSource, v.Released, sourceName(v.Manager))
	case v.Latest == "":
		return v, refuse(http.StatusConflict, codeUpdateNoRelease, v.Channel)
	case !v.Available:
		return v, refuse(http.StatusConflict, codeUpdateUpToDate, v.Current)
	}
	// One request at a time: a second POST waits for the first's write
	// and start, and the helper that start left at work answers it.
	s.updateMu.Lock()
	locked := true
	defer func() {
		if locked {
			s.updateMu.Unlock()
		}
	}()
	// The helper's status again, after the release check: updateRelease
	// read it before, when the helper a POST of the moment before started
	// may not have written its first state yet. Refused here, this POST
	// writes no request, which a path unit (PathExists) would take for a
	// second run and which the Windows starter's guard leaves behind.
	if err := s.helperRunning(); err != nil {
		return v, err
	}
	if err := s.startUnderWay(); err != nil {
		return v, err
	}
	skip, due := s.planBackup(ctx, noBackup)
	if !due {
		return s.askHelper(ctx, v, rel, userID, auto, skip)
	}
	// The backup runs without the lock: a POST meanwhile is refused at
	// once while the update is being started, rather than held.
	if !s.claimStart(v.Latest) {
		return v, fail(http.StatusConflict, "an update is being started: the server's backup runs")
	}
	s.updateMu.Unlock()
	locked = false
	if auto {
		return s.backupThenAsk(ctx, v, rel, userID, auto)
	}
	s.startInBackground(ctx, v, rel, userID)
	v.Starting, _ = s.startState()
	return v, nil
}

// helperRunning is a fail's 409 where the helper's status says it works,
// nil where it does not or the box has no status.
func (s *Server) helperRunning() error {
	if path := s.opts.Update.Status; path != "" {
		if st, err := update.ReadStatus(path); err == nil && st.Running() {
			return refuseSaying(http.StatusConflict, fmt.Sprintf("an update is already running (%s)", st.State), codeUpdateRunning)
		}
	}
	return nil
}

// startInBackground runs a request's update past its answer
// (backupThenAsk): on Run's context where the server runs, whose end it
// waits for, else on the request's without its end, so that a browser
// that gives up ends nothing; a failure is the log's and GET's.
func (s *Server) startInBackground(ctx context.Context, v updateView, rel *update.Release, userID int64) {
	base := s.track()
	tracked := base != nil
	if !tracked {
		base = context.WithoutCancel(ctx)
	}
	u := &s.updates.start
	u.runs.Add(1)
	go func() {
		defer u.runs.Done()
		if tracked {
			defer s.wg.Done()
		}
		if _, err := s.backupThenAsk(base, v, rel, userID, false); err != nil {
			s.log.Warn("update not started", "version", v.Latest, "error", err)
		}
	}()
}

// backupThenAsk takes the backup an update takes first (backupForUpdate)
// and then asks the helper (askHelper), and ends the start claimStart
// marked. A backup that fails stops the update: an update line at
// error, and GET says so until an update starts.
func (s *Server) backupThenAsk(ctx context.Context, v updateView, rel *update.Release, userID int64, auto bool) (updateView, error) {
	defer s.endStart()
	taken, err := s.backupForUpdate(ctx, userID)
	if err != nil {
		failed := systemEvent("update", userID, updateSummary(v, auto))
		failed.Level, failed.Error = "error", "the backup before the update failed: "+err.Error()
		s.logServer(failed)
		stopped := updateBackupFailed(err)
		s.noteStartFailure(stopped, err)
		return v, stopped
	}
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	return s.askHelper(ctx, v, rel, userID, auto, taken)
}

// updateSummary is the summary of an update's line of the log.
func updateSummary(v updateView, auto bool) map[string]any {
	summary := map[string]any{"from": v.Current, "to": v.Latest, "channel": v.Channel}
	if auto {
		summary["auto"] = true
	}
	return summary
}

// askHelper asks the helper for the update v found, with s.updateMu
// held, once its backup was taken or skipped (taken): the helper's
// status once more, the channel kept, the request, the helper started
// where the server starts it, the history's entry and the log's line.
func (s *Server) askHelper(ctx context.Context, v updateView, rel *update.Release, userID int64, auto bool, taken updateBackupTaken) (updateView, error) {
	// a backup takes a while, in which the helper a POST of the moment
	// before started may have written its first state
	if err := s.helperRunning(); err != nil {
		return v, err
	}
	summary := updateSummary(v, auto)
	switch {
	case taken.uuid != "":
		summary["backup"] = taken.uuid
	case taken.skipped != "":
		summary["backup_skipped"] = taken.skipped
	}
	s.keepChannel(ctx, v.Channel, rel)
	asked := time.Now()
	if err := update.WriteRequest(s.opts.Update.Request, v.Channel); err != nil {
		err = fail(http.StatusInternalServerError, "asking the update helper: %v", err)
		s.noteStartFailure(err, nil)
		return v, err
	}
	if start := s.opts.Update.Start; start != nil {
		switch err := start(); {
		case errors.Is(err, update.ErrHelperRunning):
			// The running helper takes the request or has taken its own;
			// a leftover one names the same channel and goes to the next.
			return v, fail(http.StatusConflict, "%v (starting)", err)
		case err != nil:
			os.Remove(s.opts.Update.Request)
			failed := systemEvent("update", userID, summary)
			failed.Level, failed.Error = "error", err.Error()
			s.logServer(failed)
			err = fail(http.StatusInternalServerError, "starting the update helper: %v", err)
			s.noteStartFailure(err, nil)
			return v, err
		}
	}
	s.noteStop("update")
	s.noteStartFailure(nil, nil)
	s.noteUpdateStart(ctx, asked, userID, v.Current, v.Latest, v.Channel, v.Helper, auto, taken)
	s.logServer(systemEvent("update", userID, summary))
	return v, nil
}
