// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The backup history's routes (docs/api-v1.md, "Backups"): a Cube's
// stored backups, listed newest first, taken by hand, downloaded as
// stored or in the other format, and deleted; the restore takes a
// stored one by id (restore_api.go).
//
//	GET    /api/v1/cubes/{sn}/backups        the list, and whether a backup is due
//	POST   /api/v1/cubes/{sn}/backups        take one now, with an optional note
//	GET    /api/v1/cubes/{sn}/backups/{id}   the file (?format=json|yaml)
//	DELETE /api/v1/cubes/{sn}/backups/{id}   remove it
//	GET    /api/v1/cubes/{sn}/backups/{id}/diff?against={id}
//	                                         what would turn the other into this one

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/store"
)

// maxBackupNote is the longest note a backup taken by hand keeps.
const maxBackupNote = 200

// backupView is a stored backup as the API shows it.
type backupView struct {
	ID      int64     `json:"id"`
	TakenAt time.Time `json:"taken_at"`
	Reason  string    `json:"reason"`
	Format  string    `json:"format"`
	Tool    string    `json:"tool,omitempty"`
	Size    int64     `json:"size"`
	SHA256  string    `json:"sha256"`
	User    string    `json:"user,omitempty"`
	Note    string    `json:"note,omitempty"`
}

func viewOfBackup(b store.Backup) backupView {
	return backupView{ID: b.ID, TakenAt: b.TakenAt, Reason: b.Reason, Format: b.Format, Tool: b.Tool, Size: b.Size, SHA256: b.SHA256, User: b.UserName, Note: b.Note}
}

// backupList is the answer of GET …/backups: the backups, newest first,
// and when the Cube's configuration last changed against the newest one
// and its automatic backup is due, absent when it did not change.
type backupList struct {
	Backups []backupView `json:"backups"`
	Changed *time.Time   `json:"changed,omitempty"`
	Due     *time.Time   `json:"due,omitempty"`
}

// backupHistory returns the request's Cube with its store and row, or
// errNoHistory.
func (s *Server) backupHistory(r *http.Request) (*runtime, *store.Store, int64, error) {
	rt, err := s.cube(r)
	if err != nil {
		return nil, nil, 0, err
	}
	if rt.store == nil || rt.storeID.Load() == 0 {
		return nil, nil, 0, errNoHistory
	}
	return rt, rt.store, rt.storeID.Load(), nil
}

// userIDOf is the request's user, 0 outside users mode.
func userIDOf(r *http.Request) int64 {
	if p := principalOf(r); p.named() {
		return p.user.ID
	}
	return 0
}

// backupID reads {id}; a malformed one is no backup of the Cube.
func backupID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, fail(http.StatusNotFound, "no backup %q", r.PathValue("id"))
	}
	return id, nil
}

