// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// A key made with an API key as the bearer names it (made_by), for its
// own user or another; one made by a session, of a sign-in or made from
// a key, names none (AP KeyLife, Block 2, D5-A). A revoke takes the keys
// a key made, and theirs, and names them in its answer, its log line
// and its change line, under the user's tokens as by the key itself; a
// user's removal takes them too.
func TestKeyHeirs(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet(), withLog(slog.LevelInfo), withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleManager}, "placeholder-pass-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddUser(ctx, store.User{Name: "gina", Role: store.RoleGuest}, "", 0); err != nil {
		t.Fatal(err)
	}
	annaKey, annaSecret, _ := st.AddToken(ctx, anna.ID, store.TokenAPI, "anna's", time.Time{})
	type made struct {
		Token string `json:"token"`
		ID    int64  `json:"id"`
	}
	create := func(bearer, user, label string) made {
		t.Helper()
		code, text := callAPI(t, ts, "POST", "/api/v1/users/"+user+"/tokens", bearer, `{"label":"`+label+`"}`)
		var m made
		if err := json.Unmarshal([]byte(text), &m); err != nil || code != 201 || m.Token == "" {
			t.Fatalf("a key %q for %s: %d %s %v", label, user, code, text, err)
		}
		return m
	}
	madeBy := func(user string, id int64) int64 {
		t.Helper()
		var list []tokenView
		getJSON(t, ts, "secret", "/api/v1/users/"+user+"/tokens", &list)
		for _, tk := range list {
			if tk.ID == id {
				return tk.MadeBy
			}
		}
		t.Fatalf("no token %d of %s: %+v", id, user, list)
		return 0
	}
	session := func(path, bearer, body string) string {
		t.Helper()
		code, text := callAPI(t, ts, "POST", path, bearer, body)
		var m made
		if err := json.Unmarshal([]byte(text), &m); err != nil || code != 200 || m.Token == "" {
			t.Fatalf("a session by %s: %d %s %v", path, code, text, err)
		}
		return m.Token
	}
	alive := func(what, token string, want bool) {
		t.Helper()
		if code, _ := callAPI(t, ts, "GET", "/api/v1/me", token, ""); (code == 200) != want {
			t.Errorf("%s: %d, alive want %v", what, code, want)
		}
	}

	child := create(annaSecret, "anna", "child")
	ginas := create(annaSecret, "gina", "gina's")
	grandchild := create(child.Token, "gina", "grandchild")
	bySession := create(session("/api/v1/login", "", `{"user":"anna","password":"placeholder-pass-1"}`), "anna", "by a sign-in's session")
	byKeySession := create(session("/api/v1/login/key", annaSecret, ""), "anna", "by a key's session")
	for _, c := range []struct {
		what, user string
		id, want   int64
	}{
		{"a key made with a key", "anna", child.ID, annaKey.ID},
		{"another user's key made with a key", "gina", ginas.ID, annaKey.ID},
		{"a key made with that key", "gina", grandchild.ID, child.ID},
		{"a key made by a sign-in's session", "anna", bySession.ID, 0},
		{"a key made by a key's session", "anna", byKeySession.ID, 0},
		{"a key made by no request", "anna", annaKey.ID, 0},
	} {
		if got := madeBy(c.user, c.id); got != c.want {
			t.Errorf("%s: made by %d, want %d", c.what, got, c.want)
		}
	}
	var me meView
	getJSON(t, ts, child.Token, "/api/v1/me", &me)
	if me.Token == nil || me.Token.MadeBy != annaKey.ID {
		t.Errorf("GET /api/v1/me of the child: %+v", me.Token)
	}

	// The revoke under the user's tokens names the heirs, in its answer,
	// its log line and its change line.
	logged := r.log.Mark()
	code, text := callAPI(t, ts, "DELETE", "/api/v1/users/anna/tokens/"+strconv.FormatInt(annaKey.ID, 10), "secret", "")
	var answer struct {
		OK    bool    `json:"ok"`
		ID    int64   `json:"id"`
		Heirs []int64 `json:"heirs"`
	}
	want := []int64{child.ID, ginas.ID, grandchild.ID}
	if err := json.Unmarshal([]byte(text), &answer); err != nil || code != 200 || !answer.OK || answer.ID != annaKey.ID || fmt.Sprint(answer.Heirs) != fmt.Sprint(want) {
		t.Fatalf("the revoke: %d %s %v, want heirs %v", code, text, err, want)
	}
	r.log.Await(t, logged, "the revoke's line naming the heirs", testkit.Msg("token revoked", "token", strconv.FormatInt(annaKey.ID, 10), "heirs", fmt.Sprint(want)))
	alive("the key", annaSecret, false)
	alive("the child", child.Token, false)
	alive("another user's heir", ginas.Token, false)
	alive("the grandchild", grandchild.Token, false)
	alive("a key made by a sign-in's session", bySession.Token, true)
	alive("a key made by a key's session", byKeySession.Token, true)
	lines := changeSummaries(t, r, "token.revoke", "token:"+strconv.FormatInt(annaKey.ID, 10))
	if len(lines) != 1 {
		t.Errorf("the change lines of the revoke: %v", lines)
	} else if subject, _ := lines[0]["subject"].(map[string]any); fmt.Sprint(subject["heirs"]) != fmt.Sprint([]any{float64(child.ID), float64(ginas.ID), float64(grandchild.ID)}) {
		t.Errorf("the change line of the revoke naming the heirs: %v", lines[0])
	}

	// A key revoking itself names its heirs too; one without names none.
	maker := create(byKeySession.Token, "anna", "maker")
	heir := create(maker.Token, "gina", "heir of the maker")
	code, text = callAPI(t, ts, "DELETE", "/api/v1/me/token", maker.Token, "")
	answer.Heirs = nil
	if err := json.Unmarshal([]byte(text), &answer); err != nil || code != 200 || answer.ID != maker.ID || fmt.Sprint(answer.Heirs) != fmt.Sprint([]int64{heir.ID}) {
		t.Fatalf("a key revoking itself: %d %s %v", code, text, err)
	}
	alive("the heir of a key that revoked itself", heir.Token, false)
	code, text = callAPI(t, ts, "DELETE", "/api/v1/users/anna/tokens/"+strconv.FormatInt(bySession.ID, 10), "secret", "")
	answer.Heirs = nil
	if err := json.Unmarshal([]byte(text), &answer); err != nil || code != 200 || answer.Heirs == nil || len(answer.Heirs) != 0 {
		t.Fatalf("a revoke without heirs: %d %s %v", code, text, err)
	}

	// A user's removal revokes its keys, and the keys they made.
	if _, err := st.AddUser(ctx, store.User{Name: "carol", Role: store.RoleManager}, "", 0); err != nil {
		t.Fatal(err)
	}
	carols := create("secret", "carol", "carol's")
	ofCarols := create(carols.Token, "gina", "made with carol's key")
	ginasOwn := create("secret", "gina", "gina's own")
	if code, text := callAPI(t, ts, "DELETE", "/api/v1/users/carol", "secret", ""); code != 200 {
		t.Fatalf("carol removed: %d %s", code, text)
	}
	alive("the heir of a removed user's key", ofCarols.Token, false)
	alive("a key of the heir's user beside it", ginasOwn.Token, true)
}

// A key made with the settings file's key as the bearer, then put into
// the file, is the file's key at the reload (SetToken), no heir of the
// key it replaces: it lets the owner in, at that reload and the next.
func TestSettingsFileTakesAKeyItsOldKeyMade(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet(), withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, s := r.ts, r.s
	code, text := callAPI(t, ts, "POST", "/api/v1/users/owner/tokens", "secret", `{"label":"laptop"}`)
	var made struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(text), &made); err != nil || code != 201 || made.Token == "" {
		t.Fatalf("a key made with the file's key: %d %s %v", code, text, err)
	}
	for _, at := range []string{"the reload", "the next reload"} {
		if err := s.SetToken(t.Context(), made.Token); err != nil {
			t.Fatalf("%s: %v", at, err)
		}
		if code, text := callAPI(t, ts, "GET", "/api/v1/me", made.Token, ""); code != 200 {
			t.Fatalf("the file's new key after %s: %d %s", at, code, text)
		}
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", "secret", ""); code != 401 {
		t.Fatalf("the file's old key after the reloads: %d", code)
	}
}
