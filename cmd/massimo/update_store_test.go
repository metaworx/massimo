// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// runMainVar makes the test binary run massimo's main with its
// arguments, so that a test runs the command line as a binary of its
// own, also where there is no go to build one (the Windows test binary
// through WSL's interop).
const runMainVar = "MASSIMO_TEST_RUN_MAIN"

func TestMain(m *testing.M) {
	if os.Getenv(runMainVar) == "1" {
		main()
		os.Exit(0)
	}
	code := m.Run()
	if built.dir != "" {
		os.RemoveAll(built.dir)
	}
	os.Exit(code)
}

// massimoBinaries copies the test binary, as massimo, to a directory of
// its own and returns it and its kept name, massimo.failed (on Windows
// massimo.exe and massimo.exe.failed); kept is made only with withKept.
// With -short it skips the test: the armv6 job runs the test binary
// under QEMU, which cannot start a copy of it by itself.
func massimoBinaries(t *testing.T, withKept bool) (target, kept string) {
	t.Helper()
	if testing.Short() {
		t.Skip("runs copies of the test binary")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	target = filepath.Join(t.TempDir(), "massimo")
	if runtime.GOOS == "windows" {
		target += ".exe"
	}
	kept = target + ".failed"
	for _, p := range []string{target, kept} {
		if p == kept && !withKept {
			break
		}
		if err := os.WriteFile(p, b, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(runMainVar, "1")
	return target, kept
}

// raisedStore makes a store at this binary's schema version in a
// directory of its own, named by MASSIMO_DATA, and moves it on as a
// later binary's raising migration would: a table of its own, a
// schema_version row whose min_reader this binary does not read, and
// that row's down part. It returns the directory.
func raisedStore(t *testing.T) string {
	t.Helper()
	testkit.Swapping(t)
	dir := t.TempDir()
	t.Setenv(env.DataVar, dir)
	old := serviceStateDir
	serviceStateDir = filepath.Join(t.TempDir(), "none")
	t.Cleanup(func() { serviceStateDir = old })
	st, err := store.Open(dir, store.Options{Logger: quietLog()})
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(store.Path(dir)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	next := store.SchemaVersion() + 1
	if _, err := db.Exec(`CREATE TABLE later_notes (id INTEGER PRIMARY KEY, text TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_version (version, min_reader, applied_at, applied_by, down_sql) VALUES (?, ?, '2026-11-16T00:00:00.000Z', 'later', 'DROP TABLE later_notes;')`, next, next); err != nil {
		t.Fatal(err)
	}
	return dir
}

// stoppedUnits records what stopService was asked to stop.
func stoppedUnits(t *testing.T) *[]string {
	t.Helper()
	testkit.Swapping(t)
	var units []string
	old := stopService
	stopService = func(name string) error { units = append(units, name); return nil }
	t.Cleanup(func() { stopService = old })
	return &units
}

// checkTakenBack fails unless the store in dir is at this binary's
// version again, with a min_reader it reads, and opens.
func checkTakenBack(t *testing.T, dir string) {
	t.Helper()
	v, mr, err := store.FileVersion(dir)
	if err != nil || v != store.SchemaVersion() || mr > store.SchemaVersion() {
		t.Fatalf("store at %d (min_reader %d) %v", v, mr, err)
	}
	st, err := store.Open(dir, store.Options{Logger: quietLog()})
	if err != nil {
		t.Fatalf("this binary does not open the store: %v", err)
	}
	st.Close()
}

// The binary put back says it cannot open the store; the service is
// stopped, and the kept binary, under its kept name (on Windows
// massimo.exe.failed), takes the store back. Then there is nothing to
// do.
func TestRollbackStoreByTheKeptBinary(t *testing.T) {
	target, kept := massimoBinaries(t, true)
	dir := raisedStore(t)
	units := stoppedUnits(t)
	msg, err := rollbackStore(context.Background(), target, kept, false, "massimo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "by "+kept) || strings.Join(*units, ",") != "massimo" {
		t.Fatalf("message %q, stopped %v", msg, *units)
	}
	checkTakenBack(t, dir)
	if _, err := os.Stat(store.Path(dir) + ".before-downgrade-" + strconv.Itoa(store.SchemaVersion()+1)); err != nil {
		t.Fatalf("no copy: %v", err)
	}

	*units = nil
	if msg, err := rollbackStore(context.Background(), target, kept, false, "massimo"); err != nil || msg != "" || len(*units) != 0 {
		t.Fatalf("a second time: %q %v, stopped %v", msg, err, *units)
	}
}

// After a failed health check the binary put back is this one: where the
// kept binary is gone, it takes the store back by the store's own down
// part. Where it is not this one, the rollback says what to run.
func TestRollbackStoreByTheStoresDownParts(t *testing.T) {
	target, kept := massimoBinaries(t, false)
	dir := raisedStore(t)
	units := stoppedUnits(t)
	_, err := rollbackStore(context.Background(), target, kept, false, "massimo.service")
	if err == nil || !strings.Contains(err.Error(), "store downgrade -to "+strconv.Itoa(store.SchemaVersion())) {
		t.Fatalf("without the kept binary, not this one: %v", err)
	}
	msg, err := rollbackStore(context.Background(), target, kept, true, "massimo.service")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "by the down parts it keeps") || strings.Join(*units, ",") != "massimo.service,massimo.service" {
		t.Fatalf("message %q, stopped %v", msg, *units)
	}
	checkTakenBack(t, dir)
}

// No store, nothing to do.
func TestRollbackStoreWithoutStore(t *testing.T) {
	t.Setenv(env.DataVar, t.TempDir())
	units := stoppedUnits(t)
	if msg, err := rollbackStore(context.Background(), "massimo", "massimo.failed", true, "massimo.service"); err != nil || msg != "" || len(*units) != 0 {
		t.Fatalf("%q %v, stopped %v", msg, err, *units)
	}
}

// massimo store downgrade -to <N> takes the store back; it needs -to.
func TestStoreDowngradeCommand(t *testing.T) {
	dir := raisedStore(t)
	if err := runStore([]string{"downgrade"}); err == nil {
		t.Fatal("downgrade without -to")
	}
	if err := runStore([]string{"downgrade", "-to", strconv.Itoa(store.SchemaVersion())}); err != nil {
		t.Fatal(err)
	}
	checkTakenBack(t, dir)
	if err := runStore([]string{"downgrade", "-to", strconv.Itoa(store.SchemaVersion())}); err != nil {
		t.Fatalf("nothing to do: %v", err)
	}
}
