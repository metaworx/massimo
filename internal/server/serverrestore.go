// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The restore of a server backup by scope (AP ServerBackup v1.0, Block 5,
// D6-A and D7-A; docs/api-v1.md, "Server backups"), the owner's alone
// (capOwner): POST /api/v1/server-backups/{id}/restore/plan answers what
// a restore would do, POST …/{id}/restore restores and restarts the
// server, POST …/{id}/check-key opens a backup with the key and checks
// its store. The key comes in the body, key_file, which no audit row
// keeps (secretFields); it is held in memory for the request alone and
// never written. Each runs as the one job of the server's backups.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
)

// browseOtherBox is how long the plan of a backup from another box looks
// for that box on the network.
const browseOtherBox = 2 * time.Second

// restoreRequest is the body of the restore's routes: the key file's
// text, the scopes (serverbackup.ParseScopes), and whether to take the
// backup's identity, its env file and its TLS files.
type restoreRequest struct {
	KeyFile      string   `json:"key_file"`
	Scopes       []string `json:"scopes"`
	TakeIdentity bool     `json:"take_identity"`
	Env          bool     `json:"env"`
	TLS          bool     `json:"tls"`
}

// restoreOptions reads the body of a restore's route into the options:
// a key that does not parse, or a scope that is none, is 400.
func (s *Server) restoreOptions(r *http.Request) (serverbackup.RestoreOptions, error) {
	var req restoreRequest
	if err := decodeBody(r, &req); err != nil {
		return serverbackup.RestoreOptions{}, err
	}
	if req.KeyFile == "" {
		return serverbackup.RestoreOptions{}, fail(http.StatusBadRequest, "give key_file, the text of the key that opens the backup")
	}
	if _, err := serverbackup.ParseIdentities([]byte(req.KeyFile)); err != nil {
		return serverbackup.RestoreOptions{}, fail(http.StatusBadRequest, "%v", err)
	}
	if _, err := serverbackup.ParseScopes(req.Scopes); err != nil {
		return serverbackup.RestoreOptions{}, fail(http.StatusBadRequest, "%v", err)
	}
	o := serverbackup.RestoreOptions{Backup: r.PathValue("id"), Key: []byte(req.KeyFile), Scopes: req.Scopes, Identity: req.TakeIdentity,
		Env: req.Env, TLS: req.TLS, UserID: principalOf(r).userID()}
	if s.opts.RestoreTLSPlace != nil {
		o.TLSPlace = s.opts.RestoreTLSPlace()
	}
	return o, nil
}

// restoreFail maps an error of a restore to a status: a backup not found
// 404, a key that does not open it or a newer schema 422, then as
// sbFail.
func restoreFail(err error) error {
	switch {
	case errors.Is(err, serverbackup.ErrWrongKey), errors.Is(err, serverbackup.ErrNewerSchema):
		return fail(http.StatusUnprocessableEntity, "%v", err)
	}
	return sbFail(err)
}

// backupIdentityView is what the plan says of the backup's identity: this
// box's, or another box's, and whether that box still answers on the
// network, nil where it could not be looked for.
type backupIdentityView struct {
	ThisBox     bool   `json:"this_box"`
	Fingerprint string `json:"fingerprint"`
	Node        string `json:"node"`
	Answers     *bool  `json:"answers"`
}

// planServerRestore handles POST /api/v1/server-backups/{id}/restore/plan:
// what a restore of the body's scopes and files would do, the scopes
// every one where none is named, changing nothing.
func (s *Server) planServerRestore(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.sbStore(); err != nil {
		return err
	}
	o, err := s.restoreOptions(r)
	if err != nil {
		return err
	}
	var p serverbackup.RestorePlan
	err = s.sbJob(r.Context(), func() error {
		var err error
		p, err = s.catalogue().Plan(r.Context(), o)
		return err
	})
	if err != nil {
		return restoreFail(err)
	}
	v, _ := s.restartState(r)
	if why := restartNeeded(o, v); why != "" && p.Refused == "" {
		p.Refused = why
	}
	id := backupIdentityView{ThisBox: p.ThisBox, Fingerprint: p.Manifest.Node.Fingerprint, Node: p.NodeID}
	if !p.ThisBox {
		id.Answers = s.boxAnswers(r.Context(), p.NodeID)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"backup": map[string]any{"uuid": p.Manifest.UUID, "time": p.Manifest.Created, "trigger": p.Manifest.Trigger, "label": p.Manifest.Label,
			"version": p.Manifest.Version, "schema": p.Manifest.Schema.Version},
		"identity": id, "migrated": p.Migrated, "scopes": p.Selection.Scopes, "serials": p.Selection.Serials, "cubes": orEmpty(p.Cubes),
		"env": p.Env, "tls": p.TLS, "env_refused": p.EnvRefused, "tls_refused": p.TLSRefused, "files": orEmpty(p.Files), "report": p.Report,
		"refused": p.Refused,
		"restart": map[string]any{"available": v.Available, "by": v.By, "blocked": v.Blocked},
	})
	return nil
}

// orEmpty is s, or an empty list for none, so that an answer's list is
// never null.
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// boxAnswers looks for the box of a node id on the network, as the
// discovery's browse does; nil where it cannot be looked for.
func (s *Server) boxAnswers(ctx context.Context, node string) *bool {
	if s.opts.BrowseServers == nil || node == "" {
		return nil
	}
	found, err := s.opts.BrowseServers(ctx, browseOtherBox)
	if err != nil {
		return nil
	}
	answers := false
	for _, f := range found {
		answers = answers || f.Node == node
	}
	return &answers
}

