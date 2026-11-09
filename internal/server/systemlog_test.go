// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The server's own lines (AP Logging v1.1, Block 5): its start and stop,
// the settings file read again, an update's outcome, the backups, who
// asked for them, a Cube's new address and its leaving the LAN.

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/update"
)

// runOver starts a server of a version over a store and a Cube's row,
// and returns its HTTP server and its stop, which waits for its Run.
func runOver(t *testing.T, st *store.Store, row store.Cube, opts Options) (*httptest.Server, func()) {
	t.Helper()
	cfg, _ := CubeFromRow(row)
	opts.Cubes, opts.Store = []CubeConfig{cfg}, st
	opts.Queue = engine.Options{Gap: time.Millisecond}
	opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(opts)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	ts := httptest.NewServer(s.Handler())
	waitConnected(t, ts, "KEQ0000100")
	return ts, func() { ts.Close(); cancel(); <-done }
}

// systemRows reads the server's rows of a kind from the store.
func systemRows(t *testing.T, st *store.Store, kind string) []map[string]any {
	t.Helper()
	if err := st.FlushEvents(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, err := st.Events(context.Background(), store.EventQuery{Kind: kind, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, r := range rows {
		var m map[string]any
		json.Unmarshal(r.Data, &m)
		out = append(out, m)
	}
	return out
}

// The start says the version, and the one before when it changed; the
// stop says why, written before the store closes.
func TestServerStartAndStopLogged(t *testing.T) {
	greeting, _ := fakecube.LoadGreeting(fixture)
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer fc.Close()
	st := testStore(t, "")
	row, _ := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv})
	_, stop := runOver(t, st, row, Options{Version: "v1.0.0"})
	stop()
	_, stop = runOver(t, st, row, Options{Version: "v1.1.0"})
	stop()
	started := systemRows(t, st, "server_started")
	if len(started) != 2 {
		t.Fatalf("%d starts", len(started))
	}
	newest, _ := started[0]["summary"].(map[string]any)
	oldest, _ := started[1]["summary"].(map[string]any)
	if newest["version"] != "v1.1.0" || newest["before"] != "v1.0.0" || oldest["version"] != "v1.0.0" || oldest["before"] != nil {
		t.Fatalf("the starts' summaries: %v, %v", newest, oldest)
	}
	stopping := systemRows(t, st, "server_stopping")
	if len(stopping) != 2 {
		t.Fatalf("%d stops", len(stopping))
	}
	if sum, _ := stopping[0]["summary"].(map[string]any); sum["why"] != "stop" || stopping[0]["source"] != "system" {
		t.Fatalf("the stop: %v", stopping[0])
	}
}

// The settings file read again names the variables, never their values;
// read through the API, it says who asked.
func TestSettingsReadNamesOnly(t *testing.T) {
	_, ts, s, st := startStored(t, Options{})
	token := userToken(t, st, "anna", store.RoleOwner)
	s.SetReloader("/etc/massimo/massimo.env", func(ctx context.Context) (ReloadResult, error) {
		s.LogSettingsRead(ctx, "the API", []string{"MASSIMO_LOG_LEVEL"}, []string{"MASSIMO_CUBE_KEQ0000000_ADDRESS"})
		return ReloadResult{Changed: []string{"MASSIMO_LOG_LEVEL"}, Pending: []string{}, ReadAt: time.Now()}, nil
	})
	if code, body := callAPI(t, ts, "POST", "/api/v1/reload", token, ""); code != 200 {
		t.Fatalf("reload: %d %s", code, body)
	}
	var evs []event
	getJSON(t, ts, token, "/api/v1/cubes/KEQ0000100/log?kind=settings_read", &evs)
	if len(evs) != 1 || evs[0].By == nil || evs[0].By.Name != "anna" || evs[0].Source != "system" {
		t.Fatalf("the reload's line: %+v", evs)
	}
	sum := evs[0].Summary
	changed, _ := sum["changed"].([]any)
	pending, _ := sum["at_next_start"].([]any)
	if sum["why"] != "the API" || len(changed) != 1 || changed[0] != "MASSIMO_LOG_LEVEL" || len(pending) != 1 || len(sum) != 3 {
		t.Fatalf("the summary: %v", sum)
	}
}

// An update's outcome the helper reported is logged once, at the first
// start after it, a failure at warn.
func TestUpdateResultLoggedOnce(t *testing.T) {
	greeting, _ := fakecube.LoadGreeting(fixture)
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer fc.Close()
	status := filepath.Join(t.TempDir(), "status.json")
	if err := update.WriteStatus(status, update.Status{State: "failed", From: "v1.0.0", To: "v1.1.0", Message: "checksum", Time: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	st := testStore(t, "")
	row, _ := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv})
	for i := 0; i < 2; i++ {
		_, stop := runOver(t, st, row, Options{Version: "v1.0.0", Update: UpdateOptions{Status: status}})
		stop()
	}
	rows := systemRows(t, st, "update_result")
	if len(rows) != 1 || rows[0]["level"] != "warn" || rows[0]["error"] != "checksum" {
		t.Fatalf("the update's outcome: %v", rows)
	}
}

// A backup by hand says its reason and who took it; a deletion says
// which and who; both are the Cube's lines of the system.
func TestBackupsLogged(t *testing.T) {
	_, ts, _, st := startStored(t, Options{})
	token := userToken(t, st, "anna", store.RoleOwner)
	code, body := callAPI(t, ts, "POST", "/api/v1/cubes/KEQ0000100/backups", token, `{}`)
	if code != 201 {
		t.Fatalf("backup: %d %s", code, body)
	}
	var b struct{ ID int64 }
	json.Unmarshal([]byte(body), &b)
	if code, body := callAPI(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100/backups/"+strconv.FormatInt(b.ID, 10), token, ""); code != 204 {
		t.Fatalf("delete: %d %s", code, body)
	}
	var evs []event
	getJSON(t, ts, token, "/api/v1/cubes/KEQ0000100/log?source=system&limit=100", &evs)
	kinds := map[string]event{}
	for _, e := range evs {
		kinds[e.Kind] = e
	}
	taken, deleted := kinds["backup"], kinds["backup_deleted"]
	if taken.Summary["reason"] != "manual" || taken.By == nil || taken.By.Name != "anna" {
		t.Fatalf("the backup: %+v", taken)
	}
	if deleted.Summary["id"] != float64(b.ID) || deleted.By == nil || deleted.By.Name != "anna" {
		t.Fatalf("the deletion: %+v", deleted)
	}
}

// A Cube that follows discovery and answers from a new address says so
// in its own log, from where to where.
func TestAddressChangedLogged(t *testing.T) {
	_, ts, s, _ := startStored(t, Options{})
	rt := s.cubes["keq0000100"]
	rt.cfgMu.Lock()
	rt.cfg.FollowDiscovery = true
	rt.cfgMu.Unlock()
	// 127.0.0.2 is the loopback, where nothing answers: no packet leaves.
	s.note(context.Background(), cubelan.Discovered{Serial: "KEQ0000100", IP: net.ParseIP("127.0.0.2")}, time.Now())
	var evs []event
	eventually(t, "address_changed", func() bool {
		_, evs = logOf(t, ts, "?kind=address_changed")
		return len(evs) == 1
	})
	if evs[0].Summary["to"] != "127.0.0.2" || evs[0].Summary["from"] != "127.0.0.1" || evs[0].Source != "cube" || evs[0].Serial != "KEQ0000100" {
		t.Fatalf("the line: %+v", evs[0])
	}
}

// A found Cube that no longer answers is a line of the server's, in the
// configured Cube's log.
func TestCubeGoneLogged(t *testing.T) {
	_, ts, s, _ := startStored(t, Options{})
	s.cubeGone(foundCube{Serial: "KEQ0000009", Address: "192.0.2.9"})
	_, evs := logOf(t, ts, "?kind=cube_gone")
	if len(evs) != 1 || evs[0].Cube == nil || evs[0].Cube.Serial != "KEQ0000009" || evs[0].Source != "cube" {
		t.Fatalf("cube_gone: %+v", evs)
	}
}
