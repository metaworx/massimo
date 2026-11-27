// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
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
