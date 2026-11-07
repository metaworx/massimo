// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package certs

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme"
)

// A certificate by the DNS-01 challenge over the low-level acme client:
// the TXT record the CA looks for goes through a hook script, so the
// binary knows no DNS provider, no port has to be open (the name may
// point at a LAN address), and wildcards work. The certificate and its
// key are files in the data directory's tls/acme/, served through the
// files source of this package, and ordered anew 30 days before the end
// by the hourly watch; the account key is the one autocert names, shared
// with -tls acme without a hook.
const (
	// AccountKeyFile is the account key in the ACME directory, as
	// autocert names it.
	AccountKeyFile = "acme_account+key"
	// dnsWait is how long an order waits for the TXT record to show at
	// the resolver, dnsEvery how often it looks.
	dnsWait  = 10 * time.Minute
	dnsEvery = 10 * time.Second
	// hookTimeout bounds one run of the hook, hookWaitDelay the wait for
	// its output after the time is up.
	hookTimeout   = 2 * time.Minute
	hookWaitDelay = 5 * time.Second
	// orderTimeout bounds a whole order.
	orderTimeout = 30 * time.Minute
)

// DNS01Paths are the certificate and key files of a DNS-01 order under
// the data directory, named after the first name (a wildcard's star
// spelled out, for the file system).
func DNS01Paths(dataDir, name string) (certPath, keyPath string) {
	dir, _ := ACMEPaths(dataDir, "")
	base := strings.ReplaceAll(name, "*", "_wildcard")
	return filepath.Join(dir, base+".pem"), filepath.Join(dir, base+".key")
}

// DNS01 orders and serves a certificate by DNS-01 through a hook.
type DNS01 struct {
	dataDir   string
	names     []string
	email     string
	directory string
	hook      string
	log       *slog.Logger

	// Seams for the tests: the resolver, the waits, the clock.
	lookupTXT  func(ctx context.Context, name string) ([]string, error)
	dnsWait    time.Duration
	dnsEvery   time.Duration
	httpClient *http.Client

	mu     sync.Mutex
	files  *Files // nil until the first certificate
	status Status
}

// OpenDNS01 prepares DNS-01 orders for the names (DNS names, wildcards
// among them) through the hook, an executable run as "<hook> set <name>
// <txt>" and "<hook> clear <name> <txt>"; the directory "" is Let's
// Encrypt's. A certificate ordered before is served at once; the first
// order happens at the start, in Check.
func OpenDNS01(dataDir string, names []string, email, directory, hook string, log *slog.Logger) (*DNS01, error) {
	if log == nil {
		log = slog.Default()
	}
	if len(names) == 0 {
		return nil, errors.New("tls: -tls acme needs the names of -tls-names")
	}
	for _, n := range names {
		if net.ParseIP(n) != nil {
			return nil, fmt.Errorf("tls: no certificate for an address (%s); use a DNS name", n)
		}
		if !strings.Contains(n, ".") {
			return nil, fmt.Errorf("tls: %q is not a public DNS name", n)
		}
	}
	if hook == "" {
		return nil, errors.New("tls: DNS-01 needs the hook of -acme-dns-hook")
	}
	if fi, err := os.Stat(hook); err != nil {
		return nil, fmt.Errorf("tls: -acme-dns-hook: %w", err)
	} else if runtime.GOOS != "windows" && fi.Mode()&0o111 == 0 {
		return nil, fmt.Errorf("tls: -acme-dns-hook %s is not executable", hook)
	}
	if directory == "" {
		directory = acme.LetsEncryptURL
	}
	dir, _ := ACMEPaths(dataDir, "")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	d := &DNS01{dataDir: dataDir, names: names, email: email, directory: directory, hook: hook, log: log,
		dnsWait: dnsWait, dnsEvery: dnsEvery, httpClient: &http.Client{Timeout: 30 * time.Second},
		status: Status{Source: "acme", Names: names, Warning: "no certificate yet: ordered at the start by DNS-01"}}
	d.lookupTXT = func(ctx context.Context, name string) ([]string, error) {
		return net.DefaultResolver.LookupTXT(ctx, name)
	}
	certPath, keyPath := DNS01Paths(dataDir, names[0])
	if f, err := openFiles("acme", certPath, keyPath, log); err == nil {
		d.files = f
		d.status = f.Status()
		d.status.Names = names
	}
	return d, nil
}