// serverRestartAnswer is what a restore says of the restart that follows it:
// started, and by whom, or why not, the server then to be restarted by
// hand, since its store keeps opaque.key and its node in memory.
type serverRestartAnswer struct {
	Started bool   `json:"started"`
	By      string `json:"by,omitempty"`
	Blocked string `json:"blocked,omitempty"`
}

// serverRestoreAnswer is the answer of a restore, the restart first.
type serverRestoreAnswer struct {
	Restart serverRestartAnswer `json:"restart"`
	// Error is a failure after the store's commit: a file not put in
	// place; the server restarts all the same.
	Error  string `json:"error,omitempty"`
	Before string `json:"before_backup,omitempty"`
	// BeforeTime and At are when the backup taken first was made and
	// when the store was restored: what the server wrote between them is
	// in neither.
	BeforeTime *time.Time                 `json:"before_backup_time,omitempty"`
	At         *time.Time                 `json:"restored_at,omitempty"`
	Scopes     []string                   `json:"scopes"`
	Serials    []string                   `json:"serials"`
	Report     store.RestoreReport        `json:"report"`
	Files      []serverbackup.PlannedFile `json:"files"`
	// Cubes are the Cubes restored whose configuration the Cube restore
	// could write onto them (POST /api/v1/cubes/{sn}/restore).
	Cubes []store.RestoredCube `json:"cubes"`
}

// restoreServerBackup handles POST /api/v1/server-backups/{id}/restore:
// the body's scopes and files restored from the backup, a pinned local
// backup taken first, then the server restarted. A restore the store
// refuses is 409 with the report: what is missing, or no owner left.
func (s *Server) restoreServerBackup(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.sbStore(); err != nil {
		return err
	}
	o, err := s.restoreOptions(r)
	if err != nil {
		return err
	}
	switch {
	case s.restartGate.asked.Load():
		return fail(http.StatusConflict, "a restart is under way")
	case s.updateRunning():
		return fail(http.StatusConflict, "an update is running")
	}
	// The restart is taken before the restore: a store that took the
	// backup's opaque.key, or a node.key replaced, must not stay with the
	// keys this server holds in memory.
	v, _ := s.restartState(r)
	if why := restartNeeded(o, v); why != "" {
		return fail(http.StatusConflict, "%s", why)
	}
	held := v.Available && s.restartGate.asked.CompareAndSwap(false, true)
	if v.Available && !held {
		return fail(http.StatusConflict, "a restart is under way")
	}
	var res serverbackup.Restored
	err = s.sbJob(r.Context(), func() error {
		var err error
		res, err = s.catalogue().Restore(r.Context(), o)
		if len(res.Steps) > 0 {
			s.reportServerBackup(res.Steps, serverbackup.TriggerBeforeRestore, o.UserID)
		}
		return err
	})
	if !res.Committed {
		if held {
			s.restartGate.asked.Store(false)
		}
		var code string
		switch {
		case errors.Is(err, store.ErrRestoreMissing):
			code = "missing"
		case errors.Is(err, store.ErrRestoreNoOwner):
			code = "no_owner"
		case err != nil:
			return restoreFail(err)
		}
		if code != "" {
			writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": code, "report": res.Report})
			return nil
		}
	}
	auditChange(r, "scopes", nil, res.Selection.Scopes)
	ans := serverRestoreAnswer{Before: res.Before.UUID, BeforeTime: timeOrNil(res.Before.Time), At: timeOrNil(res.At), Scopes: orEmpty(res.Selection.Scopes),
		Serials: orEmpty(res.Selection.Serials), Report: res.Report, Files: orEmpty(res.Files), Cubes: orEmpty(res.Report.Cubes)}
	// Once the server changed it restarts, an error of a file after the
	// store's commit included.
	switch {
	case !held:
		ans.Restart.Blocked = v.Blocked
	default:
		if rerr := s.restartNow(r, v); rerr != nil {
			ans.Restart.Blocked = "starting the restart: " + rerr.Error()
		} else {
			ans.Restart.Started, ans.Restart.By = true, v.By
		}
	}
	status := http.StatusOK
	if err != nil {
		status, ans.Error = http.StatusInternalServerError, err.Error()
	}
	writeJSON(w, status, ans)
	return nil
}

// restartNeeded is why a restore cannot go without the restart v
// allows: users and sign-in take the backup's opaque.key, the identity
// its node.key, and the server holds both in memory; "" where it can.
func restartNeeded(o serverbackup.RestoreOptions, v restartView) string {
	sel, _ := serverbackup.ParseScopes(o.Scopes)
	if v.Available || !sel.Has(serverbackup.ScopeUsers) && !o.Identity {
		return ""
	}
	return "this server cannot restart itself (" + v.Blocked + "), and a restore of users and sign-in or of the identity needs it: " +
		"stop the server and run massimo store restore on the box"
}

// checkServerBackupKey handles POST /api/v1/server-backups/{id}/check-key:
// the backup opened with the body's key, its files listed and its store
// checked (PRAGMA integrity_check), changing nothing.
func (s *Server) checkServerBackupKey(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.sbStore(); err != nil {
		return err
	}
	o, err := s.restoreOptions(r)
	if err != nil {
		return err
	}
	var kc serverbackup.KeyCheck
	err = s.sbJob(r.Context(), func() error {
		var err error
		kc, err = s.catalogue().CheckKey(r.Context(), o)
		return err
	})
	if err != nil {
		return restoreFail(err)
	}
	files := []map[string]string{}
	for _, f := range kc.Files {
		files = append(files, map[string]string{"name": f.Name, "role": string(f.Role)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": kc.OK, "integrity": kc.Integrity, "this_box": kc.ThisBox, "files": files,
		"backup": map[string]any{"uuid": kc.Manifest.UUID, "time": kc.Manifest.Created, "schema": kc.Manifest.Schema.Version}})
	return nil
}
