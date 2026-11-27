// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The backup history's routes (docs/api-v1.md, "Backups"): a Cube's
// stored backups, listed newest first, taken by hand, downloaded as
// stored or in the other format, and deleted; the restore takes a
// stored one by id (restore_api.go). A Cube the store keeps a row of
// and no runtime serves, one taken out of the configuration or only
// found, keeps its backups listed, downloaded, compared and deleted, but
// takes no backup and no restore (AP BackupWatch v1.0, D4-B); the
// Cubes not served that keep backups are listed by a route of their own
// (D6-A).
//
//	GET    /api/v1/backups/unserved          the Cubes not served that keep backups
//	GET    /api/v1/cubes/{sn}/backups        the list, and whether a backup is due
//	POST   /api/v1/cubes/{sn}/backups        take one now, with an optional note
//	GET    /api/v1/cubes/{sn}/backups/{id}   the file (?format=json|yaml)
//	DELETE /api/v1/cubes/{sn}/backups/{id}   remove it
//	GET    /api/v1/cubes/{sn}/backups/{id}/diff?against={id}
//	                                         what would turn the other into this one

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
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

// history is the Cube a backup route is about: the runtime that serves
// it, nil for a row of the store that no runtime serves; the store, the
// row's id and the Cube's serial; and the devices a comparison leaves
// out, the hidden ones unless the server shows them.
type history struct {
	rt         *runtime
	st         *store.Store
	cubeID     int64
	serial     string
	hidden     []cube.RFAddress
	showHidden bool
}

// servedHistory is the history of a served Cube with a row in the
// store.
func servedHistory(rt *runtime) history {
	return history{rt: rt, st: rt.store, cubeID: rt.storeID.Load(), serial: rt.cubeSerial(), hidden: rt.config().Hidden, showHidden: rt.showHidden}
}

// backupHistory returns the request's Cube with its store and row:
// the served Cube's, errNoHistory for a served Cube without a row, or,
// for a serial no runtime serves, its row in the store that is not
// removed (unservedHistory); 404 without either.
func (s *Server) backupHistory(r *http.Request) (history, error) {
	rt, err := s.cube(r)
	if err != nil {
		return s.unservedHistory(r, err)
	}
	if rt.store == nil || rt.storeID.Load() == 0 {
		return history{}, errNoHistory
	}
	return servedHistory(rt), nil
}

// unservedHistory is the history of a serial that no runtime serves:
// its row in the store that is not removed, configured or not, with
// the row's hidden list; notFound, the 404 of s.cube, where there is no
// such row or no store.
func (s *Server) unservedHistory(r *http.Request, notFound error) (history, error) {
	st := s.opts.Store
	if st == nil {
		return history{}, notFound
	}
	row, ok, err := st.Cube(r.Context(), r.PathValue("sn"))
	if err != nil {
		return history{}, fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return history{}, notFound
	}
	h := history{st: st, cubeID: row.ID, serial: row.Serial, showHidden: s.opts.ShowHidden}
	if cfg, err := CubeFromRow(row); err == nil {
		h.hidden = cfg.Hidden
	}
	return h, nil
}

// notServed is the error of a backup taken or a restore asked for a
// Cube the store keeps a row of and no runtime serves.
func notServed(serial string) error {
	return refuse(http.StatusConflict, codeCubeNotServed, serial)
}

// servedCube returns the request's Cube for a route that needs it
// served: a serial whose row the store keeps and no runtime serves is
// notServed (409), any other unknown serial the 404 of s.cube.
func (s *Server) servedCube(r *http.Request) (*runtime, error) {
	rt, err := s.cube(r)
	if err == nil {
		return rt, nil
	}
	h, err := s.unservedHistory(r, err)
	if err != nil {
		return nil, err
	}
	return nil, notServed(h.serial)
}

// emitHistory sends an event about a Cube's backups: on its stream and
// into its log where a runtime serves it, else into its row's log in the
// store and the server's log lines.
func (s *Server) emitHistory(h history, ev event) {
	if h.rt != nil {
		s.emitBy(h.rt, ev)
		return
	}
	s.nameUser(context.Background(), &ev)
	ev.settle()
	eventLine(s.opts.Logger, h.serial, ev)
	h.st.AddEvent(storedEvent(h.cubeID, ev))
}

// unservedView is a Cube the store keeps backups of and no runtime
// serves, as GET /api/v1/backups/unserved lists it: its serial, its
// name where its row has one, the backups kept and when the newest was
// taken.
type unservedView struct {
	Serial  string    `json:"serial"`
	Name    string    `json:"name,omitempty"`
	Backups int       `json:"backups"`
	Newest  time.Time `json:"newest"`
}