// GetCertificate serves the certificate ordered, or fails the handshake
// while there is none yet.
func (d *DNS01) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	d.mu.Lock()
	f := d.files
	d.mu.Unlock()
	if f == nil {
		return nil, errors.New("tls: no certificate yet; the DNS-01 order has not succeeded")
	}
	return f.GetCertificate(hello)
}

// Status describes the certificate served, or why there is none.
func (d *DNS01) Status() Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.files != nil {
		st := d.files.Status()
		st.Names = d.names
		st.Warning = d.status.Warning
		return st
	}
	return d.status
}

// Check orders the certificate when there is none or it ends within
// RenewBefore, and serves the new one from the next handshake; a failed
// order is logged, the certificate served stays, and the next check
// tries again.
func (d *DNS01) Check(now time.Time) {
	st := d.Status()
	if d.files != nil && now.Add(RenewBefore).Before(st.NotAfter) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), orderTimeout)
	defer cancel()
	d.log.Info("tls: ordering a certificate by DNS-01", "names", d.names, "directory", d.directory)
	if err := d.order(ctx, now); err != nil {
		d.log.Error("tls: the DNS-01 order failed; the next check tries again", "names", d.names, "error", err)
		d.mu.Lock()
		d.status.Warning = "the DNS-01 order failed: " + err.Error()
		d.mu.Unlock()
		return
	}
	st = d.Status()
	d.log.Info("tls: certificate issued by DNS-01", "names", st.Names, "expires", st.NotAfter, "fingerprint", st.Fingerprint)
}

// order runs one order: the account, the authorizations through the
// hook, the CSR, the certificate into the files.
func (d *DNS01) order(ctx context.Context, now time.Time) error {
	dir, _ := ACMEPaths(d.dataDir, "")
	key, err := loadOrMakeAccountKey(filepath.Join(dir, AccountKeyFile))
	if err != nil {
		return err
	}
	client := &acme.Client{Key: key, DirectoryURL: d.directory, HTTPClient: d.httpClient}
	acct := &acme.Account{}
	if d.email != "" {
		acct.Contact = []string{"mailto:" + d.email}
	}
	if _, err := client.Register(ctx, acct, acme.AcceptTOS); err != nil && !errors.Is(err, acme.ErrAccountAlreadyExists) {
		return fmt.Errorf("account: %w", err)
	}
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(d.names...))
	if err != nil {
		return fmt.Errorf("order: %w", err)
	}
	for _, u := range order.AuthzURLs {
		if err := d.authorize(ctx, client, u); err != nil {
			return err
		}
	}
	if order, err = client.WaitOrder(ctx, order.URI); err != nil {
		return fmt.Errorf("order: %w", err)
	}
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	cn := d.names[0]
	for _, n := range d.names {
		if !strings.Contains(n, "*") {
			cn = n
			break
		}
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: cn}, DNSNames: d.names}, certKey)
	if err != nil {
		return err
	}
	der, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return fmt.Errorf("certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(certKey)
	if err != nil {
		return err
	}
	var chain []byte
	for _, b := range der {
		chain = append(chain, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: b})...)
	}
	certPath, keyPath := DNS01Paths(d.dataDir, d.names[0])
	if err := writeAtomic(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})); err != nil {
		return err
	}
	if err := writeAtomic(certPath, chain); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.files == nil {
		f, err := openFiles("acme", certPath, keyPath, d.log)
		if err != nil {
			return err
		}
		d.files = f
	} else {
		d.files.Reload()
	}
	d.status.Warning = ""
	return nil
}

