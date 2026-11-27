// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// A new box started from a backup (AP ServerBackup v1.0, Block 11, D6-A
// and D7-A; docs/api-v1.md, "A new box from a backup"): while the server
// is open, before its first regular user (opensetup.go), whoever may make
// that user may instead hand the server a backup of another box with its
// key, and the server takes every scope of it and restarts, with the
// backup's users. The routes answer in that window alone, the principal's
// mode read as the request came, and a restore is refused once a regular
// user exists, the store counting again in the restore's own transaction
// (serverbackup.RestoreOptions.NoUsers), so that a first owner made
// meanwhile is never restored over. The backup comes as an upload, of at
// most maxServerBackupUpload bytes, or from a target the request
// describes, used for that request alone to list its backups by their
// manifests or to fetch the one picked, and never stored (the restore of
// the settings brings the old box's targets back). It waits in a
// temporary directory of the data directory, one at a time and for at
// most newBoxHeld, and goes with the restore, the next one handed, its
// time, or the window's end. The key comes in the body of the plan and of
// the restore, held in memory for that request alone, never written, and
// kept by no audit row (secretFields). No backup is taken first: a new
// box has nothing to keep and no key to encrypt it for. The restart that
// follows is the window's, from the local network too, as the first
// owner is: an open server otherwise restarts only from the box itself.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
)

// newBoxHeld is how long a backup handed to a new box waits for its
// restore.
const newBoxHeld = time.Hour

// maxNewBoxBody bounds the JSON body of a new box's routes: a key file,
// a target's fields and its secret.
const maxNewBoxBody = 1 << 20

// newBoxListWait bounds the listing of a target's backups, newBoxFetchWait
// the fetch of one.
const (
	newBoxListWait  = 2 * time.Minute
	newBoxFetchWait = time.Hour
)

// newBoxDir is the prefix of the temporary directory a backup handed to
// a new box waits in: serverbackup's, so that one a crash left goes with
// that package's stale temporary files.
const newBoxDir = ".serverbackup-newbox-*"

// newBoxClosed is the refusal of a new box's route on a server with users.
const newBoxClosed = "this server has users: a new box starts from a backup only before its first user; sign in, and restore from Maintenance → Server backups"

// newBoxState is the backup a new box was handed: its temporary
// directory and file, when it came, its manifest and size. Its file is
// read, replaced and removed only within the one job of the server's
// backups (sbJob), so that a plan or a restore never loses it midway.
type newBoxState struct {
	mu   sync.Mutex
	dir  string // "" for none
	path string
	at   time.Time
	m    serverbackup.Manifest
	size int64
	// receiving holds one upload or fetch at a time: each may write up
	// to maxServerBackupUpload bytes into the data directory.
	receiving atomic.Bool
}

// replace holds a backup received into dir, and removes the one held
// before.
func (n *newBoxState) replace(dir, path string, m serverbackup.Manifest, size int64, at time.Time) {
	n.mu.Lock()
	old := n.dir
	n.dir, n.path, n.m, n.size, n.at = dir, path, m, size, at
	n.mu.Unlock()
	if old != "" && old != dir {
		os.RemoveAll(old)
	}
}

// drop removes the backup held, if any.
func (n *newBoxState) drop() {
	n.mu.Lock()
	old := n.dir
	n.dir, n.path, n.m, n.size, n.at = "", "", serverbackup.Manifest{}, 0, time.Time{}
	n.mu.Unlock()
	if old != "" {
		os.RemoveAll(old)
	}
}

// take is the file of the backup held at now; one held longer than
// newBoxHeld goes, and none is 404.
func (n *newBoxState) take(now time.Time) (string, error) {
	n.mu.Lock()
	path, at := n.path, n.at
	n.mu.Unlock()
	switch {
	case path == "":
		return "", fail(http.StatusNotFound, "no backup was handed to this server: upload one, or fetch one from a target")
	case now.Sub(at) > newBoxHeld:
		n.drop()
		return "", fail(http.StatusNotFound, "the backup handed waited more than %s: hand it again", newBoxHeld)
	}
	return path, nil
}

// held reports whether a backup is held.
func (n *newBoxState) held() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.dir != ""
}

