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

// An API key's end date (AP KeyLife, Block 4; D4-A, D6-A): POST …/tokens
// makes a key with expires_at, one that has passed or is no time 400 and
// nothing made, null none; at its date the key and its session are
// refused as ended tokens, not counted toward the lockout, and the list
// says expired; PUT …/tokens/{id} moves it later, and the key and its
// session live again, null removes it, a date that has passed is 400 and
// changes nothing; audited as token.update, its change line from and to,
// and logged. The store's clock starts ten days back and catches up with
// the real time, which the server's own checks keep, as in TestKeyIdle.
func TestKeyEndDate(t *testing.T) {
	t.Parallel()
	day := 24 * time.Hour
	clock := testkit.NewClock(time.Now().UTC().Add(-10 * day).Truncate(time.Second))
	r := startServer(t, quiet(), withLog(slog.LevelInfo), withOptions(storeOptionsAt(t, t.TempDir(), "secret", clock.Now)))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	if _, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "", 0); err != nil {
		t.Fatal(err)
	}
	at := func(d time.Duration) string { return clock.Now().Add(d).Format(time.RFC3339) }
	type made struct {
		Token string `json:"token"`
		ID    int64  `json:"id"`
	}
	post := func(what, body string, want int, says string) made {
		t.Helper()
		code, text := callAPI(t, ts, "POST", "/api/v1/users/anna/tokens", "secret", body)
		var m made
		json.Unmarshal([]byte(text), &m)
		if code != want || (says != "" && !strings.Contains(errorOf(text), says)) {
			t.Fatalf("%s: %d %s, want %d %q", what, code, text, want, says)
		}
		return m
	}

	// POST: a date that has passed, now, or no time makes no key; null none.
	before := len(tokensOf(t, r, "secret", "anna"))
	post("an end an hour ago", `{"label":"x","expires_at":"`+at(-time.Hour)+`"}`, 400, "has passed")
	post("an end now", `{"label":"x","expires_at":"`+at(0)+`"}`, 400, "has passed")
	post("an end that is no time", `{"label":"x","expires_at":"tomorrow"}`, 400, "expires_at")
	post("an end that is a number", `{"label":"x","expires_at":7}`, 400, "expires_at")
	if n := len(tokensOf(t, r, "secret", "anna")); n != before {
		t.Fatalf("a refused end made a key: %d tokens, %d before", n, before)
	}
	none := post("an end of null", `{"label":"no end","expires_at":null}`, 201, "")
	if k := tokensOf(t, r, "secret", "anna")[none.ID]; k.ExpiresAt != nil {
		t.Errorf("a key made with null: %+v", k)
	}

	// A key ending in two days, and its session.
	end := clock.Now().Add(2 * day)
	logged := r.log.Mark()
	key := post("an end in two days", `{"label":"SM-X, login by fingerprint","expires_at":"`+end.Format(time.RFC3339)+`"}`, 201, "")
	r.log.Await(t, logged, "the creation's line", testkit.Contains("api key created", "expires_at="))
	if k := tokensOf(t, r, "secret", "anna")[key.ID]; k.ExpiresAt == nil || !k.ExpiresAt.Equal(end) || k.Ended != "" {
		t.Fatalf("the key made with an end date: %+v", k)
	}
	code, text := callAPI(t, ts, "POST", "/api/v1/login/key", key.Token, "")
	var session made
	if err := json.Unmarshal([]byte(text), &session); err != nil || code != 200 {
		t.Fatalf("the key's session: %d %s %v", code, text, err)
	}

	// Past its date, the key and its session are refused as ended tokens,
	// which count toward no lockout; listed, the key says expired.
	clock.Add(3 * day)
	for i := 0; i < DefaultAuthFailures+1; i++ {
		for _, c := range []struct{ what, method, path, token string }{
			{"the key past its end", "POST", "/api/v1/login/key", key.Token},
			{"its session", "GET", "/api/v1/cubes", session.Token},
		} {
			if code, text := callAPI(t, ts, c.method, c.path, c.token, ""); code != 401 || !strings.Contains(text, errBadToken.Error()) {
				t.Fatalf("%s, refusal %d: %d %s", c.what, i+1, code, text)
			}
		}
	}
	if keys, err := st.Lockout(DefaultAuthFailures, DefaultAuthWindow, DefaultAuthLockout).Keys(ctx); err != nil || len(keys) != 0 {
		t.Fatalf("a key past its end counted: %v %v", keys, err)
	}
	if k := tokensOf(t, r, "secret", "anna")[key.ID]; k.Ended != store.EndedExpired || k.ExpiresAt == nil || !k.ExpiresAt.Equal(end) {
		t.Fatalf("the key past its end, listed: %+v", k)
	}

	// Moved later, the key and its session live again (D6-A).
	put := func(what, body string, want int, says string) idleView {
		t.Helper()
		code, text := callAPI(t, ts, "PUT", "/api/v1/users/anna/tokens/"+strconv.FormatInt(key.ID, 10), "secret", body)
		var v idleView
		json.Unmarshal([]byte(text), &v)
		if code != want || (says != "" && !strings.Contains(errorOf(text), says)) {
			t.Fatalf("%s: %d %s, want %d %q", what, code, text, want, says)
		}
		return v
	}
	later := clock.Now().Add(5 * day)
	logged = r.log.Mark()
	if v := put("the end moved later", `{"expires_at":"`+later.Format(time.RFC3339)+`"}`, 200, ""); v.ID != key.ID || v.ExpiresAt == nil || !v.ExpiresAt.Equal(later) || v.Ended != "" {
		t.Fatalf("the answer: %+v", v)
	}
	r.log.Await(t, logged, "the move's line", testkit.Contains("token changed", "token="+strconv.FormatInt(key.ID, 10), "expires_at="))
	if code, text := callAPI(t, ts, "GET", "/api/v1/cubes", session.Token, ""); code != 200 {
		t.Fatalf("the session, its key's end moved later: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "GET", "/api/v1/me", key.Token, ""); code != 200 {
		t.Fatalf("the key, its end moved later: %d %s", code, text)
	}

	// A date that has passed is refused and changes nothing; null removes
	// the end.
	put("the end moved to the past", `{"expires_at":"`+at(-time.Minute)+`"}`, 400, "has passed")
	put("the end moved to now", `{"expires_at":"`+at(0)+`"}`, 400, "has passed")
	put("an end that is no time", `{"expires_at":"soon"}`, 400, "expires_at")
	if k := tokensOf(t, r, "secret", "anna")[key.ID]; k.ExpiresAt == nil || !k.ExpiresAt.Equal(later) {
		t.Fatalf("a refused end changed the key: %+v", k)
	}
	logged = r.log.Mark()
	if v := put("the end removed", `{"expires_at":null}`, 200, ""); v.ExpiresAt != nil || v.Ended != "" {
		t.Fatalf("the end removed: %+v", v)
	}
	r.log.Await(t, logged, "the removal's line", testkit.Msg("token changed", "token", strconv.FormatInt(key.ID, 10), "expires_at", "none"))
	if k := tokensOf(t, r, "secret", "anna")[key.ID]; k.ExpiresAt != nil {
		t.Fatalf("the end removed, listed: %+v", k)
	}

	// Its change lines say from and to: an end moved, then removed.
	name := func(v any) string {
		if v == nil {
			return "none"
		}
		at, err := time.Parse(time.RFC3339, fmt.Sprint(v))
		switch {
		case err != nil:
			return fmt.Sprint(v)
		case at.Equal(end):
			return "end"
		case at.Equal(later):
			return "later"
		}
		return at.String()
	}
	var changes []string
	for _, sum := range changeSummaries(t, r, "token.update", "token:"+strconv.FormatInt(key.ID, 10)) {
		c, _ := sum["changes"].(map[string]any)
		e, _ := c["expires_at"].(map[string]any)
		changes = append(changes, name(e["from"])+"→"+name(e["to"]))
	}
	if fmt.Sprint(changes) != "[end→later later→none]" {
		t.Errorf("the change lines of the end moved, then removed: %v", changes)
	}
}

