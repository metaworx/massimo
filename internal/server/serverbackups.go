// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The server's own backups (AP ServerBackup v1.0, Block 2, D11-A, D12-A,
// D13-A, D15-A; docs/store.md, "Server backups"; docs/api-v1.md, "Server
// backups"): the daily schedule in the box's local time, a run missed
// made five minutes after the start, the copies by what each target
// takes and each target's retention after them (internal/serverbackup,
// Catalogue), the retries of a failed backup or copy after 5 min, 30 min
// and 2 h, then a notification, a warning in the log and its alert; and
// the routes. Nothing is made while no key is set. One job runs at a
// time: the schedule's, a retry, or one asked for through the API.

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
)

// serverBackupLate is how long after the start a run the schedule missed
// while the server was down is made.
const serverBackupLate = 5 * time.Minute

// serverBackupMinWait is the loop's shortest wait, so that a retry whose
// time is past and that a failure could not move on is not run again at
// once, over and over.
const serverBackupMinWait = time.Second

// maxServerBackupUpload bounds a server backup uploaded.
const maxServerBackupUpload = 8 << 30

// serverBackups is the state of the server's backups.
type serverBackups struct {
	// busy holds one job at a time; wake wakes the loop: a timer, a
	// setting changed, a key or a target added.
	busy chan struct{}
	wake chan struct{}
	// keyMu serialises the keys' changes, so that two of other kinds
	// cannot both pass the check of the kinds.
	keyMu sync.Mutex
	// armed counts the loop's timers set, for the tests, which move
	// the clock only once the loop waits.
	armed atomic.Int64
}

// newServerBackups makes the state.
func newServerBackups() serverBackups {
	return serverBackups{busy: make(chan struct{}, 1), wake: make(chan struct{}, 1)}
}

// wakeServerBackups wakes the loop, which reads the schedule again.
func (s *Server) wakeServerBackups() {
	select {
	case s.sb.wake <- struct{}{}:
	default:
	}
}

// sbClock is the schedule's clock: Options.serverBackupClock in the
// tests.
func (s *Server) sbClock() notify.Clock {
	if s.opts.serverBackupClock != nil {
		return s.opts.serverBackupClock
	}
	return realClock{}
}

// sbLocation is the box's time zone, whose time the schedule's is and
// whose days, weeks and months the targets count.
func (s *Server) sbLocation() *time.Location {
	if s.opts.serverBackupZone != nil {
		return s.opts.serverBackupZone
	}
	return time.Local
}

// catalogue is the catalogue's jobs over the store, with what serve was
// started with: its env file and its TLS files.
func (s *Server) catalogue() *serverbackup.Catalogue {
	var tls []serverbackup.TLSFile
	if s.opts.BackupTLSFiles != nil {
		tls = s.opts.BackupTLSFiles()
	}
	version := cmp.Or(s.opts.Update.Version, s.opts.Version)
	return &serverbackup.Catalogue{Store: s.opts.Store, EnvFile: s.opts.SettingsFile, TLSFiles: tls, Version: version,
		Now: s.sbClock().Now, Location: s.sbLocation(), Log: s.partLog("serverbackup", nil)}
}

// sbJob runs f as the one job, waiting for one that runs, of this
// server or of a command beside it (serverbackup.LockJobs); ctx ending
// first is an error.
func (s *Server) sbJob(ctx context.Context, f func() error) error {
	select {
	case s.sb.busy <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-s.sb.busy }()
	unlock, err := serverbackup.LockJobs(ctx, s.opts.Store.Dir())
	if err != nil {
		return err
	}
	defer unlock()
	return f()
}

// scheduleTime reads the daily time, HH:MM; a setting that does not
// parse is the default, with a warning.
func (s *Server) scheduleTime(ctx context.Context) (hour, minute int) {
	v, err := s.opts.Store.SettingString(ctx, store.ServerBackupTimeKey)
	if err == nil && v != "" {
		if h, m, err := parseClock(v); err == nil {
			return h, m
		}
		s.log.Warn("server backups: the setting's time does not parse, the default applies", "key", store.ServerBackupTimeKey, "value", v)
	}
	h, m, _ := parseClock(store.DefaultServerBackupTime)
	return h, m
}

// parseClock reads HH:MM, 00:00 to 23:59.
func parseClock(v string) (hour, minute int, err error) {
	t, err := time.Parse("15:04", strings.TrimSpace(v))
	if err != nil || len(strings.TrimSpace(v)) != 5 {
		return 0, 0, fmt.Errorf("%q: want a time HH:MM, such as 03:30", v)
	}
	return t.Hour(), t.Minute(), nil
}

// slotAfter is the first run of the schedule after t, in loc: that day's
// time, or the next day's. A time a change to summer time skips is the
// hour after it (time.Date's normalisation); one a change back repeats
// is its first.
func slotAfter(t time.Time, hour, minute int, loc *time.Location) time.Time {
	l := t.In(loc)
	at := time.Date(l.Year(), l.Month(), l.Day(), hour, minute, 0, 0, loc)
	for !at.After(t) {
		l = l.AddDate(0, 0, 1)
		at = time.Date(l.Year(), l.Month(), l.Day(), hour, minute, 0, 0, loc)
	}
	return at
}

// slotBefore is the last run of the schedule at or before t.
func slotBefore(t time.Time, hour, minute int, loc *time.Location) time.Time {
	l := t.In(loc)
	at := time.Date(l.Year(), l.Month(), l.Day(), hour, minute, 0, 0, loc)
	if at.After(t) {
		l = l.AddDate(0, 0, -1)
		at = time.Date(l.Year(), l.Month(), l.Day(), hour, minute, 0, 0, loc)
	}
	return at
}

// missedRun reports whether the schedule missed its last run while the
// server was down: no scheduled backup with a copy there since it.
func (s *Server) missedRun(ctx context.Context, now time.Time) bool {
	h, m := s.scheduleTime(ctx)
	last := slotBefore(now, h, m, s.sbLocation())
	bs, err := s.opts.Store.ServerBackups(ctx)
	if err != nil {
		return false
	}
	for _, b := range bs {
		if b.Trigger == string(serverbackup.TriggerScheduled) && !b.Time.Before(last) &&
			slices.ContainsFunc(b.Copies, func(c store.ServerBackupCopy) bool { return c.State == store.CopyThere }) {
			return false
		}
	}
	return true
}

