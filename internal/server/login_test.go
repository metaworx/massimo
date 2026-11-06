// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// A login answers a session that works until it is logged out; wrong
// pairs, disabled users and API keys are refused where they should be.
func TestLogin(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", DisplayName: "Anna", Role: store.RoleOperator}, "pw", 0)
	if err != nil {
		t.Fatal(err)
	}
	st.AddUser(ctx, store.User{Name: "nopw", Role: store.RoleViewer}, "", 0)
	before := time.Now()
	code, text := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"Anna","password":"pw","label":"test browser"}`)
	if code != 200 {
		t.Fatalf("login: %d %s", code, text)
	}
	var sess struct {
		Token        string    `json:"token"`
		ExpiresAt    time.Time `json:"expires_at"`
		User         userView  `json:"user"`
		Role         roleRef   `json:"role"`
		Capabilities []string  `json:"capabilities"`
	}
	json.Unmarshal([]byte(text), &sess)
	if len(sess.Token) != 64 || sess.ExpiresAt.Before(before.Add(SessionLifetime-time.Minute)) || sess.User.Name != "anna" || sess.User.DisplayName != "Anna" || sess.Role.Key != store.RoleOperator ||
		strings.Join(sess.Capabilities, ",") != "backup,configure,discover,lock,preferences,read,setpoint" || sess.User.LastLoginAt == nil {
		t.Fatalf("session %+v", sess)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", sess.Token, ""); code != 200 {
		t.Fatalf("session on a route: %d", code)
	}
	code, text = callAPI(t, ts, "GET", "/api/v1/me", sess.Token, "")
	var me meView
	json.Unmarshal([]byte(text), &me)
	if code != 200 || me.Mode != "users" || me.User == nil || me.User.Name != "anna" || me.Token == nil || me.Token.Kind != store.TokenSession || me.Token.Label != "test browser" || me.Setup || len(me.Capabilities) != 7 {
		t.Fatalf("me: %d %+v", code, me)
	}
	tokens, _ := st.Tokens(ctx, anna.ID)
	if len(tokens) != 1 || tokens[0].LastUsedFrom != "127.0.0.1" || tokens[0].LastUsedAt.IsZero() {
		t.Fatalf("session row %+v", tokens)
	}

	for _, c := range []struct {
		body string
		want int
	}{
		{`{"user":"anna","password":"wrong"}`, 401},
		{`{"user":"nobody","password":"pw"}`, 401},
		{`{"user":"nopw","password":""}`, 400},
		{`{"user":"nopw","password":"x"}`, 401},
		{`{"user":"anna"}`, 400},
		{`nope`, 400},
	} {
		if code, text := callAPI(t, ts, "POST", "/api/v1/login", "", c.body); code != c.want {
			t.Errorf("login %s: %d %s, want %d", c.body, code, text, c.want)
		}
	}
	st.UpdateUser(ctx, "anna", func(u *store.User) error { u.Disabled = true; return nil }, 0)
	if code, text := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"pw"}`); code != 403 || !strings.Contains(text, "disabled") {
		t.Fatalf("disabled user's login: %d %s", code, text)
	}
	st.UpdateUser(ctx, "anna", func(u *store.User) error { u.Disabled = false; return nil }, 0)

	// A user whose role is disabled logs in and is refused every route.
	st.SetRoleDisabled(ctx, store.RoleOperator, true, 0)
	code, text = callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"pw"}`)
	if code != 200 {
		t.Fatalf("login with a disabled role: %d %s", code, text)
	}
	st.SetRoleDisabled(ctx, store.RoleOperator, false, 0)

	// Logout ends the session; an API key is not a session.
	if code, _ := callAPI(t, ts, "POST", "/api/v1/logout", "secret", ""); code != 400 {
		t.Fatalf("logout with a key: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/logout", sess.Token, ""); code != 200 {
		t.Fatalf("logout: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", sess.Token, ""); code != 401 {
		t.Fatalf("session after logout: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/logout", sess.Token, ""); code != 401 {
		t.Fatalf("logout twice: %d", code)
	}
}

// Failed logins count against the user name as well as the address.
func TestLoginLockout(t *testing.T) {
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.AuthFailures = 3
	_, ts, s := startWith(t, opts)
	st := s.opts.Store
	st.AddUser(t.Context(), store.User{Name: "anna", Role: store.RoleViewer}, "pw", 0)
	for i := 0; i < 2; i++ {
		if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"wrong"}`); code != 401 {
			t.Fatalf("failure %d: %d", i+1, code)
		}
	}
	// A success forgets the failures of both keys.
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"pw"}`); code != 200 {
		t.Fatal("right password after two failures")
	}
	for i := 0; i < 3; i++ {
		callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"wrong"}`)
	}
	l := st.Lockout(3, time.Minute, 5*time.Minute)
	if d, _ := l.LockedFor(t.Context(), store.LockoutUser("anna")); d == 0 {
		t.Fatal("the user name is not locked")
	}
	if d, _ := l.LockedFor(t.Context(), store.LockoutAddr("127.0.0.1")); d == 0 {
		t.Fatal("the address is not locked")
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"pw"}`); code != 429 {
		t.Fatalf("login while locked: %d", code)
	}
}

// GET /api/v1/me in every mode, and the login's answers outside users
// mode.
func TestMeModes(t *testing.T) {
	read := func(ts string) meView {
		var me meView
		json.Unmarshal([]byte(ts), &me)
		return me
	}
	_, open, s := startWith(t, storeOptions(t, t.TempDir(), ""))
	code, text := callAPI(t, open, "GET", "/api/v1/me", "", "")
	if me := read(text); code != 200 || me.Mode != "open" || me.User != nil || !me.Setup || len(me.Capabilities) != len(capabilities) {
		t.Fatalf("open: %d %+v", code, text)
	}
	if code, text := callAPI(t, open, "POST", "/api/v1/login", "", `{"user":"a","password":"b"}`); code != 409 || !strings.Contains(text, "no users yet") {
		t.Fatalf("login on an open server: %d %s", code, text)
	}
	userToken(t, s.opts.Store, "anna", store.RoleViewer)
	if code, _ := callAPI(t, open, "GET", "/api/v1/me", "", ""); code != 401 {
		t.Fatalf("me without a token in users mode: %d", code)
	}

	_, legacy, _ := start(t, "secret")
	code, text = callAPI(t, legacy, "GET", "/api/v1/me", "secret", "")
	if me := read(text); code != 200 || me.Mode != "token" || me.User != nil || me.Setup {
		t.Fatalf("token mode: %d %s", code, text)
	}
	if code, _ := callAPI(t, legacy, "POST", "/api/v1/login", "", `{"user":"a","password":"b"}`); code != 409 {
		t.Fatalf("login without a store: %d", code)
	}
	if code, _ := callAPI(t, legacy, "POST", "/api/v1/logout", "secret", ""); code != 400 {
		t.Fatalf("logout without a store: %d", code)
	}
	_, none, _ := start(t, "")
	code, text = callAPI(t, none, "GET", "/api/v1/me", "", "")
	if me := read(text); code != 200 || me.Mode != "none" {
		t.Fatalf("no authentication: %d %s", code, text)
	}
}
