// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/store"
)

// roleSets are the default roles' capabilities as docs/store.md section
// 6 seeds them; owner has every one.
var roleSets = map[string][]string{
	store.RoleManager:     {"read", "preferences", "setpoint", "lock", "configure", "rename", "structure", "pair", "discover", "backup", "restore", "guests", "restart", "integrations"},
	store.RoleOperator:    {"read", "preferences", "setpoint", "lock", "configure", "discover", "backup"},
	store.RoleGuest:       {"read", "setpoint", "configure"},
	store.RoleViewer:      {"read", "preferences"},
	store.RoleIntegration: integrationCeiling,
}

// storeOptions are the options of a server over a store in dir with a
// bootstrap token: discovery answers at once, so a run of every route
// stays quick.
func storeOptions(t *testing.T, dir, token string) Options {
	t.Helper()
	st := testStore(t, dir)
	// The fake Cube has a row, as a served Cube has in production, so
	// that the audit can name it; the runtime itself comes from Cubes.
	if _, ok, _ := st.Cube(t.Context(), "KEQ0000100"); !ok {
		if _, err := st.AddCube(t.Context(), store.Cube{Serial: "KEQ0000100", Configured: true, Source: store.SourceEnv}); err != nil {
			t.Fatal(err)
		}
	}
	return Options{Store: st, Token: token, DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{"127.0.0.1:9"}, DiscoveryWait: 20 * time.Millisecond,
		pushClient: loopbackOnly, pushRetry: []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond}}
}

// userToken adds a user with a role and returns an API key of it.
func userToken(t *testing.T, st *store.Store, name, role string) string {
	t.Helper()
	u, err := st.AddUser(t.Context(), store.User{Name: name, Role: role}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := st.AddToken(t.Context(), u.ID, store.TokenAPI, "test", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	return secret
}

// callAPI sends a request with a token and a body and returns the status
// and the body; the event stream's body is not read.
func callAPI(t *testing.T, ts *httptest.Server, method, path, token, body string) (int, string) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, ts.URL+path, rd)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if p, _, _ := strings.Cut(path, "?"); strings.HasSuffix(p, "/events") && resp.StatusCode == 200 {
		return resp.StatusCode, ""
	}
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// errorOf reads the error of a JSON error body.
func errorOf(body string) string {
	var e struct {
		Error string `json:"error"`
	}
	json.Unmarshal([]byte(body), &e)
	return e.Error
}

// fillRoute puts test values into a route pattern.
func fillRoute(path string) string {
	r := strings.NewReplacer("{sn}", "KEQ0000100", "{rf}", "0a010d", "{id}", "6", "{name}", "someone", "{key}", "night", "{tid}", "1")
	return r.Replace(path)
}

// MASSIMO_API_TOKEN bootstraps the owner, follows the file at later
// starts, and a key of the owner's own survives all that.
func TestBootstrapThroughServer(t *testing.T) {
	dir := t.TempDir()
	_, ts, s := startWith(t, storeOptions(t, dir, "secret"))
	if s.AuthMode() != "users" {
		t.Fatalf("mode %s", s.AuthMode())
	}
	for _, c := range []struct {
		token string
		want  int
	}{{"secret", 200}, {"", 401}, {"wrong", 401}} {
		if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", c.token, ""); code != c.want {
			t.Fatalf("token %q: %d, want %d", c.token, code, c.want)
		}
	}
	users, _ := s.opts.Store.Users(t.Context())
	if len(users) != 1 || users[0].Name != store.BootstrapUser || users[0].Role != store.RoleOwner {
		t.Fatalf("users %+v", users)
	}
	_, own, _ := s.opts.Store.AddToken(t.Context(), users[0].ID, store.TokenAPI, "Loxone", time.Time{})
	tk := ticketFor(t, ts, "secret", "KEQ0000100")
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes/KEQ0000100/events?ticket="+tk, "", ""); code != 200 {
		t.Fatalf("stream with a ticket: %d", code)
	}
	ts.Close()
	s.opts.Store.Close()

	// The file's token changed: the old key is revoked, the new one works.
	_, ts, s = startWith(t, storeOptions(t, dir, "other"))
	for _, c := range []struct {
		token string
		want  int
	}{{"secret", 401}, {"other", 200}, {own, 200}} {
		if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", c.token, ""); code != c.want {
			t.Fatalf("after the change, token %q: %d, want %d", c.token, code, c.want)
		}
	}
	ts.Close()
	s.opts.Store.Close()

	// The variable removed: its key is revoked, the user's own key stays,
	// and the API still needs a token.
	_, ts, s = startQuiet(t, storeOptions(t, dir, ""))
	for _, c := range []struct {
		token string
		want  int
	}{{"other", 401}, {"", 401}, {own, 200}} {
		if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", c.token, ""); code != c.want {
			t.Fatalf("after the removal, token %q: %d, want %d", c.token, code, c.want)
		}
	}
}

