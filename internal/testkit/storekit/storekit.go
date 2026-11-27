// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package storekit is the test kit's store: the migrations run once per
// test binary, into a template, and every test opens a copy of it in
// place of a store the migrations make again. Under the race detector
// a fresh store costs most of a second, much of it under the one lock
// of SQLite's allocator, so the tests that open one wait for each
// other; a copy costs a few milliseconds. It is a package of its own
// because it needs the store, whose tests import the kit; the template's
// half that needs no store is the kit's (testkit.FileTemplate,
// testkit.WriteNew), on which the store's own tests build their
// template too (internal/store's template_test.go).
//
// Only test files import the package (testkit's
// TestOnlyTestsImportTheKit). docs/testing.md says when a test opens a
// fresh store instead.
package storekit

import (
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// version is the massimo version the template's migrations record, and
// the one each copy is opened with.
const version = "test"

// quiet is the log of the template's store and of each copy's.
var quiet = slog.New(slog.DiscardHandler)

// Open opens the store of dir, of a new temporary directory where dir is
// "", and closes it at the test's end. Where dir has no massimo.db, its
// file is first a copy of the template. Open makes node.key and
// opaque.key in dir as for a fresh store; node.id and tenant.id, which
// the file holds, are the template's in every copy.
func Open(t testing.TB, dir string) *store.Store {
	t.Helper()
	if dir == "" {
		dir = t.TempDir()
	}
	b, err := template()
	if err == nil {
		err = testkit.WriteNew(store.Path(dir), b)
	}
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, store.Options{Version: version, Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// Raw opens a second connection to the file of st, past the store, as
// another process holds one: for a test that writes what the store never
// writes, or holds the file's lock while the store works. It waits for a
// lock as the store does, and is closed at the test's end.
func Raw(t testing.TB, st *store.Store) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(st.File())+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// template is the template's file, made once per test binary.
var template = testkit.FileTemplate(store.FileName, build, check)

// build makes the template: a store the migrations make, closed, which
// brings the WAL into the file.
func build(dir string) error {
	st, err := store.Open(dir, store.Options{Version: version, Logger: quiet})
	if err != nil {
		return err
	}
	return st.Close()
}

// check refuses a template that is not at this binary's schema version,
// which every copy's open would migrate.
func check(dir string) error {
	ro, err := store.OpenReadOnly(dir, store.Options{Logger: quiet})
	if err != nil {
		return fmt.Errorf("no migrated store: %w", err)
	}
	defer ro.Close()
	if v, _ := ro.SchemaVersion(); v != store.SchemaVersion() {
		return fmt.Errorf("at schema version %d, not this binary's %d", v, store.SchemaVersion())
	}
	return nil
}