// Who moves an API key's end date (AP KeyLife, Block 4; D3-A, D4-A): the
// key's user, signed in or by another of its keys, a session made from the
// key, who manages the user, and for an integration's key a holder of
// users or integrations; never the key itself (403), and never the
// settings file's key (409); a session is 400, a caller that neither is
// the user nor manages it 403, another user's token 404; and idle_exempt
// beside an end keeps its own rule (D7-A).
func TestKeyEndWho(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet(), withOptions(storeOptions(t, t.TempDir(), "secret")))
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
	annaOther, annaOtherSecret, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "phone"})
	_, annaSessionSecret, _ := st.AddToken(ctx, anna.ID, store.TokenSession, "Firefox", time.Now().Add(SessionLifetime))
	annaSession, _, _ := st.TokenByHash(ctx, store.HashToken(annaSessionSecret))
	ginaKey, _, _ := st.AddKey(ctx, gina.ID, store.KeyOptions{Label: "tablet"})
	ownKey, ownSecret, _ := st.AddKey(ctx, owner.ID, store.KeyOptions{Label: "laptop"})
	manager := userToken(t, st, "mia", store.RoleManager) // guests and integrations, not users
	i, _ := registerHA(t, st, []string{"read"})
	integ, err := st.Tokens(ctx, i.UserID)
	if err != nil || len(integ) != 1 {
		t.Fatalf("the integration's key: %+v %v", integ, err)
	}
	var me meView
	getJSON(t, ts, "secret", "/api/v1/me", &me)
	file := me.Token.ID
	soon := func(h int) string {
		return `{"expires_at":"` + time.Now().Add(time.Duration(h)*time.Hour).UTC().Format(time.RFC3339) + `"}`
	}
	put := func(what, bearer, user string, id int64, body string, want int, says string) {
		t.Helper()
		code, text := callAPI(t, ts, "PUT", "/api/v1/users/"+user+"/tokens/"+strconv.FormatInt(id, 10), bearer, body)
		if code != want || (says != "" && !strings.Contains(errorOf(text), says)) {
			t.Errorf("%s: %d %s, want %d %q", what, code, text, want, says)
		}
	}

	put("the owner, anna's key", "secret", "anna", annaKey.ID, soon(24), 200, "")
	put("anna signed in, her own key", annaSessionSecret, "anna", annaKey.ID, soon(48), 200, "")
	put("another key of anna's, her key", annaOtherSecret, "anna", annaKey.ID, soon(72), 200, "")
	put("anna signed in, removing the end", annaSessionSecret, "anna", annaKey.ID, `{"expires_at":null}`, 200, "")
	put("the bearer that very key", annaSecret, "anna", annaKey.ID, soon(24), 403, "its own life")
	put("the bearer that very key, removing its end", annaSecret, "anna", annaKey.ID, `{"expires_at":null}`, 403, "its own life")
	put("the owner's key, its own end", ownSecret, store.BootstrapUser, ownKey.ID, soon(24), 403, "its own life")
	put("the settings file's key", "secret", store.BootstrapUser, file, soon(24), 409, "settings file")
	put("the settings file's key, null", "secret", store.BootstrapUser, file, `{"expires_at":null}`, 409, "settings file")
	put("a session", "secret", "anna", annaSession.ID, soon(24), 400, "is a session")
	put("anna, gina's key", annaSecret, "gina", ginaKey.ID, soon(24), 403, "")
	put("a manager of guests, a guest's key", manager, "gina", ginaKey.ID, soon(24), 200, "")
	put("a manager of guests, an operator's key", manager, "anna", annaKey.ID, soon(24), 403, "")
	put("another user's token", "secret", "gina", annaKey.ID, soon(24), 404, "")
	put("the owner, an integration's key", "secret", i.UUID+":", integ[0].ID, soon(24), 200, "")
	put("a holder of integrations, an integration's key", manager, i.UUID+":", integ[0].ID, soon(48), 200, "")
	put("an operator, an integration's key", annaSecret, i.UUID+":", integ[0].ID, soon(24), 403, "")
	put("an integration's key, an end and kept", "secret", i.UUID+":", integ[0].ID, `{"expires_at":null,"idle_exempt":true}`, 400, "always held to the idle limit")
	put("anna signed in, an end and kept", annaSessionSecret, "anna", annaOther.ID, `{"expires_at":null,"idle_exempt":true}`, 403, "only who manages user anna")
	if k := tokensOf(t, r, "secret", "anna")[annaOther.ID]; k.ExpiresAt != nil || k.IdleExempt {
		t.Errorf("a refused change of two fields changed one: %+v", k)
	}
	if k := tokensOf(t, r, "secret", i.UUID+":")[integ[0].ID]; k.ExpiresAt == nil {
		t.Errorf("the integration's key's end: %+v", k)
	}

	// A session made from a key is not the key.
	code, text := callAPI(t, ts, "POST", "/api/v1/login/key", ownSecret, "")
	var made struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(text), &made); err != nil || code != 200 {
		t.Fatalf("the owner's key's session: %d %s %v", code, text, err)
	}
	put("a session made from that key", made.Token, store.BootstrapUser, ownKey.ID, soon(24), 200, "")
}

