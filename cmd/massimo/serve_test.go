// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/testkit"
)

// clearCubeEnv unsets every MASSIMO_CUBE_* variable for the test, and
// points the settings file and the data directory away from the
// checkout's.
func clearCubeEnv(t *testing.T) {
	t.Helper()
	testkit.Swapping(t)
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

// freeAddr returns a loopback address whose port was free a moment ago;
// another process can take it before serve listens on it. It stays for
// what serve takes as an address alone, in the test's process or in a
// child: -proxy, whose listener the log names by the address it was
// given, and the Windows service test's -listen (docs/testing.md,
// "Ports").
func freeAddr(t *testing.T) string {
	t.Helper()
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer free.Close()
	return free.Addr().String()
}

// A listen address the update helper cannot ask /healthz at, port 0 or
// a service name, blocks updates from the API with the reason, naming
// the setting, and the start goes on: serveUpdateHelper returns no
// error (AP StartConfiguration, D21-A). A readable one prepares the
// helper as before, which only Windows starts.
func TestServeUpdateHelperListen(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	req, status := filepath.Join(dir, "update-request"), filepath.Join(dir, "update-status.json")
	for _, listen := range []string{"127.0.0.1:0", ":0", "127.0.0.1:http", "[::1]:http"} {
		log, logs := testkit.LogWatch(nil)
		start, blocked, err := serveUpdateHelper(log, listen, false, req, status)
		if err != nil {
			t.Errorf("%s: the start stops with %v", listen, err)
			continue
		}
		want := fmt.Sprintf("-listen or MASSIMO_LISTEN: the update helper cannot ask %q for /healthz: bad port", listen)
		if start != nil || blocked != want {
			t.Errorf("%s: start %v, blocked %q; want no start, blocked %q", listen, start != nil, blocked, want)
		}
		logs.Has(t, 0, listen+": the reason in the log", testkit.MsgAt(slog.LevelWarn, "updates cannot be started from the API", "reason", want))
	}
	start, blocked, err := serveUpdateHelper(quietLog(), "127.0.0.1:8099", true, req, status)
	switch {
	case err != nil:
		t.Fatalf("a readable address: %v", err)
	case runtime.GOOS == "windows" && (start == nil || blocked != ""):
		t.Fatalf("a readable address: start %v, blocked %q", start != nil, blocked)
	case runtime.GOOS != "windows" && (start != nil || blocked != ""):
		t.Fatalf("a readable address outside Windows: start %v, blocked %q; the operating system starts the helper", start != nil, blocked)
	}
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
