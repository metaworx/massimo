// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The automatic backup's windows across a start of the server and the
// backup at a clean stop (AP BackupWatch v1.0, Block 1; docs/store.md
// section 9 (e)). The tests look at the Cube themselves, through
// backupCheck at times of their own: with a BackupMinute of an hour the
// loop never looks while they run, and the windows' minutes are hours.

import (
	"context"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// configHash is the hash of the Cube's configuration as the runtime's
// snapshot holds it; "" before its greeting.
func configHash(rt *runtime) string {
	f, err := backup.Take(rt.client.Snapshot(), nil, "", time.Time{}, rt.hidden())
	if err != nil {
		return ""
	}
	return backup.ConfigHash(f)
}

// renamedOnTheCube renames room 6 on the fake Cube itself, as another
// app does, so that the name holds across the server's starts, and
// returns the configuration's hash once the server has it: the fake
// drops the server's connection, and the client reads the new greeting.
func renamedOnTheCube(t *testing.T, r *rig, name string) string {
	t.Helper()
	before := configHash(r.rt)
	r.fc.SetGreeting(greetingWithRoomName(t, recorded(t), 6, name))
	r.fc.Drop()
	var h string
	testkit.Eventually(t, "the renamed room in the snapshot", func() bool {
		h = configHash(r.rt)
		return r.rt.client.Connected() && h != "" && h != before
	})
	pumpSync(t, r.s) // the reconnect handled, its touch included
	return h
}

// pendingOf reads the windows the store keeps for the rig's Cube.
func pendingOf(t *testing.T, r *rig) (store.BackupPending, bool) {
	t.Helper()
	p, ok, err := r.st.BackupPending(context.Background(), r.rt.storeID.Load())
	if err != nil {
		t.Fatal(err)
	}
	return p, ok
}

// hours makes a look an hour of the windows (backups.auto_min and
// backups.auto_max in hours), never by the loop while the test runs;
// unclean also leaves out the stop's backups, as a crash would.
func hours(unclean bool) startOpt {
	return withOptions(Options{BackupMinute: time.Hour, noStopBackups: unclean})
}

// windows sets backups.auto_min and backups.auto_max.
func windows(t *testing.T, st *store.Store, autoMin, autoMax int) {
	t.Helper()
	ctx := context.Background()
	if err := st.SetSetting(ctx, store.BackupsAutoMinKey, autoMin); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, store.BackupsAutoMaxKey, autoMax); err != nil {
		t.Fatal(err)
	}
}

// A change's backup is due when it would be without a start in between:
// the look that finds the change keeps its windows in the store, the
// start after a stop that took no backup (a crash) reads them before its
// first request, and its first look, finding the configuration as kept,
// keeps them; the backup follows backups.auto_min after the change, not
// after the start, and its windows go from the store.
func TestBackupWindowsAcrossAStart(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := startServer(t, stored(), hours(true))
	windows(t, r.st, 3, 10)
	t0 := time.Now().Truncate(time.Second)
	r.s.backupCheck(ctx, r.rt, t0)
	if _, l := historyOf(t, r.ts.URL); reasons(l) != "auto" || l.Changed != nil {
		t.Fatalf("the first backup: %s, changed %v", reasons(l), l.Changed)
	}
	ref := configHash(r.rt)
	h := renamedOnTheCube(t, r, "Renamed")
	changed := t0.Add(time.Hour)
	r.rt.bk.touch()
	r.s.backupCheck(ctx, r.rt, changed)
	if p, ok := pendingOf(t, r); !ok || p.Ref != ref || p.Hash != h || !p.FirstChange.Equal(changed) || !p.LastChange.Equal(changed) {
		t.Fatalf("the windows kept: %+v %v", p, ok)
	}
	r.stop()

	r2 := startServer(t, again(r), hours(false))
	due := changed.Add(3 * time.Hour)
	// The first request, before any look.
	if _, l := historyOf(t, r2.ts.URL); l.Changed == nil || !l.Changed.Equal(changed) || l.Due == nil || !l.Due.Equal(due) {
		t.Fatalf("after the start: changed %v, due %v; want %v and %v", l.Changed, l.Due, changed, due)
	}
	r2.s.backupCheck(ctx, r2.rt, due.Add(-time.Second))
	if _, l := historyOf(t, r2.ts.URL); reasons(l) != "auto" || l.Changed == nil || !l.Changed.Equal(changed) {
		t.Fatalf("the first look after the start: %s, changed %v", reasons(l), l.Changed)
	}
	r2.s.backupCheck(ctx, r2.rt, due)
	if _, l := historyOf(t, r2.ts.URL); reasons(l) != "auto,auto" || l.Changed != nil {
		t.Fatalf("at backups.auto_min after the change: %s, changed %v", reasons(l), l.Changed)
	}
	if p, ok := pendingOf(t, r2); ok {
		t.Fatalf("the windows kept after the backup: %+v", p)
	}
}

