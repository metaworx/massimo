// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// idleView is what a test reads of a token's answer: its life (AP
// KeyLife, Blocks 3 and 4).
type idleView struct {
	ID           int64      `json:"id"`
	Kind         string     `json:"kind"`
	ExpiresAt    *time.Time `json:"expires_at"`
	LastUsedAt   *time.Time `json:"last_used_at"`
	LastUsedFrom string     `json:"last_used_from"`
	IdleExempt   bool       `json:"idle_exempt"`
	IdleEndsAt   *time.Time `json:"idle_ends_at"`
	Ended        string     `json:"ended"`
	FromSettings bool       `json:"from_settings"`
}

// tokensOf lists a user's tokens as the API answers them, by id.
func tokensOf(t *testing.T, r *rig, token, user string) map[int64]idleView {
	t.Helper()
	var list []idleView
	getJSON(t, r.ts, token, "/api/v1/users/"+user+"/tokens", &list)
	m := map[int64]idleView{}
	for _, v := range list {
		m[v.ID] = v
	}
	return m
}

// The setting auth.key_idle_days (AP KeyLife, Block 3, D2-A): 0 and not
// set by default, taken from 0 to 3650 as a number or a string, refused
// beyond and as no whole number, null the default again.
func TestKeyIdleDaysSetting(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts := r.ts
	read := func() settingView {
		t.Helper()
		var all map[string]settingView
		getJSON(t, ts, "secret", "/api/v1/settings", &all)
		v, ok := all[store.KeyIdleDaysKey]
		if !ok {
			t.Fatalf("no %s in %v", store.KeyIdleDaysKey, all)
		}
		return v
	}
	if v := read(); v.Value != "0" || v.Default != "0" || v.Set || v.InEffect != "0" || v.Applies != "now" {
		t.Fatalf("the default: %+v", v)
	}
	for _, c := range []struct {
		body, want string
		code       int
	}{
		{`0`, "0", 200}, {`3650`, "3650", 200}, {`"30"`, "30", 200}, {`7`, "7", 200},
		{`-1`, "", 400}, {`3651`, "", 400}, {`"many"`, "", 400}, {`1.5`, "", 400},
	} {
		code, text := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", `{"`+store.KeyIdleDaysKey+`": `+c.body+`}`)
		if code != c.code {
			t.Errorf("%s: %d %s, want %d", c.body, code, text, c.code)
			continue
		}
		if v := read(); c.code == 200 && (v.Value != c.want || !v.Set) {
			t.Errorf("%s: %+v", c.body, v)
		}
	}
	if code, text := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", `{"`+store.KeyIdleDaysKey+`": null}`); code != 200 {
		t.Fatalf("null: %d %s", code, text)
	}
	if v := read(); v.Value != "0" || v.Set {
		t.Fatalf("null, the default again: %+v", v)
	}
}

