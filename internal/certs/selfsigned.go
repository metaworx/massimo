// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// A self-signed certificate the server makes for the box's names at its
// first start, into the data directory's tls/, the one place every
// service may write. It is served through the files source, so that
// "massimo tls self-signed" beside a running server takes effect at the
// next handshake, and it is made anew 30 days before it expires. Names
// are never changed on the server's own: a new name set is a new
// certificate and a new exception on every device, so a box whose
// address moved is told the command instead.
const (
	// SelfSignedDir is the directory under the data directory.
	SelfSignedDir  = "tls"
	selfSignedCert = "self-signed.pem"
	selfSignedKey  = "self-signed.key"
	// SelfSignedValidity is 825 days: Apple trusts no TLS server
	// certificate issued for longer, even one installed by hand. They
	// count from NotBefore, the hour it is dated back included.
	SelfSignedValidity = 825 * 24 * time.Hour
	// RenewBefore is how long before it expires the pair is made anew.
	RenewBefore = 30 * 24 * time.Hour
	// RegenerateCommand is what the log names when the certificate lacks
	// a name of the box.
	RegenerateCommand = "massimo tls self-signed"
)

// SelfSignedPaths are the files of the self-signed pair under the data
// directory.
func SelfSignedPaths(dataDir string) (certPath, keyPath string) {
	dir := filepath.Join(dataDir, SelfSignedDir)
	return filepath.Join(dir, selfSignedCert), filepath.Join(dir, selfSignedKey)
}

// SelfSigned serves the self-signed pair of a data directory and renews
// it (Check).
type SelfSigned struct {
	*Files
	names []string
	now   func() time.Time
}

// OpenSelfSigned serves the self-signed pair of dataDir for the names
// (Names gives the box's own), made when missing, unreadable or
// expiring within RenewBefore; an existing one that lacks a name is
// served as it is and the log names the command. now is for tests;
// nil is time.Now.
func OpenSelfSigned(dataDir string, names []string, log *slog.Logger, now func() time.Time) (*SelfSigned, error) {
	if log == nil {
		log = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	certPath, keyPath := SelfSignedPaths(dataDir)
	_, st, err := Load("self-signed", certPath, keyPath)
	reason := ""
	switch {
	case err != nil && os.IsNotExist(unwrapPath(err)):
		reason = "no certificate yet"
	case err != nil:
		reason = err.Error()
	case now().Add(RenewBefore).After(st.NotAfter):
		reason = "expires " + st.NotAfter.Format("2006-01-02")
	}
	if reason != "" {
		if st, err = Generate(certPath, keyPath, names, now()); err != nil {
			return nil, err
		}
		log.Info("tls: self-signed certificate made", "reason", reason, "cert", certPath, "names", st.Names, "expires", st.NotAfter, "fingerprint", st.Fingerprint)
	} else if missing := Missing(st.Names, names); len(missing) > 0 {
		log.Warn("tls: not in the self-signed certificate; a new one needs a new exception on every device, so make it yourself with: "+RegenerateCommand, "missing", missing)
	}
	f, err := openFiles("self-signed", certPath, keyPath, log)
	if err != nil {
		return nil, err
	}
	return &SelfSigned{Files: f, names: names, now: now}, nil
}

// unwrapPath finds the path error under a load error.
func unwrapPath(err error) error {
	for err != nil {
		if pe, ok := err.(*os.PathError); ok {
			return pe
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return err
		}
		err = u.Unwrap()
	}
	return err
}

// Check makes the pair anew when it expires within RenewBefore, with
// the names it was opened for and those of the certificate it renews (a
// name added by hand with massimo tls self-signed -names stays), and
// serves it from the next handshake.
func (s *SelfSigned) Check(now time.Time) {
	st := s.Status()
	if !now.Add(RenewBefore).After(st.NotAfter) {
		return
	}
	certPath, keyPath := s.Paths()
	names := append(slices.Clone(s.names), Missing(s.names, st.Names)...)
	st, err := Generate(certPath, keyPath, names, now)
	if err != nil {
		s.log.Error("tls: self-signed certificate not renewed", "error", err)
		return
	}
	s.Reload()
	s.log.Info("tls: self-signed certificate renewed", "names", st.Names, "expires", st.NotAfter, "fingerprint", st.Fingerprint)
}

// Generate writes a self-signed ECDSA P-256 pair for the names (DNS
// names and addresses; the first DNS name is the subject) valid from an
// hour before now, for a clock a little behind, for SelfSignedValidity
// in all, the files mode 0600 in a directory mode 0700, each written
// beside its place and renamed over.
func Generate(certPath, keyPath string, names []string, now time.Time) (Status, error) {
	if len(names) == 0 {
		return Status{}, fmt.Errorf("tls: a certificate needs at least one name")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Status{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return Status{}, err
	}
	notBefore := now.Add(-time.Hour)
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(SelfSignedValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, n)
		}
	}
	if len(tmpl.DNSNames) > 0 {
		tmpl.Subject = pkix.Name{CommonName: tmpl.DNSNames[0]}
	} else {
		tmpl.Subject = pkix.Name{CommonName: "massimo"}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return Status{}, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return Status{}, err
	}
	if err := os.MkdirAll(filepath.Dir(certPath), 0o700); err != nil {
		return Status{}, err
	}
	if err := writeAtomic(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})); err != nil {
		return Status{}, err
	}
	if err := writeAtomic(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})); err != nil {
		return Status{}, err
	}
	_, st, err := Load("self-signed", certPath, keyPath)
	return st, err
}

// writeAtomic writes b to a temporary file beside path, mode 0600, and
// renames it over.
func writeAtomic(path string, b []byte) error {
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	os.Remove(tmp)
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Names are the names a self-signed certificate holds for this box: the
// listen host when it is a specific address, the hostname and its first
// label with .local, localhost and the loopback addresses, the box's
// own addresses (unicast, neither loopback nor link-local), and the
// extra names given (-tls-names), each once in that order.
func Names(listenHost string, extra []string) []string {
	host, _ := os.Hostname()
	var addrs []net.IP
	if ifaces, err := net.InterfaceAddrs(); err == nil {
		for _, a := range ifaces {
			if n, ok := a.(*net.IPNet); ok {
				addrs = append(addrs, n.IP)
			}
		}
	}
	return namesWith(host, addrs, listenHost, extra)
}

func namesWith(hostname string, addrs []net.IP, listenHost string, extra []string) []string {
	var names []string
	add := func(n string) {
		n = strings.ToLower(strings.TrimSpace(n))
		if n != "" && !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	if ip := net.ParseIP(listenHost); ip != nil && !ip.IsUnspecified() && !ip.IsLoopback() {
		add(ip.String())
	} else if listenHost != "" && ip == nil {
		add(listenHost)
	}
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if label, _, _ := strings.Cut(hostname, "."); label != "" && label != "localhost" {
		add(hostname)
		add(label + ".local")
	}
	add("localhost")
	add("127.0.0.1")
	add("::1")
	for _, ip := range addrs {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
			continue
		}
		add(ip.String())
	}
	for _, n := range extra {
		add(n)
	}
	return names
}

// ParseNames reads -tls-names: names and addresses separated by commas
// or spaces.
func ParseNames(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' || r == '\t' })
}

// Missing lists the names of want that have lacks.
func Missing(have, want []string) []string {
	var out []string
	for _, w := range want {
		if !slices.ContainsFunc(have, func(h string) bool { return strings.EqualFold(h, w) }) {
			out = append(out, w)
		}
	}
	return out
}