// nextRetry is the earliest retry due of a failed backup or copy, zero
// for none.
func (s *Server) nextRetry(ctx context.Context) time.Time {
	bs, err := s.opts.Store.ServerBackups(ctx)
	if err != nil {
		return time.Time{}
	}
	var next time.Time
	earlier := func(t time.Time) {
		if !t.IsZero() && (next.IsZero() || t.Before(next)) {
			next = t
		}
	}
	for _, b := range bs {
		if b.State == store.ServerBackupFailed {
			earlier(b.NextTry)
		}
		for _, c := range b.Copies {
			if c.State == store.CopyFailed {
				earlier(c.NextTry)
			}
		}
	}
	return next
}

// serverBackupLoop runs the schedule and the retries until ctx ends: a
// run missed while the server was down five minutes after the start,
// then the daily time, read again at each wake.
func (s *Server) serverBackupLoop(ctx context.Context) {
	st := s.opts.Store
	if st == nil || st.ReadOnly() {
		return
	}
	clock := s.sbClock()
	var late time.Time // a missed run's
	if start := clock.Now(); s.missedRun(ctx, start) {
		late = start.Add(serverBackupLate)
	}
	for {
		now := clock.Now()
		h, m := s.scheduleTime(ctx)
		due := slotAfter(now, h, m, s.sbLocation())
		if !late.IsZero() {
			due = late
		}
		next := due
		if r := s.nextRetry(ctx); !r.IsZero() && r.Before(next) {
			next = r
		}
		t := clock.AfterFunc(max(next.Sub(now), serverBackupMinWait), s.wakeServerBackups)
		s.sb.armed.Add(1)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-s.sb.wake:
		}
		t.Stop()
		now = clock.Now()
		if !now.Before(due) {
			late = time.Time{}
			s.scheduledBackup(ctx)
		}
		s.retryDue(ctx, now)
	}
}

// scheduledBackup makes the schedule's backup: a failed one of the
// schedule's before it gives way to it; without a key nothing is made.
func (s *Server) scheduledBackup(ctx context.Context) {
	err := s.sbJob(ctx, func() error {
		st := s.opts.Store
		if bs, err := st.ServerBackups(ctx); err == nil {
			for _, b := range bs {
				if b.State == store.ServerBackupFailed && b.Trigger == string(serverbackup.TriggerScheduled) {
					st.DeleteServerBackup(ctx, b.UUID)
				}
			}
		}
		_, outs, err := s.catalogue().Make(ctx, serverbackup.MakeRequest{Trigger: serverbackup.TriggerScheduled})
		s.reportServerBackup(outs, serverbackup.TriggerScheduled, 0)
		return err
	})
	switch {
	case errors.Is(err, serverbackup.ErrNoKey):
		s.log.Debug("server backups: no key; the schedule made nothing")
	case err != nil && ctx.Err() == nil:
		s.log.Debug("server backups: the scheduled backup failed", "error", err)
	}
}

// retryDue retries the failed backups and copies whose retry is due.
func (s *Server) retryDue(ctx context.Context, now time.Time) {
	bs, err := s.opts.Store.ServerBackups(ctx)
	if err != nil {
		return
	}
	for _, b := range bs {
		due := b.State == store.ServerBackupFailed && !b.NextTry.IsZero() && !now.Before(b.NextTry)
		for _, c := range b.Copies {
			due = due || c.State == store.CopyFailed && !c.NextTry.IsZero() && !now.Before(c.NextTry)
		}
		if !due || ctx.Err() != nil {
			continue
		}
		s.sbJob(ctx, func() error {
			_, outs, err := s.catalogue().RetryDue(ctx, b.UUID, now)
			s.reportServerBackup(outs, serverbackup.Trigger(b.Trigger), 0)
			return err
		})
	}
}

// shortID is a backup's uuid as a line names it: its last twelve hex
// digits, which a UUIDv7's random part makes its own.
func shortID(uuid string) string {
	if len(uuid) > 12 {
		return uuid[len(uuid)-12:]
	}
	return uuid
}

// reportServerBackup writes the steps of a job into the log, as the
// server's lines server_backup, by the user by: a backup made, copied,
// pruned, scanned or checked at info, a failure at info while a retry
// comes, else at warn, which is an alert until a later success on its
// target settles it (serverBackupSettles). A failure whose retries
// are spent is told too, to whoever holds backup.retry
// (notify.KindServerBackupFailed).
func (s *Server) reportServerBackup(outs []serverbackup.Outcome, trigger serverbackup.Trigger, by int64) {
	for _, o := range outs {
		sum := map[string]any{"step": o.Step}
		if o.Backup != "" {
			sum["backup"] = o.Backup
			sum["id"] = shortID(o.Backup)
		}
		if o.Target != "" {
			sum["target"] = o.Target
		}
		if o.Step == "make" {
			sum["trigger"] = string(trigger)
		}
		ev := systemEvent("server_backup", by, sum)
		switch {
		case o.Err != nil:
			sum["outcome"], sum["tries"] = "failed", o.Tries
			ev.Error = o.Err.Error()
			if !o.NextTry.IsZero() {
				sum["next_try"] = o.NextTry.UTC().Format(time.RFC3339)
			} else {
				ev.Level = "warn"
			}
		case o.Step == "prune" && o.Removed == 0:
			continue
		default:
			sum["outcome"] = "ok"
			if o.Step == "prune" {
				sum["removed"] = o.Removed
			}
			if o.Step == "scan" {
				sum["added"] = o.Added
			}
		}
		s.logServer(ev)
		if o.Err != nil && o.Spent {
			s.pushNotify(notify.Event{Kind: notify.KindServerBackupFailed, Values: map[string]any{"step": o.Step, "target": o.Target, "backup": o.Backup}})
		}
	}
}

// serverBackupSettles: a later success on the same target, which reaches
// it again, or a backup made, for a backup that failed; a check that
// found a copy changed, a later check on that target.
func serverBackupSettles(alert, later *event) bool {
	if summaryText(later, "outcome") != "ok" || summaryText(later, "target") != summaryText(alert, "target") {
		return false
	}
	switch step := summaryText(alert, "step"); step {
	case "check", "make":
		return summaryText(later, "step") == step
	}
	return true
}

