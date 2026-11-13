// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// fakeIdP is an OpenID Connect provider for the tests: its discovery
// document, and a token endpoint that answers a code the test granted
// with an id_token of the claims given, once the client authenticated
// and the PKCE verifier matched.
type fakeIdP struct {
	t      *testing.T
	srv    *httptest.Server
	secret string
	// methods is token_endpoint_auth_methods_supported; the client's
	// method is checked against it.
	methods []string
	// userinfo, where set, is the userinfo endpoint's answer for the
	// access token, its status userinfoStatus where not 0.
	userinfo       map[string]any
	userinfoStatus int
	mu             sync.Mutex
	grants         map[string]fakeGrant
}

type fakeGrant struct {
	claims    map[string]any
	challenge string
	redirect  string
}

const fakeClientID = "massimo-test"

func newFakeIdP(t *testing.T) *fakeIdP {
	f := &fakeIdP{t: t, secret: "s3cr:et", grants: map[string]fakeGrant{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]any{"issuer": f.srv.URL, "authorization_endpoint": f.srv.URL + "/authorize?prompt=login",
			"token_endpoint": f.srv.URL + "/token", "token_endpoint_auth_methods_supported": f.methods}
		if f.userinfo != nil {
			doc["userinfo_endpoint"] = f.srv.URL + "/userinfo"
		}
		writeJSON(w, 200, doc)
	})
	mux.HandleFunc("GET /userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at" {
			writeJSON(w, 401, map[string]string{"error": "invalid_token"})
			return
		}
		writeJSON(w, cmp.Or(f.userinfoStatus, 200), f.userinfo)
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		id, secret, basic := r.BasicAuth()
		if basic {
			id, _ = url.QueryUnescape(id)
			secret, _ = url.QueryUnescape(secret)
		} else {
			id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
		}
		post := len(f.methods) == 1 && f.methods[0] == "client_secret_post"
		if id != fakeClientID || secret != f.secret || basic == post {
			writeJSON(w, 401, map[string]string{"error": "invalid_client"})
			return
		}
		f.mu.Lock()
		g, ok := f.grants[r.PostForm.Get("code")]
		delete(f.grants, r.PostForm.Get("code"))
		f.mu.Unlock()
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if !ok || r.PostForm.Get("grant_type") != "authorization_code" || base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge || r.PostForm.Get("redirect_uri") != g.redirect {
			writeJSON(w, 400, map[string]string{"error": "invalid_grant"})
			return
		}
		payload, _ := json.Marshal(g.claims)
		writeJSON(w, 200, map[string]any{"access_token": "at", "token_type": "Bearer",
			"id_token": "eyJhbGciOiJSUzI1NiJ9." + base64.RawURLEncoding.EncodeToString(payload) + ".c2ln"})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// oidcRun is one sign-in through the fake: the start, the provider's
// grant of a code for the claims (the defaults of a good id_token,
// overridden by those given, a nil one removed), and the callback.
type oidcRun struct {
	status   int    // the callback's
	fragment string // the fragment of its Location: oidc=… or oidc-error=…
}

func (f *fakeIdP) signIn(t *testing.T, ts *httptest.Server, provider string, claims map[string]any) oidcRun {
	t.Helper()
	code, text := callAPI(t, ts, "POST", "/api/v1/login/oidc/"+provider, "", `{"redirect": "`+ts.URL+`/"}`)
	if code != 200 {
		t.Fatalf("start: %d %s", code, text)
	}
	return f.grant(t, ts, text, claims)
}

// grant has the fake grant a code for the claims to a start's answer,
// and calls the callback with it.
func (f *fakeIdP) grant(t *testing.T, ts *httptest.Server, startAnswer string, claims map[string]any) oidcRun {
	t.Helper()
	var started struct{ URL string }
	json.Unmarshal([]byte(startAnswer), &started)
	auth, err := url.Parse(started.URL)
	if err != nil || !strings.HasPrefix(started.URL, f.srv.URL+"/authorize?") {
		t.Fatalf("the authorization URL %q", started.URL)
	}
	q := auth.Query()
	if q.Get("prompt") != "login" || q.Get("response_type") != "code" || q.Get("client_id") != fakeClientID || q.Get("code_challenge_method") != "S256" ||
		q.Get("scope") != store.DefaultScopes || q.Get("redirect_uri") != ts.URL+"/api/v1/login/oidc/callback" {
		t.Fatalf("the authorization request %v", q)
	}
	all := map[string]any{"iss": f.srv.URL, "aud": fakeClientID, "exp": time.Now().Add(5 * time.Minute).Unix(), "nonce": q.Get("nonce"), "sub": "sub-1"}
	for k, v := range claims {
		if v == nil {
			delete(all, k)
		} else {
			all[k] = v
		}
	}
	grant := randomID()
	f.mu.Lock()
	f.grants[grant] = fakeGrant{claims: all, challenge: q.Get("code_challenge"), redirect: q.Get("redirect_uri")}
	f.mu.Unlock()
	return callback(t, ts, "code="+grant+"&state="+q.Get("state"))
}

// callback calls the callback with the query, without following its
// redirect.
func callback(t *testing.T, ts *httptest.Server, query string) oidcRun {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(ts.URL + "/api/v1/login/oidc/callback?" + query)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	run := oidcRun{status: resp.StatusCode}
	if loc := resp.Header.Get("Location"); loc != "" {
		before, frag, _ := strings.Cut(loc, "#")
		if !strings.HasPrefix(before, ts.URL+"/") || !strings.HasSuffix(before, "/") {
			t.Fatalf("back to %q, not the page", loc)
		}
		run.fragment = frag
	}
	return run
}

// finish trades a callback's code for the session.
func finish(t *testing.T, ts *httptest.Server, run oidcRun, header ...string) (int, map[string]any) {
	t.Helper()
	code, ok := strings.CutPrefix(run.fragment, "oidc=")
	if !ok {
		t.Fatalf("no code: %+v", run)
	}
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/login/oidc/finish", strings.NewReader(`{"code": "`+code+`"}`))
	req.Header.Set("Content-Type", "application/json")
	if len(header) == 2 {
		req.Header.Set(header[0], header[1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func signedInAs(t *testing.T, ts *httptest.Server, run oidcRun) string {
	t.Helper()
	code, out := finish(t, ts, run)
	if code != 200 || out["token"] == nil || out["provider"] != "cloud" {
		t.Fatalf("finish: %d %v", code, out)
	}
	var me struct{ User struct{ Name string } }
	if c := getJSON(t, ts, out["token"].(string), "/api/v1/me", &me); c != 200 {
		t.Fatalf("me: %d", c)
	}
	return me.User.Name
}

// The sign-in through a provider (AP Oidc v1.0, Block 2): a pre-link
// binds at the first sign-in with its e-mail verified and finds the user
// by its subject alone from then on; the callback's code is good once,
// from its address; a refusal names its reason and counts against the
// address.
func TestOIDCSignIn(t *testing.T) {
	f := newFakeIdP(t)
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	_, ts, s := startWith(t, opts)
	st := s.opts.Store
	ctx := t.Context()
	p, err := st.AddProvider(ctx, store.AuthProvider{Key: "cloud", Name: "Nextcloud", Issuer: f.srv.URL, ClientID: fakeClientID, ClientSecret: f.secret, Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	st.AddProvider(ctx, store.AuthProvider{Key: "off", Issuer: f.srv.URL, ClientID: "x"}, 0)
	var listed []map[string]string
	if c := getJSON(t, ts, "", "/api/v1/login/providers", &listed); c != 200 || len(listed) != 1 || listed[0]["key"] != "cloud" || listed[0]["name"] != "Nextcloud" {
		t.Fatalf("providers: %d %v", c, listed)
	}
	anna, _ := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "", 0)
	if _, err := st.AddIdentity(ctx, anna.ID, "cloud", "anna@example.com", 0); err != nil {
		t.Fatal(err)
	}

	// an unverified e-mail binds nothing, and says so where an e-mail
	// waits; none sent says that
	if run := f.signIn(t, ts, "cloud", map[string]any{"email": "anna@example.com"}); run.status != 303 || run.fragment != "oidc-error=unverified" {
		t.Fatalf("unverified: %+v", run)
	}
	if run := f.signIn(t, ts, "cloud", map[string]any{"email": "nobody@example.com"}); run.fragment != "oidc-error=refused" {
		t.Fatalf("unverified, none waiting: %+v", run)
	}
	if run := f.signIn(t, ts, "cloud", nil); run.fragment != "oidc-error=noemail" {
		t.Fatalf("unverified: %+v", run)
	}
	run := f.signIn(t, ts, "cloud", map[string]any{"email": "Anna@Example.com", "email_verified": true})
	if name := signedInAs(t, ts, run); name != "anna" {
		t.Fatalf("signed in as %q", name)
	}
	if code, _ := finish(t, ts, run); code != 401 {
		t.Fatalf("the code twice: %d", code)
	}
	ids, _ := st.Identities(ctx, anna.ID)
	if len(ids) != 1 || ids[0].Subject != "sub-1" || ids[0].LastUsedAt.IsZero() {
		t.Fatalf("the identity: %+v", ids)
	}
	// by the subject from then on, whatever the e-mail
	if name := signedInAs(t, ts, f.signIn(t, ts, "cloud", map[string]any{"email": "other@example.net", "email_verified": "true"})); name != "anna" {
		t.Fatalf("by subject: %q", name)
	}
	// a code from another address
	if code, _ := finish(t, ts, f.signIn(t, ts, "cloud", nil), "X-Forwarded-For", "192.0.2.7"); code != 401 {
		t.Fatalf("from another address: %d", code)
	}

	// what the id_token must say
	for name, claims := range map[string]map[string]any{
		"issuer":   {"iss": "https://elsewhere.example.com"},
		"audience": {"aud": []string{"someone-else"}},
		"azp":      {"aud": []string{fakeClientID, "x"}, "azp": "x"},
		"nonce":    {"nonce": "replayed"},
		"expired":  {"exp": time.Now().Add(-5 * time.Minute).Unix()},
		"no sub":   {"sub": nil},
	} {
		if run := f.signIn(t, ts, "cloud", claims); run.fragment != "oidc-error=failed" {
			t.Errorf("%s: %+v", name, run)
		}
	}
	if run := f.signIn(t, ts, "cloud", map[string]any{"aud": []string{"x", fakeClientID}}); !strings.HasPrefix(run.fragment, "oidc=") {
		t.Errorf("the client among the audiences: %+v", run)
	}

	// the state: unknown, and good once; the provider's own refusal
	if run := callback(t, ts, "code=x&state=nope"); run.status != 400 {
		t.Fatalf("an unknown state: %+v", run)
	}
	code, text := callAPI(t, ts, "POST", "/api/v1/login/oidc/cloud", "", `{"redirect": "`+ts.URL+`/app"}`)
	var started struct{ URL string }
	json.Unmarshal([]byte(text), &started)
	u, _ := url.Parse(started.URL)
	if code != 200 || u.Query().Get("redirect_uri") != ts.URL+"/app/api/v1/login/oidc/callback" {
		t.Fatalf("a page under a path: %d %s", code, text)
	}
	state := u.Query().Get("state")
	if run := callback(t, ts, "error=access_denied&state="+state); run.fragment != "oidc-error=denied" {
		t.Fatalf("denied: %+v", run)
	}
	if run := callback(t, ts, "code=x&state="+state); run.status != 400 {
		t.Fatalf("a state twice: %+v", run)
	}

	// a disabled user
	st.UpdateUser(ctx, "anna", func(u *store.User) error { u.Disabled = true; return nil }, 0)
	if run := f.signIn(t, ts, "cloud", nil); run.fragment != "oidc-error=disabled" {
		t.Fatalf("disabled: %+v", run)
	}

	// the starts refused
	for body, want := range map[string]int{`{"redirect": "javascript:alert(1)"}`: 400, `{"redirect": "` + ts.URL + `/?x=1"}`: 400, `{}`: 400} {
		if c, text := callAPI(t, ts, "POST", "/api/v1/login/oidc/cloud", "", body); c != want {
			t.Errorf("start %s: %d %s", body, c, text)
		}
	}
	for _, key := range []string{"nope", "off"} {
		if c, _ := callAPI(t, ts, "POST", "/api/v1/login/oidc/"+key, "", `{"redirect": "`+ts.URL+`/"}`); c != 404 {
			t.Errorf("start at %s: %d", key, c)
		}
	}
	_ = p
}

// A provider that creates users: only for a verified e-mail of its
// domains, with its role, which may hold no administrative capability;
// a name from preferred_username, else the e-mail. A provider that names
// client_secret_post alone gets the secret in the form.
func TestOIDCAutoCreate(t *testing.T) {
	f := newFakeIdP(t)
	f.methods = []string{"client_secret_post"}
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	ctx := t.Context()
	if _, err := st.AddProvider(ctx, store.AuthProvider{Key: "cloud", Issuer: f.srv.URL, ClientID: fakeClientID, ClientSecret: f.secret, Enabled: true}, 0); err != nil {
		t.Fatal(err)
	}
	verified := func(sub, email string, more ...any) map[string]any {
		c := map[string]any{"sub": sub, "email": email, "email_verified": true}
		for i := 0; i+1 < len(more); i += 2 {
			c[more[i].(string)] = more[i+1]
		}
		return c
	}
	if run := f.signIn(t, ts, "cloud", verified("s1", "carl@example.org")); run.fragment != "oidc-error=refused" {
		t.Fatalf("auto-create off: %+v", run)
	}
	st.UpdateProvider(ctx, "cloud", func(p *store.AuthProvider) error {
		p.AutoCreate, p.AutoRole, p.AutoDomains = true, store.RoleGuest, []string{"example.org"}
		return nil
	}, 0)
	if run := f.signIn(t, ts, "cloud", verified("s1", "eve@example.net")); run.fragment != "oidc-error=refused" {
		t.Fatalf("a foreign domain: %+v", run)
	}
	if run := f.signIn(t, ts, "cloud", map[string]any{"sub": "s1", "email": "carl@example.org"}); run.fragment != "oidc-error=unverified" {
		t.Fatalf("an unverified e-mail: %+v", run)
	}
	if name := signedInAs(t, ts, f.signIn(t, ts, "cloud", verified("s1", "carl@example.org", "preferred_username", "Carl Example", "name", "Carl E."))); name != "carl-example" {
		t.Fatalf("created as %q", name)
	}
	carl, _, _ := st.User(ctx, "carl-example")
	if carl.Role != store.RoleGuest || carl.DisplayName != "Carl E." || carl.HasPassword {
		t.Fatalf("carl: %+v", carl)
	}
	if name := signedInAs(t, ts, f.signIn(t, ts, "cloud", verified("s2", "carl@example.org"))); name != "carl" {
		t.Fatalf("from the e-mail: %q", name)
	}
	if name := signedInAs(t, ts, f.signIn(t, ts, "cloud", verified("s1", "carl@example.org"))); name != "carl-example" {
		t.Fatalf("again by its subject: %q", name)
	}
	st.UpdateProvider(ctx, "cloud", func(p *store.AuthProvider) error { p.AutoRole = store.RoleManager; return nil }, 0)
	if run := f.signIn(t, ts, "cloud", verified("s3", "dora@example.org")); run.fragment != "oidc-error=refused" {
		t.Fatalf("an administrative role: %+v", run)
	}
	if _, ok, _ := st.User(ctx, "dora"); ok {
		t.Fatal("dora created with an administrative role")
	}
}

// newUserName makes a login name of what a provider says.
func TestNewUserName(t *testing.T) {
	for in, want := range map[string]string{
		"Carl Example": "carl-example", "  anna.b ": "anna.b", "__x__": "x", "Ünïcode": "n-code", "---": "",
		strings.Repeat("a", 70): strings.Repeat("a", 60),
	} {
		if got := newUserName(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

// The userinfo endpoint (AP Oidc v1.1, Block 6, D25-A): it fills in the
// e-mail and its verification the id_token lacks, as Nextcloud's OIDC
// app answers; an answer for another subject fails the sign-in, one that
// fails leaves the id_token's claims alone. A user id assigned signs in
// at once.
func TestOIDCUserinfo(t *testing.T) {
	f := newFakeIdP(t)
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	ctx := t.Context()
	if _, err := st.AddProvider(ctx, store.AuthProvider{Key: "cloud", Issuer: f.srv.URL, ClientID: fakeClientID, ClientSecret: f.secret, Enabled: true}, 0); err != nil {
		t.Fatal(err)
	}
	anna, _ := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "", 0)
	st.AddIdentity(ctx, anna.ID, "cloud", "anna@example.com", 0)

	f.userinfo = map[string]any{"sub": "other", "email": "anna@example.com", "email_verified": true}
	if run := f.signIn(t, ts, "cloud", nil); run.fragment != "oidc-error=failed" {
		t.Fatalf("another subject: %+v", run)
	}
	f.userinfo, f.userinfoStatus = map[string]any{"error": "x"}, 500
	if run := f.signIn(t, ts, "cloud", nil); run.fragment != "oidc-error=noemail" {
		t.Fatalf("userinfo down: %+v", run)
	}
	f.userinfoStatus = 0
	// the id_token's e-mail, its verification from the userinfo
	f.userinfo = map[string]any{"sub": "sub-1", "email": "ANNA@example.com", "email_verified": true, "preferred_username": "anna.b"}
	if name := signedInAs(t, ts, f.signIn(t, ts, "cloud", map[string]any{"email": "anna@example.com"})); name != "anna" {
		t.Fatalf("verified by the userinfo: %q", name)
	}
	// the e-mail from the userinfo alone
	bert, _ := st.AddUser(ctx, store.User{Name: "bert", Role: store.RoleOperator}, "", 0)
	st.AddIdentity(ctx, bert.ID, "cloud", "bert@example.com", 0)
	f.userinfo = map[string]any{"sub": "sub-2", "email": "bert@example.com", "email_verified": "true", "groups": []string{"g1"}}
	if name := signedInAs(t, ts, f.signIn(t, ts, "cloud", map[string]any{"sub": "sub-2"})); name != "bert" {
		t.Fatalf("e-mail from the userinfo: %q", name)
	}
	if ids, _ := st.Identities(ctx, bert.ID); len(ids) != 1 || strings.Join(ids[0].Groups, ",") != "g1" {
		t.Fatalf("the groups from the userinfo: %+v", ids)
	}

	// a user id assigned: no e-mail needed
	f.userinfo = map[string]any{"sub": "mdr"}
	carl, _ := st.AddUser(ctx, store.User{Name: "carl", Role: store.RoleOperator}, "", 0)
	if _, err := st.AddSubjectIdentity(ctx, carl.ID, "cloud", "mdr", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddSubjectIdentity(ctx, anna.ID, "cloud", "mdr", 0); !errors.Is(err, store.ErrIdentityExists) {
		t.Fatalf("a user id twice: %v", err)
	}
	if name := signedInAs(t, ts, f.signIn(t, ts, "cloud", map[string]any{"sub": "mdr"})); name != "carl" {
		t.Fatalf("by the user id: %q", name)
	}
}

// fakeDiscovery answers every discovery request with an issuer: the one
// asked for, without its trailing slash, or the one says gives for it;
// "" for an issuer not reached.
func fakeDiscovery(t *testing.T, says map[string]string) {
	t.Helper()
	old := oidcClient
	oidcClient = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		asked := strings.TrimSuffix(r.URL.Scheme+"://"+r.URL.Host+strings.TrimSuffix(r.URL.Path, "/.well-known/openid-configuration"), "/")
		issuer, ok := says[asked]
		if !ok {
			issuer = asked
		}
		if issuer == "" {
			return nil, context.DeadlineExceeded
		}
		rec := httptest.NewRecorder()
		writeJSON(rec, 200, map[string]any{"issuer": issuer, "authorization_endpoint": asked + "/authorize", "token_endpoint": asked + "/token"})
		return rec.Result(), nil
	})}
	t.Cleanup(func() { oidcClient = old })
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// A user links its own account (AP Oidc v1.1, Block 7, D21-A): only for
// itself; the link carries the account's details, replaces the user's
// other link at the provider, and is refused where another user has the
// account; the last way a user signs in is not the user's to remove; a
// manager changes a link no sign-in used.
func TestOIDCLink(t *testing.T) {
	f := newFakeIdP(t)
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	ctx := t.Context()
	if _, err := st.AddProvider(ctx, store.AuthProvider{Key: "cloud", Issuer: f.srv.URL, ClientID: fakeClientID, ClientSecret: f.secret, Enabled: true}, 0); err != nil {
		t.Fatal(err)
	}
	anna := userToken(t, st, "anna", store.RoleOperator)
	userToken(t, st, "bert", store.RoleOperator)
	bert, _, _ := st.User(ctx, "bert")
	st.AddSubjectIdentity(ctx, bert.ID, "cloud", "bert-sub", 0)
	link := func(token string, claims map[string]any) oidcRun {
		t.Helper()
		code, text := callAPI(t, ts, "POST", "/api/v1/users/anna/identities/link/cloud", token, `{"redirect": "`+ts.URL+`/"}`)
		if code != 200 {
			t.Fatalf("link start: %d %s", code, text)
		}
		return f.grant(t, ts, text, claims)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/users/anna/identities/link/cloud", "secret", `{"redirect": "`+ts.URL+`/"}`); code != 403 {
		t.Fatalf("the owner links for anna: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/users/anna/identities/link/nope", anna, `{"redirect": "`+ts.URL+`/"}`); code != 404 {
		t.Fatalf("an unknown provider: %d", code)
	}
	if run := link(anna, map[string]any{"sub": "a1", "email": "Anna@example.com", "name": "Anna B", "preferred_username": "anna.b", "groups": []string{"family", "admins"}}); run.fragment != "oidc-linked=cloud" {
		t.Fatalf("link: %+v", run)
	}
	var list []identityView
	if c := getJSON(t, ts, anna, "/api/v1/users/anna/identities", &list); c != 200 || len(list) != 1 {
		t.Fatalf("its own list: %d %+v", c, list)
	}
	if i := list[0]; i.Subject != "a1" || !i.Bound || i.Email != "anna@example.com" || i.Name != "Anna B" || i.Username != "anna.b" ||
		strings.Join(i.Groups, ",") != "family,admins" || i.LastUsedAt == nil {
		t.Fatalf("the link: %+v", i)
	}
	// linked again with another account at the provider: it replaces
	if run := link(anna, map[string]any{"sub": "a2", "groups": "solo"}); run.fragment != "oidc-linked=cloud" {
		t.Fatalf("link again: %+v", run)
	}
	if getJSON(t, ts, anna, "/api/v1/users/anna/identities", &list); len(list) != 1 || list[0].Subject != "a2" || strings.Join(list[0].Groups, ",") != "solo" {
		t.Fatalf("replaced: %+v", list)
	}
	if run := link(anna, map[string]any{"sub": "bert-sub"}); run.fragment != "oidc-link-error=taken" {
		t.Fatalf("bert's account: %+v", run)
	}
	// the details at a sign-in
	if name := signedInAs(t, ts, f.signIn(t, ts, "cloud", map[string]any{"sub": "a2", "name": "Anna C"})); name != "anna" {
		t.Fatalf("signed in as %q", name)
	}
	if getJSON(t, ts, anna, "/api/v1/users/anna/identities", &list); list[0].Name != "Anna C" {
		t.Fatalf("the details of the sign-in: %+v", list)
	}

	// the last way in: anna has no password
	id := itoa(list[0].ID)
	if code, _ := callAPI(t, ts, "DELETE", "/api/v1/users/anna/identities/"+id, anna, ""); code != 409 {
		t.Fatalf("its last way in: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", "/api/v1/users/bert/identities/1", anna, ""); code != 403 {
		t.Fatalf("bert's, by anna: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", "/api/v1/users/anna/identities/"+id, "secret", ""); code != 200 {
		t.Fatalf("the owner removes it: %d", code)
	}

	// a manager changes a link no sign-in used, not one used
	code, text := callAPI(t, ts, "POST", "/api/v1/users/anna/identities", "secret", `{"provider": "cloud", "email": "a@example.com"}`)
	var w identityView
	json.Unmarshal([]byte(text), &w)
	if code != 201 {
		t.Fatalf("assign: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/users/anna/identities/"+itoa(w.ID), anna, `{"email": "b@example.com"}`); code != 403 {
		t.Fatalf("anna changes her own: %d", code)
	}
	code, text = callAPI(t, ts, "PUT", "/api/v1/users/anna/identities/"+itoa(w.ID), "secret", `{"subject": "bert-sub"}`)
	if code != 409 {
		t.Fatalf("to bert's user id: %d %s", code, text)
	}
	code, text = callAPI(t, ts, "PUT", "/api/v1/users/anna/identities/"+itoa(w.ID), "secret", `{"subject": "a3"}`)
	if json.Unmarshal([]byte(text), &w); code != 200 || w.Subject != "a3" || !w.Bound || w.Email != "" {
		t.Fatalf("to a user id: %d %s", code, text)
	}
	code, text = callAPI(t, ts, "PUT", "/api/v1/users/anna/identities/"+itoa(w.ID), "secret", `{"email": "C@example.com"}`)
	if json.Unmarshal([]byte(text), &w); code != 200 || w.Bound || w.Email != "c@example.com" {
		t.Fatalf("back to an e-mail: %d %s", code, text)
	}
	if name := signedInAs(t, ts, f.signIn(t, ts, "cloud", map[string]any{"sub": "bert-sub"})); name != "bert" {
		t.Fatalf("bert: %q", name)
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/users/bert/identities/1", "secret", `{"email": "x@example.com"}`); code != 409 {
		t.Fatalf("one a sign-in used: %d", code)
	}
}

// A provider's groups (AP Oidc v1.1, Block 8, D22-A): an account in none
// of the groups it admits is refused, a sign-in and a link alike; a list
// or one string; another claim, from the userinfo too; none named admits
// any.
func TestOIDCGroups(t *testing.T) {
	f := newFakeIdP(t)
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	ctx := t.Context()
	if _, err := st.AddProvider(ctx, store.AuthProvider{Key: "cloud", Issuer: f.srv.URL, ClientID: fakeClientID, ClientSecret: f.secret, Enabled: true,
		AllowedGroups: []string{"family", " family", ""}}, 0); err != nil {
		t.Fatal(err)
	}
	p, _, _ := st.Provider(ctx, "cloud")
	if p.GroupsClaim != "groups" || strings.Join(p.AllowedGroups, ",") != "family" {
		t.Fatalf("the provider: %+v", p)
	}
	anna := userToken(t, st, "anna", store.RoleOperator)
	annaU, _, _ := st.User(ctx, "anna")
	st.AddSubjectIdentity(ctx, annaU.ID, "cloud", "a1", 0)
	for name, groups := range map[string]any{"none": nil, "others": []string{"guests"}, "a number": 7} {
		if run := f.signIn(t, ts, "cloud", map[string]any{"sub": "a1", "groups": groups}); run.fragment != "oidc-error=group" {
			t.Errorf("%s: %+v", name, run)
		}
	}
	for name, groups := range map[string]any{"a list": []string{"x", "family"}, "a string": "family"} {
		if who := signedInAs(t, ts, f.signIn(t, ts, "cloud", map[string]any{"sub": "a1", "groups": groups})); who != "anna" {
			t.Errorf("%s: %q", name, who)
		}
	}
	code, text := callAPI(t, ts, "POST", "/api/v1/users/anna/identities/link/cloud", anna, `{"redirect": "`+ts.URL+`/"}`)
	if code != 200 {
		t.Fatalf("link: %d %s", code, text)
	}
	if run := f.grant(t, ts, text, map[string]any{"sub": "a2"}); run.fragment != "oidc-link-error=group" {
		t.Fatalf("a link outside the groups: %+v", run)
	}
	// another claim
	st.UpdateProvider(ctx, "cloud", func(p *store.AuthProvider) error { p.GroupsClaim = "roles"; return nil }, 0)
	if who := signedInAs(t, ts, f.signIn(t, ts, "cloud", map[string]any{"sub": "a1", "groups": "nope", "roles": []string{"family"}})); who != "anna" {
		t.Fatalf("the roles claim: %q", who)
	}
	var list []identityView
	if getJSON(t, ts, anna, "/api/v1/users/anna/identities", &list); len(list) != 1 || strings.Join(list[0].Groups, ",") != "family" {
		t.Fatalf("the groups kept: %+v", list)
	}
	// none named: any
	st.UpdateProvider(ctx, "cloud", func(p *store.AuthProvider) error { p.AllowedGroups = nil; return nil }, 0)
	if who := signedInAs(t, ts, f.signIn(t, ts, "cloud", map[string]any{"sub": "a1"})); who != "anna" {
		t.Fatalf("no restriction: %q", who)
	}
}
