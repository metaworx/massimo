// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// serveFrom answers a request as if it came from addr, with the headers.
func serveFrom(s *Server, method, path, addr string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	r.RemoteAddr = addr
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

// A server that asks for no key answers its local network only: the
// box, the private ranges, IPv6 unique local and link-local addresses;
// a public address gets 403 with the reason, and so does a request that
// names a client in a forwarding header without a trusted proxy. With
// users, or a token, the address does not matter.
func TestOpenServerAnswersItsNetwork(t *testing.T) {
	_, _, s := startWith(t, storeOptions(t, t.TempDir(), ""))
	cubes := "/api/v1/cubes"
	for addr, want := range map[string]int{
		"127.0.0.1:5000": 200, "[::1]:5000": 200, "192.168.1.20:5000": 200, "10.0.0.5:5000": 200, "172.16.0.5:5000": 200,
		"[fd00::5]:5000": 200, "[fe80::5]:5000": 200, "169.254.1.5:5000": 200,
		"203.0.113.9:5000": 403, "[2001:db8::1]:5000": 403, "100.64.1.1:5000": 403, "8.8.8.8:5000": 403,
	} {
		rec := serveFrom(s, "GET", cubes, addr, nil)
		if rec.Code != want || want == 403 && errorOf(rec.Body.String()) != openRefused {
			t.Errorf("%s: %d %s, want %d", addr, rec.Code, rec.Body.String(), want)
		}
	}
	for _, h := range []map[string]string{{"X-Forwarded-For": "192.168.1.20"}, {"X-Real-IP": "192.168.1.20"}, {"Forwarded": "for=192.168.1.20"}} {
		if rec := serveFrom(s, "GET", cubes, "127.0.0.1:5000", h); rec.Code != 403 {
			t.Errorf("through a proxy that is not trusted, %v: %d %s", h, rec.Code, rec.Body.String())
		}
	}
	if rec := serveFrom(s, "GET", "/api/v1/cubes/KEQ0000100/events", "203.0.113.9:5000", nil); rec.Code != 403 {
		t.Errorf("the event stream from the internet: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveFrom(s, "GET", "/healthz", "203.0.113.9:5000", nil); rec.Code != 200 {
		t.Errorf("/healthz stays open: %d", rec.Code)
	}
	// A refused write is counted with its address's refusals, as a write
	// refused before anyone is known always is (audit.go).
	if rec := serveFrom(s, "POST", "/api/v1/cubes/discover", "203.0.113.9:5000", nil); rec.Code != 403 {
		t.Errorf("a write from the internet: %d %s", rec.Code, rec.Body.String())
	}
	s.refusals.mu.Lock()
	_, noted := s.refusals.open["203.0.113.9"]
	s.refusals.mu.Unlock()
	if !noted {
		t.Error("the refused write of 203.0.113.9 is not in the audit")
	}

	// Through a trusted proxy, the client it names decides.
	opts := storeOptions(t, t.TempDir(), "")
	trusted, err := ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	opts.TrustedProxies = trusted
	_, _, p := startWith(t, opts)
	if rec := serveFrom(p, "GET", cubes, "127.0.0.1:5000", map[string]string{"X-Forwarded-For": "192.168.1.20"}); rec.Code != 200 {
		t.Errorf("a LAN client through the trusted proxy: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveFrom(p, "GET", cubes, "127.0.0.1:5000", map[string]string{"X-Forwarded-For": "203.0.113.9"}); rec.Code != 403 {
		t.Errorf("an internet client through the trusted proxy: %d %s", rec.Code, rec.Body.String())
	}

	// With users the address does not matter.
	_, _, u := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	if rec := serveFrom(u, "GET", cubes, "203.0.113.9:5000", map[string]string{"Authorization": "Bearer secret"}); rec.Code != 200 {
		t.Errorf("users mode from the internet: %d %s", rec.Code, rec.Body.String())
	}
}

// Without a store, the same holds without a token, and a token lifts it.
func TestOpenLegacyServerAnswersItsNetwork(t *testing.T) {
	s := New(Options{})
	if rec := serveFrom(s, "GET", "/api/v1/cubes", "192.168.1.20:5000", nil); rec.Code != http.StatusOK {
		t.Errorf("from the LAN: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveFrom(s, "GET", "/api/v1/cubes", "203.0.113.9:5000", nil); rec.Code != http.StatusForbidden || errorOf(rec.Body.String()) != openRefused {
		t.Errorf("from the internet: %d %s", rec.Code, rec.Body.String())
	}
	tok := New(Options{Token: "secret"})
	if rec := serveFrom(tok, "GET", "/api/v1/cubes", "203.0.113.9:5000", map[string]string{"Authorization": "Bearer secret"}); rec.Code != http.StatusOK {
		t.Errorf("with the token from the internet: %d %s", rec.Code, rec.Body.String())
	}
}