// serverBackupRoutes registers the routes of the server's backups.
func (s *Server) serverBackupRoutes(api func(pattern, capability string, h func(http.ResponseWriter, *http.Request) error)) {
	api("GET /api/v1/server-backups", "backup.retry", s.listServerBackups)
	api("POST /api/v1/server-backups", "backup.server", s.makeServerBackup)
	api("POST /api/v1/server-backups/upload", "backup.server", s.uploadServerBackup)
	api("PUT /api/v1/server-backups/{id}/pin", "backup.server", s.pinServerBackup)
	api("POST /api/v1/server-backups/{id}/check", "backup.server", s.checkServerBackup)
	api("GET /api/v1/server-backups/{id}/file", "backup.server", s.serverBackupFile)
	api("POST /api/v1/server-backups/{id}/retry", "backup.retry", s.retryServerBackup)
	api("DELETE /api/v1/server-backups/{id}", "backup.server", s.deleteServerBackup)
	api("PUT /api/v1/server-backups/{id}/copies/{target}", "backup.server", s.pushServerBackup)
	api("DELETE /api/v1/server-backups/{id}/copies/{target}", "backup.server", s.deleteServerBackupCopy)
	api("POST /api/v1/server-backups/{id}/copies/{target}/pull", "backup.server", s.pullServerBackup)
	api("GET /api/v1/server-backup-targets", "backup.server", s.listServerBackupTargets)
	api("GET /api/v1/server-backup-targets/kinds", "backup.server", s.serverBackupKinds)
	api("POST /api/v1/server-backup-targets", "backup.server", s.addServerBackupTarget)
	api("PUT /api/v1/server-backup-targets/{uuid}", "backup.server", s.putServerBackupTarget)
	api("DELETE /api/v1/server-backup-targets/{uuid}", "backup.server", s.deleteServerBackupTarget)
	api("POST /api/v1/server-backup-targets/{uuid}/test", "backup.server", s.testServerBackupTarget)
	api("GET /api/v1/server-backup-keys", "backup.server", s.listServerBackupKeys)
	// "owner" is capOwner, as a literal for the route tests' reading
	api("POST /api/v1/server-backup-keys", "owner", s.addServerBackupKey)
	api("POST /api/v1/server-backup-keys/{uuid}/confirm", "owner", s.confirmServerBackupKey)
	api("DELETE /api/v1/server-backup-keys/{uuid}", "owner", s.deleteServerBackupKey)
}

// sbFail maps an error of the catalogue to a status.
func sbFail(err error) error {
	var he *httpError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &he):
		return err
	case errors.Is(err, store.ErrNoServerBackup), errors.Is(err, serverbackup.ErrNoTarget), errors.Is(err, store.ErrNoServerBackupTarget),
		errors.Is(err, store.ErrNoServerBackupKey):
		return fail(http.StatusNotFound, "%v", err)
	case errors.Is(err, serverbackup.ErrNoKey), errors.Is(err, serverbackup.ErrNoCopy), errors.Is(err, serverbackup.ErrSameTarget), errors.Is(err, serverbackup.ErrMixedKinds),
		errors.Is(err, serverbackup.ErrNotRetried), errors.Is(err, serverbackup.ErrHasCopies), errors.Is(err, serverbackup.ErrJobRunning),
		errors.Is(err, store.ErrServerBackupTargetExists), errors.Is(err, store.ErrServerBackupKeyExists):
		return fail(http.StatusConflict, "%v", err)
	case errors.Is(err, serverbackup.ErrNotABackup):
		return fail(http.StatusBadRequest, "%v", err)
	case errors.Is(err, serverbackup.ErrNewerFormat), errors.Is(err, serverbackup.ErrBadSignature), errors.Is(err, serverbackup.ErrPayloadChanged),
		errors.Is(err, store.ErrServerBackupKeyConfirm):
		return fail(http.StatusUnprocessableEntity, "%v", err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return fail(http.StatusServiceUnavailable, "%v", err)
	}
	return fail(http.StatusBadGateway, "%v", err)
}

// sbStore is the store, or 503 without one.
func (s *Server) sbStore() (*store.Store, error) {
	st, err := s.cubeStore()
	if err != nil {
		return nil, err
	}
	if st.ReadOnly() {
		return nil, fail(http.StatusServiceUnavailable, "the store is open read-only")
	}
	return st, nil
}

// timeOrNil is a time for an answer, nil for the zero time.
func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

// serverBackupView is a backup as the API shows it.
type serverBackupView struct {
	UUID        string               `json:"uuid"`
	Time        time.Time            `json:"time"`
	Trigger     string               `json:"trigger"`
	By          string               `json:"by,omitempty"`
	State       string               `json:"state"`
	Size        int64                `json:"size"`
	SHA256      string               `json:"sha256,omitempty"`
	Label       string               `json:"label,omitempty"`
	Version     string               `json:"version,omitempty"`
	Node        string               `json:"node,omitempty"`
	ThisBox     bool                 `json:"this_box"`
	Manifest    json.RawMessage      `json:"manifest,omitempty"`
	PinnedUntil *time.Time           `json:"pinned_until"`
	Error       string               `json:"error,omitempty"`
	Tries       int                  `json:"tries"`
	NextTry     *time.Time           `json:"next_try"`
	Copies      []serverBackupCopyAt `json:"copies"`
}

// serverBackupCopyAt is a backup's copy on a target, as the API shows it.
type serverBackupCopyAt struct {
	Target     string     `json:"target"`
	TargetName string     `json:"target_name"`
	Path       string     `json:"path,omitempty"`
	State      string     `json:"state"`
	Pulled     bool       `json:"pulled"`
	PushedAt   *time.Time `json:"pushed_at"`
	CheckedAt  *time.Time `json:"checked_at"`
	Error      string     `json:"error,omitempty"`
	Tries      int        `json:"tries"`
	NextTry    *time.Time `json:"next_try"`
}

// viewServerBackup is a backup as the API shows it; names are the
// targets' names by uuid.
func (s *Server) viewServerBackup(ctx context.Context, b store.ServerBackup, names map[string]string) serverBackupView {
	v := serverBackupView{UUID: b.UUID, Time: b.Time.UTC(), Trigger: b.Trigger, State: b.State, Size: b.Size, SHA256: b.SHA256,
		Manifest: b.Manifest, PinnedUntil: timeOrNil(b.PinnedUntil), Error: b.Error, Tries: b.Tries, NextTry: timeOrNil(b.NextTry),
		Copies: []serverBackupCopyAt{}}
	if b.UserID != 0 {
		if u, ok, err := s.opts.Store.UserByID(ctx, b.UserID); err == nil && ok {
			v.By = u.Name
		}
	}
	if len(b.Manifest) > 0 {
		if m, sig, err := serverbackup.ParseManifest(serverbackup.Raw{Manifest: b.Manifest, Signature: []byte(b.Signature)}); err == nil {
			v.Label, v.Version, v.Node = m.Label, m.Version, m.Node.Fingerprint
			v.ThisBox = sig.Verdict(s.opts.Store.NodePublicKey()) == serverbackup.ThisBox
		}
	}
	for _, c := range b.Copies {
		v.Copies = append(v.Copies, serverBackupCopyAt{Target: c.TargetUUID, TargetName: names[c.TargetUUID], Path: c.Path, State: c.State, Pulled: c.Pulled,
			PushedAt: timeOrNil(c.PushedAt), CheckedAt: timeOrNil(c.CheckedAt), Error: c.Error, Tries: c.Tries, NextTry: timeOrNil(c.NextTry)})
	}
	return v
}

