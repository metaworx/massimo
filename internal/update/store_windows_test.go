// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package update

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// A .massimo.run.exe beside the kept binary that cannot be removed, here
// a directory with a file in it, is named as the cause of StoreSchema's
// error, not hidden behind the copy's error that follows (AP
// StartConfiguration v1.3, Block 28, D68-A).
func TestStoreSchemaNamesTheRunLinkNotRemoved(t *testing.T) {
	dir := t.TempDir()
	kept := keptBinary(t, dir)
	run := filepath.Join(dir, ".massimo.run.exe")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run, "in-the-way"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(fakeMassimoVar, "reads")
	_, _, err := StoreSchema(context.Background(), kept, dir, "")
	var pe *fs.PathError
	if !errors.As(err, &pe) || pe.Op != "remove" || pe.Path != run {
		t.Fatalf("error %v, want the removal of %s named", err, run)
	}
}
