// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/certs"
	"go.metaworx.ch/massimo/internal/env"
)

// The TLS of serve: -tls names the certificate source (internal/certs),
// one tls.Config serves it, and the plain listener stays the default. The
// variables are those of the settings file, where install writes them
// (docs/store.md section 4: needed before the store opens).
const (
	// The variables install writes, named once in internal/env.
	tlsVar      = env.TLS
	tlsCertVar  = env.TLSCert
	tlsKeyVar   = env.TLSKey
	tlsNamesVar = env.TLSNames
	hstsVar     = env.HSTS
	// -tls acme: the account's contact address, the directory (Let's
	// Encrypt's unless set; its staging one for experiments) and the
	// plain helper listener for HTTP-01 and the redirect.
	acmeEmailVar     = env.ACMEEmail
	acmeDirectoryVar = env.ACMEDirectory
	tlsHTTPVar       = env.TLSHTTP
	// -acme-dns-hook: the DNS-01 challenge through a hook instead of
	// TLS-ALPN-01 and HTTP-01; no port, wildcards allowed.
	acmeDNSHookVar = env.ACMEDNSHook
	// -trusted-proxies: the reverse proxies whose headers name the client.
	trustedProxiesVar = env.TrustedProxies
	// certWatch is how often a source checks its certificate: a renewal
	// 30 days before the end needs no finer clock.
	certWatch = time.Hour
	// defaultHSTSDays is how long browsers are told to use https://
	// for the host: a year, as the owner decided.
	defaultHSTSDays = 365
)

// tlsMode is -tls as it applies: "files" when only the files are given,
// "off" when nothing is.
func tlsMode(mode, certPath, keyPath string) string {
	if mode == "" {
		if certPath != "" || keyPath != "" {
			return "files"
		}
		return "off"
	}
	return mode
}

// tlsOptions are serve's TLS flags.
type tlsOptions struct {
	mode, certPath, keyPath string
	names                   string // -tls-names
	listen                  string // -listen, for the self-signed names
	dataDir                 string // where the self-signed pair and the ACME cache live
	email, directory        string // -acme-email, -acme-directory
	httpAddr                string // -tls-http
	hook                    string // -acme-dns-hook
}

// tlsSource is the certificate source -tls names, nil for off.
func tlsSource(o tlsOptions, log *slog.Logger) (certs.Source, error) {
	m := tlsMode(o.mode, o.certPath, o.keyPath)
	if o.httpAddr != "" && m != "acme" {
		return nil, errors.New("serve: -tls-http goes with -tls acme")
	}
	if o.hook != "" && m != "acme" {
		return nil, errors.New("serve: -acme-dns-hook goes with -tls acme")
	}
	if o.hook != "" && o.httpAddr != "" {
		return nil, errors.New("serve: -tls-http is for the HTTP-01 challenge; with -acme-dns-hook the DNS-01 challenge needs no port")
	}
	switch m {
	case "off":
		return nil, nil
	case "files":
		if o.certPath == "" || o.keyPath == "" {
			return nil, errors.New("serve: -tls files needs -tls-cert and -tls-key (" + tlsCertVar + " and " + tlsKeyVar + ")")
		}
		src, err := certs.OpenFiles(o.certPath, o.keyPath, log)
		if err != nil {
			return nil, fmt.Errorf("serve: %w", err)
		}
		return src, nil
	case "self-signed":
		src, err := certs.OpenSelfSigned(o.dataDir, certs.Names(listenHost(o.listen), certs.ParseNames(o.names)), log, nil)
		if err != nil {
			return nil, fmt.Errorf("serve: %w", err)
		}
		return src, nil
	case "acme":
		if o.hook != "" {
			src, err := certs.OpenDNS01(o.dataDir, certs.ParseNames(o.names), o.email, o.directory, o.hook, log)
			if err != nil {
				return nil, fmt.Errorf("serve: %w", err)
			}
			return src, nil
		}
		src, err := certs.OpenACME(o.dataDir, certs.ParseNames(o.names), o.email, o.directory, log)
		if err != nil {
			return nil, fmt.Errorf("serve: %w", err)
		}
		return src, nil
	default:
		return nil, fmt.Errorf("serve: -tls %q: want off, files, self-signed or acme", m)
	}
}

// listenHost is the host of a listen address, "" for every interface.
func listenHost(listen string) string {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return strings.Trim(listen, "[]")
	}
	return host
}

// hstsDays reads -hsts: a number of days, 0 or more; "" is the default.
func hstsDays(v string) (int, error) {
	if v == "" {
		return defaultHSTSDays, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("serve: -hsts %q: want a number of days, 0 or more", v)
	}
	return n, nil
}

// serveHTTP serves hs on ln until ctx is done, over TLS when hs has a
// TLSConfig (whose GetCertificate is the source, so no file is named),
// then shuts down with a short grace.
func serveHTTP(ctx context.Context, hs *http.Server, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(shutdown)
	}()
	var err error
	if hs.TLSConfig != nil {
		err = hs.ServeTLS(ln, "", "")
	} else {
		err = hs.Serve(ln)
	}
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