// The settings file's key never ends by a date (AP KeyLife, Block 4,
// D3-A): the owner makes a key with an end date and puts its secret into
// the settings file (SetToken, as a reading of the file does); past that
// date the key still lets the owner in, and the list shows it from the
// settings file, living, without an end date. A row of it that holds an
// end date all the same, as an older binary may leave it, changes
// nothing of that, and its PUT, with an end, without one or with the
// exclusion, and its DELETE stay 409; the next reading of the file clears
// that end date from its row. The store's clock starts ten days back, as
// in TestKeyEndDate.
func TestSettingsKeyEndDate(t *testing.T) {
	t.Parallel()
	day := 24 * time.Hour
	clock := testkit.NewClock(time.Now().UTC().Add(-10 * day).Truncate(time.Second))
	r := startServer(t, quiet(), withOptions(storeOptionsAt(t, t.TempDir(), "secret", clock.Now)))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	other := userToken(t, st, "olga", store.RoleOwner)
	path := "/api/v1/users/" + store.BootstrapUser + "/tokens"
	code, text := callAPI(t, ts, "POST", path, "secret", `{"label":"laptop","expires_at":"`+clock.Now().Add(2*day).Format(time.RFC3339)+`"}`)
	var made struct {
		Token string `json:"token"`
		ID    int64  `json:"id"`
	}
	if err := json.Unmarshal([]byte(text), &made); err != nil || code != 201 {
		t.Fatalf("the key with an end date: %d %s %v", code, text, err)
	}
	if err := r.s.SetToken(ctx, made.Token); err != nil {
		t.Fatal(err)
	}
	clock.Add(3 * day)
	key := path + "/" + strconv.FormatInt(made.ID, 10)
	check := func(what string) {
		t.Helper()
		if code, text := callAPI(t, ts, "GET", "/api/v1/cubes", made.Token, ""); code != 200 {
			t.Errorf("%s: the settings file's key %d %s, want 200", what, code, text)
		}
		if k := tokensOf(t, r, other, store.BootstrapUser)[made.ID]; !k.FromSettings || k.Ended != "" {
			t.Errorf("%s: listed %+v, want from the settings file and living", what, k)
		}
		for _, body := range []string{`{"expires_at":null}`, `{"expires_at":"` + clock.Now().Add(day).Format(time.RFC3339) + `"}`, `{"idle_exempt":true}`} {
			if code, text := callAPI(t, ts, "PUT", key, other, body); code != 409 || !strings.Contains(errorOf(text), "settings file") {
				t.Errorf("%s: PUT %s: %d %s, want 409", what, body, code, text)
			}
		}
		if code, text := callAPI(t, ts, "DELETE", key, other, ""); code != 409 {
			t.Errorf("%s: DELETE: %d %s, want 409", what, code, text)
		}
	}
	check("past the end date it was made with")
	if k := tokensOf(t, r, other, store.BootstrapUser)[made.ID]; k.ExpiresAt != nil {
		t.Errorf("the settings file's key keeps the end date it was made with: %+v", k)
	}

	// Its row holding an end date all the same, past.
	if err := st.TouchToken(ctx, made.ID, "", clock.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	check("its row holding an end date that has passed")
	if err := r.s.SetToken(ctx, made.Token); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := st.Token(ctx, made.ID); !got.ExpiresAt.IsZero() {
		t.Errorf("the next reading of the file left the end date in its row: %v", got.ExpiresAt)
	}
}

// A time that reads as the zero time, the year 1 in UTC, is neither an
// end date nor "none" (AP KeyLife, Block 4): expires_at of an API key's
// POST and PUT and of a user's POST and PUT is 400 and changes nothing,
// where it read as null before.
func TestYearOneIsNoEnd(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet(), withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	end := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	if _, err := st.AddUser(ctx, store.User{Name: "gina", Role: store.RoleGuest, ExpiresAt: end}, "", 0); err != nil {
		t.Fatal(err)
	}
	owner, ok, _ := st.User(ctx, store.BootstrapUser)
	if !ok {
		t.Fatal("no owner")
	}
	key, _, err := st.AddKey(ctx, owner.ID, store.KeyOptions{Label: "laptop", Expires: end})
	if err != nil {
		t.Fatal(err)
	}
	const zero = `"0001-01-01T00:00:00Z"`
	before := len(tokensOf(t, r, "secret", store.BootstrapUser))
	for _, c := range []struct{ what, method, path, body string }{
		{"a key made", "POST", "/api/v1/users/" + store.BootstrapUser + "/tokens", `{"label":"year one","expires_at":` + zero + `}`},
		{"a key moved", "PUT", "/api/v1/users/" + store.BootstrapUser + "/tokens/" + strconv.FormatInt(key.ID, 10), `{"expires_at":` + zero + `}`},
		{"a user made", "POST", "/api/v1/users", `{"name":"yuri","role":"guest","expires_at":` + zero + `}`},
		{"a user changed", "PUT", "/api/v1/users/gina", `{"expires_at":` + zero + `}`},
	} {
		if code, text := callAPI(t, ts, c.method, c.path, "secret", c.body); code != 400 || !strings.Contains(errorOf(text), "expires_at") {
			t.Errorf("%s with the year 1: %d %s, want 400", c.what, code, text)
		}
	}
	if n := len(tokensOf(t, r, "secret", store.BootstrapUser)); n != before {
		t.Errorf("a key was made with the year 1: %d tokens, %d before", n, before)
	}
	if k := tokensOf(t, r, "secret", store.BootstrapUser)[key.ID]; k.ExpiresAt == nil || !k.ExpiresAt.Equal(end) {
		t.Errorf("the key's end after the year 1: %+v", k)
	}
	if _, ok, _ := st.User(ctx, "yuri"); ok {
		t.Error("a user was made with the year 1")
	}
	if u, _, _ := st.User(ctx, "gina"); !u.ExpiresAt.Equal(end) {
		t.Errorf("the user's end after the year 1: %v, want %v", u.ExpiresAt, end)
	}
}
