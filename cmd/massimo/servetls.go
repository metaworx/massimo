// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"fmt"
	"strconv"

	"go.metaworx.ch/massimo/internal/env"
)

// The TLS of serve: -tls names the certificate source (internal/serve's
// TLS, made by internal/certs), and the plain listener stays the default.
// The variables are those of the settings file, where install writes
// them (docs/store.md section 4: needed before the store opens).
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
	// defaultHSTSDays is how long browsers are told to use https://
	// for the host: a year, as the owner decided.
	defaultHSTSDays = 365
)

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
