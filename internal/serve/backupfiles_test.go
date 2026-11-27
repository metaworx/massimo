// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serve

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.metaworx.ch/massimo/internal/certs"
	"go.metaworx.ch/massimo/internal/serverbackup"
)

// A server backup carries the TLS files of the mode serve was started
// with, each with its role, those there now alone.
func TestBackupTLSFiles(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	write := func(p string) string {
		t.Helper()
		os.MkdirAll(filepath.Dir(p), 0o700)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cert, key := write(filepath.Join(data, "own", "cert.pem")), write(filepath.Join(data, "own", "key.pem"))
	if got := BackupTLSFiles(TLS{Cert: cert, Key: key}, data); !slices.Equal(got, []serverbackup.TLSFile{{Role: serverbackup.RoleTLSCert, Path: cert}, {Role: serverbackup.RoleTLSKey, Path: key}}) {
		t.Fatalf("files %v", got)
	}
	if got := BackupTLSFiles(TLS{Mode: "self-signed"}, data); len(got) != 0 {
		t.Fatalf("self-signed before its pair: %v", got)
	}
	sc, sk := certs.SelfSignedPaths(data)
	write(sc)
	write(sk)
	if got := BackupTLSFiles(TLS{Mode: "self-signed"}, data); len(got) != 2 || got[0].Path != sc || got[1].Role != serverbackup.RoleTLSKey {
		t.Fatalf("self-signed %v", got)
	}
	dir, _ := certs.ACMEPaths(data, "")
	acct, crt, k := write(filepath.Join(dir, certs.AccountKeyFile)), write(filepath.Join(dir, "box.example.pem")), write(filepath.Join(dir, "box.example.key"))
	write(filepath.Join(dir, ".tmp-123"))
	got := BackupTLSFiles(TLS{Mode: "acme"}, data)
	want := []serverbackup.TLSFile{{Role: serverbackup.RoleACMEAccount, Path: acct}, {Role: serverbackup.RoleACMEKey, Path: k}, {Role: serverbackup.RoleACMECert, Path: crt}}
	if !slices.Equal(got, want) {
		t.Fatalf("acme %v, want %v", got, want)
	}
	if got := BackupTLSFiles(TLS{}, data); got != nil {
		t.Fatalf("off %v", got)
	}
}
