// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// Migration 0021 makes user_identities again for providers and
// integrations, keeping every identity; the role integration's users keep
// their set in integration-v0, and integration is fixed at the ceiling.
func TestMigration21(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s20 := openAt(t, dir, 20)
	p, err := s20.AddProvider(ctx, AuthProvider{Key: "cloud", Issuer: "https://id.example.com", ClientID: "massimo", Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	anna, err := s20.AddUser(ctx, User{Name: "anna", Role: RoleViewer}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s20.AddSubjectIdentity(ctx, anna.ID, p.Key, "sub-1", 0); err != nil {
		t.Fatal(err)
	}
	// A key made by hand for an integration, its role narrowed by the owner.
	key, err := s20.AddUser(ctx, User{Name: "loxone", Role: RoleIntegration}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s20.db.ExecContext(ctx, `DELETE FROM role_capabilities WHERE capability = 'lock' AND role_id = (SELECT id FROM roles WHERE key = 'integration')`); err != nil {
		t.Fatal(err)
	}
	s20.Close()

	s := open(t, dir)
	ids, err := s.Identities(ctx, anna.ID)
	if err != nil || len(ids) != 1 || ids[0].Subject != "sub-1" || ids[0].Provider != "cloud" {
		t.Fatalf("the OIDC identity after the migration: %+v %v", ids, err)
	}
	if u, _, _, err := s.UserBySubject(ctx, p.ID, "sub-1"); err != nil || u.ID != anna.ID {
		t.Fatalf("sign-in by subject: %+v %v", u, err)
	}
	u, ok, err := s.UserByID(ctx, key.ID)
	if err != nil || !ok || u.Role != RoleIntegrationV0 {
		t.Fatalf("the integration key's user: %+v %v %v", u, ok, err)
	}
	v0, _, _ := s.Role(ctx, RoleIntegrationV0)
	if !slices.Equal(v0.Capabilities, []string{"configure", "read", "setpoint"}) || v0.Builtin {
		t.Fatalf("integration-v0: %+v", v0)
	}
	in, _, _ := s.Role(ctx, RoleIntegration)
	if !slices.Contains(in.Capabilities, "update") || slices.Contains(in.Capabilities, "users") || slices.Contains(in.Capabilities, "passwords") || !in.Builtin {
		t.Fatalf("integration: %+v", in)
	}
	// An identity names a provider or an integration, never both or none.
	for _, q := range []string{
		`INSERT INTO user_identities (uuid, user_id, created_at) VALUES ('u1', 1, '2026-11-14T00:00:00Z')`,
		`INSERT INTO user_identities (uuid, user_id, provider_id, integration_id, created_at) VALUES ('u2', 1, 1, 1, '2026-11-14T00:00:00Z')`,
	} {
		if _, err := s.db.ExecContext(ctx, q); err == nil {
			t.Fatalf("taken: %s", q)
		}
	}
}

// A store without the role integration's users gets no integration-v0.
func TestMigration21WithoutIntegrationUsers(t *testing.T) {
	s := open(t, t.TempDir())
	if _, ok, _ := s.Role(context.Background(), RoleIntegrationV0); ok {
		t.Fatal("integration-v0 made for nobody")
	}
}

// The role integration is fixed, as owner is.
func TestIntegrationRoleFixed(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	if _, err := s.UpdateRole(ctx, RoleIntegration, func(r *Role) error { r.Capabilities = []string{"read"}; return nil }, 0); !errors.Is(err, ErrImmutable) {
		t.Fatalf("update: %v", err)
	}
	if err := s.SetRoleDisabled(ctx, RoleIntegration, true, 0); !errors.Is(err, ErrImmutable) {
		t.Fatalf("disable: %v", err)
	}
	if err := s.DeleteRole(ctx, RoleIntegration, 0); !errors.Is(err, ErrImmutable) {
		t.Fatalf("delete: %v", err)
	}
}
