// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The server's backup before an update (AP ServerBackup v1.0, Block 10;
// docs/api-v1.md, "POST /api/v1/update"): every update the server
// starts, POST /api/v1/update, Install now and the night's (beginUpdate),
// first takes a backup of the server to its local targets, pinned for 30
// days (serverbackup.PinBefore). It runs as the server, under whoever
// holds update, with no capability of the backups, as the one job of the
// server's backups (sbJob), on a context of its own with its own time
// limit (updateBackupTimeout): a request's answers at once, the update
// being started (GET /api/v1/update, starting) until the helper is
// asked, so that a browser that gives up ends nothing; the night's runs
// in the update loop. Its id goes into the update's history and into its
// line of the log. Without a key there is nothing to encrypt for: the
// update goes on, and its line says so. A backup that fails stops the
// update, which GET says until an update starts (start_error, backup);
// no_backup, Update without a backup, goes on without one and is an
// audit row of its own, update.no_backup. A new backup before an update
// takes the pin of an earlier one no update used. POST
// /api/v1/update/backup takes the same backup alone, for massimo update
// by hand, which installs the release itself.

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/update"
)

// updateBackupTimeout bounds the backup an update takes first, the wait
// for a backup job that runs included.
const updateBackupTimeout = 30 * time.Minute

// Why an update took no backup, as its history's entry and its line of
// the log say it.
const (
	// skippedAsked: Update without a backup (no_backup) asked for none.
	skippedAsked = "asked"
	// skippedNoKey: no key is set for the server's backups.
	skippedNoKey = "no_key"
)

// The states of the backup an update takes first, as GET
// /api/v1/update says them (updateBackupView).
const (
	backupReady  = "ready"
	backupNoKey  = "no_key"
	backupFailed = "failed"
)

// stepBackup is what an update being started does while its backup
// runs (updateStartingView).
const stepBackup = "backup"

// updateBackupTaken is the backup an update took first: its uuid, or why
// it took none (skippedAsked, skippedNoKey); both empty where the server
// has no store to back up, or one opened read-only.
type updateBackupTaken struct {
	uuid, skipped string
}

// updateStart is an update being started and the last start that
// failed, kept in memory: after a restart the next Update tries again.
type updateStart struct {
	mu sync.Mutex
	// step is what the start under way does, stepBackup, "" for none;
	// version is the version it asks for, since when it began.
	step, version string
	since         time.Time
	// failedAt and failed are the last start that failed before the
	// helper was asked, until an update starts; backup is its backup's
	// error, where it was its backup that failed.
	failedAt time.Time
	failed   string
	backup   string
	// runs counts the starts in the background, for the tests.
	runs sync.WaitGroup
}

// updateBackupView is what GET /api/v1/update says of the backup an
// update takes first: ready, where it takes one; no_key, where no key is
// set and an update goes on without one; failed, where the last update's
// backup failed and no update has started since, with its error and
// when. It is absent without a store, and with one opened read-only.
type updateBackupView struct {
	State string     `json:"state"`
	Error string     `json:"error,omitempty"`
	At    *time.Time `json:"at,omitempty"`
}

// updateStartingView is an update being started, as GET /api/v1/update
// says it: its step, backup while the server's backup runs, the version
// it asks for, and since when.
type updateStartingView struct {
	Step    string    `json:"step"`
	Version string    `json:"version"`
	Since   time.Time `json:"since"`
}

// updateStartError is the last start that failed before the helper was
// asked, as GET /api/v1/update says it until an update starts.
type updateStartError struct {
	Error string    `json:"error"`
	At    time.Time `json:"at"`
}

// claimStart marks an update to version as being started, at its backup;
// false where one is being started already.
func (s *Server) claimStart(version string) bool {
	u := &s.updates.start
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.step != "" {
		return false
	}
	u.step, u.version, u.since = stepBackup, version, time.Now()
	return true
}

// endStart ends the start claimStart marked.
func (s *Server) endStart() {
	u := &s.updates.start
	u.mu.Lock()
	defer u.mu.Unlock()
	u.step, u.version, u.since = "", "", time.Time{}
}

// startUnderWay is a fail's 409 where an update is being started, nil
// where none is.
func (s *Server) startUnderWay() error {
	u := &s.updates.start
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.step == "" {
		return nil
	}
	return fail(http.StatusConflict, "an update to %s is being started: the server's backup runs", u.version)
}