// newBoxRoutes registers the routes of a new box started from a backup.
// They take capOwner, so that on a server with users only its owner
// reaches them, to be told that the window has closed; on an open server
// every caller on the local network does, as every route.
func (s *Server) newBoxRoutes(api func(pattern, capability string, h func(http.ResponseWriter, *http.Request) error)) {
	api("POST /api/v1/setup/backup", "owner", s.uploadNewBox)
	api("POST /api/v1/setup/backup/list", "owner", s.listNewBoxTarget)
	api("POST /api/v1/setup/backup/fetch", "owner", s.fetchNewBoxTarget)
	api("POST /api/v1/setup/backup/plan", "owner", s.planNewBox)
	api("POST /api/v1/setup/backup/restore", "owner", s.restoreNewBox)
}

// newBoxOpen checks that a request comes in the first owner's window:
// a server with a store that is open, as the principal's mode says. On
// a server with users the backup held goes, where no job holds it, and
// the request is 409.
func (s *Server) newBoxOpen(r *http.Request) (*store.Store, error) {
	st, err := s.sbStore()
	if err != nil {
		return nil, err
	}
	if principalOf(r).mode == modeOpen {
		return st, nil
	}
	if s.newBox.held() {
		select {
		case s.sb.busy <- struct{}{}:
			s.newBox.drop()
			<-s.sb.busy
		default:
			// A job runs: the backup goes at its time, or with the
			// stale temporary files.
		}
	}
	return nil, fail(http.StatusConflict, "%s", newBoxClosed)
}

// readNewBoxBody reads a JSON body of at most maxNewBoxBody bytes into v;
// a field the route does not know is refused, and so is an empty body.
func readNewBoxBody(w http.ResponseWriter, r *http.Request, v any) error {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxNewBoxBody))
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		return fail(http.StatusRequestEntityTooLarge, "a body of more than %d bytes", maxNewBoxBody)
	case err != nil:
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	return nil
}

// newBoxBackupView is a backup handed to a new box as its manifest says
// it, read without the key: what it is, the box that made it, its rows
// per scope, and whether its store is of a newer schema than this
// server's, which takes it once updated.
type newBoxBackupView struct {
	UUID        string                `json:"uuid"`
	Time        time.Time             `json:"time"`
	Trigger     string                `json:"trigger"`
	Label       string                `json:"label,omitempty"`
	Version     string                `json:"version"`
	Schema      int                   `json:"schema"`
	Newer       bool                  `json:"newer"`
	Fingerprint string                `json:"fingerprint"`
	Size        int64                 `json:"size"`
	Counts      store.SnapshotCounts  `json:"counts"`
	Files       serverbackup.Files    `json:"files"`
	Name        string                `json:"name,omitempty"`
	Recipients  []newBoxRecipientView `json:"recipients"`
}

// newBoxRecipientView is a key a backup is encrypted for, by its label
// and fingerprint, so that its owner knows which key file opens it.
type newBoxRecipientView struct {
	Label       string `json:"label"`
	Fingerprint string `json:"fingerprint"`
}

// viewNewBoxBackup is a manifest as a new box's routes answer it; name
// is its file on a target, "" for an upload.
func viewNewBoxBackup(m serverbackup.Manifest, size int64, name string, live int) newBoxBackupView {
	v := newBoxBackupView{UUID: m.UUID, Time: m.Created, Trigger: string(m.Trigger), Label: m.Label, Version: m.Version, Schema: m.Schema.Version,
		Newer: m.Schema.Version > live, Fingerprint: m.Node.Fingerprint, Size: size, Counts: m.Counts, Files: m.Files, Name: name,
		Recipients: []newBoxRecipientView{}}
	for _, r := range m.Recipients {
		v.Recipients = append(v.Recipients, newBoxRecipientView{Label: r.Label, Fingerprint: r.Fingerprint})
	}
	return v
}

