// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build unix

package main

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
)

// Run by root (the update helper), the binaries run as the owner of
// massimo.db, and nothing in the data directory is root's afterwards:
// root opens no store the service's user can write. Run as root only, in
// a container; the test's directories are opened up to 0755 so that the
// owner reaches the binaries.
func TestRollbackStoreRunsAsTheStoresOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	const owner = 65534 // nobody
	target, kept := massimoBinaries(t, true)
	dir := raisedStore(t)
	for _, d := range []string{filepath.Dir(target), dir} {
		for p := d; p != os.TempDir() && p != "/"; p = filepath.Dir(p) {
			if err := os.Chmod(p, 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	paths := []string{dir}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		paths = append(paths, filepath.Join(dir, e.Name()))
	}
	for _, p := range paths {
		if err := os.Chown(p, owner, owner); err != nil {
			t.Fatal(err)
		}
	}
	stoppedUnits(t)
	if _, err := rollbackStore(context.Background(), target, kept, false, ""); err != nil {
		t.Fatal(err)
	}
	entries, _ = os.ReadDir(dir)
	for _, e := range entries {
		fi, err := os.Lstat(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if st := fi.Sys().(*syscall.Stat_t); st.Uid != owner {
			t.Errorf("%s is owned by %d", e.Name(), st.Uid)
		}
	}
	if v, _, err := store.FileVersion(dir); err != nil || v != store.SchemaVersion() {
		t.Fatalf("store at %d: %v", v, err)
	}
}
