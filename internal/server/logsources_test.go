// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The log's sources, levels and users (AP Logging v1.1, Block 3): the
// server's events beside a Cube's, kept once; source=, level= and user=;
// who started an event named at read.

import (
	"context"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// A Cube's log holds the server's events, kept once with no Cube; source=
// keeps one source, and every event says its own.
func TestLogSourceFilter(t *testing.T) {
	_, ts, s, st := startStored(t, Options{})
	eventually(t, "the Cube's states", func() bool {
		_, evs := logOf(t, ts, "?kind=state")
		return len(evs) > 0
	})
	s.logServer(event{Kind: "server_started", Source: "system", Time: time.Now()})
	_, all := logOf(t, ts, "?limit=1000")
	sources := map[string]int{}
	for _, e := range all {
		if e.Source == "" || e.Level == "" {
			t.Fatalf("an event without its source or level: %+v", e)
		}
		sources[e.Source]++
	}
	if sources["system"] != 1 || sources["device"] == 0 || sources["cube"] == 0 {
		t.Fatalf("the sources of the log: %v", sources)
	}
	for _, src := range []string{"system", "device", "cube"} {
		code, evs := logOf(t, ts, "?limit=1000&source="+src)
		if code != 200 || len(evs) != sources[src] {
			t.Fatalf("source=%s: %d, %d events, want %d", src, code, len(evs), sources[src])
		}
		for _, e := range evs {
			if e.Source != src {
				t.Fatalf("source=%s answered %+v", src, e)
			}
		}
	}
	rows, err := st.Events(context.Background(), store.EventQuery{Kind: "server_started", Limit: 10})
	if err != nil || len(rows) != 1 || rows[0].CubeID != 0 || rows[0].Source != "system" {
		t.Fatalf("the server's event in the store: %+v %v", rows, err)
	}
	if code, _ := logOf(t, ts, "?source=radio"); code != 400 {
		t.Fatalf("a bad source: %d", code)
	}
}

// level= keeps that level and above, info by default: the H line is at
// debug, a lost connection at warn.
func TestLogLevelFilter(t *testing.T) {
	_, ts, s, _ := startStored(t, Options{})
	eventually(t, "the greeting's hello", func() bool {
		_, evs := logOf(t, ts, "?kind=hello&level=debug")
		return len(evs) > 0
	})
	if _, evs := logOf(t, ts, "?kind=hello"); len(evs) != 0 {
		t.Fatalf("hello at the default level: %+v", evs)
	}
	s.logServer(event{Kind: "server_stopping", Source: "system", Level: "warn", Time: time.Now()})
	_, warns := logOf(t, ts, "?level=warn")
	if len(warns) == 0 {
		t.Fatal("no warning")
	}
	for _, e := range warns {
		if e.Level != "warn" && e.Level != "error" {
			t.Fatalf("level=warn answered %+v", e)
		}
	}
	_, info := logOf(t, ts, "?limit=1000")
	_, debug := logOf(t, ts, "?limit=1000&level=debug")
	if len(debug) <= len(info) {
		t.Fatalf("debug %d events, info %d", len(debug), len(info))
	}
	for _, e := range info {
		if e.Level == "debug" {
			t.Fatalf("a debug event at the default level: %+v", e)
		}
	}
	if code, _ := logOf(t, ts, "?level=loud"); code != 400 {
		t.Fatalf("a bad level: %d", code)
	}
}

// user= keeps the events a user started, a removed user's too; the page
// names them as the users table does now; a name the store does not
// know has none.
func TestLogUserFilter(t *testing.T) {
	_, ts, s, st := startStored(t, Options{})
	ctx := context.Background()
	token := userToken(t, st, "anna", store.RoleViewer)
	if _, err := st.UpdateUser(ctx, "anna", func(u *store.User) error { u.DisplayName = "Anna"; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	anna, _, _ := st.User(ctx, "anna")
	logAs := func(query string) []event {
		t.Helper()
		var evs []event
		if code := getJSON(t, ts, token, "/api/v1/cubes/KEQ0000100/log"+query, &evs); code != 200 {
			t.Fatalf("log%s: %d", query, code)
		}
		return evs
	}
	s.logServer(event{Kind: "change", Source: "config", Time: time.Now(), userID: anna.ID})
	s.logServer(event{Kind: "change", Source: "config", Time: time.Now()})
	if evs := logAs("?user=Anna"); len(evs) != 1 || evs[0].By == nil || evs[0].By.Name != "anna" || evs[0].By.DisplayName != "Anna" {
		t.Fatalf("user=anna: %+v", evs)
	}
	if _, err := st.UpdateUser(ctx, "anna", func(u *store.User) error { u.DisplayName = "Anna B."; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	if evs := logAs("?user=anna"); len(evs) != 1 || evs[0].By.DisplayName != "Anna B." {
		t.Fatalf("the name as it is now: %+v", evs)
	}
	if evs := logAs("?kind=change"); len(evs) != 2 {
		t.Fatalf("%d change events", len(evs))
	}
	if evs := logAs("?user=nobody"); len(evs) != 0 {
		t.Fatalf("a user the store does not know: %d", len(evs))
	}
}

// A Cube found on the LAN is an event of the server's: kept once, with no
// Cube, and in the configured Cube's log.
func TestFoundCubeStoredOnce(t *testing.T) {
	_, ts, s, st := startStored(t, Options{})
	s.cubeFound(foundCube{Serial: "KEQ0000009", Address: "192.0.2.9", LastSeen: time.Now()})
	_, evs := logOf(t, ts, "?kind=cube_found")
	if len(evs) != 1 || evs[0].Cube == nil || evs[0].Cube.Serial != "KEQ0000009" || evs[0].Source != "cube" || evs[0].Serial != "KEQ0000009" {
		t.Fatalf("the Cube's log: %+v", evs)
	}
	rows, err := st.Events(context.Background(), store.EventQuery{Kind: "cube_found", Limit: 10})
	if err != nil || len(rows) != 1 || rows[0].CubeID != 0 {
		t.Fatalf("the server's rows: %+v %v", rows, err)
	}
	row, ok, err := st.Cube(context.Background(), "KEQ0000100")
	if err != nil || !ok {
		t.Fatalf("the Cube's row: %v %v", ok, err)
	}
	if own, _ := st.Events(context.Background(), store.EventQuery{CubeID: row.ID, Kind: "cube_found", Limit: 10}); len(own) != 0 {
		t.Fatalf("a copy among the Cube's own rows: %+v", own)
	}
}
