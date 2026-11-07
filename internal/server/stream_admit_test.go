// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// openStream opens a Cube's event stream with the token, "" for none,
// and returns a channel that gets the keep-alives it reads and is closed
// when the stream ends.
func openStream(t *testing.T, ts *httptest.Server, token string) <-chan string {
	t.Helper()
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/cubes/KEQ0000100/events", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("stream: %d", resp.StatusCode)
	}
	lines := make(chan string, 64)
	go func() {
		defer close(lines)
		defer resp.Body.Close()
		rd := bufio.NewReader(resp.Body)
		for {
			l, err := rd.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(l, ": keep-alive") {
				lines <- l
			}
		}
	}()
	t.Cleanup(func() { resp.Body.Close() })
	return lines
}

// ended waits for the stream to end, failing after a while.
func ended(t *testing.T, lines <-chan string, what string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-lines:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatalf("%s: the stream did not end", what)
		}
	}
}

// An event stream asks at each keep-alive whether its caller would be
// let in now: one opened on an open server ends once a user exists, one
// opened with a key ends once the key is revoked, and one whose caller
// is still allowed goes on.
func TestStreamEndsWhenItsCallerIsNotLetIn(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), ""))
	s.streamKeepAlive.Store(int64(30 * time.Millisecond))
	st := s.opts.Store
	open := openStream(t, ts, "")
	select {
	case <-open:
	case <-time.After(5 * time.Second):
		t.Fatal("no keep-alive on the open server")
	}
	if _, err := st.AddUser(t.Context(), store.User{Name: "owner1", Role: store.RoleOwner}, "placeholder-pass", 0); err != nil {
		t.Fatal(err)
	}
	ended(t, open, "the open server's first user")

	viewer := userToken(t, st, "viewer1", store.RoleViewer)
	operator := userToken(t, st, "operator1", store.RoleOperator)
	stays := openStream(t, ts, operator)
	revoked := openStream(t, ts, viewer)
	toks, err := st.Tokens(t.Context(), mustUser(t, st, "viewer1").ID)
	if err != nil || len(toks) != 1 {
		t.Fatalf("the viewer's tokens: %v %v", toks, err)
	}
	if err := st.RevokeToken(t.Context(), toks[0].ID); err != nil {
		t.Fatal(err)
	}
	ended(t, revoked, "a revoked key")
	for i := 0; i < 3; i++ {
		select {
		case _, ok := <-stays:
			if !ok {
				t.Fatal("the stream of a caller still allowed ended")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no keep-alive for a caller still allowed")
		}
	}
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
	_, ts, s := startWith(t, Options{Token: "secret"})
	s.streamKeepAlive.Store(int64(30 * time.Millisecond))
	lines := openStream(t, ts, "secret")
	select {
	case <-lines:
	case <-time.After(5 * time.Second):
		t.Fatal("no keep-alive")
	}
	if err := s.SetToken(t.Context(), "placeholder-token-2"); err != nil {
		t.Fatal(err)
	}
	ended(t, lines, "a new token")
}
