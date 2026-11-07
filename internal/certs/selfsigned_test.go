// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package certs

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := SelfSignedPaths(dir)
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	st, err := Generate(certPath, keyPath, []string{"pi", "pi.local", "localhost", "127.0.0.1", "::1", "192.0.2.10", "massimo.example.net"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if st.Source != "self-signed" || !st.SelfSigned || strings.Join(st.Names, ",") != "pi,pi.local,localhost,massimo.example.net,127.0.0.1,::1,192.0.2.10" {
		t.Fatalf("status %+v", st)
	}
	if !st.NotAfter.Equal(now.Add(SelfSignedValidity)) || SelfSignedValidity != 825*24*time.Hour {
		t.Fatalf("validity %v", st.NotAfter)
	}
	cert, _, err := Load("self-signed", certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(cert.Certificate[0])
	if k, ok := leaf.PublicKey.(*ecdsa.PublicKey); !ok || k.Curve != elliptic.P256() {
		t.Fatalf("key %T", leaf.PublicKey)
	}
	if leaf.Subject.CommonName != "pi" || leaf.IsCA || !leaf.NotBefore.Before(now) {
		t.Fatalf("leaf %+v", leaf.Subject)
	}
	if runtime.GOOS != "windows" {
		for _, p := range []string{certPath, keyPath} {
			if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
				t.Errorf("%s mode %v", p, fi.Mode())
			}
		}
		if fi, _ := os.Stat(filepath.Dir(certPath)); fi.Mode().Perm() != 0o700 {
			t.Errorf("dir mode %v", fi.Mode())
		}
	}
	if _, err := Generate(certPath, keyPath, nil, now); err == nil {
		t.Fatal("no names accepted")
	}
}

func TestOpenSelfSigned(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	names := []string{"pi", "localhost", "127.0.0.1"}

	s, err := OpenSelfSigned(dir, names, log, clock)
	if err != nil {
		t.Fatal(err)
	}
	first := s.Status()
	if !first.SelfSigned || first.Source != "self-signed" || !strings.Contains(buf.String(), "no certificate yet") || !strings.Contains(buf.String(), first.Fingerprint) {
		t.Fatalf("first start: %+v\n%s", first, buf.String())
	}

	// A second start keeps the pair; a missing name is logged, not fixed.
	buf.Reset()
	s, err = OpenSelfSigned(dir, append(names, "192.0.2.20"), log, clock)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status().Fingerprint != first.Fingerprint || !strings.Contains(buf.String(), RegenerateCommand) || !strings.Contains(buf.String(), "192.0.2.20") {
		t.Fatalf("second start: %+v\n%s", s.Status(), buf.String())
	}

	// Within 30 days of the end: made anew at the start, and by Check.
	buf.Reset()
	now = first.NotAfter.Add(-20 * 24 * time.Hour)
	s, err = OpenSelfSigned(dir, names, log, clock)
	if err != nil {
		t.Fatal(err)
	}
	second := s.Status()
	if second.Fingerprint == first.Fingerprint || !strings.Contains(buf.String(), "expires") {
		t.Fatalf("not renewed at the start:\n%s", buf.String())
	}
	buf.Reset()
	s.Check(now)
	if s.Status().Fingerprint != second.Fingerprint || buf.Len() != 0 {
		t.Fatalf("renewed too early:\n%s", buf.String())
	}
	s.Check(second.NotAfter.Add(-RenewBefore + time.Hour))
	third := s.Status()
	if third.Fingerprint == second.Fingerprint || !strings.Contains(buf.String(), "renewed") {
		t.Fatalf("Check did not renew:\n%s", buf.String())
	}
	if cert, _ := s.GetCertificate(nil); Fingerprint(cert.Certificate[0]) != third.Fingerprint {
		t.Fatal("the renewed pair is not served")
	}

	// A broken file is replaced.
	certPath, _ := SelfSignedPaths(dir)
	os.WriteFile(certPath, []byte("broken"), 0o600)
	s, err = OpenSelfSigned(dir, names, log, clock)
	if err != nil || s.Status().Fingerprint == third.Fingerprint || !s.Status().SelfSigned {
		t.Fatalf("broken file: %v %+v", err, s.Status())
	}

	// Watch calls Check on the ticker.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	Watch(ctx, s, 20*time.Millisecond)
	Watch(ctx, fakeSource{}, time.Millisecond) // not a Checker: returns at once
}

type fakeSource struct{}

func (fakeSource) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) { return nil, nil }
func (fakeSource) Status() Status                                                { return Status{} }

func TestNames(t *testing.T) {
	addrs := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("192.0.2.10"), net.ParseIP("fe80::1"), net.ParseIP("2001:db8::10"), net.ParseIP("::1")}
	got := namesWith("Pi.lan", addrs, "", []string{"Massimo.example.net", "192.0.2.10"})
	if strings.Join(got, ",") != "pi.lan,pi.local,localhost,127.0.0.1,::1,192.0.2.10,2001:db8::10,massimo.example.net" {
		t.Fatalf("%v", got)
	}
	got = namesWith("pi", nil, "192.0.2.5", nil)
	if strings.Join(got, ",") != "192.0.2.5,pi,pi.local,localhost,127.0.0.1,::1" {
		t.Fatalf("%v", got)
	}
	got = namesWith("localhost", nil, "0.0.0.0", nil)
	if strings.Join(got, ",") != "localhost,127.0.0.1,::1" {
		t.Fatalf("%v", got)
	}
	if n := ParseNames(" a.example.net, b.example.net 192.0.2.1;c"); strings.Join(n, "|") != "a.example.net|b.example.net|192.0.2.1|c" {
		t.Fatalf("%q", n)
	}
	if m := Missing([]string{"pi", "localhost"}, []string{"PI", "192.0.2.10", "localhost"}); strings.Join(m, ",") != "192.0.2.10" {
		t.Fatalf("%v", m)
	}
	if len(Names("", nil)) < 3 {
		t.Fatal("the box's own names")
	}
}

// The renewal keeps the names of the certificate it renews: a name added
// by hand (massimo tls self-signed -names) that the running server took
// up stays in the next certificate.
func TestSelfSignedRenewalKeepsNames(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := OpenSelfSigned(dir, []string{"pi", "localhost"}, log, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := SelfSignedPaths(dir)
	if _, err := Generate(certPath, keyPath, []string{"pi", "localhost", "nas.lan", "192.0.2.20"}, now); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	if !slices.Contains(s.Status().Names, "nas.lan") {
		t.Fatalf("the pair made by hand is not served: %v", s.Status().Names)
	}
	s.Check(s.Status().NotAfter.Add(-RenewBefore + time.Hour))
	if got := s.Status().Names; Missing(got, []string{"pi", "localhost", "nas.lan", "192.0.2.20"}) != nil {
		t.Fatalf("renewed for %v, names lost", got)
	}
}