// Under the idle limit (AP KeyLife, Block 3): a session's use writes its
// key's last use and address; a key unused past the limit ends, its
// session with it, each refused as an ended token, not counted toward
// the lockout, and the refusals write no use; the list says why and
// when; the limit raised, both live again (D6-A). The settings file's
// key never ends by it (D3-A). The store's clock starts ten days back
// and catches up with the real time, which the server's own checks
// keep (the ended token's, the lockout's).
func TestKeyIdle(t *testing.T) {
	t.Parallel()
	day := 24 * time.Hour
	clock := testkit.NewClock(time.Now().UTC().Add(-10 * day).Truncate(time.Millisecond))
	r := startServer(t, withOptions(storeOptionsAt(t, t.TempDir(), "secret", clock.Now)))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	key, keySecret, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "SM-X, login by fingerprint"})
	session, sessionSecret, err := st.AddKeySession(ctx, key.ID, "SM-X", time.Now().Add(SessionLifetime))
	if err != nil {
		t.Fatal(err)
	}

	// Three days on, the session's first use is its key's too.
	clock.Add(3 * day)
	if code, text := callAPI(t, ts, "GET", "/api/v1/me", sessionSecret, ""); code != 200 {
		t.Fatalf("the session: %d %s", code, text)
	}
	used := clock.Now()
	k := tokensOf(t, r, "secret", "anna")[key.ID]
	if k.LastUsedAt == nil || !k.LastUsedAt.Equal(used) || k.LastUsedFrom != "127.0.0.1" {
		t.Fatalf("the key after its session's use: %+v, want used at %v from 127.0.0.1", k, used)
	}
	if k.IdleEndsAt != nil || k.Ended != "" || k.IdleExempt {
		t.Fatalf("the key with the limit off: %+v", k)
	}

	// On at 7 days, a week after that use the key ends, and its session.
	if code, text := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", `{"auth.key_idle_days": 7}`); code != 200 {
		t.Fatalf("the limit: %d %s", code, text)
	}
	if k := tokensOf(t, r, "secret", "anna")[key.ID]; k.IdleEndsAt == nil || !k.IdleEndsAt.Equal(used.Add(7*day)) || k.Ended != "" {
		t.Fatalf("the key under the limit: %+v", k)
	}
	clock.Add(7 * day)
	for i := 0; i < DefaultAuthFailures+1; i++ {
		for _, c := range []struct{ what, method, path, token string }{
			{"the key ended unused", "POST", "/api/v1/login/key", keySecret},
			{"its session", "GET", "/api/v1/cubes", sessionSecret},
		} {
			if code, text := callAPI(t, ts, c.method, c.path, c.token, ""); code != 401 || !strings.Contains(text, errBadToken.Error()) {
				t.Fatalf("%s, refusal %d: %d %s", c.what, i+1, code, text)
			}
		}
	}
	if keys, err := st.Lockout(DefaultAuthFailures, DefaultAuthWindow, DefaultAuthLockout).Keys(ctx); err != nil || len(keys) != 0 {
		t.Fatalf("an idle key counted: %v %v", keys, err)
	}
	list := tokensOf(t, r, "secret", "anna")
	if k := list[key.ID]; k.Ended != store.EndedIdle || k.IdleEndsAt == nil || !k.IdleEndsAt.Equal(used.Add(7*day)) || k.LastUsedAt == nil || !k.LastUsedAt.Equal(used) {
		t.Fatalf("the key ended unused, its use not written by the refusals: %+v", k)
	}
	if _, ok := list[session.ID]; ok {
		t.Fatalf("the session of an ended key is listed: %+v", list)
	}

	// The settings file's key was last used at day 3, as the key was, so
	// its next use at day 10, the list's read above, came exactly at the
	// limit's edge, where the key was refused: kept from the limit, it was
	// let in, lives and says it is kept from the limit.
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", "secret", ""); code != 200 {
		t.Fatalf("the settings file's key under the limit: %d", code)
	}
	var me meView
	getJSON(t, ts, "secret", "/api/v1/me", &me)
	if me.Token == nil || !me.Token.IdleExempt || me.Token.IdleEndsAt != nil || me.Token.Ended != "" {
		t.Fatalf("the settings file's key in /api/v1/me: %+v", me.Token)
	}
	for _, v := range tokensOf(t, r, "secret", store.BootstrapUser) {
		if v.FromSettings && (!v.IdleExempt || v.IdleEndsAt != nil) {
			t.Fatalf("the settings file's key listed: %+v", v)
		}
	}

	// The limit raised: the key and its session live again.
	if code, text := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", `{"auth.key_idle_days": 30}`); code != 200 {
		t.Fatalf("the limit raised: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "GET", "/api/v1/cubes", sessionSecret, ""); code != 200 {
		t.Fatalf("the session, the limit raised: %d %s", code, text)
	}
}

