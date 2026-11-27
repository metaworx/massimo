// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// openClocked opens a store of the template on a clock the test moves.
func openClocked(t *testing.T) (*Store, *atomic.Int64) {
	t.Helper()
	dir := t.TempDir()
	copyTemplate(t, dir)
	var at atomic.Int64
	at.Store(time.Date(2026, 11, 27, 3, 30, 0, 0, time.UTC).UnixNano())
	s, err := Open(dir, Options{Version: "test", Logger: quiet(), Now: func() time.Time { return time.Unix(0, at.Load()).UTC() }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, &at
}

// Migration 0100 is additive: a binary for 26 opens the file it made and
// reads it as before; its down part takes the schema back to the one of
// 26 and the manager's backup.retry with it.
func TestServerBackupsMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := fixedClock()
	at26 := t.TempDir()
	migrateOnly(t, at26, migrationsUpTo(t, 26), clock)
	dir := copyStore(t, at26)
	migrateOnly(t, dir, nil, clock)

	old, err := Open(dir, Options{Logger: quiet(), migrations: migrationsUpTo(t, 26), readerVersion: 26})
	if err != nil {
		t.Fatalf("a binary for 26: %v", err)
	}
	if v, mr := old.SchemaVersion(); v != 100 || mr != 24 {
		t.Fatalf("a binary for 26 reads version %d, min_reader %d", v, mr)
	}
	if _, _, off, err := old.Effective(ctx, RoleManager, []string{"read", "backup.retry"}); err != nil || off {
		t.Fatalf("a binary for 26 reads the manager: %v %v", off, err)
	}
	old.Close()

	down, err := fs.ReadFile(embeddedMigrations(), "0100_server_backups.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(Path(dir)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM role_capabilities WHERE capability = 'backup.retry'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("backup.retry seeded %d times: %v", n, err)
	}
	if _, err := db.ExecContext(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	want, got := fileSchema(t, rawDB(t, at26)), fileSchema(t, db)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("after the down part:\n%v\nwant the schema of 26:\n%v", got, want)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM role_capabilities WHERE capability LIKE 'backup.%'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("after the down part, %d rows of backup.*: %v", n, err)
	}
}

// A key pasted is a key at once and once; a key made waits for its last
// six characters, compared by hash, and goes after an hour.
func TestServerBackupKeys(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, at := openClocked(t)
	k, err := s.AddServerBackupKey(ctx, ServerBackupKey{Label: "laptop", Recipient: "age1aaa", Fingerprint: "SHA256:a", Kind: "x25519"}, "", 0)
	if err != nil || k.Pending || k.UUID == "" || k.Label != "laptop" {
		t.Fatalf("add %+v %v", k, err)
	}
	if _, err := s.AddServerBackupKey(ctx, ServerBackupKey{Recipient: "age1aaa", Fingerprint: "SHA256:a", Kind: "x25519"}, "", 0); !errors.Is(err, ErrServerBackupKeyExists) {
		t.Fatalf("the same key again: %v", err)
	}
	p, err := s.AddServerBackupKey(ctx, ServerBackupKey{Label: "made", Recipient: "age1bbb", Fingerprint: "SHA256:b", Kind: "x25519"}, "hash-b", 0)
	if err != nil || !p.Pending {
		t.Fatalf("a key made: %+v %v", p, err)
	}
	if _, err := s.ConfirmServerBackupKey(ctx, p.UUID, "wrong"); !errors.Is(err, ErrServerBackupKeyConfirm) {
		t.Fatalf("a wrong confirmation: %v", err)
	}
	if c, err := s.ConfirmServerBackupKey(ctx, p.UUID, "hash-b"); err != nil || c.Pending {
		t.Fatalf("confirmed: %+v %v", c, err)
	}
	late, _ := s.AddServerBackupKey(ctx, ServerBackupKey{Recipient: "age1ccc", Fingerprint: "SHA256:c", Kind: "x25519"}, "hash-c", 0)
	at.Add(int64(PendingKeyLifetime + time.Second))
	keys, err := s.ServerBackupKeys(ctx)
	if err != nil || len(keys) != 2 || keys[0].Fingerprint != "SHA256:a" || keys[1].Fingerprint != "SHA256:b" || keys[1].Pending {
		t.Fatalf("keys after the hour: %+v %v", keys, err)
	}
	if _, err := s.ConfirmServerBackupKey(ctx, late.UUID, "hash-c"); !errors.Is(err, ErrNoServerBackupKey) {
		t.Fatalf("a confirmation after the hour: %v", err)
	}
	if err := s.DeleteServerBackupKey(ctx, k.UUID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteServerBackupKey(ctx, k.UUID); !errors.Is(err, ErrNoServerBackupKey) {
		t.Fatalf("deleted twice: %v", err)
	}
}

// A target's secret is sealed with its uuid as the additional data: the
// row holds no plain secret, it opens for its own row and not for
// another's; a name is unique without regard to case.
func TestServerBackupTargetSecret(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _ := openClocked(t)
	a, err := s.AddServerBackupTarget(ctx, ServerBackupTarget{Name: "NAS", Kind: "webdav", Config: map[string]string{"url": "https://nas.example/dav", "user": "massimo"},
		Secret: "hunter2", Takes: "daily", Retention: json.RawMessage(`{"daily":7}`), Enabled: true}, 0)
	if err != nil || a.Secret != "hunter2" || a.SecretLost || a.Config["user"] != "massimo" || a.Takes != "daily" || string(a.Retention) != `{"daily":7}` {
		t.Fatalf("add %+v %v", a, err)
	}
	var kept string
	if err := s.db.QueryRowContext(ctx, `SELECT secret FROM server_backup_targets WHERE uuid = ?`, a.UUID).Scan(&kept); err != nil || !strings.HasPrefix(kept, sealedPrefix) || strings.Contains(kept, "hunter2") {
		t.Fatalf("kept %q %v", kept, err)
	}
	if _, err := s.AddServerBackupTarget(ctx, ServerBackupTarget{Name: "nas", Kind: "local"}, 0); !errors.Is(err, ErrServerBackupTargetExists) {
		t.Fatalf("a name taken: %v", err)
	}
	b, err := s.AddServerBackupTarget(ctx, ServerBackupTarget{Name: "Local", Kind: "local", Config: map[string]string{"path": "backups"}, Enabled: true}, 0)
	if err != nil || b.Takes != "every" || string(b.Retention) != "{}" || b.Secret != "" {
		t.Fatalf("add local %+v %v", b, err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE server_backup_targets SET secret = ? WHERE uuid = ?`, kept, b.UUID); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.ServerBackupTarget(ctx, b.UUID); !got.SecretLost || got.Secret != "" {
		t.Fatalf("another target's sealed secret opened: %+v", got)
	}
	// A lost secret not entered again stays as it was kept.
	got, err := s.UpdateServerBackupTarget(ctx, b.UUID, func(t *ServerBackupTarget) error { t.Takes = "weekly"; return nil }, 0)
	if err != nil || got.Takes != "weekly" || !got.SecretLost {
		t.Fatalf("update %+v %v", got, err)
	}
	if _, err := s.UpdateServerBackupTarget(ctx, b.UUID, func(t *ServerBackupTarget) error { t.Takes = "hourly"; return nil }, 0); err == nil {
		t.Fatal("takes hourly accepted")
	}
	if got, err := s.UpdateServerBackupTarget(ctx, a.UUID, func(t *ServerBackupTarget) error { t.Secret = "other"; t.Kind = "s3"; return nil }, 0); err != nil || got.Secret != "other" || got.Kind != "webdav" {
		t.Fatalf("a new secret, the kind kept: %+v %v", got, err)
	}
}

// A backup and its copies: kept by uuid, listed newest first with their
// copies, pinned, a failed one without a file; a backup goes with its
// last copy, and with its last target.
func TestServerBackupCatalogue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _ := openClocked(t)
	local, _ := s.AddServerBackupTarget(ctx, ServerBackupTarget{Name: "local", Kind: "local", Enabled: true}, 0)
	nas, _ := s.AddServerBackupTarget(ctx, ServerBackupTarget{Name: "nas", Kind: "webdav", Enabled: true}, 0)
	t0 := time.Date(2026, 11, 26, 3, 30, 0, 0, time.UTC)
	if _, err := s.SaveServerBackup(ctx, ServerBackup{UUID: "b1", Time: t0, Trigger: "scheduled"}); err == nil {
		t.Fatal("a backup made without its file saved")
	}
	b1, err := s.SaveServerBackup(ctx, ServerBackup{UUID: "b1", Time: t0, Trigger: "scheduled", Size: 100, SHA256: "aa", Manifest: json.RawMessage(`{"format":1}`)})
	if err != nil || b1.State != ServerBackupMade || len(b1.Copies) != 0 {
		t.Fatalf("save %+v %v", b1, err)
	}
	s.SaveServerBackup(ctx, ServerBackup{UUID: "b2", Time: t0.Add(24 * time.Hour), Trigger: "manual", Size: 200, SHA256: "bb", Manifest: json.RawMessage(`{"format":1}`)})
	failed, err := s.SaveServerBackup(ctx, ServerBackup{UUID: "f1", Time: t0.Add(48 * time.Hour), Trigger: "scheduled", State: ServerBackupFailed, Error: "disk full", Tries: 1, NextTry: t0.Add(48*time.Hour + 5*time.Minute)})
	if err != nil || failed.State != ServerBackupFailed || failed.NextTry.IsZero() {
		t.Fatalf("a failed one %+v %v", failed, err)
	}
	for _, c := range []struct {
		backup string
		c      ServerBackupCopy
	}{
		{"b1", ServerBackupCopy{TargetUUID: local.UUID, Path: "massimo-b1.zip", State: CopyThere, PushedAt: t0}},
		{"b1", ServerBackupCopy{TargetUUID: nas.UUID, Path: "massimo-b1.zip", State: CopyFailed, Error: "401", Tries: 1, NextTry: t0.Add(5 * time.Minute)}},
		{"b2", ServerBackupCopy{TargetUUID: local.UUID, Path: "massimo-b2.zip", State: CopyThere, Pulled: true}},
	} {
		if err := s.SaveServerBackupCopy(ctx, c.backup, c.c); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SaveServerBackupCopy(ctx, "nope", ServerBackupCopy{TargetUUID: local.UUID, State: CopyThere}); !errors.Is(err, ErrNoServerBackup) {
		t.Fatalf("a copy of no backup: %v", err)
	}
	// A copy saved again replaces its row.
	if err := s.SaveServerBackupCopy(ctx, "b1", ServerBackupCopy{TargetUUID: nas.UUID, Path: "massimo-b1.zip", State: CopyThere, PushedAt: t0.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	list, err := s.ServerBackups(ctx)
	if err != nil || len(list) != 3 || list[0].UUID != "f1" || list[1].UUID != "b2" || list[2].UUID != "b1" {
		t.Fatalf("list %+v %v", list, err)
	}
	if c, ok := list[2].Copy(nas.UUID); !ok || c.State != CopyThere || c.Tries != 0 || !c.NextTry.IsZero() || !c.PushedAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("b1 on the nas %+v %v", c, ok)
	}
	if c, _ := list[1].Copy(local.UUID); !c.Pulled {
		t.Fatalf("b2 pulled %+v", c)
	}
	if err := s.PinServerBackup(ctx, "b2", t0.Add(30*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.ServerBackup(ctx, "b2"); !got.Pinned(t0) || got.Pinned(t0.Add(31*24*time.Hour)) || got.UserID != 0 || got.Trigger != "manual" {
		t.Fatalf("pinned %+v", got)
	}
	if ok, err := s.DeleteServerBackupCopy(ctx, "b1", local.UUID); err != nil || !ok {
		t.Fatalf("delete a copy: %v %v", ok, err)
	}
	if _, ok, _ := s.ServerBackup(ctx, "b1"); !ok {
		t.Fatal("b1 went with a copy it still has another of")
	}
	if ok, _ := s.DeleteServerBackupCopy(ctx, "b1", local.UUID); ok {
		t.Fatal("a copy deleted twice")
	}
	s.DeleteServerBackupCopy(ctx, "b1", nas.UUID)
	if _, ok, _ := s.ServerBackup(ctx, "b1"); ok {
		t.Fatal("b1 stayed without a copy")
	}
	if err := s.DeleteServerBackupTarget(ctx, local.UUID); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ServerBackups(ctx)
	if len(list) != 1 || list[0].UUID != "f1" {
		t.Fatalf("after the local target went: %+v", list)
	}
	if err := s.DeleteServerBackup(ctx, "f1"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteServerBackup(ctx, "f1"); !errors.Is(err, ErrNoServerBackup) {
		t.Fatalf("deleted twice: %v", err)
	}

}
