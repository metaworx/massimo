// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"testing"
)

// A provider is checked as it is written: its key, its issuer, its
// client id and its scopes; it is changed but for its key, and its
// removal frees the key and deletes its identities (AP Oidc v1.0,
// Block 1).
func TestProviders(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	good := AuthProvider{Key: "Cloud", Name: "Nextcloud", Issuer: "https://cloud.example.com", ClientID: "massimo", ClientSecret: "s3cret", Enabled: true}
	for name, bad := range map[string]func(p *AuthProvider){
		"key":          func(p *AuthProvider) { p.Key = "1cloud" },
		"http":         func(p *AuthProvider) { p.Issuer = "http://cloud.example.com" },
		"query":        func(p *AuthProvider) { p.Issuer = "https://cloud.example.com/?x=1" },
		"relative":     func(p *AuthProvider) { p.Issuer = "cloud.example.com" },
		"client":       func(p *AuthProvider) { p.ClientID = " " },
		"scopes":       func(p *AuthProvider) { p.Scopes = "email profile" },
		"domain":       func(p *AuthProvider) { p.AutoDomains = []string{"a@example.com"} },
		"unknown role": func(p *AuthProvider) { p.AutoRole = "nope" },
	} {
		p := good
		bad(&p)
		if _, err := s.AddProvider(ctx, p, 0); err == nil {
			t.Errorf("%s: accepted %+v", name, p)
		}
	}
	local := good
	local.Key, local.Issuer = "local", "http://127.0.0.1:8123/realm"
	if _, err := s.AddProvider(ctx, local, 0); err != nil {
		t.Fatalf("http on loopback: %v", err)
	}
	p, err := s.AddProvider(ctx, good, 0)
	if err != nil || p.Key != "cloud" || p.Scopes != DefaultScopes || p.ClientSecret != "s3cret" || p.AutoRole != "" || !p.Enabled || p.AutoCreate {
		t.Fatalf("add: %+v %v", p, err)
	}
	if _, err := s.AddProvider(ctx, good, 0); !errors.Is(err, ErrProviderExists) {
		t.Fatalf("twice: %v", err)
	}
	p, err = s.UpdateProvider(ctx, "cloud", func(p *AuthProvider) error {
		p.Key = "other"
		p.AutoCreate, p.AutoRole, p.AutoDomains = true, RoleGuest, []string{" Example.COM ", "example.com", "example.org"}
		return nil
	}, 0)
	if err != nil || p.Key != "cloud" || !p.AutoCreate || p.AutoRole != RoleGuest || len(p.AutoDomains) != 2 || p.Version != 2 {
		t.Fatalf("update: %+v %v", p, err)
	}
	if !p.AllowsDomain("anna@example.com") || !p.AllowsDomain("bob@EXAMPLE.org") || p.AllowsDomain("eve@example.net") || p.AllowsDomain("nobody") {
		t.Fatal("the domains")
	}
	if list, _ := s.Providers(ctx); len(list) != 2 || list[0].Key != "cloud" || list[1].Key != "local" {
		t.Fatalf("list: %+v", list)
	}
	anna, _ := s.AddUser(ctx, User{Name: "anna"}, "", 0)
	if _, err := s.AddIdentity(ctx, anna.ID, "cloud", "anna@example.com", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProvider(ctx, "cloud", 0); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Provider(ctx, "cloud"); ok {
		t.Fatal("removed, still found")
	}
	if ids, _ := s.Identities(ctx, anna.ID); len(ids) != 0 {
		t.Fatalf("its identities: %+v", ids)
	}
	var n int
	s.db.QueryRowContext(ctx, `SELECT count(*) FROM user_identities`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d identity rows left", n)
	}
	if err := s.DeleteProvider(ctx, "cloud", 0); !errors.Is(err, ErrNoProvider) {
		t.Fatalf("remove twice: %v", err)
	}
	if _, err := s.AddProvider(ctx, good, 0); err != nil {
		t.Fatalf("the key again: %v", err)
	}
}

// An e-mail assigned is a pre-link: the first sign-in with it binds it
// to the subject, which alone finds the user from then on; a waiting
// e-mail is one per provider; a removed user's identities go with it;
// a user created for a subject takes a free name.
func TestIdentities(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	p, err := s.AddProvider(ctx, AuthProvider{Key: "cloud", Issuer: "https://cloud.example.com", ClientID: "m", Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	anna, _ := s.AddUser(ctx, User{Name: "anna"}, "", 0)
	bob, _ := s.AddUser(ctx, User{Name: "bob"}, "", 0)
	for _, bad := range []string{"", "anna", "@example.com", "anna@", "a b@example.com"} {
		if _, err := s.AddIdentity(ctx, anna.ID, "cloud", bad, 0); !errors.Is(err, ErrBadEmail) {
			t.Errorf("e-mail %q: %v", bad, err)
		}
	}
	if _, err := s.AddIdentity(ctx, anna.ID, "nope", "anna@example.com", 0); !errors.Is(err, ErrNoProvider) {
		t.Fatalf("an unknown provider: %v", err)
	}
	pre, err := s.AddIdentity(ctx, anna.ID, "cloud", " Anna@Example.com ", anna.ID)
	if err != nil || pre.Email != "anna@example.com" || pre.Bound() || pre.Provider != "cloud" {
		t.Fatalf("pre-link: %+v %v", pre, err)
	}
	if _, err := s.AddIdentity(ctx, bob.ID, "cloud", "ANNA@example.com", 0); !errors.Is(err, ErrIdentityExists) {
		t.Fatalf("the same e-mail waiting twice: %v", err)
	}
	if _, _, ok, _ := s.UserBySubject(ctx, p.ID, "sub-anna"); ok {
		t.Fatal("found before the binding")
	}
	if _, _, ok, _ := s.BindIdentity(ctx, p.ID, "other@example.com", "sub-x"); ok {
		t.Fatal("another e-mail bound")
	}
	u, i, ok, err := s.BindIdentity(ctx, p.ID, "anna@EXAMPLE.com", "sub-anna")
	if err != nil || !ok || u.ID != anna.ID || i.Subject != "sub-anna" || i.BoundAt.IsZero() {
		t.Fatalf("bind: %+v %+v %v %v", u, i, ok, err)
	}
	if _, _, ok, _ := s.BindIdentity(ctx, p.ID, "anna@example.com", "sub-other"); ok {
		t.Fatal("a bound pre-link bound again")
	}
	if u, _, ok, _ := s.UserBySubject(ctx, p.ID, "sub-anna"); !ok || u.ID != anna.ID {
		t.Fatalf("by subject: %+v %v", u, ok)
	}
	// Bound, the e-mail may wait again, for another user.
	if _, err := s.AddIdentity(ctx, bob.ID, "cloud", "anna@example.com", 0); err != nil {
		t.Fatalf("the e-mail once bound: %v", err)
	}
	created, ci, err := s.AddIdentityUser(ctx, User{Name: "anna", DisplayName: "Anna B.", Role: RoleViewer}, p.ID, "sub-anna-b", "anna.b@example.com")
	if err != nil || created.Name != "anna-2" || created.DisplayName != "Anna B." || created.Role != RoleViewer || created.HasPassword || ci.Subject != "sub-anna-b" {
		t.Fatalf("created: %+v %+v %v", created, ci, err)
	}
	if u, _, ok, _ := s.UserBySubject(ctx, p.ID, "sub-anna-b"); !ok || u.ID != created.ID {
		t.Fatal("the created user by its subject")
	}
	if ids, _ := s.Identities(ctx, anna.ID); len(ids) != 1 || ids[0].ID != i.ID {
		t.Fatalf("anna's: %+v", ids)
	}
	if err := s.DeleteIdentity(ctx, bob.ID, i.ID); !errors.Is(err, ErrNoIdentity) {
		t.Fatalf("another user's identity: %v", err)
	}
	if err := s.DeleteUser(ctx, "anna", 0); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, _ := s.UserBySubject(ctx, p.ID, "sub-anna"); ok {
		t.Fatal("a removed user found by its subject")
	}
	var n int
	s.db.QueryRowContext(ctx, `SELECT count(*) FROM user_identities WHERE user_id = ?`, anna.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("%d identities of the removed user", n)
	}
}
