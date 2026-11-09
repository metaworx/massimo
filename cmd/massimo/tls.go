// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/certs"
	"go.metaworx.ch/massimo/internal/serve"
)

// runTLS handles massimo tls show | self-signed: the certificate serve
// presents, by the settings in effect, and a new self-signed pair into
// the data directory's tls/, which a running serve takes up at the next
// connection. Like store, it runs as the service user beside the server.
func runTLS(args []string) error { return tlsCommand(args, os.Stdout) }

func tlsCommand(args []string, w io.Writer) error {
	usage := func() {
		fmt.Fprint(os.Stderr, `usage: massimo [-data <dir>] tls show | self-signed [-names <name,...>]

  show         the certificate serve presents: its source (MASSIMO_TLS), names, expiry and fingerprint;
               for acme, the certificate cached for each name of MASSIMO_TLS_NAMES
  self-signed  make a new self-signed certificate for the box's names in <data>/tls/; a running
               serve -tls self-signed takes it up at the next connection, browsers ask once more;
               a host name the service listens on (MASSIMO_LISTEN, set by its unit) goes in -names

`+dataDirUsage("tls", "show"))
	}
	if len(args) == 0 {
		usage()
		return errors.New("tls: which command?")
	}
	switch args[0] {
	case "show":
		printTLS(w) // what massimo config prints of it
	case "self-signed":
		fs := flag.NewFlagSet("tls self-signed", flag.ContinueOnError)
		names := fs.String("names", os.Getenv(tlsNamesVar), "names and addresses the certificate holds besides the box's own, comma-separated (default "+tlsNamesVar+")")
		fs.Usage = usage
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		listen := os.Getenv("MASSIMO_LISTEN")
		certPath, keyPath := certs.SelfSignedPaths(dataDir())
		st, err := certs.Generate(certPath, keyPath, certs.Names(serve.ListenHost(listen), certs.ParseNames(*names)), time.Now())
		if err != nil {
			return fmt.Errorf("tls: %w", err)
		}
		fmt.Fprintf(w, "wrote %s and %s\n", certPath, keyPath)
		fmt.Fprintf(w, "names:         %s\n", strings.Join(st.Names, ", "))
		fmt.Fprintf(w, "expires:       %s\n", st.NotAfter.Format("2006-01-02"))
		fmt.Fprintf(w, "fingerprint:   %s\n", st.Fingerprint)
		fmt.Fprintln(w, "a running serve -tls self-signed presents it from the next connection; browsers ask once more on every device")
	default:
		usage()
		return fmt.Errorf("tls: unknown command %q", args[0])
	}
	return nil
}