// listUnserved handles GET /api/v1/backups/unserved: the Cube rows of
// the store that are not removed, keep a backup and that no runtime
// serves, by serial, from one query (store.KeptBackups); their backups
// are the routes' of …/backups (unservedHistory). Empty without a
// store, which keeps none.
func (s *Server) listUnserved(w http.ResponseWriter, r *http.Request) error {
	out := []unservedView{}
	if st := s.opts.Store; st != nil {
		kept, err := st.KeptBackups(r.Context())
		if err != nil {
			return fail(http.StatusInternalServerError, "%v", err)
		}
		for _, c := range kept {
			if !s.serves(c.Serial) {
				out = append(out, unservedView{Serial: c.Serial, Name: c.Name, Backups: c.Count, Newest: c.Newest})
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// serves reports whether a runtime serves serial, found as s.cube finds
// the path's Cube, so that a Cube listed as not served is one whose
// backup routes take its row (backupHistory).
func (s *Server) serves(serial string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cubes[strings.ToLower(serial)] != nil
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

// listBackups handles GET …/backups; a Cube no runtime serves has no
// watch, so its list says no changed and no due.
func (s *Server) listBackups(w http.ResponseWriter, r *http.Request) error {
	h, err := s.backupHistory(r)
	if err != nil {
		return err
	}
	var list []store.Backup
	var changed, due time.Time
	if rt := h.rt; rt != nil {
		set, setErr := h.st.BackupSettings(r.Context())
		// The rows and the state in one step against takeBackup, which
		// stores a row and makes the Cube clean under the same lock: a
		// backup is never listed beside the change it covered.
		rt.bk.rows.RLock()
		list, err = h.st.Backups(r.Context(), h.cubeID)
		if err == nil && setErr == nil {
			changed, due = rt.bk.state(set, s.backupMinute())
		}
		rt.bk.rows.RUnlock()
	} else {
		list, err = h.st.Backups(r.Context(), h.cubeID)
	}
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	out := backupList{Backups: make([]backupView, 0, len(list))}
	for _, b := range list {
		out.Backups = append(out.Backups, viewOfBackup(b))
	}
	if !changed.IsZero() {
		out.Changed, out.Due = &changed, &due
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// postBackup handles POST …/backups: a backup of the Cube now, with an
// optional {"note": …}.
func (s *Server) postBackup(w http.ResponseWriter, r *http.Request) error {
	h, err := s.backupHistory(r)
	if err != nil {
		return err
	}
	if h.rt == nil {
		return notServed(h.serial)
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
	b, _, err := s.takeBackup(r.Context(), h.rt, store.ReasonManual, userIDOf(r), req.Note)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, viewOfBackup(b))
	return nil
}

// getStoredBackup handles GET …/backups/{id}: the file as it was stored,
// or in the format ?format= asks for.
func (s *Server) getStoredBackup(w http.ResponseWriter, r *http.Request) error {
	h, err := s.backupHistory(r)
	if err != nil {
		return err
	}
	id, err := backupID(r)
	if err != nil {
		return err
	}
	b, ok, err := h.st.StoredBackup(r.Context(), h.cubeID, id)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return refuse(http.StatusNotFound, codeBackupUnknown, id, h.serial)
	}
	format := r.URL.Query().Get("format")
	switch format {
	case "":
		format = b.Format
	case "json", "yaml":
	default:
		return fail(http.StatusBadRequest, "format is json or yaml, not %q", format)
	}
	body, ok, err := h.st.BackupBody(r.Context(), h.cubeID, id)
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
	w.Header().Set("Content-Disposition", "attachment; filename=massimo-backup-"+h.serial+"-"+b.TakenAt.Local().Format("20060102-150405")+"."+format)
	w.Write(body)
	return nil
}

// deleteBackup handles DELETE …/backups/{id}. Deleting the newest
// makes the automatic backup forget its reference, read again from the
// one now newest at the next look; with none left, one follows at once.
// A Cube no runtime serves has no reference to forget: a runtime made
// for it later reads its reference at its first look, and drops windows
// kept against another (backups.go).
func (s *Server) deleteBackup(w http.ResponseWriter, r *http.Request) error {
	h, err := s.backupHistory(r)
	if err != nil {
		return err
	}
	id, err := backupID(r)
	if err != nil {
		return err
	}
	var ok bool
	if rt := h.rt; rt != nil {
		// The newest, the delete and forget() in one step against
		// takeBackup, which stores a row and makes it the reference
		// under the same lock; a newest not readable forgets as well.
		rt.bk.rows.Lock()
		newest, has, newestErr := h.st.NewestBackup(r.Context(), h.cubeID)
		ok, err = h.st.DeleteBackup(r.Context(), h.cubeID, id)
		if ok && (newestErr != nil || !has || newest.ID == id) && rt.bk.forget() {
			s.dropKept(r.Context(), rt)
		}
		rt.bk.rows.Unlock()
	} else {
		ok, err = h.st.DeleteBackup(r.Context(), h.cubeID, id)
	}
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return refuse(http.StatusNotFound, codeBackupUnknown, id, h.serial)
	}
	s.emitHistory(h, systemEvent("backup_deleted", userIDOf(r), map[string]any{"id": id}))
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// storedFile reads one of the Cube's stored backups as a file.
func (s *Server) storedFile(r *http.Request, h history, id int64) (*backup.File, error) {
	b, ok, err := h.st.StoredBackup(r.Context(), h.cubeID, id)
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return nil, refuse(http.StatusNotFound, codeBackupUnknown, id, h.serial)
	}
	body, ok, err := h.st.BackupBody(r.Context(), h.cubeID, id)
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
	h, err := s.backupHistory(r)
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
	opts.Hidden, opts.ShowHidden = h.hidden, h.showHidden
	newer, err := s.storedFile(r, h, id)
	if err != nil {
		return err
	}
	older, err := s.storedFile(r, h, against)
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
