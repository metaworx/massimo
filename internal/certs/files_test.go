// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package certs

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writePair writes a self-signed ECDSA P-256 certificate for the names
// into dir and returns the paths and the certificate's fingerprint. The
// files' time is set ahead of the previous pair's, so that a rewrite
// within the same second still shows as a change.
func writePair(t *testing.T, dir string, names []string, notAfter time.Time, mod time.Time) (certPath, keyPath, fingerprint string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: names[0]},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, n)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{certPath, keyPath} {
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	return certPath, keyPath, Fingerprint(der)
}

func TestFingerprint(t *testing.T) {
	fp := Fingerprint([]byte("x"))
	if !strings.HasPrefix(fp, "SHA256:") || len(fp) != len("SHA256:")+32*3-1 || strings.Count(fp, ":") != 32 {
		t.Fatalf("fingerprint %q", fp)
	}
	if strings.ToUpper(fp) != fp {
		t.Fatal("not upper case")
	}
}

func TestFilesServesAndReloads(t *testing.T) {
	dir := t.TempDir()
	expires := time.Now().Add(400 * 24 * time.Hour).Truncate(time.Second)
	base := time.Now().Add(-time.Minute)
	certPath, keyPath, fpA := writePair(t, dir, []string{"massimo.example.net", "192.0.2.10"}, expires, base)
	var buf bytes.Buffer
	f, err := OpenFiles(certPath, keyPath, slog.New(slog.NewTextHandler(&buf, nil)))
	if err != nil {
		t.Fatal(err)
	}
	cert, err := f.GetCertificate(nil)
	if err != nil || Fingerprint(cert.Certificate[0]) != fpA {
		t.Fatalf("served %v %v", err, cert)
	}
	st := f.Status()
	if st.Source != "files" || strings.Join(st.Names, ",") != "massimo.example.net,192.0.2.10" || !st.SelfSigned || st.Fingerprint != fpA || !st.NotAfter.Equal(expires) {
		t.Fatalf("status %+v", st)
	}
	cfg := Config(f)
	if cfg.MinVersion != tls.VersionTLS12 || strings.Join(cfg.NextProtos, ",") != "h2,http/1.1" || cfg.GetCertificate == nil {
		t.Fatalf("config %+v", cfg)
	}

	// A renewal: the files rewritten are served at the next handshake.
	_, _, fpB := writePair(t, dir, []string{"massimo.example.net"}, expires.Add(24*time.Hour), base.Add(2*time.Second))
	cert, _ = f.GetCertificate(nil)
	if Fingerprint(cert.Certificate[0]) != fpB {
		t.Fatal("rewritten pair not served")
	}
	if !strings.Contains(buf.String(), "certificate reloaded") || !strings.Contains(buf.String(), fpB) {
		t.Fatalf("reload not logged:\n%s", buf.String())
	}

	// A bad rewrite: the loaded pair stays, said once.
	buf.Reset()
	os.WriteFile(certPath, []byte("not a certificate\n"), 0o600)
	os.Chtimes(certPath, base.Add(4*time.Second), base.Add(4*time.Second))
	for i := 0; i < 3; i++ {
		cert, _ = f.GetCertificate(nil)
		if Fingerprint(cert.Certificate[0]) != fpB {
			t.Fatal("bad pair served")
		}
	}
	if n := strings.Count(buf.String(), "do not load"); n != 1 {
		t.Fatalf("warned %d times:\n%s", n, buf.String())
	}
	if f.Status().Fingerprint != fpB {
		t.Fatal("status changed on a bad pair")
	}

	// Good again.
	_, _, fpC := writePair(t, dir, []string{"massimo.example.net"}, expires, base.Add(6*time.Second))
	cert, _ = f.GetCertificate(nil)
	if Fingerprint(cert.Certificate[0]) != fpC {
		t.Fatal("repaired pair not served")
	}
}

func TestFilesRefusesMissingOrBad(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath, _ := writePair(t, dir, []string{"localhost"}, time.Now().Add(time.Hour), time.Now())
	if _, err := OpenFiles(certPath, filepath.Join(dir, "missing.pem"), nil); err == nil {
		t.Fatal("missing key accepted")
	}
	os.WriteFile(keyPath, []byte("garbage"), 0o600)
	if _, err := OpenFiles(certPath, keyPath, nil); err == nil {
		t.Fatal("bad key accepted")
	}
}