// targetNames are the targets' names by uuid.
func targetNames(ts []store.ServerBackupTarget) map[string]string {
	out := map[string]string{}
	for _, t := range ts {
		out[t.UUID] = t.Name
	}
	return out
}

// targetState is a target as the list of backups shows it: its state,
// without its fields.
type targetState struct {
	UUID        string     `json:"uuid"`
	Name        string     `json:"name"`
	Kind        string     `json:"kind"`
	Takes       string     `json:"takes"`
	Enabled     bool       `json:"enabled"`
	LastSuccess *time.Time `json:"last_success"`
	Behind      bool       `json:"behind"`
}

// targetStates are the targets' states: each one's last copy put there,
// and whether it is two periods behind.
func (s *Server) targetStates(ts []store.ServerBackupTarget, bs []store.ServerBackup, now time.Time) []targetState {
	out := []targetState{}
	for _, t := range ts {
		var last time.Time
		for _, b := range bs {
			if c, ok := b.Copy(t.UUID); ok && c.State == store.CopyThere && c.PushedAt.After(last) {
				last = c.PushedAt
			}
		}
		out = append(out, targetState{UUID: t.UUID, Name: t.Name, Kind: t.Kind, Takes: t.Takes, Enabled: t.Enabled, LastSuccess: timeOrNil(last),
			Behind: t.Enabled && serverbackup.Takes(t.Takes).Behind(last, t.CreatedAt, now)})
	}
	return out
}

// listServerBackups handles GET /api/v1/server-backups: the backups with
// their copies, newest first, each target's state, whether a key is set,
// the daily time and the next run.
func (s *Server) listServerBackups(w http.ResponseWriter, r *http.Request) error {
	st, err := s.cubeStore()
	if err != nil {
		return err
	}
	ctx := r.Context()
	bs, err := st.ServerBackups(ctx)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	ts, err := st.ServerBackupTargets(ctx)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	keys, err := st.ServerBackupKeys(ctx)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	names := targetNames(ts)
	out := []serverBackupView{}
	for _, b := range bs {
		out = append(out, s.viewServerBackup(ctx, b, names))
	}
	confirmed := 0
	for _, k := range keys {
		if !k.Pending {
			confirmed++
		}
	}
	now := s.sbClock().Now()
	h, m := s.scheduleTime(ctx)
	writeJSON(w, http.StatusOK, map[string]any{
		"backups": out, "targets": s.targetStates(ts, bs, now), "keys": confirmed,
		"time": fmt.Sprintf("%02d:%02d", h, m), "next_run": slotAfter(now, h, m, s.sbLocation()).UTC(),
	})
	return nil
}

