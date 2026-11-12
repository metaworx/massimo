// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The backup history (docs/store.md section 9 (e), AP StoreBackups):
// the server takes a Cube's backup by itself after a change of its
// configuration. The configuration is the decoded part of a backup
// without the live state (backup.ConfigHash); its hash is compared with
// the newest stored backup's, or after a restore with the file
// restored. A Cube whose hash differs is dirty from the first change;
// its backup is taken once no change has come for backups.auto_min
// minutes, and at the latest backups.auto_max minutes after it became
// dirty. A Cube without a backup gets one at once, also once its last
// is deleted or pruned. The comparison runs once a minute after a
// connection or a configuration event; the reference lives in memory,
// read from the newest backup at the first look and again once the
// newest is deleted or pruned.

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/store"
)

// backupPruneEvery is how often the backups are pruned besides after
// each new one and a change of a retention.
const backupPruneEvery = 24 * time.Hour

// backupRetry is how many minutes a failed automatic backup waits
// before the next try.
const backupRetry = 10

// backupWatch is a Cube's configuration against its newest backup.
type backupWatch struct {
	touched atomic.Bool // a connection or a configuration event since the last look

	// rows makes a backup's row and the clean state one step for the
	// list: takeBackup holds it to store the row and call stored(),
	// listBackups shares it to read the rows and state(). deleteBackup
	// holds it to find whether the row is the newest, delete it and
	// forget(), pruneBackups to forget() after a prune that may have
	// removed it; the first look shares it to read the reference, so that
	// it never keeps the hash of a row deleted meanwhile. Taken before
	// mu, never while mu is held.
	rows sync.RWMutex

	mu         sync.Mutex
	known      bool      // ref is read: the newest backup's hash, or none
	none       bool      // the Cube has no backup
	ref        string    // the hash of the newest backup, or of the file restored
	cur        string    // the hash at the last look
	dirtySince time.Time // the first change against ref; zero when the Cube is clean
	lastChange time.Time // the last change seen
	retryAt    time.Time // after a failed automatic backup
}

// touch marks that the configuration may have changed.
func (w *backupWatch) touch() { w.touched.Store(true) }

// stored makes a backup just stored, of configuration hash, the
// reference: the Cube is clean.
func (w *backupWatch) stored(hash string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.known, w.none, w.ref, w.cur = true, false, hash, hash
	w.dirtySince, w.lastChange, w.retryAt = time.Time{}, time.Time{}, time.Time{}
}

// forget drops the reference once the newest backup is gone: the next
// look reads it again from the backup now newest, or finds none, and
// compares the configuration with it afresh, which needs the hash of
// the last look cleared as well. The caller holds rows.
func (w *backupWatch) forget() {
	w.mu.Lock()
	w.known, w.none, w.ref = false, false, ""
	w.cur, w.dirtySince, w.lastChange = "", time.Time{}, time.Time{}
	w.mu.Unlock()
	w.touch()
}

// restored makes the file a restore sent the reference; the next look
// says whether the Cube took all of it.
func (w *backupWatch) restored(hash string) {
	w.mu.Lock()
	w.known, w.none, w.ref = true, false, hash
	w.cur, w.dirtySince, w.lastChange = "", time.Time{}, time.Time{}
	w.mu.Unlock()
	w.touch()
}

// state reports the watch for the API: when the configuration last
// changed against the newest backup, and when its backup is due; zero
// times when the Cube is clean.
func (w *backupWatch) state(set store.BackupSettings, minute time.Duration) (changed, due time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.dirtySince.IsZero() {
		return time.Time{}, time.Time{}
	}
	due = w.lastChange.Add(time.Duration(set.AutoMin) * minute)
	if latest := w.dirtySince.Add(time.Duration(set.AutoMax) * minute); latest.Before(due) {
		due = latest
	}
	return w.lastChange, due
}

// backupMinute is the unit of the automatic backup's windows.
func (s *Server) backupMinute() time.Duration {
	if s.opts.BackupMinute > 0 {
		return s.opts.BackupMinute
	}
	return time.Minute
}

// errNoHistory answers a backup request for a Cube without a row in the
// store, whose backups have nowhere to go.
var errNoHistory = fail(http.StatusServiceUnavailable, "this Cube has no row in the store, so no backup history")

// backupLoop looks at the Cube's configuration once a minute and takes
// its automatic backup when it is due.
func (s *Server) backupLoop(ctx context.Context, rt *runtime) {
	t := time.NewTicker(s.backupMinute())
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		s.backupCheck(ctx, rt, time.Now())
	}
}