// GET /api/v1/me shows the key presented as this request leaves it (AP
// KeyLife, Block 3): its use written, its idle end counts from now, not
// from the use before, days old; and the list read next says the same.
// The store's clock starts three days back and catches up with the real
// time, as TestKeyIdle's does.
func TestMeIdleEnd(t *testing.T) {
	t.Parallel()
	day := 24 * time.Hour
	clock := testkit.NewClock(time.Now().UTC().Add(-3 * day).Truncate(time.Millisecond))
	r := startServer(t, withOptions(storeOptionsAt(t, t.TempDir(), "secret", clock.Now)))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	key, secret, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "Loxone"})
	if err := st.TouchToken(ctx, key.ID, "192.0.2.10", time.Time{}, ""); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, store.KeyIdleDaysKey, 7); err != nil {
		t.Fatal(err)
	}
	clock.Add(3 * day)
	var me meView
	getJSON(t, ts, secret, "/api/v1/me", &me)
	want := clock.Now().Add(7 * day)
	if me.Token == nil || me.Token.ID != key.ID || me.Token.IdleEndsAt == nil || !me.Token.IdleEndsAt.Equal(want) {
		t.Fatalf("the key's idle end in /api/v1/me: %+v, want %v, a week from this use", me.Token, want)
	}
	if me.Token.LastUsedAt == nil || !me.Token.LastUsedAt.Equal(clock.Now()) || me.Token.LastUsedFrom != "127.0.0.1" {
		t.Fatalf("the key's last use in /api/v1/me: %+v, want this one", me.Token)
	}
	if k := tokensOf(t, r, "secret", "anna")[key.ID]; k.IdleEndsAt == nil || !k.IdleEndsAt.Equal(want) {
		t.Fatalf("the list after it: %+v, want %v", k, want)
	}
}

