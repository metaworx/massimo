// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

func decodeUserView(text string) userView {
	var u userView
	json.Unmarshal([]byte(text), &u)
	return u
}

// The users routes: create, list, change, remove, the own user, a
// guests holder, the tokens, the last owner.
func TestUsersRoutes(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, s := r.ts, r.s
	st := s.opts.Store
	ctx := t.Context()
	owner := "secret"
	users := "/api/v1/users"
	code, text := callAPI(t, ts, "POST", users, owner, `{"name":"Anna","display_name":"Anna","role":"operator","password":"a-pass-001"}`)
	anna := decodeUserView(text)
	if code != 201 || anna.Name != "anna" || anna.Role != store.RoleOperator || !anna.HasPassword || anna.InvitedBy != store.BootstrapUser {
		t.Fatalf("create: %d %s", code, text)
	}
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"name":"anna","password":"a-pass-003"}`, 409},
		{`{"name":"x y"}`, 400},
		{`{"name":"bob","role":"nope"}`, 400},
		{`{"name":"bob","expires_at":"soon"}`, 400},
		{`{"name":"bob","unknown":1}`, 400},
		{`{"name":"guest1","role":"guest","expires_at":"2026-12-24T18:00:00Z","password":"g-pass-001"}`, 201},
	} {
		if code, text := callAPI(t, ts, "POST", users, owner, c.body); code != c.want {
			t.Errorf("create %s: %d %s, want %d", c.body, code, text, c.want)
		}
	}
	code, text = callAPI(t, ts, "GET", users, owner, "")
	var list []userView
	json.Unmarshal([]byte(text), &list)
	if code != 200 || len(list) != 3 || list[0].Name != "anna" || list[1].Name != "guest1" || list[1].ExpiresAt == nil || list[2].Name != store.BootstrapUser {
		t.Fatalf("list: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "GET", users+"/nobody", owner, ""); code != 404 {
		t.Fatalf("unknown user: %d", code)
	}
	code, text = callAPI(t, ts, "PUT", users+"/anna", owner, `{"display_name":"Anna S.","role":"manager"}`)
	if u := decodeUserView(text); code != 200 || u.DisplayName != "Anna S." || u.Role != store.RoleManager {
		t.Fatalf("change: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", users+"/guest1", owner, `{"expires_at":null}`); code != 200 {
		t.Fatalf("clear the end date: %d", code)
	}
	if u, _, _ := st.User(ctx, "guest1"); !u.ExpiresAt.IsZero() {
		t.Fatal("end date not cleared")
	}
	for _, body := range []string{`{"name":"x"}`, `{"unknown":1}`, `{"role":"nope"}`} {
		if code, _ := callAPI(t, ts, "PUT", users+"/anna", owner, body); code != 400 {
			t.Errorf("change %s: %d", body, code)
		}
	}

	// A password set by someone else ends the user's sessions.
	code, text = callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"a-pass-001"}`)
	var sess struct {
		Token string `json:"token"`
	}
	json.Unmarshal([]byte(text), &sess)
	if code != 200 || sess.Token == "" {
		t.Fatalf("login: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", users+"/anna", owner, `{"password":"a-pass-002"}`); code != 200 {
		t.Fatalf("password by the owner: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", sess.Token, ""); code != 401 {
		t.Fatalf("session after a password reset: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"a-pass-002"}`); code != 200 {
		t.Fatalf("login with the new password: %d", code)
	}

	// The own user: display name and password with the current one.
	code, text = callAPI(t, ts, "POST", users+"/anna/tokens", owner, `{"label":"Loxone"}`)
	var key struct {
		Token string `json:"token"`
		ID    int64  `json:"id"`
		Kind  string `json:"kind"`
	}
	json.Unmarshal([]byte(text), &key)
	if code != 201 || len(key.Token) != 64 || key.Kind != store.TokenAPI || key.ID == 0 {
		t.Fatalf("key: %d %s", code, text)
	}
	annaKey := key.Token
	if code, _ := callAPI(t, ts, "PUT", users+"/anna", annaKey, `{"display_name":"Me"}`); code != 200 {
		t.Fatalf("own display name: %d", code)
	}
	for _, body := range []string{`{"role":"owner"}`, `{"disabled":false}`, `{"expires_at":null}`} {
		if code, _ := callAPI(t, ts, "PUT", users+"/anna", annaKey, body); code != 403 {
			t.Errorf("own %s: %d", body, code)
		}
	}
	if code, text := callAPI(t, ts, "PUT", users+"/anna", annaKey, `{"password":"a-pass-003","current_password":"wrong"}`); code != 403 || !strings.Contains(text, "current_password") {
		t.Fatalf("own password with the wrong current one: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", users+"/anna", annaKey, `{"password":"a-pass-003","current_password":"a-pass-002"}`); code != 200 {
		t.Fatalf("own password: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"a-pass-003"}`); code != 200 {
		t.Fatalf("login with the own new password: %d", code)
	}

	// A manager (anna: guests and passwords) sees every user, manages
	// guests and itself, and may set the password of none but those it
	// outranks: not the owner's.
	code, text = callAPI(t, ts, "GET", users, annaKey, "")
	json.Unmarshal([]byte(text), &list)
	may := map[string]userMay{}
	for _, u := range list {
		if u.May != nil {
			may[u.Name] = *u.May
		}
	}
	if code != 200 || len(may) < 3 || may["owner"] != (userMay{}) || may["guest1"] != (userMay{Manage: true, Password: true}) ||
		may["anna"] != (userMay{Manage: false, Password: true}) {
		t.Fatalf("a manager's list: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "GET", users+"/owner", annaKey, ""); code != 200 {
		t.Fatalf("a manager reads the owner: %d", code)
	}
	if code, text := callAPI(t, ts, "PUT", users+"/owner", annaKey, `{"password":"taken-over"}`); code != 403 || errorOf(text) != "only an owner changes an owner" {
		t.Fatalf("a manager sets the owner's password: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "POST", users, annaKey, `{"name":"g2","role":"guest","password":"guest-pass-2"}`); code != 201 {
		t.Fatalf("a manager creates a guest: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", users, annaKey, `{"name":"v","role":"viewer"}`); code != 403 {
		t.Fatalf("a manager creates a viewer: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", users+"/g2", annaKey, `{"role":"viewer"}`); code != 403 {
		t.Fatalf("a manager promotes a guest: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", users+"/g2", annaKey, `{"disabled":true}`); code != 200 {
		t.Fatalf("a manager disables a guest: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", users+"/g2", annaKey, ""); code != 200 {
		t.Fatalf("a manager removes a guest: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", users+"/owner", annaKey, ""); code != 403 {
		t.Fatalf("a manager removes the owner: %d", code)
	}

	// A viewer reaches its own user only, and may not remove it.
	bobKey := userToken(t, st, "bob", store.RoleViewer)
	if code, _ := callAPI(t, ts, "PUT", users+"/bob", bobKey, `{"display_name":"B"}`); code != 200 {
		t.Fatalf("a viewer's own display name: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", users+"/bob", bobKey, ""); code != 403 {
		t.Fatalf("a viewer removes itself: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", users, bobKey, ""); code != 403 {
		t.Fatalf("a viewer lists the users: %d", code)
	}
	if code, text := callAPI(t, ts, "GET", users+"/bob", bobKey, ""); code != 200 || decodeUserView(text).DisplayName != "B" {
		t.Fatalf("a viewer reads itself: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "GET", users+"/anna", bobKey, ""); code != 403 {
		t.Fatalf("a viewer reads another user: %d", code)
	}

	// Tokens: listed, revoked by id, the user's own only.
	code, text = callAPI(t, ts, "GET", users+"/anna/tokens", owner, "")
	var tokens []tokenView
	json.Unmarshal([]byte(text), &tokens)
	found := false
	for _, tk := range tokens {
		found = found || tk.ID == key.ID && tk.Kind == store.TokenAPI && tk.Label == "Loxone"
	}
	if code != 200 || len(tokens) < 2 || !found {
		t.Fatalf("tokens: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "POST", users+"/anna/tokens", owner, `{}`); code != 400 {
		t.Fatalf("key without a label: %d", code)
	}
	id := strconv.FormatInt(key.ID, 10)
	if code, _ := callAPI(t, ts, "DELETE", users+"/bob/tokens/"+id, owner, ""); code != 404 {
		t.Fatalf("another user's token: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", users+"/anna/tokens/"+id, annaKey, ""); code != 200 {
		t.Fatalf("revoke the own key: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", annaKey, ""); code != 401 {
		t.Fatalf("revoked key: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", users+"/anna/tokens/"+id, owner, ""); code != 404 {
		t.Fatalf("revoke twice: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", users+"/anna/tokens/x", owner, ""); code != 400 {
		t.Fatalf("bad token id: %d", code)
	}

	// The last owner stays; a user is removed with its tokens.
	if code, _ := callAPI(t, ts, "DELETE", users+"/owner", owner, ""); code != 409 {
		t.Fatalf("remove the last owner: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", users+"/owner", owner, `{"disabled":true}`); code != 409 {
		t.Fatalf("disable the last owner: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", users+"/anna", owner, ""); code != 200 {
		t.Fatalf("remove anna: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", users+"/anna", owner, ""); code != 404 {
		t.Fatalf("removed user: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"a-pass-003"}`); code != 401 {
		t.Fatalf("login of a removed user: %d", code)
	}
}

// The settings file's key (MASSIMO_API_TOKEN) is marked from_settings in
// its user's token list and refused by id with 409, by itself and by
// another owner, as DELETE /api/v1/me/token refuses it: the next start
// could not add it back, its hash being taken (AP MobileApp v1.4,
// D86-A). The mark is the file's token, not the label: a key given that
// label by hand is not marked and is revoked; and the mark follows the
// file's token when it changes.
func TestSettingsFileKey(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, s := r.ts, r.s
	ctx := t.Context()
	olga := userToken(t, s.opts.Store, "olga", store.RoleOwner)
	type row struct {
		ID           int64  `json:"id"`
		Kind         string `json:"kind"`
		Label        string `json:"label"`
		FromSettings *bool  `json:"from_settings"`
	}
	// marked lists the owner's tokens and returns the ids marked.
	marked := func(what string) []int64 {
		t.Helper()
		code, text := callAPI(t, ts, "GET", "/api/v1/users/owner/tokens", olga, "")
		var rows []row
		if err := json.Unmarshal([]byte(text), &rows); code != 200 || err != nil {
			t.Fatalf("%s: the owner's tokens: %d %s", what, code, text)
		}
		var ids []int64
		for _, tk := range rows {
			if tk.FromSettings == nil {
				continue
			}
			if !*tk.FromSettings || tk.Kind != store.TokenAPI {
				t.Fatalf("%s: from_settings on %+v: true on the settings file's key alone, absent elsewhere", what, tk)
			}
			ids = append(ids, tk.ID)
		}
		return ids
	}
	code, text := callAPI(t, ts, "POST", "/api/v1/users/owner/tokens", olga, `{"label":"`+store.BootstrapLabel+`"}`)
	var byHand struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(text), &byHand); code != 201 || err != nil {
		t.Fatalf("a key labelled %s by hand: %d %s", store.BootstrapLabel, code, text)
	}
	file, ok, err := s.opts.Store.TokenByHash(ctx, store.HashToken("secret"))
	if err != nil || !ok {
		t.Fatalf("the settings file's key: %v %v", ok, err)
	}
	if ids := marked("at the start"); len(ids) != 1 || ids[0] != file.ID {
		t.Fatalf("at the start: marked %v, want [%d] (the key labelled by hand is %d)", ids, file.ID, byHand.ID)
	}
	revoke := func(id int64, bearer string) (int, string) {
		t.Helper()
		return callAPI(t, ts, "DELETE", "/api/v1/users/owner/tokens/"+strconv.FormatInt(id, 10), bearer, "")
	}
	for _, by := range []struct{ what, bearer string }{{"by itself", "secret"}, {"by another owner", olga}} {
		if code, text := revoke(file.ID, by.bearer); code != 409 || !strings.Contains(text, "settings file") {
			t.Fatalf("the settings file's key revoked %s: %d %s, want 409", by.what, code, text)
		}
	}
	if code, text := callAPI(t, ts, "GET", "/api/v1/me", "secret", ""); code != 200 {
		t.Fatalf("the settings file's key after the refusals: %d %s", code, text)
	}
	if code, text := revoke(byHand.ID, olga); code != 200 {
		t.Fatalf("the key labelled by hand: %d %s", code, text)
	}

	// The file's token changes: the old key is revoked, as at a start,
	// and the new one is the key marked and refused.
	if err := s.SetToken(ctx, "fresh"); err != nil {
		t.Fatal(err)
	}
	fresh, ok, err := s.opts.Store.TokenByHash(ctx, store.HashToken("fresh"))
	if err != nil || !ok {
		t.Fatalf("the settings file's new key: %v %v", ok, err)
	}
	if ids := marked("after the change"); len(ids) != 1 || ids[0] != fresh.ID {
		t.Fatalf("after the change: marked %v, want [%d]", ids, fresh.ID)
	}
	if code, text := revoke(fresh.ID, olga); code != 409 {
		t.Fatalf("the settings file's new key: %d %s, want 409", code, text)
	}
	if code, text := revoke(file.ID, olga); code != 404 {
		t.Fatalf("the file's old key, revoked by the change: %d %s, want 404", code, text)
	}

	// Without a token in the file no key is marked.
	if err := s.SetToken(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if ids := marked("without a token in the file"); len(ids) != 0 {
		t.Fatalf("without a token in the file: marked %v", ids)
	}
}

// The user that holds the settings file's key (MASSIMO_API_TOKEN) is not
// removed, 409 to another owner, its key left live: the removal would
// revoke the key with the user's tokens, and the next start could not add
// it back, its hash being taken (AP MobileApp v1.4, D89-A). Another owner
// is removed as before, and so is the holder once the file has no token.
func TestSettingsFileKeyHolder(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, s := r.ts, r.s
	ctx := t.Context()
	olga := userToken(t, s.opts.Store, "olga", store.RoleOwner)
	userToken(t, s.opts.Store, "otto", store.RoleOwner)
	if code, text := callAPI(t, ts, "DELETE", "/api/v1/users/"+store.BootstrapUser, olga, ""); code != 409 || !strings.Contains(text, "settings file") {
		t.Fatalf("the key's holder removed by another owner: %d %s, want 409", code, text)
	}
	if code, text := callAPI(t, ts, "GET", "/api/v1/me", "secret", ""); code != 200 {
		t.Fatalf("the settings file's key after the refusal: %d %s", code, text)
	}
	if _, err := s.opts.Store.Bootstrap(ctx, "secret"); err != nil {
		t.Fatalf("the next start with the file's token: %v", err)
	}
	if code, text := callAPI(t, ts, "DELETE", "/api/v1/users/otto", olga, ""); code != 200 {
		t.Fatalf("another owner: %d %s", code, text)
	}

	// Without a token in the file the holder is removed as any user is.
	if err := s.SetToken(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if code, text := callAPI(t, ts, "DELETE", "/api/v1/users/"+store.BootstrapUser, olga, ""); code != 200 {
		t.Fatalf("the former holder, the file without a token: %d %s", code, text)
	}
}

// The first owner is created on an open server from the local network.
func TestFirstOwner(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "")))
	ts, s := r.ts, r.s
	users := "/api/v1/users"
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"name":"root","role":"viewer","password":"r-pass-001"}`, 400},
		{`{"name":"root"}`, 400},
		{`{"name":"root","password":"r-pass-001","display_name":"Root"}`, 201},
	} {
		if code, text := callAPI(t, ts, "POST", users, "", c.body); code != c.want {
			t.Fatalf("%s: %d %s, want %d", c.body, code, text, c.want)
		}
	}
	if u, _, _ := s.opts.Store.User(t.Context(), "root"); u.Role != store.RoleOwner || !u.HasPassword {
		t.Fatalf("first owner %+v", u)
	}
	if s.AuthMode() != "users" {
		t.Fatalf("mode %s", s.AuthMode())
	}
	if code, _ := callAPI(t, ts, "POST", users, "", `{"name":"x","password":"r-pass-001"}`); code != 401 {
		t.Fatalf("a second user without a token: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"root","password":"r-pass-001"}`); code != 200 {
		t.Fatalf("the owner's login: %d", code)
	}
	for addr, want := range map[string]bool{"127.0.0.1": true, "::1": true, "10.1.2.3": true, "192.168.1.20": true, "172.16.0.1": true, "169.254.1.1": true, "fd00::1": true, "fe80::1": true,
		"8.8.8.8": false, "2001:db8::1": false, "172.32.0.1": false, "nope": false} {
		if localPeer(addr) != want {
			t.Errorf("localPeer(%s) = %v", addr, !want)
		}
	}
}

// The roles and capabilities routes.
func TestRolesRoutes(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, s := r.ts, r.s
	st := s.opts.Store
	owner := "secret"
	roles := "/api/v1/roles"
	code, text := callAPI(t, ts, "GET", roles, owner, "")
	var view struct {
		Roles        []roleView       `json:"roles"`
		Capabilities []capabilityView `json:"capabilities"`
	}
	json.Unmarshal([]byte(text), &view)
	if code != 200 || len(view.Roles) != 6 || view.Roles[0].Key != store.RoleOwner || len(view.Roles[0].Capabilities) != len(capabilities) || len(view.Capabilities) != len(capabilities) || view.Capabilities[0].Disabled {
		t.Fatalf("roles: %d %s", code, text)
	}
	code, text = callAPI(t, ts, "POST", roles, owner, `{"key":"Night","name":"Night shift","capabilities":["read","setpoint"]}`)
	var night roleView
	json.Unmarshal([]byte(text), &night)
	if code != 201 || night.Key != "night" || night.Name != "Night shift" || strings.Join(night.Capabilities, ",") != "read,setpoint" || night.Builtin {
		t.Fatalf("create: %d %s", code, text)
	}
	for _, c := range []struct {
		body string
		want int
	}{{`{"key":"night"}`, 409}, {`{"key":"1x"}`, 400}, {`{"key":"x","capabilities":["fly"]}`, 400}, {`{"key":"x","nope":1}`, 400}} {
		if code, _ := callAPI(t, ts, "POST", roles, owner, c.body); code != c.want {
			t.Errorf("create %s: %d, want %d", c.body, code, c.want)
		}
	}
	code, text = callAPI(t, ts, "PUT", roles+"/night", owner, `{"capabilities":["read","lock"],"name":"Nights"}`)
	json.Unmarshal([]byte(text), &night)
	if code != 200 || night.Name != "Nights" || strings.Join(night.Capabilities, ",") != "lock,read" {
		t.Fatalf("change: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", roles+"/owner", owner, `{"name":"x"}`); code != 403 {
		t.Fatalf("change owner: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", roles+"/nope", owner, `{"name":"x"}`); code != 404 {
		t.Fatalf("change unknown: %d", code)
	}
	n := userToken(t, st, "n", "night")
	lock := "/api/v1/cubes/KEQ0000100/devices/0a010d/lock"
	if code, _ := callAPI(t, ts, "PUT", lock, n, `{"locked":true}`); code == 403 {
		t.Fatal("the created role's lock refused")
	}
	if code, _ := callAPI(t, ts, "PUT", roles+"/night/disabled", owner, `{"disabled":true}`); code != 200 {
		t.Fatalf("disable: %d", code)
	}
	if code, text := callAPI(t, ts, "GET", "/api/v1/cubes", n, ""); code != 403 || !strings.Contains(text, "disabled") {
		t.Fatalf("user of a disabled role: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", roles+"/night/disabled", owner, `{"disabled":false}`); code != 200 {
		t.Fatalf("enable: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", roles+"/owner/disabled", owner, `{"disabled":true}`); code != 403 {
		t.Fatalf("disable owner: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", roles+"/night/disabled", owner, `{}`); code != 400 {
		t.Fatalf("disable without a value: %d", code)
	}
	mgr := userToken(t, st, "mgr", store.RoleManager)
	if code, _ := callAPI(t, ts, "PUT", roles+"/night/disabled", mgr, `{"disabled":true}`); code != 403 {
		t.Fatalf("a manager disables a role: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", roles+"/night", owner, ""); code != 409 {
		t.Fatalf("remove a role in use: %d", code)
	}
	st.DeleteUser(t.Context(), "n", 0)
	if code, _ := callAPI(t, ts, "DELETE", roles+"/night", owner, ""); code != 200 {
		t.Fatalf("remove a role: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", roles+"/viewer", owner, ""); code != 403 {
		t.Fatalf("remove a default role: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", roles+"/nope", owner, ""); code != 404 {
		t.Fatalf("remove unknown: %d", code)
	}

	// Capabilities: disabled for every role but owner.
	op := userToken(t, st, "op", store.RoleOperator)
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/capabilities/lock", owner, `{"disabled":true}`); code != 200 {
		t.Fatalf("disable a capability: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", lock, op, `{"locked":true}`); code != 403 {
		t.Fatalf("operator with lock disabled: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", lock, owner, `{"locked":true}`); code == 403 {
		t.Fatal("owner with lock disabled")
	}
	code, text = callAPI(t, ts, "GET", "/api/v1/capabilities", owner, "")
	var caps []capabilityView
	json.Unmarshal([]byte(text), &caps)
	disabled := 0
	for _, c := range caps {
		if c.Disabled {
			disabled++
			if c.Key != "lock" {
				t.Errorf("%s disabled", c.Key)
			}
		}
	}
	if code != 200 || len(caps) != len(capabilities) || disabled != 1 {
		t.Fatalf("capabilities: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/capabilities/fly", owner, `{"disabled":true}`); code != 404 {
		t.Fatalf("unknown capability: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/capabilities/lock", owner, `{}`); code != 400 {
		t.Fatalf("no value: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/capabilities/lock", mgr, `{"disabled":false}`); code != 403 {
		t.Fatalf("a manager enables a capability: %d", code)
	}
}

// A password check or hash the store refuses for the load, or that
// waited until the client went away, answers 503 with Retry-After;
// a wrong password is not that.
func TestPasswordBusy(t *testing.T) {
	t.Parallel()
	for _, err := range []error{store.ErrBusy, fmt.Errorf("store: password check: %w", context.Canceled)} {
		var he *httpError
		if !errors.As(userFail(err), &he) || he.status != 503 || he.retryAfter != busyRetry {
			t.Errorf("%v: %+v", err, he)
		}
	}
	if passwordBusy(store.ErrBadPassword) != nil || passwordBusy(store.ErrUserExists) != nil {
		t.Error("an answer that is not the load is busy")
	}
}

// A refusal of the users and roles routes says what is wrong, without
// the store's "store: " prefix, which the web UI showed as it came:
// `test manager: store: bad user name: "test manager"`.
func TestUsersRefusalsWithoutStorePrefix(t *testing.T) {
	t.Parallel()
	ts := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret"))).ts
	owner := "secret"
	for _, c := range []struct {
		method, path, body string
		code               int
		want               string
	}{
		{"POST", "/api/v1/users", `{"name":"test manager"}`, 400, `bad user name: "test manager"`},
		{"POST", "/api/v1/users", `{"name":"anna","password":"a-pass-001"}`, 201, ""},
		{"POST", "/api/v1/users", `{"name":"Anna"}`, 409, "user exists: anna"},
		{"PUT", "/api/v1/users/nobody", `{"display_name":"x"}`, 404, `no user "nobody"`},
		{"DELETE", "/api/v1/roles/owner", "", 403, "the role cannot be changed"},
	} {
		code, text := callAPI(t, ts, c.method, c.path, owner, c.body)
		if code != c.code {
			t.Fatalf("%s %s: %d %s, want %d", c.method, c.path, code, text, c.code)
		}
		if c.want == "" {
			continue
		}
		var e struct {
			Error string `json:"error"`
		}
		json.Unmarshal([]byte(text), &e)
		if strings.Contains(e.Error, "store:") || !strings.HasPrefix(e.Error, c.want) {
			t.Errorf("%s %s: %q, want it to start with %q", c.method, c.path, e.Error, c.want)
		}
	}
}

// The rank rule: a user changes only users whose role has no capability
// its own lacks, and gives only such roles; nobody but an owner changes
// an owner. passwords, the manager's by default, sets the password of
// such a user and nothing else.
func TestRankRule(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "secret")
	r := startServer(t, withOptions(opts))
	ts, s := r.ts, r.s
	st := s.opts.Store
	ctx := t.Context()
	if _, err := st.AddRole(ctx, store.Role{Key: "keeper", Capabilities: []string{"read", "preferences", "users"}}, 0); err != nil {
		t.Fatal(err)
	}
	keeper := userToken(t, st, "keeper1", "keeper")
	manager := userToken(t, st, "manager1", store.RoleManager)
	userToken(t, st, "operator1", store.RoleOperator)
	userToken(t, st, "viewer1", store.RoleViewer)
	users := "/api/v1/users/"
	cases := []struct {
		what, key, path, body string
		code                  int
		err                   string
	}{
		{"users, a viewer's password", keeper, "viewer1", `{"password":"v-pass-1"}`, 200, ""},
		{"users, a viewer's name", keeper, "viewer1", `{"display_name":"V"}`, 200, ""},
		{"users, an operator's password", keeper, "operator1", `{"password":"o-pass-1"}`, 403, "the role operator has capabilities yours lacks"},
		{"users, a manager's role", keeper, "manager1", `{"role":"viewer"}`, 403, "the role manager has capabilities yours lacks"},
		{"users, giving operator", keeper, "viewer1", `{"role":"operator"}`, 403, "the role operator has capabilities yours lacks"},
		{"users, giving owner", keeper, "viewer1", `{"role":"owner"}`, 403, "only an owner gives the role owner"},
		{"users, the owner's password", keeper, "owner", `{"password":"x-pass-1"}`, 403, "only an owner changes an owner"},
		{"passwords, an operator's password", manager, "operator1", `{"password":"o-pass-2"}`, 200, ""},
		{"passwords, an operator's name", manager, "operator1", `{"display_name":"O"}`, 403, `capability "users" required`},
		{"passwords, an operator's role", manager, "operator1", `{"role":"viewer"}`, 403, `capability "users" required`},
		{"passwords, a keeper's password", manager, "keeper1", `{"password":"k-pass-1"}`, 403, "the role keeper has capabilities yours lacks"},
		{"owner, a manager's password", "secret", "manager1", `{"password":"m-pass-1"}`, 200, ""},
	}
	for _, c := range cases {
		code, text := callAPI(t, ts, "PUT", users+c.path, c.key, c.body)
		if code != c.code || c.err != "" && errorOf(text) != c.err {
			t.Errorf("%s: %d %s, want %d %q", c.what, code, text, c.code, c.err)
		}
	}
	if code, text := callAPI(t, ts, "POST", "/api/v1/users", keeper, `{"name":"x1","role":"owner"}`); code != 403 || errorOf(text) != "only an owner gives the role owner" {
		t.Errorf("users, creating an owner: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "DELETE", users+"manager1", keeper, ""); code != 403 || errorOf(text) != "the role manager has capabilities yours lacks" {
		t.Errorf("users, removing a manager: %d %s", code, text)
	}
}

// A user with keys only sets its first password as itself, without a
// current one it has not; the next change asks for the current one, as
// for every user.
func TestFirstOwnPassword(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "secret")
	r := startServer(t, withOptions(opts))
	ts, s := r.ts, r.s
	key := userToken(t, s.opts.Store, "keysonly", store.RoleViewer)
	path := "/api/v1/users/keysonly"
	code, text := callAPI(t, ts, "PUT", path, key, `{"password":"first-pass-1"}`)
	var v userView
	if err := json.Unmarshal([]byte(text), &v); err != nil || code != 200 || !v.HasPassword {
		t.Fatalf("the first password: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "PUT", path, key, `{"password":"second-pass-1"}`); code != 403 || errorOf(text) != "current_password is wrong" {
		t.Fatalf("the second without the current one: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "PUT", path, key, `{"password":"second-pass-1","current_password":"first-pass-1"}`); code != 200 {
		t.Fatalf("the second with the current one: %d %s", code, text)
	}
}

// A removed user keeps its history: its row stays, marked and renamed
// <name>-del-<time>, so that the log and the audit still name it, while
// its keys and its push subscriptions go with it, and a new user under
// the name starts with none of it (docs/store.md, "users").
func TestRemovedUserKeepsItsHistory(t *testing.T) {
	t.Parallel()
	rg := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, s := rg.ts, rg.s
	st := s.opts.Store
	ctx := t.Context()
	anna := userToken(t, st, "anna", store.RoleViewer)
	if _, err := st.UpdateUser(ctx, "anna", func(u *store.User) error { u.DisplayName = "Anna"; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	row, _, err := st.User(ctx, "anna")
	if err != nil {
		t.Fatal(err)
	}
	if code, text := callAPI(t, ts, "POST", "/api/v1/me/push", anna, subscription(t, "https://push.example.net/send/anna", "phone")); code != 201 {
		t.Fatalf("subscribe: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "PUT", "/api/v1/me/preferences/ui", anna, `{"poll": 20}`); code != 200 {
		t.Fatalf("a preference: %d %s", code, text)
	}
	s.logServer(event{Kind: "change", Source: "config", Time: time.Now(), userID: row.ID})

	if code, text := callAPI(t, ts, "DELETE", "/api/v1/users/anna", "secret", ""); code != 200 {
		t.Fatalf("remove: %d %s", code, text)
	}

	// Gone as a user: its key, its name, its subscriptions.
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", anna, ""); code != 401 {
		t.Fatalf("the removed user's key: %d", code)
	}
	if _, ok, _ := st.User(ctx, "anna"); ok {
		t.Fatal("found by name")
	}
	if subs, err := st.PushSubscriptions(ctx, row.ID); err != nil || len(subs) != 0 {
		t.Fatalf("its push subscriptions: %v %v", subs, err)
	}
	// Its row kept with its name (migration 0024), by which the log and
	// the audit name it while no other user has the name.
	u, ok, err := st.UserByID(ctx, row.ID)
	if err != nil || !ok || u.Name != "anna" || u.DisplayName != "Anna" {
		t.Fatalf("its row: %+v %v %v", u, ok, err)
	}
	gone := u.Name
	var evs []event
	if code := getJSON(t, ts, "secret", "/api/v1/cubes/KEQ0000100/log?user="+gone, &evs); code != 200 || len(evs) != 1 || evs[0].By == nil || evs[0].By.Name != gone || evs[0].By.DisplayName != "Anna" {
		t.Fatalf("the log of the removed user: %d %+v", code, evs)
	}
	seen := map[string]bool{}
	for _, r := range auditRows(t, ts, "secret", "?user="+gone+"&limit=50") {
		if r.User != gone {
			t.Errorf("an audit row of %q under user=%s", r.User, gone)
		}
		seen[r.Action] = true
	}
	// A new anna is another person: a row of its own, none of the
	// removed one's history.
	if code, text := callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name": "anna", "role": "viewer"}`); code != 201 {
		t.Fatalf("a new anna: %d %s", code, text)
	}
	if again, _, _ := st.User(ctx, "anna"); again.ID == row.ID {
		t.Fatal("the new anna took the removed one's row")
	}
	if rows := auditRows(t, ts, "secret", "?user=anna&limit=50"); len(rows) != 0 {
		t.Fatalf("the new anna has the removed one's audit: %+v", rows)
	}
	if !seen["push.subscribe"] || !seen["preference.set"] {
		t.Fatalf("the audit of the removed user: %v", seen)
	}
	if rows := auditRows(t, ts, "secret", "?limit=50"); !strings.Contains(fmt.Sprint(rows), "user.remove") {
		t.Fatalf("no user.remove in the audit")
	}
}

// A plain password of POST and PUT /api/v1/users… is refused where it
// contains the user's names, the server's name, the first label of the
// request's Host, massimo or maxcube, whatever the case: 400, with words
// that name none of them, before anything is written (AP PasswordRules,
// D50-A). The host's name stands for the server's where the option is
// empty, and the first owner of an open server is held to it too.
func TestUsersPasswordNames(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")), editOptions(func(o *Options) { o.ServerName = "Attic Box" }))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	users := "/api/v1/users"
	call := func(method, path, host, body string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set("Content-Type", "application/json")
		if host != "" {
			req.Host = host
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	const said = "password: contains the user's or the server's name"
	refused := func(what string, code int, text string) {
		t.Helper()
		if code != 400 || errorOf(text) != said {
			t.Errorf("%s: %d %s, want 400 %q", what, code, text, said)
		}
	}
	for _, c := range []struct{ what, host, body string }{
		{"POST, the login name", "", `{"name":"anna","password":"ANNA-river-2026"}`},
		{"POST, a word of the display name", "", `{"name":"anna","display_name":"Anna Berg","password":"river-BERG-2026"}`},
		{"POST, the server's name", "", `{"name":"anna","password":"my-attic-2026"}`},
		{"POST, the request's host", "kitchen.example.org:8443", `{"name":"anna","password":"ferry-KITCHEN-7"}`},
		{"POST, maxcube", "", `{"name":"anna","password":"MaxCube-2026"}`},
	} {
		code, text := call("POST", users, c.host, c.body)
		refused(c.what, code, text)
	}
	if _, ok, _ := st.User(ctx, "anna"); ok {
		t.Fatal("a refused POST added the user")
	}
	// The host's name refuses nothing where the request names another.
	if code, text := call("POST", users, "", `{"name":"anna","display_name":"Anna Berg","password":"ferry-kitchen-7"}`); code != 201 {
		t.Fatalf("POST, kitchen with another host: %d %s", code, text)
	}
	code, text := call("PUT", users+"/anna", "", `{"display_name":"Lake View","password":"lake-river-2026"}`)
	refused("PUT, the new display name", code, text)
	code, text = call("PUT", users+"/anna", "", `{"display_name":"Lake View","password":"berg-river-2026"}`)
	refused("PUT, the display name it had", code, text)
	code, text = call("PUT", users+"/anna", "", `{"password":"my-Massimo-1"}`)
	refused("PUT, massimo", code, text)
	if u, _, _ := st.User(ctx, "anna"); u.DisplayName != "Anna Berg" {
		t.Errorf("a refused PUT changed the display name: %q", u.DisplayName)
	}
	if err := st.CheckPassword(ctx, "anna", "ferry-kitchen-7"); err != nil {
		t.Errorf("a refused PUT changed the password: %v", err)
	}
	if code, text := call("PUT", users+"/anna", "", `{"password":"anne-river-2026"}`); code != 200 {
		t.Fatalf("PUT, the name with one letter changed: %d %s", code, text)
	}

	// Without the option, the host's name is the server's: the tests' own
	// (TestMain).
	r2 := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	code, text = callAPI(t, r2.ts, "POST", users, "secret", `{"name":"anna","password":"x-KITCHEN-2026-x"}`)
	refused("POST, the host's name without the option", code, text)
	if code, text := callAPI(t, r2.ts, "POST", users, "secret", `{"name":"anna","password":"x-kitchan-2026-x"}`); code != 201 {
		t.Fatalf("POST, the host's name with one letter changed: %d %s", code, text)
	}

	// The first owner, on an open server, is held to the rule as well.
	r3 := startServer(t, withOptions(storeOptions(t, t.TempDir(), "")))
	code, text = callAPI(t, r3.ts, "POST", users, "", `{"name":"root","password":"ROOT-river-1"}`)
	refused("POST, the first owner's name", code, text)
	if code, text := callAPI(t, r3.ts, "POST", users, "", `{"name":"root","password":"rook-river-1"}`); code != 201 {
		t.Fatalf("POST, the first owner with one letter changed: %d %s", code, text)
	}
}

// A plain password that a request sets has 8 to 64 characters, counted
// in code points (AP PasswordRules, D7-A, D11-A): POST and PUT answer
// 400 with the rule's words before anything of the request is written,
// and one's own change keeps its proof for the next try. A record, the
// empty password that removes one, and a sign-in with a password set
// before are not held to it.
func TestUsersPasswordRule(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	users := "/api/v1/users"
	short, long := "seven-7", strings.Repeat("x", 65)
	refused := func(what string, code int, text, want string) {
		t.Helper()
		if code != 400 || errorOf(text) != want {
			t.Errorf("%s: %d %s, want 400 %q", what, code, text, want)
		}
	}
	code, text := callAPI(t, ts, "POST", users, "secret", `{"name":"anna","password":"`+short+`"}`)
	refused("POST, 7", code, text, "password: at least 8 characters")
	code, text = callAPI(t, ts, "POST", users, "secret", `{"name":"anna","password":"`+long+`"}`)
	refused("POST, 65", code, text, "password: at most 64 characters")
	// Seven characters beyond the basic plane are 14 UTF-16 units and
	// 28 bytes: still seven.
	code, text = callAPI(t, ts, "POST", users, "secret", `{"name":"anna","password":"`+strings.Repeat("😀", 7)+`"}`)
	refused("POST, 7 beyond the basic plane", code, text, "password: at least 8 characters")
	if _, ok, _ := st.User(ctx, "anna"); ok {
		t.Fatal("a refused POST added the user")
	}
	if code, text := callAPI(t, ts, "POST", users, "secret", `{"name":"anna","display_name":"Anna","password":"`+strings.Repeat("😀", 64)+`"}`); code != 201 {
		t.Fatalf("POST, 64 beyond the basic plane: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "PUT", users+"/anna", "secret", `{"password":"äöüäöüäö"}`); code != 200 {
		t.Fatalf("PUT, äöü counted as three: %d %s", code, text)
	}

	// A PUT that fails the rule writes nothing of the request.
	code, text = callAPI(t, ts, "PUT", users+"/anna", "secret", `{"display_name":"Changed","password":"`+short+`"}`)
	refused("PUT, 7", code, text, "password: at least 8 characters")
	code, text = callAPI(t, ts, "PUT", users+"/anna", "secret", `{"display_name":"Changed","password":"`+long+`"}`)
	refused("PUT, 65", code, text, "password: at most 64 characters")
	if u, _, _ := st.User(ctx, "anna"); u.DisplayName != "Anna" {
		t.Errorf("a refused PUT changed the display name: %q", u.DisplayName)
	}
	if err := st.CheckPassword(ctx, "anna", "äöüäöüäö"); err != nil {
		t.Errorf("a refused PUT changed the password: %v", err)
	}

	// One's own change: refused before the proof is used, which then
	// still sets a record of any length.
	if code, text := callAPI(t, ts, "PUT", users+"/anna", "secret", `{"record":"`+opaqueRecord(t, ts, "secret", "anna", "pw")+`","ksf":"`+store.OpaqueKSF+`"}`); code != 200 {
		t.Fatalf("PUT, a record of a short password: %d %s", code, text)
	}
	_, code, body := opaqueSignIn(t, ts, "anna", "pw", "")
	var sess struct{ Token string }
	if json.Unmarshal([]byte(body), &sess); code != 200 || sess.Token == "" {
		t.Fatalf("a sign-in with the record of a short password: %d %s", code, body)
	}
	_, code, body = opaqueSignIn(t, ts, "anna", "pw", purposeProof)
	var proof struct{ Proof string }
	if json.Unmarshal([]byte(body), &proof); code != 200 || proof.Proof == "" {
		t.Fatalf("a proof: %d %s", code, body)
	}
	code, text = callAPI(t, ts, "PUT", users+"/anna", sess.Token, `{"password":"`+short+`","proof":"`+proof.Proof+`"}`)
	refused("one's own, 7", code, text, "password: at least 8 characters")
	if code, text := callAPI(t, ts, "PUT", users+"/anna", sess.Token, `{"record":"`+opaqueRecord(t, ts, sess.Token, "anna", "p2")+`","ksf":"`+store.OpaqueKSF+`","proof":"`+proof.Proof+`"}`); code != 200 {
		t.Fatalf("one's own record with the proof the refusal left: %d %s", code, text)
	}

	// The empty password removes one, whatever the rule says.
	code, text = callAPI(t, ts, "PUT", users+"/anna", "secret", `{"password":""}`)
	if u := decodeUserView(text); code != 200 || u.HasPassword {
		t.Fatalf("PUT, the empty password: %d %s", code, text)
	}
	// A record of a short password, POST, as the page sends one.
	if code, text := callAPI(t, ts, "POST", users, "secret", `{"name":"carl","record":"`+opaqueRecord(t, ts, "secret", "carl", "c")+`","ksf":"`+store.OpaqueKSF+`"}`); code != 201 {
		t.Fatalf("POST, a record of a short password: %d %s", code, text)
	}
	// A password set before the rule signs in as it is.
	if _, err := st.AddUser(ctx, store.User{Name: "dora", Role: store.RoleViewer}, "d1", 0); err != nil {
		t.Fatal(err)
	}
	if code, text := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"dora","password":"d1"}`); code != 200 {
		t.Fatalf("a sign-in with a password of two characters set before: %d %s", code, text)
	}
}

// The first owner's plain password is held to the rule too, on an open
// server, before the owner is written.
func TestFirstOwnerPasswordRule(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "")))
	code, text := callAPI(t, r.ts, "POST", "/api/v1/users", "", `{"name":"root","password":"seven-7"}`)
	if code != 400 || errorOf(text) != "password: at least 8 characters" {
		t.Fatalf("the first owner, 7: %d %s", code, text)
	}
	if n, _ := r.s.opts.Store.CountUsers(t.Context()); n != 0 || r.s.AuthMode() != "open" {
		t.Fatalf("a refused first owner: %d users, mode %s", n, r.s.AuthMode())
	}
}
