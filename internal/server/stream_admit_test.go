// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// openStream opens a Cube's event stream with the token, "" for none,
// and returns the watch of what it sends, which ends with the stream.
func openStream(t *testing.T, ts *httptest.Server, token string) *testkit.Watch[testkit.Event] {
	t.Helper()
	return openStreamWith(t, ts, token, nil)
}

// openStreamWith is openStream with the headers given, such as an
// integration's X-MassimoHA-On-Behalf-Of (as, onbehalf_test.go).
func openStreamWith(t *testing.T, ts *httptest.Server, token string, headers map[string]string) *testkit.Watch[testkit.Event] {
	t.Helper()
	return openStreamAt(t, ts, "", token, headers)
}

// openTicketStream opens the stream with a ticket (ticketFor), without a
// token or a header, as the page does.
func openTicketStream(t *testing.T, ts *httptest.Server, ticket string) *testkit.Watch[testkit.Event] {
	t.Helper()
	return openStreamAt(t, ts, "?ticket="+ticket, "", nil)
}

// openStreamAt is openStreamWith with the query given, "" for none.
func openStreamAt(t *testing.T, ts *httptest.Server, query, token string, headers map[string]string) *testkit.Watch[testkit.Event] {
	t.Helper()
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/cubes/KEQ0000100/events"+query, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		t.Fatalf("stream: %d", resp.StatusCode)
	}
	return testkit.StreamWatch(t, resp.Body)
}

// isKeepAlive matches the stream's keep-alive comment.
func isKeepAlive(e testkit.Event) bool { return e.Kind == "" && e.Comment == "keep-alive" }

// An event stream asks at each keep-alive whether its caller would be
// let in now: one opened on an open server ends once a user exists, one
// opened with a key ends once the key is revoked, one opened with a
// key's session once the key is past its end, and one whose caller is
// still allowed goes on.
func TestStreamEndsWhenItsCallerIsNotLetIn(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "")))
	ts, s := r.ts, r.s
	s.streamKeepAlive.Store(int64(30 * time.Millisecond))
	st := s.opts.Store
	open := openStream(t, ts, "")
	open.Await(t, 0, "a keep-alive on the open server", isKeepAlive)
	if _, err := st.AddUser(t.Context(), store.User{Name: "owner1", Role: store.RoleOwner}, "placeholder-pass", 0); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the end of the open server's stream at its first user", open.Ended)

	viewer := userToken(t, st, "viewer1", store.RoleViewer)
	operator := userToken(t, st, "operator1", store.RoleOperator)
	stays := openStream(t, ts, operator)
	revoked := openStream(t, ts, viewer)
	toks, err := st.Tokens(t.Context(), mustUser(t, st, "viewer1").ID)
	if err != nil || len(toks) != 1 {
		t.Fatalf("the viewer's tokens: %v %v", toks, err)
	}
	if _, err := st.RevokeToken(t.Context(), toks[0].ID); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the end of the stream of a revoked key", revoked.Ended)
	// Counted from the revoked stream's end: keep-alives that came before
	// the revocation say nothing of it.
	stays.AwaitN(t, stays.Mark(), "three keep-alives for a caller still allowed, after the revocation", 3, isKeepAlive)

	// One opened with a session made from a key ends once the key is past
	// its end (AP KeyLife, Block 2).
	key := userToken(t, st, "viewer2", store.RoleViewer)
	code, text := callAPI(t, ts, "POST", "/api/v1/login/key", key, "")
	var made struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(text), &made); err != nil || code != 200 || made.Token == "" {
		t.Fatalf("a session from the key: %d %s %v", code, text, err)
	}
	ofKey := openStream(t, ts, made.Token)
	ofKey.Await(t, 0, "a keep-alive of the key's session", isKeepAlive)
	keyID := int64(0)
	toks, err = st.Tokens(t.Context(), mustUser(t, st, "viewer2").ID)
	for _, tk := range toks {
		if tk.Kind == store.TokenAPI {
			keyID = tk.ID
		}
	}
	if err != nil || keyID == 0 {
		t.Fatalf("the key of viewer2: %+v %v", toks, err)
	}
	endToken(t, st, keyID)
	testkit.Eventually(t, "the end of the stream of a key's session at the key's end", ofKey.Ended)
}

func mustUser(t *testing.T, st *store.Store, name string) store.User {
	t.Helper()
	u, ok, err := st.User(t.Context(), name)
	if err != nil || !ok {
		t.Fatalf("user %s: %v %v", name, ok, err)
	}
	return u
}

// Without a store, a stream opened with the one token ends once the
// settings file gives the server another (SetToken).
func TestLegacyStreamEndsWithItsToken(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(Options{Token: "secret"}))
	ts, s := r.ts, r.s
	s.streamKeepAlive.Store(int64(30 * time.Millisecond))
	evs := openStream(t, ts, "secret")
	evs.Await(t, 0, "a keep-alive", isKeepAlive)
	if err := s.SetToken(t.Context(), "placeholder-token-2"); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the end of the stream at a new token", evs.Ended)
}