// PUT /api/v1/users/{name}/tokens/{id} (AP KeyLife, Block 3, D7-A): who
// manages the key's user keeps a key from the idle limit and holds it to
// it again, answered with the key's view, audited as token.update with
// its change line; each refusal: no field, an unknown one, a session, the
// settings file's key, the bearer that very key, a caller that does not
// manage the user, an integration's key kept, another user's token, a
// revoked one. POST …/tokens takes idle_exempt from who manages the user
// alone.
func TestPutToken(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet(), withLog(slog.LevelInfo), withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "placeholder-pass-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	gina, _ := st.AddUser(ctx, store.User{Name: "gina", Role: store.RoleGuest}, "", 0)
	owner, ok, _ := st.User(ctx, store.BootstrapUser)
	if !ok {
		t.Fatal("no owner")
	}
	annaKey, annaSecret, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "Loxone"})
	annaOther, _, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "phone"})
	annaSession, _, _ := st.AddToken(ctx, anna.ID, store.TokenSession, "Firefox", time.Now().Add(SessionLifetime))
	ginaKey, _, _ := st.AddKey(ctx, gina.ID, store.KeyOptions{Label: "tablet"})
	ownKey, ownSecret, _ := st.AddKey(ctx, owner.ID, store.KeyOptions{Label: "laptop"})
	manager := userToken(t, st, "mia", store.RoleManager) // guests, not users
	i, _ := registerHA(t, st, []string{"read"})
	integ, err := st.Tokens(ctx, i.UserID)
	if err != nil || len(integ) != 1 {
		t.Fatalf("the integration's key: %+v %v", integ, err)
	}
	var me meView
	getJSON(t, ts, "secret", "/api/v1/me", &me)
	file := me.Token.ID
	path := func(user string, id int64) string {
		return "/api/v1/users/" + user + "/tokens/" + strconv.FormatInt(id, 10)
	}
	put := func(what, bearer, user string, id int64, body string, want int, says string) string {
		t.Helper()
		code, text := callAPI(t, ts, "PUT", path(user, id), bearer, body)
		if code != want || (says != "" && !strings.Contains(errorOf(text), says)) {
			t.Errorf("%s: %d %s, want %d %q", what, code, text, want, says)
		}
		return text
	}

	logged := r.log.Mark()
	text := put("the owner keeps anna's key", "secret", "anna", annaKey.ID, `{"idle_exempt": true}`, 200, "")
	var v idleView
	if err := json.Unmarshal([]byte(text), &v); err != nil || v.ID != annaKey.ID || v.Kind != store.TokenAPI || !v.IdleExempt {
		t.Fatalf("the answer: %s %v", text, err)
	}
	r.log.Await(t, logged, "the change's line", testkit.Msg("token changed", "token", strconv.FormatInt(annaKey.ID, 10), "idle_exempt", "true"))
	if k := tokensOf(t, r, "secret", "anna")[annaKey.ID]; !k.IdleExempt {
		t.Fatalf("listed kept: %+v", k)
	}
	put("held to it again", "secret", "anna", annaKey.ID, `{"idle_exempt": false}`, 200, "")
	var changes []string
	for _, sum := range changeSummaries(t, r, "token.update", "token:"+strconv.FormatInt(annaKey.ID, 10)) {
		c, _ := sum["changes"].(map[string]any)
		changes = append(changes, fmt.Sprint(c["idle_exempt"]))
	}
	if fmt.Sprint(changes) != "[map[from:false to:true] map[from:true to:false]]" {
		t.Errorf("the change lines of token.update: %v", changes)
	}

	put("no field", "secret", "anna", annaKey.ID, `{}`, 400, "nothing to change")
	put("an unknown field", "secret", "anna", annaKey.ID, `{"idle_exempt": true, "label": "x"}`, 400, "bad body")
	put("a session", "secret", "anna", annaSession.ID, `{"idle_exempt": true}`, 400, "is a session")
	if text := put("the settings file's key", "secret", store.BootstrapUser, file, `{"idle_exempt": false}`, 409, "settings file"); answerCode(text) != "token_settings_key" {
		t.Errorf("the settings file's key: %s, want token_settings_key", text)
	}
	put("the bearer that very key", ownSecret, store.BootstrapUser, ownKey.ID, `{"idle_exempt": true}`, 403, "its own life")
	put("a key's user who manages nobody", annaSecret, "anna", annaOther.ID, `{"idle_exempt": true}`, 403, "only who manages user anna")
	put("a key's user who manages nobody, false", annaSecret, "anna", annaOther.ID, `{"idle_exempt": false}`, 403, "only who manages user anna")
	put("a manager of guests, a guest's key", manager, "gina", ginaKey.ID, `{"idle_exempt": true}`, 200, "")
	put("a manager of guests, an operator's key", manager, "anna", annaKey.ID, `{"idle_exempt": true}`, 403, "")
	put("an integration's key kept", "secret", i.UUID+":", integ[0].ID, `{"idle_exempt": true}`, 400, "always held to the idle limit")
	put("an integration's key held to it, as it is", "secret", i.UUID+":", integ[0].ID, `{"idle_exempt": false}`, 200, "")
	for what, c := range map[string]struct {
		user string
		id   int64
	}{"another user's token": {"gina", annaKey.ID}, "no such token": {"anna", 9999}} {
		if text := put(what, "secret", c.user, c.id, `{"idle_exempt": true}`, 404, "has no token"); answerCode(text) != "token_unknown" {
			t.Errorf("%s: %s, want token_unknown", what, text)
		}
	}
	if _, err := st.RevokeToken(ctx, annaOther.ID); err != nil {
		t.Fatal(err)
	}
	put("a revoked key", "secret", "anna", annaOther.ID, `{"idle_exempt": true}`, 404, "")

	// A session made from the key is not the key.
	code, text := callAPI(t, ts, "POST", "/api/v1/login/key", ownSecret, "")
	var made struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(text), &made); err != nil || code != 200 {
		t.Fatalf("the owner's key's session: %d %s %v", code, text, err)
	}
	put("a session made from that key", made.Token, store.BootstrapUser, ownKey.ID, `{"idle_exempt": true}`, 200, "")

	// POST takes idle_exempt from who manages the key's user alone.
	if code, text := callAPI(t, ts, "POST", "/api/v1/users/anna/tokens", annaSecret, `{"label":"kept by herself","idle_exempt":true}`); code != 403 ||
		!strings.Contains(errorOf(text), "only who manages user anna") {
		t.Errorf("a key kept by its user who manages nobody: %d %s", code, text)
	}
	code, text = callAPI(t, ts, "POST", "/api/v1/users/anna/tokens", "secret", `{"label":"kept","idle_exempt":true}`)
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(text), &created); err != nil || code != 201 {
		t.Fatalf("a key made kept: %d %s %v", code, text, err)
	}
	if k := tokensOf(t, r, "secret", "anna")[created.ID]; !k.IdleExempt {
		t.Errorf("the key made kept, listed: %+v", k)
	}
}

