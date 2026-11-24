// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The last updates (AP UpdateHistory v1.0, v1.1): an update started in
// Massimo, completed at the next start; a new version from outside; the
// channel of each; the helper's outcome where it did not reach its
// version, at the next start or, without a restart, at the next read;
// ten kept.

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

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
	t.Parallel()
	st := testStore(t, "")
	s, ts, _ := historyServer(t, st, "")
	token := userToken(t, st, "anna", store.RoleOwner)
	if code, body := callAPI(t, ts, "POST", "/api/v1/update", token, "{}"); code != 202 {
		t.Fatalf("POST: %d %s", code, body)
	}
	list := s.updateHistory(t.Context())
	if len(list) != 1 || list[0].By == nil || list[0].By.Name != "anna" || list[0].From != "v0.1.0-alpha.9" || list[0].To != "v0.1.0-alpha.10" ||
		list[0].Channel != "alpha" || list[0].Outside || !list[0].waiting() || list[0].At.IsZero() {
		t.Fatalf("the started update: %+v", list)
	}
	s.noteVersionStart(t.Context(), "v0.1.0-alpha.9", "v0.1.0-alpha.10", nil)
	list = s.updateHistory(t.Context())
	if len(list) != 1 || list[0].Reached != "v0.1.0-alpha.10" || list[0].ReachedAt == nil || list[0].Outside || list[0].By == nil || list[0].Channel != "alpha" {
		t.Fatalf("the update reached: %+v", list)
	}
}

// An update from outside carries the channel the box followed at the
// start that found it: apt's suite where neither the variable nor the
// setting names one, and the setting where it does.
func TestUpdateHistoryOutsideChannel(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	s := New(Options{Store: st, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Update: UpdateOptions{
		Version: "v0.1.0-beta.2", Flavour: "alpha", Manager: "apt", Suite: func() string { return "beta" }, Finder: &fakeFinder{},
	}})
	ctx := t.Context()
	s.noteVersionStart(ctx, "v0.1.0-beta.1", "v0.1.0-beta.2", nil)
	if list := s.updateHistory(ctx); len(list) != 1 || !list[0].Outside || list[0].Channel != "beta" {
		t.Fatalf("apt's suite: %+v", list)
	}
	if err := st.SetSetting(ctx, store.UpdateChannelKey, "rc"); err != nil {
		t.Fatal(err)
	}
	s.noteVersionStart(ctx, "v0.1.0-beta.2", "v0.1.0-rc.1", nil)
	if list := s.updateHistory(ctx); len(list) != 2 || !list[0].Outside || list[0].Channel != "rc" || list[1].Channel != "beta" {
		t.Fatalf("the setting: %+v", list)
	}
}

// An update the helper ended without restarting the server, apt having
// nothing newer yet, ends at the next read of GET /api/v1/update rather
// than at the next start; a status older than the update is not its,
// and one that says the helper restarts the server, the old version
// still answering, does not end it.
func TestUpdateHistoryOutcomeAtRead(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	s, ts, status := historyServer(t, st, "apt")
	token := userToken(t, st, "anna", store.RoleOwner)
	write := func(us update.Status) {
		t.Helper()
		if err := update.WriteStatus(status, us); err != nil {
			t.Fatal(err)
		}
	}
	newest := func() map[string]any {
		t.Helper()
		var out struct {
			History []map[string]any `json:"history"`
		}
		if code := getJSON(t, ts, token, "/api/v1/update", &out); code != 200 || len(out.History) == 0 {
			t.Fatalf("GET: %d %+v", code, out.History)
		}
		return out.History[0]
	}
	write(update.Status{State: "up-to-date", Channel: "alpha", Time: time.Now().Add(-time.Hour)})
	if code, body := callAPI(t, ts, "POST", "/api/v1/update", token, "{}"); code != 202 {
		t.Fatalf("POST: %d %s", code, body)
	}
	if e := newest(); e["result"] != nil || e["to"] != "v0.1.0-alpha.10" {
		t.Fatalf("an older status ended the update: %v", e)
	}
	write(update.Status{State: "restarting", From: "v0.1.0-alpha.9", To: "v0.1.0-alpha.10", Channel: "alpha", Time: time.Now().Add(time.Second)})
	if e := newest(); e["result"] != nil {
		t.Fatalf("the helper restarting the server ended the update: %v", e)
	}
	write(update.Status{State: "up-to-date", From: "v0.1.0-alpha.9", Channel: "alpha", Message: "apt has nothing newer yet", Time: time.Now().Add(2 * time.Second)})
	if e := newest(); e["result"] != "up-to-date" || e["reached"] != nil || e["channel"] != "alpha" {
		t.Fatalf("the update apt had nothing for: %v", e)
	}
	if list := s.updateHistory(t.Context()); len(list) != 1 || list[0].Result != "up-to-date" || list[0].Outside {
		t.Fatalf("the row: %+v", list)
	}
}

