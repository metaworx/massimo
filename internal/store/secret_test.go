// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
)

// The client secrets sealed (AP Oidc v1.1, Block 9, D20-A): the file
// holds the box, the row the secret; secret.key is made with the first
// secret, 0600, never by a store opened read-only; a plain secret of
// v0.15.0-alpha.1 is sealed at the next open; a box copied to another
// row, or opened without the key, is lost, and a change that does not
// write the secret again keeps it as kept, which the key opens again.
func TestSealedSecrets(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s := open(t, dir)
	if _, err := os.Stat(SecretKeyPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("a key before any secret: %v", err)
	}
	cloud, err := s.AddProvider(ctx, AuthProvider{Key: "cloud", Issuer: "https://cloud.example.com", ClientID: "m", ClientSecret: "s3cret", Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if cloud.ClientSecret != "s3cret" || cloud.SecretLost {
		t.Fatalf("the row: %+v", cloud)
	}
	kept := func(key string) string {
		var v string
		if err := s.db.QueryRowContext(ctx, `SELECT client_secret FROM auth_providers WHERE key = ? AND deleted_at IS NULL`, key).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	if v := kept("cloud"); !strings.HasPrefix(v, "v1:") || strings.Contains(v, "s3cret") {
		t.Fatalf("in the file: %q", v)
	}
	fi, err := os.Stat(SecretKeyPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("secret.key: %v", fi.Mode())
	}
	none, _ := s.AddProvider(ctx, AuthProvider{Key: "none", Issuer: "https://none.example.com", ClientID: "m"}, 0)
	if none.ClientSecret != "" || kept("none") != "" {
		t.Fatalf("no secret: %+v %q", none, kept("none"))
	}

	// a box copied to another row does not open
	other, _ := s.AddProvider(ctx, AuthProvider{Key: "other", Issuer: "https://other.example.com", ClientID: "m", ClientSecret: "x"}, 0)
	s.db.ExecContext(ctx, `UPDATE auth_providers SET client_secret = ? WHERE id = ?`, kept("cloud"), other.ID)
	if p, _, _ := s.Provider(ctx, "other"); !p.SecretLost || p.ClientSecret != "" {
		t.Fatalf("a copied box: %+v", p)
	}

	// a plain secret left from v0.15.0-alpha.1, sealed at the next open
	s.db.ExecContext(ctx, `UPDATE auth_providers SET client_secret = 'plain' WHERE id = ?`, other.ID)
	s.Close()
	ro, err := OpenReadOnly(dir, Options{Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	if p, _, _ := ro.Provider(ctx, "other"); p.ClientSecret != "plain" {
		t.Fatalf("plain, read-only: %+v", p)
	}
	ro.Close()
	s = open(t, dir)
	if v := kept("other"); !strings.HasPrefix(v, "v1:") {
		t.Fatalf("sealed at the open: %q", v)
	}
	if p, _, _ := s.Provider(ctx, "other"); p.ClientSecret != "plain" {
		t.Fatalf("opened: %+v", p)
	}

	// the key lost: the secret too, kept by a change that does not write it
	s.Close()
	key, _ := os.ReadFile(SecretKeyPath(dir))
	os.Remove(SecretKeyPath(dir))
	s = open(t, dir)
	p, _, _ := s.Provider(ctx, "cloud")
	if !p.SecretLost || p.ClientSecret != "" {
		t.Fatalf("without the key: %+v", p)
	}
	if _, err := s.UpdateProvider(ctx, "cloud", func(p *AuthProvider) error { p.Name = "Cloud"; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	s.Close()
	os.WriteFile(SecretKeyPath(dir), key, 0o600)
	s = open(t, dir)
	if p, _, _ := s.Provider(ctx, "cloud"); p.ClientSecret != "s3cret" || p.Name != "Cloud" {
		t.Fatalf("the key found again: %+v", p)
	}

	// read-only never makes a key
	dir2 := t.TempDir()
	s2 := open(t, dir2)
	s2.AddProvider(ctx, AuthProvider{Key: "c", Issuer: "https://c.example.com", ClientID: "m"}, 0)
	s2.db.ExecContext(ctx, `UPDATE auth_providers SET client_secret = 'plain'`)
	s2.Close()
	ro, err = OpenReadOnly(dir2, Options{Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	ro.Close()
	if _, err := os.Stat(SecretKeyPath(dir2)); !os.IsNotExist(err) {
		t.Fatalf("a read-only store made the key: %v", err)
	}
}
