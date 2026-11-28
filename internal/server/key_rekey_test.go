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

// rekeyed is the answer of POST …/tokens/{id}/rekey, as of POST …/tokens.
type rekeyed struct {
	Token     string    `json:"token"`
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
}

// A key's new secret (AP KeyLife, Block 5; MobileApp.D76-A, D81-A): POST
// …/tokens/{id}/rekey answers 200 with the new secret, the key's id,
// kind, label and making; the list keeps its end date, exclusion, maker
// and heirs; the new secret lets in, the old one is 401 as an ended token,
// counted toward no lockout, and so is the key's session; a session made
// from the key that asks for it stays, the log line saying so. Audited as
// token.rekey, with a change line, and logged: none of them, nor the
// audit's rows, keeps a secret.
func TestRekey(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet(), withLog(slog.LevelInfo), withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "placeholder-pass-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	maker, makerSecret, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "laptop"})
	end := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	var phone rekeyed
	code, text := callAPI(t, ts, "POST", "/api/v1/users/anna/tokens", makerSecret, `{"label":"SM-X, login by fingerprint","expires_at":"`+end.Format(time.RFC3339)+`"}`)
	if err := json.Unmarshal([]byte(text), &phone); err != nil || code != 201 {
		t.Fatalf("the phone's key: %d %s %v", code, text, err)
	}
	path := "/api/v1/users/anna/tokens/" + strconv.FormatInt(phone.ID, 10)
	if code, text := callAPI(t, ts, "PUT", path, "secret", `{"idle_exempt":true}`); code != 200 {
		t.Fatalf("the phone's key kept from the idle limit: %d %s", code, text)
	}
	var heir rekeyed
	code, text = callAPI(t, ts, "POST", "/api/v1/users/anna/tokens", phone.Token, `{"label":"made by the phone's key"}`)
	if err := json.Unmarshal([]byte(text), &heir); err != nil || code != 201 {
		t.Fatalf("the phone's key's heir: %d %s %v", code, text, err)
	}
	login := func(key string) string {
		t.Helper()
		code, text := callAPI(t, ts, "POST", "/api/v1/login/key", key, "")
		var s struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal([]byte(text), &s); err != nil || code != 200 {
			t.Fatalf("a session of the key: %d %s %v", code, text, err)
		}
		return s.Token
	}
	rekey := func(what, bearer string) rekeyed {
		t.Helper()
		code, text := callAPI(t, ts, "POST", path+"/rekey", bearer, "")
		var m rekeyed
		if err := json.Unmarshal([]byte(text), &m); err != nil || code != 200 {
			t.Fatalf("%s: %d %s %v", what, code, text, err)
		}
		return m
	}
	refused := func(what, token string) {
		t.Helper()
		for i := 0; i < DefaultAuthFailures+1; i++ {
			if code, text := callAPI(t, ts, "GET", "/api/v1/me", token, ""); code != 401 || !strings.Contains(text, errBadToken.Error()) {
				t.Fatalf("%s, refusal %d: %d %s", what, i+1, code, text)
			}
		}
	}
	works := func(what, token string) {
		t.Helper()
		var me meView
		if code := getJSON(t, ts, token, "/api/v1/me", &me); code != 200 || me.Token == nil || (me.Token.Kind == store.TokenAPI && me.Token.ID != phone.ID) {
			t.Fatalf("%s: %d %+v", what, code, me.Token)
		}
	}
	session := login(phone.Token)

	// The owner gives the key a new secret: the same key, its old secret
	// and its session refused as ended tokens, not counted.
	logged := r.log.Mark()
	first := rekey("the owner, the phone's key", "secret")
	r.log.Await(t, logged, "the new secret's line", testkit.Msg("api key given a new secret", "user", "anna", "token", strconv.FormatInt(phone.ID, 10)))
	if first.Token == "" || first.Token == phone.Token || first.ID != phone.ID || first.Kind != store.TokenAPI || first.Label != phone.Label || !first.CreatedAt.Equal(phone.CreatedAt) {
		t.Fatalf("the answer: %+v, the key %+v", first, phone)
	}
	works("the new secret", first.Token)
	refused("the old secret", phone.Token)
	refused("the key's session", session)
	if keys, err := st.Lockout(DefaultAuthFailures, DefaultAuthWindow, DefaultAuthLockout).Keys(ctx); err != nil || len(keys) != 0 {
		t.Fatalf("the old secret counted: %v %v", keys, err)
	}
	list := tokensOf(t, r, "secret", "anna")
	if k := list[phone.ID]; k.Kind != store.TokenAPI || k.ExpiresAt == nil || !k.ExpiresAt.Equal(end) || !k.IdleExempt || k.Ended != "" || k.MadeBy != maker.ID {
		t.Errorf("the key with its new secret, listed: %+v, its maker %d", k, maker.ID)
	}
	if k := list[heir.ID]; k.MadeBy != phone.ID || k.Ended != "" {
		t.Errorf("its heir, listed: %+v, its maker %d", k, phone.ID)
	}
	if len(list) != 3 {
		t.Errorf("anna's tokens: %+v, want the laptop's key, the phone's and its heir", list)
	}

	// Asked by a session made from the key, that session stays.
	mine := login(first.Token)
	var me meView
	getJSON(t, ts, mine, "/api/v1/me", &me)
	logged = r.log.Mark()
	second := rekey("the key's own session", mine)
	r.log.Await(t, logged, "the line of the session kept", testkit.Msg("api key given a new secret", "token", strconv.FormatInt(phone.ID, 10), "kept_session", strconv.FormatInt(me.Token.ID, 10)))
	works("the session that asked", mine)
	works("the newer secret", second.Token)
	refused("the first new secret", first.Token)

	// Audited as token.rekey, a change line each; no secret kept anywhere.
	sums := changeSummaries(t, r, "token.rekey", "token:"+strconv.FormatInt(phone.ID, 10))
	if len(sums) != 2 {
		t.Fatalf("the change lines: %v", sums)
	}
	if subject, _ := sums[0]["subject"].(map[string]any); subject["name"] != phone.Label || subject["user"] != "anna" {
		t.Errorf("the change line's subject: %v", sums[0])
	}
	_, audit := callAPI(t, ts, "GET", "/api/v1/audit?limit=50", "secret", "")
	if !strings.Contains(audit, `"token.rekey"`) {
		t.Fatalf("no audit row of token.rekey: %s", audit)
	}
	for _, secret := range []string{first.Token, second.Token} {
		if strings.Contains(fmt.Sprint(sums), secret) || strings.Contains(audit, secret) {
			t.Errorf("a change line or the audit keeps a new secret")
		}
		r.log.HasNot(t, 0, "a new secret in the log", testkit.Contains(secret))
	}
}