// A new version that no entry waits for came from outside Massimo,
// through the box's package manager; so does one beyond the version an
// entry waits for, which keeps waiting.
func TestUpdateHistoryOutside(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	s, _, _ := historyServer(t, st, "apt")
	s.noteVersionStart(t.Context(), "v0.1.0-alpha.9", "v0.1.0-alpha.10", nil)
	list := s.updateHistory(t.Context())
	if len(list) != 1 || !list[0].Outside || list[0].Manager != "apt" || list[0].By != nil || list[0].From != "v0.1.0-alpha.9" ||
		list[0].Reached != "v0.1.0-alpha.10" || list[0].ReachedAt == nil || list[0].To != "" {
		t.Fatalf("the update from outside: %+v", list)
	}
	s.noteUpdateStart(t.Context(), time.Now(), 0, "v0.1.0-alpha.10", "v0.1.0-alpha.11", "alpha")
	s.noteVersionStart(t.Context(), "v0.1.0-alpha.10", "v0.1.0-alpha.12", nil)
	list = s.updateHistory(t.Context())
	if len(list) != 3 || !list[0].Outside || list[0].Reached != "v0.1.0-alpha.12" || !list[1].waiting() || list[1].By != nil {
		t.Fatalf("an update beyond the one asked for: %+v", list)
	}
}

// A start with the version of the last one, or without a last one,
// adds nothing.
func TestUpdateHistoryUnchangedStart(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	st := testStore(t, "")
	s, _, _ := historyServer(t, st, "")
	for i := 1; i <= 11; i++ {
		s.noteUpdateStart(t.Context(), time.Now(), 0, fmt.Sprintf("v1.0.%d", i-1), fmt.Sprintf("v1.0.%d", i), "stable")
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
	t.Parallel()
	st := testStore(t, "")
	s, _, _ := historyServer(t, st, "")
	ctx := t.Context()
	started := time.Now().Add(-time.Minute)
	s.noteUpdateStart(ctx, started, 0, "v1.0.0", "v1.1.0", "stable")
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
		s.noteUpdateStart(ctx, started, 0, "v1.0.0", "v1.1.0", "stable")
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
	t.Parallel()
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	first := startServer(t, stored(), withOptions(Options{Version: "v1.0.0", Logger: discard}))
	first.stop()
	startServer(t, again(first), withOptions(Options{Version: "v1.1.0", Update: UpdateOptions{Manager: "brew"}, Logger: discard})).stop()
	st := first.st
	s := New(Options{Store: st})
	list := s.updateHistory(t.Context())
	if len(list) != 1 || !list[0].Outside || list[0].Manager != "brew" || list[0].From != "v1.0.0" || list[0].Reached != "v1.1.0" {
		t.Fatalf("the history after two starts: %+v", list)
	}
}

// GET /api/v1/update answers the history, newest first, and an empty
// list before the first update.
func TestUpdateHistoryInTheAnswer(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	s, ts, _ := historyServer(t, st, "apt")
	token := userToken(t, st, "anna", store.RoleOwner)
	var out struct {
		History []map[string]any `json:"history"`
	}
	var empty map[string]json.RawMessage
	if getJSON(t, ts, token, "/api/v1/update", &empty); string(empty["history"]) != "[]" {
		t.Fatalf("before the first update: %s", empty["history"])
	}
	s.noteVersionStart(t.Context(), "v0.1.0-alpha.8", "v0.1.0-alpha.9", nil)
	if code, body := callAPI(t, ts, "POST", "/api/v1/update", token, "{}"); code != 202 {
		t.Fatalf("POST: %d %s", code, body)
	}
	getJSON(t, ts, token, "/api/v1/update", &out)
	if len(out.History) != 2 {
		t.Fatalf("the history: %+v", out.History)
	}
	newest, oldest := out.History[0], out.History[1]
	by, _ := newest["by"].(map[string]any)
	if by["name"] != "anna" || newest["to"] != "v0.1.0-alpha.10" || newest["channel"] != "alpha" || newest["reached"] != nil || newest["outside"] != nil {
		t.Errorf("the update started: %v", newest)
	}
	if oldest["outside"] != true || oldest["manager"] != "apt" || oldest["channel"] != "alpha" || oldest["reached"] != "v0.1.0-alpha.9" || oldest["by"] != nil {
		t.Errorf("the update from outside: %v", oldest)
	}
}
