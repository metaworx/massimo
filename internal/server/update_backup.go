// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The server's backup before an update (AP ServerBackup v1.0, Block 10;
// docs/api-v1.md, "POST /api/v1/update"): every update the server
// starts, POST /api/v1/update, Install now and the night's (beginUpdate),
// first takes a backup of the server to its local targets, pinned for 30
// days (serverbackup.PinBefore). It runs as the server, under whoever
// holds update, with no capability of the backups, as the one job of the
// server's backups (sbJob); its id goes into the update's history and
// into its line of the log. Without a key there is nothing to encrypt
// for: the update goes on, and its line says so. A backup that fails
// stops the update, which says why, and GET /api/v1/update says it until
// an update starts; no_backup, Update without a backup, goes on without
// one and is an audit row of its own, update.no_backup. POST
// /api/v1/update/backup takes the same backup alone, for massimo update
// by hand, which installs the release itself.

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
)

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

// updateBackupTaken is the backup an update took first: its uuid, or why
// it took none (skippedAsked, skippedNoKey); both empty where the server
// has no store to back up, or one opened read-only.
type updateBackupTaken struct {
	uuid, skipped string
}

// updateBackupFailure is the last update's backup that failed, when and
// why, until an update starts; kept in memory: after a restart the next
// Update tries again.
type updateBackupFailure struct {
	mu  sync.Mutex
	at  time.Time
	err string
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

// takeUpdateBackup takes the backup an update takes first, for userID (0
// for the owner's key or the server itself): to the local targets,
// pinned for serverbackup.PinBefore, as the one job of the server's
// backups, its steps in the log (reportServerBackup); one that failed
// leaves no row, as no backup by hand does. serverbackup.ErrNoKey says
// that no key is set.
func (s *Server) takeUpdateBackup(ctx context.Context, userID int64) (store.ServerBackup, []serverbackup.Outcome, error) {
	var b store.ServerBackup
	var outs []serverbackup.Outcome
	err := s.sbJob(ctx, func() error {
		var err error
		b, outs, err = s.catalogue().Make(ctx, serverbackup.MakeRequest{Trigger: serverbackup.TriggerBeforeUpdate, UserID: userID,
			Local: true, PinFor: serverbackup.PinBefore})
		s.reportServerBackup(outs, serverbackup.TriggerBeforeUpdate, userID)
		return err
	})
	return b, outs, err
}

// backupForUpdate is the backup beginUpdate takes before it asks the
// helper: none where noBackup asks for none, or where there is no store
// or only a read-only one; none with a line in the log where no key is
// set; else takeUpdateBackup's, whose error stops the update.
func (s *Server) backupForUpdate(ctx context.Context, userID int64, noBackup bool) (updateBackupTaken, error) {
	st := s.opts.Store
	switch {
	case noBackup:
		return updateBackupTaken{skipped: skippedAsked}, nil
	case st == nil:
		return updateBackupTaken{}, nil
	case st.ReadOnly():
		s.log.Info("update: the store is open read-only; the update goes on without a backup of the server")
		return updateBackupTaken{}, nil
	}
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

// updateBackupFailed is the answer of an update whose backup failed, the
// update not started: 409 where no local target is switched on, 503
// where the request ended first, else 500; its text says how to go on
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

// noteBackupFailure keeps the failure of an update's backup, err, for
// GET /api/v1/update; nil forgets it, once an update has started.
func (s *Server) noteBackupFailure(err error) {
	f := &s.updates.backup
	f.mu.Lock()
	defer f.mu.Unlock()
	if err == nil {
		f.at, f.err = time.Time{}, ""
		return
	}
	f.at, f.err = time.Now(), err.Error()
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
	if !slices.ContainsFunc(keys, func(k store.ServerBackupKey) bool { return !k.Pending }) {
		return &updateBackupView{State: backupNoKey}
	}
	f := &s.updates.backup
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.at.IsZero() {
		return &updateBackupView{State: backupFailed, Error: f.err, At: timeOrNil(f.at)}
	}
	return &updateBackupView{State: backupReady}
}

// postUpdateBackup handles POST /api/v1/update/backup, for a holder of
// update: the backup an update takes first, alone, for massimo update by
// hand (cmd/massimo), which then installs the release itself. It answers
// 201 with the backup and its steps, as POST /api/v1/server-backups
// does; 200 with {"skipped": "no_key"} where no key is set, and the
// update goes on; a failure as an update's backup that failed
// (updateBackupFailed), which stops the update.
func (s *Server) postUpdateBackup(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.sbStore(); err != nil {
		return err
	}
	b, outs, err := s.takeUpdateBackup(r.Context(), userIDOf(r))
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