// A store without users is open; the first user closes it.
func TestOpenMode(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), ""))
	if s.AuthMode() != "open" {
		t.Fatalf("mode %s", s.AuthMode())
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6/setpoint", "", `{"mode":"auto"}`); code != 200 {
		t.Fatalf("open server: %d", code)
	}
	tok := userToken(t, s.opts.Store, "anna", store.RoleViewer)
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", "", ""); code != 401 {
		t.Fatalf("after the first user, without a token: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", tok, ""); code != 200 || s.AuthMode() != "users" {
		t.Fatalf("after the first user, with one: %d %s", code, s.AuthMode())
	}
}

// Each default role against each route: the registered capability
// decides, and nothing else answers 403 with that message.
func TestRolesAgainstRoutes(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	routes := registeredRoutes(t)
	// Removing the Cube last, so that the other routes still find it.
	sort.SliceStable(routes, func(i, j int) bool {
		di, dj := routes[i].method == "DELETE" && routes[i].path == "/api/v1/cubes/{sn}", routes[j].method == "DELETE" && routes[j].path == "/api/v1/cubes/{sn}"
		return !di && dj
	})
	roles := []string{store.RoleViewer, store.RoleIntegration, store.RoleGuest, store.RoleOperator, store.RoleManager, store.RoleOwner}
	for _, role := range roles {
		tok := userToken(t, s.opts.Store, "u-"+role, role)
		has := map[string]bool{}
		for _, c := range roleSets[role] {
			has[c] = true
		}
		for _, r := range routes {
			body := ""
			if r.method == "PUT" || r.method == "POST" {
				body = "{}"
			}
			if r.method == "POST" && r.path == "/api/v1/users" {
				body = `{"role":"guest"}` // what a guests holder may create; the empty name fails after the gate
			}
			code, text := callAPI(t, ts, r.method, fillRoute(r.path), tok, body)
			refused := code == 403 && strings.Contains(text, "capability")
			// The users routes admit a guests holder too; the handlers narrow.
			want := role == store.RoleOwner || has[r.capability] || r.capability == capNone || r.capability == capAny || r.capability == "users" && has["guests"]
			if want && refused {
				t.Errorf("%s: %s %s refused: %s", role, r.method, r.path, text)
			}
			if !want && !refused {
				t.Errorf("%s: %s %s answered %d %s, want 403 for %q", role, r.method, r.path, code, text, r.capability)
			}
			if !want && errorOf(text) != `capability "`+r.capability+`" required` {
				t.Errorf("%s: %s %s: message %q", role, r.method, r.path, text)
			}
		}
	}
}

// A move needs structure and a rename needs rename, on the one route.
func TestMoveNeedsStructure(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	st.AddRole(t.Context(), store.Role{Key: "mover", Capabilities: []string{"read", "structure"}}, 0)
	st.AddRole(t.Context(), store.Role{Key: "namer", Capabilities: []string{"read", "rename"}}, 0)
	mover := userToken(t, st, "mover", "mover")
	namer := userToken(t, st, "namer", "namer")
	op := userToken(t, st, "op", store.RoleOperator)
	path := "/api/v1/cubes/KEQ0000100/devices/0a010d"
	for _, c := range []struct {
		who, token, body, want string
	}{
		{"operator", op, `{"room_id":6}`, "rename"},
		{"operator", op, `{"name":"x"}`, "rename"},
		{"mover", mover, `{"name":"x"}`, "rename"},
		{"mover", mover, `{"room_id":6}`, ""},
		{"namer", namer, `{"room_id":6}`, "structure"},
		{"namer", namer, `{"name":"x"}`, ""},
		{"namer", namer, `{"name":"x","room_id":6}`, "structure"},
	} {
		code, text := callAPI(t, ts, "PUT", path, c.token, c.body)
		if c.want == "" && code == 403 || c.want != "" && (code != 403 || errorOf(text) != `capability "`+c.want+`" required`) {
			t.Errorf("%s with %s: %d %s, want refused for %q", c.who, c.body, code, text, c.want)
		}
	}
}

