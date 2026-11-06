// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/certs"
	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/server"
)

// testPair writes a self-signed certificate for localhost into dir.
func testPair(t *testing.T, dir string) (certPath, keyPath, fingerprint string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	certPath, keyPath = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600)
	return certPath, keyPath, certs.Fingerprint(der)
}

func TestTLSSource(t *testing.T) {
	certPath, keyPath, _ := testPair(t, t.TempDir())
	if src, err := tlsSource(tlsOptions{}, nil); err != nil || src != nil {
		t.Fatalf("off: %v %v", src, err)
	}
	if src, err := tlsSource(tlsOptions{mode: "off", certPath: certPath, keyPath: keyPath}, nil); err != nil || src != nil {
		t.Fatalf("off with files: %v %v", src, err)
	}
	if src, err := tlsSource(tlsOptions{certPath: certPath, keyPath: keyPath}, quietLog()); err != nil || src == nil || src.Status().Source != "files" {
		t.Fatalf("files implied: %v %v", src, err)
	}
	data := t.TempDir()
	src, err := tlsSource(tlsOptions{mode: "self-signed", names: "massimo.example.net", listen: "192.0.2.10:8443", dataDir: data}, quietLog())
	if err != nil || src == nil {
		t.Fatalf("self-signed: %v %v", src, err)
	}
	if st := src.Status(); st.Source != "self-signed" || !st.SelfSigned || !slices.Contains(st.Names, "192.0.2.10") || !slices.Contains(st.Names, "massimo.example.net") || !slices.Contains(st.Names, "localhost") {
		t.Fatalf("self-signed status %+v", st)
	}
	if _, err := os.Stat(filepath.Join(data, "tls", "self-signed.key")); err != nil {
		t.Fatal("no pair in the data directory")
	}
	acme, err := tlsSource(tlsOptions{mode: "acme", names: "massimo.example.net", dataDir: t.TempDir(), directory: "http://127.0.0.1:1/directory", email: "owner@example.net"}, quietLog())
	if err != nil || acme.Status().Source != "acme" || !strings.Contains(acme.Status().Warning, "no certificate yet") {
		t.Fatalf("acme: %v %+v", err, acme)
	}
	for _, c := range []tlsOptions{{mode: "files"}, {mode: "files", certPath: certPath}, {certPath: certPath}, {mode: "acme"}, {mode: "acme", names: "192.0.2.10"}, {mode: "yes"}, {mode: "files", certPath: certPath, keyPath: filepath.Join(t.TempDir(), "none.pem")}, {mode: "files", certPath: certPath, keyPath: keyPath, httpAddr: ":0"}} {
		if _, err := tlsSource(c, quietLog()); err == nil {
			t.Errorf("%+v accepted", c)
		}
	}
	if listenHost(":8099") != "" || listenHost("192.0.2.1:80") != "192.0.2.1" || listenHost("[::1]:80") != "::1" {
		t.Fatal("listenHost")
	}
	for v, want := range map[string]int{"": 365, "0": 0, "30": 30} {
		if n, err := hstsDays(v); err != nil || n != want {
			t.Errorf("hsts %q: %d %v", v, n, err)
		}
	}
	for _, bad := range []string{"-1", "x", "1.5"} {
		if _, err := hstsDays(bad); err == nil {
			t.Errorf("hsts %q accepted", bad)
		}
	}
}

// TestServeTLS serves the API over the listener serve builds and reads
// /healthz over https.
func TestServeTLS(t *testing.T) {
	certPath, keyPath, fp := testPair(t, t.TempDir())
	src, err := tlsSource(tlsOptions{mode: "files", certPath: certPath, keyPath: keyPath}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	srv := server.New(server.Options{Logger: quietLog(), Version: "massimo v9.9.9", TLS: src, HSTSDays: 7})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	hs := &http.Server{Handler: srv.Handler(), TLSConfig: certs.Config(src)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveHTTP(ctx, hs, ln) }()

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, Timeout: 5 * time.Second}
	resp, err := client.Get("https://" + ln.Addr().String() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var h struct {
		OK  bool `json:"ok"`
		TLS struct {
			Source      string `json:"source"`
			Fingerprint string `json:"fingerprint"`
		} `json:"tls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil || !h.OK || h.TLS.Source != "files" || h.TLS.Fingerprint != fp {
		t.Fatalf("healthz over TLS: %v %+v", err, h)
	}
	if got := resp.Header.Get("Strict-Transport-Security"); got != "max-age=604800" {
		t.Fatalf("HSTS %q", got)
	}
	if resp.TLS == nil || resp.TLS.Version < tls.VersionTLS12 || !strings.HasPrefix(resp.Proto, "HTTP/") {
		t.Fatalf("not TLS: %v %s", resp.TLS, resp.Proto)
	}
	// Plain HTTP to the TLS port is refused, not served.
	if presp, err := http.Get("http://" + ln.Addr().String() + "/healthz"); err == nil {
		presp.Body.Close()
		if presp.StatusCode != http.StatusBadRequest {
			t.Fatalf("plain request answered %d", presp.StatusCode)
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServeTLSVariablesAreKnown(t *testing.T) {
	for _, k := range []string{tlsVar, tlsCertVar, tlsKeyVar, tlsNamesVar, hstsVar, acmeEmailVar, acmeDirectoryVar, tlsHTTPVar} {
		if !env.Known(k) {
			t.Errorf("%s unknown to massimo config", k)
		}
	}
}

func TestConfigPrintsTLS(t *testing.T) {
	certPath, keyPath, fp := testPair(t, t.TempDir())
	t.Setenv(tlsVar, "")
	t.Setenv(tlsCertVar, "")
	t.Setenv(tlsKeyVar, "")
	t.Setenv(hstsVar, "")
	var out strings.Builder
	printTLS(&out)
	if out.String() != "tls:           off\n" {
		t.Fatalf("off: %q", out.String())
	}
	t.Setenv(tlsCertVar, certPath)
	t.Setenv(tlsKeyVar, keyPath)
	t.Setenv(hstsVar, "0")
	out.Reset()
	printTLS(&out)
	for _, want := range []string{"tls:           files (cert " + certPath, "hsts 0 days", "names localhost; expires ", fp, "self-signed"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
}
