// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// A backup is stored as given and read back so, listed newest first
// without its body; a bad one is refused; a deleted one is a tombstone
// the list, the lookup and the body leave out.
func TestBackupsStored(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	a, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	b, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000002", Configured: true})
	t0 := time.Now().UTC().Truncate(time.Second)
	body := []byte(`{"version": 1, "cube": {"serial": "KEQ0000001"}}`)
	first, err := s.AddBackup(ctx, Backup{CubeID: a.ID, Reason: ReasonAuto, Format: "json", Tool: "test", TakenAt: t0.Add(-time.Hour)}, body)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if first.ID == 0 || first.UUID == "" || first.Size != int64(len(body)) || first.SHA256 != hex.EncodeToString(sum[:]) ||
		first.Reason != ReasonAuto || first.Format != "json" || first.Tool != "test" || !first.TakenAt.Equal(t0.Add(-time.Hour)) {
		t.Fatalf("stored: %+v", first)
	}
	yaml := []byte("version: 1\ncube:\n  serial: KEQ0000001\n")
	second, err := s.AddBackup(ctx, Backup{CubeID: a.ID, Reason: ReasonManual, Format: "yaml", Note: "before the holidays", TakenAt: t0}, yaml)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddBackup(ctx, Backup{CubeID: b.ID, Reason: ReasonUpload, Format: "json"}, body); err != nil {
		t.Fatal(err)
	}
	list, err := s.Backups(ctx, a.ID)
	if err != nil || len(list) != 2 || list[0].ID != second.ID || list[1].ID != first.ID || list[0].Note != "before the holidays" {
		t.Fatalf("list: %+v %v", list, err)
	}
	if got, ok, err := s.BackupBody(ctx, a.ID, second.ID); err != nil || !ok || string(got) != string(yaml) {
		t.Fatalf("body: %q %v %v", got, ok, err)
	}
	if _, ok, _ := s.StoredBackup(ctx, b.ID, first.ID); ok {
		t.Fatal("another Cube's backup found")
	}
	if n, ok, _ := s.NewestBackup(ctx, a.ID); !ok || n.ID != second.ID {
		t.Fatalf("newest: %+v %v", n, ok)
	}
	for _, bad := range []Backup{{CubeID: a.ID, Reason: "daily", Format: "json"}, {CubeID: a.ID, Reason: ReasonAuto, Format: "xml"}, {Reason: ReasonAuto, Format: "json"}} {
		if _, err := s.AddBackup(ctx, bad, body); !errors.Is(err, ErrBadBackup) {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
	if _, err := s.AddBackup(ctx, Backup{CubeID: a.ID, Reason: ReasonAuto, Format: "json"}, make([]byte, MaxBackupBytes+1)); !errors.Is(err, ErrBadBackup) {
		t.Fatalf("too large: %v", err)
	}
	if ok, err := s.DeleteBackup(ctx, a.ID, second.ID); !ok || err != nil {
		t.Fatalf("delete: %v %v", ok, err)
	}
	if ok, _ := s.DeleteBackup(ctx, a.ID, second.ID); ok {
		t.Fatal("deleted twice")
	}
	if _, ok, _ := s.BackupBody(ctx, a.ID, second.ID); ok {
		t.Fatal("the tombstone's body")
	}
	if list, _ := s.Backups(ctx, a.ID); len(list) != 1 || list[0].ID != first.ID {
		t.Fatalf("after the delete: %+v", list)
	}
	var left int
	s.db.QueryRow(`SELECT length(body) FROM backups WHERE id = ?`, second.ID).Scan(&left)
	if left != 0 {
		t.Fatalf("the tombstone keeps %d bytes", left)
	}
}

// The backup counts are by serial, of the rows that are not removed,
// configured or only found: a tombstone does not count, a row without
// a backup and a removed row are not listed; the kept list has the
// same rows by serial, each with its newest backup's time.
func TestBackupCounts(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	served, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true, Name: "Hall"})
	found, _, err := s.NoteDiscovered(ctx, Discovered{Serial: "KEQ0000002", Address: "192.0.2.2"})
	if err != nil || found.Configured {
		t.Fatalf("found: %+v %v", found, err)
	}
	s.AddCube(ctx, Cube{Serial: "KEQ0000003", Configured: true})
	gone, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000004", Configured: true})
	body := []byte(`{"version": 1}`)
	add := func(id int64) Backup {
		t.Helper()
		b, err := s.AddBackup(ctx, Backup{CubeID: id, Reason: ReasonManual, Format: "json"}, body)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	add(served.ID)
	deleted := add(served.ID)
	add(served.ID)
	add(found.ID)
	add(gone.ID)
	if ok, err := s.DeleteBackup(ctx, served.ID, deleted.ID); !ok || err != nil {
		t.Fatalf("delete: %v %v", ok, err)
	}
	if _, err := s.RemoveCube(ctx, gone.Serial, 0); err != nil {
		t.Fatal(err)
	}
	got, err := s.BackupCounts(ctx)
	if err != nil || len(got) != 2 || got["KEQ0000001"] != 2 || got["KEQ0000002"] != 1 {
		t.Fatalf("counts: %v %v", got, err)
	}
	kept, err := s.KeptBackups(ctx)
	if err != nil || len(kept) != 2 || kept[0].Serial != "KEQ0000001" || kept[0].Name != "Hall" || kept[0].Count != 2 ||
		kept[1].Serial != "KEQ0000002" || kept[1].Name != "" || kept[1].Count != 1 {
		t.Fatalf("kept: %+v %v", kept, err)
	}
	if newest, _, _ := s.NewestBackup(ctx, served.ID); !kept[0].Newest.Equal(newest.TakenAt) || kept[0].Newest.IsZero() {
		t.Fatalf("the newest: %v, want %v", kept[0].Newest, newest.TakenAt)
	}
}

// The prune keeps the newest backups.keep_newest of each Cube whatever
// their age, and of the others removes an automatic one past
// backups.keep_days_auto and any other past backups.keep_days_manual,
// tombstones too; 0 days keeps a reason's.
func TestBackupsPruned(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	a, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	b, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000002", Configured: true})
	for k, v := range map[string]int{BackupsKeepNewestKey: 2, BackupsKeepDaysAutoKey: 30, BackupsKeepDaysManualKey: 100} {
		if err := s.SetSetting(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC().Truncate(time.Second)
	days := func(d int) time.Time { return now.Add(-time.Duration(d) * 24 * time.Hour) }
	add := func(cube int64, reason string, age int) int64 {
		t.Helper()
		bk, err := s.AddBackup(ctx, Backup{CubeID: cube, Reason: reason, Format: "json", TakenAt: days(age)}, []byte(`{"version":1}`))
		if err != nil {
			t.Fatal(err)
		}
		return bk.ID
	}
	newest1 := add(a.ID, ReasonAuto, 300) // the newest two of A are old,
	newest2 := add(a.ID, ReasonAuto, 301) // and kept all the same
	oldAuto := add(a.ID, ReasonAuto, 302)
	oldManual := add(a.ID, ReasonManual, 400)
	// Ages are what count, not the order of adding: these are newer.
	s.db.Exec(`UPDATE backups SET taken_at = ? WHERE id = ?`, formatTime(days(1)), newest1)
	s.db.Exec(`UPDATE backups SET taken_at = ? WHERE id = ?`, formatTime(days(2)), newest2)
	keptAuto := add(a.ID, ReasonAuto, 20)
	keptManual := add(a.ID, ReasonBeforeRestore, 90)
	onlyB := add(b.ID, ReasonAuto, 500)
	tomb := add(b.ID, ReasonManual, 3)
	s.DeleteBackup(ctx, b.ID, tomb)
	s.db.Exec(`UPDATE backups SET deleted_at = ? WHERE id = ?`, formatTime(days(150)), tomb)
	n, err := s.PruneBackups(ctx, now)
	if err != nil || n != 3 {
		t.Fatalf("pruned %d, %v; want the old auto, the old manual and the tombstone", n, err)
	}
	ids := func(cube int64) map[int64]bool {
		out := map[int64]bool{}
		list, _ := s.Backups(ctx, cube)
		for _, bk := range list {
			out[bk.ID] = true
		}
		return out
	}
	gotA := ids(a.ID)
	if len(gotA) != 4 || !gotA[newest1] || !gotA[newest2] || !gotA[keptAuto] || !gotA[keptManual] || gotA[oldAuto] || gotA[oldManual] {
		t.Fatalf("Cube A kept %v", gotA)
	}
	if gotB := ids(b.ID); len(gotB) != 1 || !gotB[onlyB] {
		t.Fatalf("Cube B kept %v", gotB)
	}
	var tombs int
	s.db.QueryRow(`SELECT count(*) FROM backups WHERE deleted_at IS NOT NULL`).Scan(&tombs)
	if tombs != 0 {
		t.Fatalf("%d tombstones left", tombs)
	}
	// 0 days keeps a reason's backups for ever.
	s.SetSetting(ctx, BackupsKeepNewestKey, 0)
	s.SetSetting(ctx, BackupsKeepDaysAutoKey, 0)
	s.SetSetting(ctx, BackupsKeepDaysManualKey, 0)
	if n, _ := s.PruneBackups(ctx, now.Add(10*365*24*time.Hour)); n != 0 {
		t.Fatalf("0 days removed %d", n)
	}
}

// The settings' defaults, and what the store holds in their place.
func TestBackupSettings(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	got, err := s.BackupSettings(ctx)
	want := BackupSettings{AutoMin: 60, AutoMax: 120, KeepDaysAuto: 90, KeepDaysManual: 365, KeepNewest: 10}
	if err != nil || got != want {
		t.Fatalf("defaults %+v %v", got, err)
	}
	s.SetSetting(ctx, BackupsAutoMinKey, 5)
	s.SetSetting(ctx, BackupsKeepNewestKey, 3)
	if got, _ := s.BackupSettings(ctx); got.AutoMin != 5 || got.KeepNewest != 3 || got.AutoMax != 120 {
		t.Fatalf("set: %+v", got)
	}
}

// Migration 0005 copies a backup of the old reasons connect and daily as
// auto, and the table refuses them afterwards.
func TestBackupsMigration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(embedded, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "0005") {
			break
		}
		data, _ := embedded.ReadFile("migrations/" + e.Name())
		before[e.Name()] = &fstest.MapFile{Data: data}
	}
	old, err := Open(dir, Options{Logger: quiet(), migrations: before, readerVersion: 4})
	if err != nil {
		t.Fatal(err)
	}
	c := Cube{ID: oldCube(t, old, "KEQ0000001")}
	for i, reason := range []string{"connect", "daily", "manual"} {
		if _, err := old.db.Exec(`INSERT INTO backups (uuid, cube_id, taken_at, reason, format, size, sha256, body) VALUES (?, ?, ?, ?, 'json', 1, '', x'00')`,
			newUUID(), c.ID, formatTime(time.Now().Add(time.Duration(i)*time.Second)), reason); err != nil {
			t.Fatal(err)
		}
	}
	old.Close()
	s := openFresh(t, dir)
	rows, err := s.db.Query(`SELECT reason FROM backups ORDER BY taken_at`)
	if err != nil {
		t.Fatal(err)
	}
	var reasons []string
	for rows.Next() {
		var r string
		rows.Scan(&r)
		reasons = append(reasons, r)
	}
	rows.Close()
	if strings.Join(reasons, ",") != "auto,auto,manual" {
		t.Fatalf("copied as %v", reasons)
	}
	if _, err := s.db.Exec(`INSERT INTO backups (uuid, cube_id, taken_at, reason, format, size, sha256) VALUES (?, ?, ?, 'daily', 'json', 1, '')`,
		newUUID(), c.ID, s.nowText()); err == nil {
		t.Fatal("the reason daily taken after 0005")
	}
}

// A backup's id is never given out again (migration 0013): not after the
// prune removed the newest backup past backups.keep_newest, nor after it
// purged the tombstone of a deleted newest one.
func TestBackupIDsNotReused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	day := 24 * time.Hour
	add := func(t *testing.T, s *Store, cube int64, reason string, at time.Time) int64 {
		t.Helper()
		b, err := s.AddBackup(ctx, Backup{CubeID: cube, Reason: reason, Format: "json", TakenAt: at}, []byte(`{"version":1}`))
		if err != nil {
			t.Fatal(err)
		}
		return b.ID
	}
	t.Run("pruned", func(t *testing.T) {
		s := open(t, t.TempDir())
		c, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
		s.SetSetting(ctx, BackupsKeepNewestKey, 0)
		add(t, s, c.ID, ReasonManual, now.Add(-200*day))         // kept: manual, 365 days
		newest := add(t, s, c.ID, ReasonAuto, now.Add(-100*day)) // pruned: auto, 90 days
		if n, err := s.PruneBackups(ctx, now); err != nil || n != 1 {
			t.Fatalf("pruned %d, %v; want the auto", n, err)
		}
		if next := add(t, s, c.ID, ReasonAuto, now); next <= newest {
			t.Fatalf("the next backup is %d, the pruned newest was %d", next, newest)
		}
	})
	t.Run("tombstone", func(t *testing.T) {
		s := open(t, t.TempDir())
		c, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
		add(t, s, c.ID, ReasonManual, now.Add(-time.Hour))
		newest := add(t, s, c.ID, ReasonManual, now)
		if ok, err := s.DeleteBackup(ctx, c.ID, newest); !ok || err != nil {
			t.Fatalf("delete: %v %v", ok, err)
		}
		// Past backups.keep_days_manual (365) the tombstone goes, and the
		// older backup stays as one of the newest backups.keep_newest.
		if n, err := s.PruneBackups(ctx, now.Add(400*day)); err != nil || n != 1 {
			t.Fatalf("pruned %d, %v; want the tombstone", n, err)
		}
		if next := add(t, s, c.ID, ReasonAuto, now.Add(time.Minute)); next <= newest {
			t.Fatalf("the next backup is %d, the purged tombstone was %d", next, newest)
		}
	})
}

