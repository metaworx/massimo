// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// restoreSide is a box a backup is restored onto: a catalogue with a key
// of its own, its env file and its TLS place in a directory of their
// own.
type restoreSide struct {
	c     *Catalogue
	env   string
	place TLSPlace
}

func newRestoreSide(t *testing.T) restoreSide {
	t.Helper()
	st := storekit.Open(t, "")
	r, _ := keyOf(t, KindX25519, "the new box")
	if _, err := st.AddServerBackupKey(context.Background(), store.ServerBackupKey{Label: r.Label, Recipient: r.Line, Fingerprint: r.Fingerprint, Kind: string(r.Kind)}, "", 0); err != nil {
		t.Fatal(err)
	}
	d := t.TempDir()
	env := filepath.Join(d, "env")
	if err := os.WriteFile(env, []byte("MASSIMO_LISTEN=:9000\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	return restoreSide{c: &Catalogue{Store: st, EnvFile: env, Version: "v0.0.0-test", Location: time.UTC, Log: slog.New(slog.DiscardHandler)}, env: env,
		place: TLSPlace{Cert: filepath.Join(d, "tls", "cert.pem"), Key: filepath.Join(d, "tls", "key.pem"), ACMEDir: filepath.Join(d, "acme")}}
}

// oldBox is a box with a Cube, rooms, a user and a provider's secret,
// backed up with its env file and TLS files for a key whose file is
// returned.
func oldBox(t *testing.T) (*store.Store, string, []byte) {
	t.Helper()
	st := testStore(t)
	if _, err := st.AddProvider(context.Background(), store.AuthProvider{Key: "cloud", Issuer: "https://cloud.example.com", ClientID: "massimo", ClientSecret: "provider secret", Enabled: true}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddUser(context.Background(), store.User{Name: "olga", Role: store.RoleOwner}, "olga's secret", 0); err != nil {
		t.Fatal(err)
	}
	k, err := GenerateKey(KindX25519)
	if err != nil {
		t.Fatal(err)
	}
	k.Recipient.Label = "safe"
	envFile, tls := sideFiles(t)
	path, _ := write(t, st, Inputs{EnvFile: envFile, TLSFiles: tls}, k.Recipient)
	return st, path, k.File(time.Now())
}

// noKeyWritten fails where any file under dir holds the key's secret.
func noKeyWritten(t *testing.T, dir string, key []byte) {
	t.Helper()
	secret := key[bytes.LastIndex(bytes.TrimSpace(key), []byte("\n"))+1:]
	secret = bytes.TrimSpace(secret)
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if b, err := os.ReadFile(p); err == nil && bytes.Contains(b, secret) {
			t.Errorf("%s holds the key", p)
		}
		return nil
	})
}

