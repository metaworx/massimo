// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
	"go.metaworx.ch/massimo/internal/view"
)

// testStore opens a store in a temporary directory, or in dir: a copy of
// the migrated template where dir has no store yet (storekit.Open).
func testStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	return storekit.Open(t, dir)
}

// The stale threshold is the server's, not the process's.
func TestStaleAfterPerServer(t *testing.T) {
	t.Parallel()
	a := New(Options{StaleAfter: 36 * time.Hour})
	b := New(Options{})
	if a.StaleAfter() != 36*time.Hour || b.StaleAfter() != view.StaleAfter {
		t.Fatalf("%v %v", a.StaleAfter(), b.StaleAfter())
	}
	b.SetStaleAfter(2 * time.Hour)
	if a.StaleAfter() != 36*time.Hour {
		t.Fatal("one server's threshold changed the other's")
	}
}

// A Cube discovery found is a row of the store: after a restart of the
// server it is listed again once a run has heard it, not before, and
// not announced a second time.
func TestFoundCubeSurvivesRestart(t *testing.T) {
	t.Parallel()
	other, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	dir := t.TempDir()
	st := testStore(t, dir)
	first := startServer(t, discovering(Options{Store: st, DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{other.Addr()}}))
	ts := first.ts
	if code, out := postDiscovery(t, ts); code != 200 || len(out["found"].([]any)) != 1 {
		t.Fatalf("discover %d %v", code, out)
	}
	row, ok, err := st.Cube(t.Context(), "KEQ0000000")
	if err != nil || !ok || row.Configured || row.Source != store.SourceDiscovery || row.DiscoveredAddress != "127.0.0.1" || row.RFAddress != "0a0b0c" {
		t.Fatalf("row %+v %v %v", row, ok, err)
	}
	first.stop() // the API, then Run, before its store closes
	st.Close()

	// The restarted server lists it once a run has heard it, and that
	// run sends no cube_found.
	st = testStore(t, dir)
	ts = startServer(t, discovering(Options{Store: st, DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{other.Addr()}})).ts
	var cubes []map[string]any
	if getJSON(t, ts, "", "/api/v1/cubes", &cubes) != 200 || len(cubes) != 1 {
		t.Fatalf("cubes after the restart, before a run %v", cubes)
	}
	postDiscovery(t, ts)
	if getJSON(t, ts, "", "/api/v1/cubes", &cubes) != 200 || len(cubes) != 2 || cubes[1]["serial"] != "KEQ0000000" || cubes[1]["configured"] != false || cubes[1]["found_at"] == nil {
		t.Fatalf("cubes after the restart and a run %v", cubes)
	}
	var evs []event
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log", &evs)
	for _, e := range evs {
		if e.Kind == "cube_found" {
			t.Fatalf("announced again after the restart: %+v", e)
		}
	}
}