// receiveNewBox writes a backup through fill into a new temporary
// directory of the data directory, one at a time; checks it as a backup
// whose signature and payload are intact (serverbackup.Verify); and
// holds it in place of the one held before. fill answers its own errors
// as the request's. Where anything fails, nothing is held anew and the
// directory goes.
func (s *Server) receiveNewBox(ctx context.Context, st *store.Store, fill func(f *os.File) error) (newBoxBackupView, error) {
	if !s.newBox.receiving.CompareAndSwap(false, true) {
		return newBoxBackupView{}, fail(http.StatusConflict, "a backup is being received already: wait for it")
	}
	defer s.newBox.receiving.Store(false)
	dir, err := os.MkdirTemp(st.Dir(), newBoxDir)
	if err != nil {
		return newBoxBackupView{}, fail(http.StatusInternalServerError, "%v", err)
	}
	kept := false
	defer func() {
		if !kept {
			os.RemoveAll(dir)
		}
	}()
	path := filepath.Join(dir, "backup.zip")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return newBoxBackupView{}, fail(http.StatusInternalServerError, "%v", err)
	}
	err = fill(f)
	if cerr := f.Close(); err == nil && cerr != nil {
		err = fail(http.StatusInternalServerError, "%v", cerr)
	}
	if err != nil {
		return newBoxBackupView{}, err
	}
	fh, err := os.Open(path)
	if err != nil {
		return newBoxBackupView{}, fail(http.StatusInternalServerError, "%v", err)
	}
	fi, err := fh.Stat()
	var m serverbackup.Manifest
	if err == nil {
		m, _, err = serverbackup.Verify(fh, fi.Size())
	}
	fh.Close()
	if err != nil {
		return newBoxBackupView{}, sbFail(err)
	}
	err = s.sbJob(ctx, func() error {
		s.newBox.replace(dir, path, m, fi.Size(), s.sbClock().Now())
		return nil
	})
	if err != nil {
		return newBoxBackupView{}, sbFail(err)
	}
	kept = true
	live, _ := st.SchemaVersion()
	return viewNewBoxBackup(m, fi.Size(), "", live), nil
}

