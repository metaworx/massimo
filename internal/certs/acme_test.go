// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package certs

import (
	"bytes"
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"go.metaworx.ch/massimo/internal/testkit"
)

// CheckACMENames refuses what OpenACME refuses of its names, in the same
// words, and takes DNS names.
func TestCheckACMENames(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		names []string
		want  string
	}{
		{nil, "tls: -tls acme needs the names of -tls-names"},
		{[]string{"massimo.example.net", "192.0.2.10"}, "tls: Let's Encrypt issues no certificate for an address (192.0.2.10); use a DNS name"},
		{[]string{"*.example.net"}, "tls: a wildcard (*.example.net) needs the DNS-01 challenge (-acme-dns-hook)"},
		{[]string{"pi"}, `tls: "pi" is not a public DNS name`},
	} {
		if err := CheckACMENames(c.names); err == nil || err.Error() != c.want {
			t.Errorf("%q: %v, want %s", c.names, err, c.want)
		}
		if _, err := OpenACME(t.TempDir(), c.names, "", "", nil); err == nil || err.Error() != c.want {
			t.Errorf("OpenACME %q: %v, want %s", c.names, err, c.want)
		}
	}
	if err := CheckACMENames([]string{"massimo.example.net", "example.net"}); err != nil {
		t.Fatalf("DNS names: %v", err)
	}
}

func TestOpenACME(t *testing.T) {
	for _, names := range [][]string{nil, {"192.0.2.10"}, {"*.example.net"}, {"pi"}} {
		if _, err := OpenACME(t.TempDir(), names, "", "", nil); err == nil {
			t.Errorf("%q accepted", names)
		}
	}
	// A directory that answers nothing: the order fails at once, the
	// failure is logged once in a while, the status says there is no
	// certificate yet.
	dir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	defer dir.Close()
	data := t.TempDir()
	log, logged := testkit.LogWatch(nil)
	a, err := OpenACME(data, []string{"massimo.example.net"}, "owner@example.net", dir.URL+"/directory", log)
	if err != nil {
		t.Fatal(err)
	}
	if st := a.Status(); st.Source != "acme" || st.Names[0] != "massimo.example.net" || !strings.Contains(st.Warning, "no certificate yet") {
		t.Fatalf("status %+v", st)
	}
	cfg := Config(a)
	if strings.Join(cfg.NextProtos, ",") != "h2,http/1.1,"+acme.ALPNProto {
		t.Fatalf("protos %v", cfg.NextProtos)
	}
	a.Check(time.Now())
	a.Check(time.Now())
	if n := logged.Count(0, testkit.Msg("tls: no certificate from the ACME directory")); n != 1 {
		t.Fatalf("logged %d times: %v", n, logged.Since(0, nil))
	}
	if _, err := a.GetCertificate(&tls.ClientHelloInfo{ServerName: "other.example.net"}); err == nil {
		t.Fatal("a name outside the whitelist served")
	}
	if fi, err := os.Stat(filepath.Join(data, "tls", "acme")); err != nil || !fi.IsDir() {
		t.Fatal("no cache directory")
	}

	// The helper handler: the challenge path for an unknown token, a
	// redirect for everything else, 400 for a POST.
	h := a.HTTPHandler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "http://massimo.example.net/.well-known/acme-challenge/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown token: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "http://massimo.example.net/ui/app.js?v=1", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://massimo.example.net/ui/app.js?v=1" {
		t.Fatalf("redirect: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "http://massimo.example.net/api/v1/cubes", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST: %d", rec.Code)
	}

	// A certificate served is remembered, with the warning near its end.
	certPath, keyPath, fp := writePair(t, t.TempDir(), []string{"massimo.example.net"}, time.Now().Add(10*24*time.Hour), time.Now())
	cert, _, err := Load("acme", certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	a.remember(cert, time.Now())
	if st := a.Status(); st.Fingerprint != fp || st.Warning != "the certificate expires in 9 days" && st.Warning != "the certificate expires in 10 days" {
		t.Fatalf("remembered %+v", st)
	}

	// The cache of a name reads back as autocert writes it: the key, then
	// the chain.
	keyPEM, _ := os.ReadFile(keyPath)
	certPEM, _ := os.ReadFile(certPath)
	os.WriteFile(filepath.Join(data, "tls", "acme", "massimo.example.net"), append(keyPEM, certPEM...), 0o600)
	st, err := LoadACMECache(data, "massimo.example.net")
	if err != nil || st.Fingerprint != fp || st.Source != "acme" {
		t.Fatalf("cache: %v %+v", err, st)
	}
	if _, err := LoadACMECache(data, "none.example.net"); err == nil {
		t.Fatal("missing cache read")
	}
}

func TestFilesWarnBeforeExpiry(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	certPath, keyPath, _ := writePair(t, dir, []string{"localhost"}, now.Add(100*24*time.Hour), now)
	log, logged := testkit.LogWatch(nil)
	f, err := OpenFiles(certPath, keyPath, log)
	if err != nil {
		t.Fatal(err)
	}
	f.Check(now)
	if f.Status().Warning != "" || logged.Count(0, nil) != 0 {
		t.Fatalf("warned early: %+v %v", f.Status(), logged.Since(0, nil))
	}
	m := logged.Mark()
	f.Check(now.Add(90 * 24 * time.Hour))
	if w := f.Status().Warning; w != "the certificate expires in 10 days" && w != "the certificate expires in 9 days" ||
		logged.Count(m, testkit.Contains("expires in")) == 0 {
		t.Fatalf("not warned: %q %v", w, logged.Since(m, nil))
	}
	f.Check(now.Add(101 * 24 * time.Hour))
	if f.Status().Warning != "the certificate has expired" {
		t.Fatalf("expired: %q", f.Status().Warning)
	}
	// Watch checks at once, at the time it runs: the warning of the
	// check after the end goes, long before Watch's first tick.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	Watch(ctx, f, time.Hour)
	if w := f.Status().Warning; w != "" {
		t.Fatalf("Watch did not check at once: %q", w)
	}
}

// cachedACME is an ACME source whose cache holds an ECDSA certificate for
// the name, as autocert writes it, and whose directory answers nothing,
// so that every certificate it serves comes from the cache.
func cachedACME(t *testing.T, name string) (a *ACME, fingerprint string, logged *testkit.Watch[testkit.Record]) {
	t.Helper()
	dir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	t.Cleanup(dir.Close)
	data := t.TempDir()
	log, logged := testkit.LogWatch(nil)
	a, err := OpenACME(data, []string{name}, "", dir.URL+"/directory", log)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath, fp := writePair(t, t.TempDir(), []string{name}, time.Now().Add(80*24*time.Hour), time.Now())
	keyPEM, _ := os.ReadFile(keyPath)
	certPEM, _ := os.ReadFile(certPath)
	os.MkdirAll(filepath.Join(data, "tls", "acme"), 0o700)
	if err := os.WriteFile(filepath.Join(data, "tls", "acme", name), append(keyPEM, certPEM...), 0o600); err != nil {
		t.Fatal(err)
	}
	return a, fp, logged
}

// A handshake without a server name, as Go makes to an address (the
// update helper's https://127.0.0.1 health check, a browser at
// https://<ip>), gets the certificate of the first name.
func TestACMEWithoutServerName(t *testing.T) {
	a, fp, logged := cachedACME(t, "massimo.example.net")
	ln, err := tls.Listen("tcp", "127.0.0.1:0", Config(a))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { c.(*tls.Conn).Handshake(); c.Close() }()
		}
	}()
	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("a handshake without SNI: %v %v", err, logged.Since(0, nil))
	}
	defer conn.Close()
	if got := Fingerprint(conn.ConnectionState().PeerCertificates[0].Raw); got != fp {
		t.Fatalf("served %s, want the first name's %s", got, fp)
	}
}

