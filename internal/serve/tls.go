// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serve

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/certs"
	"go.metaworx.ch/massimo/internal/env"
)

// certWatch is how often a source checks its certificate: a renewal 30
// days before the end needs no finer clock.
const certWatch = time.Hour

// TLS names the certificate source of the listener (internal/certs),
// as serve's -tls flags give it; one tls.Config serves it.
type TLS struct {
	// Mode is off, files, self-signed or acme; "" is files where Cert
	// or Key is given, else off (TLSMode).
	Mode string
	// Cert and Key are the PEM files of files mode, re-read when they
	// change.
	Cert, Key string
	// Names, comma-separated: with self-signed, the names and addresses
	// the certificate holds besides the box's own; with acme, the DNS
	// names to get a certificate for.
	Names string
	// ACMEEmail is the account's contact address, ACMEDirectory the
	// directory URL (Let's Encrypt's when empty).
	ACMEEmail, ACMEDirectory string
	// HTTP, with acme, is a plain address such as :80 also listened on
	// for the HTTP-01 challenge and a redirect to https://.
	HTTP string
	// DNSHook, with acme, gets the certificate by the DNS-01 challenge
	// through this executable.
	DNSHook string
}

// TLSMode is the mode as it applies: "files" when only the files are
// given, "off" when nothing is.
func TLSMode(mode, certPath, keyPath string) string {
	if mode == "" {
		if certPath != "" || keyPath != "" {
			return "files"
		}
		return "off"
	}
	return mode
}

// tlsSource is the certificate source o names, nil for off; listen is
// the API's address, whose host a self-signed certificate holds, and
// dataDir where the self-signed pair and the ACME cache live.
func tlsSource(o TLS, listen, dataDir string, log *slog.Logger) (certs.Source, error) {
	m := TLSMode(o.Mode, o.Cert, o.Key)
	if o.HTTP != "" && m != "acme" {
		return nil, errors.New("serve: -tls-http goes with -tls acme")
	}
	if o.DNSHook != "" && m != "acme" {
		return nil, errors.New("serve: -acme-dns-hook goes with -tls acme")
	}
	if o.DNSHook != "" && o.HTTP != "" {
		return nil, errors.New("serve: -tls-http is for the HTTP-01 challenge; with -acme-dns-hook the DNS-01 challenge needs no port")
	}
	switch m {
	case "off":
		return nil, nil
	case "files":
		if o.Cert == "" || o.Key == "" {
			return nil, errors.New("serve: -tls files needs -tls-cert and -tls-key (" + env.TLSCert + " and " + env.TLSKey + ")")
		}
		src, err := certs.OpenFiles(o.Cert, o.Key, log)
		if err != nil {
			return nil, fmt.Errorf("serve: %w", err)
		}
		return src, nil
	case "self-signed":
		src, err := certs.OpenSelfSigned(dataDir, certs.Names(ListenHost(listen), certs.ParseNames(o.Names)), log, nil)
		if err != nil {
			return nil, fmt.Errorf("serve: %w", err)
		}
		return src, nil
	case "acme":
		if o.DNSHook != "" {
			src, err := certs.OpenDNS01(dataDir, certs.ParseNames(o.Names), o.ACMEEmail, o.ACMEDirectory, o.DNSHook, log)
			if err != nil {
				return nil, fmt.Errorf("serve: %w", err)
			}
			return src, nil
		}
		src, err := certs.OpenACME(dataDir, certs.ParseNames(o.Names), o.ACMEEmail, o.ACMEDirectory, log)
		if err != nil {
			return nil, fmt.Errorf("serve: %w", err)
		}
		return src, nil
	default:
		return nil, fmt.Errorf("serve: -tls %q: want off, files, self-signed or acme", m)
	}
}

// ListenHost is the host of a listen address, "" for every interface.
func ListenHost(listen string) string {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return strings.Trim(listen, "[]")
	}
	return host
}

// serveHTTP serves hs on ln until ctx is done, over TLS when hs has a
// TLSConfig (whose GetCertificate is the source, so no file is named),
// then shuts down with a short grace. It returns once the requests still
// being answered have finished, or the grace is over, so that their
// audit rows are written before the store closes; Serve itself returns
// as soon as the listener is closed.
func serveHTTP(ctx context.Context, hs *http.Server, ln net.Listener) error {
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
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
	<-stopped
	return nil
}
