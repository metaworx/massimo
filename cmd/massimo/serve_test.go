// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/server"
)

// clearCubeEnv unsets every MASSIMO_CUBE_* variable for the test.
func clearCubeEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, env.Prefix) {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}
	selectedSN, snExplicit = "", false
}

// A single Cube given only as MASSIMO_CUBE_ADDRESS, without a serial, is
// served with its proxy; the serial comes from the Cube's greeting.
func TestServeSingleCubeWithoutSerial(t *testing.T) {
	clearCubeEnv(t)
	greeting, err := fakecube.LoadGreeting("../../internal/protocol/cube/testdata/cube-fw0113-greeting.txt")
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	cubeAddr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer fc.Close()
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxyAddr := free.Addr().String()
	free.Close()
	envFile := filepath.Join(t.TempDir(), "x.env")
	os.WriteFile(envFile, []byte("MASSIMO_CUBE_ADDRESS="+cubeAddr+"\nMASSIMO_CUBE_PROXY="+proxyAddr+"\n"), 0o600)
	for _, k := range []string{env.Address, env.Proxy} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	if err := env.LoadFile(envFile); err != nil {
		t.Fatal(err)
	}

	cubes, listeners, err := serveTargets("", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cubes) != 1 || cubes[0].Serial != "" || cubes[0].Addr != cubeAddr || len(listeners) != 1 || listeners[0].Addr != proxyAddr || listeners[0].Serial != "" {
		t.Fatalf("cubes %+v listeners %+v", cubes, listeners)
	}
	// Without a proxy address the single Cube listens on the Cube port.
	os.Unsetenv(env.Proxy)
	if _, l, err := serveTargets("", nil, ""); err != nil || len(l) != 1 || l[0].Addr != ":62910" || !l[0].Discovery {
		t.Fatalf("default listener %+v %v", l, err)
	}

	srv := server.New(server.Options{Cubes: cubes, Proxy: listeners})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	var conn net.Conn
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if conn, err = net.Dial("tcp", proxyAddr); err == nil {
			break
		}
	}
	if conn == nil {
		t.Fatal("proxy never listened")
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	m, err := cube.NewReader(conn).Read()
	if err != nil || m.Type != 'H' || !strings.HasPrefix(m.Data, "KEQ0537249,") {
		t.Fatalf("greeting %v %v", m, err)
	}
}

// With several Cubes, a proxy listener without a serial is refused.
func TestServeSeveralCubesNeedNamedListeners(t *testing.T) {
	clearCubeEnv(t)
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	t.Setenv("MASSIMO_CUBE_KEQ0000002_ADDRESS", "10.0.0.2")
	if _, _, err := serveTargets("", []string{":62910"}, ""); err == nil || !strings.Contains(err.Error(), "several cubes") {
		t.Fatalf("unnamed listener with two cubes: %v", err)
	}
	if _, _, err := serveTargets("", nil, ""); err == nil || !strings.Contains(err.Error(), "KEQ0000001_PROXY") {
		t.Fatalf("two cubes without proxy addresses: %v", err)
	}
	t.Setenv("MASSIMO_CUBE_KEQ0000001_PROXY", "10.0.0.11")
	t.Setenv("MASSIMO_CUBE_KEQ0000002_PROXY", "10.0.0.12")
	cubes, l, err := serveTargets("", nil, "")
	if err != nil || len(cubes) != 2 || len(l) != 2 || l[0].Serial != "KEQ0000001" || l[1].Addr != "10.0.0.12:62910" {
		t.Fatalf("cubes %+v listeners %+v %v", cubes, l, err)
	}
}

// Cubes with serve's own variables but no address are served by serial,
// their address left to discovery; so is a Cube selected with -sn alone.
func TestServeCubesConfiguredBySerial(t *testing.T) {
	clearCubeEnv(t)
	t.Cleanup(func() { selectedSN, snExplicit = "", false })
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	t.Setenv("MASSIMO_CUBE_KEQ0000002_NAME", "Attic")
	t.Setenv("MASSIMO_CUBE_KEQ0000003_PROXY", "10.0.0.13")
	t.Setenv("MASSIMO_CUBE_KEQ0000004_KEY", "k0123") // a key alone does not make serve take the Cube
	t.Setenv("MASSIMO_CUBE_KEQ0000001_PROXY", "10.0.0.11")
	t.Setenv("MASSIMO_CUBE_KEQ0000002_PROXY", "10.0.0.12")
	cubes, _, err := serveTargets("", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cubes) != 3 || cubes[0].Addr != "10.0.0.1" || cubes[1].Serial != "KEQ0000002" || cubes[1].Addr != "" || cubes[1].Name != "Attic" || cubes[2].Serial != "KEQ0000003" || cubes[2].Addr != "" {
		t.Fatalf("cubes %+v", cubes)
	}

	clearCubeEnv(t)
	selectedSN, snExplicit = "keq0000009", true
	cubes, _, err = serveTargets("", nil, "")
	if err != nil || len(cubes) != 1 || cubes[0].Serial != "KEQ0000009" || cubes[0].Addr != "" {
		t.Fatalf("-sn alone: %+v %v", cubes, err)
	}
}