// backups.auto_max counts from the first change across starts: a second
// change after a start moves the last change and keeps the first, and
// the backup follows backups.auto_max after the first though the quiet
// window after the second has not ended.
func TestBackupAutoMaxAcrossStarts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := startServer(t, stored(), hours(true))
	windows(t, r.st, 3, 4)
	t0 := time.Now().Truncate(time.Second)
	r.s.backupCheck(ctx, r.rt, t0)
	first := t0.Add(time.Hour)
	renamedOnTheCube(t, r, "One")
	r.rt.bk.touch()
	r.s.backupCheck(ctx, r.rt, first)
	r.stop()

	r2 := startServer(t, again(r), hours(true))
	second := first.Add(2 * time.Hour)
	h := renamedOnTheCube(t, r2, "Two")
	r2.rt.bk.touch()
	r2.s.backupCheck(ctx, r2.rt, second)
	if p, ok := pendingOf(t, r2); !ok || p.Hash != h || !p.FirstChange.Equal(first) || !p.LastChange.Equal(second) {
		t.Fatalf("the windows after the second change: %+v %v", p, ok)
	}
	r2.stop()

	r3 := startServer(t, again(r2), hours(false))
	latest := first.Add(4 * time.Hour)
	if _, l := historyOf(t, r3.ts.URL); l.Changed == nil || !l.Changed.Equal(second) || l.Due == nil || !l.Due.Equal(latest) {
		t.Fatalf("after the second start: changed %v, due %v; want %v and %v", l.Changed, l.Due, second, latest)
	}
	r3.s.backupCheck(ctx, r3.rt, latest.Add(-time.Second))
	if _, l := historyOf(t, r3.ts.URL); reasons(l) != "auto" {
		t.Fatalf("a backup before backups.auto_max: %s", reasons(l))
	}
	r3.s.backupCheck(ctx, r3.rt, latest)
	if _, l := historyOf(t, r3.ts.URL); reasons(l) != "auto,auto" || l.Changed != nil {
		t.Fatalf("at backups.auto_max after the first change: %s, changed %v", reasons(l), l.Changed)
	}
}

// A change made while no server runs is a change at the first look: the
// Cube was clean at the stop, so the store kept nothing, and the list
// names no change until that look; from it the windows count.
func TestBackupChangeWhileStopped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := startServer(t, stored(), hours(false))
	windows(t, r.st, 3, 10)
	t0 := time.Now().Truncate(time.Second)
	r.s.backupCheck(ctx, r.rt, t0)
	r.stop()
	if _, ok := pendingOf(t, r); ok {
		t.Fatal("windows kept for a clean Cube")
	}
	r.fc.SetGreeting(greetingWithRoomName(t, recorded(t), 6, "Renamed while stopped"))

	r2 := startServer(t, again(r), hours(false))
	if _, l := historyOf(t, r2.ts.URL); reasons(l) != "auto" || l.Changed != nil {
		t.Fatalf("before the first look: %s, changed %v", reasons(l), l.Changed)
	}
	look := t0.Add(5 * time.Hour)
	r2.s.backupCheck(ctx, r2.rt, look)
	_, l := historyOf(t, r2.ts.URL)
	if reasons(l) != "auto" || l.Changed == nil || !l.Changed.Equal(look) || l.Due == nil || !l.Due.Equal(look.Add(3*time.Hour)) {
		t.Fatalf("the first look: %s, changed %v, due %v; want %v", reasons(l), l.Changed, l.Due, look)
	}
	if p, ok := pendingOf(t, r2); !ok || !p.FirstChange.Equal(look) || p.Hash != configHash(r2.rt) {
		t.Fatalf("the windows kept: %+v %v", p, ok)
	}
}

