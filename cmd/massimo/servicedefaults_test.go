// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
)

// serviceFiles puts the service's settings file, owned by the user the
// test runs as, and its data directory into a directory of the test's
// own, unsets MASSIMO_ENV_FILE and MASSIMO_DATA, and puts the defaults
// back after the test.
func serviceFiles(t *testing.T) (envPath, stateDir string) {
	t.Helper()
	dir := t.TempDir()
	envPath, stateDir = filepath.Join(dir, "env"), filepath.Join(dir, "state")
	if err := os.WriteFile(envPath, []byte("MASSIMO_TEST_SERVICE=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(stateDir, 0o750); err != nil {
		t.Fatal(err)
	}
	savedEnv, savedState := serviceEnvPath, serviceStateDir
	serviceEnvPath, serviceStateDir = envPath, stateDir
	t.Setenv(env.FileVar, "")
	t.Setenv(env.DataVar, "")
	t.Cleanup(func() {
		serviceEnvPath, serviceStateDir = savedEnv, savedState
		serviceDefaults = false
		env.SetDefaultFile(".env")
	})
	return envPath, stateDir
}

// The data directory is -data or MASSIMO_DATA, then the service user's
// default, then the current directory.
func TestDataDirOrder(t *testing.T) {
	_, stateDir := serviceFiles(t)
	if d := dataDir(); d != "." {
		t.Fatalf("no default: %q", d)
	}
	serviceDefaults = true
	if d := dataDir(); d != stateDir {
		t.Fatalf("the service user's default: %q", d)
	}
	if _, err := globalOptions([]string{"-data", "given", "store", "info"}); err != nil {
		t.Fatal(err)
	}
	if d := dataDir(); d != "given" {
		t.Fatalf("-data over the default: %q", d)
	}
	t.Setenv(env.DataVar, "from-file")
	if d := dataDir(); d != "from-file" {
		t.Fatalf("MASSIMO_DATA over the default: %q", d)
	}
}

// config's two lines say when they are the service user's defaults.
func TestConfigLinesNameTheDefault(t *testing.T) {
	envPath, stateDir := serviceFiles(t)
	if l := envFileLine(); l != "env file:      .env (not found)" && l != "env file:      .env (loaded)" {
		t.Fatalf("without the default: %q", l)
	}
	if l := dataDirLine(); strings.Contains(l, "the service user's default") {
		t.Fatalf("without the default: %q", l)
	}
	serviceDefaults = true
	env.SetDefaultFile(envPath)
	if l, want := envFileLine(), "env file:      "+envPath+" (loaded; the service user's default, -env for another)"; l != want {
		t.Fatalf("env file:\n got %q\nwant %q", l, want)
	}
	if l := dataDirLine(); !strings.HasPrefix(l, "data dir:      "+stateDir+" (") || !strings.HasSuffix(l, "; the service user's default)") {
		t.Fatalf("data dir: %q", l)
	}
	t.Setenv(env.DataVar, "given")
	if l := dataDirLine(); !strings.HasPrefix(l, "data dir:      given (") || strings.Contains(l, "the service user's default") {
		t.Fatalf("data dir with MASSIMO_DATA: %q", l)
	}
}

// The usage texts' example runs this binary for the service: as the
// service user, or on Windows with the service's -env and -data.
func TestServiceExample(t *testing.T) {
	got := serviceExample("store", "info")
	if !strings.HasSuffix(got, " store info") {
		t.Fatalf("args: %q", got)
	}
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(got, `& "`) || !strings.Contains(got, `" -env `) || !strings.Contains(got, " -data ") {
			t.Fatalf("windows: %q", got)
		}
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if want := "sudo -u _massimo " + exe + " store info"; got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	if u := dataDirUsage("store", "info"); !strings.Contains(u, "the service user, without -env, reads") || !strings.HasSuffix(u, "  "+got+"\n") {
		t.Fatalf("usage paragraph: %q", u)
	}
}

// Windows has no service user: its service runs as LocalSystem.
func TestNoServiceUserOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows only")
	}
	serviceFiles(t)
	useServiceDefaults()
	if serviceDefaults || env.File() != ".env" {
		t.Fatalf("defaults on Windows: %v %q", serviceDefaults, env.File())
	}
}
