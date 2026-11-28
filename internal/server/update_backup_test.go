// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The server's backup before an update (AP ServerBackup v1.0, Block 10):
// taken before the helper is asked, local and pinned, its id in the
// history and the log; a failure stops the update, no_backup goes on as
// an audit row of its own; no key goes on; the night's update and
// Install now take one too; POST /api/v1/update/backup takes it alone.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/update"
)

// addBackupKey gives st a classic key of the server's backups.
func addBackupKey(t *testing.T, st *store.Store) {
	t.Helper()
	k, err := serverbackup.GenerateKey(serverbackup.KindX25519)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddServerBackupKey(t.Context(), store.ServerBackupKey{Label: "laptop", Recipient: k.Recipient.Line, Fingerprint: k.Recipient.Fingerprint,
		Kind: string(k.Recipient.Kind)}, "", 0); err != nil {
		t.Fatal(err)
	}
}

// blockLocal makes st's local target one that cannot take a file: its
// directory is a file.
func blockLocal(t *testing.T, st *store.Store) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(st.Dir(), "blocked"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddServerBackupTarget(t.Context(), store.ServerBackupTarget{Name: "local", Kind: serverbackup.KindLocal,
		Config: map[string]string{"path": "blocked"}, Takes: string(serverbackup.TakesEvery), Enabled: true}, 0); err != nil {
		t.Fatal(err)
	}
}

// backupBox is a server over st that can start updates from
// v0.1.0-alpha.9 to alpha.10 and starts the helper itself, as on
// Windows: each start runs onStart and is counted.
type backupBox struct {
	s       *Server
	ts      *httptest.Server
	st      *store.Store
	request string
	starts  *atomic.Int32
	logs    *testkit.Watch[testkit.Record]
	anna    string
}

func newBackupBox(t *testing.T, st *store.Store, onStart func()) backupBox {
	t.Helper()
	dir := t.TempDir()
	b := backupBox{st: st, request: filepath.Join(dir, "update-request"), starts: &atomic.Int32{}}
	logger, logs := testkit.LogWatch(nil)
	b.logs = logs
	b.s = New(Options{Store: st, Logger: logger, Update: UpdateOptions{Version: "v0.1.0-alpha.9", Channel: "alpha", Request: b.request,
		Status: filepath.Join(dir, "status.json"), Finder: &fakeFinder{tag: "v0.1.0-alpha.10"},
		Start: func() error {
			b.starts.Add(1)
			if onStart != nil {
				onStart()
			}
			return nil
		}}})
	b.ts = httptest.NewServer(b.s.Handler())
	t.Cleanup(b.ts.Close)
	// the log's writer, which Run starts: the update's lines are read back
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); st.RunEvents(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	b.anna = userToken(t, st, "anna", store.RoleOwner)
	return b
}

// state is GET /api/v1/update's backup and history.
func (b backupBox) state(t *testing.T) (*updateBackupView, []updateEntry) {
	t.Helper()
	var out struct {
		Backup  *updateBackupView `json:"backup"`
		History []updateEntry     `json:"history"`
	}
	if code := getJSON(t, b.ts, b.anna, "/api/v1/update", &out); code != 200 {
		t.Fatalf("GET: %d", code)
	}
	return out.Backup, out.History
}

// settle waits for the updates started in the background.
func (b backupBox) settle() { b.s.updates.start.runs.Wait() }

