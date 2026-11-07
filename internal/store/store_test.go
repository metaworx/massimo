// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func open(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir, Options{Version: "test", Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// An empty directory gets massimo.db at the latest version, mode 0600,
// in WAL mode with foreign keys on.
func TestOpenMigratesAnEmptyFile(t *testing.T) {
	dir := t.TempDir()
	var logged bytes.Buffer
	s, err := Open(dir, Options{Version: "v9.9.9", Logger: slog.New(slog.NewTextHandler(&logged, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !s.Created() {
		t.Error("not reported as created")
	}
	v, mr := s.SchemaVersion()
	if v != SchemaVersion() || v < 1 || mr != 1 {
		t.Fatalf("schema %d min_reader %d, binary %d", v, mr, SchemaVersion())
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(Path(dir))
		if err != nil || fi.Mode().Perm() != 0o600 {
			t.Fatalf("file mode %v %v", fi.Mode(), err)
		}
	}
	var mode string
	var fk int
	s.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode)
	s.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk)
	if mode != "wal" || fk != 1 {
		t.Fatalf("journal_mode %q foreign_keys %d", mode, fk)
	}
	var by string
	s.db.QueryRow(`SELECT applied_by FROM schema_version WHERE version = 1`).Scan(&by)
	if by != "v9.9.9" {
		t.Fatalf("applied_by %q", by)
	}
	for _, want := range []string{"store created", "schema migrated", "node key created"} {
		if !strings.Contains(logged.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, logged.String())
		}
	}
	// Every table of docs/store.md section 5 exists.
	for _, table := range []string{"schema_version", "roles", "role_capabilities", "capabilities_disabled", "users", "tokens", "settings", "cubes", "preferences", "events", "audit", "backups", "auth_failures"} {
		var n int
		if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil || n != 1 {
			t.Errorf("table %s: %d %v", table, n, err)
		}
	}
	s.Close()

	// The second open migrates nothing and creates nothing.
	logged.Reset()
	s2, err := Open(dir, Options{Version: "v9.9.9", Logger: slog.New(slog.NewTextHandler(&logged, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if s2.Created() || logged.Len() != 0 {
		t.Fatalf("second open: created %v, log %s", s2.Created(), logged.String())
	}
	var rows int
	s2.db.QueryRow(`SELECT count(*) FROM schema_version`).Scan(&rows)
	if rows != SchemaVersion() {
		t.Fatalf("%d schema_version rows", rows)
	}
}

// The owned and replicated tables carry the columns the notes of
// docs/store.md section 5 add.
func TestSyncColumns(t *testing.T) {
	s := open(t, t.TempDir())
	cols := func(table string) map[string]bool {
		rows, err := s.db.Query(`SELECT name FROM pragma_table_info(?)`, table)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]bool{}
		for rows.Next() {
			var n string
			rows.Scan(&n)
			out[n] = true
		}
		return out
	}
	for _, table := range []string{"users", "tokens", "cubes", "preferences", "events", "audit", "backups"} {
		if !cols(table)["uuid"] {
			t.Errorf("%s has no uuid", table)
		}
	}
	for _, table := range []string{"users", "tokens", "cubes", "settings", "preferences", "backups", "roles"} {
		c := cols(table)
		for _, want := range []string{"version", "deleted_at", "origin"} {
			if !c[want] {
				t.Errorf("%s has no %s", table, want)
			}
		}
	}
	for _, table := range []string{"users", "roles", "cubes", "settings"} {
		c := cols(table)
		if !c["updated_at"] || !c["updated_by"] {
			t.Errorf("%s lacks updated_at or updated_by", table)
		}
	}
	if c := cols("tenants"); len(c) != 0 {
		t.Error("a tenants table")
	}
	for _, table := range []string{"users", "cubes", "events", "audit", "backups"} {
		if cols(table)["tenant_id"] {
			t.Errorf("%s has tenant_id", table)
		}
	}
}

// The six default roles and their sets, as docs/store.md section 6
// lists them (preferences seeded by migration 0003); owner has every
// capability by code and no rows.
func TestSeededRoles(t *testing.T) {
	s := open(t, t.TempDir())
	want := map[string][]string{
		"owner":       nil,
		"manager":     {"backup", "configure", "discover", "guests", "lock", "pair", "preferences", "read", "rename", "restart", "restore", "setpoint", "structure"},
		"operator":    {"backup", "configure", "discover", "lock", "preferences", "read", "setpoint"},
		"guest":       {"configure", "read", "setpoint"},
		"viewer":      {"preferences", "read"},
		"integration": {"lock", "read", "setpoint"},
	}
	rows, err := s.db.Query(`SELECT r.key, r.builtin, r.origin, coalesce(group_concat(c.capability), '') FROM roles r LEFT JOIN role_capabilities c ON c.role_id = r.id GROUP BY r.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string][]string{}
	for rows.Next() {
		var key, origin, caps string
		var builtin bool
		if err := rows.Scan(&key, &builtin, &origin, &caps); err != nil {
			t.Fatal(err)
		}
		if !builtin || origin != s.NodeID() {
			t.Errorf("role %s: builtin %v origin %q", key, builtin, origin)
		}
		var list []string
		if caps != "" {
			list = strings.Split(caps, ",")
		}
		sort.Strings(list)
		got[key] = list
	}
	if len(got) != len(want) {
		t.Fatalf("roles %v", got)
	}
	for k, w := range want {
		if strings.Join(got[k], ",") != strings.Join(w, ",") {
			t.Errorf("role %s: %v, want %v", k, got[k], w)
		}
	}
}

// node.id and tenant.id are UUIDv7; the key file is made once, 0600,
// and read back on the next open.
func TestNodeIdentity(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	id := s.NodeID()
	tenant, err := s.TenantID()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{id, tenant} {
		u, err := uuid.Parse(v)
		if err != nil || u.Version() != 7 {
			t.Fatalf("%q is not a UUIDv7: %v", v, err)
		}
	}
	if id == tenant {
		t.Fatal("node.id equals tenant.id")
	}
	pub := s.NodePublicKey()
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("public key %x", pub)
	}
	sig, err := s.Sign([]byte("hello"))
	if err != nil || !ed25519.Verify(pub, []byte("hello"), sig) {
		t.Fatalf("signature %v", err)
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(KeyPath(dir)); err != nil || fi.Mode().Perm() != 0o600 {
			t.Fatalf("key file %v %v", fi, err)
		}
	}
	key1, _ := os.ReadFile(KeyPath(dir))
	s.Close()

	s2 := open(t, dir)
	key2, _ := os.ReadFile(KeyPath(dir))
	if s2.NodeID() != id || !bytes.Equal(s2.NodePublicKey(), pub) || !bytes.Equal(key1, key2) {
		t.Fatal("identity changed on the second open")
	}
}

// A file with tables but without schema_version is not the store's.
func TestRefusesAStrangersFile(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE other (x INTEGER)`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Open(dir, Options{Logger: quiet()}); !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "no schema_version") {
		t.Fatalf("open: %v", err)
	}
}

// testMigrations is the embedded first migration plus the given files.
func testMigrations(t *testing.T, extra map[string]string) fstest.MapFS {
	t.Helper()
	first, err := embedded.ReadFile("migrations/0001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := fstest.MapFS{"0001_initial.sql": {Data: first}}
	for name, sql := range extra {
		m[name] = &fstest.MapFile{Data: []byte(sql)}
	}
	return m
}

// An additive migration keeps min_reader, so a binary built for the
// older version opens the newer file; one that raises it is refused by
// that binary with the copy named, and the copy is made before it.
func TestMinReader(t *testing.T) {
	dir := t.TempDir()
	additive := testMigrations(t, map[string]string{"0002_note.sql": "-- min_reader: 1\nALTER TABLE cubes ADD COLUMN note2 TEXT NOT NULL DEFAULT '';\n"})
	s, err := Open(dir, Options{Logger: quiet(), migrations: additive})
	if err != nil {
		t.Fatal(err)
	}
	if v, mr := s.SchemaVersion(); v != 2 || mr != 1 {
		t.Fatalf("after 0002: %d %d", v, mr)
	}
	if _, err := os.Stat(Path(dir) + ".before-2"); err == nil {
		t.Fatal("copy made before an additive migration")
	}
	s.Close()

	// The binary of version 1 opens the additive version 2.
	old, err := Open(dir, Options{Logger: quiet(), migrations: testMigrations(t, nil), readerVersion: 1})
	if err != nil {
		t.Fatalf("older reader, additive file: %v", err)
	}
	if v, _ := old.SchemaVersion(); v != 2 {
		t.Fatalf("version %d", v)
	}
	old.Close()

	// 0003 changes a meaning: min_reader 3, and a copy first.
	breaking := testMigrations(t, map[string]string{
		"0002_note.sql":   "-- min_reader: 1\nALTER TABLE cubes ADD COLUMN note2 TEXT NOT NULL DEFAULT '';\n",
		"0003_rename.sql": "-- min_reader: 3\nALTER TABLE cubes RENAME COLUMN note2 TO note3;\n",
	})
	s, err = Open(dir, Options{Logger: quiet(), migrations: breaking})
	if err != nil {
		t.Fatal(err)
	}
	if v, mr := s.SchemaVersion(); v != 3 || mr != 3 {
		t.Fatalf("after 0003: %d %d", v, mr)
	}
	s.Close()
	copyDir := t.TempDir()
	b, err := os.ReadFile(Path(dir) + ".before-3")
	if err != nil {
		t.Fatalf("no copy before 0003: %v", err)
	}
	os.WriteFile(Path(copyDir), b, 0o600)
	os.WriteFile(KeyPath(copyDir), mustRead(t, KeyPath(dir)), 0o600)
	cp, err := Open(copyDir, Options{Logger: quiet(), migrations: testMigrations(t, nil), readerVersion: 2})
	if err != nil {
		t.Fatalf("the copy: %v", err)
	}
	if v, mr := cp.SchemaVersion(); v != 2 || mr != 1 {
		t.Fatalf("copy at %d %d", v, mr)
	}
	cp.Close()

	// The binary of version 2 refuses version 3 and names the copy.
	_, err = Open(dir, Options{Logger: quiet(), migrations: additive, readerVersion: 2})
	if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), ".before-3") {
		t.Fatalf("older reader, breaking file: %v", err)
	}
	if _, err := OpenReadOnly(dir, Options{readerVersion: 2}); !errors.Is(err, ErrRefused) {
		t.Fatalf("read-only, older reader: %v", err)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Broken migration sets are refused before anything is applied.
func TestMigrationSetChecked(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"gap":          {"0003_x.sql": "-- min_reader: 1\nSELECT 1;"},
		"no min":       {"0002_x.sql": "SELECT 1;"},
		"falling min":  {"0002_x.sql": "-- min_reader: 2\nSELECT 1;", "0003_y.sql": "-- min_reader: 1\nSELECT 1;"},
		"min too high": {"0002_x.sql": "-- min_reader: 3\nSELECT 1;"},
	} {
		if _, err := loadMigrations(testMigrations(t, files)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Settings round-trip as JSON; a write bumps the version, a delete
// leaves a tombstone.
func TestSettings(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	if _, ok, err := s.SettingDuration(ctx, StaleAfterKey); ok || err != nil {
		t.Fatalf("unset: %v %v", ok, err)
	}
	if err := s.SetSetting(ctx, StaleAfterKey, "12h"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(ctx, StaleAfterKey, "36h"); err != nil {
		t.Fatal(err)
	}
	d, ok, err := s.SettingDuration(ctx, StaleAfterKey)
	if err != nil || !ok || d != 36*time.Hour {
		t.Fatalf("%v %v %v", d, ok, err)
	}
	var version int
	var origin string
	s.db.QueryRow(`SELECT version, origin FROM settings WHERE key = ?`, StaleAfterKey).Scan(&version, &origin)
	if version != 2 || origin != s.NodeID() {
		t.Fatalf("version %d origin %q", version, origin)
	}
	if err := s.DeleteSetting(ctx, StaleAfterKey); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Setting(ctx, StaleAfterKey); ok {
		t.Fatal("deleted setting still read")
	}
	var deleted sql.NullString
	s.db.QueryRow(`SELECT deleted_at FROM settings WHERE key = ?`, StaleAfterKey).Scan(&deleted)
	if !deleted.Valid {
		t.Fatal("no tombstone")
	}
	all, _ := s.Settings(ctx)
	keys := []string{}
	for _, st := range all {
		keys = append(keys, st.Key)
	}
	if strings.Join(keys, ",") != NodeIDKey+","+TenantIDKey {
		t.Fatalf("settings %v", keys)
	}
}

// A consistent copy opens as a store of its own.
func TestBackupAndInfo(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	ctx := context.Background()
	if _, err := s.AddCube(ctx, Cube{Serial: "keq0000001", Configured: true}); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "copy.db")
	if err := s.Backup(ctx, dst); err != nil {
		t.Fatal(err)
	}
	if err := s.Backup(ctx, dst); err == nil {
		t.Fatal("backup over an existing file")
	}
	cp, err := OpenReadOnly(filepath.Dir(dst), Options{})
	if err == nil {
		cp.Close()
		t.Fatal("copy.db opened as massimo.db")
	}
	os.Rename(dst, Path(filepath.Dir(dst)))
	cp, err = OpenReadOnly(filepath.Dir(dst), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	in, err := cp.Info(ctx)
	if err != nil || in.Counts["cubes"] != 1 || in.Counts["roles"] != 6 || in.Schema != SchemaVersion() || in.NodeID != s.NodeID() {
		t.Fatalf("info %+v %v", in, err)
	}
	if err := cp.SetSetting(ctx, "x", 1); err == nil {
		t.Fatal("write on a read-only store")
	}
}

// A file left at version 1 by v0.7.0-alpha.1 is migrated to version 2
// by the next binary, additively: min_reader stays 1, and the columns
// and the table of 0002 are there.
func TestMigratesAVersionOneFile(t *testing.T) {
	dir := t.TempDir()
	s1, err := Open(dir, Options{Version: "v0.7.0-alpha.1", Logger: quiet(), readerVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := s1.SchemaVersion(); v != 1 {
		t.Fatalf("version %d after the first open", v)
	}
	s1.Close()
	s := open(t, dir)
	if v, mr := s.SchemaVersion(); v != SchemaVersion() || v < 2 || mr != 1 {
		t.Fatalf("schema %d min_reader %d", v, mr)
	}
	has := func(table, col string) bool {
		var n int
		s.db.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name = ?`, table, col).Scan(&n)
		return n == 1
	}
	if !has("users", "auth_version") || !has("audit", "command_id") || !has("commands", "idempotency_key") {
		t.Fatal("the columns of 0002 are missing")
	}
	if _, err := os.Stat(Path(dir) + ".before-2"); err == nil {
		t.Fatal("an additive migration made a copy")
	}
	in, err := s.Info(context.Background())
	if err != nil || in.Counts["commands"] != 0 || in.Schema != SchemaVersion() {
		t.Fatalf("info %+v %v", in, err)
	}
}