// uploadNewBox handles POST /api/v1/setup/backup: a backup file, the
// body, held for the new box's plan and restore; its body is not read
// ahead for the audit (streamedBodies).
func (s *Server) uploadNewBox(w http.ResponseWriter, r *http.Request) error {
	st, err := s.newBoxOpen(r)
	if err != nil {
		return err
	}
	if r.ContentLength > maxServerBackupUpload {
		return fail(http.StatusRequestEntityTooLarge, "a server backup of more than %d bytes", int64(maxServerBackupUpload))
	}
	v, err := s.receiveNewBox(r.Context(), st, func(f *os.File) error {
		_, err := io.Copy(f, http.MaxBytesReader(w, r.Body, maxServerBackupUpload))
		var tooLarge *http.MaxBytesError
		switch {
		case errors.As(err, &tooLarge):
			return fail(http.StatusRequestEntityTooLarge, "a server backup of more than %d bytes", int64(maxServerBackupUpload))
		case err != nil:
			return fail(http.StatusBadRequest, "the upload: %v", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	auditTarget(r, "serverbackup:"+v.UUID)
	writeJSON(w, http.StatusOK, map[string]any{"backup": v})
	return nil
}

// newBoxTarget is a target a new box's request describes, used for that
// request alone and never stored: its kind, its fields and its secret.
type newBoxTarget struct {
	Kind   string            `json:"kind"`
	Config map[string]string `json:"config"`
	Secret string            `json:"secret"`
}

// newBoxFetch is the body of a fetch: the target, and the file of the
// backup picked from its list.
type newBoxFetch struct {
	newBoxTarget
	Name string `json:"name"`
}

// open makes the target. A kind that makes its own secret (an SFTP
// target's key) is refused: the old box's key is in its backup alone.
func (t newBoxTarget) open(c *serverbackup.Catalogue) (serverbackup.Target, error) {
	k, ok := serverbackup.KindOf(t.Kind)
	if !ok {
		var names []string
		for _, k := range serverbackup.Kinds() {
			if k.MakeSecret == nil {
				names = append(names, k.Name)
			}
		}
		return nil, fail(http.StatusBadRequest, "kind %q: want one of %s", t.Kind, strings.Join(names, ", "))
	}
	if k.MakeSecret != nil {
		return nil, fail(http.StatusBadRequest, "a target of kind %s signs in with the old box's key, which only its backup holds: "+
			"download the backup from it and start from the file", k.Name)
	}
	if _, field := k.Secret(); !field && t.Secret != "" {
		return nil, fail(http.StatusBadRequest, "a target of kind %s takes no secret", k.Name)
	}
	tgt, err := c.Open(store.ServerBackupTarget{Name: k.Name, Kind: k.Name, Config: t.Config, Secret: t.Secret})
	if err != nil {
		return nil, fail(http.StatusBadRequest, "%v", err)
	}
	return tgt, nil
}

// listNewBoxTarget handles POST /api/v1/setup/backup/list: the backups
// on the target the body describes, found by their manifests (serverbackup
// Scan), the newest first, with the number of files that are none.
func (s *Server) listNewBoxTarget(w http.ResponseWriter, r *http.Request) error {
	st, err := s.newBoxOpen(r)
	if err != nil {
		return err
	}
	var req newBoxTarget
	if err := readNewBoxBody(w, r, &req); err != nil {
		return err
	}
	tgt, err := req.open(s.catalogue())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), newBoxListWait)
	defer cancel()
	found, skipped, err := serverbackup.Scan(ctx, tgt)
	if err != nil {
		return fail(http.StatusBadGateway, "%v", err)
	}
	live, _ := st.SchemaVersion()
	out := []newBoxBackupView{}
	for _, f := range found {
		out = append(out, viewNewBoxBackup(f.Manifest, f.Entry.Size, f.Entry.Name, live))
	}
	slices.SortStableFunc(out, func(a, b newBoxBackupView) int { return b.Time.Compare(a.Time) })
	writeJSON(w, http.StatusOK, map[string]any{"backups": out, "skipped": len(skipped)})
	return nil
}

// fetchNewBoxTarget handles POST /api/v1/setup/backup/fetch: the backup
// of the body's name, one the target lists, fetched from the target the
// body describes and held as an upload is.
func (s *Server) fetchNewBoxTarget(w http.ResponseWriter, r *http.Request) error {
	st, err := s.newBoxOpen(r)
	if err != nil {
		return err
	}
	var req newBoxFetch
	if err := readNewBoxBody(w, r, &req); err != nil {
		return err
	}
	if req.Name == "" {
		return fail(http.StatusBadRequest, "give name, a backup of the target's list")
	}
	tgt, err := req.open(s.catalogue())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), newBoxFetchWait)
	defer cancel()
	// Only a file the target lists as a backup: a name is no path.
	entries, err := tgt.List(ctx)
	if err != nil {
		return fail(http.StatusBadGateway, "%v", err)
	}
	i := slices.IndexFunc(entries, func(e serverbackup.Entry) bool { return e.Name == req.Name })
	if i < 0 {
		return fail(http.StatusNotFound, "the target holds no backup %q", req.Name)
	}
	if entries[i].Size > maxServerBackupUpload {
		return fail(http.StatusRequestEntityTooLarge, "a server backup of more than %d bytes", int64(maxServerBackupUpload))
	}
	v, err := s.receiveNewBox(ctx, st, func(f *os.File) error {
		src, size, err := tgt.Open(ctx, req.Name)
		if err != nil {
			return fail(http.StatusBadGateway, "%v", err)
		}
		defer src.Close()
		if size > maxServerBackupUpload {
			return fail(http.StatusRequestEntityTooLarge, "a server backup of more than %d bytes", int64(maxServerBackupUpload))
		}
		if _, err := io.Copy(f, io.NewSectionReader(src, 0, size)); err != nil {
			return fail(http.StatusBadGateway, "fetching %s: %v", req.Name, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	v.Name = req.Name
	auditTarget(r, "serverbackup:"+v.UUID)
	writeJSON(w, http.StatusOK, map[string]any{"backup": v})
	return nil
}

// newBoxRequest is the body of a new box's plan and restore: the key
// file's text, and whether to take the backup's identity, its env file
// and its TLS files; every scope is restored.
type newBoxRequest struct {
	KeyFile      string `json:"key_file"`
	TakeIdentity bool   `json:"take_identity"`
	Env          bool   `json:"env"`
	TLS          bool   `json:"tls"`
}

// newBoxOptions reads the body of a new box's plan or restore into the
// options of a restore of every scope, with no backup first and refused
// where the store has a regular user; a key that does not parse is 400.
func (s *Server) newBoxOptions(w http.ResponseWriter, r *http.Request) (serverbackup.RestoreOptions, error) {
	var req newBoxRequest
	if err := readNewBoxBody(w, r, &req); err != nil {
		return serverbackup.RestoreOptions{}, err
	}
	if req.KeyFile == "" {
		return serverbackup.RestoreOptions{}, fail(http.StatusBadRequest, "give key_file, the text of the key that opens the backup")
	}
	if _, err := serverbackup.ParseIdentities([]byte(req.KeyFile)); err != nil {
		return serverbackup.RestoreOptions{}, fail(http.StatusBadRequest, "%v", err)
	}
	o := serverbackup.RestoreOptions{Key: []byte(req.KeyFile), Scopes: []string{serverbackup.ScopeAll}, Identity: req.TakeIdentity, Env: req.Env,
		TLS: req.TLS, NoBackupFirst: true, NoUsers: true}
	if s.opts.RestoreTLSPlace != nil {
		o.TLSPlace = s.opts.RestoreTLSPlace()
	}
	return o, nil
}

// newBoxRestart is whether the server can restart itself after a new
// box's restore: as restartState says, but that an open server restarts
// here from the local network too, the window being the first owner's,
// and the restart following a restore that brought the server's users.
func (s *Server) newBoxRestart(r *http.Request) restartView {
	v, open := s.restartState(r)
	if open {
		v.Blocked, v.Available = "", true
	}
	return v
}

// newBoxFail maps an error of a new box's plan or restore to a status: a
// regular user made meanwhile is 409, else as restoreFail.
func newBoxFail(err error) error {
	if errors.Is(err, store.ErrHasUsers) {
		return fail(http.StatusConflict, "%s", newBoxClosed)
	}
	return restoreFail(err)
}

// planNewBox handles POST /api/v1/setup/backup/plan: what a restore of
// the backup held would do, every scope, the identity, the env file and
// the TLS files as the body asks, changing nothing; answered as POST
// …/server-backups/{id}/restore/plan answers.
func (s *Server) planNewBox(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.newBoxOpen(r); err != nil {
		return err
	}
	o, err := s.newBoxOptions(w, r)
	if err != nil {
		return err
	}
	var p serverbackup.RestorePlan
	err = s.sbJob(r.Context(), func() error {
		path, err := s.newBox.take(s.sbClock().Now())
		if err != nil {
			return err
		}
		o.File = path
		p, err = s.catalogue().Plan(r.Context(), o)
		return err
	})
	if err != nil {
		return newBoxFail(err)
	}
	v := s.newBoxRestart(r)
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
		"env": p.Env, "tls": p.TLS, "files": orEmpty(p.Files), "report": p.Report, "refused": p.Refused,
		"restart": map[string]any{"available": v.Available, "by": v.By, "blocked": v.Blocked},
	})
	return nil
}

// restoreNewBox handles POST /api/v1/setup/backup/restore: every scope of
// the backup held, the identity, the env file and the TLS files as the
// body asks, no backup taken first; then the server restarts, which the
// restore of users and sign-in needs, so that a server that cannot
// restart itself is refused first. Answered as POST
// …/server-backups/{id}/restore answers; the backup held goes once the
// server changed.
func (s *Server) restoreNewBox(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.newBoxOpen(r); err != nil {
		return err
	}
	o, err := s.newBoxOptions(w, r)
	if err != nil {
		return err
	}
	switch {
	case s.restartGate.asked.Load():
		return fail(http.StatusConflict, "a restart is under way")
	case s.updateRunning():
		return fail(http.StatusConflict, "an update is running")
	}
	v := s.newBoxRestart(r)
	if why := restartNeeded(o, v); why != "" {
		return fail(http.StatusConflict, "%s", why)
	}
	if !s.restartGate.asked.CompareAndSwap(false, true) {
		return fail(http.StatusConflict, "a restart is under way")
	}
	var res serverbackup.Restored
	err = s.sbJob(r.Context(), func() error {
		path, err := s.newBox.take(s.sbClock().Now())
		if err != nil {
			return err
		}
		o.File = path
		res, err = s.catalogue().Restore(r.Context(), o)
		if res.Committed {
			s.newBox.drop()
		}
		return err
	})
	if !res.Committed {
		s.restartGate.asked.Store(false)
		var code string
		switch {
		case errors.Is(err, store.ErrRestoreMissing):
			code = "missing"
		case errors.Is(err, store.ErrRestoreNoOwner):
			code = "no_owner"
		default:
			return newBoxFail(err)
		}
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": code, "report": res.Report})
		return nil
	}
	auditChange(r, "scopes", nil, res.Selection.Scopes)
	s.log.Info("a new box started from a backup", "scopes", strings.Join(res.Selection.Scopes, ","), "files", len(res.Files), "remote", s.clientIP(r))
	ans := serverRestoreAnswer{At: timeOrNil(res.At), Scopes: orEmpty(res.Selection.Scopes), Serials: orEmpty(res.Selection.Serials), Report: res.Report,
		Files: orEmpty(res.Files), Cubes: orEmpty(res.Report.Cubes)}
	// Once the server changed it restarts, an error of a file after the
	// store's commit included.
	if rerr := s.restartNow(r, v); rerr != nil {
		ans.Restart.Blocked = "starting the restart: " + rerr.Error()
	} else {
		ans.Restart.Started, ans.Restart.By = true, v.By
	}
	status := http.StatusOK
	if err != nil {
		status, ans.Error = http.StatusInternalServerError, err.Error()
	}
	writeJSON(w, status, ans)
	return nil
}
