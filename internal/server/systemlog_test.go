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
	"slices"
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
		s.LogSettingsRead(ctx, "the API", []string{"MASSIMO_LOG_LEVEL"}, []string{"MASSIMO_CUBE_KEQ0000000_ADDRESS"}, nil, nil)
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

// settings_read's error keeps what the next start refuses and the notes
// on the file apart, the refusals first, each kind after its own words
// and none of them for a kind the read did not find (AP
// StartConfiguration, D3-B).
func TestSettingsReadErrorKeepsTheKindsApart(t *testing.T) {
	t.Parallel()
	refused := []Refusal{{Settings: []string{"MASSIMO_HSTS"}, Error: `-hsts or MASSIMO_HSTS "-1": want a number of days, 0 or more`}, {Error: "env:3: expected KEY=VALUE"}}
	notes := []string{"MASSIMO_HSTS is given on lines 1 and 4; the first wins", "line 5 has no name before its =, ignored"}
	for _, c := range []struct {
		what    string
		refused []Refusal
		notes   []string
		want    string
	}{
		{"both", refused, notes, `the next start refuses: -hsts or MASSIMO_HSTS "-1": want a number of days, 0 or more; env:3: expected KEY=VALUE; ` +
			"the settings file: MASSIMO_HSTS is given on lines 1 and 4; the first wins; line 5 has no name before its =, ignored"},
		{"refusals alone", refused[:1], nil, `the next start refuses: -hsts or MASSIMO_HSTS "-1": want a number of days, 0 or more`},
		{"notes alone", nil, notes[1:], "the settings file: line 5 has no name before its =, ignored"},
		{"neither", []Refusal{}, []string{}, ""},
	} {
		if got := settingsReadError(c.refused, c.notes); got != c.want {
			t.Errorf("%s:\n%q, want\n%q", c.what, got, c.want)
		}
	}
}

// The notes on the settings file the start read are a settings_read
// warning, why "start", after its server_started, which settles every
// settings_read warning before it: the log's sign stays across a restart
// while a note stands, and goes with the first start without one, which
// writes no settings_read (AP StartConfiguration, D33-A).
func TestStartLogsTheSettingsNotes(t *testing.T) {
	t.Parallel()
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	notes := []string{"MASSIMO_HSTS is given on lines 3 and 4; the first wins", "line 7 has no name before its =, ignored"}
	// started checks a start's lines, with the store's written: its
	// server_started the newest but for the settings_read of notes, at
	// warn with them after "the settings file: ", where it has any; and
	// the log's sign, a settings_read the alert where it has notes, that
	// one, and none where it has none.
	starts := 0
	started := func(r *rig, notes []string) {
		t.Helper()
		starts++
		if err := r.st.FlushEvents(t.Context()); err != nil {
			t.Fatal(err)
		}
		rows, err := r.st.Events(t.Context(), store.EventQuery{Source: "system", Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		var ours []store.Event // the starts' and their notes', oldest first
		for _, row := range slices.Backward(rows) {
			if row.Kind == "server_started" || row.Kind == "settings_read" {
				ours = append(ours, row)
			}
		}
		last := ours[len(ours)-1]
		var alerts logAlertsView
		if code := getJSON(t, r.ts, "", "/api/v1/log/alerts", &alerts); code != 200 {
			t.Fatalf("start %d: the alerts: %d", starts, code)
		}
		var signs []logAlert
		for _, a := range alerts.Alerts {
			if a.Kind == "settings_read" {
				signs = append(signs, a)
			}
		}
		if len(notes) == 0 {
			if last.Kind != "server_started" || len(signs) != 0 {
				t.Fatalf("start %d without notes: its last line %s, the settings_read alerts %+v", starts, last.Kind, signs)
			}
			return
		}
		var ev event
		json.Unmarshal(last.Data, &ev)
		before := ours[len(ours)-2]
		if last.Kind != "settings_read" || before.Kind != "server_started" || last.ID <= before.ID || last.Level != store.LevelWarn ||
			ev.Error != "the settings file: "+strings.Join(notes, "; ") || ev.Summary["why"] != "start" {
			t.Fatalf("start %d: %s %d then %s %d at %d: %+v", starts, before.Kind, before.ID, last.Kind, last.ID, last.Level, ev)
		}
		if len(signs) != 1 || signs[0].ID != last.ID || signs[0].Level != "warn" {
			t.Fatalf("start %d: the settings_read alerts %+v, want the start's line %d", starts, signs, last.ID)
		}
	}
	first := startServer(t, stored(), withOptions(Options{SettingsNotes: notes, Logger: discard}))
	started(first, notes)
	first.stop()
	second := startServer(t, again(first), withOptions(Options{SettingsNotes: notes[:1], Logger: discard}))
	started(second, notes[:1])
	second.stop()
	third := startServer(t, again(second), withOptions(Options{Logger: discard}))
	started(third, nil)
	third.stop()
	if n := len(systemRows(t, first.st, "settings_read")); n != 2 {
		t.Fatalf("%d settings_read lines, want the two starts' with notes", n)
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
	m := logMarkAs(t, ts, token)
	s.LogRotated("size", "serve.log.1", 8, []string{"serve.log.9.gz"}, nil)
	s.LogRotated("sighup", "serve.log.1", 3, nil, errors.New("log not rotated: permission denied"))
	isRotated := logLine("log_rotated", "")
	evs, next := logAfterN(t, m, "the rotations' lines", 2, isRotated)
	if more := logLines(t, next, isRotated); len(more) != 0 {
		t.Fatalf("%d lines: %+v", 2+len(more), append(evs, more...))
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
	m := logMark(t, ts)
	s.note(context.Background(), cubelan.Discovered{Serial: "KEQ0000100", IP: net.ParseIP("127.0.0.2"), Kind: cubelan.KindCube}, time.Now())
	isChanged := logLine("address_changed", "")
	ev, next := logAfter(t, m, "address_changed", isChanged)
	if ev.Summary["to"] != "127.0.0.2" || ev.Summary["from"] != "127.0.0.1" || ev.Source != "cube" || ev.Serial != "KEQ0000100" {
		t.Fatalf("the line: %+v", ev)
	}
	if more := logLines(t, next, isChanged); len(more) != 0 {
		t.Fatalf("address_changed again: %+v", more)
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
