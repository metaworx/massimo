// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// What a store leaves to its first regular user (AP AccessEnds, Block
// 3): nothing on a new store; an integration with a key, by the name
// the owner gave it, one whose key may still be collected, an enabled
// provider, roles added, by their names, integration-v0 among them,
// default roles changed and disabled, and a capability turned off, each
// listed; a request still waiting, a refused and a revoked integration,
// an approval that lapsed uncollected, a disabled and a removed
// provider and a removed role not.
func TestOpenSetup(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	if set, err := s.OpenSetup(ctx); err != nil || !set.Empty() {
		t.Fatalf("a new store: %+v %v", set, err)
	}
	n := 0
	request := func(name string) Integration {
		t.Helper()
		n++
		r := haRequest(fmt.Sprintf("%032x", n))
		r.Name = name
		req, err := s.AddIntegrationRequest(ctx, r, fmt.Sprintf("CODE%02d", n))
		if err != nil {
			t.Fatal(err)
		}
		return req
	}
	approve := func(name string, collect bool) Integration {
		t.Helper()
		req := request(name)
		i, err := s.ApproveIntegration(ctx, req.UUID, []string{"read"}, "", 0)
		if err != nil {
			t.Fatal(err)
		}
		if collect {
			if _, _, err := s.CollectIntegrationKey(ctx, req.UUID, req.RequestCode); err != nil {
				t.Fatal(err)
			}
		}
		return i
	}
	kitchen := approve("Kitchen", true)
	if _, err := s.SetIntegration(ctx, kitchen.UUID, func(i *Integration) error { i.DisplayName = "Küche"; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	garage := approve("Garage", false)
	revoked := approve("Shed", true)
	if err := s.RevokeIntegration(ctx, revoked.UUID, 0); err != nil {
		t.Fatal(err)
	}
	refused := request("Attic")
	if err := s.RefuseIntegration(ctx, refused.UUID, 0); err != nil {
		t.Fatal(err)
	}
	request("Cellar") // still waiting
	for _, p := range []AuthProvider{
		{Key: "cloud", Name: "Nextcloud", Issuer: "https://cloud.example.com", ClientID: "massimo", ClientSecret: "s3cret", AutoCreate: true, Enabled: true},
		{Key: "off", Issuer: "https://off.example.com", ClientID: "massimo"},
		{Key: "gone", Issuer: "https://gone.example.com", ClientID: "massimo", Enabled: true},
	} {
		if _, err := s.AddProvider(ctx, p, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeleteProvider(ctx, "gone", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateRole(ctx, RoleViewer, func(r *Role) error { r.Capabilities = append(r.Capabilities, "users"); return nil }, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRoleDisabled(ctx, RoleGuest, true, 0); err != nil {
		t.Fatal(err)
	}
	// integration-v0 as a request may make it: listed as any other role
	for _, r := range []Role{{Key: "night", Name: "Night shift", Capabilities: []string{"read"}}, {Key: RoleIntegrationV0, Capabilities: []string{"read", "users"}},
		{Key: "gone", Capabilities: []string{"read"}}} {
		if _, err := s.AddRole(ctx, r, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeleteRole(ctx, "gone", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCapabilityDisabled(ctx, "update", true, 0); err != nil {
		t.Fatal(err)
	}
	set, err := s.OpenSetup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("integrations [{%s Garage} {%s Küche}] providers [{cloud Nextcloud true}] roles [{guest } {viewer } {integration-v0 } {night Night shift}] off [update]",
		garage.UUID, kitchen.UUID)
	if got := fmt.Sprintf("integrations %v providers %v roles %v off %v", set.Integrations, set.Providers, set.Roles, set.CapabilitiesOff); got != want || set.Empty() {
		t.Fatalf("listed:\n%s\nwant\n%s", got, want)
	}
	// Past the approval's time to collect its key, an integration without
	// one gives nothing any more; the one with a key stays.
	s.now = func() time.Time { return time.Now().Add(RequestLife + time.Minute) }
	set, err = s.OpenSetup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Integrations) != 1 || set.Integrations[0].UUID != kitchen.UUID {
		t.Fatalf("after the time to collect: %+v", set.Integrations)
	}
}

// The command line's add says whether its user is the store's first
// regular one (AP AccessEnds, D12), as the transaction that writes it
// counts them: first beside an integration's users alone, with or
// without a password, not beside another regular user, a disabled one
// too, and not for a user refused.
func TestAddCommandUserSaysFirst(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, c := range []struct {
		name     string
		password string
		before   *User
		first    bool
	}{
		{name: "beside an integration's users", password: "a-long-secret-1", first: true},
		{name: "keys only", first: true},
		{name: "beside a regular user", before: &User{Name: "bob", Role: RoleViewer}},
		{name: "beside a disabled user", password: "a-long-secret-1", before: &User{Name: "bob", Role: RoleOwner, Disabled: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := open(t, t.TempDir())
			req, err := s.AddIntegrationRequest(ctx, haRequest(instanceA), "CODE01")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.ApproveIntegration(ctx, req.UUID, []string{"read"}, "", 0); err != nil {
				t.Fatal(err)
			}
			if c.before != nil {
				if _, err := s.AddUser(ctx, *c.before, "", 0); err != nil {
					t.Fatal(err)
				}
			}
			u, first, err := s.AddCommandUser(ctx, User{Name: "anna", Role: RoleViewer}, c.password, 0)
			if err != nil || u.Name != "anna" || u.HasPassword != (c.password != "") || first != c.first {
				t.Fatalf("anna %+v, first %v, %v", u, first, err)
			}
			if _, first, err := s.AddCommandUser(ctx, User{Name: "anna", Role: RoleViewer}, "", 0); err == nil || first {
				t.Fatalf("anna again: first %v, %v", first, err)
			}
			if _, first, err := s.AddCommandUser(ctx, User{Name: "carl", Role: RoleViewer}, "", 0); err != nil || first {
				t.Fatalf("a user after anna: first %v, %v", first, err)
			}
		})
	}
}

// The command line's add counts the regular users in the transaction
// that writes its user (AP AccessEnds, D12): a regular user another
// process adds once the user is checked and its password registered,
// as the page's first owner may be while the registration runs, makes
// it not the first, with a password or with keys only.
func TestAddCommandUserCountsInItsTransaction(t *testing.T) {
	ctx := context.Background()
	t.Cleanup(func() { commandStep = nil })
	for _, password := range []string{"a-long-secret-1", ""} {
		dir := t.TempDir()
		s := open(t, dir)
		other := openFresh(t, dir)
		commandStep = func() {
			if _, err := other.AddUser(ctx, User{Name: "bob", Role: RoleOwner}, "", 0); err != nil {
				t.Error(err)
			}
		}
		u, first, err := s.AddCommandUser(ctx, User{Name: "anna", Role: RoleViewer}, password, 0)
		commandStep = nil
		if err != nil || u.Name != "anna" || first {
			t.Fatalf("password %q: anna %+v, first %v, %v", password, u, first, err)
		}
	}
}
