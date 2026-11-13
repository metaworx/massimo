// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
)

// GET /api/v1/system: the version and start /healthz has, how the server
// was installed, the computer, and the caller's address, the client a
// trusted proxy names.
func TestGetSystem(t *testing.T) {
	proxies, _ := ParseTrustedProxies("127.0.0.1,::1")
	_, ts, _ := startWith(t, Options{Token: "secret", Version: "v0.0.0-test (go1.26.0)", Install: "apt", TrustedProxies: proxies})
	get := func(forwarded string) systemView {
		t.Helper()
		req, _ := http.NewRequest("GET", ts.URL+"/api/v1/system", nil)
		req.Header.Set("Authorization", "Bearer secret")
		if forwarded != "" {
			req.Header.Set("X-Forwarded-For", forwarded)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status %d", resp.StatusCode)
		}
		var v systemView
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	v := get("")
	host, _ := os.Hostname()
	if v.Version != "v0.0.0-test (go1.26.0)" || v.StartedAt.IsZero() || v.Install != "apt" || v.OS != goruntime.GOOS ||
		v.Arch != goruntime.GOARCH || v.Hostname != host || v.ClientIP != "127.0.0.1" || v.TLS != nil {
		t.Fatalf("system %+v", v)
	}
	if v := get("192.0.2.10"); v.ClientIP != "192.0.2.10" {
		t.Fatalf("behind a trusted proxy: client %q", v.ClientIP)
	}
	resp, err := http.Get(ts.URL + "/api/v1/system")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("without a token: %d", resp.StatusCode)
	}
}

// A server that does not say how it was installed is "other".
func TestGetSystemInstallOther(t *testing.T) {
	_, ts, _ := startWith(t, Options{Token: "secret"})
	var v systemView
	if code := getJSON(t, ts, "secret", "/api/v1/system", &v); code != 200 || v.Install != InstallOther {
		t.Fatalf("%d %+v", code, v)
	}
}

// The distribution's name comes from os-release's PRETTY_NAME, quoted
// or not; none without the file.
func TestOSName(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct{ body, want string }{
		{"NAME=\"Debian GNU/Linux\"\nPRETTY_NAME=\"Debian GNU/Linux 13 (trixie)\"\nID=debian\n", "Debian GNU/Linux 13 (trixie)"},
		{"PRETTY_NAME=Alpine\n", "Alpine"},
		{"ID=nothing\n", ""},
	} {
		p := filepath.Join(dir, "os-release")
		if err := os.WriteFile(p, []byte(c.body), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := osName(p); got != c.want {
			t.Errorf("%q: %q, want %q", c.body, got, c.want)
		}
	}
	if got := osName(filepath.Join(dir, "none")); got != "" {
		t.Errorf("without the file: %q", got)
	}
}
