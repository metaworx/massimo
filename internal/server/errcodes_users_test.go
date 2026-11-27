// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/update"
)

// errorAnswer is an error answer as a client reads it: its English error,
// its code and its params.
type errorAnswer struct {
	Error  string          `json:"error"`
	Code   string          `json:"code"`
	Params json.RawMessage `json:"params"`
}

// answerRefusal reads an error answer, its params compacted.
func answerRefusal(body string) errorAnswer {
	var r errorAnswer
	json.Unmarshal([]byte(body), &r)
	if len(r.Params) > 0 {
		var b bytes.Buffer
		if json.Compact(&b, r.Params) == nil {
			r.Params = b.Bytes()
		}
	}
	return r
}

// answerCode reads the code of a JSON error body.
func answerCode(body string) string { return answerRefusal(body).Code }

// isRefusal reports whether an answer is the refusal with status, code,
// params as compact JSON ("" for none) and the English error msg.
func isRefusal(gotStatus int, body string, status int, code, params, msg string) bool {
	r := answerRefusal(body)
	return gotStatus == status && r.Code == code && string(r.Params) == params && r.Error == msg
}

// refusalBody is the answer an error of a handler gets, as the route
// wrapper writes it: its status and its body.
func refusalBody(t *testing.T, err error) (int, string) {
	t.Helper()
	var he *httpError
	if !errors.As(err, &he) {
		t.Fatalf("%v is no httpError", err)
	}
	rec := httptest.NewRecorder()
	writeJSON(rec, he.status, he.body())
	return he.status, rec.Body.String()
}

