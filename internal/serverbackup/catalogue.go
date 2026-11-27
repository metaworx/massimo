// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The catalogue's jobs (AP ServerBackup v1.0, Block 2): a backup made and
// copied to the targets that take it, each target pruned by its
// retention; a copy pushed or pulled; a target scanned for the backups
// it holds; a copy checked by its size and hash; a file uploaded. They
// work on the store's catalogue (internal/store, serverbackups.go), so
// that serve and massimo store run them alike. The schedule, the retries
// and the alerts are serve's (internal/server).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"go.metaworx.ch/massimo/internal/store"
)

// RetryDelays are the waits before the retries of a scheduled backup or
// of a copy that failed: after the last retry fails, the failure is told
// (a notification, a warning in the log).
var RetryDelays = []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour}

// PinBefore is how long a backup before an update or a restore is
// pinned (D12-A); a pin by hand lasts until it is lifted (PinForever).
const PinBefore = 30 * 24 * time.Hour

// PinForever is the time a pin by hand lasts until.
var PinForever = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)

// DefaultLocalName is the name of the local target made where a backup
// finds no target at all.
const DefaultLocalName = "local"

// The errors of the catalogue's jobs.
var (
	// ErrNoKey is a backup asked for while no key is set: the
	// schedule makes nothing until the owner gives one.
	ErrNoKey = errors.New("serverbackup: no key is set; backups are made once the owner has given one")
	// ErrNoCopy is a backup of which no target holds a copy to read.
	ErrNoCopy = errors.New("serverbackup: no target holds a copy of this backup")
	// ErrNoTarget is a target the catalogue does not hold, or one
	// switched off.
	ErrNoTarget = errors.New("serverbackup: no such target")
	// ErrNotRetried is a retry of a backup made by hand that failed,
	// which is made again by hand, with its own request.
	ErrNotRetried = errors.New("serverbackup: a backup made by hand that failed is made again by hand, not retried")
	// ErrHasCopies is a backup deleted from the catalogue while a target
	// holds a copy; its copies are deleted first.
	ErrHasCopies = errors.New("serverbackup: a target holds a copy of this backup; delete its copies")
	// ErrSameTarget is a pull from the local target to itself.
	ErrSameTarget = errors.New("serverbackup: a copy is pulled from another target than the local one")
)

// Catalogue runs the jobs on a store's catalogue.
type Catalogue struct {
	// Store is the store, opened with store.Open; its directory, the
	// data directory, holds the temporary files and the relative paths
	// of the targets.
	Store *store.Store
	// EnvFile and TLSFiles are what a backup carries beside the store
	// (Inputs); Version is Massimo's.
	EnvFile  string
	TLSFiles []TLSFile
	Version  string
	// Now is the clock, nil time.Now; Location the box's time zone,
	// whose days, weeks and months the targets take and keep by, nil
	// time.Local.
	Now      func() time.Time
	Location *time.Location
	Log      *slog.Logger

	// renameStaged renames a restore's staged file into place, nil
	// safefile.Rename; for the tests.
	renameStaged func(from, to string, wait time.Duration) error
}

// dataDir is the store's directory, absolute: a target's relative path
// is relative to it, and a target takes only an absolute one. The store
// is opened in "." where serve runs in its data directory without -data.
func (c *Catalogue) dataDir() (string, error) {
	dir, err := filepath.Abs(c.Store.Dir())
	if err != nil {
		return "", fmt.Errorf("serverbackup: the data directory %q: %w", c.Store.Dir(), err)
	}
	return dir, nil
}

func (c *Catalogue) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Catalogue) loc() *time.Location {
	if c.Location != nil {
		return c.Location
	}
	return time.Local
}

func (c *Catalogue) log() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.Default()
}

// Outcome is what one step of a job did, for the log and the alerts.
type Outcome struct {
	// Step is make, copy, prune, scan, check, pull or upload.
	Step string
	// Backup is the backup's uuid; Target the target's name and
	// TargetUUID its uuid, "" for the backup's own step.
	Backup     string
	Target     string
	TargetUUID string
	Err        error
	// Tries and NextTry are a failure's: its tries, and when it is tried
	// again, zero for not at all; Spent says its retries are spent and
	// the failure is to be told.
	Tries   int
	NextTry time.Time
	Spent   bool
	// Removed is what a prune removed, Added what a scan added.
	Removed, Added int
}

// MakeRequest asks for a backup.
type MakeRequest struct {
	Trigger Trigger
	// UserID is who asks, 0 for the schedule or the server.
	UserID int64
	Label  string
	// Target, a target's uuid or name, copies the backup there alone,
	// whatever it takes; Local copies it to the local targets alone.
	// Neither copies it to every target that takes it (Takes), or, where
	// none does, to the local ones.
	Target string
	Local  bool
	// PinFor pins it for that long; 0 not.
	PinFor time.Duration
	// UUID is a failed backup's, retried: the same row made again.
	UUID string
}

// Recipients are the keys the backups are encrypted for: the store's
// keys, not the pending ones, parsed and labelled.
func (c *Catalogue) Recipients(ctx context.Context) ([]Recipient, error) {
	keys, err := c.Store.ServerBackupKeys(ctx)
	if err != nil {
		return nil, err
	}
	var out []Recipient
	for _, k := range keys {
		if k.Pending {
			continue
		}
		r, err := ParseRecipient(k.Recipient)
		if err != nil {
			return nil, fmt.Errorf("serverbackup: the key %s: %w", k.Fingerprint, err)
		}
		r.Label = k.Label
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, ErrNoKey
	}
	return out, nil
}

