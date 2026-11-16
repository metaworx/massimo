// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeMassimoVar makes the test binary act as a massimo that prints the
// arguments it was given; its value is what store info answers:
// "reads" prints the line of a store it reads, "refuses" the refusal of
// one too new, "fails" exits 1 with nothing to say.
const fakeMassimoVar = "MASSIMO_UPDATE_TEST_FAKE"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeMassimoVar); mode != "" {
		os.Exit(fakeMassimo(mode, os.Args[1:]))
	}
	os.Exit(m.Run())
}

// fakeMassimo is the stand-in binary: it checks that it reads no .env,
// prints its arguments and answers store info as mode says.
func fakeMassimo(mode string, args []string) int {
	if f := os.Getenv("MASSIMO_ENV_FILE"); f == "" {
		fmt.Println("no MASSIMO_ENV_FILE")
		return 2
	} else if _, err := os.Stat(f); err == nil {
		fmt.Println("MASSIMO_ENV_FILE exists:", f)
		return 2
	}
	fmt.Println("args:", strings.Join(args, " "))
	if len(args) == 4 && args[2] == "store" && args[3] == "info" {
		switch mode {
		case "reads":
			fmt.Println("schema version: 23 (min_reader 1; this binary: 21)")
		case "refuses":
			fmt.Fprintln(os.Stderr, "store: file refused: massimo.db is at schema version 24 and needs a binary for version 24 or later; this one is for version 22. Install the newer release again")
			return 1
		default:
			return 1
		}
	}
	return 0
}

// keptBinary copies the test binary to dir as a binary a rollback keeps
// aside, massimo.failed (on Windows massimo.exe.failed, which Windows
// would not run by that name). With -short it skips the test: the armv6
// job runs the test binary under QEMU, which cannot start a copy of it
// by itself.
func keptBinary(t *testing.T, dir string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("runs a copy of the test binary")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "massimo")
	if runtime.GOOS == "windows" {
		target += ".exe"
	}
	kept := FailedPath(target)
	if err := copyFile(self, kept); err != nil {
		t.Fatal(err)
	}
	return kept
}

// The kept binary's downgrade runs with the store's directory and the
// target version, reading no .env; on Windows through a link named .exe,
// which is gone afterwards.
func TestStoreDowngradeRunsTheKeptBinary(t *testing.T) {
	dir := t.TempDir()
	kept := keptBinary(t, dir)
	t.Setenv(fakeMassimoVar, "reads")
	data := filepath.Join(dir, "data")
	owner := "" // run as this process (TestRunsAsTheStoresOwner)
	out, err := StoreDowngrade(context.Background(), kept, data, owner, 21)
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if want := "args: -data " + data + " store downgrade -to 21"; !strings.Contains(out, want) {
		t.Fatalf("output %q, want %q", out, want)
	}
	if _, err := os.Stat(filepath.Join(dir, ".massimo.run.exe")); !os.IsNotExist(err) {
		t.Fatalf("the .exe link is left: %v", err)
	}
	if _, err := StoreDowngrade(context.Background(), filepath.Join(dir, "gone"), data, owner, 21); err == nil {
		t.Fatal("a missing binary ran")
	}
}

// StoreSchema reads the binary's schema version from its store info, and
// whether it reads the store, also from its refusal of a store too new
// for it; a binary that says neither is an error.
func TestStoreSchema(t *testing.T) {
	dir := t.TempDir()
	kept := keptBinary(t, dir)
	for mode, want := range map[string]struct {
		schema int
		reads  bool
	}{"reads": {21, true}, "refuses": {22, false}} {
		t.Setenv(fakeMassimoVar, mode)
		schema, reads, err := StoreSchema(context.Background(), kept, dir, "")
		if err != nil || schema != want.schema || reads != want.reads {
			t.Errorf("%s: %d %v %v, want %+v", mode, schema, reads, err, want)
		}
	}
	t.Setenv(fakeMassimoVar, "fails")
	if _, _, err := StoreSchema(context.Background(), kept, dir, ""); err == nil {
		t.Fatal("a binary without a schema version answered")
	}
}
