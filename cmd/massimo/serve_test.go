// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/env"
)

// clearCubeEnv unsets every MASSIMO_CUBE_* variable for the test, and
// points the settings file and the data directory away from the
// checkout's.
func clearCubeEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, env.Prefix) {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}
	t.Setenv(env.FileVar, os.DevNull)
	t.Setenv(env.DataVar, t.TempDir())
	selectedSN, snExplicit = "", false
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func startFakeCube(t *testing.T) string {
	t.Helper()
	greeting, err := fakecube.LoadGreeting("../../internal/protocol/cube/testdata/cube-fw0113-greeting.txt")
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	cubeAddr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fc.Close() })
	return cubeAddr
}

func freeAddr(t *testing.T) string {
	t.Helper()
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer free.Close()
	return free.Addr().String()
}

// dataDir follows -data, then MASSIMO_DATA, then the current directory.
func TestDataDir(t *testing.T) {
	t.Setenv(env.DataVar, "")
	os.Unsetenv(env.DataVar)
	if d := dataDir(); d != "." {
		t.Fatalf("default %q", d)
	}
	t.Setenv(env.DataVar, "/srv/massimo")
	if d := dataDir(); d != "/srv/massimo" {
		t.Fatalf("MASSIMO_DATA %q", d)
	}
	if _, err := globalOptions([]string{"-data", "/var/lib/massimo", "serve"}); err != nil || dataDir() != "/var/lib/massimo" {
		t.Fatalf("-data %q %v", dataDir(), err)
	}
}
