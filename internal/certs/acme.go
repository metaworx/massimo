// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package certs

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// A Let's Encrypt certificate through autocert: the order is validated
// by TLS-ALPN-01 on the TLS listener itself, so the public port must be
// 443 (the router may forward it to any port of the box), or by HTTP-01
// on a plain helper listener once HTTPHandler is served on port 80. The
// account key and the certificates live in <data>/tls/acme/ and survive
// updates and installs; autocert renews 30 days before the end on its
// own, and the hourly watch (Check) gets the certificate of every name
// at the start, so the log says at once whether the order worked, and
// warns under ACMEWarnBefore. autocert logs nothing itself: its errors
// reach only GetCertificate, which logs each name's once in a while, and
// the cache logs each certificate autocert stores (logCache).
const (
	// ACMEDir is the cache directory under the data directory.
	ACMEDir = "tls/acme"
	// WarnBefore is how long before the end a certificate the server
	// does not renew itself (files, ACME) is warned about.
	WarnBefore = 14 * 24 * time.Hour
	// acmeErrorEvery is how often one name's failing order is logged.
	acmeErrorEvery = 10 * time.Minute
)

// ACMEPaths are the cache directory of the account and the certificates
// under the data directory, and the cached certificate of a name.
func ACMEPaths(dataDir, name string) (dir, certPath string) {
	dir = filepath.Join(dataDir, filepath.FromSlash(ACMEDir))
	return dir, filepath.Join(dir, name)
}

// ACME serves certificates from Let's Encrypt, or another ACME directory,
// for a fixed list of DNS names.
type ACME struct {
	m     *autocert.Manager
	names []string
	log   *slog.Logger

	mu     sync.Mutex
	status Status
	errAt  map[string]time.Time // when each name's failure was last logged
}

// OpenACME prepares the manager for the names (DNS names, no address
// and no wildcard: those need DNS-01), with the account's contact
// address and the directory URL ("" is Let's Encrypt's), the cache in
// dataDir's tls/acme/. Nothing is ordered here: the first connection, or
// Check, does that.
func OpenACME(dataDir string, names []string, email, directory string, log *slog.Logger) (*ACME, error) {
	if log == nil {
		log = slog.Default()
	}
	if len(names) == 0 {
		return nil, errors.New("tls: -tls acme needs the names of -tls-names")
	}
	for _, n := range names {
		switch {
		case net.ParseIP(n) != nil:
			return nil, fmt.Errorf("tls: Let's Encrypt issues no certificate for an address (%s); use a DNS name", n)
		case strings.Contains(n, "*"):
			return nil, fmt.Errorf("tls: a wildcard (%s) needs the DNS-01 challenge (-acme-dns-hook)", n)
		case !strings.Contains(n, "."):
			return nil, fmt.Errorf("tls: %q is not a public DNS name", n)
		}
	}
	dir, _ := ACMEPaths(dataDir, "")
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      logCache{Cache: autocert.DirCache(dir), log: log},
		HostPolicy: autocert.HostWhitelist(names...),
		Email:      email,
		Client:     &acme.Client{DirectoryURL: directory, HTTPClient: &http.Client{Timeout: 30 * time.Second}},
	}
	if directory == "" {
		m.Client.DirectoryURL = acme.LetsEncryptURL
	}
	return &ACME{m: m, names: names, log: log, errAt: map[string]time.Time{},
		status: Status{Source: "acme", Names: names, Warning: "no certificate yet: ordered at the start and at the first connection to " + names[0]}}, nil
}

// NextProtos is the ALPN protocol of the TLS-ALPN-01 challenge, which
// Config adds to the listener's.
func (a *ACME) NextProtos() []string { return []string{acme.ALPNProto} }

// GetCertificate serves the name's certificate, ordering or renewing it
// when needed, and answers the TLS-ALPN-01 challenge. A handshake
// without a server name, as clients make to an address (the update
// helper's https://127.0.0.1 health check, a browser at https://<ip>),
// gets the first name's: a copy of the hello names it, since autocert
// refuses a hello without one.
func (a *ACME) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if hello.ServerName == "" {
		named := *hello
		named.ServerName = a.names[0]
		hello = &named
	}
	cert, err := a.m.GetCertificate(hello)
	if err != nil {
		a.logError(hello.ServerName, err)
		return nil, err
	}
	a.remember(cert, time.Now())
	return cert, nil
}

// Status describes the certificate last served, or why there is none.
func (a *ACME) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

// HTTPHandler answers the HTTP-01 challenge and redirects everything
// else to https://, for the plain helper listener on port 80; serving it
// is what makes the manager use HTTP-01 beside TLS-ALPN-01.
func (a *ACME) HTTPHandler() http.Handler { return a.m.HTTPHandler(nil) }

