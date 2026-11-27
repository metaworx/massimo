// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !windows

package main

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// Root does not write a store another user owns; anyone else tries, and
// the store's own rights decide.
func TestStoreOfAnother(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "massimo.db")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if ownedByAnother(file, 1000) {
		t.Fatal("a user's run counts as root's")
	}
	if !ownedByAnother(filepath.Join(t.TempDir(), "none"), 0) {
		t.Fatal("a file root cannot read counts as root's own")
	}
	if os.Geteuid() == 0 {
		t.Skip("the test runs as root, who owns its file")
	}
	if !ownedByAnother(file, 0) {
		t.Fatal("root over another user's store")
	}
}

// Where nobody names the data directory or a settings file, as for root
// in any directory, the hand update asks the service with its settings
// file's key, address and TLS, and a backup on the store carries that
// file; the environment's own come first, and a settings file named
// keeps the service's out.
func TestUpdateSettingsAreTheService(t *testing.T) {
	envPath, stateDir := serviceFiles(t)
	if err := os.WriteFile(envPath, []byte("MASSIMO_API_TOKEN=the-service-key\nMASSIMO_LISTEN=127.0.0.1:8443\nMASSIMO_TLS=self-signed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{tokenVar, listenVar, env.TLS, env.TLSCert, env.TLSKey} {
		t.Setenv(name, "")
	}
	s := updateSettingsFor(stateDir)
	if s.envFile != envPath || s.token != "the-service-key" || s.listen != "127.0.0.1:8443" || s.tls.Mode != "self-signed" {
		t.Fatalf("the service's: %+v", s)
	}
	t.Setenv(tokenVar, "mine")
	if s := updateSettingsFor(stateDir); s.token != "mine" || s.listen != "127.0.0.1:8443" {
		t.Fatalf("the environment's first: %+v", s)
	}
	t.Setenv(env.FileVar, os.DevNull)
	if s := updateSettingsFor(stateDir); s.envFile != os.DevNull || s.listen != defaultListen || s.tls.Mode != "" {
		t.Fatalf("a settings file named: %+v", s)
	}
	t.Setenv(env.FileVar, "")
	if s := updateSettingsFor(""); s.envFile == envPath {
		t.Fatalf("no service's store, yet its settings: %+v", s)
	}
}

// Root, which cannot write the service's store, asks the service; where
// the service refuses the key, or none answers: without a key a line
// and the update goes on; with one it stops and says how to go on,
// and nothing is made.
func TestBackupBeforeUpdateAsRoot(t *testing.T) {
	testkit.Swapping(t)
	if os.Geteuid() == 0 {
		t.Skip("the test runs as root, who owns its store")
	}
	saved := euid
	euid = func() int { return 0 }
	t.Cleanup(func() { euid = saved })
	ctx := context.Background()
	dir := t.TempDir()
	storekit.Open(t, dir).Close()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "authentication required"}`))
	}))
	defer ts.Close()
	var out bytes.Buffer
	c := &backupCLI{dir: dir, api: &serveAPI{base: ts.URL, client: ts.Client()}, out: &out, log: slog.New(slog.DiscardHandler)}
	if err := c.beforeUpdate(ctx, "v0.17.0"); err != nil || !strings.Contains(out.String(), "no key is set") {
		t.Fatalf("no key: %v\n%s", err, out.String())
	}
	st := storekit.Open(t, dir)
	k, err := serverbackup.GenerateKey(serverbackup.KindX25519)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddServerBackupKey(ctx, store.ServerBackupKey{Label: "laptop", Recipient: k.Recipient.Line, Fingerprint: k.Recipient.Fingerprint, Kind: string(k.Recipient.Kind)}, "", 0); err != nil {
		t.Fatal(err)
	}
	st.Close()
	err = c.beforeUpdate(ctx, "v0.17.0")
	if err == nil || !strings.Contains(err.Error(), "a key is set") || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "start the service") {
		t.Fatalf("a key, the service refusing: %v", err)
	}
	c.api = nil
	if err := c.beforeUpdate(ctx, "v0.17.0"); err == nil || !strings.Contains(err.Error(), "no serve of") {
		t.Fatalf("a key, no service: %v", err)
	}
	st = storekit.Open(t, dir)
	defer st.Close()
	if bs, err := st.ServerBackups(ctx); err != nil || len(bs) != 0 {
		t.Fatalf("made as root: %+v %v", bs, err)
	}
}
