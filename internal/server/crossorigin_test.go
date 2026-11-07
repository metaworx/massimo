// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sendWith sends a request with the given headers and nothing else, as a
// browser's fetch from another page would: a plain-text body needs no
// preflight.
func sendWith(t *testing.T, ts *httptest.Server, method, path, body string, headers map[string]string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// A write another site's page sends is refused before it is routed, on
// an open server, where it would otherwise create the first owner, and
// with users; reads, the page's own writes and clients that are not
// browsers pass.
func TestCrossOriginWritesRefused(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), ""))
	st := s.opts.Store
	owner := `{"name":"intruder","role":"owner","password":"placeholder-pass-1234"}`
	plain := "text/plain;charset=UTF-8"
	for name, h := range map[string]map[string]string{
		"cross-site":           {"Content-Type": plain, "Sec-Fetch-Site": "cross-site", "Origin": "https://attacker.example"},
		"same-site":            {"Content-Type": plain, "Sec-Fetch-Site": "same-site", "Origin": "http://other.localhost"},
		"origin of other host": {"Content-Type": plain, "Origin": "https://attacker.example"},
	} {
		code, text := sendWith(t, ts, "POST", "/api/v1/users", owner, h)
		if code != http.StatusForbidden || !strings.Contains(text, "cross-origin request refused") {
			t.Errorf("%s: %d %s", name, code, text)
		}
	}
	if n, err := st.CountUsers(t.Context()); err != nil || n != 0 {
		t.Fatalf("users after the refused requests: %d %v", n, err)
	}
	s.refusals.mu.Lock()
	noted := len(s.refusals.open)
	s.refusals.mu.Unlock()
	if noted != 1 {
		t.Errorf("refusals noted for %d addresses, want 1", noted)
	}
	if code, text := sendWith(t, ts, "GET", "/api/v1/cubes", "", map[string]string{"Sec-Fetch-Site": "cross-site"}); code != http.StatusOK {
		t.Errorf("a cross-site read: %d %s", code, text)
	}
	// Past the protection, the users route answers the empty body itself.
	for name, h := range map[string]map[string]string{
		"same-origin":    {"Sec-Fetch-Site": "same-origin", "Origin": ts.URL},
		"typed URL":      {"Sec-Fetch-Site": "none"},
		"own origin":     {"Origin": ts.URL},
		"not a browser":  {},
		"curl with JSON": {"Content-Type": "application/json"},
	} {
		if code, text := sendWith(t, ts, "POST", "/api/v1/users", "{}", h); code != http.StatusBadRequest {
			t.Errorf("%s: %d %s, want the route's 400", name, code, text)
		}
	}

	// With users, a cross-site write is refused even with the right key.
	_, ts, _ = startWith(t, storeOptions(t, t.TempDir(), "secret"))
	body := `{"name":"anna","role":"operator","password":"pw"}`
	if code, text := sendWith(t, ts, "POST", "/api/v1/users", body, map[string]string{"Authorization": "Bearer secret", "Sec-Fetch-Site": "cross-site"}); code != http.StatusForbidden {
		t.Errorf("users mode, cross-site: %d %s", code, text)
	}
	if code, text := sendWith(t, ts, "POST", "/api/v1/users", body, map[string]string{"Authorization": "Bearer secret", "Sec-Fetch-Site": "same-origin"}); code != http.StatusCreated {
		t.Errorf("users mode, same-origin: %d %s", code, text)
	}
}
