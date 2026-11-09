// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
)

// The owner of the service's settings file, without -env or
// MASSIMO_ENV_FILE, reads that file and keeps its data in the service's
// directory; -env, MASSIMO_ENV_FILE, root, another user and a box
// without the file keep ./.env and the current directory.
func TestServiceUserDefaults(t *testing.T) {
	uid := os.Geteuid()
	if uid == 0 {
		t.Skip("the test's files are root's, and root has no defaults")
	}
	for _, c := range []struct {
		name     string
		args     []string // the global options
		fileVar  string   // MASSIMO_ENV_FILE
		euid     int
		noFile   bool
		defaults bool
		file     string // env.File(), "" for the service's
	}{
		{name: "the service user", euid: uid, defaults: true},
		{name: "-env", args: []string{"-env", "given.env"}, euid: uid, file: "given.env"},
		{name: "MASSIMO_ENV_FILE", fileVar: "other.env", euid: uid, file: "other.env"},
		{name: "root", euid: 0, file: ".env"},
		{name: "another user", euid: uid + 1, file: ".env"},
		{name: "no service here", euid: uid, noFile: true, file: ".env"},
	} {
		t.Run(c.name, func(t *testing.T) {
			envPath, stateDir := serviceFiles(t)
			if c.noFile {
				if err := os.Remove(envPath); err != nil {
					t.Fatal(err)
				}
			}
			saved := euid
			euid = func() int { return c.euid }
			t.Cleanup(func() { euid = saved })
			t.Setenv(env.FileVar, c.fileVar)
			if _, err := globalOptions(append(c.args, "store", "info")); err != nil {
				t.Fatal(err)
			}
			useServiceDefaults()
			want, wantData := c.file, "."
			if c.defaults {
				want, wantData = envPath, stateDir
			}
			if serviceDefaults != c.defaults || env.File() != want || dataDir() != wantData {
				t.Fatalf("defaults %v, file %q, data %q; want %v, %q, %q", serviceDefaults, env.File(), dataDir(), c.defaults, want, wantData)
			}
		})
	}
}

// A settings file this user cannot see is no service user's.
func TestServiceUserUnseenFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root sees every file")
	}
	dir := filepath.Join(t.TempDir(), "closed")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "env")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	if serviceUser(p) {
		t.Fatal("a file in a directory closed to this user")
	}
}
