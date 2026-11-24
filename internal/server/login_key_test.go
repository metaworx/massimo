// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// POST /api/v1/login/key makes a session from the key that calls (AP
// MobileApp v1.1, Block 8): the session names the key, the key's next
// session ends it, revoking the key ends it at its next request, and
// what may not make one is refused with the status the docs give.
func TestLoginKey(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.AuthFailures = 3
	r := startServer(t, withOptions(opts))
	ts, s := r.ts, r.s
	st := s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "pw", 0)
	if err != nil {
		t.Fatal(err)
	}
	keyTok, key, _ := st.AddToken(ctx, anna.ID, store.TokenAPI, "SM-X, login by fingerprint", time.Time{})

	type session struct {
		Token        string    `json:"token"`
		ExpiresAt    time.Time `json:"expires_at"`
		User         userView  `json:"user"`
		Role         roleRef   `json:"role"`
		Capabilities []string  `json:"capabilities"`
	}
	login := func(body string) session {
		t.Helper()
		code, text := callAPI(t, ts, "POST", "/api/v1/login/key", key, body)
		if code != 200 {
			t.Fatalf("login by key: %d %s", code, text)
		}
		var sess session
		json.Unmarshal([]byte(text), &sess)
		if len(sess.Token) != 64 || sess.User.Name != "anna" || sess.Role.Key != store.RoleOperator || len(sess.Capabilities) != 7 ||
			sess.ExpiresAt.Before(time.Now().Add(SessionLifetime-time.Minute)) {
			t.Fatalf("session %+v", sess)
		}
		return sess
	}
	first := login(`{"label":"Massimo/1 (Android 17; SM-X)"}`)
	code, text := callAPI(t, ts, "GET", "/api/v1/me", first.Token, "")
	var me meView
	json.Unmarshal([]byte(text), &me)
	if code != 200 || me.Token == nil || me.Token.Kind != store.TokenSession || me.Token.KeyID != keyTok.ID || me.Token.Label != "Massimo/1 (Android 17; SM-X)" {
		t.Fatalf("me with the key's session: %d %s", code, text)
	}
	// The list names the session's key.
	code, text = callAPI(t, ts, "GET", "/api/v1/users/anna/tokens", first.Token, "")
	var list []tokenView
	json.Unmarshal([]byte(text), &list)
	if code != 200 || len(list) != 2 || list[0].ID != keyTok.ID || list[0].KeyID != 0 || list[1].KeyID != keyTok.ID {
		t.Fatalf("tokens: %d %s", code, text)
	}

	// The next session ends the earlier one; no body takes the
	// User-Agent as the label.
	second := login("")
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", first.Token, ""); code != 401 {
		t.Fatalf("the earlier session after the next: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", second.Token, ""); code != 200 {
		t.Fatalf("the next session: %d", code)
	}
	if tokens, _ := st.Tokens(ctx, anna.ID); len(tokens) != 2 || tokens[1].Label == "" {
		t.Fatalf("tokens after the next session %+v", tokens)
	}

	// A session as the bearer, a bad body; neither counts as a failed check.
	if code, text := callAPI(t, ts, "POST", "/api/v1/login/key", second.Token, ""); code != 400 || !strings.Contains(text, "session") {
		t.Fatalf("a session as the bearer: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login/key", key, "nope"); code != 400 {
		t.Fatalf("a bad body: %d", code)
	}

	// A disabled user's key is 403, not a failed check.
	st.UpdateUser(ctx, "anna", func(u *store.User) error { u.Disabled = true; return nil }, 0)
	if code, text := callAPI(t, ts, "POST", "/api/v1/login/key", key, ""); code != 403 || !strings.Contains(text, "disabled") {
		t.Fatalf("a disabled user's key: %d %s", code, text)
	}
	st.UpdateUser(ctx, "anna", func(u *store.User) error { u.Disabled = false; return nil }, 0)

	// A registered integration's key makes none.
	_, haKey := registerHA(t, st, nil)
	if code, text := callAPI(t, ts, "POST", "/api/v1/login/key", haKey, ""); code != 403 || !strings.Contains(text, "integration") {
		t.Fatalf("an integration's key: %d %s", code, text)
	}

	// Revoking the key ends its session at the session's next request;
	// the revoked key is refused, but has ended, which is no guess and
	// does not count toward the lockout (D64-A); a wrong key counts.
	if code, text := callAPI(t, ts, "DELETE", "/api/v1/users/anna/tokens/"+strconv.FormatInt(keyTok.ID, 10), second.Token, ""); code != 200 {
		t.Fatalf("revoke the key: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", second.Token, ""); code != 401 {
		t.Fatalf("the key's session after the key: %d", code)
	}
	for i := 0; i < 3; i++ {
		if code, _ := callAPI(t, ts, "POST", "/api/v1/login/key", key, ""); code != 401 {
			t.Fatalf("a revoked key, try %d: %d", i+1, code)
		}
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", "secret", ""); code != 200 {
		t.Fatalf("after a revoked key three times: %d", code)
	}
	for i := 0; i < 3; i++ {
		if code, _ := callAPI(t, ts, "POST", "/api/v1/login/key", "wrong-key", ""); code != 401 {
			t.Fatalf("a wrong key, try %d: %d", i+1, code)
		}
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", "secret", ""); code != 429 {
		t.Fatalf("after three wrong keys: %d", code)
	}
}

// The login by key works where the password sign-in is off, and is 409
// on a server without users.
func TestLoginKeyModes(t *testing.T) {
	t.Parallel()
	open := startServer(t, withOptions(Options{Store: testStore(t, t.TempDir())})).ts
	if code, text := callAPI(t, open, "POST", "/api/v1/login/key", "", ""); code != 409 {
		t.Fatalf("open server: %d %s", code, text)
	}
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	key := userToken(t, r.s.opts.Store, "anna", store.RoleViewer)
	if code, text := callAPI(t, r.ts, "PUT", "/api/v1/settings", "secret", `{"`+store.PasswordLoginKey+`":false}`); code != 200 {
		t.Fatalf("the password sign-in off: %d %s", code, text)
	}
	if code, _ := callAPI(t, r.ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"pw"}`); code != 403 {
		t.Fatalf("the password sign-in while off: %d", code)
	}
	if code, text := callAPI(t, r.ts, "POST", "/api/v1/login/key", key, ""); code != 200 {
		t.Fatalf("the login by key while the password sign-in is off: %d %s", code, text)
	}
}

// DELETE /api/v1/me/token revokes the token that calls (AP MobileApp,
// D80-B): a session by itself, a key by itself with its session, each
// refused after and audited as token.revoke; a caller without a token
// of a user is 400, and so is an integration's call on a user's behalf,
// the integration's key revoking itself without the header; the settings
// file's token is 409 and stays.
func TestDeleteMyToken(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "secret")
	r := startServer(t, withOptions(opts))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleViewer}, "pw", 0)
	if err != nil {
		t.Fatal(err)
	}
	revoke := func(token, what string) int64 {
		t.Helper()
		code, text := callAPI(t, ts, "DELETE", "/api/v1/me/token", token, "")
		var got struct {
			OK bool  `json:"ok"`
			ID int64 `json:"id"`
		}
		json.Unmarshal([]byte(text), &got)
		if code != 200 || !got.OK || got.ID == 0 {
			t.Fatalf("%s revokes itself: %d %s", what, code, text)
		}
		return got.ID
	}
	live := func(id int64) bool {
		t.Helper()
		tokens, err := st.Tokens(ctx, anna.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, tk := range tokens {
			if tk.ID == id {
				return true
			}
		}
		return false
	}

	// A session revokes itself; the key it was not made from stays.
	other, _, _ := st.AddToken(ctx, anna.ID, store.TokenAPI, "another key", time.Time{})
	sessTok, sess, _ := st.AddToken(ctx, anna.ID, store.TokenSession, "a browser", time.Now().Add(time.Hour))
	if id := revoke(sess, "a session"); id != sessTok.ID {
		t.Fatalf("the session's id: %d, not %d", id, sessTok.ID)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", sess, ""); code != 401 {
		t.Fatalf("the session after its own revoke: %d", code)
	}
	if live(sessTok.ID) || !live(other.ID) {
		t.Fatalf("the session gone, the other key live: %v %v", live(sessTok.ID), live(other.ID))
	}

	// A key revokes itself, and the session made from it goes with it.
	keyTok, key, _ := st.AddToken(ctx, anna.ID, store.TokenAPI, "SM-X, login by fingerprint", time.Time{})
	code, text := callAPI(t, ts, "POST", "/api/v1/login/key", key, "")
	var made struct {
		Token string `json:"token"`
	}
	json.Unmarshal([]byte(text), &made)
	if code != 200 || made.Token == "" {
		t.Fatalf("a session from the key: %d %s", code, text)
	}
	if id := revoke(key, "a key"); id != keyTok.ID {
		t.Fatalf("the key's id: %d, not %d", id, keyTok.ID)
	}
	for what, token := range map[string]string{"the key": key, "its session": made.Token} {
		if code, _ := callAPI(t, ts, "GET", "/api/v1/me", token, ""); code != 401 {
			t.Fatalf("%s after the key's own revoke: %d", what, code)
		}
	}
	if live(keyTok.ID) || !live(other.ID) {
		t.Fatalf("the key gone, the other key live: %v %v", live(keyTok.ID), live(other.ID))
	}
	rows, err := st.AuditList(ctx, time.Time{}, 100, "")
	if err != nil {
		t.Fatal(err)
	}
	revokes := map[string]bool{}
	for _, row := range rows {
		if row.Action == "token.revoke" && row.Result == "ok" {
			revokes[row.Target] = true
		}
	}
	for _, id := range []int64{sessTok.ID, keyTok.ID} {
		if !revokes["token:"+strconv.FormatInt(id, 10)] {
			t.Fatalf("no audit row token.revoke for token %d: %+v", id, rows)
		}
	}

	// The settings file's token is refused and stays, as the next start
	// could not bring it back.
	if code, text := callAPI(t, ts, "DELETE", "/api/v1/me/token", "secret", ""); code != 409 || !strings.Contains(text, "settings file") {
		t.Fatalf("the settings file's token: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", "secret", ""); code != 200 {
		t.Fatalf("the settings file's token after its refused revoke: %d", code)
	}

	// An integration's call on a user's behalf revokes nothing; its key
	// revokes itself without the header.
	_, haKey := registerHA(t, st, []string{"read"})
	if resp, text := callWith(t, ts, "DELETE", "/api/v1/me/token", haKey, "", as(haAnna, LevelView)); resp.StatusCode != 400 || !strings.Contains(text, OnBehalfHeader) {
		t.Fatalf("on a user's behalf: %d %s", resp.StatusCode, text)
	}
	if code, text := callAPI(t, ts, "GET", "/api/v1/me", haKey, ""); code != 200 {
		t.Fatalf("the integration's key after a call on a user's behalf: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "DELETE", "/api/v1/me/token", haKey, ""); code != 200 {
		t.Fatalf("the integration's key by itself: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", haKey, ""); code != 401 {
		t.Fatalf("the integration's key after its own revoke: %d", code)
	}

	// Without a token of a user: an open server, the one token of a
	// server without users.
	open := startServer(t, withOptions(Options{Store: testStore(t, t.TempDir())})).ts
	if code, text := callAPI(t, open, "DELETE", "/api/v1/me/token", "", ""); code != 400 {
		t.Fatalf("an open server: %d %s", code, text)
	}
	legacy := startServer(t, withOptions(Options{Token: "only"})).ts
	if code, text := callAPI(t, legacy, "DELETE", "/api/v1/me/token", "only", ""); code != 400 {
		t.Fatalf("the one token of a server without users: %d %s", code, text)
	}
}
