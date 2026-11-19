// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The template (AP TestSpeed, Block 9, D12-A): the migrations run once
// per test binary, and a test that is not about migrating opens a copy
// of what they made. internal/testkit/storekit holds the template of the
// other packages' tests; the store's own cannot import it, since it
// imports the store, and build theirs here on the same half of the kit
// (testkit.FileTemplate, testkit.WriteNew). docs/testing.md, "The
// store's template".

import (
	"fmt"
	"testing"

	"go.metaworx.ch/massimo/internal/testkit"
)

// open opens the store of dir and closes it at the test's end. Where dir
// has no massimo.db, its file is first a copy of the template
// (copyTemplate). A test about migrating opens it with openFresh.
func open(t *testing.T, dir string) *Store {
	t.Helper()
	copyTemplate(t, dir)
	return openFresh(t, dir)
}

// openFresh opens the store of dir as Open does, the migrations
// included, and closes it at the test's end.
func openFresh(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir, Options{Version: "test", Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// copied returns a new temporary directory that holds a copy of the
// template, for a test that opens it with options of its own.
func copied(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	copyTemplate(t, dir)
	return dir
}

// copyTemplate writes the template into dir as massimo.db, creating dir,
// where dir has no massimo.db; a file there is left as it is.
func copyTemplate(t testing.TB, dir string) {
	t.Helper()
	b, err := template()
	if err == nil {
		err = testkit.WriteNew(Path(dir), b)
	}
	if err != nil {
		t.Fatal(err)
	}
}

// template is the template's file, made once per test binary: a store
// the migrations make, closed, which brings the WAL into the file, and
// checked to be at this binary's schema version, which every copy's
// open would migrate otherwise.
var template = testkit.FileTemplate(FileName, func(dir string) error {
	s, err := Open(dir, Options{Version: "test", Logger: quiet()})
	if err != nil {
		return err
	}
	return s.Close()
}, func(dir string) error {
	ro, err := OpenReadOnly(dir, Options{Logger: quiet()})
	if err != nil {
		return fmt.Errorf("no migrated store: %w", err)
	}
	defer ro.Close()
	if v, _ := ro.SchemaVersion(); v != SchemaVersion() {
		return fmt.Errorf("at schema version %d, not this binary's %d", v, SchemaVersion())
	}
	return nil
})
