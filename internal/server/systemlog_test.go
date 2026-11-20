// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The server's own lines (AP Logging v1.1, Block 5): its start and stop,
// the settings file read again, an update's outcome, the backups, who
// asked for them, a Cube's new address and its leaving the LAN.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/update"
)

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
	t.Parallel()
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	first := startServer(t, stored(), withOptions(Options{Version: "v1.0.0", Logger: discard}))
	first.stop()
	startServer(t, again(first), withOptions(Options{Version: "v1.1.0", Logger: discard})).stop()
	st := first.st
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
	t.Parallel()
	r := startServer(t, stored())
	ts, s, st := r.ts, r.s, r.st
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

// A rotation of serve's log file is a line of the server's, with why,
// the old file's name and the files kept and deleted; a failed step at
// warn with its error.
func TestLogRotatedLine(t *testing.T) {
	t.Parallel()
	r := startServer(t, stored())
	ts, s, st := r.ts, r.s, r.st
	token := userToken(t, st, "anna", store.RoleOwner)
	s.LogRotated("size", "serve.log.1", 8, []string{"serve.log.9.gz"}, nil)
	s.LogRotated("sighup", "serve.log.1", 3, nil, errors.New("log not rotated: permission denied"))
	var evs []event
	testkit.Within(5*time.Second, func() bool {
		evs = nil
		getJSON(t, ts, token, "/api/v1/cubes/KEQ0000100/log?kind=log_rotated", &evs)
		return len(evs) >= 2
	})
	if len(evs) != 2 {
		t.Fatalf("%d lines: %+v", len(evs), evs)
	}
	done, failed := evs[0], evs[1]
	if done.Summary["why"] != "size" {
		done, failed = failed, done
	}
	removed, _ := done.Summary["removed"].([]any)
	if done.Source != "system" || done.Level != "info" || done.Summary["why"] != "size" || done.Summary["previous"] != "serve.log.1" ||
		done.Summary["kept"] != float64(8) || len(removed) != 1 || removed[0] != "serve.log.9.gz" {
		t.Errorf("the rotation: %+v", done)
	}
	if failed.Level != "warn" || !strings.Contains(failed.Error, "permission denied") || failed.Summary["why"] != "sighup" {
		t.Errorf("the failed rotation: %+v", failed)
	}
}

// An update's outcome the helper reported is logged once, at the first
// start after it, a failure at warn.
func TestUpdateResultLoggedOnce(t *testing.T) {
	t.Parallel()
	status := filepath.Join(t.TempDir(), "status.json")
	if err := update.WriteStatus(status, update.Status{State: "failed", From: "v1.0.0", To: "v1.1.0", Message: "checksum", Time: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	opts := Options{Version: "v1.0.0", Update: UpdateOptions{Status: status}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	first := startServer(t, stored(), withOptions(opts))
	first.stop()
	startServer(t, again(first), withOptions(opts)).stop()
	st := first.st
	rows := systemRows(t, st, "update_result")
	if len(rows) != 1 || rows[0]["level"] != "warn" || rows[0]["error"] != "checksum" {
		t.Fatalf("the update's outcome: %v", rows)
	}
}

// A backup by hand says its reason and who took it; a deletion says
// which and who; both are the Cube's lines of the system.
func TestBackupsLogged(t *testing.T) {
	t.Parallel()
	r := startServer(t, stored())
	ts, st := r.ts, r.st
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
	t.Parallel()
	// The Cube is lost at the new address and searched for: on the
	// loopback, not the LAN.
	r := startServer(t, stored(), withOptions(Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{testkit.ListenPacket(t).LocalAddr().String()}}))
	ts, s := r.ts, r.s
	rt := s.cubes["keq0000100"]
	rt.cfgMu.Lock()
	rt.cfg.FollowDiscovery = true
	rt.cfgMu.Unlock()
	// 127.0.0.2 is the loopback, where nothing answers: no packet leaves.
	s.note(context.Background(), cubelan.Discovered{Serial: "KEQ0000100", IP: net.ParseIP("127.0.0.2"), Kind: cubelan.KindCube}, time.Now())
	var evs []event
	testkit.Eventually(t, "address_changed", func() bool {
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
	t.Parallel()
	r := startServer(t, stored())
	ts, s := r.ts, r.s
	s.cubeGone(foundCube{Serial: "KEQ0000009", Address: "192.0.2.9"})
	_, evs := logOf(t, ts, "?kind=cube_gone")
	if len(evs) != 1 || evs[0].Cube == nil || evs[0].Cube.Serial != "KEQ0000009" || evs[0].Source != "cube" {
		t.Fatalf("cube_gone: %+v", evs)
	}
}
