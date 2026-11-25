// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// noWayInText is what a refused way in for an integration's user says.
const noWayInText = "acts through its integration alone"

// refusesWayIn checks that every route that would give the user name a
// way in of its own refuses it, asked from addr with token ("" for
// none), and that its role stays what the refused requests would have
// changed it from.
func refusesWayIn(t *testing.T, s *Server, addr, token, name string) {
	t.Helper()
	base := "/api/v1/users/" + name
	for _, c := range []struct{ method, path, body string }{
		{"POST", base + "/tokens", `{"label":"planted"}`},
		{"PUT", base, `{"role":"viewer","password":"correct horse"}`},
		// Told that it has no password, not that this one is short.
		{"PUT", base, `{"role":"viewer","password":"x"}`},
		{"PUT", base, `{"role":"viewer","record":"AAAA","ksf":"` + store.OpaqueKSF + `"}`},
		{"POST", base + "/register", `{"request":"AAAA"}`},
		{"POST", base + "/identities", `{"provider":"cloud","email":"anna@example.com"}`},
		{"PUT", base + "/identities/1", `{"email":"anna@example.com"}`},
		{"POST", base + "/identities/link/cloud", `{"redirect":"http://192.168.1.2:8099/"}`},
	} {
		if rec := sendFrom(s, c.method, c.path, addr, token, c.body); rec.Code != 403 || !strings.Contains(errorOf(rec.Body.String()), noWayInText) {
			t.Errorf("%s %s %s: %d %s", c.method, c.path, c.body, rec.Code, rec.Body.String())
		}
	}
	if u, _, _ := s.opts.Store.User(t.Context(), name); u.Role != store.RoleOwner {
		t.Errorf("%s: the role changed by a refused request: %s", name, u.Role)
	}
}

// meCaps reads the capabilities of GET /api/v1/me with key and the
// headers given.
func meCaps(t *testing.T, s *Server, key string, headers map[string]string) []string {
	t.Helper()
	r := httptest.NewRequest("GET", "/api/v1/me", nil)
	r.RemoteAddr = "192.168.1.20:5000"
	r.Header.Set("Authorization", "Bearer "+key)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	var me meView
	if json.Unmarshal(rec.Body.Bytes(), &me); rec.Code != 200 || me.Integration == nil {
		t.Fatalf("/me with the integration's key: %d %s", rec.Code, rec.Body.String())
	}
	return me.Capabilities
}

// An integration's user acts through its integration alone: given the
// role owner, it gets no key, password or provider's identity from an
// owner, and every call through the integration stays within the role
// integration, with its own header and without; the integration's key
// makes no session.
func TestIntegrationUserHasNoWayIn(t *testing.T) {
	t.Parallel()
	s := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret"))).s
	st := s.opts.Store
	lan := "192.168.1.20:5000"
	i, key := registerHA(t, st, integrationCeiling)
	meCaps(t, s, key, as(haAnna, LevelAdmin)) // the user of the header made
	for _, name := range []string{i.UUID + ":", i.UUID + ":system", i.UUID + ":" + haAnna} {
		if rec := sendFrom(s, "PUT", "/api/v1/users/"+name, lan, "secret", `{"role":"owner"}`); rec.Code != 200 {
			t.Fatalf("%s given the role owner: %d %s", name, rec.Code, rec.Body.String())
		}
		refusesWayIn(t, s, lan, "secret", name)
	}
	for _, h := range []map[string]string{nil, as(haAnna, LevelAdmin)} {
		for _, c := range meCaps(t, s, key, h) {
			if !slices.Contains(integrationCeiling, c) {
				t.Errorf("header %v: %s beyond the role integration", h, c)
			}
		}
	}
	if rec := sendFrom(s, "POST", "/api/v1/login/key", lan, key, ""); rec.Code != 403 {
		t.Errorf("a session from the integration's key: %d %s", rec.Code, rec.Body.String())
	}
}

// asAnIntegrations makes the user name an integration's, past the
// store, which makes none so: the state a version before this one left
// where an owner had given such a user a key, a password or an identity.
func asAnIntegrations(t *testing.T, st *store.Store, name string, i store.Integration) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(st.File())+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(t.Context(), `UPDATE users SET integration_id = ? WHERE name = ?`, i.ID, name); err != nil {
		t.Fatal(err)
	}
}

