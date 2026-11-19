// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package storekit

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// Copies are at this binary's schema version and share nothing but the
// template's identity; a store already in the directory is opened as it
// is, as a restart's.
func TestCopiesShareNothing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	a, other := Open(t, dir), Open(t, "")
	for _, st := range []*store.Store{a, other} {
		if v, _ := st.SchemaVersion(); v != store.SchemaVersion() {
			t.Fatalf("schema version %d, want %d", v, store.SchemaVersion())
		}
	}
	if _, err := a.AddCube(ctx, store.Cube{Serial: "KEQ0000001", Configured: true}); err != nil {
		t.Fatal(err)
	}
	if cubes, err := other.Cubes(ctx); err != nil || len(cubes) != 0 {
		t.Fatalf("one copy's row in another: %+v %v", cubes, err)
	}
	if bytes.Equal(a.NodePublicKey(), other.NodePublicKey()) {
		t.Fatal("two copies share a node key")
	}
	if a.NodeID() != other.NodeID() {
		t.Fatal("two copies differ in node.id, which the template holds")
	}
	a.Close()
	if cubes, err := Open(t, dir).Cubes(ctx); err != nil || len(cubes) != 1 {
		t.Fatalf("the store opened again: %+v %v", cubes, err)
	}
}

// A template whose store is still open, its WAL holding what the file
// lacks, and a file the migrations did not make are refused.
func TestTemplateChecked(t *testing.T) {
	open := func(dir string) error {
		st, err := store.Open(dir, store.Options{Version: version, Logger: quiet})
		if err != nil {
			return err
		}
		t.Cleanup(func() { st.Close() })
		_, err = st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000001", Configured: true})
		return err
	}
	if _, err := testkit.FileTemplate(store.FileName, open, check)(); err == nil || !strings.Contains(err.Error(), "-wal behind") {
		t.Fatalf("a store still open: %v", err)
	}
	empty := func(dir string) error { return os.WriteFile(store.Path(dir), nil, 0o600) }
	if _, err := testkit.FileTemplate(store.FileName, empty, check)(); err == nil || !strings.Contains(err.Error(), "no migrated store") {
		t.Fatalf("an empty file: %v", err)
	}
}