// outcomeView is a step of a job as an answer names it.
type outcomeView struct {
	Step    string `json:"step"`
	Target  string `json:"target,omitempty"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Removed int    `json:"removed,omitempty"`
	Added   int    `json:"added,omitempty"`
}

func outcomeViews(outs []serverbackup.Outcome) []outcomeView {
	v := []outcomeView{}
	for _, o := range outs {
		ov := outcomeView{Step: o.Step, Target: o.Target, OK: o.Err == nil, Removed: o.Removed, Added: o.Added}
		if o.Err != nil {
			ov.Error = o.Err.Error()
		}
		v = append(v, ov)
	}
	return v
}

// answerBackup writes a backup and the steps that made it.
func (s *Server) answerBackup(w http.ResponseWriter, r *http.Request, status int, b store.ServerBackup, outs []serverbackup.Outcome) error {
	ts, err := s.opts.Store.ServerBackupTargets(r.Context())
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	writeJSON(w, status, map[string]any{"backup": s.viewServerBackup(r.Context(), b, targetNames(ts)), "steps": outcomeViews(outs)})
	return nil
}

// decodeBody reads a JSON body into v; an empty body leaves v as it is.
// A field the route does not know is refused.
func decodeBody(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	return nil
}

// makeServerBackup handles POST /api/v1/server-backups: a backup now,
// {"label", "target"}, the target a uuid or a name; without one, to the
// targets that take it.
func (s *Server) makeServerBackup(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.sbStore(); err != nil {
		return err
	}
	var req struct {
		Label  string `json:"label"`
		Target string `json:"target"`
	}
	if err := decodeBody(r, &req); err != nil {
		return err
	}
	if err := serverbackup.CheckLabel(req.Label); err != nil {
		return fail(http.StatusBadRequest, "%v", err)
	}
	by := principalOf(r).userID()
	var b store.ServerBackup
	var outs []serverbackup.Outcome
	err := s.sbJob(r.Context(), func() error {
		var err error
		b, outs, err = s.catalogue().Make(r.Context(), serverbackup.MakeRequest{Trigger: serverbackup.TriggerManual, UserID: by, Label: req.Label, Target: req.Target})
		s.reportServerBackup(outs, serverbackup.TriggerManual, by)
		return err
	})
	if err != nil {
		return sbFail(err)
	}
	auditTarget(r, "serverbackup:"+b.UUID)
	return s.answerBackup(w, r, http.StatusCreated, b, outs)
}

// uploadServerBackup handles POST /api/v1/server-backups/upload: a
// backup file from elsewhere, the body, to the local target, where it
// stays until it is deleted. Its body is not read ahead for the audit
// (streamedBodies).
func (s *Server) uploadServerBackup(w http.ResponseWriter, r *http.Request) error {
	st, err := s.sbStore()
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(st.Dir(), ".serverbackup-upload-*")
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	defer os.RemoveAll(tmp)
	path := filepath.Join(tmp, "upload.zip")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	_, err = io.Copy(f, http.MaxBytesReader(w, r.Body, maxServerBackupUpload))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		return fail(http.StatusRequestEntityTooLarge, "a server backup of more than %d bytes", int64(maxServerBackupUpload))
	case err != nil:
		return fail(http.StatusBadRequest, "the upload: %v", err)
	}
	by := principalOf(r).userID()
	var b store.ServerBackup
	var o serverbackup.Outcome
	err = s.sbJob(r.Context(), func() error {
		var err error
		b, o, err = s.catalogue().Upload(r.Context(), path, by)
		if o.Step != "" {
			s.reportServerBackup([]serverbackup.Outcome{o}, "", by)
		}
		return err
	})
	if err != nil {
		return sbFail(err)
	}
	auditTarget(r, "serverbackup:"+b.UUID)
	return s.answerBackup(w, r, http.StatusCreated, b, []serverbackup.Outcome{o})
}

// pinServerBackup handles PUT /api/v1/server-backups/{id}/pin:
// {"pinned": true} keeps it from every retention until it is unpinned,
// with "until" until then; {"pinned": false} unpins it.
func (s *Server) pinServerBackup(w http.ResponseWriter, r *http.Request) error {
	st, err := s.sbStore()
	if err != nil {
		return err
	}
	var req struct {
		Pinned *bool      `json:"pinned"`
		Until  *time.Time `json:"until"`
	}
	if err := decodeBody(r, &req); err != nil {
		return err
	}
	if req.Pinned == nil {
		return fail(http.StatusBadRequest, `want {"pinned": true} or {"pinned": false}`)
	}
	until := time.Time{}
	if *req.Pinned {
		until = serverbackup.PinForever
		if req.Until != nil {
			if !req.Until.After(s.sbClock().Now()) {
				return fail(http.StatusBadRequest, "until lies in the past")
			}
			until = *req.Until
		}
	}
	id := r.PathValue("id")
	if b, ok, _ := st.ServerBackup(r.Context(), id); ok {
		auditChange(r, "pinned_until", timeOrNil(b.PinnedUntil), timeOrNil(until))
	}
	if err := st.PinServerBackup(r.Context(), id, until); err != nil {
		return sbFail(err)
	}
	b, _, err := st.ServerBackup(r.Context(), id)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	return s.answerBackup(w, r, http.StatusOK, b, nil)
}

// checkServerBackup handles POST /api/v1/server-backups/{id}/check: its
// copies read back, each whole, for their size and hash, without the
// key; {"target"} the one on that target alone.
func (s *Server) checkServerBackup(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.sbStore(); err != nil {
		return err
	}
	var req struct {
		Target string `json:"target"`
	}
	if err := decodeBody(r, &req); err != nil {
		return err
	}
	by := principalOf(r).userID()
	var b store.ServerBackup
	var outs []serverbackup.Outcome
	err := s.sbJob(r.Context(), func() error {
		var err error
		b, outs, err = s.catalogue().Check(r.Context(), r.PathValue("id"), req.Target)
		s.reportServerBackup(outs, "", by)
		return err
	})
	if err != nil {
		return sbFail(err)
	}
	return s.answerBackup(w, r, http.StatusOK, b, outs)
}

// serverBackupFile handles GET /api/v1/server-backups/{id}/file: the
// file, from a target that holds it, the local ones first, by ranges.
func (s *Server) serverBackupFile(w http.ResponseWriter, r *http.Request) error {
	st, err := s.cubeStore()
	if err != nil {
		return err
	}
	b, ok, err := st.ServerBackup(r.Context(), r.PathValue("id"))
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return fail(http.StatusNotFound, "no server backup %q", r.PathValue("id"))
	}
	f, size, name, err := s.catalogue().OpenCopy(r.Context(), b.UUID)
	if err != nil {
		return sbFail(err)
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeContent(w, r, name, b.Time, io.NewSectionReader(f, 0, size))
	return nil
}

// retryServerBackup handles POST /api/v1/server-backups/{id}/retry: a
// failed backup made again as the schedule made it, or a backup's failed
// copies made again, each target pruned after as the schedule would
// (D13-A).
func (s *Server) retryServerBackup(w http.ResponseWriter, r *http.Request) error {
	st, err := s.sbStore()
	if err != nil {
		return err
	}
	id := r.PathValue("id")
	b, ok, err := st.ServerBackup(r.Context(), id)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return fail(http.StatusNotFound, "no server backup %q", id)
	}
	if b.State != store.ServerBackupFailed && !slices.ContainsFunc(b.Copies, func(c store.ServerBackupCopy) bool { return c.State == store.CopyFailed }) {
		return fail(http.StatusConflict, "the backup %s has nothing that failed to retry", id)
	}
	by := principalOf(r).userID()
	var outs []serverbackup.Outcome
	err = s.sbJob(r.Context(), func() error {
		var err error
		b, outs, err = s.catalogue().Retry(r.Context(), id)
		s.reportServerBackup(outs, serverbackup.Trigger(b.Trigger), by)
		return err
	})
	if err != nil {
		return sbFail(err)
	}
	s.wakeServerBackups()
	return s.answerBackup(w, r, http.StatusOK, b, outs)
}

// deleteServerBackup handles DELETE /api/v1/server-backups/{id}: a
// backup of which no target holds a copy goes from the catalogue, one
// that failed, or whose copies a stop broke off; one with a copy there
// is refused, its copies deleted first.
func (s *Server) deleteServerBackup(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.sbStore(); err != nil {
		return err
	}
	err := s.sbJob(r.Context(), func() error {
		return s.catalogue().Delete(r.Context(), r.PathValue("id"))
	})
	if err != nil {
		return sbFail(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// pushServerBackup handles PUT /api/v1/server-backups/{id}/copies/{target}:
// a copy put on the target, a uuid or a name, from one that is there.
func (s *Server) pushServerBackup(w http.ResponseWriter, r *http.Request) error {
	return s.copyServerBackup(w, r, false)
}

// pullServerBackup handles POST …/copies/{target}/pull: the copy on the
// target fetched to the local target, where it stays until it is deleted.
func (s *Server) pullServerBackup(w http.ResponseWriter, r *http.Request) error {
	return s.copyServerBackup(w, r, true)
}

// copyServerBackup is a push or a pull.
func (s *Server) copyServerBackup(w http.ResponseWriter, r *http.Request, pull bool) error {
	if _, err := s.sbStore(); err != nil {
		return err
	}
	by := principalOf(r).userID()
	var b store.ServerBackup
	var o serverbackup.Outcome
	err := s.sbJob(r.Context(), func() error {
		var err error
		c := s.catalogue()
		if pull {
			b, o, err = c.Pull(r.Context(), r.PathValue("id"), r.PathValue("target"))
		} else {
			b, o, err = c.Push(r.Context(), r.PathValue("id"), r.PathValue("target"))
		}
		if o.Step != "" {
			s.reportServerBackup([]serverbackup.Outcome{o}, "", by)
		}
		return err
	})
	if err != nil {
		return sbFail(err)
	}
	return s.answerBackup(w, r, http.StatusOK, b, []serverbackup.Outcome{o})
}

// deleteServerBackupCopy handles DELETE …/copies/{target}: the copy's
// file removed from the target, and the backup from the catalogue with
// its last copy.
func (s *Server) deleteServerBackupCopy(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.sbStore(); err != nil {
		return err
	}
	err := s.sbJob(r.Context(), func() error {
		return s.catalogue().DeleteCopy(r.Context(), r.PathValue("id"), r.PathValue("target"))
	})
	if err != nil {
		return sbFail(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// targetView is a target as the API shows it: its fields, never its
// secret, whether it has one; of a secret the server made, its public
// part (an SFTP target's key line, for authorized_keys).
type targetView struct {
	targetState
	Config     map[string]string   `json:"config"`
	HasSecret  bool                `json:"has_secret"`
	SecretLost bool                `json:"secret_lost,omitempty"`
	PublicKey  string              `json:"public_key,omitempty"`
	Retention  serverbackup.Policy `json:"retention"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// viewTarget is a target as the API shows it.
func viewTarget(t store.ServerBackupTarget, state targetState) targetView {
	var p serverbackup.Policy
	json.Unmarshal(t.Retention, &p)
	v := targetView{targetState: state, Config: t.Config, HasSecret: t.Secret != "" || t.SecretLost, SecretLost: t.SecretLost,
		Retention: p, CreatedAt: t.CreatedAt.UTC(), UpdatedAt: t.UpdatedAt.UTC()}
	if k, ok := serverbackup.KindOf(t.Kind); ok && k.Public != nil && t.Secret != "" {
		v.PublicKey, _ = k.Public(t.Secret)
	}
	return v
}

// listServerBackupTargets handles GET /api/v1/server-backup-targets.
func (s *Server) listServerBackupTargets(w http.ResponseWriter, r *http.Request) error {
	st, err := s.cubeStore()
	if err != nil {
		return err
	}
	ts, err := st.ServerBackupTargets(r.Context())
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	bs, err := st.ServerBackups(r.Context())
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	states := s.targetStates(ts, bs, s.sbClock().Now())
	out := []targetView{}
	for i, t := range ts {
		out = append(out, viewTarget(t, states[i]))
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// kindView is a kind of target as GET …/kinds shows it.
type kindView struct {
	Name   string      `json:"name"`
	Secret string      `json:"secret,omitempty"`
	Fields []fieldView `json:"fields"`
}

// fieldView is one field of a kind's form; its text is
// serverbackup.field.<kind>.<key> in the web UI's languages, and that
// of its warning, for a value Warn matches, serverbackup.warn.<kind>.<key>.
type fieldView struct {
	Key      string   `json:"key"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Default  string   `json:"default,omitempty"`
	Choices  []string `json:"choices,omitempty"`
	Warn     string   `json:"warn,omitempty"`
}

// serverBackupKinds handles GET /api/v1/server-backup-targets/kinds: the
// kinds of target this server knows and their fields, which the card's
// form is drawn from.
func (s *Server) serverBackupKinds(w http.ResponseWriter, r *http.Request) error {
	out := []kindView{}
	for _, k := range serverbackup.Kinds() {
		kv := kindView{Name: k.Name, Fields: []fieldView{}}
		if f, ok := k.Secret(); ok {
			kv.Secret = f.Key
		}
		for _, f := range k.Fields {
			kv.Fields = append(kv.Fields, fieldView{Key: f.Key, Type: f.Type, Required: f.Required, Default: f.Default, Choices: f.Choices, Warn: f.Warn})
		}
		out = append(out, kv)
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// targetRequest is the body of a target added or changed: a field left
// out stays as it is (a new target's defaults).
type targetRequest struct {
	Name      *string              `json:"name"`
	Kind      string               `json:"kind"`
	Config    map[string]string    `json:"config"`
	Secret    *string              `json:"secret"`
	Takes     *string              `json:"takes"`
	Retention *serverbackup.Policy `json:"retention"`
	Enabled   *bool                `json:"enabled"`
}

// apply checks the request and writes it into t, a target of kind k.
func (req targetRequest) apply(k serverbackup.Kind, t *store.ServerBackupTarget) error {
	if req.Name != nil {
		t.Name = strings.TrimSpace(*req.Name)
	}
	if t.Name == "" || len(t.Name) > 64 || strings.ContainsFunc(t.Name, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return fail(http.StatusBadRequest, "a target needs a name of at most 64 characters, without control characters")
	}
	if req.Secret != nil {
		_, field := k.Secret()
		switch {
		case k.MakeSecret != nil && *req.Secret != "":
			return fail(http.StatusBadRequest, `a target of kind %s takes no secret: the server makes its key, and "secret": "" a new one`, k.Name)
		case !field && *req.Secret != "":
			return fail(http.StatusBadRequest, "a target of kind %s takes no secret", k.Name)
		}
		t.Secret, t.SecretLost = *req.Secret, false
	}
	if k.MakeSecret != nil && t.Secret == "" && !t.SecretLost {
		secret, err := k.MakeSecret()
		if err != nil {
			return fail(http.StatusInternalServerError, "%v", err)
		}
		t.Secret = secret
	}
	if req.Config != nil {
		t.Config = req.Config
	}
	cfg, err := k.Check(t.Config, t.Secret != "" || t.SecretLost)
	if err != nil {
		return fail(http.StatusBadRequest, "%v", err)
	}
	t.Config = cfg
	if req.Takes != nil {
		if !serverbackup.Takes(*req.Takes).Valid() {
			return fail(http.StatusBadRequest, "takes %q: want every, daily, weekly or monthly", *req.Takes)
		}
		t.Takes = *req.Takes
	}
	if req.Retention != nil {
		if err := req.Retention.Check(); err != nil {
			return fail(http.StatusBadRequest, "%v", err)
		}
		t.Retention, _ = json.Marshal(*req.Retention)
	}
	if req.Enabled != nil {
		t.Enabled = *req.Enabled
	}
	return nil
}

// answerTarget writes a target with its state.
func (s *Server) answerTarget(w http.ResponseWriter, r *http.Request, status int, t store.ServerBackupTarget, extra map[string]any) error {
	bs, err := s.opts.Store.ServerBackups(r.Context())
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	out := map[string]any{"target": viewTarget(t, s.targetStates([]store.ServerBackupTarget{t}, bs, s.sbClock().Now())[0])}
	for k, v := range extra {
		out[k] = v
	}
	writeJSON(w, status, out)
	return nil
}

// addServerBackupTarget handles POST /api/v1/server-backup-targets: a
// target of a kind with its fields, its secret, what it takes and its
// retention (serverbackup.DefaultPolicy where left out); it is scanned
// at once for the backups it holds.
func (s *Server) addServerBackupTarget(w http.ResponseWriter, r *http.Request) error {
	st, err := s.sbStore()
	if err != nil {
		return err
	}
	var req targetRequest
	if err := decodeBody(r, &req); err != nil {
		return err
	}
	k, ok := serverbackup.KindOf(req.Kind)
	if !ok {
		var names []string
		for _, k := range serverbackup.Kinds() {
			names = append(names, k.Name)
		}
		return fail(http.StatusBadRequest, "kind %q: want one of %s", req.Kind, strings.Join(names, ", "))
	}
	def, _ := json.Marshal(serverbackup.DefaultPolicy)
	t := store.ServerBackupTarget{Kind: k.Name, Takes: string(serverbackup.TakesEvery), Retention: def, Enabled: true}
	if err := req.apply(k, &t); err != nil {
		return err
	}
	t, err = st.AddServerBackupTarget(r.Context(), t, principalOf(r).userID())
	if err != nil {
		return sbFail(err)
	}
	auditTarget(r, "serverbackup-target:"+t.UUID)
	extra := map[string]any{}
	s.sbJob(r.Context(), func() error {
		added, skipped, err := s.catalogue().ScanTarget(r.Context(), t)
		s.reportServerBackup([]serverbackup.Outcome{{Step: "scan", Target: t.Name, TargetUUID: t.UUID, Added: added, Err: err}}, "", principalOf(r).userID())
		extra["found"] = added
		if err != nil {
			extra["scan_error"] = err.Error()
			hostKeyAnswer(err, extra)
		}
		var names []string
		for _, sk := range skipped {
			names = append(names, sk.Name)
		}
		if len(names) > 0 {
			extra["skipped"] = names
		}
		return err
	})
	s.wakeServerBackups()
	return s.answerTarget(w, r, http.StatusCreated, t, extra)
}

// putServerBackupTarget handles PUT …/server-backup-targets/{uuid}: the
// fields given changed, the others kept; "secret" replaces the secret,
// "" removes it, left out keeps it. Of a kind that makes its secret (an
// SFTP target's key), "" makes a new one, and no other is taken.
func (s *Server) putServerBackupTarget(w http.ResponseWriter, r *http.Request) error {
	st, err := s.sbStore()
	if err != nil {
		return err
	}
	var req targetRequest
	if err := decodeBody(r, &req); err != nil {
		return err
	}
	cur, ok, err := st.ServerBackupTarget(r.Context(), r.PathValue("uuid"))
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return fail(http.StatusNotFound, "no server backup target %q", r.PathValue("uuid"))
	}
	if req.Kind != "" && req.Kind != cur.Kind {
		return fail(http.StatusBadRequest, "a target's kind does not change; add a target of kind %s", req.Kind)
	}
	k, ok := serverbackup.KindOf(cur.Kind)
	if !ok {
		return fail(http.StatusConflict, "the target's kind %q is unknown to this Massimo", cur.Kind)
	}
	t, err := st.UpdateServerBackupTarget(r.Context(), cur.UUID, func(t *store.ServerBackupTarget) error { return req.apply(k, t) }, principalOf(r).userID())
	if err != nil {
		return sbFail(err)
	}
	auditChange(r, "target", cur.Name, t.Name)
	s.wakeServerBackups()
	return s.answerTarget(w, r, http.StatusOK, t, nil)
}

// deleteServerBackupTarget handles DELETE …/server-backup-targets/{uuid}:
// the target and its copies go from the catalogue; its files stay.
func (s *Server) deleteServerBackupTarget(w http.ResponseWriter, r *http.Request) error {
	st, err := s.sbStore()
	if err != nil {
		return err
	}
	if err := st.DeleteServerBackupTarget(r.Context(), r.PathValue("uuid")); err != nil {
		return sbFail(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// hostKeyAnswer adds to an answer the host key an SFTP target's error
// names: host_key, the host's, to confirm as the target's host_key, and
// pinned_host_key, the one confirmed, where the host's changed.
func hostKeyAnswer(err error, out map[string]any) {
	var hk *serverbackup.HostKeyError
	if !errors.As(err, &hk) {
		return
	}
	out["host_key"] = hk.Fingerprint
	if hk.Pinned != "" {
		out["pinned_host_key"] = hk.Pinned
	}
}

// testServerBackupTarget handles POST …/server-backup-targets/{uuid}/test:
// the target reached and written to, its outcome the answer's "ok" and
// "error", and an SFTP host's key to confirm (hostKeyAnswer).
func (s *Server) testServerBackupTarget(w http.ResponseWriter, r *http.Request) error {
	st, err := s.cubeStore()
	if err != nil {
		return err
	}
	t, ok, err := st.ServerBackupTarget(r.Context(), r.PathValue("uuid"))
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return fail(http.StatusNotFound, "no server backup target %q", r.PathValue("uuid"))
	}
	tgt, err := s.catalogue().Open(t)
	if err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
		err = tgt.Check(ctx)
		cancel()
	}
	if err != nil {
		out := map[string]any{"ok": false, "error": err.Error()}
		hostKeyAnswer(err, out)
		writeJSON(w, http.StatusOK, out)
		return nil
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// keyView is a key as the API shows it.
type keyView struct {
	UUID        string     `json:"uuid"`
	Label       string     `json:"label"`
	Recipient   string     `json:"recipient"`
	Fingerprint string     `json:"fingerprint"`
	Kind        string     `json:"kind"`
	Pending     bool       `json:"pending"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
}

func viewKey(k store.ServerBackupKey) keyView {
	v := keyView{UUID: k.UUID, Label: k.Label, Recipient: k.Recipient, Fingerprint: k.Fingerprint, Kind: k.Kind, Pending: k.Pending, CreatedAt: k.CreatedAt.UTC()}
	if k.Pending {
		v.ExpiresAt = timeOrNil(k.CreatedAt.Add(store.PendingKeyLifetime))
	}
	return v
}

// listServerBackupKeys handles GET /api/v1/server-backup-keys: the keys,
// the pending ones with their end.
func (s *Server) listServerBackupKeys(w http.ResponseWriter, r *http.Request) error {
	st, err := s.cubeStore()
	if err != nil {
		return err
	}
	keys, err := st.ServerBackupKeys(r.Context())
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	out := []keyView{}
	for _, k := range keys {
		out = append(out, viewKey(k))
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// confirmHash is the hash a key made keeps of its identity's last six
// characters, bound to its fingerprint; the six are typed back to make it
// a key.
func confirmHash(fingerprint, lastSix string) string {
	sum := sha256.Sum256([]byte("massimo server backup key\n" + fingerprint + "\n" + strings.ToUpper(strings.TrimSpace(lastSix))))
	return hex.EncodeToString(sum[:])
}

// addServerBackupKey handles POST /api/v1/server-backup-keys, the owner's
// alone (D4-A, D17-C): {"label", "recipient"} keeps a recipient of the
// owner's own; {"label", "make": "pq"|"classic"} makes a key, answers its
// identity once, as text and as a key file, and keeps it pending until
// its last six characters are typed back (…/confirm), an hour at most.
// A key of another kind than the box's keys is refused, with why.
func (s *Server) addServerBackupKey(w http.ResponseWriter, r *http.Request) error {
	st, err := s.sbStore()
	if err != nil {
		return err
	}
	var req struct {
		Label     string `json:"label"`
		Recipient string `json:"recipient"`
		Make      string `json:"make"`
	}
	if err := decodeBody(r, &req); err != nil {
		return err
	}
	req.Label = strings.TrimSpace(req.Label)
	if err := serverbackup.CheckLabel(req.Label); err != nil {
		return fail(http.StatusBadRequest, "%v", err)
	}
	var rec serverbackup.Recipient
	var made *serverbackup.Key
	switch {
	case req.Recipient != "" && req.Make != "":
		return fail(http.StatusBadRequest, "give a recipient, or make a key, not both")
	case req.Recipient != "":
		if rec, err = serverbackup.ParseRecipient(req.Recipient); err != nil {
			return fail(http.StatusBadRequest, "%v", err)
		}
	case req.Make == "pq" || req.Make == "classic":
		kind := serverbackup.KindPostQuantum
		if req.Make == "classic" {
			kind = serverbackup.KindX25519
		}
		k, err := serverbackup.GenerateKey(kind)
		if err != nil {
			return fail(http.StatusInternalServerError, "%v", err)
		}
		made, rec = &k, k.Recipient
	default:
		return fail(http.StatusBadRequest, `give "recipient", a public key, or "make": "pq" or "classic"`)
	}
	rec.Label = req.Label
	s.sb.keyMu.Lock()
	defer s.sb.keyMu.Unlock()
	keys, err := st.ServerBackupKeys(r.Context())
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	all := []serverbackup.Recipient{rec}
	for _, k := range keys {
		if o, err := serverbackup.ParseRecipient(k.Recipient); err == nil {
			all = append(all, o)
		}
	}
	if err := serverbackup.CheckKinds(all); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "mixed_kinds"})
		return nil
	}
	hash := ""
	if made != nil {
		hash = confirmHash(rec.Fingerprint, made.Identity[len(made.Identity)-6:])
	}
	k, err := st.AddServerBackupKey(r.Context(), store.ServerBackupKey{Label: rec.Label, Recipient: rec.Line, Fingerprint: rec.Fingerprint, Kind: string(rec.Kind)},
		hash, principalOf(r).userID())
	if err != nil {
		return sbFail(err)
	}
	auditTarget(r, "serverbackup-key:"+k.UUID)
	out := map[string]any{"key": viewKey(k)}
	if made != nil {
		// shown once: secretFields keeps both out of the command's row
		out["identity"], out["key_file"] = made.Identity, string(made.File(s.sbClock().Now()))
	}
	s.wakeServerBackups()
	writeJSON(w, http.StatusCreated, out)
	return nil
}

// confirmServerBackupKey handles POST …/server-backup-keys/{uuid}/confirm,
// {"last_six"}: the identity's last six characters typed back make a key
// made a key.
func (s *Server) confirmServerBackupKey(w http.ResponseWriter, r *http.Request) error {
	st, err := s.sbStore()
	if err != nil {
		return err
	}
	var req struct {
		LastSix string `json:"last_six"`
	}
	if err := decodeBody(r, &req); err != nil {
		return err
	}
	if len(strings.TrimSpace(req.LastSix)) != 6 {
		return fail(http.StatusBadRequest, "give the key's last six characters as last_six")
	}
	keys, err := st.ServerBackupKeys(r.Context())
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	fp := ""
	for _, k := range keys {
		if k.UUID == r.PathValue("uuid") {
			fp = k.Fingerprint
		}
	}
	if fp == "" {
		return fail(http.StatusNotFound, "no server backup key %q, or its hour is over", r.PathValue("uuid"))
	}
	k, err := st.ConfirmServerBackupKey(r.Context(), r.PathValue("uuid"), confirmHash(fp, req.LastSix))
	if err != nil {
		return sbFail(err)
	}
	s.wakeServerBackups()
	writeJSON(w, http.StatusOK, map[string]any{"key": viewKey(k)})
	return nil
}

// deleteServerBackupKey handles DELETE …/server-backup-keys/{uuid}: later
// backups are not encrypted for it; those made before still are.
func (s *Server) deleteServerBackupKey(w http.ResponseWriter, r *http.Request) error {
	st, err := s.sbStore()
	if err != nil {
		return err
	}
	if err := st.DeleteServerBackupKey(r.Context(), r.PathValue("uuid")); err != nil {
		return sbFail(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// userHolds reports whether a user holds a capability, or one that
// includes it, as a check of the API would: for a notification only its
// holders get (notifyNeeds).
func (s *Server) userHolds(ctx context.Context, userID int64, capability string) bool {
	st := s.opts.Store
	if st == nil {
		return false
	}
	u, ok, err := st.UserByID(ctx, userID)
	if err != nil || !ok || u.Disabled || u.IntegrationID != 0 {
		return false
	}
	caps, off, roleOff, err := st.Effective(ctx, u.Role, capabilities)
	if err != nil || roleOff {
		return false
	}
	set := setOf(caps)
	return set[capability] || included(set, setOf(off), capability)
}

// notifyNeeds are the notifications only a holder of a capability gets.
var notifyNeeds = map[string]string{notify.KindServerBackupFailed: "backup.retry"}
