// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net/http"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func TestAuthLimiter(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)}
	l := newAuthLimiter(3, time.Minute, 5*time.Minute, clock.now)
	if l.fail("a") || l.fail("a") {
		t.Fatal("locked too early")
	}
	// Failures in an earlier window do not count.
	clock.t = clock.t.Add(2 * time.Minute)
	if l.fail("a") || l.fail("a") {
		t.Fatal("old failures counted")
	}
	if !l.fail("a") {
		t.Fatal("third failure in the window did not lock")
	}
	if d := l.lockedFor("a"); d != 5*time.Minute {
		t.Fatalf("locked for %v", d)
	}
	if l.lockedFor("b") != 0 {
		t.Fatal("other address locked")
	}
	l.succeed("a") // a success during the lockout does not lift it
	clock.t = clock.t.Add(5*time.Minute + time.Second)
	if l.lockedFor("a") != 0 {
		t.Fatal("still locked after the lockout")
	}
	l.fail("b")
	l.succeed("b")
	if _, ok := l.addrs["b"]; ok {
		t.Fatal("success did not forget the failures")
	}
}

// send sends a request and returns the response with its body closed; for
// the event stream, the context is cancelled once the headers are in.
func send(t *testing.T, method, url, token string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, method, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestFailedTokensLockTheAddressOut(t *testing.T) {
	_, ts, _ := startWith(t, Options{Token: "secret", AuthFailures: 3})
	cubes := ts.URL + "/api/v1/cubes"
	// A success in between forgets the failures.
	send(t, "GET", cubes, "wrong")
	send(t, "GET", cubes, "wrong")
	if r := send(t, "GET", cubes, "secret"); r.StatusCode != 200 {
		t.Fatalf("right token: %d", r.StatusCode)
	}
	for i := 0; i < 3; i++ {
		if r := send(t, "GET", cubes, "wrong"); r.StatusCode != 401 {
			t.Fatalf("failure %d: %d", i+1, r.StatusCode)
		}
	}
	r := send(t, "GET", cubes, "secret")
	if r.StatusCode != 429 || r.Header.Get("Retry-After") == "" {
		t.Fatalf("after the limit: %d %v", r.StatusCode, r.Header)
	}
	// The lockout covers the stream too.
	if r := send(t, "GET", cubes+"/KEQ0537249/events?token=secret", ""); r.StatusCode != 429 {
		t.Fatalf("stream during the lockout: %d", r.StatusCode)
	}
	// Without a token configured nothing is limited.
	_, open, _ := startWith(t, Options{AuthFailures: 1})
	for i := 0; i < 3; i++ {
		if r := send(t, "GET", open.URL+"/api/v1/cubes", "wrong"); r.StatusCode != 200 {
			t.Fatalf("open server: %d", r.StatusCode)
		}
	}
	if r := send(t, "GET", ts.URL+"/healthz", ""); r.StatusCode != 200 {
		t.Fatalf("healthz during the lockout: %d", r.StatusCode)
	}
}