// A backup of another box onto a new one: the plan first, which changes
// nothing; then every scope, the env file and the TLS files, a pinned
// backup taken first; opaque.key comes with the users, node.key stays
// this box's, the files replaced stay beside, the key is written
// nowhere; after the restart the provider's secret opens and the owner
// signs in with the old password. With the identity, node.key too.
func TestRestoreFromAnotherBox(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	old, path, key := oldBox(t)
	side := newRestoreSide(t)
	c := side.c
	dir := c.Store.Dir()
	nodeKey, _ := os.ReadFile(store.KeyPath(dir))
	o := RestoreOptions{File: path, Key: key, TLSPlace: side.place, UserID: 0}

	p, err := c.Plan(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	if p.ThisBox || p.Migrated || p.Refused != "" || len(p.Cubes) != 1 || p.Cubes[0].Serial != "KEQ0000000" || p.Cubes[0].Live || !p.Env || p.TLS != 2 ||
		!slices.Equal(p.Selection.Scopes, Scopes) || p.NodeID != old.NodeID() || len(p.Report.Missing) != 0 {
		t.Fatalf("plan %+v", p)
	}
	if cs, _ := c.Store.Cubes(ctx); len(cs) != 0 {
		t.Fatal("the plan changed the store")
	}
	if bs, _ := c.Store.ServerBackups(ctx); len(bs) != 0 {
		t.Fatal("the plan took a backup")
	}

	o.Scopes, o.Env, o.TLS = []string{ScopeAll}, true, true
	res, err := c.Restore(ctx, o)
	if err != nil {
		t.Fatalf("%v %+v", err, res.Report)
	}
	if res.Before.Trigger != string(TriggerBeforeRestore) || res.Before.PinnedUntil.IsZero() {
		t.Errorf("the backup first %+v", res.Before)
	}
	if b, _ := os.ReadFile(store.KeyPath(dir)); !bytes.Equal(b, nodeKey) {
		t.Error("node.key changed without the identity")
	}
	oldOpaque, _ := os.ReadFile(store.OpaqueKeyPath(old.Dir()))
	if b, _ := os.ReadFile(store.OpaqueKeyPath(dir)); !bytes.Equal(b, oldOpaque) {
		t.Error("opaque.key did not come with the users")
	}
	if _, err := os.Stat(store.OpaqueKeyPath(dir) + BeforeRestoreSuffix); err != nil {
		t.Errorf("the replaced opaque.key: %v", err)
	}
	if b, _ := os.ReadFile(side.env); string(b) != "MASSIMO_LISTEN=:8080\n" {
		t.Errorf("env %q", b)
	}
	if b, _ := os.ReadFile(side.env + BeforeRestoreSuffix); string(b) != "MASSIMO_LISTEN=:9000\n" {
		t.Errorf("env before %q", b)
	}
	if b, _ := os.ReadFile(side.place.Cert); string(b) != "a certificate\n" {
		t.Errorf("cert %q", b)
	}
	if b, _ := os.ReadFile(side.place.Key); string(b) != "a key\n" {
		t.Errorf("key %q", b)
	}
	if len(res.Files) != 4 || len(res.Report.Cubes) != 1 {
		t.Errorf("files %+v, cubes %+v", res.Files, res.Report.Cubes)
	}
	noKeyWritten(t, dir, key)
	noKeyWritten(t, filepath.Dir(side.env), key)
	if left, _ := filepath.Glob(filepath.Join(dir, ".serverbackup-*")); len(left) > 0 {
		t.Errorf("left in the data directory: %v", left)
	}

	// The restart.
	c.Store.Close()
	st, err := store.Open(dir, store.Options{Version: "test", Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if p, ok, _ := st.Provider(ctx, "cloud"); !ok || p.ClientSecret != "provider secret" {
		t.Errorf("provider %+v", p)
	}
	if err := st.CheckPassword(ctx, "olga", "olga's secret"); err != nil {
		t.Errorf("olga: %v", err)
	}
	if bs, _ := st.ServerBackups(ctx); len(bs) != 1 || bs[0].Trigger != string(TriggerBeforeRestore) {
		t.Errorf("the catalogue %+v", bs)
	}

	// The identity, alone; the old box had no key of the server's
	// backups, and the settings took its none, so one is given again.
	c.Store = st
	nk, _ := keyOf(t, KindX25519, "again")
	if _, err := st.AddServerBackupKey(ctx, store.ServerBackupKey{Label: nk.Label, Recipient: nk.Line, Fingerprint: nk.Fingerprint, Kind: string(nk.Kind)}, "", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Restore(ctx, RestoreOptions{File: path, Key: key, Identity: true}); err != nil {
		t.Fatal(err)
	}
	oldNode, _ := os.ReadFile(store.KeyPath(old.Dir()))
	if b, _ := os.ReadFile(store.KeyPath(dir)); !bytes.Equal(b, oldNode) {
		t.Error("node.key not taken with the identity")
	}
}

// A backup of an older schema is migrated on its copy and restored; one
// of a newer schema is refused before it is opened; a wrong key and
// nothing to restore are refused.
func TestRestoreSchemas(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	old := testStore(t)
	snap := filepath.Join(t.TempDir(), "old")
	os.MkdirAll(snap, 0o700)
	if err := old.Backup(ctx, store.Path(snap)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Downgrade(snap, 26, store.Options{Version: "test", Logger: slog.New(slog.DiscardHandler)}); err != nil {
		t.Fatal(err)
	}
	db, err := os.ReadFile(store.Path(snap))
	if err != nil {
		t.Fatal(err)
	}
	opq, _ := os.ReadFile(store.OpaqueKeyPath(old.Dir()))
	node, _ := os.ReadFile(store.KeyPath(old.Dir()))
	k, _ := GenerateKey(KindX25519)
	r := k.Recipient
	r.Label = "safe"
	keyFile := k.File(time.Now())
	wrong, _ := GenerateKey(KindX25519)
	entries := []innerEntry{{name: store.FileName, data: db}, {name: store.OpaqueKeyFileName, data: opq}, {name: store.KeyFileName, data: node}}
	older, _ := craft(t, r, entries, nil, nil, nil)
	newer, _ := craft(t, r, entries, nil, nil, func(m *Manifest) { m.Schema.Version = 9999 })

	side := newRestoreSide(t)
	p, err := side.c.Plan(ctx, RestoreOptions{File: older, Key: keyFile, Scopes: []string{"cube:keq0000000"}})
	if err != nil || !p.Migrated || len(p.Cubes) != 1 || p.Refused != "" {
		t.Fatalf("older: %+v %v", p, err)
	}
	if _, err := side.c.Restore(ctx, RestoreOptions{File: older, Key: keyFile, Scopes: []string{"cube:KEQ0000000"}}); err != nil {
		t.Fatalf("older restored: %v", err)
	}
	if cs, _ := side.c.Store.Cubes(ctx); len(cs) != 1 {
		t.Fatalf("cubes %+v", cs)
	}
	if _, err := side.c.Plan(ctx, RestoreOptions{File: newer, Key: keyFile}); !errors.Is(err, ErrNewerSchema) {
		t.Errorf("newer: %v", err)
	}
	if _, err := side.c.Restore(ctx, RestoreOptions{File: older, Key: wrong.File(time.Now()), Scopes: []string{ScopeCubes}}); !errors.Is(err, ErrWrongKey) {
		t.Errorf("a wrong key: %v", err)
	}
	if _, err := side.c.Restore(ctx, RestoreOptions{File: older, Key: keyFile}); !errors.Is(err, errNothing) {
		t.Errorf("nothing: %v", err)
	}
}

// The check with the key opens a backup of the catalogue and finds its
// store sound, changing nothing; a wrong key is refused.
func TestCheckKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c, _ := catalogue(t)
	k, err := GenerateKey(KindX25519)
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := c.Store.ServerBackupKeys(ctx)
	c.Store.DeleteServerBackupKey(ctx, keys[0].UUID)
	if _, err := c.Store.AddServerBackupKey(ctx, store.ServerBackupKey{Label: "safe", Recipient: k.Recipient.Line, Fingerprint: k.Recipient.Fingerprint, Kind: string(k.Recipient.Kind)}, "", 0); err != nil {
		t.Fatal(err)
	}
	b, _, err := c.Make(ctx, MakeRequest{Trigger: TriggerManual})
	if err != nil {
		t.Fatal(err)
	}
	kc, err := c.CheckKey(ctx, RestoreOptions{Backup: b.UUID, Key: k.File(time.Now())})
	if err != nil || !kc.OK || !kc.ThisBox || len(kc.Files) < 3 || kc.Manifest.UUID != b.UUID {
		t.Fatalf("check %+v %v", kc, err)
	}
	other, _ := GenerateKey(KindX25519)
	if _, err := c.CheckKey(ctx, RestoreOptions{Backup: b.UUID, Key: other.File(time.Now())}); !errors.Is(err, ErrWrongKey) {
		t.Errorf("a wrong key: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(c.Store.Dir(), ".serverbackup-*")); len(left) > 0 {
		t.Errorf("left in the data directory: %v", left)
	}
}

// The scopes read: each, a Cube by its serial, all, and what is none.
func TestParseScopes(t *testing.T) {
	t.Parallel()
	sel, err := ParseScopes([]string{"history,users", " cube:keq0000001 "})
	if err != nil || !slices.Equal(sel.Scopes, []string{ScopeUsers, ScopeCubes, ScopeHistory}) || !slices.Equal(sel.Serials, []string{"KEQ0000001"}) {
		t.Fatalf("%+v %v", sel, err)
	}
	if sel, _ := ParseScopes([]string{"cube:KEQ0000001", "cubes"}); len(sel.Serials) != 0 {
		t.Errorf("cubes is every Cube: %+v", sel)
	}
	if sel, _ := ParseScopes([]string{"all"}); !slices.Equal(sel.Scopes, Scopes) {
		t.Errorf("all %+v", sel)
	}
	for _, bad := range []string{"identity", "cube:KEQ", "cube:KEQ000000!", "everything"} {
		if _, err := ParseScopes([]string{bad}); err == nil || !strings.Contains(err.Error(), "serverbackup:") {
			t.Errorf("%q: %v", bad, err)
		}
	}
}