// A key ended unused is ended by its code (POST /api/v1/tokens/revoke,
// AP KeyLife, D6-A), its heir with it; and its use after the end, refused
// with 401, writes no hash of its revoke code for a key made before the
// store kept codes, which only a use while it lives fills. The store's
// clock starts ten days back and catches up with the real time, as
// TestKeyIdle's does.
func TestIdleKeyEndedByCode(t *testing.T) {
	t.Parallel()
	day := 24 * time.Hour
	clock := testkit.NewClock(time.Now().UTC().Add(-10 * day).Truncate(time.Millisecond))
	r := startServer(t, withOptions(storeOptionsAt(t, t.TempDir(), "secret", clock.Now)))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleManager}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	key, keySecret, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "phone"})
	heir, heirSecret, err := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "heir", MadeBy: key.ID, IdleExempt: true})
	if err != nil {
		t.Fatal(err)
	}
	// A key made before the update, its secret one the store never made.
	const oldSecret = "ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100"
	old, err := st.AddTokenHash(ctx, anna.ID, store.TokenAPI, "made before the codes", store.HashToken(oldSecret), time.Time{})
	if err != nil || old.RevokeKept {
		t.Fatalf("a key without a revoke code: %+v %v", old, err)
	}
	if code, text := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", `{"auth.key_idle_days": 7}`); code != 200 {
		t.Fatalf("the limit: %d %s", code, text)
	}
	clock.Add(8 * day)
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", oldSecret, ""); code != 401 {
		t.Fatalf("a key ended unused: %d", code)
	}
	if got, _, _ := st.Token(ctx, old.ID); got.RevokeKept {
		t.Fatal("the use of a key ended unused wrote the hash of its code")
	}
	if code, _, _ := postCode(t, ts, codeBody(keySecret), ""); code != 204 {
		t.Fatalf("the code of a key ended unused: %d", code)
	}
	if got, _, _ := st.Token(ctx, key.ID); got.DeletedAt.IsZero() {
		t.Fatal("the key ended unused is not revoked by its code")
	}
	if got, _, _ := st.Token(ctx, heir.ID); got.DeletedAt.IsZero() {
		t.Fatal("the heir of a key ended unused lives on after its maker's code")
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", heirSecret, ""); code != 401 {
		t.Fatalf("the heir after its maker's code: %d", code)
	}
	// The limit off, the key that only ended lives, and its first use
	// fills the hash.
	if code, text := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", `{"auth.key_idle_days": 0}`); code != 200 {
		t.Fatalf("the limit off: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", oldSecret, ""); code != 200 {
		t.Fatalf("the key with the limit off: %d", code)
	}
	if got, _, _ := st.Token(ctx, old.ID); !got.RevokeKept {
		t.Fatal("the first use of a live key without the hash of its code did not write it")
	}
}
