// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// A backup is stored as given and read back so, listed newest first
// without its body; a bad one is refused; a deleted one is a tombstone
// the list, the lookup and the body leave out.
func TestBackupsStored(t *testing.T) {
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

// The prune keeps the newest backups.keep_newest of each Cube whatever
// their age, and of the others removes an automatic one past
// backups.keep_days_auto and any other past backups.keep_days_manual,
// tombstones too; 0 days keeps a reason's.
func TestBackupsPruned(t *testing.T) {
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
	c, _ := old.AddCube(context.Background(), Cube{Serial: "KEQ0000001", Configured: true})
	for i, reason := range []string{"connect", "daily", "manual"} {
		if _, err := old.db.Exec(`INSERT INTO backups (uuid, cube_id, taken_at, reason, format, size, sha256, body) VALUES (?, ?, ?, ?, 'json', 1, '', x'00')`,
			newUUID(), c.ID, formatTime(time.Now().Add(time.Duration(i)*time.Second)), reason); err != nil {
			t.Fatal(err)
		}
	}
	old.Close()
	s := open(t, dir)
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
	if _, err := s.db.Exec(`INSERT INTO backups (uuid, cube_id, taken_at, reason, format, size, sha256, body) VALUES (?, ?, ?, 'daily', 'json', 1, '', x'00')`,
		newUUID(), c.ID, s.nowText()); err == nil {
		t.Fatal("the reason daily taken after 0005")
	}
}
