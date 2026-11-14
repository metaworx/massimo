// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// Where a proxy serves a request, in each of the common headers' forms.
func TestForwardedTarget(t *testing.T) {
	for _, c := range []struct {
		name    string
		headers map[string]string
		host    string // the request's Host
		proto   string
		target  string
		port    int
	}{
		{"X-Forwarded-Proto, -Host and -Port", map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Host": "massimo.example.net", "X-Forwarded-Port": "8443"}, "127.0.0.1:8099", "https", "massimo.example.net", 8443},
		{"RFC 7239 Forwarded", map[string]string{"Forwarded": `for=192.0.2.60;proto=https;host="massimo.example.net:8099", for=192.0.2.61`}, "127.0.0.1:8099", "https", "massimo.example.net", 8099},
		{"X-Forwarded-Ssl on, the Host kept", map[string]string{"X-Forwarded-Ssl": "on"}, "massimo.example.net", "https", "massimo.example.net", 443},
		{"Front-End-Https on", map[string]string{"Front-End-Https": "on", "X-Forwarded-Host": "massimo.example.net"}, "", "https", "massimo.example.net", 443},
		{"Cloudflare's CF-Visitor", map[string]string{"CF-Visitor": `{"scheme":"https"}`}, "massimo.example.net", "https", "massimo.example.net", 443},
		{"X-Forwarded-Scheme", map[string]string{"X-Forwarded-Scheme": "https"}, "massimo.example.net", "https", "massimo.example.net", 443},
		{"X-Forwarded-Protocol", map[string]string{"X-Forwarded-Protocol": "https"}, "massimo.example.net", "https", "massimo.example.net", 443},
		{"X-Url-Scheme", map[string]string{"X-Url-Scheme": "https"}, "massimo.example.net", "https", "massimo.example.net", 443},
		{"X-Scheme", map[string]string{"X-Scheme": "HTTPS"}, "massimo.example.net", "https", "massimo.example.net", 443},
		{"a list: the first proxy's", map[string]string{"X-Forwarded-Proto": "https, http", "X-Forwarded-Host": "massimo.example.net, 10.0.0.2"}, "", "https", "massimo.example.net", 443},
		{"plain HTTP at an address and a port", map[string]string{"X-Forwarded-Proto": "http"}, "192.0.2.10:8099", "http", "192.0.2.10", 8099},
		{"an IPv6 address", map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Host": "[2001:db8::1]:8443"}, "", "https", "2001:db8::1", 8443},
		{"nothing said", map[string]string{"X-Forwarded-For": "192.0.2.60"}, "massimo.example.net:8099", "", "massimo.example.net", 8099},
		{"no scheme, no port: the port unknown", map[string]string{"X-Forwarded-For": "192.0.2.60"}, "massimo.example.net", "", "massimo.example.net", 0},
		{"Forwarded wins", map[string]string{"Forwarded": "proto=http", "X-Forwarded-Proto": "https"}, "massimo.example.net", "http", "massimo.example.net", 80},
	} {
		h := http.Header{}
		for k, v := range c.headers {
			h.Set(k, v)
		}
		proto, host, port := forwardedTarget(h, c.host)
		if proto != c.proto || host != c.target || port != c.port {
			t.Errorf("%s: %q %q %d, want %q %q %d", c.name, proto, host, port, c.proto, c.target, c.port)
		}
		if !proxyHeaders(h) {
			t.Errorf("%s: not taken for a proxy's request", c.name)
		}
	}
	if proxyHeaders(http.Header{"Accept": {"*/*"}}) {
		t.Error("a request without a proxy's header taken for one")
	}
}

// The proxy's certificate, read through a handshake with the proxy: any
// certificate, the one the handshake refuses too, and whether it is valid
// for the host, a name or an address, against the roots given.
func TestReadProxyCert(t *testing.T) {
	ts := httptest.NewTLSServer(http.NotFoundHandler())
	defer ts.Close()
	addr := ts.Listener.Addr().String()
	got := readProxyCert(context.Background(), addr, "example.com", x509.NewCertPool())
	if got.Error != "" || got.Trusted || got.NotAfter.IsZero() || got.Fingerprint == "" || got.Source != "proxy" {
		t.Fatalf("against no roots: %+v", got)
	}
	found := false
	for _, n := range got.Names {
		found = found || n == "example.com"
	}
	if !found {
		t.Fatalf("names %v", got.Names)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ts.Certificate())
	if got := readProxyCert(context.Background(), addr, "example.com", roots); !got.Trusted {
		t.Fatalf("against its own root: %+v", got)
	}
	if got := readProxyCert(context.Background(), addr, "other.example.org", roots); got.Trusted || got.Error != "" || len(got.Names) == 0 {
		t.Fatalf("for a name it does not carry, read all the same: %+v", got)
	}
	if got := readProxyCert(context.Background(), addr, "127.0.0.1", roots); !got.Trusted {
		t.Fatalf("for the address it carries: %+v", got)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := ln.Addr().String()
	ln.Close()
	if got := readProxyCert(context.Background(), closed, "example.com", nil); got.Error == "" || got.CheckedAt.IsZero() {
		t.Fatalf("a closed port: %+v", got)
	}
}

// GET /api/v1/system through a trusted proxy that serves HTTPS: the proxy's
// address at once, its certificate once the check in the background has
// read it; through no trusted proxy, nothing of the headers.
func TestSystemBehindAProxy(t *testing.T) {
	proxy := httptest.NewTLSServer(http.NotFoundHandler())
	defer proxy.Close()
	_, port, _ := net.SplitHostPort(proxy.Listener.Addr().String())
	trusted, _ := ParseTrustedProxies("127.0.0.1,::1")
	_, ts, _ := startWith(t, Options{Token: "secret", TrustedProxies: trusted})
	get := func(ts *httptest.Server) systemView {
		t.Helper()
		req, _ := http.NewRequest("GET", ts.URL+"/api/v1/system", nil)
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set("X-Forwarded-For", "192.0.2.20")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", "example.com")
		req.Header.Set("X-Forwarded-Port", port)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var v systemView
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	v := get(ts)
	if v.Proxy == nil || v.Proxy.Proto != "https" || v.Proxy.Host != "example.com" || strconv.Itoa(v.Proxy.Port) != port || v.ClientIP != "192.0.2.20" {
		t.Fatalf("proxy %+v, client %q", v.Proxy, v.ClientIP)
	}
	deadline := time.Now().Add(5 * time.Second)
	for v.Proxy.Certificate == nil && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		v = get(ts)
	}
	c := v.Proxy.Certificate
	if c == nil || c.Error != "" || c.Trusted || c.CheckedAt.IsZero() || len(c.Names) == 0 {
		t.Fatalf("the proxy's certificate %+v", c)
	}
	_, untrusting, _ := startWith(t, Options{Token: "secret"})
	if v := get(untrusting); v.Proxy != nil {
		t.Fatalf("headers of an untrusted peer taken: %+v", v.Proxy)
	}
}

// The address a trusted proxy serves the server at, from an authorized
// request, is kept for the announcement, the scheme's own port left out;
// an anonymous request, an untrusted peer or a request without a scheme
// changes nothing.
func TestProxiedURL(t *testing.T) {
	trusted, _ := ParseTrustedProxies("127.0.0.1,::1")
	_, ts, s, st := startStored(t, Options{TrustedProxies: trusted})
	owner := userToken(t, st, "boss", store.RoleOwner)
	call := func(path, token string, h map[string]string) {
		t.Helper()
		req, _ := http.NewRequest("GET", ts.URL+path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		for k, v := range h {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	https := map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Host": "massimo.example.net", "X-Forwarded-Port": "443"}
	call("/api/v1/integrations/requests/x", "", https) // anonymous
	if got := s.ProxiedURL(); got != "" {
		t.Fatalf("taken from an anonymous request: %q", got)
	}
	call("/api/v1/me", owner, map[string]string{"X-Forwarded-For": "192.0.2.20"}) // no scheme
	call("/api/v1/me", owner, https)
	if got := s.ProxiedURL(); got != "https://massimo.example.net" {
		t.Fatalf("proxied %q", got)
	}
	if v, _ := st.SettingString(t.Context(), store.ProxiedURLKey); v != "https://massimo.example.net" {
		t.Fatalf("stored %q", v)
	}
	call("/api/v1/me", owner, map[string]string{"X-Forwarded-Proto": "http", "X-Forwarded-Host": "massimo.example.net:8080"})
	if got := s.ProxiedURL(); got != "http://massimo.example.net:8080" {
		t.Fatalf("another port %q", got)
	}
	// A new server reads it from the store.
	fresh := New(Options{Store: st})
	if got := fresh.ProxiedURL(); got != "http://massimo.example.net:8080" {
		t.Fatalf("after a restart %q", got)
	}
	for in, want := range map[[3]string]string{
		{"https", "h", "443"}: "https://h", {"http", "h", "80"}: "http://h", {"https", "h", "80"}: "https://h:80", {"https", "::1", "8443"}: "https://[::1]:8443",
	} {
		p, _ := strconv.Atoi(in[2])
		if got := publicURL(in[0], in[1], p); got != want {
			t.Errorf("publicURL%v = %q, want %q", in, got, want)
		}
	}
}
