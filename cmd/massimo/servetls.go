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
	"time"

	"go.metaworx.ch/massimo/internal/certs"
)

// The TLS of serve: -tls names the certificate source (internal/certs),
// one tls.Config serves it, and the plain listener stays the default. The
// variables are those of the settings file, where install writes them
// (docs/store.md section 4: needed before the store opens).
const (
	tlsVar     = "MASSIMO_TLS"
	tlsCertVar = "MASSIMO_TLS_CERT"
	tlsKeyVar  = "MASSIMO_TLS_KEY"
	hstsVar    = "MASSIMO_HSTS"
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

// tlsSource is the certificate source -tls names, nil for off.
func tlsSource(mode, certPath, keyPath string, log *slog.Logger) (certs.Source, error) {
	switch m := tlsMode(mode, certPath, keyPath); m {
	case "off":
		return nil, nil
	case "files":
		if certPath == "" || keyPath == "" {
			return nil, errors.New("serve: -tls files needs -tls-cert and -tls-key (" + tlsCertVar + " and " + tlsKeyVar + ")")
		}
		src, err := certs.OpenFiles(certPath, keyPath, log)
		if err != nil {
			return nil, fmt.Errorf("serve: %w", err)
		}
		return src, nil
	case "self-signed", "acme":
		return nil, fmt.Errorf("serve: -tls %s is not in this version yet; use -tls files", m)
	default:
		return nil, fmt.Errorf("serve: -tls %q: want off or files", m)
	}
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