// noteStartFailure keeps a start that failed before the helper was
// asked, err, for GET /api/v1/update; backup is its backup's error where
// that failed, nil else. nil forgets it, once an update has started.
func (s *Server) noteStartFailure(err, backup error) {
	u := &s.updates.start
	u.mu.Lock()
	defer u.mu.Unlock()
	if err == nil {
		u.failedAt, u.failed, u.backup = time.Time{}, "", ""
		return
	}
	u.failedAt, u.failed, u.backup = time.Now(), err.Error(), ""
	if backup != nil {
		u.backup = backup.Error()
	}
}

// startState is the start under way and the last that failed, as GET
// /api/v1/update says them.
func (s *Server) startState() (*updateStartingView, *updateStartError) {
	u := &s.updates.start
	u.mu.Lock()
	defer u.mu.Unlock()
	var starting *updateStartingView
	var failed *updateStartError
	if u.step != "" {
		starting = &updateStartingView{Step: u.step, Version: u.version, Since: u.since.UTC()}
	}
	if !u.failedAt.IsZero() {
		failed = &updateStartError{Error: u.failed, At: u.failedAt.UTC()}
	}
	return starting, failed
}

// planBackup says whether an update takes a backup first: not where
// noBackup asks for none (skippedAsked), where there is no store or only
// a read-only one, or where no key is set (skippedNoKey, with a line in
// the log); skip is then what the update's history and line say.
func (s *Server) planBackup(ctx context.Context, noBackup bool) (skip updateBackupTaken, due bool) {
	st := s.opts.Store
	switch {
	case noBackup:
		return updateBackupTaken{skipped: skippedAsked}, false
	case st == nil:
		return updateBackupTaken{}, false
	case st.ReadOnly():
		s.log.Info("update: the store is open read-only; the update goes on without a backup of the server")
		return updateBackupTaken{}, false
	}
	if keys, err := st.ServerBackupKeys(ctx); err == nil && !slices.ContainsFunc(keys, confirmedKey) {
		s.log.Info("update: no key is set for the server's backups; the update goes on without a backup")
		return updateBackupTaken{skipped: skippedNoKey}, false
	}
	return updateBackupTaken{}, true
}

// confirmedKey reports whether k encrypts backups: not a pending key.
func confirmedKey(k store.ServerBackupKey) bool { return !k.Pending }

// takeUpdateBackup takes the backup an update takes first, for userID (0
// for the owner's key or the server itself): to the local targets,
// pinned for serverbackup.PinBefore, as the one job of the server's
// backups, its steps in the log (reportServerBackup); one that failed
// leaves no row, as no backup by hand does. A backup made takes the pin
// of the earlier ones no update used (unpinUnused).
// serverbackup.ErrNoKey says that no key is set.
func (s *Server) takeUpdateBackup(ctx context.Context, userID int64) (store.ServerBackup, []serverbackup.Outcome, error) {
	var b store.ServerBackup
	var outs []serverbackup.Outcome
	err := s.sbJob(ctx, func() error {
		var err error
		b, outs, err = s.catalogue().Make(ctx, serverbackup.MakeRequest{Trigger: serverbackup.TriggerBeforeUpdate, UserID: userID,
			Local: true, PinFor: serverbackup.PinBefore})
		s.reportServerBackup(outs, serverbackup.TriggerBeforeUpdate, userID)
		if err == nil {
			s.unpinUnused(context.WithoutCancel(ctx), b.UUID)
		}
		return err
	})
	return b, outs, err
}

// unpinUnused takes the pin of the backups before an update that no
// update used, but kept, the one just made: none of the history's
// entries names it, and it was made by the version that runs, so that no
// update has come since. Retention keeps them no longer than any other;
// a pin by hand stays.
func (s *Server) unpinUnused(ctx context.Context, kept string) {
	st := s.opts.Store
	bs, err := st.ServerBackups(ctx)
	if err != nil {
		return
	}
	used := map[string]bool{}
	for _, e := range s.updateHistory(ctx) {
		used[e.Backup] = true
	}
	running := cmp.Or(s.opts.Update.Version, s.opts.Version)
	for _, b := range bs {
		if b.UUID == kept || used[b.UUID] || b.Trigger != string(serverbackup.TriggerBeforeUpdate) || b.PinnedUntil.IsZero() ||
			!b.PinnedUntil.Before(serverbackup.PinForever) {
			continue
		}
		if m, _, err := serverbackup.ParseManifest(serverbackup.Raw{Manifest: b.Manifest, Signature: []byte(b.Signature)}); err == nil && m.Version != running {
			continue
		}
		if err := st.PinServerBackup(ctx, b.UUID, time.Time{}); err != nil {
			s.log.Debug("update: an unused backup before an update keeps its pin", "backup", b.UUID, "error", err)
		}
	}
}