// beforeUpdate are st's backups before an update.
func beforeUpdate(t *testing.T, st *store.Store) []store.ServerBackup {
	t.Helper()
	bs, err := st.ServerBackups(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var out []store.ServerBackup
	for _, b := range bs {
		if b.Trigger == string(serverbackup.TriggerBeforeUpdate) {
			out = append(out, b)
		}
	}
	return out
}

// updateLines are the server's update lines, newest first.
func updateLines(t *testing.T, st *store.Store) []store.Event {
	t.Helper()
	if err := st.FlushEvents(t.Context()); err != nil {
		t.Fatal(err)
	}
	evs, err := st.Events(t.Context(), store.EventQuery{Server: true, Kind: "update", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	return evs
}

// An update takes its backup before the helper is asked: when the helper
// starts, a backup before an update is made, on the local target, pinned
// for 30 days; the history's entry and the update's line name it, and
// GET says an update takes one.
func TestUpdateTakesABackupFirst(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	addBackupKey(t, st)
	var seen []store.ServerBackup
	var b backupBox
	b = newBackupBox(t, st, func() { seen = beforeUpdate(t, st) })
	if v, _ := b.state(t); v == nil || v.State != backupReady {
		t.Fatalf("before: %+v", v)
	}
	started := time.Now()
	if code, body := callAPI(t, b.ts, "POST", "/api/v1/update", b.anna, "{}"); code != 202 || !strings.Contains(body, `"step": "backup"`) {
		t.Fatalf("POST: %d %s", code, body)
	}
	b.settle()
	if b.starts.Load() != 1 || len(seen) != 1 {
		t.Fatalf("%d starts, %d backups before an update at the start", b.starts.Load(), len(seen))
	}
	made := seen[0]
	if made.State != store.ServerBackupMade || len(made.Copies) != 1 || made.Copies[0].State != store.CopyThere || made.UserID == 0 {
		t.Fatalf("the backup: %+v", made)
	}
	if pin := made.PinnedUntil.Sub(started); pin < serverbackup.PinBefore-time.Minute || pin > serverbackup.PinBefore+time.Minute {
		t.Fatalf("pinned until %s, %s after the update", made.PinnedUntil, pin)
	}
	v, history := b.state(t)
	if len(history) != 1 || history[0].Backup != made.UUID || history[0].BackupSkipped != "" || v == nil || v.State != backupReady {
		t.Fatalf("the history %+v, the state %+v", history, v)
	}
	lines := updateLines(t, st)
	if len(lines) != 1 || !strings.Contains(string(lines[0].Data), `"backup":"`+made.UUID+`"`) {
		t.Fatalf("the update's line: %+v", lines)
	}
}

// A backup that fails stops the update: no request, no start, an error
// line of the update, and GET says it failed; the failed backup leaves
// no row. no_backup then goes on without one, as an audit row of its
// own, and the history says it was asked so.
func TestUpdateStopsWhereItsBackupFails(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	addBackupKey(t, st)
	blockLocal(t, st)
	b := newBackupBox(t, st, nil)
	for range 2 {
		if code, body := callAPI(t, b.ts, "POST", "/api/v1/update", b.anna, "{}"); code != 202 {
			t.Fatalf("POST: %d %s", code, body)
		}
		b.settle()
	}
	if _, err := os.Stat(b.request); !errors.Is(err, os.ErrNotExist) || b.starts.Load() != 0 {
		t.Fatalf("the helper asked: %v, %d starts", err, b.starts.Load())
	}
	if failed := beforeUpdate(t, st); len(failed) != 0 {
		t.Fatalf("the failed backups: %+v", failed)
	}
	v, history := b.state(t)
	if v == nil || v.State != backupFailed || v.Error == "" || v.At == nil || len(history) != 0 {
		t.Fatalf("the state %+v, the history %+v", v, history)
	}
	var got struct {
		StartError *updateStartError `json:"start_error"`
		CanUpdate  bool              `json:"can_update"`
	}
	if getJSON(t, b.ts, b.anna, "/api/v1/update", &got); got.StartError == nil || !got.CanUpdate ||
		!strings.Contains(got.StartError.Error, "backup of the server before the update failed") || !strings.Contains(got.StartError.Error, "no_backup") {
		t.Fatalf("the start's error: %+v", got)
	}
	if lines := updateLines(t, st); len(lines) != 2 || lines[0].Level != store.LevelError {
		t.Fatalf("the update's lines: %+v", lines)
	}

	if code, body := callAPI(t, b.ts, "POST", "/api/v1/update", b.anna, `{"no_backup": true}`); code != 202 {
		t.Fatalf("POST no_backup: %d %s", code, body)
	}
	if b.starts.Load() != 1 {
		t.Fatalf("%d starts", b.starts.Load())
	}
	rows := auditRows(t, b.ts, b.anna, "?limit=1")
	if len(rows) != 1 || rows[0].Action != "update.no_backup" || rows[0].Result != store.ResultOK {
		t.Fatalf("the audit: %+v", rows)
	}
	v, history = b.state(t)
	if len(history) != 1 || history[0].BackupSkipped != skippedAsked || history[0].Backup != "" || v == nil || v.State != backupReady {
		t.Fatalf("the history %+v, the state %+v", history, v)
	}
	var after struct {
		StartError *updateStartError `json:"start_error"`
	}
	if getJSON(t, b.ts, b.anna, "/api/v1/update", &after); after.StartError != nil {
		t.Fatalf("the start's error stays: %+v", after.StartError)
	}
	if lines := updateLines(t, st); len(lines) != 3 || !strings.Contains(string(lines[0].Data), `"backup_skipped":"asked"`) {
		t.Fatalf("the update's line: %+v", lines[0])
	}
}

// Without a key the update goes on: a line in the log, the history's
// no_key, and GET says so.
func TestUpdateWithoutAKeyGoesOn(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	b := newBackupBox(t, st, nil)
	if v, _ := b.state(t); v == nil || v.State != backupNoKey {
		t.Fatalf("the state: %+v", v)
	}
	if code, body := callAPI(t, b.ts, "POST", "/api/v1/update", b.anna, "{}"); code != 202 || b.starts.Load() != 1 {
		t.Fatalf("POST: %d %s, %d starts", code, body, b.starts.Load())
	}
	if _, history := b.state(t); len(history) != 1 || history[0].BackupSkipped != skippedNoKey {
		t.Fatalf("the history: %+v", history)
	}
	if got := b.logs.Since(0, testkit.Msg("update: no key is set for the server's backups; the update goes on without a backup")); len(got) != 1 {
		t.Fatal("no line on the missing key")
	}
	if len(beforeUpdate(t, st)) != 0 {
		t.Fatal("a backup without a key")
	}
}

// The night's update takes its backup too, and one that fails ends the
// night's try; Install now takes one as Update does.
func TestNightAndInstallNowTakeABackup(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	addBackupKey(t, st)
	night := newNightBox(t, st, &fakeFinder{tag: "v0.1.0-alpha.10"}, nil)
	if n := night.tick(t, nightAt(3, 0)); n != 1 {
		t.Fatalf("%d starts at night", n)
	}
	list := night.s.updateHistory(t.Context())
	bs := beforeUpdate(t, st)
	if len(bs) != 1 || len(list) != 1 || !list[0].Auto || list[0].Backup != bs[0].UUID || bs[0].UserID != 0 {
		t.Fatalf("the night's backup %+v, history %+v", bs, list)
	}

	failing := testStore(t, "")
	addBackupKey(t, failing)
	blockLocal(t, failing)
	stopped := newNightBox(t, failing, &fakeFinder{tag: "v0.1.0-alpha.10"}, nil)
	if n := stopped.tick(t, nightAt(3, 0)); n != 0 {
		t.Fatalf("%d starts after a failed backup", n)
	}
	if got := stopped.logs.Since(0, testkit.Msg("automatic update not started; the next night tries again")); len(got) != 1 {
		t.Fatal("no line on the night's failed try")
	}

	box := newAnswerBox(t, &fakeFinder{tag: "v0.1.0-alpha.10"})
	addBackupKey(t, box.st)
	if code, body := box.answer(t, box.anna, "v0.1.0-alpha.10", "now"); code != 202 || !strings.Contains(body, `"starting"`) {
		t.Fatalf("Install now: %d %s", code, body)
	}
	box.s.updates.start.runs.Wait()
	if bs := beforeUpdate(t, box.st); len(bs) != 1 || bs[0].State != store.ServerBackupMade {
		t.Fatalf("Install now's backup: %+v", bs)
	}
	if req, ok, err := update.TakeRequest(box.request); err != nil || !ok || req != "alpha" {
		t.Fatalf("Install now's request: %q %v %v", req, ok, err)
	}
}

// POST /api/v1/update/backup takes the update's backup alone, for a
// holder of update and the version an update installs: without a key it
// says so, with one it answers the backup, pinned, audited as
// update.backup; a second takes the pin of the first, which no update
// used; a failure is the update's; no version, or the running one, is
// refused.
func TestUpdateBackupRoute(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	b := newBackupBox(t, st, nil)
	const to = `{"to": "v0.1.0-alpha.10"}`
	code, body := callAPI(t, b.ts, "POST", "/api/v1/update/backup", b.anna, to)
	if code != 200 || !strings.Contains(body, `"skipped": "no_key"`) {
		t.Fatalf("without a key: %d %s", code, body)
	}
	addBackupKey(t, st)
	for _, c := range []struct {
		body string
		want int
	}{{"", 400}, {`{"to": "later"}`, 400}, {`{"to": "v0.1.0-alpha.9"}`, 409}} {
		if code, body := callAPI(t, b.ts, "POST", "/api/v1/update/backup", b.anna, c.body); code != c.want {
			t.Fatalf("%s: %d %s, want %d", c.body, code, body, c.want)
		}
	}
	made := func() serverBackupView {
		t.Helper()
		code, body := callAPI(t, b.ts, "POST", "/api/v1/update/backup", b.anna, to)
		var out struct {
			Backup serverBackupView `json:"backup"`
		}
		if err := json.Unmarshal([]byte(body), &out); code != 201 || err != nil || out.Backup.Trigger != string(serverbackup.TriggerBeforeUpdate) ||
			out.Backup.PinnedUntil == nil || len(out.Backup.Copies) != 1 {
			t.Fatalf("with a key: %d %s", code, body)
		}
		return out.Backup
	}
	first := made()
	if rows := auditRows(t, b.ts, b.anna, "?limit=1"); len(rows) != 1 || rows[0].Action != "update.backup" || rows[0].Target != "serverbackup:"+first.UUID {
		t.Fatalf("the audit: %+v", rows)
	}
	second := made()
	pins := map[string]bool{}
	for _, bk := range beforeUpdate(t, st) {
		pins[bk.UUID] = !bk.PinnedUntil.IsZero()
	}
	if pins[first.UUID] || !pins[second.UUID] {
		t.Fatalf("the pins after a second: %v (first %s, second %s)", pins, first.UUID, second.UUID)
	}
	if b.starts.Load() != 0 {
		t.Fatal("the helper started")
	}
	viewer := userToken(t, st, "val", store.RoleViewer)
	if code, body := callAPI(t, b.ts, "POST", "/api/v1/update/backup", viewer, to); code != 403 {
		t.Fatalf("a viewer: %d %s", code, body)
	}
	blocked := testStore(t, "")
	addBackupKey(t, blocked)
	blockLocal(t, blocked)
	f := newBackupBox(t, blocked, nil)
	if code, body := callAPI(t, f.ts, "POST", "/api/v1/update/backup", f.anna, to); code != 500 || !strings.Contains(body, "before the update failed") {
		t.Fatalf("a failed backup: %d %s", code, body)
	}
}

// While its backup runs, the update is being started: GET says so and
// offers no Update, a second request is refused at once; the backup and
// the start outlive the request's context, and a used backup keeps its
// pin when the next update takes one.
func TestUpdateStartsInTheBackground(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	addBackupKey(t, st)
	b := newBackupBox(t, st, nil)
	b.s.sb.busy <- struct{}{} // a backup job runs: the update's waits for it
	ctx, cancel := context.WithCancel(t.Context())
	v, err := b.s.beginUpdate(ctx, 0, false, false)
	cancel() // the browser gave up
	if err != nil || v.Starting == nil || v.Starting.Step != stepBackup {
		t.Fatalf("begun: %+v %v", v.Starting, err)
	}
	var got struct {
		Starting  *updateStartingView `json:"starting"`
		CanUpdate bool                `json:"can_update"`
	}
	if getJSON(t, b.ts, b.anna, "/api/v1/update", &got); got.Starting == nil || got.Starting.Version != "v0.1.0-alpha.10" || got.CanUpdate {
		t.Fatalf("GET while starting: %+v", got)
	}
	if code, body := callAPI(t, b.ts, "POST", "/api/v1/update", b.anna, "{}"); code != 409 || !strings.Contains(body, "being started") {
		t.Fatalf("a second POST: %d %s", code, body)
	}
	<-b.s.sb.busy
	b.settle()
	if b.starts.Load() != 1 {
		t.Fatalf("%d starts after the request's end", b.starts.Load())
	}
	var after struct {
		Starting *updateStartingView `json:"starting"`
	}
	if getJSON(t, b.ts, b.anna, "/api/v1/update", &after); after.Starting != nil {
		t.Fatalf("still starting: %+v", after.Starting)
	}
	used := b.s.updateHistory(t.Context())[0].Backup
	if _, _, err := b.s.takeUpdateBackup(t.Context(), 0); err != nil {
		t.Fatal(err)
	}
	if bk, ok, err := st.ServerBackup(t.Context(), used); err != nil || !ok || bk.PinnedUntil.IsZero() {
		t.Fatalf("the used backup's pin: %+v %v %v", bk, ok, err)
	}
}