// authorize fulfils one authorization: the TXT record through the hook,
// the wait for it, the challenge accepted and validated, the record
// cleared whatever the outcome.
func (d *DNS01) authorize(ctx context.Context, client *acme.Client, url string) error {
	z, err := client.GetAuthorization(ctx, url)
	if err != nil {
		return fmt.Errorf("authorization: %w", err)
	}
	if z.Status == acme.StatusValid {
		return nil
	}
	var ch *acme.Challenge
	for _, c := range z.Challenges {
		if c.Type == "dns-01" {
			ch = c
		}
	}
	if ch == nil {
		return fmt.Errorf("%s: the CA offers no dns-01 challenge", z.Identifier.Value)
	}
	txt, err := client.DNS01ChallengeRecord(ch.Token)
	if err != nil {
		return err
	}
	record := "_acme-challenge." + z.Identifier.Value
	if err := d.runHook(ctx, "set", record, txt); err != nil {
		return err
	}
	err = d.waitTXT(ctx, record, txt)
	if err == nil {
		if _, err = client.Accept(ctx, ch); err == nil {
			_, err = client.WaitAuthorization(ctx, z.URI)
		}
	}
	// The clear runs on a context of its own: the order's may be over
	// (its 30 minutes, or the wait for the record), and a record left
	// behind would pile up with every retry.
	if cerr := d.runHook(context.WithoutCancel(ctx), "clear", record, txt); cerr != nil {
		d.log.Warn("tls: the dns hook could not clear the record", "record", record, "error", cerr)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", z.Identifier.Value, err)
	}
	return nil
}

// waitTXT waits until the resolver returns the record's value, so that
// the CA is not asked before the zone shows it.
func (d *DNS01) waitTXT(ctx context.Context, record, txt string) error {
	deadline := time.Now().Add(d.dnsWait)
	for {
		if values, err := d.lookupTXT(ctx, record); err == nil {
			for _, v := range values {
				if v == txt {
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the TXT record %s did not show within %s", record, d.dnsWait)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d.dnsEvery):
		}
	}
}

// runHook runs the hook with the verb, the record and the value, with
// the server's environment less its MASSIMO_ variables (the keys and the
// token are none of the hook's business) and a timeout, after which the
// hook and, on unix, its children are ended and its output is waited
// for hookWaitDelay at most; its output goes to the log.
func (d *DNS01) runHook(ctx context.Context, verb, record, txt string) error {
	ctx, cancel := context.WithTimeout(ctx, hookTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, d.hook, verb, record, txt)
	cmd.Env = hookEnv(os.Environ())
	cmd.WaitDelay = hookWaitDelay
	ownGroup(cmd)
	out, err := cmd.CombinedOutput()
	if s := strings.TrimSpace(string(out)); s != "" {
		d.log.Info("tls: dns hook", "verb", verb, "record", record, "output", s)
	}
	if err != nil {
		return fmt.Errorf("hook %s %s: %w", verb, record, err)
	}
	return nil
}

// hookEnv is the environment without the MASSIMO_ variables.
func hookEnv(environ []string) []string {
	var out []string
	for _, kv := range environ {
		if name, _, _ := strings.Cut(kv, "="); len(name) >= 8 && strings.EqualFold(name[:8], "MASSIMO_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// loadOrMakeAccountKey reads the ECDSA account key, or makes one, as
// autocert keeps it.
func loadOrMakeAccountKey(path string) (*ecdsa.PrivateKey, error) {
	if b, err := os.ReadFile(path); err == nil {
		if block, _ := pem.Decode(b); block != nil {
			if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
				return key, nil
			}
		}
		return nil, fmt.Errorf("tls: %s is not an EC private key", path)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := writeAtomic(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b})); err != nil {
		return nil, err
	}
	return key, nil
}