// Open makes the target of a row: its kind's New over its fields, a
// path relative to the data directory made absolute, its secret opened.
// A kind this Massimo does not know, and a secret that does not open,
// are errors.
func (c *Catalogue) Open(t store.ServerBackupTarget) (Target, error) {
	k, ok := KindOf(t.Kind)
	if !ok {
		return nil, fmt.Errorf("serverbackup: target %s: a kind %q this Massimo does not know", t.Name, t.Kind)
	}
	if t.SecretLost {
		return nil, fmt.Errorf("serverbackup: target %s: its secret does not open (opaque.key changed); enter it again", t.Name)
	}
	cfg, err := k.Check(t.Config, t.Secret != "")
	if err != nil {
		return nil, fmt.Errorf("serverbackup: target %s: %w", t.Name, err)
	}
	dir := ""
	for _, f := range k.Fields {
		if v := cfg[f.Key]; f.Type == FieldPath && v != "" && !filepath.IsAbs(v) {
			if dir == "" {
				if dir, err = c.dataDir(); err != nil {
					return nil, fmt.Errorf("serverbackup: target %s: %w", t.Name, err)
				}
			}
			cfg[f.Key] = filepath.Join(dir, v)
		}
	}
	var secret []byte
	if t.Secret != "" {
		secret = []byte(t.Secret)
	}
	tgt, err := k.New(cfg, secret)
	if err != nil {
		return nil, fmt.Errorf("serverbackup: target %s: %w", t.Name, err)
	}
	return tgt, nil
}

// FindTarget returns the target of a uuid or a name, the name without
// regard to case.
func (c *Catalogue) FindTarget(ctx context.Context, ref string) (store.ServerBackupTarget, error) {
	ts, err := c.Store.ServerBackupTargets(ctx)
	if err != nil {
		return store.ServerBackupTarget{}, err
	}
	for _, t := range ts {
		if t.UUID == ref || strings.EqualFold(t.Name, ref) {
			return t, nil
		}
	}
	return store.ServerBackupTarget{}, fmt.Errorf("%w: %q", ErrNoTarget, ref)
}

