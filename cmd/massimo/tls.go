// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/certs"
)

// runTLS handles massimo tls show | self-signed: the certificate serve
// presents, by the settings in effect, and a new self-signed pair into
// the data directory's tls/, which a running serve takes up at the next
// connection. Like store, it runs as the service user beside the server.
func runTLS(args []string) error { return tlsCommand(args, os.Stdout) }

func tlsCommand(args []string, w io.Writer) error {
	usage := func() {
		fmt.Fprint(os.Stderr, `usage: massimo [-data <dir>] tls show | self-signed [-names <name,...>]

  show         the certificate serve presents: its source (MASSIMO_TLS), names, expiry and fingerprint
  self-signed  make a new self-signed certificate for the box's names in <data>/tls/; a running
               serve -tls self-signed takes it up at the next connection, browsers ask once more

The data directory is -data, else MASSIMO_DATA, else the current directory;
for the service run it as its user, for example
  sudo -u massimo massimo -data /var/lib/massimo tls show
`)
	}
	if len(args) == 0 {
		usage()
		return errors.New("tls: which command?")
	}
	switch args[0] {
	case "show":
		certPath, keyPath := os.Getenv(tlsCertVar), os.Getenv(tlsKeyVar)
		switch mode := tlsMode(os.Getenv(tlsVar), certPath, keyPath); mode {
		case "off":
			fmt.Fprintf(w, "serve has no TLS (%s is off)\n", tlsVar)
		case "files":
			fmt.Fprintf(w, "source:        files (cert %s, key %s)\n", orNone(certPath), orNone(keyPath))
			printCertificate(w, mode, certPath, keyPath)
		case "self-signed":
			certPath, keyPath = certs.SelfSignedPaths(dataDir())
			if _, err := os.Stat(certPath); err != nil {
				fmt.Fprintf(w, "source:        self-signed; no certificate in %s yet: serve makes one at its start, or make it now with: massimo tls self-signed\n", filepath.Dir(certPath))
				return nil
			}
			fmt.Fprintf(w, "source:        self-signed (%s)\n", filepath.Dir(certPath))
			printCertificate(w, mode, certPath, keyPath)
		default:
			fmt.Fprintf(w, "source:        %s, not in this version yet\n", mode)
		}
	case "self-signed":
		fs := flag.NewFlagSet("tls self-signed", flag.ContinueOnError)
		names := fs.String("names", os.Getenv(tlsNamesVar), "names and addresses the certificate holds besides the box's own, comma-separated (default "+tlsNamesVar+")")
		fs.Usage = usage
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		listen := os.Getenv("MASSIMO_LISTEN")
		certPath, keyPath := certs.SelfSignedPaths(dataDir())
		st, err := certs.Generate(certPath, keyPath, certs.Names(listenHost(listen), certs.ParseNames(*names)), time.Now())
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