// Who gives a key a new secret, and which (AP KeyLife, Block 5): the
// key's user, signed in or with another of its keys, and who manages the
// user; never the key itself (403), the settings file's key (409), an
// integration's key (403), a key that has ended (409), nor a session
// (400); a caller that neither is the user nor manages it 403, another
// user's token and a revoked one 404. A refused key keeps its secret.
func TestRekeyWho(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet(), withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "placeholder-pass-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	gina, _ := st.AddUser(ctx, store.User{Name: "gina", Role: store.RoleGuest}, "", 0)
	annaKey, annaSecret, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "Loxone"})
	_, annaOtherSecret, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "phone"})
	ended, endedSecret, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "old"})
	endToken(t, st, ended.ID)
	revoked, _, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "gone"})
	if _, err := st.RevokeToken(ctx, revoked.ID); err != nil {
		t.Fatal(err)
	}
	_, annaSessionSecret, _ := st.AddToken(ctx, anna.ID, store.TokenSession, "Firefox", time.Now().Add(SessionLifetime))
	annaSession, _, _ := st.TokenByHash(ctx, store.HashToken(annaSessionSecret))
	ginaKey, _, _ := st.AddKey(ctx, gina.ID, store.KeyOptions{Label: "tablet"})
	manager := userToken(t, st, "mia", store.RoleManager) // guests and integrations, not users
	i, integSecret := registerHA(t, st, []string{"read"})
	integ, err := st.Tokens(ctx, i.UserID)
	if err != nil || len(integ) != 1 {
		t.Fatalf("the integration's key: %+v %v", integ, err)
	}
	var me meView
	getJSON(t, ts, "secret", "/api/v1/me", &me)
	file := me.Token.ID
	rekey := func(what, bearer, user, id string, want int, says string) {
		t.Helper()
		code, text := callAPI(t, ts, "POST", "/api/v1/users/"+user+"/tokens/"+id+"/rekey", bearer, "")
		if code != want || (says != "" && !strings.Contains(errorOf(text), says)) {
			t.Errorf("%s: %d %s, want %d %q", what, code, text, want, says)
		}
		var m rekeyed
		if json.Unmarshal([]byte(text), &m); want == 200 && (m.Token == "" || id != strconv.FormatInt(m.ID, 10)) {
			t.Errorf("%s: no new secret in %s", what, text)
		}
	}
	id := func(n int64) string { return strconv.FormatInt(n, 10) }

	rekey("the bearer that very key", annaSecret, "anna", id(annaKey.ID), 403, "its own life")
	rekey("the settings file's key", "secret", store.BootstrapUser, id(file), 409, "settings file")
	rekey("an integration's key", "secret", i.UUID+":", id(integ[0].ID), 403, "integration's")
	rekey("an integration's key, by a holder of integrations", manager, i.UUID+":", id(integ[0].ID), 403, "integration's")
	rekey("a key that has ended", "secret", "anna", id(ended.ID), 409, "has ended")
	rekey("a session", "secret", "anna", id(annaSession.ID), 400, "is a session")
	rekey("a revoked key", "secret", "anna", id(revoked.ID), 404, "")
	rekey("another user's token", "secret", "gina", id(annaKey.ID), 404, "")
	rekey("no token id", "secret", "anna", "x", 400, "bad token id")
	rekey("anna, gina's key", annaSecret, "gina", id(ginaKey.ID), 403, "")
	rekey("a manager of guests, an operator's key", manager, "anna", id(annaKey.ID), 403, "")
	rekey("no bearer", "", "anna", id(annaKey.ID), 401, "")
	for _, c := range []struct{ what, secret string }{{"the settings file's key", "secret"}, {"the integration's key", integSecret}, {"anna's key", annaSecret}} {
		if code, text := callAPI(t, ts, "GET", "/api/v1/me", c.secret, ""); code != 200 {
			t.Errorf("%s, refused a new secret: its own %d %s", c.what, code, text)
		}
	}
	if ok, err := st.TokenEnded(ctx, store.HashToken(endedSecret), time.Now()); err != nil || !ok {
		t.Errorf("the ended key's secret, refused a new one: ended %v %v", ok, err)
	}

	rekey("anna signed in, her own key", annaSessionSecret, "anna", id(annaKey.ID), 200, "")
	rekey("another key of anna's, her key", annaOtherSecret, "anna", id(annaKey.ID), 200, "")
	rekey("the owner, anna's key", "secret", "anna", id(annaKey.ID), 200, "")
	rekey("a manager of guests, a guest's key", manager, "gina", id(ginaKey.ID), 200, "")
}

