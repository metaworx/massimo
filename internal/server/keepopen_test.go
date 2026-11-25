// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// sendFrom answers a request with a body as if it came from addr.
func sendFrom(s *Server, method, path, addr, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = addr
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

// keepOpenOf reads the setting auth.keep_open from GET /api/v1/settings.
func keepOpenOf(t *testing.T, s *Server, token string) settingView {
	t.Helper()
	rec := sendFrom(s, "GET", "/api/v1/settings", "127.0.0.1:5000", token, "")
	var all map[string]settingView
	if json.Unmarshal(rec.Body.Bytes(), &all); rec.Code != 200 {
		t.Fatalf("settings: %d %s", rec.Code, rec.Body.String())
	}
	v, ok := all[store.KeepOpenKey]
	if !ok {
		t.Fatalf("no %s in %s", store.KeepOpenKey, rec.Body.String())
	}
	return v
}

// meFrom reads GET /api/v1/me as a caller at addr.
func meFrom(t *testing.T, s *Server, addr, token string) (int, meView, string) {
	t.Helper()
	rec := sendFrom(s, "GET", "/api/v1/me", addr, token, "")
	var me meView
	json.Unmarshal(rec.Body.Bytes(), &me)
	return rec.Code, me, rec.Body.String()
}

// On an open server the setting is off by default, set and reset by
// anyone on the local network, and /me says kept_open while it is on;
// it changes nothing about who is let in: beyond the local network the
// server refuses as before, the setting's PUT included, and the sign-in
// answers as before (AP FirstOwner, D11). The checks from beyond the
// local network and of the sign-in are forward guards: this code is not
// the block's, and they keep it so.
func TestKeepOpenOnAnOpenServer(t *testing.T) {
	t.Parallel()
	s := startServer(t, withOptions(storeOptions(t, t.TempDir(), ""))).s
	lan, far := "192.168.1.20:5000", "203.0.113.9:5000"
	if v := keepOpenOf(t, s, ""); v.Value != "false" || v.Default != "false" || v.Set || v.InEffect != "false" || v.Applies != "now" {
		t.Fatalf("the default: %+v", v)
	}
	if code, me, text := meFrom(t, s, lan, ""); code != 200 || me.Mode != "open" || !me.Setup || me.KeptOpen || strings.Contains(text, "kept_open") {
		t.Fatalf("/me, not kept open: %d %s", code, text)
	}
	if rec := sendFrom(s, "PUT", "/api/v1/settings", far, "", `{"auth.keep_open":true}`); rec.Code != 403 || errorOf(rec.Body.String()) != openRefused {
		t.Fatalf("the PUT from beyond the local network: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendFrom(s, "PUT", "/api/v1/settings", lan, "", `{"auth.keep_open":"yes"}`); rec.Code != 400 {
		t.Fatalf("not a switch: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendFrom(s, "PUT", "/api/v1/settings", lan, "", `{"auth.keep_open":true}`); rec.Code != 200 {
		t.Fatalf("the PUT from the local network: %d %s", rec.Code, rec.Body.String())
	}
	if v := keepOpenOf(t, s, ""); v.Value != "true" || !v.Set || v.InEffect != "true" {
		t.Fatalf("set: %+v", v)
	}
	if code, me, text := meFrom(t, s, lan, ""); code != 200 || me.Mode != "open" || !me.Setup || !me.KeptOpen {
		t.Fatalf("/me, kept open: %d %s", code, text)
	}
	for _, path := range []string{"/api/v1/me", "/api/v1/cubes", "/api/v1/settings"} {
		if rec := sendFrom(s, "GET", path, far, "", ""); rec.Code != 403 || errorOf(rec.Body.String()) != openRefused {
			t.Errorf("%s from beyond the local network, kept open: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	if rec := sendFrom(s, "POST", "/api/v1/login", lan, "", `{"user":"a","password":"b"}`); rec.Code != 409 || !strings.Contains(rec.Body.String(), "no users yet") {
		t.Fatalf("the sign-in, kept open: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendFrom(s, "PUT", "/api/v1/settings", lan, "", `{"auth.keep_open":null}`); rec.Code != 200 {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body.String())
	}
	if v := keepOpenOf(t, s, ""); v.Value != "false" || v.Set {
		t.Fatalf("reset: %+v", v)
	}
	if _, me, _ := meFrom(t, s, lan, ""); me.KeptOpen {
		t.Fatal("/me, reset: kept open")
	}
}

// The first owner by the API takes the setting with it, and /me of the
// owner has no kept_open; on a server with users the setting cannot be
// switched on, off and the default are taken, and it shows off (D12).
func TestKeepOpenGoesWithTheFirstOwner(t *testing.T) {
	t.Parallel()
	s := startServer(t, withOptions(storeOptions(t, t.TempDir(), ""))).s
	lan := "192.168.1.20:5000"
	if rec := sendFrom(s, "PUT", "/api/v1/settings", lan, "", `{"auth.keep_open":true}`); rec.Code != 200 {
		t.Fatalf("set: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendFrom(s, "POST", "/api/v1/users", lan, "", `{"name":"root","password":"root-pass-1"}`); rec.Code != 201 {
		t.Fatalf("the first owner: %d %s", rec.Code, rec.Body.String())
	}
	if _, set, _ := s.opts.Store.Setting(t.Context(), store.KeepOpenKey); set {
		t.Fatal("the setting stays with the first owner")
	}
	key := userToken(t, s.opts.Store, "anna", store.RoleOwner)
	if code, me, text := meFrom(t, s, lan, key); code != 200 || me.Mode != "users" || me.KeptOpen || strings.Contains(text, "kept_open") {
		t.Fatalf("/me with users: %d %s", code, text)
	}
	rec := sendFrom(s, "PUT", "/api/v1/settings", lan, key, `{"auth.keep_open":true}`)
	if rec.Code != 409 || !strings.Contains(errorOf(rec.Body.String()), "without users") {
		t.Fatalf("on with users: %d %s", rec.Code, rec.Body.String())
	}
	for _, body := range []string{`{"auth.keep_open":false}`, `{"auth.keep_open":null}`} {
		if rec := sendFrom(s, "PUT", "/api/v1/settings", lan, key, body); rec.Code != 200 {
			t.Fatalf("%s with users: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	// A value left by a process that does not know the setting shows off.
	if err := s.opts.Store.SetSetting(t.Context(), store.KeepOpenKey, true); err != nil {
		t.Fatal(err)
	}
	if v := keepOpenOf(t, s, key); v.InEffect != "false" {
		t.Fatalf("with users: %+v", v)
	}
	if code, me, _ := meFrom(t, s, lan, key); code != 200 || me.KeptOpen {
		t.Fatal("/me with users: kept open")
	}
}

// The server removes a setting left beside users at its start and with
// DropKeepOpen, which a reload calls (D12).
func TestKeepOpenDroppedAtTheStart(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "")
	st := opts.Store
	if _, err := st.AddUser(t.Context(), store.User{Name: "anna", Role: store.RoleOwner}, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(t.Context(), store.KeepOpenKey, true); err != nil {
		t.Fatal(err)
	}
	s := New(opts)
	if st.KeepOpen(t.Context()) {
		t.Fatal("kept open after the start")
	}
	if err := st.SetSetting(t.Context(), store.KeepOpenKey, true); err != nil {
		t.Fatal(err)
	}
	s.DropKeepOpen(t.Context())
	if st.KeepOpen(t.Context()) {
		t.Fatal("kept open after DropKeepOpen")
	}
}

// A server without a store has no setting: no kept_open, and the
// settings' routes answer as before.
func TestKeepOpenWithoutAStore(t *testing.T) {
	t.Parallel()
	s := New(Options{})
	if code, me, text := meFrom(t, s, "192.168.1.20:5000", ""); code != 200 || me.Mode != "none" || me.KeptOpen || strings.Contains(text, "kept_open") {
		t.Fatalf("/me: %d %s", code, text)
	}
	if rec := sendFrom(s, "PUT", "/api/v1/settings", "192.168.1.20:5000", "", `{"auth.keep_open":true}`); rec.Code != 503 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	s.DropKeepOpen(t.Context())
	tok := New(Options{Token: "secret"})
	if code, me, text := meFrom(t, tok, "192.168.1.20:5000", "secret"); code != 200 || me.Mode != "token" || me.KeptOpen || strings.Contains(text, "kept_open") {
		t.Fatalf("/me with a token: %d %s", code, text)
	}
}

// recordFrom runs a registration for a name through the route as a
// caller on the local network, and returns the record to send.
func recordFrom(t *testing.T, s *Server, name, password string) string {
	t.Helper()
	c := opaqueClient(t)
	req, err := c.RegistrationInit([]byte(password))
	if err != nil {
		t.Fatal(err)
	}
	rec := sendFrom(s, "POST", "/api/v1/users/"+name+"/register", "192.168.1.20:5000", "", `{"request":"`+opaqueB64.EncodeToString(req.Serialize())+`"}`)
	if rec.Code != 200 {
		t.Fatalf("register %s: %d %s", name, rec.Code, rec.Body.String())
	}
	var out struct{ Response string }
	json.Unmarshal(rec.Body.Bytes(), &out)
	b, _ := opaqueB64.DecodeString(out.Response)
	resp, err := c.Deserialize.RegistrationResponse(b)
	if err != nil {
		t.Fatal(err)
	}
	record, _, err := c.RegistrationFinalize(resp, nil, nil, opaqueOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	return opaqueB64.EncodeToString(record.Serialize())
}

// startedReading is a request body that says when the handler first
// reads it, and hands out what its pipe is given.
type startedReading struct {
	*io.PipeReader
	once  sync.Once
	began chan struct{}
}

func (b *startedReading) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.began) })
	return b.PipeReader.Read(p)
}

// A first owner whose request passed the check of the mode while the
// server was open, and is held there until another one has been
// created, is 409, by a password and by OPAQUE alike: the store counts
// the users again as it writes (D15). The handler reads the body after
// authorize, so a body that blocks holds it there; a 401 would say that
// the mode was read after the first owner.
func TestFirstOwnerHeldIsRefused(t *testing.T) {
	t.Parallel()
	for _, opaque := range []bool{false, true} {
		s := New(storeOptions(t, t.TempDir(), ""))
		first, second := `{"name":"anna","password":"anna-pass-1"}`, `{"name":"bob","password":"bob-pass-1"}`
		if opaque {
			first = `{"name":"anna","record":"` + recordFrom(t, s, "anna", "p") + `","ksf":"` + store.OpaqueKSF + `"}`
			second = `{"name":"bob","record":"` + recordFrom(t, s, "bob", "p") + `","ksf":"` + store.OpaqueKSF + `"}`
		}
		pr, pw := io.Pipe()
		body := &startedReading{PipeReader: pr, began: make(chan struct{})}
		r := httptest.NewRequest("POST", "/api/v1/users", body)
		r.RemoteAddr = "192.168.1.20:5000"
		r.Header.Set("Content-Type", "application/json")
		held := httptest.NewRecorder()
		done := make(chan struct{})
		go func() {
			defer close(done)
			s.Handler().ServeHTTP(held, r)
		}()
		select {
		case <-body.began:
		case <-done:
			t.Fatalf("opaque %v: answered before reading its body: %d %s", opaque, held.Code, held.Body.String())
		}
		if rec := sendFrom(s, "POST", "/api/v1/users", "192.168.1.20:5000", "", first); rec.Code != 201 {
			t.Fatalf("opaque %v: the first: %d %s", opaque, rec.Code, rec.Body.String())
		}
		io.WriteString(pw, second)
		pw.Close()
		<-done
		if held.Code != 409 || errorOf(held.Body.String()) != "the first owner exists already: sign in" {
			t.Fatalf("opaque %v: the held one: %d %s", opaque, held.Code, held.Body.String())
		}
		if n, _ := s.opts.Store.CountUsers(t.Context()); n != 1 {
			t.Fatalf("opaque %v: %d users", opaque, n)
		}
	}
}

// Two first owners at once, by password and by OPAQUE, round after
// round: never two owners (D15). A round in which the second came after
// the first one's commit is 201 and 401 and raced nothing; the test above
// holds the race without the timing. A registration keeps nothing, so a
// refused one leaves nothing behind.
func TestFirstOwnerOnce(t *testing.T) {
	t.Parallel()
	raced := 0
	for round := range 12 {
		s := New(storeOptions(t, t.TempDir(), ""))
		bodies := []string{`{"name":"anna","password":"anna-pass-1"}`, `{"name":"bob","password":"bob-pass-1"}`}
		if round%2 == 1 {
			bodies = []string{
				`{"name":"anna","record":"` + recordFrom(t, s, "anna", "p") + `","ksf":"` + store.OpaqueKSF + `"}`,
				`{"name":"bob","record":"` + recordFrom(t, s, "bob", "p") + `","ksf":"` + store.OpaqueKSF + `"}`,
			}
		}
		recs := make(chan *httptest.ResponseRecorder, len(bodies))
		start := make(chan struct{})
		for _, body := range bodies {
			go func() {
				<-start
				recs <- sendFrom(s, "POST", "/api/v1/users", "192.168.1.20:5000", "", body)
			}()
		}
		close(start)
		codes := map[int]int{}
		for range bodies {
			rec := <-recs
			codes[rec.Code]++
			if rec.Code == 409 && errorOf(rec.Body.String()) != "the first owner exists already: sign in" {
				t.Errorf("round %d: %s", round, rec.Body.String())
			}
		}
		// A request that came after the first one's commit finds users
		// and no token: 401, and no race.
		n, _ := s.opts.Store.CountUsers(t.Context())
		if codes[201] != 1 || codes[409]+codes[401] != 1 || n != 1 {
			t.Fatalf("round %d: %v, %d users", round, codes, n)
		}
		raced += codes[409]
	}
	t.Logf("%d of 12 rounds raced", raced)
}

// An integration approved on an open server kept open leaves it open:
// /me still says open, setup and kept_open, a read without a token is
// answered, and the first owner can still be made, who then closes the
// server and takes the setting with it. Before AP LooseEnds (D1) the
// approval closed the server, and nobody could sign in.
func TestIntegrationLeavesAnOpenServerOpen(t *testing.T) {
	t.Parallel()
	s := startServer(t, withOptions(storeOptions(t, t.TempDir(), ""))).s
	lan := "192.168.1.20:5000"
	if rec := sendFrom(s, "PUT", "/api/v1/settings", lan, "", `{"auth.keep_open":true}`); rec.Code != 200 {
		t.Fatalf("kept open: %d %s", rec.Code, rec.Body.String())
	}
	rec := sendFrom(s, "POST", "/api/v1/integrations/requests", lan, "",
		`{"kind":"homeassistant","name":"Casa Leone","instance_id":"0123456789ABCDEF0123456789abcdef","url":"http://ha:8123","version":"2026.11.1"}`)
	if rec.Code != 201 {
		t.Fatalf("the request: %d %s", rec.Code, rec.Body.String())
	}
	var req struct{ ID, Code string }
	decodeInto(t, rec.Body.String(), &req)
	if rec := sendFrom(s, "POST", "/api/v1/integrations/"+req.ID+"/approve", lan, "", `{"max_caps":["read"]}`); rec.Code != 200 {
		t.Fatalf("the approval: %d %s", rec.Code, rec.Body.String())
	}
	if n, _ := s.opts.Store.CountIntegrationUsers(t.Context()); n != 2 {
		t.Fatalf("%d integration users", n)
	}
	if code, me, text := meFrom(t, s, lan, ""); code != 200 || me.Mode != "open" || !me.Setup || !me.KeptOpen {
		t.Fatalf("/me after the approval: %d %s", code, text)
	}
	if rec := sendFrom(s, "GET", "/api/v1/cubes", lan, "", ""); rec.Code != 200 {
		t.Fatalf("a read without a token: %d %s", rec.Code, rec.Body.String())
	}
	// The integration's key is not looked at: anyone's from the local
	// network, refused from beyond it (D40-A).
	rec = sendFrom(s, "GET", "/api/v1/integrations/requests/"+req.ID+"?code="+req.Code, lan, "", "")
	var got struct{ Key string }
	if decodeInto(t, rec.Body.String(), &got); rec.Code != 200 || got.Key == "" {
		t.Fatalf("the key collected: %d %s", rec.Code, rec.Body.String())
	}
	if code, me, text := meFrom(t, s, lan, got.Key); code != 200 || me.Mode != "open" || me.Integration != nil {
		t.Fatalf("/me with the integration's key: %d %s", code, text)
	}
	if code, _, text := meFrom(t, s, "203.0.113.9:5000", got.Key); code != 403 {
		t.Fatalf("/me with the integration's key from beyond the local network: %d %s", code, text)
	}
	// Anyone on the local network may give an integration's user the role
	// owner, and nothing else: no way in of its own.
	system := req.ID + ":system"
	if rec := sendFrom(s, "PUT", "/api/v1/users/"+system, lan, "", `{"role":"owner"}`); rec.Code != 200 {
		t.Fatalf("the role owner on an open server: %d %s", rec.Code, rec.Body.String())
	}
	refusesWayIn(t, s, lan, "", system)
	if rec := sendFrom(s, "POST", "/api/v1/users", lan, "", `{"name":"root","password":"root-pass-1"}`); rec.Code != 201 {
		t.Fatalf("the first owner: %d %s", rec.Code, rec.Body.String())
	}
	if code, _, text := meFrom(t, s, lan, ""); code != 401 {
		t.Fatalf("/me without a token after the owner: %d %s", code, text)
	}
	if _, set, _ := s.opts.Store.Setting(t.Context(), store.KeepOpenKey); set {
		t.Fatal("the setting stays with the first owner")
	}
	// Then nothing answers for that user but the integration's key,
	// within the role integration (what it might hold from before:
	// TestHeldCredentialsOfAnIntegrationsUser).
	for _, c := range meCaps(t, s, got.Key, nil) {
		if !slices.Contains(integrationCeiling, c) {
			t.Errorf("the integration's key: %s beyond the role integration", c)
		}
	}
}

// The states an older version can leave, at the start: one without
// users is open without a word; one whose only users are an
// integration's is open and warns, once its owner is removed or its
// integration revoked too, but not while it is kept open on purpose,
// which then stays; with a regular user, a disabled owner alone among
// them, it is in users mode, without a warning, and a setting left goes
// (AP LooseEnds, D1, D2).
func TestOpenWarningAtTheStart(t *testing.T) {
	t.Parallel()
	const warning = "the server is open: its only users are an integration's; create the owner"
	for _, c := range []struct {
		name                  string
		integration, keepOpen bool
		make                  func(t *testing.T, st *store.Store)
		mode                  string
		warns, keptOpen       bool
	}{
		{name: "no users at all", mode: "open"},
		{name: "integration users alone", integration: true, mode: "open", warns: true},
		{name: "kept open beside them", integration: true, keepOpen: true, mode: "open", keptOpen: true},
		{name: "beside a regular owner", integration: true, keepOpen: true, mode: "users",
			make: func(t *testing.T, st *store.Store) { userToken(t, st, "anna", store.RoleOwner) }},
		{name: "a removed owner beside them", integration: true, mode: "open", warns: true, make: func(t *testing.T, st *store.Store) {
			if _, err := st.AddUser(t.Context(), store.User{Name: "anna", Role: store.RoleOwner, ExpiresAt: time.Now().Add(time.Hour)}, "", 0); err != nil {
				t.Fatal(err)
			}
			if err := st.DeleteUser(t.Context(), "anna", 0); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "a revoked integration's users", integration: true, mode: "open", warns: true, make: func(t *testing.T, st *store.Store) {
			list, _ := st.Integrations(t.Context())
			if err := st.RevokeIntegration(t.Context(), list[0].UUID, 0); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "a disabled owner alone", mode: "users", make: func(t *testing.T, st *store.Store) {
			if _, err := st.AddUser(t.Context(), store.User{Name: "anna", Role: store.RoleOwner, Disabled: true}, "", 0); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			opts := storeOptions(t, t.TempDir(), "")
			st := opts.Store
			if c.integration {
				registerHA(t, st, []string{"read"})
			}
			if c.make != nil {
				c.make(t, st)
			}
			if err := st.SetSetting(t.Context(), store.KeepOpenKey, c.keepOpen); err != nil {
				t.Fatal(err)
			}
			var logs *testkit.Watch[testkit.Record]
			opts.Logger, logs = testkit.LogWatch(nil)
			m := logs.Mark()
			s := New(opts)
			// by the message alone, so that a warning with another count is
			// counted where none is expected; where one is, it counts two
			warned := logs.Count(m, testkit.MsgAt(slog.LevelWarn, warning))
			counted := logs.Count(m, testkit.MsgAt(slog.LevelWarn, warning, "integration_users", "2"))
			if s.AuthMode() != c.mode || (warned == 1) != c.warns || warned > 1 || counted != warned || st.KeepOpen(t.Context()) != c.keptOpen {
				t.Fatalf("%s, %d warnings, kept open %v: %v", s.AuthMode(), warned, st.KeepOpen(t.Context()), logs.Since(m, nil))
			}
		})
	}
}

// The removal of the last regular user turns the server open, and the
// log warns, naming whom and by whom: an owner with an end date, alone
// beside an integration's users, as the command line or an older
// version can leave one, removes itself. The removal of an
// integration's user on a server that was open already says nothing
// (AP LooseEnds, D1).
func TestLastRegularUserRemovedWarns(t *testing.T) {
	t.Parallel()
	const warning = "the server is open: its last regular user was removed; create the owner"
	opts := storeOptions(t, t.TempDir(), "")
	var logs *testkit.Watch[testkit.Record]
	opts.Logger, logs = testkit.LogWatch(nil)
	s := startServer(t, withOptions(opts)).s
	st := s.opts.Store
	lan := "192.168.1.20:5000"
	i, _ := registerHA(t, st, []string{"read"})
	anna, err := st.AddUser(t.Context(), store.User{Name: "anna", Role: store.RoleOwner, ExpiresAt: time.Now().Add(time.Hour)}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := st.AddToken(t.Context(), anna.ID, store.TokenAPI, "test", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	m := logs.Mark()
	if rec := sendFrom(s, "DELETE", "/api/v1/users/anna", lan, key, ""); rec.Code != 200 {
		t.Fatalf("anna removes itself: %d %s", rec.Code, rec.Body.String())
	}
	logs.Await(t, m, "the warning", testkit.MsgAt(slog.LevelWarn, warning, "user", "anna", "by", "anna"))
	if s.AuthMode() != "open" {
		t.Fatalf("after: %s", s.AuthMode())
	}
	m = logs.Mark()
	if rec := sendFrom(s, "DELETE", "/api/v1/users/"+i.UUID+":system", lan, "", ""); rec.Code != 200 {
		t.Fatalf("an integration's user removed on the open server: %d %s", rec.Code, rec.Body.String())
	}
	logs.Await(t, m, "the removal's line", testkit.Msg("user removed"))
	if n := logs.Count(m, testkit.Msg(warning)); n != 0 {
		t.Fatalf("%d warnings for an integration's user on an open server", n)
	}
}
