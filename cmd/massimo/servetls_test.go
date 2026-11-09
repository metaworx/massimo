// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/certs"
	"go.metaworx.ch/massimo/internal/env"
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

// hstsDays takes a number of days, 0 or more; "" is a year.
func TestHSTSDays(t *testing.T) {
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

func TestServeTLSVariablesAreKnown(t *testing.T) {
	for _, k := range []string{tlsVar, tlsCertVar, tlsKeyVar, tlsNamesVar, hstsVar, acmeEmailVar, acmeDirectoryVar, tlsHTTPVar, acmeDNSHookVar, trustedProxiesVar} {
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
	if out.String() != "tls:           off (serve has no TLS)\n" {
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