// browserHello is the hello Check asks autocert with: what a modern
// browser sends, so that the certificate ordered and renewed is the
// ECDSA one browsers get, and not a second, RSA one for a hello that
// names nothing else.
func browserHello(name string) *tls.ClientHelloInfo {
	return &tls.ClientHelloInfo{
		ServerName:        name,
		CipherSuites:      []uint16{tls.TLS_AES_128_GCM_SHA256, tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		SignatureSchemes:  []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256, tls.PSSWithSHA256, tls.PKCS1WithSHA256},
		SupportedCurves:   []tls.CurveID{tls.X25519, tls.CurveP256},
		SupportedVersions: []uint16{tls.VersionTLS13, tls.VersionTLS12},
	}
}

// Check gets the certificate of every name, as a browser does (which
// orders or renews it when due), and warns under WarnBefore.
func (a *ACME) Check(now time.Time) {
	for _, n := range a.names {
		cert, err := a.m.GetCertificate(browserHello(n))
		if err != nil {
			a.logError(n, err)
			continue
		}
		a.remember(cert, now)
	}
	if st := a.Status(); st.Warning != "" && !st.NotAfter.IsZero() {
		a.log.Warn("tls: "+st.Warning, "names", st.Names, "expires", st.NotAfter)
	}
}

// logCache is autocert's cache with a line in the log for every
// certificate autocert stores, which is a new one: the first order's or
// a renewal's. A certificate an earlier run left in the cache is only
// read (Get), so the line never repeats an old one.
type logCache struct {
	autocert.Cache
	log *slog.Logger
}

// Put stores the data and, for a certificate, logs it as the DNS-01
// source logs its own: the key is the name ("<name>+rsa" for an RSA
// one); the account key (acme_account+key), an HTTP-01 token
// (<token>+http-01) and a TLS-ALPN-01 challenge's certificate
// (<name>+token) are no certificate of the server's. The data is the
// key's PEM followed by the chain's, the leaf first.
func (c logCache) Put(ctx context.Context, key string, data []byte) error {
	if err := c.Cache.Put(ctx, key, data); err != nil {
		return err
	}
	name := strings.TrimSuffix(key, "+rsa")
	if strings.Contains(name, "+") {
		return nil
	}
	for rest := data; ; {
		var b *pem.Block
		if b, rest = pem.Decode(rest); b == nil {
			break
		}
		if b.Type != "CERTIFICATE" {
			continue
		}
		if leaf, err := x509.ParseCertificate(b.Bytes); err == nil {
			c.log.Info("tls: certificate issued by the ACME directory", "name", name, "expires", leaf.NotAfter, "fingerprint", Fingerprint(b.Bytes))
			return nil
		}
		break
	}
	c.log.Info("tls: certificate issued by the ACME directory", "name", name)
	return nil
}

// remember keeps the certificate served as the status.
func (a *ACME) remember(cert *tls.Certificate, now time.Time) {
	st, err := Describe("acme", cert)
	if err != nil {
		return
	}
	st.Names = a.names
	st.Warning = expiryWarning(st.NotAfter, now)
	a.mu.Lock()
	a.status = st
	a.mu.Unlock()
}

// logError logs a name's failing order, once per acmeErrorEvery.
func (a *ACME) logError(name string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if last, ok := a.errAt[name]; ok && time.Since(last) < acmeErrorEvery {
		return
	}
	a.errAt[name] = time.Now()
	a.log.Error("tls: no certificate from the ACME directory", "name", name, "error", err)
}

// expiryWarning is the warning for a certificate that ends within
// WarnBefore of now, "" otherwise.
func expiryWarning(notAfter, now time.Time) string {
	if notAfter.IsZero() {
		return ""
	}
	left := notAfter.Sub(now)
	switch {
	case left <= 0:
		return "the certificate has expired"
	case left < WarnBefore:
		return fmt.Sprintf("the certificate expires in %d days", int(left.Hours()/24))
	}
	return ""
}

// LoadACMECache reads the certificate autocert cached for a name under
// the data directory, for massimo tls show and config.
func LoadACMECache(dataDir, name string) (Status, error) {
	_, path := ACMEPaths(dataDir, name)
	b, err := os.ReadFile(path)
	if err != nil {
		return Status{}, err
	}
	var keyPEM, certPEM []byte
	for {
		var block *pem.Block
		if block, b = pem.Decode(b); block == nil {
			break
		}
		if strings.HasSuffix(block.Type, "PRIVATE KEY") {
			keyPEM = append(keyPEM, pem.EncodeToMemory(block)...)
		} else {
			certPEM = append(certPEM, pem.EncodeToMemory(block)...)
		}
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return Status{}, fmt.Errorf("tls: %s: %w", path, err)
	}
	return Describe("acme", &cert)
}