// The hourly check asks for the certificate a browser gets, the ECDSA
// one under the name's key: with that cached it orders nothing, and the
// status shows its fingerprint.
func TestACMECheckAsksAsABrowser(t *testing.T) {
	a, fp, logged := cachedACME(t, "massimo.example.net")
	a.Check(time.Now())
	if st := a.Status(); st.Fingerprint != fp || st.Warning != "" {
		t.Fatalf("status %+v, want the cached %s %v", st, fp, logged.Since(0, nil))
	}
	logged.HasNot(t, 0, "another certificate ordered by the check", testkit.Msg("tls: no certificate from the ACME directory"))
}

// A certificate autocert stores is a new one, and goes into the log with
// its name and fingerprint; the account key, a token and a challenge's
// certificate do not, and a certificate an earlier run left in the cache
// is served without the line.
func TestACMELogsIssued(t *testing.T) {
	log, logged := testkit.LogWatch(nil)
	c := logCache{Cache: autocert.DirCache(t.TempDir()), log: log}
	certPath, keyPath, fp := writePair(t, t.TempDir(), []string{"massimo.example.net"}, time.Now().Add(80*24*time.Hour), time.Now())
	keyPEM, _ := os.ReadFile(keyPath)
	certPEM, _ := os.ReadFile(certPath)
	data := append(append([]byte{}, keyPEM...), certPEM...) // as autocert stores it
	ctx := context.Background()
	for _, key := range []string{"acme_account+key", "tok3n+http-01", "massimo.example.net+token"} {
		if err := c.Put(ctx, key, data); err != nil {
			t.Fatal(err)
		}
	}
	logged.HasNot(t, 0, "a line for what is no certificate of the server's", nil)
	m := logged.Mark()
	if err := c.Put(ctx, "massimo.example.net", data); err != nil {
		t.Fatal(err)
	}
	if issued := logged.Since(m, testkit.Msg("tls: certificate issued by the ACME directory")); len(issued) != 1 ||
		issued[0].Attrs["name"] != "massimo.example.net" || issued[0].Attrs["fingerprint"] != fp {
		t.Fatalf("log %v, want one line with the fingerprint %s", logged.Since(m, nil), fp)
	}
	if got, err := c.Get(ctx, "massimo.example.net"); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("stored %d bytes, %v", len(got), err)
	}

	a, cachedFP, cached := cachedACME(t, "massimo.example.net")
	if _, ok := a.m.Cache.(logCache); !ok {
		t.Fatalf("the manager's cache is a %T", a.m.Cache)
	}
	cert, err := a.GetCertificate(browserHello("massimo.example.net"))
	if err != nil {
		t.Fatal(err)
	}
	if got := Fingerprint(cert.Certificate[0]); got != cachedFP {
		t.Fatalf("served %s, want the cached %s", got, cachedFP)
	}
	cached.HasNot(t, 0, "a cached certificate logged as issued", testkit.Contains("certificate issued"))
}
