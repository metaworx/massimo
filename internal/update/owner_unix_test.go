// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build unix

package update

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Run by root, a kept binary runs as the owner of the store's file, not
// followed where it is a link; as root where root owns it. Run as root
// only, in a container.
func TestRunsAsTheStoresOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	dir := t.TempDir()
	for p := dir; p != os.TempDir() && p != "/"; p = filepath.Dir(p) {
		if err := os.Chmod(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(dir, "massimo.failed")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nid -u\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(dir, "massimo.db")
	if err := os.WriteFile(db, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.db")
	if err := os.Symlink(db, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Lchown(link, 65534, 65534); err != nil {
		t.Fatal(err)
	}
	for owner, want := range map[string]string{db: "0", link: "65534"} {
		out, err := StoreDowngrade(context.Background(), bin, dir, owner, 1)
		if err != nil || strings.TrimSpace(out) != want {
			t.Errorf("owner %s: ran as %q (%v), want %s", filepath.Base(owner), out, err, want)
		}
	}
	if err := os.Chown(db, 65534, 65534); err != nil {
		t.Fatal(err)
	}
	if out, err := StoreDowngrade(context.Background(), bin, dir, db, 1); err != nil || strings.TrimSpace(out) != "65534" {
		t.Errorf("ran as %q (%v), want 65534", out, err)
	}
}
