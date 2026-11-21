// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
)

// sendFrom answers a request with a body as if it came from addr.
func sendFrom(s *Server, method, path, addr, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = addr
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

// keepOpenOf reads the setting auth.keep_open from GET /api/v1/settings.
func keepOpenOf(t *testing.T, s *Server, token string) settingView {
	t.Helper()
	rec := sendFrom(s, "GET", "/api/v1/settings", "127.0.0.1:5000", token, "")
	var all map[string]settingView
	if json.Unmarshal(rec.Body.Bytes(), &all); rec.Code != 200 {
		t.Fatalf("settings: %d %s", rec.Code, rec.Body.String())
	}
	v, ok := all[store.KeepOpenKey]
	if !ok {
		t.Fatalf("no %s in %s", store.KeepOpenKey, rec.Body.String())
	}
	return v
}

// meFrom reads GET /api/v1/me as a caller at addr.
func meFrom(t *testing.T, s *Server, addr, token string) (int, meView, string) {
	t.Helper()
	rec := sendFrom(s, "GET", "/api/v1/me", addr, token, "")
	var me meView
	json.Unmarshal(rec.Body.Bytes(), &me)
	return rec.Code, me, rec.Body.String()
}

// On an open server the setting is off by default, set and reset by
// anyone on the local network, and /me says kept_open while it is on;
// it changes nothing about who is let in: beyond the local network the
// server refuses as before, the setting's PUT included, and the sign-in
// answers as before (AP FirstOwner, D11). The checks from beyond the
// local network and of the sign-in are forward guards: this code is not
// the block's, and they keep it so.
func TestKeepOpenOnAnOpenServer(t *testing.T) {
	t.Parallel()
	s := startServer(t, withOptions(storeOptions(t, t.TempDir(), ""))).s
	lan, far := "192.168.1.20:5000", "203.0.113.9:5000"
	if v := keepOpenOf(t, s, ""); v.Value != "false" || v.Default != "false" || v.Set || v.InEffect != "false" || v.Applies != "now" {
		t.Fatalf("the default: %+v", v)
	}
	if code, me, text := meFrom(t, s, lan, ""); code != 200 || me.Mode != "open" || !me.Setup || me.KeptOpen || strings.Contains(text, "kept_open") {
		t.Fatalf("/me, not kept open: %d %s", code, text)
	}
	if rec := sendFrom(s, "PUT", "/api/v1/settings", far, "", `{"auth.keep_open":true}`); rec.Code != 403 || errorOf(rec.Body.String()) != openRefused {
		t.Fatalf("the PUT from beyond the local network: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendFrom(s, "PUT", "/api/v1/settings", lan, "", `{"auth.keep_open":"yes"}`); rec.Code != 400 {
		t.Fatalf("not a switch: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendFrom(s, "PUT", "/api/v1/settings", lan, "", `{"auth.keep_open":true}`); rec.Code != 200 {
		t.Fatalf("the PUT from the local network: %d %s", rec.Code, rec.Body.String())
	}
	if v := keepOpenOf(t, s, ""); v.Value != "true" || !v.Set || v.InEffect != "true" {
		t.Fatalf("set: %+v", v)
	}
	if code, me, text := meFrom(t, s, lan, ""); code != 200 || me.Mode != "open" || !me.Setup || !me.KeptOpen {
		t.Fatalf("/me, kept open: %d %s", code, text)
	}
	for _, path := range []string{"/api/v1/me", "/api/v1/cubes", "/api/v1/settings"} {
		if rec := sendFrom(s, "GET", path, far, "", ""); rec.Code != 403 || errorOf(rec.Body.String()) != openRefused {
			t.Errorf("%s from beyond the local network, kept open: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	if rec := sendFrom(s, "POST", "/api/v1/login", lan, "", `{"user":"a","password":"b"}`); rec.Code != 409 || !strings.Contains(rec.Body.String(), "no users yet") {
		t.Fatalf("the sign-in, kept open: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendFrom(s, "PUT", "/api/v1/settings", lan, "", `{"auth.keep_open":null}`); rec.Code != 200 {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body.String())
	}
	if v := keepOpenOf(t, s, ""); v.Value != "false" || v.Set {
		t.Fatalf("reset: %+v", v)
	}
	if _, me, _ := meFrom(t, s, lan, ""); me.KeptOpen {
		t.Fatal("/me, reset: kept open")
	}
}

// The first owner by the API takes the setting with it, and /me of the
// owner has no kept_open; on a server with users the setting cannot be
// switched on, off and the default are taken, and it shows off (D12).
func TestKeepOpenGoesWithTheFirstOwner(t *testing.T) {
	t.Parallel()
	s := startServer(t, withOptions(storeOptions(t, t.TempDir(), ""))).s
	lan := "192.168.1.20:5000"
	if rec := sendFrom(s, "PUT", "/api/v1/settings", lan, "", `{"auth.keep_open":true}`); rec.Code != 200 {
		t.Fatalf("set: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendFrom(s, "POST", "/api/v1/users", lan, "", `{"name":"root","password":"p"}`); rec.Code != 201 {
		t.Fatalf("the first owner: %d %s", rec.Code, rec.Body.String())
	}
	if _, set, _ := s.opts.Store.Setting(t.Context(), store.KeepOpenKey); set {
		t.Fatal("the setting stays with the first owner")
	}
	key := userToken(t, s.opts.Store, "anna", store.RoleOwner)
	if code, me, text := meFrom(t, s, lan, key); code != 200 || me.Mode != "users" || me.KeptOpen || strings.Contains(text, "kept_open") {
		t.Fatalf("/me with users: %d %s", code, text)
	}
	rec := sendFrom(s, "PUT", "/api/v1/settings", lan, key, `{"auth.keep_open":true}`)
	if rec.Code != 409 || !strings.Contains(errorOf(rec.Body.String()), "without users") {
		t.Fatalf("on with users: %d %s", rec.Code, rec.Body.String())
	}
	for _, body := range []string{`{"auth.keep_open":false}`, `{"auth.keep_open":null}`} {
		if rec := sendFrom(s, "PUT", "/api/v1/settings", lan, key, body); rec.Code != 200 {
			t.Fatalf("%s with users: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	// A value left by a process that does not know the setting shows off.
	if err := s.opts.Store.SetSetting(t.Context(), store.KeepOpenKey, true); err != nil {
		t.Fatal(err)
	}
	if v := keepOpenOf(t, s, key); v.InEffect != "false" {
		t.Fatalf("with users: %+v", v)
	}
	if code, me, _ := meFrom(t, s, lan, key); code != 200 || me.KeptOpen {
		t.Fatal("/me with users: kept open")
	}
}

// The server removes a setting left beside users at its start and with
// DropKeepOpen, which a reload calls (D12).
func TestKeepOpenDroppedAtTheStart(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "")
	st := opts.Store
	if _, err := st.AddUser(t.Context(), store.User{Name: "anna", Role: store.RoleOwner}, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(t.Context(), store.KeepOpenKey, true); err != nil {
		t.Fatal(err)
	}
	s := New(opts)
	if st.KeepOpen(t.Context()) {
		t.Fatal("kept open after the start")
	}
	if err := st.SetSetting(t.Context(), store.KeepOpenKey, true); err != nil {
		t.Fatal(err)
	}
	s.DropKeepOpen(t.Context())
	if st.KeepOpen(t.Context()) {
		t.Fatal("kept open after DropKeepOpen")
	}
}

// A server without a store has no setting: no kept_open, and the
// settings' routes answer as before.
func TestKeepOpenWithoutAStore(t *testing.T) {
	t.Parallel()
	s := New(Options{})
	if code, me, text := meFrom(t, s, "192.168.1.20:5000", ""); code != 200 || me.Mode != "none" || me.KeptOpen || strings.Contains(text, "kept_open") {
		t.Fatalf("/me: %d %s", code, text)
	}
	if rec := sendFrom(s, "PUT", "/api/v1/settings", "192.168.1.20:5000", "", `{"auth.keep_open":true}`); rec.Code != 503 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	s.DropKeepOpen(t.Context())
	tok := New(Options{Token: "secret"})
	if code, me, text := meFrom(t, tok, "192.168.1.20:5000", "secret"); code != 200 || me.Mode != "token" || me.KeptOpen || strings.Contains(text, "kept_open") {
		t.Fatalf("/me with a token: %d %s", code, text)
	}
}
