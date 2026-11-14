// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"slices"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
)

// The role integration in the store is the ceiling the server knows; no
// regular user is given it, through the API or by a change of role.
func TestIntegrationRoleIsTheCeiling(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	r, ok, err := s.opts.Store.Role(t.Context(), store.RoleIntegration)
	if err != nil || !ok {
		t.Fatal(err)
	}
	want := slices.Clone(integrationCeiling)
	slices.Sort(want)
	if !slices.Equal(r.Capabilities, want) {
		t.Fatalf("the store's integration %v, the server's ceiling %v", r.Capabilities, want)
	}
	for _, c := range []string{"users", "passwords", "roles", "roles.disable", "capabilities", "auth", "integrations"} {
		if slices.Contains(integrationCeiling, c) {
			t.Errorf("the ceiling holds %s", c)
		}
	}
	if code, text := callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name":"ha","role":"integration"}`); code != 403 || errorOf(text) != "the role integration is an integration's and its users' alone" {
		t.Fatalf("a user with the role integration: %d %s", code, text)
	}
	callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name":"anna","role":"viewer"}`)
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/users/anna", "secret", `{"role":"integration"}`); code != 403 {
		t.Fatalf("anna given the role integration: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/roles/integration", "secret", `{"capabilities":["read"]}`); code < 400 {
		t.Fatalf("the role integration changed: %d", code)
	}
}