// A clean stop takes the backup of a change pending before the Cube's
// runtime ends, and the store keeps no windows after it; the stop looks
// first, so a change since the last look counts as well.
func TestBackupAtTheStop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := startServer(t, stored(), hours(false))
	windows(t, r.st, 3, 10)
	t0 := time.Now().Truncate(time.Second)
	r.s.backupCheck(ctx, r.rt, t0)
	h := renamedOnTheCube(t, r, "Renamed")
	r.rt.bk.touch()
	r.s.backupCheck(ctx, r.rt, t0.Add(time.Hour))
	if _, ok := pendingOf(t, r); !ok {
		t.Fatal("no windows kept for the change")
	}
	r.stop()
	cubeID := r.rt.storeID.Load()
	newestIs := func(want string, n int) {
		t.Helper()
		list, err := r.st.Backups(ctx, cubeID)
		if err != nil || len(list) != n || list[0].Reason != store.ReasonAuto {
			t.Fatalf("the backups after the stop: %+v %v", list, err)
		}
		body, _, err := r.st.BackupBody(ctx, cubeID, list[0].ID)
		f, ferr := backup.Unmarshal(body, false)
		if err != nil || ferr != nil || backup.ConfigHash(f) != want {
			t.Fatalf("the stop's backup is not of the change (%v, %v)", err, ferr)
		}
		if p, ok := pendingOf(t, r); ok {
			t.Fatalf("windows kept after the stop's backup: %+v", p)
		}
	}
	newestIs(h, 2)

	// A change no look has seen: the stop's own look finds it.
	r2 := startServer(t, again(r), hours(false))
	if code, _ := do(t, r2.ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6", `{"name":"Again"}`); code != 200 {
		t.Fatalf("rename: %d", code)
	}
	var unseen string
	testkit.Eventually(t, "the rename in the snapshot", func() bool {
		unseen = configHash(r2.rt)
		return unseen != "" && unseen != h
	})
	r2.stop()
	newestIs(unseen, 3)
}

// A clean Cube costs the store no write of the windows, at a look or a
// backup by hand; a change writes them once, a look that finds nothing
// new not again, and its backup deletes them.
func TestBackupWindowsWrittenOnlyAtAChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := startServer(t, stored(), hours(false))
	windows(t, r.st, 3, 10)
	w := &r.rt.bk
	t0 := time.Now().Truncate(time.Second)
	look := func(at time.Duration) {
		t.Helper()
		w.touch()
		r.s.backupCheck(ctx, r.rt, t0.Add(at))
	}
	look(0) // the first backup
	look(time.Hour)
	if code, _ := do(t, r.ts, "POST", backupsBase, ""); code != 201 {
		t.Fatalf("a backup by hand: %d", code)
	}
	look(2 * time.Hour)
	if n := w.writes.Load(); n != 0 {
		t.Fatalf("%d writes of the windows while the Cube was clean", n)
	}
	renamedOnTheCube(t, r, "Renamed")
	look(3 * time.Hour)
	look(4 * time.Hour)
	if n := w.writes.Load(); n != 1 {
		t.Fatalf("%d writes of the windows for one change, want 1", n)
	}
	look(6 * time.Hour) // backups.auto_min after the change
	if _, l := historyOf(t, r.ts.URL); reasons(l) != "auto,manual,auto" {
		t.Fatalf("the backup of the change: %s", reasons(l))
	}
	look(7 * time.Hour)
	if n := w.writes.Load(); n != 2 {
		t.Fatalf("%d writes of the windows, want the change's and the backup's delete", n)
	}
	if p, ok := pendingOf(t, r); ok {
		t.Fatalf("windows kept after the backup: %+v", p)
	}
}

// Windows kept against another reference are left over, by a binary
// before migration 0026 that took the backup without deleting them, and
// go at the start: the Cube is as its newest backup, so it is clean.
func TestBackupWindowsLeftOver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := startServer(t, stored(), hours(false))
	r.s.backupCheck(ctx, r.rt, time.Now())
	r.stop()
	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := r.st.SetBackupPending(ctx, store.BackupPending{CubeID: r.rt.storeID.Load(), Ref: "an older backup's", Hash: "a change", FirstChange: at, LastChange: at}); err != nil {
		t.Fatal(err)
	}
	r2 := startServer(t, again(r), hours(false))
	if _, l := historyOf(t, r2.ts.URL); l.Changed != nil {
		t.Fatalf("windows left over in the list: changed %v", l.Changed)
	}
	if p, ok := pendingOf(t, r2); ok {
		t.Fatalf("windows left over kept: %+v", p)
	}
}