// backupForUpdate is the backup an update takes once planBackup said it
// is due: takeUpdateBackup's, on ctx with updateBackupTimeout; a key
// removed meanwhile is none, with a line in the log.
func (s *Server) backupForUpdate(ctx context.Context, userID int64) (updateBackupTaken, error) {
	ctx, cancel := context.WithTimeout(ctx, updateBackupTimeout)
	defer cancel()
	b, _, err := s.takeUpdateBackup(ctx, userID)
	switch {
	case errors.Is(err, serverbackup.ErrNoKey):
		s.log.Info("update: no key is set for the server's backups; the update goes on without a backup")
		return updateBackupTaken{skipped: skippedNoKey}, nil
	case err != nil:
		return updateBackupTaken{}, err
	}
	return updateBackupTaken{uuid: b.UUID}, nil
}

// updateBackupFailed is the error of an update whose backup failed, the
// update not started: 409 where no local target is switched on, 503
// where its context ended first, else 500; its text says how to go on
// without one.
func updateBackupFailed(err error) error {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, serverbackup.ErrNoTarget):
		status = http.StatusConflict
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status = http.StatusServiceUnavailable
	}
	return fail(status, "the backup of the server before the update failed, so the update did not start: %v (Update without a backup, no_backup, starts it without one)", err)
}

// updateBackupState is the backup an update takes first, as GET
// /api/v1/update says it (updateBackupView); nil without a store, with
// one opened read-only, and where its keys cannot be read.
func (s *Server) updateBackupState(ctx context.Context) *updateBackupView {
	st := s.opts.Store
	if st == nil || st.ReadOnly() {
		return nil
	}
	keys, err := st.ServerBackupKeys(ctx)
	if err != nil {
		s.log.Debug("update: the server backups' keys not read", "error", err)
		return nil
	}
	if !slices.ContainsFunc(keys, confirmedKey) {
		return &updateBackupView{State: backupNoKey}
	}
	u := &s.updates.start
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.backup != "" {
		return &updateBackupView{State: backupFailed, Error: u.backup, At: timeOrNil(u.failedAt)}
	}
	return &updateBackupView{State: backupReady}
}

// postUpdateBackup handles POST /api/v1/update/backup, {"to": "v…"}, for
// a holder of update: the backup an update takes first, alone, for
// massimo update by hand (cmd/massimo), which then installs the release
// to itself; to is another version than the one that runs, so that the
// backup is an update's (400 where it is none, 409 where it runs), and a
// new one takes the pin of an earlier one no update used. It answers 201
// with the backup and its steps, as POST /api/v1/server-backups does;
// 200 with {"skipped": "no_key"} where no key is set, and the update
// goes on; a failure as an update's backup that failed
// (updateBackupFailed), which stops the update.
func (s *Server) postUpdateBackup(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.sbStore(); err != nil {
		return err
	}
	var req struct {
		To string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return fail(http.StatusBadRequest, "bad body: %v (want {\"to\": \"v…\"})", err)
	}
	to, ok := update.ParseVersion(req.To)
	if !ok {
		return fail(http.StatusBadRequest, "to: want the version the update installs, such as v0.17.0, not %q", req.To)
	}
	if cur, ok := update.ParseVersion(cmp.Or(s.opts.Update.Version, s.opts.Version)); ok && update.Compare(to, cur) == 0 {
		return fail(http.StatusConflict, "%s runs already: no update to back up for", req.To)
	}
	ctx, cancel := context.WithTimeout(r.Context(), updateBackupTimeout)
	defer cancel()
	b, outs, err := s.takeUpdateBackup(ctx, userIDOf(r))
	switch {
	case errors.Is(err, serverbackup.ErrNoKey):
		writeJSON(w, http.StatusOK, map[string]any{"skipped": skippedNoKey})
		return nil
	case err != nil:
		return updateBackupFailed(err)
	}
	auditTarget(r, "serverbackup:"+b.UUID)
	return s.answerBackup(w, r, http.StatusCreated, b, outs)
}