// backupCheck compares the Cube's configuration with its reference and
// takes the automatic backup when it is due.
func (s *Server) backupCheck(ctx context.Context, rt *runtime, now time.Time) {
	st, cubeID := rt.store, rt.storeID.Load()
	if st == nil || cubeID == 0 || st.ReadOnly() || !rt.client.Connected() || rt.restoring() != nil {
		return
	}
	w := &rt.bk
	w.mu.Lock()
	known := w.known
	w.mu.Unlock()
	if !known {
		w.rows.RLock() // against a delete and its forget() in between
		ref, none, err := s.newestBackupHash(ctx, st, cubeID)
		w.mu.Lock()
		if !w.known {
			w.known, w.none, w.ref = true, none, ref
		}
		w.mu.Unlock()
		w.rows.RUnlock()
		if err != nil {
			s.log.Warn("backup: the newest backup is not readable; the next is taken as after a change", "cube", rt.serial, "error", err)
		}
		w.touch()
	}
	if w.touched.Swap(false) {
		f, err := backup.Take(rt.client.Snapshot(), nil, "", time.Time{}, rt.hidden())
		if err != nil {
			w.touch() // no greeting yet: look again
			return
		}
		h := backup.ConfigHash(f)
		w.mu.Lock()
		if h != w.cur {
			w.cur = h
			if h == w.ref {
				w.dirtySince, w.lastChange = time.Time{}, time.Time{}
			} else {
				w.lastChange = now
				if w.dirtySince.IsZero() {
					w.dirtySince = now
				}
			}
		}
		w.mu.Unlock()
	}
	set, err := st.BackupSettings(ctx)
	if err != nil {
		return
	}
	minute := s.backupMinute()
	w.mu.Lock()
	due := now.After(w.retryAt) && (w.none || !w.dirtySince.IsZero() &&
		(!now.Before(w.lastChange.Add(time.Duration(set.AutoMin)*minute)) || !now.Before(w.dirtySince.Add(time.Duration(set.AutoMax)*minute))))
	w.mu.Unlock()
	if !due {
		return
	}
	if _, _, err := s.takeBackup(ctx, rt, store.ReasonAuto, 0, ""); err != nil {
		w.mu.Lock()
		w.retryAt = now.Add(backupRetry * minute)
		w.mu.Unlock()
		if ctx.Err() == nil {
			failed := systemEvent("backup", 0, map[string]any{"reason": store.ReasonAuto, "retry_in": (backupRetry * minute).String()})
			failed.Level, failed.Error = "warn", err.Error()
			rt.emit(failed)
		}
	}
}

// newestBackupHash reads the configuration hash of a Cube's newest
// backup; none when it has none.
func (s *Server) newestBackupHash(ctx context.Context, st *store.Store, cubeID int64) (hash string, none bool, err error) {
	b, ok, err := st.NewestBackup(ctx, cubeID)
	if err != nil || !ok {
		return "", !ok && err == nil, err
	}
	body, ok, err := st.BackupBody(ctx, cubeID, b.ID)
	if err != nil || !ok {
		return "", false, errors.Join(err, errors.New("the body is gone"))
	}
	f, err := backup.Unmarshal(body, b.Format == "yaml")
	if err != nil {
		return "", false, err
	}
	return backup.ConfigHash(f), false, nil
}

// takeBackup takes a backup of the Cube now, as GET …/backup does with
// the NTP servers read through the queue, and stores it as JSON for a
// reason and a user (0 for the server); the Cube is clean afterwards.
func (s *Server) takeBackup(ctx context.Context, rt *runtime, reason string, userID int64, note string) (store.Backup, *backup.File, error) {
	st, cubeID := rt.store, rt.storeID.Load()
	if st == nil || cubeID == 0 {
		return store.Backup{}, nil, errNoHistory
	}
	if !rt.client.Connected() {
		return store.Backup{}, nil, fail(http.StatusServiceUnavailable, "cube not connected")
	}
	ntp, err := s.readNTP(ctx, rt)
	if err != nil {
		s.log.Debug("backup without the NTP servers", "cube", rt.serial, "error", err)
	}
	f, err := backup.Take(rt.client.Snapshot(), ntp, s.opts.Version, time.Now(), rt.hidden())
	if err != nil {
		return store.Backup{}, nil, fail(http.StatusServiceUnavailable, "%v", err)
	}
	data, err := f.Marshal(false)
	if err != nil {
		return store.Backup{}, nil, fail(http.StatusInternalServerError, "backup: %v", err)
	}
	hash := backup.ConfigHash(f)
	// The row's time is the store's, to the millisecond, so that backups
	// of one second keep their order; the file's is to the second.
	rt.bk.rows.Lock()
	b, err := st.AddBackup(ctx, store.Backup{CubeID: cubeID, Reason: reason, Format: "json", Tool: f.Tool, UserID: userID, Note: note}, data)
	if err == nil {
		rt.bk.stored(hash)
	}
	rt.bk.rows.Unlock()
	if err != nil {
		return store.Backup{}, nil, fail(http.StatusInternalServerError, "%v", err)
	}
	s.emitBy(rt, systemEvent("backup", userID, map[string]any{"reason": reason, "id": b.ID, "bytes": b.Size}))
	s.pruneBackups(ctx, st, time.Now())
	return b, f, nil
}

// pruneBackups prunes the store's backups as of now. With
// backups.keep_newest 0 a prune may remove a Cube's newest backup, and
// the store does not say whose: where rows went, every Cube's watch
// forgets its reference, which its next look reads again.
func (s *Server) pruneBackups(ctx context.Context, st *store.Store, now time.Time) {
	n, err := st.PruneBackups(ctx, now)
	if err != nil {
		if ctx.Err() == nil {
			s.log.Warn("backups not pruned", "error", err)
		}
		return
	}
	if n == 0 {
		return
	}
	s.logServer(systemEvent("backups_pruned", 0, map[string]any{"removed": n}))
	if set, err := st.BackupSettings(ctx); err == nil && set.KeepNewest > 0 {
		return
	}
	for _, rt := range s.runtimes() {
		if rt.store == st {
			rt.bk.rows.Lock()
			rt.bk.forget()
			rt.bk.rows.Unlock()
		}
	}
}

// pruneBackupsDaily prunes the backups at the start and once a day,
// besides after each new one.
func (s *Server) pruneBackupsDaily(ctx context.Context, st *store.Store) {
	if st.ReadOnly() {
		return
	}
	t := time.NewTicker(backupPruneEvery)
	defer t.Stop()
	for {
		s.pruneBackups(ctx, st, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