// Migration 0013 copies every backup with its id, uuid, body and sync
// columns, tombstones too, and starts sqlite_sequence at the highest id,
// so that a tombstone purged after it leaves its id unused.
func TestBackupsAutoincrementMigration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(embedded, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "0013") {
			break
		}
		data, _ := embedded.ReadFile("migrations/" + e.Name())
		before[e.Name()] = &fstest.MapFile{Data: data}
	}
	old, err := Open(dir, Options{Logger: quiet(), migrations: before, readerVersion: 12})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c := Cube{ID: oldCube(t, old, "KEQ0000001")}
	now := time.Now().UTC().Truncate(time.Second)
	kept := Backup{ID: oldBackup(t, old, c.ID, ReasonManual, "yaml", now.Add(-time.Hour), []byte("version: 1\n"))}
	gone := Backup{ID: oldBackup(t, old, c.ID, ReasonAuto, "json", now, []byte(`{"version":1}`))}
	if _, err := old.db.ExecContext(ctx, `UPDATE backups SET deleted_at = ?, body = x'', version = version + 1 WHERE id = ?`, formatTime(now), gone.ID); err != nil {
		t.Fatal(err)
	}
	type row struct {
		ID                         int64
		UUID, Deleted, Origin, Sum string
		Version, Body              int64
	}
	// The files are in backup_data since 0024.
	rows := func(s *Store, body string) []row {
		t.Helper()
		r, err := s.db.Query(`SELECT id, uuid, coalesce(deleted_at, ''), origin, sha256, version, ` + body + ` ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		var out []row
		for r.Next() {
			var x row
			r.Scan(&x.ID, &x.UUID, &x.Deleted, &x.Origin, &x.Sum, &x.Version, &x.Body)
			out = append(out, x)
		}
		return out
	}
	want := rows(old, `length(body) FROM backups`)
	old.Close()

	s := openFresh(t, dir)
	if got := rows(s, `coalesce((SELECT length(d.data) FROM backup_data d WHERE d.id = backups.id), 0) FROM backups`); fmt.Sprint(got) != fmt.Sprint(want) ||
		len(got) != 2 || got[1].Deleted == "" {
		t.Fatalf("copied %+v, want %+v", got, want)
	}
	if body, ok, err := s.BackupBody(ctx, c.ID, kept.ID); err != nil || !ok || string(body) != "version: 1\n" {
		t.Fatalf("the kept body: %q %v %v", body, ok, err)
	}
	var seq int64
	if err := s.db.QueryRow(`SELECT seq FROM sqlite_sequence WHERE name = 'backups'`).Scan(&seq); err != nil || seq != gone.ID {
		t.Fatalf("sqlite_sequence %d, %v; want %d", seq, err, gone.ID)
	}
	if n, err := s.PruneBackups(ctx, now.Add(400*24*time.Hour)); err != nil || n != 1 {
		t.Fatalf("pruned %d, %v; want the tombstone", n, err)
	}
	next, err := s.AddBackup(ctx, Backup{CubeID: c.ID, Reason: ReasonAuto, Format: "json"}, []byte(`{"version":1}`))
	if err != nil || next.ID <= gone.ID {
		t.Fatalf("the next backup is %d (%v), the purged tombstone was %d", next.ID, err, gone.ID)
	}
}

// A Cube's pending backup is kept, replaced and forgotten, one row per
// Cube, its times to the millisecond; one without a hash or a change is
// refused, and a store opened read-only writes none.
func TestBackupPending(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := open(t, dir)
	ctx := context.Background()
	a, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	b, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000002", Configured: true})
	if _, ok, err := s.BackupPending(ctx, a.ID); ok || err != nil {
		t.Fatalf("a pending backup before any: %v %v", ok, err)
	}
	t0 := time.Date(2026, 11, 24, 0, 32, 0, 123_000_000, time.UTC)
	same := func(got, want BackupPending) bool {
		return got.CubeID == want.CubeID && got.Ref == want.Ref && got.Hash == want.Hash && got.FirstChange.Equal(want.FirstChange) && got.LastChange.Equal(want.LastChange)
	}
	p := BackupPending{CubeID: a.ID, Ref: "ref", Hash: "first", FirstChange: t0, LastChange: t0}
	if err := s.SetBackupPending(ctx, p); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := s.BackupPending(ctx, a.ID); !ok || err != nil || !same(got, p) {
		t.Fatalf("kept %+v %v %v, want %+v", got, ok, err, p)
	}
	// A new hash while dirty: the first change stays, the last moves.
	p.Hash, p.LastChange = "second", t0.Add(10*time.Minute)
	if err := s.SetBackupPending(ctx, p); err != nil {
		t.Fatal(err)
	}
	other := BackupPending{CubeID: b.ID, Hash: "other", FirstChange: t0, LastChange: t0}
	if err := s.SetBackupPending(ctx, other); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM backup_pending WHERE cube_id = ?`, a.ID).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("%d rows of the Cube, %v", rows, err)
	}
	if got, ok, _ := s.BackupPending(ctx, a.ID); !ok || !same(got, p) {
		t.Fatalf("replaced %+v, want %+v", got, p)
	}
	for _, bad := range []BackupPending{{CubeID: a.ID, FirstChange: t0, LastChange: t0}, {CubeID: a.ID, Hash: "x"}, {Hash: "x", FirstChange: t0, LastChange: t0}} {
		if err := s.SetBackupPending(ctx, bad); err == nil {
			t.Fatalf("kept %+v", bad)
		}
	}
	if err := s.DeleteBackupPending(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.BackupPending(ctx, a.ID); ok {
		t.Fatal("the Cube's pending backup is still there")
	}
	if got, ok, _ := s.BackupPending(ctx, b.ID); !ok || !same(got, other) {
		t.Fatalf("the other Cube's went too: %+v %v", got, ok)
	}
	if err := s.DeleteBackupPending(ctx, a.ID); err != nil {
		t.Fatalf("deleting none: %v", err)
	}
	s.Close()
	ro, err := OpenReadOnly(dir, Options{Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if got, ok, err := ro.BackupPending(ctx, b.ID); !ok || err != nil || !same(got, other) {
		t.Fatalf("read-only: %+v %v %v", got, ok, err)
	}
	if ro.SetBackupPending(ctx, p) == nil || ro.DeleteBackupPending(ctx, b.ID) == nil {
		t.Fatal("a store opened read-only wrote a pending backup")
	}
}

// Migration 0026 is additive: a binary for 25 opens the file it made and
// reads it as before, and its down part takes the schema back to the one
// of 25.
func TestBackupPendingMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := fixedClock()
	at25 := t.TempDir()
	migrateOnly(t, at25, migrationsUpTo(t, 25), clock)
	dir := copyStore(t, at25)
	migrateOnly(t, dir, nil, clock)
	s := newStore(dir, Options{Logger: quiet(), Now: clock})
	if err := s.open(false); err != nil {
		t.Fatal(err)
	}
	c := oldCube(t, s, "KEQ0000001")
	if err := s.SetBackupPending(ctx, BackupPending{CubeID: c, Ref: "ref", Hash: "hash", FirstChange: clock(), LastChange: clock()}); err != nil {
		t.Fatal(err)
	}
	s.Close()

	old, err := Open(dir, Options{Logger: quiet(), migrations: migrationsUpTo(t, 25), readerVersion: 25})
	if err != nil {
		t.Fatalf("a binary for 25: %v", err)
	}
	if v, mr := old.SchemaVersion(); v != 26 || mr != 24 {
		t.Fatalf("a binary for 25 reads version %d, min_reader %d", v, mr)
	}
	if row, ok, err := old.CubeByID(ctx, c); err != nil || !ok || row.Serial != "KEQ0000001" {
		t.Fatalf("a binary for 25 reads the Cube: %+v %v %v", row, ok, err)
	}
	old.Close()

	down, err := fs.ReadFile(embeddedMigrations(), "0026_backup_pending.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(Path(dir)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	want, got := fileSchema(t, rawDB(t, at25)), fileSchema(t, db)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("after the down part:\n%v\nwant the schema of 25:\n%v", got, want)
	}
}
