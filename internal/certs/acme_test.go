// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package certs

import (
	"bytes"
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/acme"
)

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
	var buf bytes.Buffer
	a, err := OpenACME(data, []string{"massimo.example.net"}, "owner@example.net", dir.URL+"/directory", slog.New(slog.NewTextHandler(&buf, nil)))
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
	if n := strings.Count(buf.String(), "no certificate from the ACME directory"); n != 1 {
		t.Fatalf("logged %d times:\n%s", n, buf.String())
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
	var buf bytes.Buffer
	f, err := OpenFiles(certPath, keyPath, slog.New(slog.NewTextHandler(&buf, nil)))
	if err != nil {
		t.Fatal(err)
	}
	f.Check(now)
	if f.Status().Warning != "" || buf.Len() != 0 {
		t.Fatalf("warned early: %+v\n%s", f.Status(), buf.String())
	}
	f.Check(now.Add(90 * 24 * time.Hour))
	if w := f.Status().Warning; w != "the certificate expires in 10 days" && w != "the certificate expires in 9 days" || !strings.Contains(buf.String(), "expires in") {
		t.Fatalf("not warned: %q\n%s", w, buf.String())
	}
	f.Check(now.Add(101 * 24 * time.Hour))
	if f.Status().Warning != "the certificate has expired" {
		t.Fatalf("expired: %q", f.Status().Warning)
	}
	// Watch checks at once.
	buf.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	Watch(ctx, f, time.Hour)
}

// cachedACME is an ACME source whose cache holds an ECDSA certificate for
// the name, as autocert writes it, and whose directory answers nothing,
// so that every certificate it serves comes from the cache.
func cachedACME(t *testing.T, name string) (a *ACME, fingerprint string, logs *bytes.Buffer) {
	t.Helper()
	dir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	t.Cleanup(dir.Close)
	data := t.TempDir()
	logs = &bytes.Buffer{}
	a, err := OpenACME(data, []string{name}, "", dir.URL+"/directory", slog.New(slog.NewTextHandler(logs, nil)))
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
	return a, fp, logs
}

// A handshake without a server name, as Go makes to an address (the
// update helper's https://127.0.0.1 health check, a browser at
// https://<ip>), gets the certificate of the first name.
func TestACMEWithoutServerName(t *testing.T) {
	a, fp, logs := cachedACME(t, "massimo.example.net")
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
		t.Fatalf("a handshake without SNI: %v\n%s", err, logs.String())
	}
	defer conn.Close()
	if got := Fingerprint(conn.ConnectionState().PeerCertificates[0].Raw); got != fp {
		t.Fatalf("served %s, want the first name's %s", got, fp)
	}
}