// After a new secret, POST /api/v1/tokens/revoke with the old secret's
// code ends nothing: 204, and the key and its heir stay; the new secret's
// code ends the key, its heir with it (AP KeyLife, Block 5;
// MobileApp.D112-A).
func TestRekeyRevokeCode(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	key, oldSecret, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "phone"})
	var heir rekeyed
	code, text := callAPI(t, ts, "POST", "/api/v1/users/anna/tokens", oldSecret, `{"label":"made by the phone's key"}`)
	if err := json.Unmarshal([]byte(text), &heir); err != nil || code != 201 {
		t.Fatalf("the heir: %d %s %v", code, text, err)
	}
	var m rekeyed
	code, text = callAPI(t, ts, "POST", "/api/v1/users/anna/tokens/"+strconv.FormatInt(key.ID, 10)+"/rekey", "secret", "")
	if err := json.Unmarshal([]byte(text), &m); err != nil || code != 200 || m.Token == "" {
		t.Fatalf("the new secret: %d %s %v", code, text, err)
	}
	alive := func(what, token string, want bool) {
		t.Helper()
		if code, _ := callAPI(t, ts, "GET", "/api/v1/me", token, ""); (code == 200) != want {
			t.Errorf("%s: %d, alive want %v", what, code, want)
		}
	}
	if code, text, _ := postCode(t, ts, codeBody(oldSecret), ""); code != 204 || text != "" {
		t.Fatalf("the old secret's code: %d %q", code, text)
	}
	alive("the key after its old secret's code", m.Token, true)
	alive("the heir after its maker's old secret's code", heir.Token, true)
	if code, _, _ := postCode(t, ts, codeBody(m.Token), ""); code != 204 {
		t.Fatalf("the new secret's code: %d", code)
	}
	alive("the key after its new secret's code", m.Token, false)
	alive("the heir after its maker's new secret's code", heir.Token, false)
}