// keyWithPassword adds a user with a role and a password and returns an
// API key of it, and the user.
func keyWithPassword(t *testing.T, st *store.Store, name, role, password string) (string, store.User) {
	t.Helper()
	u, err := st.AddUser(t.Context(), store.User{Name: name, Role: role}, password, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := st.AddToken(t.Context(), u.ID, store.TokenAPI, "test", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	return secret, u
}

// The refusals of users, roles, the sign-ins, tokens, providers,
// identities, integrations, settings, preferences, push and commands
// (AP ErrorCodes, Block 4) carry their codes and params beside their
// English error, as each site sends them; a wrong password and a name
// nobody has are refused alike, the same body.
func TestUserRefusalCodes(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, s := r.ts, r.s
	st := s.opts.Store
	ctx := t.Context()
	anna, annaUser := keyWithPassword(t, st, "anna", store.RoleOperator, "a-pass-0042")
	dora, _ := keyWithPassword(t, st, "dora", store.RoleViewer, "d-pass-0042")
	eve, _ := keyWithPassword(t, st, "eve", store.RoleGuest, "e-pass-0042")
	if _, err := st.AddRole(ctx, store.Role{Key: "night", Capabilities: []string{"read"}}, 0); err != nil {
		t.Fatal(err)
	}
	nina := userToken(t, st, "nina", "night")
	if _, err := st.AddRole(ctx, store.Role{Key: "keeper", Capabilities: []string{"read", "preferences", "users"}}, 0); err != nil {
		t.Fatal(err)
	}
	keeper := userToken(t, st, "keeper1", "keeper")
	// passwords and every capability of a guest, but neither users nor guests
	if _, err := st.AddRole(ctx, store.Role{Key: "setter", Capabilities: []string{"read", "setpoint", "configure", "passwords"}}, 0); err != nil {
		t.Fatal(err)
	}
	setter := userToken(t, st, "setter1", "setter")
	userToken(t, st, "viewer1", store.RoleViewer)
	lena := userToken(t, st, "lena", store.RoleViewer) // no password: an identity is its one way in
	lenaUser, _, _ := st.User(ctx, "lena")
	bob, _, _ := st.User(ctx, "viewer1")
	p, err := st.AddProvider(ctx, store.AuthProvider{Key: "cloud", Name: "Nextcloud", Issuer: "https://idp.example.net", ClientID: "massimo", Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	lenaID, err := st.AddSubjectIdentity(ctx, lenaUser.ID, "cloud", "sub-lena", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddSubjectIdentity(ctx, bob.ID, "cloud", "sub-bob", 0); err != nil {
		t.Fatal(err)
	}
	integ, integKey := registerHA(t, st, []string{"read", "preferences"})
	file, ok, err := st.TokenByHash(ctx, store.HashToken("secret"))
	if err != nil || !ok {
		t.Fatalf("the settings file's key: %v %v", ok, err)
	}
	owner, _, _ := st.User(ctx, store.BootstrapUser)
	// a pending link of anna's to the account bob has, and one proven for
	// a user that is gone
	taken := s.oidc.hold(&oidcPendingLink{userID: annaUser.ID, providerID: p.ID, provider: p.Key, subject: "sub-bob", ip: "127.0.0.1", expires: time.Now().Add(time.Minute)}, time.Now())
	gone := s.oidc.prove(&oidcProven{userID: 1 << 40, provider: p.Key, ip: "127.0.0.1", expires: time.Now().Add(time.Minute)}, time.Now())
	if taken == "" || gone == "" {
		t.Fatal("no pending link or proven sign-in kept")
	}
	// a command of the owner's still queued under an Idempotency-Key
	if _, err := st.AddCommand(ctx, store.Command{UserID: owner.ID, Key: "running", Action: "settings.update", Request: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	// dora disabled, eve past its end, the role night disabled
	for _, c := range [][2]string{{"/api/v1/users/dora", `{"disabled":true}`}, {"/api/v1/users/eve", `{"expires_at":"2020-01-01T00:00:00Z"}`},
		{"/api/v1/roles/night/disabled", `{"disabled":true}`}} {
		if code, text := callAPI(t, ts, "PUT", c[0], "secret", c[1]); code != 200 {
			t.Fatalf("PUT %s %s: %d %s", c[0], c[1], code, text)
		}
	}
	if err := st.SetSetting(ctx, store.IntegrationRequestsKey, false); err != nil {
		t.Fatal(err)
	}

	sys := integ.UUID + ":system"
	own := integ.UUID + ":"
	dur := func(d time.Duration) string { return fmtDuration(d) }
	cases := []struct {
		what, method, path, token, body string
		status                          int
		code, params, msg               string
	}{
		{"a capability lacked", "GET", "/api/v1/users", anna, "", 403,
			"capability_required", `{"capability":"users"}`, `capability "users" required`},
		{"a wrong token", "GET", "/api/v1/me", "not-a-token", "", 401,
			"token_wrong", "", "missing or wrong token"},
		{"a disabled user's key", "GET", "/api/v1/me", dora, "", 403,
			"user_account_disabled", `{"user":"dora"}`, "user dora is disabled"},
		{"a disabled user's password", "POST", "/api/v1/login", "", `{"user":"dora","password":"d-pass-0042"}`, 403,
			"user_account_disabled", `{"user":"dora"}`, "user dora is disabled"},
		{"an expired user's key", "GET", "/api/v1/me", eve, "", 403,
			"user_account_expired", `{"user":"eve"}`, "user eve has expired"},
		{"an expired user's password", "POST", "/api/v1/login", "", `{"user":"eve","password":"e-pass-0042"}`, 403,
			"user_account_expired", `{"user":"eve"}`, "user eve has expired"},
		{"a disabled role's key", "GET", "/api/v1/me", nina, "", 403,
			"role_disabled", `{"role":"night"}`, "role night is disabled"},
		{"a wrong password", "POST", "/api/v1/login", "", `{"user":"anna","password":"wrong"}`, 401,
			"login_wrong", "", "wrong user or password"},
		{"a name nobody has", "POST", "/api/v1/login", "", `{"user":"nobody","password":"wrong"}`, 401,
			"login_wrong", "", "wrong user or password"},
		{"a finish without a start", "POST", "/api/v1/login/finish", "", `{"login":"nope","ke3":"AAAA"}`, 401,
			"login_not_started", "", "no sign-in started under this login, or it took too long; start again"},
		{"an integration's key for a session", "POST", "/api/v1/login/key", integKey, "", 403,
			"login_integration_key", "", "a registered integration's key makes no session"},
		{"a provider's code unknown", "POST", "/api/v1/login/oidc/finish", "", `{"code":"nope"}`, 401,
			"oidc_code_unknown", "", "no sign-in under this code, or it took too long; sign in again"},
		{"a provider's code for a user gone", "POST", "/api/v1/login/oidc/finish", "", `{"code":"` + gone + `"}`, 401,
			"oidc_user_gone", "", "the user is gone; sign in again"},
		{"a sign-in at no provider", "POST", "/api/v1/login/oidc/nope", "", `{"redirect":"http://192.168.1.2:8099/"}`, 404,
			"provider_unknown", `{"provider":"nope"}`, `no provider "nope"`},
		{"a link at no provider", "POST", "/api/v1/users/anna/identities/link/nope", anna, `{"redirect":"http://192.168.1.2:8099/"}`, 404,
			"provider_unknown", `{"provider":"nope"}`, `no provider "nope"`},
		{"a change of no provider", "PUT", "/api/v1/providers/nope", "secret", `{"name":"x"}`, 404,
			"provider_unknown", `{"provider":"nope"}`, `no provider "nope"`},
		{"a removal of no provider", "DELETE", "/api/v1/providers/nope", "secret", "", 404,
			"provider_unknown", `{"provider":"nope"}`, `no provider "nope"`},
		{"a provider without a key", "POST", "/api/v1/providers", "secret", `{"name":"x"}`, 400,
			"provider_key_needed", "", "key is needed"},
		{"no user of the name", "GET", "/api/v1/users/nobody", "secret", "", 404,
			"user_unknown", `{"user":"nobody"}`, `no user "nobody"`},
		{"a link to no user", "PUT", "/api/v1/integrations/" + integ.UUID + "/users/system", "secret", `{"user":"nobody"}`, 404,
			"user_unknown", `{"user":"nobody"}`, "no user nobody"},
		{"a bad name", "POST", "/api/v1/users", "secret", `{"name":"x y"}`, 400,
			"user_name_bad", "", `bad user name: "x y"`},
		{"a long display name", "POST", "/api/v1/users", "secret", `{"name":"dn","display_name":"` + strings.Repeat("ä", 33) + `"}`, 400,
			"user_display_name_long", `{"max":64}`, "display_name: at most 64 bytes"},
		{"a key for an integration's user", "POST", "/api/v1/users/" + sys + "/tokens", "secret", `{"label":"planted"}`, 403,
			"user_integration_no_way_in", `{"user":"` + sys + `"}`, "the user " + sys + " is an integration's: it acts through its integration alone, with no key, password or sign-in of its own"},
		{"the removal of an integration's own user", "DELETE", "/api/v1/users/" + own, "secret", "", 403,
			"user_integration_own", `{"integration":"` + integ.UUID + `","user":"` + own + `"}`,
			"the user " + own + " is the integration's own: remove the integration itself, DELETE /api/v1/integrations/" + integ.UUID + ", which keeps it to be attached again"},
		{"an owner's password by a keeper", "PUT", "/api/v1/users/owner", keeper, `{"password":"x-pass-0042"}`, 403,
			"user_owner_only", "", "only an owner changes an owner"},
		{"the removal of the settings file's user", "DELETE", "/api/v1/users/" + store.BootstrapUser, "secret", "", 409,
			"user_holds_settings_key", `{"user":"` + store.BootstrapUser + `"}`,
			"user " + store.BootstrapUser + " holds the settings file's MASSIMO_API_TOKEN: change or remove it in the settings file first"},
		{"a role beyond the caller's", "PUT", "/api/v1/users/viewer1", keeper, `{"role":"operator"}`, 403,
			"role_beyond_yours", `{"role":"operator"}`, "the role operator has capabilities yours lacks"},
		{"the role owner by a keeper", "PUT", "/api/v1/users/viewer1", keeper, `{"role":"owner"}`, 403,
			"role_owner_only", "", "only an owner gives the role owner"},
		{"the role integration for a regular user", "POST", "/api/v1/users", "secret", `{"name":"ha","role":"integration"}`, 403,
			"role_integration_only", "", "the role integration is an integration's and its users' alone"},
		{"a guest by a holder of passwords", "POST", "/api/v1/users", setter, `{"name":"g9","role":"guest"}`, 403,
			"capability_required_to_create", `{"capability":"users","role":"guest"}`, `capability "users" required to create a user with the role guest`},
		{"a wrong proof", "PUT", "/api/v1/users/anna", anna, `{"password":"b-pass-0042","proof":"bogus"}`, 403,
			"password_proof_wrong", "", "proof is wrong, used or too old: sign in again to prove the current password"},
		{"a wrong current password", "PUT", "/api/v1/users/anna", anna, `{"password":"b-pass-0042","current_password":"wrong"}`, 403,
			"password_current_wrong", "", "current_password is wrong"},
		{"a key without a label", "POST", "/api/v1/users/anna/tokens", "secret", `{"label":" "}`, 400,
			"token_label_length", fmt.Sprintf(`{"max":%d}`, maxLabel), fmt.Sprintf("label: 1 to %d bytes", maxLabel)},
		{"no token of the id", "DELETE", "/api/v1/users/anna/tokens/987654", "secret", "", 404,
			"token_unknown", `{"token":987654,"user":"anna"}`, "user anna has no token 987654"},
		{"the settings file's key revoked by itself", "DELETE", "/api/v1/me/token", "secret", "", 409,
			"token_settings_key", "", "the token is the settings file's MASSIMO_API_TOKEN: change or remove it in the settings file"},
		{"the settings file's key revoked by id", "DELETE", fmt.Sprintf("/api/v1/users/%s/tokens/%d", store.BootstrapUser, file.ID), "secret", "", 409,
			"token_settings_key", "", "the token is the settings file's MASSIMO_API_TOKEN: change or remove it in the settings file"},
		{"no capability in a role's set", "POST", "/api/v1/roles", "secret", `{"key":"r1","capabilities":["nope"]}`, 400,
			"capability_unknown", `{"capability":"nope"}`, `unknown capability "nope"`},
		{"no capability as the path's", "PUT", "/api/v1/capabilities/nope", "secret", `{"disabled":true}`, 404,
			"capability_unknown", `{"capability":"nope"}`, `no capability "nope"`},
		{"a long role name", "POST", "/api/v1/roles", "secret", `{"key":"r2","name":"` + strings.Repeat("x", 65) + `"}`, 400,
			"role_name_long", `{"max":64}`, "name: at most 64 bytes"},
		{"the last way in", "DELETE", fmt.Sprintf("/api/v1/users/lena/identities/%d", lenaID.ID), lena, "", 409,
			"identity_last_way_in", "", "this is the last way you sign in: set a password or link another account first"},
		{"another's link started", "POST", "/api/v1/users/anna/identities/link/cloud", "secret", `{"redirect":"http://192.168.1.2:8099/"}`, 403,
			"identity_link_own_only", "", "a user links only its own accounts"},
		{"another's link read", "GET", "/api/v1/users/anna/identities/link/pending/abc", "secret", "", 403,
			"identity_confirm_own_only", "", "a user confirms only its own links"},
		{"no link under the code", "GET", "/api/v1/users/anna/identities/link/pending/abc", anna, "", 404,
			"identity_link_unknown", "", "no link waits under this code, or it took too long; link again"},
		{"an account another user has", "POST", "/api/v1/users/anna/identities/link/pending/" + taken, anna, "", 409,
			"identity_taken", "", "the account is another user's"},
		{"registration requests off", "POST", "/api/v1/integrations/requests", "", `{"kind":"homeassistant","name":"Home"}`, 403,
			"integration_requests_off", `{"setting":"` + store.IntegrationRequestsKey + `"}`, "this server takes no registration requests (setting " + store.IntegrationRequestsKey + ")"},
		{"a maximum beyond the role integration", "PUT", "/api/v1/integrations/" + integ.UUID, "secret", `{"max_caps":["users"]}`, 400,
			"integration_capability_beyond", `{"capability":"users"}`, `max_caps: "users" is not an integration's to have`},
		{"an empty integration name", "PUT", "/api/v1/integrations/" + integ.UUID, "secret", `{"name":" "}`, 400,
			"integration_name_length", `{"max":64}`, "name is 1 to 64 bytes"},
		{"no integration of the uuid", "GET", "/api/v1/integrations/nope", "secret", "", 404,
			"integration_unknown", `{"integration":"nope"}`, "no integration nope"},
		{"keep_open with users", "PUT", "/api/v1/settings", "secret", `{"auth.keep_open":true}`, 409,
			"setting_keep_open_users", `{"setting":"auth.keep_open"}`, "auth.keep_open counts only on a server without users: this one has users"},
		{"announce not among its choices", "PUT", "/api/v1/settings", "secret", `{"announce":"loud"}`, 400,
			"setting_choice_bad", `{"choices":"` + strings.Join(announceValues, ", ") + `","setting":"announce"}`,
			"announce: want one of " + strings.Join(announceValues, ", ") + ", or null for auto"},
		{"a channel not among its choices", "PUT", "/api/v1/settings", "secret", `{"update.channel":"loud"}`, 400,
			"setting_choice_bad", `{"choices":"` + strings.Join(update.Flavours, ", ") + `","setting":"update.channel"}`,
			"update.channel: want one of " + strings.Join(update.Flavours, ", ") + ", or null for the build's flavour"},
		{"update.auto not among its choices", "PUT", "/api/v1/settings", "secret", `{"update.auto":"loud"}`, 400,
			"setting_choice_bad", `{"choices":"off, night","setting":"update.auto"}`, "update.auto: want off or night, or null for off"},
		{"a duration out of its range", "PUT", "/api/v1/settings", "secret", `{"poll_interval":"1s"}`, 400,
			"setting_duration_range", `{"max":"` + dur(time.Hour) + `","min":"` + dur(10*time.Second) + `","setting":"poll_interval","value":"1s"}`,
			"poll_interval 1s: between " + dur(10*time.Second) + " and " + dur(time.Hour)},
		{"a duration out of its range, 0 for off", "PUT", "/api/v1/settings", "secret", `{"refresh_interval":"1m"}`, 400,
			"setting_duration_range_off", `{"max":"` + dur(7*24*time.Hour) + `","min":"` + dur(10*time.Minute) + `","setting":"refresh_interval","value":"1m"}`,
			"refresh_interval 1m: between " + dur(10*time.Minute) + " and " + dur(7*24*time.Hour) + ", or 0 for off"},
		{"no preference of the key", "GET", "/api/v1/me/preferences/nope", "secret", "", 404,
			"preference_unknown", `{"preference":"nope"}`, `no preference "nope"`},
		{"no subscription of the id", "DELETE", "/api/v1/me/push/999", "secret", "", 404,
			"push_subscription_unknown", `{"subscription":999}`, "no subscription 999 of yours"},
		{"no command of the id", "GET", "/api/v1/commands/nope", "secret", "", 404,
			"command_unknown", `{"command":"nope"}`, `no command "nope"`},
	}
	for _, c := range cases {
		if code, text := callAPI(t, ts, c.method, c.path, c.token, c.body); !isRefusal(code, text, c.status, c.code, c.params, c.msg) {
			t.Errorf("%s, %s %s: %d %s\nwant %d %s %s %q", c.what, c.method, c.path, code, text, c.status, c.code, c.params, c.msg)
		}
	}
	// A wrong password and a name nobody has: the same answer.
	_, wrongPassword := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"wrong"}`)
	_, wrongName := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"nobody","password":"wrong"}`)
	if wrongPassword != wrongName {
		t.Errorf("a wrong password and a wrong name told apart:\n%s\n%s", wrongPassword, wrongName)
	}

	// The password's way off; a sign-in while another from the address
	// runs, with its Retry-After.
	if err := st.SetSetting(ctx, store.PasswordLoginKey, false); err != nil {
		t.Fatal(err)
	}
	if code, text := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"a-pass-0042"}`); !isRefusal(code, text, 403, "login_password_off", "",
		"the password sign-in is off on this server; sign in from the web UI, or with an API key") {
		t.Errorf("the password sign-in off: %d %s", code, text)
	}
	st.SetSetting(ctx, store.PasswordLoginKey, true)
	addr := store.LockoutAddr("127.0.0.1")
	if ok, _ := s.signIns.claim(addr); !ok {
		t.Fatal("the address is signing in already")
	}
	resp, text := callWith(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"a-pass-0042"}`, nil)
	s.signIns.release(addr)
	if !isRefusal(resp.StatusCode, text, 429, "login_running", "", "a sign-in from this address is already running; try again when it is done") || resp.Header.Get("Retry-After") != "1" {
		t.Errorf("a second sign-in meanwhile: %d %s, Retry-After %q", resp.StatusCode, text, resp.Header.Get("Retry-After"))
	}

	// Idempotency-Key: a request under the key still running, and another
	// request under a key used.
	running := map[string]string{"Idempotency-Key": "running"}
	if resp, text := callWith(t, ts, "PUT", "/api/v1/settings", "secret", `{"stale_after":"36h"}`, running); !isRefusal(resp.StatusCode, text, 409, "idempotency_running", "",
		"a request with this Idempotency-Key is still running") {
		t.Errorf("under a key still running: %d %s", resp.StatusCode, text)
	}
	used := map[string]string{"Idempotency-Key": "used"}
	if resp, text := callWith(t, ts, "PUT", "/api/v1/settings", "secret", `{"stale_after":"36h"}`, used); resp.StatusCode != 200 {
		t.Fatalf("the first under a key: %d %s", resp.StatusCode, text)
	}
	if resp, text := callWith(t, ts, "PUT", "/api/v1/settings", "secret", `{"stale_after":"30h"}`, used); !isRefusal(resp.StatusCode, text, 422, "idempotency_reused", "",
		"Idempotency-Key reused for another request") {
		t.Errorf("another request under a key used: %d %s", resp.StatusCode, text)
	}

	// A write another site's page sent.
	if resp, text := callWith(t, ts, "POST", "/api/v1/logout", "", "{}", map[string]string{"Sec-Fetch-Site": "cross-site", "Content-Type": "text/plain"}); !isRefusal(resp.StatusCode, text, 403,
		"cross_origin_refused", "", "cross-origin request refused") {
		t.Errorf("a write from another site: %d %s", resp.StatusCode, text)
	}

	// Registration requests: too many from an address, with Retry-After.
	st.SetSetting(ctx, store.IntegrationRequestsKey, true)
	limited := false
	for n := 0; n < requestBurst+1 && !limited; n++ {
		resp, text := callWith(t, ts, "POST", "/api/v1/integrations/requests", "", `{"kind":"homeassistant","name":"Home"}`, nil)
		if resp.StatusCode != 429 {
			continue
		}
		limited = true
		if !isRefusal(resp.StatusCode, text, 429, "integration_requests_too_many", "", "too many registration requests from this address") || resp.Header.Get("Retry-After") != "60" {
			t.Errorf("too many registration requests: %d %s, Retry-After %q", resp.StatusCode, text, resp.Header.Get("Retry-After"))
		}
	}
	if !limited {
		t.Error("no registration request refused for its address")
	}
}

// A server without a store and an open one: a sign-in, the users and the
// preferences refused with their codes; the open server answers its local
// network alone, and the first owner needs a password.
func TestUserRefusalCodesWithoutUsers(t *testing.T) {
	t.Parallel()
	legacy := startServer(t, withOptions(Options{Token: "secret"})).ts
	if code, text := callAPI(t, legacy, "POST", "/api/v1/login", "", `{"user":"anna","password":"a-pass-0042"}`); !isRefusal(code, text, 409, "login_no_store", "",
		"this server has no store: use the API token") {
		t.Errorf("a sign-in without a store: %d %s", code, text)
	}
	if code, text := callAPI(t, legacy, "GET", "/api/v1/users", "secret", ""); !isRefusal(code, text, 503, "user_no_store", "",
		"this server has no store, so no users: it takes the API token") {
		t.Errorf("the users without a store: %d %s", code, text)
	}

	s := startServer(t, withOptions(storeOptions(t, t.TempDir(), ""))).s
	lan, far := "192.168.1.20:5000", "203.0.113.9:5000"
	noUsers := "no users yet: create the first owner with POST /api/v1/users, or massimo user add"
	for _, c := range []struct {
		what, method, path, addr, body string
		status                         int
		code, msg                      string
	}{
		{"a sign-in", "POST", "/api/v1/login", lan, `{"user":"anna","password":"a-pass-0042"}`, 409, "login_no_users", noUsers},
		{"a sign-in by a key", "POST", "/api/v1/login/key", lan, "", 409, "login_no_users", noUsers},
		{"the first owner without a password", "POST", "/api/v1/users", lan, `{"name":"anna"}`, 400, "first_owner_password_needed", "the first owner needs a password"},
		{"the preferences", "GET", "/api/v1/me/preferences", lan, "", 409, "preference_no_user", "no user signed in: the preferences stay in the browser"},
		{"beyond the local network", "GET", "/api/v1/cubes", far, "", 403, "server_open_local_only", openRefused},
	} {
		if rec := sendFrom(s, c.method, c.path, c.addr, "", c.body); !isRefusal(rec.Code, rec.Body.String(), c.status, c.code, "", c.msg) {
			t.Errorf("%s: %d %s, want %d %s", c.what, rec.Code, rec.Body.String(), c.status, c.code)
		}
	}
}

// The lockouts: of an address after failed token checks, at the token
// check and at a sign-in, in a provider's words at its start; of a name
// after failed sign-ins from several addresses. Each with Retry-After.
func TestLockoutCodes(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.AuthFailures = 2
	s := startServer(t, withOptions(opts)).s
	st := s.opts.Store
	if _, err := st.AddUser(t.Context(), store.User{Name: "anna", Role: store.RoleViewer}, "a-pass-0042", 0); err != nil {
		t.Fatal(err)
	}
	locked := func(what, method, path, addr, token, body string, code, msg string) {
		t.Helper()
		rec := sendFrom(s, method, path, addr, token, body)
		if !isRefusal(rec.Code, rec.Body.String(), 429, code, "", msg) || rec.Header().Get("Retry-After") == "" {
			t.Errorf("%s: %d %s, Retry-After %q", what, rec.Code, rec.Body.String(), rec.Header().Get("Retry-After"))
		}
	}
	// the name: failures from two addresses, each counted once
	for _, a := range []string{"192.168.1.10:5000", "192.168.1.11:5000"} {
		if rec := sendFrom(s, "POST", "/api/v1/login", a, "", `{"user":"anna","password":"wrong"}`); rec.Code != 401 {
			t.Fatalf("a wrong password from %s: %d %s", a, rec.Code, rec.Body.String())
		}
	}
	locked("a sign-in of the name locked", "POST", "/api/v1/login", "192.168.1.12:5000", "", `{"user":"anna","password":"a-pass-0042"}`,
		"user_locked_out", "too many failed logins for this user; try again later")
	// the address: two wrong tokens
	far := "192.168.1.20:5000"
	for range 2 {
		if rec := sendFrom(s, "GET", "/api/v1/me", far, "wrong", ""); rec.Code != 401 {
			t.Fatalf("a wrong token: %d %s", rec.Code, rec.Body.String())
		}
	}
	addr := "too many failed token checks from this address; try again later"
	locked("a token check from the address locked", "GET", "/api/v1/me", far, "secret", "", "address_locked_out", addr)
	locked("a sign-in from the address locked", "POST", "/api/v1/login", far, "", `{"user":"bob","password":"b-pass-0042"}`, "address_locked_out", addr)
	locked("a provider's sign-in from the address locked", "POST", "/api/v1/login/oidc/cloud", far, "", `{"redirect":"http://192.168.1.2:8099/"}`,
		"address_locked_out", "too many failed sign-ins from this address; try again later")
}

// The refusals no request through the wrapper meets as it stands: the
// first owner from beyond the local network, which the open server's
// door refuses first; the first owner made by another request
// meanwhile, which TestFirstOwnerOnce races; the store's password checks
// all taken, with Retry-After.
func TestUserRefusalCodesOfHandlers(t *testing.T) {
	t.Parallel()
	s := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret"))).s
	open := func(addr, path, body string) *http.Request {
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.RemoteAddr = addr
		return withPrincipal(r, &principal{mode: modeOpen})
	}
	far := "203.0.113.9:5000"
	notLocal := "the first owner is created from the local network, or on the box with massimo user add"
	for _, c := range []struct {
		what string
		err  error
	}{
		{"the first owner from beyond", s.postUser(httptest.NewRecorder(), open(far, "/api/v1/users", `{"name":"anna","password":"a-pass-0042"}`))},
		{"its registration from beyond", s.mayRegister(open(far, "/api/v1/users/anna/register", ""), s.opts.Store, "anna")},
	} {
		if status, body := refusalBody(t, c.err); !isRefusal(status, body, 403, "first_owner_not_local", "", notLocal) {
			t.Errorf("%s: %d %s", c.what, status, body)
		}
	}
	err := s.postUser(httptest.NewRecorder(), open("192.168.1.20:5000", "/api/v1/users", `{"name":"anna","password":"a-pass-0042"}`))
	if status, body := refusalBody(t, err); !isRefusal(status, body, 409, "first_owner_exists", "", "the first owner exists already: sign in") {
		t.Errorf("the first owner made meanwhile: %d %s", status, body)
	}
	err = passwordBusy(store.ErrBusy)
	if status, body := refusalBody(t, err); !isRefusal(status, body, 503, "password_busy", "", "too many password checks at once; try again in a few seconds") ||
		err.(*httpError).retryAfter != busyRetry {
		t.Errorf("the password checks all taken: %d %s, Retry-After %d", status, body, err.(*httpError).retryAfter)
	}
}
