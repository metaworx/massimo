// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

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
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	ctx := t.Context()
	owner := "secret"
	users := "/api/v1/users"
	code, text := callAPI(t, ts, "POST", users, owner, `{"name":"Anna","display_name":"Anna","role":"operator","password":"pw"}`)
	anna := decodeUserView(text)
	if code != 201 || anna.Name != "anna" || anna.Role != store.RoleOperator || !anna.HasPassword || anna.InvitedBy != store.BootstrapUser {
		t.Fatalf("create: %d %s", code, text)
	}
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"name":"anna","password":"x"}`, 409},
		{`{"name":"x y"}`, 400},
		{`{"name":"bob","role":"nope"}`, 400},
		{`{"name":"bob","expires_at":"soon"}`, 400},
		{`{"name":"bob","unknown":1}`, 400},
		{`{"name":"guest1","role":"guest","expires_at":"2026-12-24T18:00:00Z","password":"g"}`, 201},
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
	code, text = callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"pw"}`)
	var sess struct {
		Token string `json:"token"`
	}
	json.Unmarshal([]byte(text), &sess)
	if code != 200 || sess.Token == "" {
		t.Fatalf("login: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", users+"/anna", owner, `{"password":"new"}`); code != 200 {
		t.Fatalf("password by the owner: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", sess.Token, ""); code != 401 {
		t.Fatalf("session after a password reset: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"new"}`); code != 200 {
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
	if code, text := callAPI(t, ts, "PUT", users+"/anna", annaKey, `{"password":"x","current_password":"wrong"}`); code != 403 || !strings.Contains(text, "current_password") {
		t.Fatalf("own password with the wrong current one: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", users+"/anna", annaKey, `{"password":"x","current_password":"new"}`); code != 200 {
		t.Fatalf("own password: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"x"}`); code != 200 {
		t.Fatalf("login with the own new password: %d", code)
	}

	// A guests holder (anna is a manager) sees and manages guests and
	// itself, nobody else.
	code, text = callAPI(t, ts, "GET", users, annaKey, "")
	json.Unmarshal([]byte(text), &list)
	if code != 200 || len(list) != 2 || list[0].Name != "anna" || list[1].Name != "guest1" {
		t.Fatalf("a manager's list: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "GET", users+"/owner", annaKey, ""); code != 403 {
		t.Fatalf("a manager reads the owner: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", users, annaKey, `{"name":"g2","role":"guest","password":"p"}`); code != 201 {
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
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"x"}`); code != 401 {
		t.Fatalf("login of a removed user: %d", code)
	}
}

// The first owner is created on an open server from the local network.
func TestFirstOwner(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), ""))
	users := "/api/v1/users"
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"name":"root","role":"viewer","password":"p"}`, 400},
		{`{"name":"root"}`, 400},
		{`{"name":"root","password":"p","display_name":"Root"}`, 201},
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
	if code, _ := callAPI(t, ts, "POST", users, "", `{"name":"x","password":"p"}`); code != 401 {
		t.Fatalf("a second user without a token: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"root","password":"p"}`); code != 200 {
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
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
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
	lock := "/api/v1/cubes/KEQ0537249/devices/1514b6/lock"
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
