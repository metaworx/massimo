// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/certs"
	"go.metaworx.ch/massimo/internal/env"
)

func TestTLSCommand(t *testing.T) {
	data := t.TempDir()
	t.Setenv(env.DataVar, data)
	t.Setenv(tlsVar, "")
	t.Setenv(tlsCertVar, "")
	t.Setenv(tlsKeyVar, "")
	t.Setenv(tlsNamesVar, "")
	t.Setenv("MASSIMO_LISTEN", "")
	var out strings.Builder
	if err := tlsCommand([]string{"show"}, &out); err != nil || !strings.Contains(out.String(), "serve has no TLS") {
		t.Fatalf("show off: %v %q", err, out.String())
	}
	t.Setenv(tlsVar, "self-signed")
	out.Reset()
	if err := tlsCommand([]string{"show"}, &out); err != nil || !strings.Contains(out.String(), "no certificate in "+filepath.Join(data, "tls")+" yet") {
		t.Fatalf("show before: %v %q", err, out.String())
	}
	out.Reset()
	if err := tlsCommand([]string{"self-signed", "-names", "massimo.example.net"}, &out); err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := certs.SelfSignedPaths(data)
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatal("no pair written")
	}
	_, st, err := certs.Load("self-signed", certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "wrote "+certPath) || !strings.Contains(out.String(), st.Fingerprint) || !strings.Contains(out.String(), "massimo.example.net") {
		t.Fatalf("self-signed: %q", out.String())
	}
	out.Reset()
	if err := tlsCommand([]string{"show"}, &out); err != nil || !strings.Contains(out.String(), "source:        self-signed") || !strings.Contains(out.String(), st.Fingerprint) || !strings.Contains(out.String(), "browsers warn once") {
		t.Fatalf("show after: %v %q", err, out.String())
	}
	// config prints the same.
	out.Reset()
	printTLS(&out)
	if !strings.Contains(out.String(), "tls:           self-signed ("+filepath.Join(data, "tls")) || !strings.Contains(out.String(), st.Fingerprint) {
		t.Fatalf("config: %q", out.String())
	}
	for _, bad := range [][]string{nil, {"what"}, {"self-signed", "-x"}} {
		if err := tlsCommand(bad, &out); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
