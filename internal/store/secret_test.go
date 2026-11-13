// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"encoding/pem"
	"os"
	"strings"
	"testing"
)

// The client secrets sealed (AP Oidc v1.1, Block 9; AP SealedSecrets
// v1.0, Block 1): the file holds the box, the row the secret, sealed with
// a key derived from opaque.key and no key file of its own; a plain
// secret of v0.15.0-alpha.1 is sealed at the next open; a box copied to
// another row, or opened with another opaque.key, is lost, and a change
// that does not write the secret again keeps it as kept; a store opened
// read-only opens them.
func TestSealedSecrets(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s := open(t, dir)
	cloud, err := s.AddProvider(ctx, AuthProvider{Key: "cloud", Issuer: "https://cloud.example.com", ClientID: "m", ClientSecret: "s3cret", Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if cloud.ClientSecret != "s3cret" || cloud.SecretLost {
		t.Fatalf("the row: %+v", cloud)
	}
	kept := func(s *Store, key string) string {
		var v string
		if err := s.db.QueryRowContext(ctx, `SELECT client_secret FROM auth_providers WHERE key = ? AND deleted_at IS NULL`, key).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	if v := kept(s, "cloud"); !strings.HasPrefix(v, "v2:") || strings.Contains(v, "s3cret") {
		t.Fatalf("in the file: %q", v)
	}
	if _, err := os.Stat(SecretKeyPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("a secret.key made: %v", err)
	}
	none, _ := s.AddProvider(ctx, AuthProvider{Key: "none", Issuer: "https://none.example.com", ClientID: "m"}, 0)
	if none.ClientSecret != "" || kept(s, "none") != "" {
		t.Fatalf("no secret: %+v %q", none, kept(s, "none"))
	}

	// a box copied to another row does not open
	other, _ := s.AddProvider(ctx, AuthProvider{Key: "other", Issuer: "https://other.example.com", ClientID: "m", ClientSecret: "x"}, 0)
	s.db.ExecContext(ctx, `UPDATE auth_providers SET client_secret = ? WHERE id = ?`, kept(s, "cloud"), other.ID)
	if p, _, _ := s.Provider(ctx, "other"); !p.SecretLost || p.ClientSecret != "" {
		t.Fatalf("a copied box: %+v", p)
	}

	// a plain secret left from v0.15.0-alpha.1, sealed at the next open;
	// read-only, it opens as it is, and the sealed one too
	s.db.ExecContext(ctx, `UPDATE auth_providers SET client_secret = 'plain' WHERE id = ?`, other.ID)
	s.Close()
	ro, err := OpenReadOnly(dir, Options{Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	if p, _, _ := ro.Provider(ctx, "other"); p.ClientSecret != "plain" {
		t.Fatalf("plain, read-only: %+v", p)
	}
	if p, _, _ := ro.Provider(ctx, "cloud"); p.ClientSecret != "s3cret" {
		t.Fatalf("sealed, read-only: %+v", p)
	}
	ro.Close()
	s = open(t, dir)
	if v := kept(s, "other"); !strings.HasPrefix(v, "v2:") {
		t.Fatalf("sealed at the open: %q", v)
	}
	if p, _, _ := s.Provider(ctx, "other"); p.ClientSecret != "plain" {
		t.Fatalf("opened: %+v", p)
	}

	// another opaque.key: the secrets lost, kept by a change that does
	// not write them, and opened again with the key found again
	s.Close()
	opq, _ := os.ReadFile(OpaqueKeyPath(dir))
	os.Remove(OpaqueKeyPath(dir))
	s = open(t, dir)
	p, _, _ := s.Provider(ctx, "cloud")
	if !p.SecretLost || p.ClientSecret != "" {
		t.Fatalf("with another opaque.key: %+v", p)
	}
	if _, err := s.UpdateProvider(ctx, "cloud", func(p *AuthProvider) error { p.Name = "Cloud"; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	s.Close()
	os.WriteFile(OpaqueKeyPath(dir), opq, 0o600)
	s = open(t, dir)
	if p, _, _ := s.Provider(ctx, "cloud"); p.ClientSecret != "s3cret" || p.Name != "Cloud" {
		t.Fatalf("the key found again: %+v", p)
	}
}

// A secret sealed with secret.key by a build of main before AP
// SealedSecrets (D35-A): opened with it at the next open and sealed
// again with the derived key, the file then retired; one whose
// secret.key is another's stays lost, and the file stays.
func TestLegacySecretKey(t *testing.T) {
	ctx := context.Background()
	legacy := func(t *testing.T, dir string, s *Store, key, plain string) []byte {
		t.Helper()
		k := make32()
		p, _, _ := s.Provider(ctx, key)
		box, err := sealBox(k, p.UUID, plain)
		if err != nil {
			t.Fatal(err)
		}
		s.db.ExecContext(ctx, `UPDATE auth_providers SET client_secret = ? WHERE id = ?`, legacyPrefix+box, p.ID)
		return k
	}
	writeKey := func(dir string, k []byte) {
		os.WriteFile(SecretKeyPath(dir), pem.EncodeToMemory(&pem.Block{Type: secretPEM, Bytes: k}), 0o600)
	}

	dir := t.TempDir()
	s := open(t, dir)
	s.AddProvider(ctx, AuthProvider{Key: "cloud", Issuer: "https://cloud.example.com", ClientID: "m"}, 0)
	writeKey(dir, legacy(t, dir, s, "cloud", "old"))
	s.Close()
	s = open(t, dir)
	var v string
	s.db.QueryRowContext(ctx, `SELECT client_secret FROM auth_providers WHERE key = 'cloud'`).Scan(&v)
	if p, _, _ := s.Provider(ctx, "cloud"); p.ClientSecret != "old" || !strings.HasPrefix(v, "v2:") {
		t.Fatalf("sealed again: %+v %q", p, v)
	}
	if _, err := os.Stat(SecretKeyPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("secret.key not retired: %v", err)
	}
	if _, err := os.Stat(SecretKeyPath(dir) + retiredSuffix); err != nil {
		t.Fatalf("secret.key.retired: %v", err)
	}

	dir2 := t.TempDir()
	s2 := open(t, dir2)
	s2.AddProvider(ctx, AuthProvider{Key: "cloud", Issuer: "https://cloud.example.com", ClientID: "m"}, 0)
	legacy(t, dir2, s2, "cloud", "old")
	another := make32()
	another[0] ^= 0xff
	writeKey(dir2, another)
	s2.Close()
	s2 = open(t, dir2)
	if p, _, _ := s2.Provider(ctx, "cloud"); !p.SecretLost {
		t.Fatalf("another's secret.key: %+v", p)
	}
	if _, err := os.Stat(SecretKeyPath(dir2)); err != nil {
		t.Fatalf("secret.key retired though a secret needs it: %v", err)
	}
}

// make32 is 32 random bytes, a test's key.
func make32() []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i*7 + 3)
	}
	return b
}
