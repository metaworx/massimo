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
//
// The windows hold across a start (AP BackupWatch v1.0, Block 1, D1-C):
// a dirty Cube's are kept in the store's backup_pending, with the hash
// of its reference, written at the look that finds the Cube dirty or
// changed again, so at most once a minute, and deleted once it is clean;
// a clean Cube costs the store no write. The runtime reads them when it
// is made, so that the list answers changed and due from the first
// request; a row kept against another reference is left over, and goes.
// A clean stop takes the backup of each connected Cube whose change is
// pending before the Cubes close (backupsAtStop); one not connected, or
// whose backup fails, keeps its windows for the next start.

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

// backupStopWithin bounds the backups a stop takes, all Cubes together:
// well within the ten seconds serve gives the server's stop before the
// store closes under it (internal/serve, stopGrace).
const backupStopWithin = 5 * time.Second

// backupWatch is a Cube's configuration against its newest backup.
type backupWatch struct {
	touched atomic.Bool // a connection or a configuration event since the last look

	// look makes the looks one after the other, the loop's (backupCheck)
	// and the stop's (backupAtStop): a semaphore, so that the stop waits
	// for the loop's within its own time only (lockLook). Taken before
	// rows.
	look chan struct{}

	// rows makes a backup's row and the clean state one step for the
	// list: takeBackup holds it to store the row and call stored(),
	// listBackups shares it to read the rows and state(). deleteBackup
	// holds it to find whether the row is the newest, delete it and
	// forget(), pruneBackups to forget() after a prune that may have
	// removed it, the end of a restore to call restored(); a look shares
	// it to read the reference, so that it never keeps the hash of a row
	// deleted meanwhile, and to keep its windows in the store, so that
	// they never outlive a reset. Taken before mu, never while mu is
	// held; the store is written under rows, never under mu.
	rows sync.RWMutex

	mu         sync.Mutex
	known      bool      // ref is read: the newest backup's hash, or none
	none       bool      // the Cube has no backup
	ref        string    // the hash of the newest backup, or of the file restored
	cur        string    // the hash at the last look
	dirtySince time.Time // the first change against ref; zero when the Cube is clean
	lastChange time.Time // the last change seen
	retryAt    time.Time // after a failed automatic backup
	kept       bool      // the store keeps the windows (backup_pending)

	writes atomic.Int64 // the store's writes of the windows, for the tests
}

// touch marks that the configuration may have changed.
func (w *backupWatch) touch() { w.touched.Store(true) }