// localTargets are the local targets switched on; where the catalogue has
// no target at all, the default local one is made first.
func (c *Catalogue) localTargets(ctx context.Context) ([]store.ServerBackupTarget, error) {
	ts, err := c.Store.ServerBackupTargets(ctx)
	if err != nil {
		return nil, err
	}
	if len(ts) == 0 {
		t, err := c.addDefaultLocal(ctx)
		if err != nil {
			return nil, err
		}
		ts = []store.ServerBackupTarget{t}
	}
	var out []store.ServerBackupTarget
	for _, t := range ts {
		if t.Enabled && t.Kind == KindLocal {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no local target is switched on", ErrNoTarget)
	}
	return out, nil
}

// addDefaultLocal makes the local target of a catalogue that has none:
// backups/ in the data directory, every backup, the default retention.
func (c *Catalogue) addDefaultLocal(ctx context.Context) (store.ServerBackupTarget, error) {
	ret, _ := json.Marshal(DefaultPolicy)
	t, err := c.Store.AddServerBackupTarget(ctx, store.ServerBackupTarget{Name: DefaultLocalName, Kind: KindLocal,
		Config: map[string]string{"path": DefaultLocalDir}, Takes: string(TakesEvery), Retention: ret, Enabled: true}, 0)
	if err != nil {
		return store.ServerBackupTarget{}, err
	}
	c.log().Info("server backups: the local target made", "path", filepath.Join(c.Store.Dir(), DefaultLocalDir))
	return t, nil
}

// retryAt is when a failure of its tries-th try is tried again, zero for
// never: a backup made by hand, or of the schedule once its retries are
// spent. spent is true the once its last retry failed, so that the
// failure is told once.
func (c *Catalogue) retryAt(tries int, scheduled bool) (next time.Time, spent bool) {
	switch {
	case !scheduled:
		return time.Time{}, false
	case tries >= 1 && tries <= len(RetryDelays):
		return c.now().Add(RetryDelays[tries-1]), false
	}
	return time.Time{}, tries == len(RetryDelays)+1
}

// scheduled reports whether a backup of a trigger is the schedule's,
// whose failures are retried by themselves.
func scheduled(trigger string) bool { return trigger == string(TriggerScheduled) }

// notCopied is the error of a backup whose file is made and not yet on
// a target: the row a stop or a crash in the middle of its copies leaves.
const notCopied = "not on a target yet: its copies were broken off"

// Make makes a backup and copies it as req asks (MakeRequest), each
// target that took a copy pruned by its retention after. The backup is
// a failed row until its first copy is there, so that a stop in the
// middle of its copies leaves a backup to retry, not one made without a
// file; the catalogue's writes after the backup is made outlive ctx. A
// scheduled backup that fails, before its file or with no copy there,
// is kept as failed with its retry, its tries counted on from the row a
// retry names; one made by hand that fails is not kept, the error
// saying why. The backup, as the catalogue holds it then, and what each
// step did are returned, its own step first; the error is the backup's
// own, a copy's failing being an outcome.
func (c *Catalogue) Make(ctx context.Context, req MakeRequest) (store.ServerBackup, []Outcome, error) {
	keep := context.WithoutCancel(ctx)
	now := c.now()
	prevTries := 0
	if req.UUID != "" {
		if prev, ok, _ := c.Store.ServerBackup(ctx, req.UUID); ok {
			prevTries = prev.Tries
		}
	}
	// fail records a failure before any copy: a retry's always, a
	// scheduled run's but for no key, which makes nothing at all.
	fail := func(err error) (store.ServerBackup, []Outcome, error) {
		if req.UUID == "" && (!scheduled(string(req.Trigger)) || errors.Is(err, ErrNoKey)) {
			return store.ServerBackup{}, nil, err
		}
		b, o := c.failed(keep, req, now, prevTries, err)
		return b, []Outcome{o}, err
	}
	rs, err := c.Recipients(ctx)
	if err != nil {
		return fail(err)
	}
	targets, err := c.chosen(ctx, req)
	if err != nil {
		return fail(err)
	}
	tmp, err := os.MkdirTemp(c.Store.Dir(), tempPrefix+"made-*")
	if err != nil {
		return fail(fmt.Errorf("serverbackup: %w", err))
	}
	defer os.RemoveAll(tmp)
	out := filepath.Join(tmp, "backup.zip")
	m, err := Write(ctx, Inputs{Store: c.Store, EnvFile: c.EnvFile, TLSFiles: c.TLSFiles, Trigger: req.Trigger, Label: req.Label,
		Version: c.Version, Time: now, UUID: req.UUID}, rs, out)
	if err != nil {
		return fail(err)
	}
	pending := store.ServerBackup{Trigger: string(req.Trigger), UserID: req.UserID, State: store.ServerBackupFailed, Error: notCopied, Tries: prevTries}
	pending.NextTry, _ = c.retryAt(prevTries+1, scheduled(string(req.Trigger)))
	b, err := c.keepFile(keep, out, pending, req.PinFor)
	if err != nil {
		if scheduled(string(req.Trigger)) {
			req.UUID = m.UUID
		}
		return fail(err)
	}
	auto := scheduled(b.Trigger)
	var outs []Outcome
	have := c.copyTimes(ctx)
	took := 0
	for _, t := range targets {
		if req.Target == "" && !req.Local && !Takes(t.Takes).Takes(m.Created.In(c.loc()), have[t.UUID]) {
			continue
		}
		took++
		outs = append(outs, c.putFile(ctx, b, out, t, false, auto))
	}
	if took == 0 {
		locals, err := c.localTargets(keep)
		if err != nil {
			outs = append(outs, Outcome{Step: "copy", Backup: b.UUID, Err: err})
		}
		for _, t := range locals {
			outs = append(outs, c.putFile(ctx, b, out, t, false, auto))
		}
	}
	var cause error = ErrNoCopy
	for _, o := range outs {
		if o.Err == nil {
			cause = nil
		} else if cause == ErrNoCopy {
			cause = o.Err
		}
	}
	if cause != nil {
		// No target holds the file, which goes with the temporary
		// directory: its copies are no copies, and the backup failed.
		cause = fmt.Errorf("serverbackup: no target took the backup: %w", cause)
		c.dropCopies(keep, b)
		if !auto {
			c.Store.DeleteServerBackup(keep, b.UUID)
			return store.ServerBackup{}, []Outcome{{Step: "make", Backup: b.UUID, Err: cause, Tries: 1}}, cause
		}
		req.UUID = b.UUID
		b, o := c.failed(keep, req, now, prevTries, cause)
		return b, []Outcome{o}, cause
	}
	b.State, b.Error, b.Tries, b.NextTry = store.ServerBackupMade, "", 0, time.Time{}
	if _, err := c.Store.SaveServerBackup(keep, b); err != nil {
		return b, outs, err
	}
	steps := []Outcome{{Step: "make", Backup: b.UUID}}
	for _, o := range outs {
		steps = append(steps, o)
		if o.Err == nil && o.Step == "copy" {
			steps = append(steps, c.pruneOutcome(keep, o.TargetUUID))
		}
	}
	b, _, err = c.Store.ServerBackup(keep, b.UUID)
	return b, steps, err
}

// dropCopies removes the copy rows of a backup whose file no target
// holds.
func (c *Catalogue) dropCopies(ctx context.Context, b store.ServerBackup) {
	cur, ok, err := c.Store.ServerBackup(ctx, b.UUID)
	if err != nil || !ok {
		return
	}
	for _, cp := range cur.Copies {
		if _, err := c.Store.DeleteServerBackupCopy(ctx, b.UUID, cp.TargetUUID); err != nil {
			c.log().Error("server backups: a copy that is none not removed", "error", err)
		}
	}
}

// chosen are the targets a request copies to: the one it names, the
// local ones, or every one switched on.
func (c *Catalogue) chosen(ctx context.Context, req MakeRequest) ([]store.ServerBackupTarget, error) {
	switch {
	case req.Target != "":
		t, err := c.FindTarget(ctx, req.Target)
		if err != nil {
			return nil, err
		}
		if !t.Enabled {
			return nil, fmt.Errorf("%w: %s is switched off", ErrNoTarget, t.Name)
		}
		return []store.ServerBackupTarget{t}, nil
	case req.Local:
		return c.localTargets(ctx)
	}
	ts, err := c.Store.ServerBackupTargets(ctx)
	if err != nil {
		return nil, err
	}
	var out []store.ServerBackupTarget
	for _, t := range ts {
		if t.Enabled {
			out = append(out, t)
		}
	}
	return out, nil
}

// failed keeps a backup that failed: as failed, its tries one more than
// prevTries, a scheduled one with its retry.
func (c *Catalogue) failed(ctx context.Context, req MakeRequest, now time.Time, prevTries int, cause error) (store.ServerBackup, Outcome) {
	id := req.UUID
	if id == "" {
		id = newBackupUUID()
	}
	tries := prevTries + 1
	o := Outcome{Step: "make", Backup: id, Err: cause, Tries: tries}
	o.NextTry, o.Spent = c.retryAt(tries, scheduled(string(req.Trigger)))
	b, err := c.Store.SaveServerBackup(ctx, store.ServerBackup{UUID: id, Time: now, Trigger: string(req.Trigger), UserID: req.UserID,
		State: store.ServerBackupFailed, Error: cause.Error(), Tries: tries, NextTry: o.NextTry})
	if err != nil {
		c.log().Error("server backups: a failed backup not kept", "error", err)
	}
	return b, o
}

// keepFile reads the file at path, a backup, and keeps it in the
// catalogue as b says, with its size, hash and manifest; b's state, its
// error, tries and next try are kept as given.
func (c *Catalogue) keepFile(ctx context.Context, path string, b store.ServerBackup, pinFor time.Duration) (store.ServerBackup, error) {
	f, err := os.Open(path)
	if err != nil {
		return store.ServerBackup{}, fmt.Errorf("serverbackup: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return store.ServerBackup{}, fmt.Errorf("serverbackup: %w", err)
	}
	m, _, raw, err := ReadManifest(f, fi.Size())
	if err != nil {
		return store.ServerBackup{}, err
	}
	sum, err := fileHash(f, fi.Size())
	if err != nil {
		return store.ServerBackup{}, fmt.Errorf("serverbackup: %w", err)
	}
	b.UUID, b.Time = m.UUID, m.Created
	if b.State == "" {
		b.State = store.ServerBackupMade
	}
	b.Size, b.SHA256, b.Manifest, b.Signature = fi.Size(), sum, json.RawMessage(raw.Manifest), string(raw.Signature)
	if pinFor > 0 {
		b.PinnedUntil = c.now().Add(pinFor)
	}
	if prev, ok, _ := c.Store.ServerBackup(ctx, b.UUID); ok && prev.PinnedUntil.After(b.PinnedUntil) {
		b.PinnedUntil = prev.PinnedUntil
	}
	return c.Store.SaveServerBackup(ctx, b)
}

// fileHash is the hex SHA-256 of r's size bytes.
func fileHash(r io.ReaderAt, size int64) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, io.NewSectionReader(r, 0, size)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyTimes are the times of the backups each target holds a copy of,
// by the target's uuid, in the box's location.
func (c *Catalogue) copyTimes(ctx context.Context) map[string][]time.Time {
	out := map[string][]time.Time{}
	bs, err := c.Store.ServerBackups(ctx)
	if err != nil {
		return out
	}
	for _, b := range bs {
		for _, cp := range b.Copies {
			if cp.State == store.CopyThere {
				out[cp.TargetUUID] = append(out[cp.TargetUUID], b.Time.In(c.loc()))
			}
		}
	}
	return out
}

// putFile copies the backup's file at path to a target and keeps the
// copy: there, or failed, with its retry where auto, the schedule's.
func (c *Catalogue) putFile(ctx context.Context, b store.ServerBackup, path string, t store.ServerBackupTarget, pulled, auto bool) Outcome {
	f, err := os.Open(path)
	if err != nil {
		return c.copyFailed(ctx, b, t, fmt.Errorf("serverbackup: %w", err), auto)
	}
	defer f.Close()
	return c.put(ctx, b, f, b.Size, t, pulled, auto)
}

// put copies r, the backup's size bytes, to a target under its file name
// and keeps the copy; the catalogue's write outlives ctx.
func (c *Catalogue) put(ctx context.Context, b store.ServerBackup, r io.Reader, size int64, t store.ServerBackupTarget, pulled, auto bool) Outcome {
	m, _, err := ParseManifest(Raw{Manifest: b.Manifest, Signature: []byte(b.Signature)})
	if err != nil {
		return c.copyFailed(ctx, b, t, err, auto)
	}
	name := FileName(m)
	tgt, err := c.Open(t)
	if err == nil {
		err = tgt.Put(ctx, name, r, size)
	}
	if err != nil {
		return c.copyFailed(ctx, b, t, err, auto)
	}
	prev, _ := b.Copy(t.UUID)
	cp := store.ServerBackupCopy{TargetUUID: t.UUID, Path: name, State: store.CopyThere, Pulled: pulled || prev.Pulled, PushedAt: c.now()}
	if err := c.Store.SaveServerBackupCopy(context.WithoutCancel(ctx), b.UUID, cp); err != nil {
		return Outcome{Step: "copy", Backup: b.UUID, Target: t.Name, TargetUUID: t.UUID, Err: err}
	}
	return Outcome{Step: "copy", Backup: b.UUID, Target: t.Name, TargetUUID: t.UUID}
}

// copyFailed keeps a copy that failed, its tries counted on from the
// copy's row, with its retry where auto, the schedule's; a copy that is
// there already stays there. The catalogue's write outlives ctx.
func (c *Catalogue) copyFailed(ctx context.Context, b store.ServerBackup, t store.ServerBackupTarget, cause error, auto bool) Outcome {
	keep := context.WithoutCancel(ctx)
	prev, had := b.Copy(t.UUID)
	if cur, ok, _ := c.Store.ServerBackup(keep, b.UUID); ok {
		prev, had = cur.Copy(t.UUID)
	}
	tries := 1
	if had && prev.State == store.CopyFailed {
		tries = prev.Tries + 1
	}
	o := Outcome{Step: "copy", Backup: b.UUID, Target: t.Name, TargetUUID: t.UUID, Err: cause, Tries: tries}
	if had && prev.State == store.CopyThere {
		// a copy that is there stays there; the push that failed is told
		return o
	}
	o.NextTry, o.Spent = c.retryAt(tries, auto)
	cp := store.ServerBackupCopy{TargetUUID: t.UUID, Path: prev.Path, State: store.CopyFailed, Pulled: prev.Pulled, PushedAt: prev.PushedAt,
		CheckedAt: prev.CheckedAt, Error: cause.Error(), Tries: tries, NextTry: o.NextTry}
	if err := c.Store.SaveServerBackupCopy(keep, b.UUID, cp); err != nil {
		c.log().Error("server backups: a failed copy not kept", "error", err)
	}
	return o
}

// Retry tries again, by hand, what of a backup failed: a scheduled
// backup that failed is made again in its row, as the schedule made it,
// its tries counted on; a backup's failed copies are put again from a
// copy that is there, each target that took one pruned after, as the
// schedule would. A backup made by hand that failed is not made again
// here (ErrNotRetried): its request, a target, a pin, is the one who
// made it's.
func (c *Catalogue) Retry(ctx context.Context, uuid string) (store.ServerBackup, []Outcome, error) {
	return c.retry(ctx, uuid, time.Time{})
}

// RetryDue is Retry for the schedule at now: what of a backup failed
// and is due, its retry's time past; a failure whose retries are spent
// waits for a retry by hand.
func (c *Catalogue) RetryDue(ctx context.Context, uuid string, now time.Time) (store.ServerBackup, []Outcome, error) {
	return c.retry(ctx, uuid, now)
}

// retry is Retry, of what is due at now alone where now is not zero.
func (c *Catalogue) retry(ctx context.Context, uuid string, now time.Time) (store.ServerBackup, []Outcome, error) {
	due := func(next time.Time) bool { return now.IsZero() || !next.IsZero() && !now.Before(next) }
	b, ok, err := c.Store.ServerBackup(ctx, uuid)
	if err != nil {
		return store.ServerBackup{}, nil, err
	}
	if !ok {
		return store.ServerBackup{}, nil, fmt.Errorf("%w: %s", store.ErrNoServerBackup, uuid)
	}
	if b.State == store.ServerBackupFailed {
		switch {
		case !scheduled(b.Trigger):
			return b, nil, fmt.Errorf("%w: %s", ErrNotRetried, uuid)
		case !due(b.NextTry):
			return b, nil, nil
		}
		return c.Make(ctx, MakeRequest{Trigger: TriggerScheduled, UUID: b.UUID})
	}
	var outs []Outcome
	for _, cp := range b.Copies {
		if cp.State != store.CopyFailed || !due(cp.NextTry) {
			continue
		}
		t, ok, err := c.Store.ServerBackupTarget(ctx, cp.TargetUUID)
		if err != nil || !ok {
			continue
		}
		o := c.copyFrom(ctx, b, t, cp.Pulled, scheduled(b.Trigger))
		outs = append(outs, o)
		if o.Err == nil {
			outs = append(outs, c.pruneOutcome(context.WithoutCancel(ctx), t.UUID))
		}
	}
	b, _, err = c.Store.ServerBackup(context.WithoutCancel(ctx), uuid)
	return b, outs, err
}

// copyFrom copies a backup to a target from a copy that is there on
// another one, the local ones first.
func (c *Catalogue) copyFrom(ctx context.Context, b store.ServerBackup, t store.ServerBackupTarget, pulled, auto bool) Outcome {
	f, size, _, err := c.openCopy(ctx, b, t.UUID)
	if err != nil {
		return c.copyFailed(ctx, b, t, err, auto)
	}
	defer f.Close()
	return c.put(ctx, b, io.NewSectionReader(f, 0, size), size, t, pulled, auto)
}

// Delete removes from the catalogue a backup of which no target holds a
// copy: one that failed, or whose copies were broken off. One with a
// copy there is refused (ErrHasCopies); its copies are deleted first
// (DeleteCopy).
func (c *Catalogue) Delete(ctx context.Context, uuid string) error {
	b, ok, err := c.Store.ServerBackup(ctx, uuid)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s", store.ErrNoServerBackup, uuid)
	}
	if anyThere(b) {
		return fmt.Errorf("%w: %s", ErrHasCopies, uuid)
	}
	return c.Store.DeleteServerBackup(ctx, uuid)
}

// Push copies a backup to a target, from a copy that is there. A push by
// hand is pruned at the target's next prune, as any copy.
func (c *Catalogue) Push(ctx context.Context, uuid, target string) (store.ServerBackup, Outcome, error) {
	return c.copyTo(ctx, uuid, target, false)
}

// Pull copies a backup from the target named to the local target, where
// it stays until it is deleted, whatever the retention (D12-A).
func (c *Catalogue) Pull(ctx context.Context, uuid, from string) (store.ServerBackup, Outcome, error) {
	src, err := c.FindTarget(ctx, from)
	if err != nil {
		return store.ServerBackup{}, Outcome{}, err
	}
	locals, err := c.localTargets(ctx)
	if err != nil {
		return store.ServerBackup{}, Outcome{}, err
	}
	b, ok, err := c.Store.ServerBackup(ctx, uuid)
	if err != nil {
		return store.ServerBackup{}, Outcome{}, err
	}
	if !ok {
		return store.ServerBackup{}, Outcome{}, fmt.Errorf("%w: %s", store.ErrNoServerBackup, uuid)
	}
	cp, ok := b.Copy(src.UUID)
	if !ok || cp.State != store.CopyThere {
		return b, Outcome{}, fmt.Errorf("%w on %s", ErrNoCopy, src.Name)
	}
	locals = slices.DeleteFunc(locals, func(t store.ServerBackupTarget) bool { return t.UUID == src.UUID })
	if len(locals) == 0 {
		return b, Outcome{}, fmt.Errorf("%w: %s is the local target", ErrSameTarget, src.Name)
	}
	tgt, err := c.Open(src)
	if err != nil {
		return b, Outcome{}, err
	}
	f, size, err := tgt.Open(ctx, cp.Path)
	if err != nil {
		return b, Outcome{}, err
	}
	defer f.Close()
	o := c.put(ctx, b, io.NewSectionReader(f, 0, size), size, locals[0], true, false)
	o.Step = "pull"
	b, _, _ = c.Store.ServerBackup(ctx, uuid)
	return b, o, o.Err
}

// copyTo copies a backup to the target of a uuid or name.
func (c *Catalogue) copyTo(ctx context.Context, uuid, target string, pulled bool) (store.ServerBackup, Outcome, error) {
	t, err := c.FindTarget(ctx, target)
	if err != nil {
		return store.ServerBackup{}, Outcome{}, err
	}
	b, ok, err := c.Store.ServerBackup(ctx, uuid)
	if err != nil {
		return store.ServerBackup{}, Outcome{}, err
	}
	if !ok || b.State != store.ServerBackupMade {
		return store.ServerBackup{}, Outcome{}, fmt.Errorf("%w: %s", store.ErrNoServerBackup, uuid)
	}
	o := c.copyFrom(ctx, b, t, pulled, false)
	b, _, _ = c.Store.ServerBackup(ctx, uuid)
	return b, o, o.Err
}

// OpenCopy opens a copy of a backup that is there, the local targets'
// first, for a download: the file, its size and its name.
func (c *Catalogue) OpenCopy(ctx context.Context, uuid string) (File, int64, string, error) {
	b, ok, err := c.Store.ServerBackup(ctx, uuid)
	if err != nil {
		return nil, 0, "", err
	}
	if !ok {
		return nil, 0, "", fmt.Errorf("%w: %s", store.ErrNoServerBackup, uuid)
	}
	return c.openCopy(ctx, b, "")
}

// openCopy opens a copy of b that is there, on a target other than not,
// the local ones first.
func (c *Catalogue) openCopy(ctx context.Context, b store.ServerBackup, not string) (File, int64, string, error) {
	ts, err := c.Store.ServerBackupTargets(ctx)
	if err != nil {
		return nil, 0, "", err
	}
	byUUID := map[string]store.ServerBackupTarget{}
	for _, t := range ts {
		byUUID[t.UUID] = t
	}
	var last error = ErrNoCopy
	for _, localFirst := range []bool{true, false} {
		for _, cp := range b.Copies {
			t, ok := byUUID[cp.TargetUUID]
			if !ok || cp.State != store.CopyThere || cp.TargetUUID == not || (t.Kind == KindLocal) != localFirst {
				continue
			}
			tgt, err := c.Open(t)
			if err != nil {
				last = err
				continue
			}
			f, size, err := tgt.Open(ctx, cp.Path)
			if err != nil {
				last = err
				continue
			}
			return f, size, cp.Path, nil
		}
	}
	return nil, 0, "", last
}

// DeleteCopy removes a backup's copy on a target, the file and its row;
// the backup goes from the catalogue with its last copy.
func (c *Catalogue) DeleteCopy(ctx context.Context, uuid, target string) error {
	t, err := c.FindTarget(ctx, target)
	if err != nil {
		return err
	}
	b, ok, err := c.Store.ServerBackup(ctx, uuid)
	if err != nil {
		return err
	}
	cp, has := b.Copy(t.UUID)
	if !ok || !has {
		return fmt.Errorf("%w: %s on %s", store.ErrNoServerBackup, uuid, t.Name)
	}
	if cp.Path != "" {
		tgt, err := c.Open(t)
		if err != nil {
			return err
		}
		if err := tgt.Delete(ctx, cp.Path); err != nil {
			return err
		}
	}
	_, err = c.Store.DeleteServerBackupCopy(ctx, uuid, t.UUID)
	return err
}

// pruneOutcome prunes the target of a uuid, as an outcome.
func (c *Catalogue) pruneOutcome(ctx context.Context, targetUUID string) Outcome {
	t, ok, err := c.Store.ServerBackupTarget(ctx, targetUUID)
	if err != nil || !ok {
		return Outcome{Step: "prune", TargetUUID: targetUUID, Err: err}
	}
	n, err := c.Prune(ctx, t)
	return Outcome{Step: "prune", Target: t.Name, TargetUUID: t.UUID, Removed: n, Err: err}
}

// Prune removes the copies on a target its retention keeps no more
// (Keep), the files and their rows; a backup left without a copy that is
// there goes from the catalogue. It returns how many it removed.
func (c *Catalogue) Prune(ctx context.Context, t store.ServerBackupTarget) (int, error) {
	var p Policy
	if len(t.Retention) > 0 {
		if err := json.Unmarshal(t.Retention, &p); err != nil {
			return 0, fmt.Errorf("serverbackup: target %s: its retention: %w", t.Name, err)
		}
	}
	bs, err := c.Store.ServerBackups(ctx)
	if err != nil {
		return 0, err
	}
	var cs []Candidate
	paths := map[string]string{}
	for _, b := range bs {
		cp, ok := b.Copy(t.UUID)
		if !ok || cp.State != store.CopyThere {
			continue
		}
		cs = append(cs, Candidate{UUID: b.UUID, Time: b.Time.In(c.loc()), PinnedUntil: b.PinnedUntil, Pulled: cp.Pulled})
		paths[b.UUID] = cp.Path
	}
	keep := Keep(p, cs, c.now())
	var tgt Target
	removed := 0
	for _, cand := range cs {
		if keep[cand.UUID] {
			continue
		}
		if tgt == nil {
			if tgt, err = c.Open(t); err != nil {
				return removed, err
			}
		}
		err := tgt.Delete(ctx, paths[cand.UUID])
		if errors.Is(err, ErrRetained) {
			// The target keeps it (its lock): the copy stays there,
			// and the next prune tries again.
			c.log().Warn("server backups: a copy the target keeps", "target", t.Name, "backup", cand.UUID, "error", err)
			continue
		}
		if err != nil {
			return removed, err
		}
		if _, err := c.Store.DeleteServerBackupCopy(ctx, cand.UUID, t.UUID); err != nil {
			return removed, err
		}
		removed++
		if b, ok, _ := c.Store.ServerBackup(ctx, cand.UUID); ok && !anyThere(b) {
			if err := c.Store.DeleteServerBackup(ctx, b.UUID); err != nil {
				return removed, err
			}
		}
	}
	return removed, nil
}

// anyThere reports whether a backup has a copy that is there.
func anyThere(b store.ServerBackup) bool {
	for _, cp := range b.Copies {
		if cp.State == store.CopyThere {
			return true
		}
	}
	return false
}

// ScanTarget finds the backups a target holds (Scan) and keeps them in
// the catalogue: a backup it does not know as made, by nobody, its hash
// unknown until a check; a copy there. A copy the catalogue has there
// that the target does not hold any more is missing. It returns how many
// backups it added, and the files it skipped.
func (c *Catalogue) ScanTarget(ctx context.Context, t store.ServerBackupTarget) (int, []Skipped, error) {
	tgt, err := c.Open(t)
	if err != nil {
		return 0, nil, err
	}
	found, skipped, err := Scan(ctx, tgt)
	if err != nil {
		return 0, skipped, err
	}
	added := 0
	seen := map[string]bool{}
	for _, f := range found {
		seen[f.Manifest.UUID] = true
		b, ok, err := c.Store.ServerBackup(ctx, f.Manifest.UUID)
		if err != nil {
			return added, skipped, err
		}
		if !ok || b.State != store.ServerBackupMade {
			b, err = c.Store.SaveServerBackup(ctx, store.ServerBackup{UUID: f.Manifest.UUID, Time: f.Manifest.Created, Trigger: string(f.Manifest.Trigger),
				Size: f.Entry.Size, Manifest: json.RawMessage(f.Raw.Manifest), Signature: string(f.Raw.Signature)})
			if err != nil {
				return added, skipped, err
			}
			added++
		}
		if cp, ok := b.Copy(t.UUID); ok && cp.State == store.CopyThere && cp.Path == f.Entry.Name {
			continue
		}
		prev, _ := b.Copy(t.UUID)
		if err := c.Store.SaveServerBackupCopy(ctx, b.UUID, store.ServerBackupCopy{TargetUUID: t.UUID, Path: f.Entry.Name, State: store.CopyThere,
			Pulled: prev.Pulled, PushedAt: f.Entry.Time, CheckedAt: prev.CheckedAt}); err != nil {
			return added, skipped, err
		}
	}
	bs, err := c.Store.ServerBackups(ctx)
	if err != nil {
		return added, skipped, err
	}
	for _, b := range bs {
		if cp, ok := b.Copy(t.UUID); ok && cp.State == store.CopyThere && !seen[b.UUID] {
			cp.State, cp.Error = store.CopyMissing, "not on the target at its scan"
			if err := c.Store.SaveServerBackupCopy(ctx, b.UUID, cp); err != nil {
				return added, skipped, err
			}
		}
	}
	return added, skipped, nil
}

// Check reads a backup's copies back, each whole, on the target named or
// on every one: its size, the file's hash and the payload's against the
// signed manifest (Verify), without the key. A copy that differs is
// failed, one that is not there missing; a copy that checks is there
// again. A backup found by a scan gets its file's hash from its first
// check.
func (c *Catalogue) Check(ctx context.Context, uuid, target string) (store.ServerBackup, []Outcome, error) {
	b, ok, err := c.Store.ServerBackup(ctx, uuid)
	if err != nil {
		return store.ServerBackup{}, nil, err
	}
	if !ok || b.State != store.ServerBackupMade {
		return store.ServerBackup{}, nil, fmt.Errorf("%w: %s", store.ErrNoServerBackup, uuid)
	}
	only := ""
	if target != "" {
		t, err := c.FindTarget(ctx, target)
		if err != nil {
			return b, nil, err
		}
		only = t.UUID
	}
	var outs []Outcome
	for _, cp := range b.Copies {
		if cp.Path == "" || only != "" && cp.TargetUUID != only {
			continue
		}
		t, ok, err := c.Store.ServerBackupTarget(ctx, cp.TargetUUID)
		if err != nil || !ok {
			continue
		}
		sum, err := c.checkCopy(ctx, b, t, cp)
		o := Outcome{Step: "check", Backup: b.UUID, Target: t.Name, TargetUUID: t.UUID, Err: err}
		outs = append(outs, o)
		cp.CheckedAt = c.now()
		switch {
		case err == nil:
			cp.State, cp.Error, cp.Tries, cp.NextTry = store.CopyThere, "", 0, time.Time{}
			if b.SHA256 == "" {
				b.SHA256 = sum
				if _, err := c.Store.SaveServerBackup(ctx, b); err != nil {
					return b, outs, err
				}
			}
		case errors.Is(err, os.ErrNotExist):
			cp.State, cp.Error = store.CopyMissing, err.Error()
		default:
			cp.State, cp.Error = store.CopyFailed, err.Error()
		}
		if err := c.Store.SaveServerBackupCopy(ctx, b.UUID, cp); err != nil {
			return b, outs, err
		}
	}
	b, _, err = c.Store.ServerBackup(ctx, uuid)
	return b, outs, err
}

// checkCopy reads one copy back whole: its size, its payload against
// its signed manifest, and its hash, which it returns.
func (c *Catalogue) checkCopy(ctx context.Context, b store.ServerBackup, t store.ServerBackupTarget, cp store.ServerBackupCopy) (string, error) {
	tgt, err := c.Open(t)
	if err != nil {
		return "", err
	}
	f, size, err := tgt.Open(ctx, cp.Path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if size != b.Size {
		return "", fmt.Errorf("serverbackup: %s on %s has %d bytes, not %d", cp.Path, t.Name, size, b.Size)
	}
	m, _, err := Verify(f, size)
	if err != nil {
		return "", err
	}
	if m.UUID != b.UUID {
		return "", fmt.Errorf("serverbackup: %s on %s is the backup %s", cp.Path, t.Name, m.UUID)
	}
	sum, err := fileHash(f, size)
	if err != nil {
		return "", err
	}
	if b.SHA256 != "" && sum != b.SHA256 {
		return "", fmt.Errorf("serverbackup: %s on %s is not the file it was", cp.Path, t.Name)
	}
	return sum, nil
}

// Upload takes a backup file from elsewhere, at path: refused where it is
// no backup, of a newer format, or changed since it was made (its
// signature, its payload), else copied to the local target, where it
// stays until it is deleted, and kept in the catalogue as uploaded by
// the user, its manifest keeping the trigger it was made with. A backup
// the catalogue holds already is copied to local where it is not there.
func (c *Catalogue) Upload(ctx context.Context, path string, by int64) (store.ServerBackup, Outcome, error) {
	f, err := os.Open(path)
	if err != nil {
		return store.ServerBackup{}, Outcome{}, fmt.Errorf("serverbackup: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return store.ServerBackup{}, Outcome{}, fmt.Errorf("serverbackup: %w", err)
	}
	if _, _, err := Verify(f, fi.Size()); err != nil {
		return store.ServerBackup{}, Outcome{}, err
	}
	locals, err := c.localTargets(ctx)
	if err != nil {
		return store.ServerBackup{}, Outcome{}, err
	}
	m, _, _, err := ReadManifest(f, fi.Size())
	if err != nil {
		return store.ServerBackup{}, Outcome{}, err
	}
	b, ok, err := c.Store.ServerBackup(ctx, m.UUID)
	if err != nil {
		return store.ServerBackup{}, Outcome{}, err
	}
	keep := context.WithoutCancel(ctx)
	fresh := !ok || b.State != store.ServerBackupMade
	if fresh {
		// a failed row until its copy is there, as Make keeps one
		if b, err = c.keepFile(keep, path, store.ServerBackup{Trigger: string(TriggerUploaded), UserID: by, State: store.ServerBackupFailed, Error: notCopied}, 0); err != nil {
			return store.ServerBackup{}, Outcome{}, err
		}
	}
	if cp, ok := b.Copy(locals[0].UUID); ok && cp.State == store.CopyThere {
		return b, Outcome{Step: "upload", Backup: b.UUID, Target: locals[0].Name, TargetUUID: locals[0].UUID}, nil
	}
	o := c.putFile(ctx, b, path, locals[0], true, false)
	o.Step = "upload"
	switch {
	case o.Err != nil && fresh:
		c.dropCopies(keep, b)
		c.Store.DeleteServerBackup(keep, b.UUID)
		return store.ServerBackup{}, o, o.Err
	case fresh:
		b.State, b.Error = store.ServerBackupMade, ""
		if _, err := c.Store.SaveServerBackup(keep, b); err != nil {
			return b, o, err
		}
	}
	b, _, _ = c.Store.ServerBackup(keep, b.UUID)
	return b, o, o.Err
}

// newBackupUUID is a new backup's uuid, a UUIDv7.
func newBackupUUID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString()
	}
	return id.String()
}