// The copy of the room's settings after a move needs configure as well:
// a role with structure alone is refused before anything is sent, and
// moves without the copy.
func TestCopyNeedsConfigure(t *testing.T) {
	fc, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	st.AddRole(t.Context(), store.Role{Key: "mover", Capabilities: []string{"read", "structure"}}, 0)
	mover := userToken(t, st, "mover", "mover")
	path := "/api/v1/cubes/KEQ0000100/devices/0a010d"
	if code, text := callAPI(t, ts, "PUT", path, mover, `{"room_id":1}`); code != 403 || errorOf(text) != `capability "configure" required` {
		t.Fatalf("a move with the copy and without configure: %d %s", code, text)
	}
	if got := fc.Received(); len(got) != 0 {
		t.Fatalf("sent before the refusal: %v", got)
	}
	if code, text := callAPI(t, ts, "PUT", path, mover, `{"room_id":1,"copy":false}`); code != 200 || strings.Contains(text, "copied") {
		t.Fatalf("a move without the copy: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "PUT", path, "secret", `{"room_id":2}`); code != 200 || !strings.Contains(text, "copied") {
		t.Fatalf("the owner's move into room 2, with the copy: %d %s", code, text)
	}
}

// A created role, a disabled capability, a disabled role, a disabled or
// expired user, and a revoked or expired token.
func TestRoleRules(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	ctx := t.Context()
	setpoint := "/api/v1/cubes/KEQ0000100/rooms/6/setpoint"
	lock := "/api/v1/cubes/KEQ0000100/devices/0a010d/lock"
	if _, err := st.AddRole(ctx, store.Role{Key: "night", Capabilities: []string{"read", "setpoint"}}, 0); err != nil {
		t.Fatal(err)
	}
	night := userToken(t, st, "n", "night")
	if code, _ := callAPI(t, ts, "PUT", setpoint, night, `{"mode":"auto"}`); code != 200 {
		t.Fatalf("created role's setpoint: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", lock, night, `{"locked":true}`); code != 403 {
		t.Fatalf("created role's lock: %d", code)
	}

	op := userToken(t, st, "op", store.RoleOperator)
	st.SetCapabilityDisabled(ctx, "setpoint", true, 0)
	if code, text := callAPI(t, ts, "PUT", setpoint, op, `{"mode":"auto"}`); code != 403 {
		t.Fatalf("disabled capability: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", setpoint, "secret", `{"mode":"auto"}`); code != 200 {
		t.Fatalf("owner with a disabled capability: %d", code)
	}
	st.SetCapabilityDisabled(ctx, "setpoint", false, 0)
	if code, _ := callAPI(t, ts, "PUT", setpoint, op, `{"mode":"auto"}`); code != 200 {
		t.Fatalf("capability enabled again: %d", code)
	}

	st.SetRoleDisabled(ctx, store.RoleOperator, true, 0)
	if code, text := callAPI(t, ts, "GET", "/api/v1/cubes", op, ""); code != 403 || !strings.Contains(text, "role operator is disabled") {
		t.Fatalf("disabled role: %d %s", code, text)
	}
	st.SetRoleDisabled(ctx, store.RoleOperator, false, 0)

	st.UpdateUser(ctx, "op", func(u *store.User) error { u.Disabled = true; return nil }, 0)
	if code, text := callAPI(t, ts, "GET", "/api/v1/cubes", op, ""); code != 403 || !strings.Contains(text, "disabled") {
		t.Fatalf("disabled user: %d %s", code, text)
	}
	st.UpdateUser(ctx, "op", func(u *store.User) error { u.Disabled = false; return nil }, 0)

	guest := userToken(t, st, "g", store.RoleGuest)
	st.UpdateUser(ctx, "g", func(u *store.User) error { u.ExpiresAt = time.Now().Add(time.Hour); return nil }, 0)
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", guest, ""); code != 200 {
		t.Fatalf("guest before the end: %d", code)
	}
	st.UpdateUser(ctx, "g", func(u *store.User) error { u.ExpiresAt = time.Now().Add(-time.Minute); return nil }, 0)
	if code, text := callAPI(t, ts, "GET", "/api/v1/cubes", guest, ""); code != 403 || !strings.Contains(text, "expired") {
		t.Fatalf("guest past the end: %d %s", code, text)
	}

	// A disabled user's refusal is not a failed check, a revoked token's is.
	u, _, _ := st.User(ctx, "op")
	list, _ := st.Tokens(ctx, u.ID)
	st.RevokeToken(ctx, list[0].ID)
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", op, ""); code != 401 {
		t.Fatalf("revoked token: %d", code)
	}
	_, expired, _ := st.AddToken(ctx, u.ID, store.TokenSession, "old", time.Now().Add(-time.Minute))
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", expired, ""); code != 401 {
		t.Fatalf("expired session: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/healthz", "", ""); code != 200 {
		t.Fatalf("healthz: %d", code)
	}
}

// The lockout counters are in the store, so a restart of the server
// does not clear them.
func TestLockoutSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	opts := storeOptions(t, dir, "secret")
	opts.AuthFailures = 3
	_, ts, s := startWith(t, opts)
	for i := 0; i < 3; i++ {
		if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", "wrong", ""); code != 401 {
			t.Fatalf("failure %d: %d", i+1, code)
		}
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", "secret", ""); code != 429 {
		t.Fatalf("after the limit: %d", code)
	}
	ts.Close()
	s.opts.Store.Close()
	opts = storeOptions(t, dir, "secret")
	opts.AuthFailures = 3
	_, ts, s = startQuiet(t, opts)
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", "secret", ""); code != 429 {
		t.Fatalf("after the restart: %d", code)
	}
}

// startQuiet starts a server over the fake Cube like startWith, but
// waits for the connection through the runtime rather than the API,
// for a server whose API refuses the test's address or token.
func startQuiet(t *testing.T, opts Options) (*fakecube.Server, *httptest.Server, *Server) {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	opts.Cubes = []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}
	opts.Queue = engine.Options{Gap: time.Millisecond}
	s := New(opts)
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, rt := range s.runtimes() {
			if rt.client.Connected() && rt.client.Snapshot().Hello() != nil {
				return fc, ts, s
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("cube never connected")
	return nil, nil, nil
}