// lockLook takes the look, or gives up once ctx ends.
func (w *backupWatch) lockLook(ctx context.Context) bool {
	select {
	case w.look <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

// unlockLook gives the look back.
func (w *backupWatch) unlockLook() { <-w.look }

// stored makes a backup just stored, of configuration hash, the
// reference: the Cube is clean. It reports whether the store kept its
// windows, which the caller, holding rows, then deletes.
func (w *backupWatch) stored(hash string) (kept bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.known, w.none, w.ref, w.cur = true, false, hash, hash
	w.dirtySince, w.lastChange, w.retryAt = time.Time{}, time.Time{}, time.Time{}
	kept, w.kept = w.kept, false
	return kept
}

// forget drops the reference once the newest backup is gone: the next
// look reads it again from the backup now newest, or finds none, and
// compares the configuration with it afresh, which needs the hash of
// the last look cleared as well. The caller holds rows, and deletes the
// windows the store kept where it reports any.
func (w *backupWatch) forget() (kept bool) {
	w.mu.Lock()
	w.known, w.none, w.ref = false, false, ""
	w.cur, w.dirtySince, w.lastChange = "", time.Time{}, time.Time{}
	kept, w.kept = w.kept, false
	w.mu.Unlock()
	w.touch()
	return kept
}

// restored makes the file a restore sent the reference; the next look
// says whether the Cube took all of it. As forget, the caller holds
// rows and deletes the windows kept.
func (w *backupWatch) restored(hash string) (kept bool) {
	w.mu.Lock()
	w.known, w.none, w.ref = true, false, hash
	w.cur, w.dirtySince, w.lastChange = "", time.Time{}, time.Time{}
	kept, w.kept = w.kept, false
	w.mu.Unlock()
	w.touch()
	return kept
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
var errNoHistory = refuse(http.StatusServiceUnavailable, codeBackupNoHistory)

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
	if !w.lockLook(ctx) {
		return
	}
	defer w.unlockLook()
	s.backupLook(ctx, rt, now)
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

// backupLook is a look at the Cube's configuration: the reference read
// where the watch has none, the hash compared with it once something
// may have changed, and the windows kept in the store where they began
// or moved, deleted where the Cube is clean again. The caller holds
// look; the Cube is connected.
func (s *Server) backupLook(ctx context.Context, rt *runtime, now time.Time) {
	w := &rt.bk
	w.mu.Lock()
	known := w.known
	w.mu.Unlock()
	if !known {
		s.backupReference(ctx, rt)
	}
	if !w.touched.Swap(false) {
		return
	}
	f, err := backup.Take(rt.client.Snapshot(), nil, "", time.Time{}, rt.hidden())
	if err != nil {
		w.touch() // no greeting yet: look again
		return
	}
	h := backup.ConfigHash(f)
	w.rows.RLock() // against a reset in between, which deletes what is kept
	defer w.rows.RUnlock()
	var keep *store.BackupPending
	drop := false
	w.mu.Lock()
	if h != w.cur {
		w.cur = h
		if h == w.ref {
			w.dirtySince, w.lastChange = time.Time{}, time.Time{}
			drop, w.kept = w.kept, false
		} else {
			w.lastChange = now
			if w.dirtySince.IsZero() {
				w.dirtySince = now
			}
			// A Cube without a backup gets one at once: no windows to keep.
			if !w.none {
				keep = &store.BackupPending{CubeID: rt.storeID.Load(), Ref: w.ref, Hash: h, FirstChange: w.dirtySince, LastChange: now}
				w.kept = true
			}
		}
	}
	w.mu.Unlock()
	switch {
	case keep != nil:
		w.writes.Add(1)
		if err := rt.store.SetBackupPending(ctx, *keep); err != nil && ctx.Err() == nil {
			s.log.Error("backup: the pending backup's windows not kept in the store; a start before its backup begins them again", "cube", rt.cubeSerial(), "error", err)
		}
	case drop:
		s.dropKept(ctx, rt)
	}
}

// backupReference reads the watch's reference, the newest backup's hash
// or none, with the windows the store kept against it. Windows kept
// against another reference are left over, by a backup or a delete
// without this watch (a binary before migration 0026) or a delete of
// them that failed, and go.
func (s *Server) backupReference(ctx context.Context, rt *runtime) {
	st, cubeID := rt.store, rt.storeID.Load()
	w := &rt.bk
	w.rows.RLock() // against a delete and its forget() in between
	defer w.rows.RUnlock()
	ref, none, err := s.newestBackupHash(ctx, st, cubeID)
	p, kept, perr := st.BackupPending(ctx, cubeID)
	stale := false
	w.mu.Lock()
	if !w.known {
		w.known, w.none, w.ref = true, none, ref
		if kept && err == nil && !none && p.Ref == ref {
			w.cur, w.dirtySince, w.lastChange, w.kept = p.Hash, p.FirstChange, p.LastChange, true
		} else {
			stale = kept
		}
	}
	w.mu.Unlock()
	if err != nil {
		s.log.Warn("backup: the newest backup is not readable; the next is taken as after a change", "cube", rt.cubeSerial(), "error", err)
	}
	if perr != nil {
		s.log.Warn("backup: the pending backup's windows are not readable; they begin at the next look", "cube", rt.cubeSerial(), "error", perr)
	}
	if stale && !st.ReadOnly() {
		s.dropKept(ctx, rt)
	}
	w.touch()
}

// resumeBackupWatch reads the windows the store kept for the Cube, so
// that the list answers changed and due from the first request; a Cube
// without any reads its reference at its first look.
func (s *Server) resumeBackupWatch(ctx context.Context, rt *runtime) {
	st, cubeID := rt.store, rt.storeID.Load()
	if st == nil || cubeID == 0 {
		return
	}
	if _, ok, err := st.BackupPending(ctx, cubeID); err != nil || !ok {
		return
	}
	s.backupReference(ctx, rt)
}

// dropKept deletes the windows the store kept for the Cube, once its
// watch has none: after a backup stored, a reset by forget or restored,
// a look that found the Cube clean again, or a row left over. The
// caller holds rows. The delete outlives a request that ends meanwhile.
func (s *Server) dropKept(ctx context.Context, rt *runtime) {
	rt.bk.writes.Add(1)
	if err := rt.store.DeleteBackupPending(context.WithoutCancel(ctx), rt.storeID.Load()); err != nil {
		s.log.Error("backup: the pending backup's windows not deleted from the store; the next start finds them left over", "cube", rt.cubeSerial(), "error", err)
	}
}

// backupsAtStop takes, as the server stops, the automatic backup of
// each connected Cube whose configuration differs from its reference,
// before the Cubes' runtimes end (Run): a change does not wait for a
// start that may be hours away. All Cubes at once, within
// backupStopWithin; ctx is the Cubes' own, still alive.
func (s *Server) backupsAtStop(ctx context.Context) {
	st := s.opts.Store
	if st == nil || st.ReadOnly() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, backupStopWithin)
	defer cancel()
	var wg sync.WaitGroup
	for _, rt := range s.runtimes() {
		wg.Add(1)
		go func() { defer wg.Done(); s.backupAtStop(ctx, rt) }()
	}
	wg.Wait()
}

// backupAtStop is the stop's backup of one Cube, after a last look for
// a change since the loop's. A Cube without a backup waits for its
// first look after the next start. One not connected or under a
// restore, or whose backup fails, is logged, and keeps its windows in
// the store for the next start.
func (s *Server) backupAtStop(ctx context.Context, rt *runtime) {
	if rt.store == nil || rt.storeID.Load() == 0 {
		return
	}
	w := &rt.bk
	if !w.lockLook(ctx) {
		s.log.Warn("backup: none at the stop, the look under way took its time; a change pending waits for the next start", "cube", rt.cubeSerial())
		return
	}
	defer w.unlockLook()
	connected, restoring := rt.client.Connected(), rt.restoring() != nil
	if connected && !restoring {
		s.backupLook(ctx, rt, time.Now())
	}
	w.mu.Lock()
	pending := w.known && !w.none && !w.dirtySince.IsZero()
	w.mu.Unlock()
	switch {
	case !pending:
		return
	case !connected:
		s.log.Warn("backup: none at the stop, the cube is not connected; its change waits for the next start", "cube", rt.cubeSerial())
		return
	case restoring:
		s.log.Warn("backup: none at the stop, a restore is under way; its change waits for the next start", "cube", rt.cubeSerial())
		return
	}
	if _, _, err := s.takeBackup(ctx, rt, store.ReasonAuto, 0, ""); err != nil {
		// What the Cube did not give is a warning, what failed here an
		// error; the windows stay in the store either way.
		failed := systemEvent("backup", 0, map[string]any{"reason": store.ReasonAuto, "at": "stop"})
		failed.Level, failed.Error = "error", err.Error()
		var he *httpError
		if errors.As(err, &he) && he.status == http.StatusServiceUnavailable {
			failed.Level = "warn"
		}
		rt.emit(failed)
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
		return store.Backup{}, nil, refuseSaying(http.StatusServiceUnavailable, "cube not connected", codeCubeNotConnected)
	}
	// Within a deadline, as at a stop, the NTP servers take half the time
	// left at most, so that the backup is stored within the rest.
	nctx := ctx
	if dl, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		nctx, cancel = context.WithTimeout(ctx, time.Until(dl)/2)
		defer cancel()
	}
	ntp, err := s.readNTP(nctx, rt)
	if err != nil {
		s.log.Debug("backup without the NTP servers", "cube", rt.cubeSerial(), "error", err)
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
	if err == nil && rt.bk.stored(hash) {
		s.dropKept(ctx, rt)
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
			if rt.bk.forget() {
				s.dropKept(ctx, rt)
			}
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
