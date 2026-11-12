// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
)

// The providers through the API (AP Oidc v1.0, Block 3): the capability
// auth, which owner alone holds by default; the secret written, never
// answered nor audited; the role of the users a provider creates without
// an administrative capability; the key fixed; the removal.
func TestProvidersAPI(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	manager := userToken(t, st, "manny", store.RoleManager)
	if code, _ := callAPI(t, ts, "GET", "/api/v1/providers", manager, ""); code != 403 {
		t.Fatalf("a manager lists the providers: %d", code)
	}
	body := `{"key": "Cloud", "name": "Nextcloud", "issuer": "https://cloud.example.com", "client_id": "massimo", "client_secret": "s3cret"}`
	code, text := callAPI(t, ts, "POST", "/api/v1/providers", "secret", body)
	if code != 201 || strings.Contains(text, "s3cret") {
		t.Fatalf("add: %d %s", code, text)
	}
	var p providerView
	json.Unmarshal([]byte(text), &p)
	if p.Key != "cloud" || !p.HasSecret || !p.Enabled || p.AutoRole != store.RoleViewer || p.Callback != "api/v1/login/oidc/callback" {
		t.Fatalf("the provider: %+v", p)
	}
	for b, want := range map[string]int{
		body: 409,
		`{"key": "x", "issuer": "http://cloud.example.com", "client_id": "m"}`:                      400,
		`{"key": "x", "issuer": "https://c.example.com", "client_id": "m", "auto_role": "owner"}`:   400,
		`{"key": "x", "issuer": "https://c.example.com", "client_id": "m", "auto_role": "manager"}`: 400,
		`{"key": "x", "issuer": "https://c.example.com", "client_id": "m", "auto_role": "nope"}`:    400,
		`{"key": "x", "issuer": "https://c.example.com", "client_id": "m", "unknown": true}`:        400,
		`{"issuer": "https://c.example.com", "client_id": "m"}`:                                     400,
	} {
		if code, text := callAPI(t, ts, "POST", "/api/v1/providers", "secret", b); code != want {
			t.Errorf("add %s: %d %s, want %d", b, code, text, want)
		}
	}
	code, text = callAPI(t, ts, "PUT", "/api/v1/providers/cloud", "secret", `{"auto_create": true, "auto_role": "guest", "auto_domains": ["example.org"], "enabled": false}`)
	json.Unmarshal([]byte(text), &p)
	if code != 200 || !p.AutoCreate || p.AutoRole != store.RoleGuest || len(p.AutoDomains) != 1 || p.Enabled || !p.HasSecret {
		t.Fatalf("change: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/providers/cloud", "secret", `{"key": "other"}`); code != 400 {
		t.Fatalf("a new key: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/providers/cloud", "secret", `{"auto_role": "manager"}`); code != 400 {
		t.Fatalf("an administrative role: %d", code)
	}
	code, text = callAPI(t, ts, "PUT", "/api/v1/providers/cloud", "secret", `{"client_secret": ""}`)
	if json.Unmarshal([]byte(text), &p); code != 200 || p.HasSecret {
		t.Fatalf("the secret cleared: %d %s", code, text)
	}
	var list []providerView
	if getJSON(t, ts, "secret", "/api/v1/providers", &list); len(list) != 1 {
		t.Fatalf("list: %+v", list)
	}
	for _, r := range auditRows(t, ts, "secret", "?limit=50") {
		if strings.Contains(string(r.Detail), "s3cret") {
			t.Fatalf("the secret in the audit: %s", r.Detail)
		}
	}
	if code, _ := callAPI(t, ts, "DELETE", "/api/v1/providers/cloud", "secret", ""); code != 200 {
		t.Fatalf("remove: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", "/api/v1/providers/cloud", "secret", ""); code != 404 {
		t.Fatalf("remove twice: %d", code)
	}
}

// A user's identities through the API: assigned by e-mail, listed
// without the subject, removed; a guests holder reaches a guest's only,
// and nobody reaches its own without users.
func TestIdentitiesAPI(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	ctx := t.Context()
	if _, err := st.AddProvider(ctx, store.AuthProvider{Key: "cloud", Issuer: "https://cloud.example.com", ClientID: "m", Enabled: true}, 0); err != nil {
		t.Fatal(err)
	}
	guest, _, _ := st.Role(ctx, store.RoleGuest)
	if _, err := st.AddRole(ctx, store.Role{Key: "host", Capabilities: append(guest.Capabilities, "guests")}, 0); err != nil {
		t.Fatal(err)
	}
	host := userToken(t, st, "hal", "host")
	anna := userToken(t, st, "anna", store.RoleOperator)
	userToken(t, st, "gus", store.RoleGuest)

	code, text := callAPI(t, ts, "POST", "/api/v1/users/anna/identities", "secret", `{"provider": "cloud", "email": "Anna@Example.com"}`)
	var i identityView
	if json.Unmarshal([]byte(text), &i); code != 201 || i.Email != "anna@example.com" || i.Bound || i.Provider != "cloud" {
		t.Fatalf("assign: %d %s", code, text)
	}
	for b, want := range map[string]int{
		`{"provider": "cloud", "email": "anna@example.com"}`: 409,
		`{"provider": "nope", "email": "x@example.com"}`:     404,
		`{"provider": "cloud", "email": "not an e-mail"}`:    400,
		`{"provider": "cloud"}`:                              400,
	} {
		if code, text := callAPI(t, ts, "POST", "/api/v1/users/anna/identities", "secret", b); code != want {
			t.Errorf("assign %s: %d %s, want %d", b, code, text, want)
		}
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/users/anna/identities", anna, ""); code != 403 {
		t.Fatalf("its own, without users: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/users/anna/identities", host, `{"provider": "cloud", "email": "a2@example.com"}`); code != 403 {
		t.Fatalf("a guests holder on an operator: %d", code)
	}
	if code, text := callAPI(t, ts, "POST", "/api/v1/users/gus/identities", host, `{"provider": "cloud", "email": "gus@example.com"}`); code != 201 {
		t.Fatalf("a guests holder on a guest: %d %s", code, text)
	}
	var list []identityView
	if c := getJSON(t, ts, "secret", "/api/v1/users/anna/identities", &list); c != 200 || len(list) != 1 || list[0].ID != i.ID {
		t.Fatalf("list: %d %+v", c, list)
	}
	if code, _ := callAPI(t, ts, "DELETE", "/api/v1/users/gus/identities/"+itoa(i.ID), "secret", ""); code != 404 {
		t.Fatalf("another user's identity: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", "/api/v1/users/anna/identities/"+itoa(i.ID), "secret", ""); code != 200 {
		t.Fatalf("remove: %d", code)
	}
	if getJSON(t, ts, "secret", "/api/v1/users/anna/identities", &list); len(list) != 0 {
		t.Fatalf("left: %+v", list)
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
