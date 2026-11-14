// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The last updates (AP UpdateHistory v1.0): an update started in
// Massimo, completed at the next start; a new version from outside; the
// helper's outcome where it did not reach its version; ten kept.

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/update"
)

// historyServer is a server over st that can start updates from
// v0.1.0-alpha.9 to alpha.10, with its helper's status file; the
// package manager is manager.
func historyServer(t *testing.T, st *store.Store, manager string) (*Server, *httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	status := filepath.Join(dir, "status.json")
	s := New(Options{Store: st, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Update: UpdateOptions{
		Version: "v0.1.0-alpha.9", Channel: "alpha", Manager: manager,
		Request: filepath.Join(dir, "update-request"), Status: status, Finder: &fakeFinder{tag: "v0.1.0-alpha.10"},
	}})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts, status
}

// An update started through the API is an entry with who started it,
// waiting; the next start with its version completes it.
func TestUpdateHistoryStartedThenReached(t *testing.T) {
	st := testStore(t, "")
	s, ts, _ := historyServer(t, st, "")
	token := userToken(t, st, "anna", store.RoleOwner)
	if code, body := callAPI(t, ts, "POST", "/api/v1/update", token, "{}"); code != 202 {
		t.Fatalf("POST: %d %s", code, body)
	}
	list := s.updateHistory(t.Context())
	if len(list) != 1 || list[0].By == nil || list[0].By.Name != "anna" || list[0].From != "v0.1.0-alpha.9" || list[0].To != "v0.1.0-alpha.10" ||
		list[0].Outside || !list[0].waiting() || list[0].At.IsZero() {
		t.Fatalf("the started update: %+v", list)
	}
	s.noteVersionStart(t.Context(), "v0.1.0-alpha.9", "v0.1.0-alpha.10", nil)
	list = s.updateHistory(t.Context())
	if len(list) != 1 || list[0].Reached != "v0.1.0-alpha.10" || list[0].ReachedAt == nil || list[0].Outside || list[0].By == nil {
		t.Fatalf("the update reached: %+v", list)
	}
}

// A new version that no entry waits for came from outside Massimo,
// through the box's package manager; so does one beyond the version an
// entry waits for, which keeps waiting.
func TestUpdateHistoryOutside(t *testing.T) {
	st := testStore(t, "")
	s, _, _ := historyServer(t, st, "apt")
	s.noteVersionStart(t.Context(), "v0.1.0-alpha.9", "v0.1.0-alpha.10", nil)
	list := s.updateHistory(t.Context())
	if len(list) != 1 || !list[0].Outside || list[0].Manager != "apt" || list[0].By != nil || list[0].From != "v0.1.0-alpha.9" ||
		list[0].Reached != "v0.1.0-alpha.10" || list[0].ReachedAt == nil || list[0].To != "" {
		t.Fatalf("the update from outside: %+v", list)
	}
	s.noteUpdateStart(t.Context(), time.Now(), 0, "v0.1.0-alpha.10", "v0.1.0-alpha.11")
	s.noteVersionStart(t.Context(), "v0.1.0-alpha.10", "v0.1.0-alpha.12", nil)
	list = s.updateHistory(t.Context())
	if len(list) != 3 || !list[0].Outside || list[0].Reached != "v0.1.0-alpha.12" || !list[1].waiting() || list[1].By != nil {
		t.Fatalf("an update beyond the one asked for: %+v", list)
	}
}

// A start with the version of the last one, or without a last one,
// adds nothing.
func TestUpdateHistoryUnchangedStart(t *testing.T) {
	st := testStore(t, "")
	s, _, _ := historyServer(t, st, "apt")
	s.noteVersionStart(t.Context(), "", "v0.1.0-alpha.9", nil)
	s.noteVersionStart(t.Context(), "v0.1.0-alpha.9", "v0.1.0-alpha.9", nil)
	if _, ok, _ := st.Setting(t.Context(), updateHistoryKey); ok {
		t.Fatalf("a row without an update: %+v", s.updateHistory(t.Context()))
	}
}

// The row keeps the newest ten.
func TestUpdateHistoryKeepsTen(t *testing.T) {
	st := testStore(t, "")
	s, _, _ := historyServer(t, st, "")
	for i := 1; i <= 11; i++ {
		s.noteUpdateStart(t.Context(), time.Now(), 0, fmt.Sprintf("v1.0.%d", i-1), fmt.Sprintf("v1.0.%d", i))
	}
	list := s.updateHistory(t.Context())
	if len(list) != updateHistoryKeep || list[0].To != "v1.0.11" || list[9].To != "v1.0.2" {
		t.Fatalf("%d entries, newest %q, oldest %q", len(list), list[0].To, list[len(list)-1].To)
	}
}

// An update the helper ended without its version gets the helper's
// outcome at the next start: failed with the old version still running,
// rolled back after the new one ran, or while the helper restarts the
// old one; an older status is not this update's.
func TestUpdateHistoryHelperOutcome(t *testing.T) {
	st := testStore(t, "")
	s, _, _ := historyServer(t, st, "")
	ctx := t.Context()
	started := time.Now().Add(-time.Minute)
	s.noteUpdateStart(ctx, started, 0, "v1.0.0", "v1.1.0")
	s.noteVersionStart(ctx, "v1.0.0", "v1.0.0", &update.Status{State: "failed", From: "v1.0.0", To: "v1.1.0", Time: started.Add(-time.Second)})
	if list := s.updateHistory(ctx); !list[0].waiting() {
		t.Fatalf("an older status ended the update: %+v", list[0])
	}
	s.noteVersionStart(ctx, "v1.0.0", "v1.0.0", &update.Status{State: "failed", From: "v1.0.0", To: "v1.1.0", Time: time.Now()})
	if list := s.updateHistory(ctx); list[0].Result != "failed" || list[0].Reached != "" {
		t.Fatalf("the failed update: %+v", list[0])
	}

	// The new version ran, then the helper put the old one back: while it
	// restarts it, and once it says so.
	for _, state := range []string{"restarting", "rolled-back"} {
		started = time.Now().Add(-time.Minute)
		s.noteUpdateStart(ctx, started, 0, "v1.0.0", "v1.1.0")
		s.noteVersionStart(ctx, "v1.0.0", "v1.1.0", nil)
		s.noteVersionStart(ctx, "v1.1.0", "v1.0.0", &update.Status{State: state, From: "v1.0.0", To: "v1.1.0", Time: time.Now()})
		list := s.updateHistory(ctx)
		if list[0].Result != "rolled-back" || list[0].Reached != "v1.1.0" || list[0].Outside || list[len(list)-1].Result != "failed" {
			t.Fatalf("rolled back (%s): %+v", state, list)
		}
	}
}

// The start writes the history: a second start with a new version and
// no update started is one from outside, by the box's manager.
func TestUpdateHistoryAtStart(t *testing.T) {
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
	_, stop = runOver(t, st, row, Options{Version: "v1.1.0", Update: UpdateOptions{Manager: "brew"}})
	stop()
	s := New(Options{Store: st})
	list := s.updateHistory(t.Context())
	if len(list) != 1 || !list[0].Outside || list[0].Manager != "brew" || list[0].From != "v1.0.0" || list[0].Reached != "v1.1.0" {
		t.Fatalf("the history after two starts: %+v", list)
	}
}
