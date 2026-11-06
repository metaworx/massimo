// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package certs provides the certificate massimo serve presents. Every
// source - the owner's certificate files, re-read when they change;
// later a self-signed pair the server makes, and Let's Encrypt - ends in
// one GetCertificate on one tls.Config, so that a renewed certificate is
// served at the next handshake without a restart, and the server has one
// TLS path to test.
package certs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// Source yields the certificate the server presents.
type Source interface {
	// GetCertificate is tls.Config.GetCertificate.
	GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)
	// Status describes the certificate served, for the log, /healthz
	// and the web UI.
	Status() Status
}

// Checker is a source with work to do now and then: a renewal, a
// warning before an expiry.
type Checker interface {
	Check(now time.Time)
}

// Watch runs src's Check once now and then every interval until ctx is
// done, for a source that is a Checker; the others need none.
func Watch(ctx context.Context, src Source, every time.Duration) {
	c, ok := src.(Checker)
	if !ok {
		return
	}
	c.Check(time.Now())
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			c.Check(now)
		}
	}
}

// Status describes the certificate a source serves.
type Status struct {
	// Source is where the certificate comes from: "files",
	// "self-signed" for the pair the server made, or "acme".
	Source string `json:"source"`
	// Names are the certificate's subject alternative names, DNS names
	// and addresses, else its common name.
	Names    []string  `json:"names"`
	NotAfter time.Time `json:"not_after"`
	// Fingerprint is the SHA-256 of the certificate, as browsers show
	// it (Fingerprint).
	Fingerprint string `json:"fingerprint"`
	// SelfSigned says the certificate signs itself: a browser warns
	// once per device, and the fingerprint is what the owner compares.
	SelfSigned bool `json:"self_signed"`
	// Warning is set while something needs the owner, such as a
	// certificate about to expire.
	Warning string `json:"warning,omitempty"`
}

// Config is the server's one tls.Config: TLS 1.2 at least, HTTP/2 and
// HTTP/1.1 (and a source's own protocol, the ACME challenge's), the
// certificate from src at every handshake.
func Config(src Source) *tls.Config {
	cfg := &tls.Config{
		MinVersion:     tls.VersionTLS12,
		NextProtos:     []string{"h2", "http/1.1"},
		GetCertificate: src.GetCertificate,
	}
	if p, ok := src.(interface{ NextProtos() []string }); ok {
		cfg.NextProtos = append(cfg.NextProtos, p.NextProtos()...)
	}
	return cfg
}

// Fingerprint is the SHA-256 of a certificate in DER form, as browsers
// show it: "SHA256:" and the bytes in upper-case hex separated by colons.
func Fingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	h := strings.ToUpper(hex.EncodeToString(sum[:]))
	var b strings.Builder
	b.WriteString("SHA256:")
	for i := 0; i < len(h); i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(h[i : i+2])
	}
	return b.String()
}

// Describe reads the leaf of a certificate into a Status with the given
// source.
func Describe(source string, cert *tls.Certificate) (Status, error) {
	if cert == nil || len(cert.Certificate) == 0 {
		return Status{}, errors.New("tls: no certificate")
	}
	leaf := cert.Leaf
	if leaf == nil {
		var err error
		if leaf, err = x509.ParseCertificate(cert.Certificate[0]); err != nil {
			return Status{}, err
		}
	}
	st := Status{Source: source, NotAfter: leaf.NotAfter, Fingerprint: Fingerprint(cert.Certificate[0])}
	st.Names = append(st.Names, leaf.DNSNames...)
	for _, ip := range leaf.IPAddresses {
		st.Names = append(st.Names, ip.String())
	}
	if len(st.Names) == 0 && leaf.Subject.CommonName != "" {
		st.Names = []string{leaf.Subject.CommonName}
	}
	st.SelfSigned = bytes.Equal(leaf.RawIssuer, leaf.RawSubject) &&
		leaf.CheckSignature(leaf.SignatureAlgorithm, leaf.RawTBSCertificate, leaf.Signature) == nil
	return st, nil
}
