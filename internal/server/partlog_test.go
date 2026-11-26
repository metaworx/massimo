// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The lines of the parts a Cube's runtime starts carry its serial
// (AP StartConfiguration, Block 20): the Cube client's, the queue's and
// the proxy's, the plain Cube's once it reported its serial, and a
// stored Cube's from the start, each once.

import (
	"log/slog"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// partSources are the sources of the parts a runtime starts.
var partSources = map[string]bool{"cube": true, "queue": true, "proxy": true}

// checkPartLines has fc drop the connection, which the Cube client
// makes again, a room's setpoint go through the queue and a client
// connect to the proxy at proxyAddr, and checks that each of them
// wrote a line, and that every line of those parts since carries the
// Cube's serial, KEQ0000100, once.
func checkPartLines(t *testing.T, r *rig, fc *fakecube.Server, proxyAddr string) {
	t.Helper()
	mark := r.log.Mark()
	fc.Drop()
	_, c := r.log.Await(t, mark, "the client's line of the reconnect", testkit.Msg("cube connection re-established", "source", "cube"), awayWait)
	if code, out := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6/setpoint", `{"mode":"auto"}`); code != 200 {
		t.Fatalf("the setpoint: %d %v", code, out)
	}
	r.log.Await(t, c, "the queue's line of the command", testkit.Msg("command accepted by the cube", "source", "queue"))
	proxyClient(t, proxyAddr)
	r.log.Await(t, c, "the proxy's line of its client", testkit.Msg("proxy: client connected", "source", "proxy"))
	seen := map[string]bool{}
	for _, rec := range r.log.Since(mark, func(rec testkit.Record) bool { return partSources[rec.Attrs["source"]] }) {
		seen[rec.Attrs["source"]] = true
		if n := strings.Count(rec.Line, " cube="); n != 1 || rec.Attrs["cube"] != "KEQ0000100" {
			t.Errorf("a line with cube %d times, want KEQ0000100 once: %s", n, rec)
		}
	}
	if len(seen) != len(partSources) {
		t.Fatalf("lines of %v alone", seen)
	}
}

// The plain Cube's parts start before it reported its serial: their
// lines carry it once it has.
func TestPartLinesOfThePlainCube(t *testing.T) {
	t.Parallel()
	fc := &fakecube.Server{Greeting: recorded(t)}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fc.Close() })
	ports := &heldPorts{}
	proxyAddr := ports.hold(t)
	r := startServer(t, withLog(slog.LevelDebug), withOptions(Options{Proxy: []ProxyListener{{CubeAddr: addr, Addr: proxyAddr}}}), listenOn(ports),
		withCubes(CubeConfig{Addr: addr, Source: store.SourceEnv, SeedFromFile: true}))
	r.s.mu.RLock()
	rt := r.s.cubes[strings.ToLower(addr)]
	r.s.mu.RUnlock()
	if rt == nil {
		t.Fatalf("no runtime at %s", addr)
	}
	testkit.Eventually(t, "the plain Cube's serial", func() bool { return rt.cubeSerial() == "KEQ0000100" && rt.client.Connected() })
	checkPartLines(t, r, fc, proxyAddr)
}

// A stored Cube's parts know its serial from the start: their lines
// carry it once, not once from the start and once more as it is read.
func TestPartLinesOfAStoredCube(t *testing.T) {
	t.Parallel()
	ports := &heldPorts{}
	proxyAddr := ports.hold(t)
	r := startServer(t, stored(), withLog(slog.LevelDebug), withOptions(Options{Proxy: []ProxyListener{{Addr: proxyAddr}}}), listenOn(ports))
	checkPartLines(t, r, r.fc, proxyAddr)
}