// What an integration's user holds from before this version: its key
// keeps working within the role integration and the integration's
// maximum, whatever its role (an owner's key removes nobody), and makes
// no session; its password and its identity at a provider sign in
// nowhere; a maximum stored beyond the role integration leaves no more
// than the role.
func TestHeldCredentialsOfAnIntegrationsUser(t *testing.T) {
	t.Parallel()
	f := newFakeIdP(t)
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	r := startServer(t, withOptions(opts))
	ts, s := r.ts, r.s
	st := s.opts.Store
	ctx := t.Context()
	lan := "192.168.1.20:5000"
	if _, err := st.AddProvider(ctx, store.AuthProvider{Key: "cloud", Name: "Nextcloud", Issuer: f.srv.URL, ClientID: fakeClientID, ClientSecret: f.secret, Enabled: true}, 0); err != nil {
		t.Fatal(err)
	}
	i, key := registerHA(t, st, []string{"read"})
	held, err := st.AddUser(ctx, store.User{Name: "held", Role: store.RoleOwner}, "correct horse", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, heldKey, err := st.AddToken(ctx, held.ID, store.TokenAPI, "from before", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddSubjectIdentity(ctx, held.ID, "cloud", "sub-1", 0); err != nil {
		t.Fatal(err)
	}
	userToken(t, st, "anna", store.RoleOwner)
	asAnIntegrations(t, st, "held", i)

	code, me, text := meFrom(t, s, lan, heldKey)
	if code != 200 || len(me.Capabilities) != 1 || me.Capabilities[0] != "read" {
		t.Fatalf("/me with the held key: %d %s", code, text)
	}
	if rec := sendFrom(s, "DELETE", "/api/v1/users/anna", lan, heldKey, ""); rec.Code != 403 || !strings.Contains(errorOf(rec.Body.String()), `capability "users" required`) {
		t.Fatalf("the held key of an owner's role removes anna: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendFrom(s, "POST", "/api/v1/login/key", lan, heldKey, ""); rec.Code != 403 || !strings.Contains(errorOf(rec.Body.String()), noWayInText) {
		t.Fatalf("a session from the held key: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendFrom(s, "POST", "/api/v1/login", lan, "", `{"user":"held","password":"correct horse"}`); rec.Code != 403 || !strings.Contains(errorOf(rec.Body.String()), noWayInText) {
		t.Fatalf("the held password: %d %s", rec.Code, rec.Body.String())
	}
	run := f.signIn(t, ts, "cloud", map[string]any{"email": "held@example.com", "email_verified": true})
	if code, body := finish(t, ts, run); code != 403 {
		t.Fatalf("the held identity at a provider: %d %v", code, body)
	}

	// A maximum beyond the role integration, as only the database can
	// hold it, and the integration's own user given the role owner.
	if _, err := st.UpdateUser(ctx, i.UUID+":", func(u *store.User) error { u.Role = store.RoleOwner; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(st.File())+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `UPDATE integrations SET max_caps = '["read","users","auth"]' WHERE id = ?`, i.ID); err != nil {
		t.Fatal(err)
	}
	for _, c := range meCaps(t, s, key, nil) {
		if !slices.Contains(integrationCeiling, c) {
			t.Errorf("the integration's key with a stored maximum beyond the role: %s", c)
		}
	}
}

// A key an integration's user holds from before is refused, with the
// code integration_disabled, once its integration is disabled or
// revoked, as the disabled integration's own key is.
func TestHeldKeyOfAnInactiveIntegration(t *testing.T) {
	t.Parallel()
	for _, how := range []string{"disabled", "revoked"} {
		t.Run(how, func(t *testing.T) {
			t.Parallel()
			s := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret"))).s
			st := s.opts.Store
			ctx := t.Context()
			i, key := registerHA(t, st, []string{"read"})
			held, err := st.AddUser(ctx, store.User{Name: "held", Role: store.RoleOwner}, "", 0)
			if err != nil {
				t.Fatal(err)
			}
			_, heldKey, err := st.AddToken(ctx, held.ID, store.TokenAPI, "from before", time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			asAnIntegrations(t, st, "held", i)
			if rec := sendFrom(s, "GET", "/api/v1/cubes", "192.168.1.20:5000", heldKey, ""); rec.Code != 200 {
				t.Fatalf("the held key while the integration is active: %d %s", rec.Code, rec.Body.String())
			}
			if how == "disabled" {
				_, err = st.SetIntegration(ctx, i.UUID, func(g *store.Integration) error { g.State = store.IntegrationDisabled; return nil }, 0)
			} else {
				err = st.RevokeIntegration(ctx, i.UUID, 0)
			}
			if err != nil {
				t.Fatal(err)
			}
			keys := []string{heldKey, key}
			if how == "revoked" {
				keys = keys[:1] // a revoke ends the integration's own key: 401
			}
			for _, k := range keys {
				rec := sendFrom(s, "GET", "/api/v1/cubes", "192.168.1.20:5000", k, "")
				var body struct{ Code string }
				if json.Unmarshal(rec.Body.Bytes(), &body); rec.Code != 403 || body.Code != "integration_disabled" {
					t.Errorf("a key of the %s integration's: %d %s", how, rec.Code, rec.Body.String())
				}
			}
		})
	}
}

// After the start that moves the settings file's key from an
// integration's user to the owner, the key is marked in the owner's list
// and not the integration's user's, and that user may be removed: the
// file's key no longer holds it (AP LooseEnds, D1).
func TestSettingsKeyLeavesAnIntegrationsUser(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "")
	st := opts.Store
	ctx := t.Context()
	i, _ := registerHA(t, st, []string{"read"})
	held, err := st.AddUser(ctx, store.User{Name: "held", Role: store.RoleOwner}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddTokenHash(ctx, held.ID, store.TokenAPI, store.BootstrapLabel, store.HashToken("file"), time.Time{}); err != nil {
		t.Fatal(err)
	}
	asAnIntegrations(t, st, "held", i)
	opts.Token = "file"
	s := startServer(t, withOptions(opts)).s
	lan := "192.168.1.20:5000"
	if s.AuthMode() != "users" {
		t.Fatalf("after the start: %s", s.AuthMode())
	}
	var list []tokenView
	rec := sendFrom(s, "GET", "/api/v1/users/held/tokens", lan, "file", "")
	if json.Unmarshal(rec.Body.Bytes(), &list); rec.Code != 200 || len(list) != 0 {
		t.Fatalf("the integration's user's keys: %d %s", rec.Code, rec.Body.String())
	}
	rec = sendFrom(s, "GET", "/api/v1/users/"+store.BootstrapUser+"/tokens", lan, "file", "")
	if json.Unmarshal(rec.Body.Bytes(), &list); rec.Code != 200 || len(list) != 1 || !list[0].FromSettings {
		t.Fatalf("the owner's keys: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendFrom(s, "DELETE", "/api/v1/users/held", lan, "file", ""); rec.Code != 200 {
		t.Fatalf("the integration's user removed: %d %s", rec.Code, rec.Body.String())
	}
}