// listBackups handles GET …/backups.
func (s *Server) listBackups(w http.ResponseWriter, r *http.Request) error {
	rt, st, cubeID, err := s.backupHistory(r)
	if err != nil {
		return err
	}
	list, err := st.Backups(r.Context(), cubeID)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	out := backupList{Backups: make([]backupView, 0, len(list))}
	for _, b := range list {
		out.Backups = append(out.Backups, viewOfBackup(b))
	}
	if set, err := st.BackupSettings(r.Context()); err == nil {
		if changed, due := rt.bk.state(set, s.backupMinute()); !changed.IsZero() {
			out.Changed, out.Due = &changed, &due
		}
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// postBackup handles POST …/backups: a backup of the Cube now, with an
// optional {"note": …}.
func (s *Server) postBackup(w http.ResponseWriter, r *http.Request) error {
	rt, _, _, err := s.backupHistory(r)
	if err != nil {
		return err
	}
	var req struct {
		Note string `json:"note"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil && err != io.EOF {
		return fail(http.StatusBadRequest, "bad body: %v (want {\"note\": \"…\"} or nothing)", err)
	}
	req.Note = strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(req.Note) > maxBackupNote {
		return fail(http.StatusBadRequest, "the note holds at most %d characters", maxBackupNote)
	}
	b, _, err := s.takeBackup(r.Context(), rt, store.ReasonManual, userIDOf(r), req.Note)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, viewOfBackup(b))
	return nil
}

// getStoredBackup handles GET …/backups/{id}: the file as it was stored,
// or in the format ?format= asks for.
func (s *Server) getStoredBackup(w http.ResponseWriter, r *http.Request) error {
	rt, st, cubeID, err := s.backupHistory(r)
	if err != nil {
		return err
	}
	id, err := backupID(r)
	if err != nil {
		return err
	}
	b, ok, err := st.StoredBackup(r.Context(), cubeID, id)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return fail(http.StatusNotFound, "no backup %d of cube %s", id, rt.serial)
	}
	format := r.URL.Query().Get("format")
	switch format {
	case "":
		format = b.Format
	case "json", "yaml":
	default:
		return fail(http.StatusBadRequest, "format is json or yaml, not %q", format)
	}
	body, ok, err := st.BackupBody(r.Context(), cubeID, id)
	if err != nil || !ok {
		return fail(http.StatusInternalServerError, "backup %d: %v", id, err)
	}
	if format != b.Format {
		f, err := backup.Unmarshal(body, b.Format == "yaml")
		if err != nil {
			return fail(http.StatusInternalServerError, "backup %d: %v", id, err)
		}
		if body, err = f.Marshal(format == "yaml"); err != nil {
			return fail(http.StatusInternalServerError, "backup %d: %v", id, err)
		}
	}
	w.Header().Set("Content-Type", "application/"+format)
	w.Header().Set("Content-Disposition", "attachment; filename=massimo-backup-"+rt.serial+"-"+b.TakenAt.Local().Format("20060102-150405")+"."+format)
	w.Write(body)
	return nil
}

// deleteBackup handles DELETE …/backups/{id}.
func (s *Server) deleteBackup(w http.ResponseWriter, r *http.Request) error {
	rt, st, cubeID, err := s.backupHistory(r)
	if err != nil {
		return err
	}
	id, err := backupID(r)
	if err != nil {
		return err
	}
	ok, err := st.DeleteBackup(r.Context(), cubeID, id)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return fail(http.StatusNotFound, "no backup %d of cube %s", id, rt.serial)
	}
	s.log.Info("backup deleted", "cube", rt.serial, "id", id)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// storedFile reads one of the Cube's stored backups as a file.
func (s *Server) storedFile(r *http.Request, rt *runtime, st *store.Store, cubeID, id int64) (*backup.File, error) {
	b, ok, err := st.StoredBackup(r.Context(), cubeID, id)
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return nil, fail(http.StatusNotFound, "no backup %d of cube %s", id, rt.serial)
	}
	body, ok, err := st.BackupBody(r.Context(), cubeID, id)
	if err != nil || !ok {
		return nil, fail(http.StatusInternalServerError, "backup %d: %v", id, err)
	}
	f, err := backup.Unmarshal(body, b.Format == "yaml")
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "backup %d: %v", id, err)
	}
	return f, nil
}

// backupDiff is the answer of GET …/backups/{id}/diff: the steps a
// restore of the backup would send to a Cube as the other backup found
// it, with the planner's notes; nothing is sent.
type backupDiff struct {
	Steps         []restoreStep `json:"steps"`
	Notes         []string      `json:"notes"`
	RadioCommands int           `json:"radio_commands"`
}

// diffBackups handles GET …/backups/{id}/diff?against={id}: the restore
// planner's comparison of the backup with the Cube as the other one's
// raw lines record it, with the restore's options from the query.
func (s *Server) diffBackups(w http.ResponseWriter, r *http.Request) error {
	rt, st, cubeID, err := s.backupHistory(r)
	if err != nil {
		return err
	}
	id, err := backupID(r)
	if err != nil {
		return err
	}
	against, err := strconv.ParseInt(r.URL.Query().Get("against"), 10, 64)
	if err != nil || against < 1 {
		return fail(http.StatusBadRequest, "against=<id> names the backup to compare with")
	}
	opts, err := restoreOptions(r.URL.Query())
	if err != nil {
		return err
	}
	opts.Hidden, opts.ShowHidden = rt.config().Hidden, rt.showHidden
	newer, err := s.storedFile(r, rt, st, cubeID, id)
	if err != nil {
		return err
	}
	older, err := s.storedFile(r, rt, st, cubeID, against)
	if err != nil {
		return err
	}
	msgs, err := older.Messages()
	if err != nil {
		return fail(http.StatusInternalServerError, "backup %d: %v", against, err)
	}
	snap, err := cubelan.SnapshotOf(msgs, older.TakenAt)
	if err != nil {
		return fail(http.StatusInternalServerError, "backup %d: %v", against, err)
	}
	p, err := backup.NewPlan(newer, backup.Current{Snapshot: snap, NTP: older.NTP}, opts)
	if err != nil {
		return fail(http.StatusInternalServerError, "backup %d against %d: %v", id, against, err)
	}
	v := planView(p, cubelan.Budget{})
	writeJSON(w, http.StatusOK, backupDiff{Steps: v.Steps, Notes: v.Notes, RadioCommands: v.RadioCommands})
	return nil
}
