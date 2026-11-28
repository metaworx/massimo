// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// jobWait is how long a test waits for a job of the loop, a backup made
// and copied, on a machine busy with the race detector and other tests.
var jobWait = testkit.For(30 * time.Second)

// stubTarget is a target that holds nothing and takes everything.
type stubTarget struct{}

func (stubTarget) Put(context.Context, string, io.Reader, int64) error { return nil }
func (stubTarget) Open(context.Context, string) (serverbackup.File, int64, error) {
	return nil, 0, os.ErrNotExist
}
func (stubTarget) List(context.Context) ([]serverbackup.Entry, error) { return nil, nil }
func (stubTarget) Delete(context.Context, string) error               { return nil }
func (stubTarget) Check(context.Context) error                        { return nil }

func init() {
	// a kind with a secret field of a key secretFields does not name
	serverbackup.Register(serverbackup.Kind{Name: "vaulttest", Fields: []serverbackup.Field{
		{Key: "url", Type: serverbackup.FieldText, Required: true},
		{Key: "passphrase", Type: serverbackup.FieldSecret},
	}, New: func(map[string]string, []byte) (serverbackup.Target, error) { return stubTarget{}, nil }})
}

// blockedTarget adds a local target whose directory is a file: its copies
// fail until the file goes, which the function returned removes.
func (r *sbRig) blockedTarget(t *testing.T) func() {
	t.Helper()
	blocked := filepath.Join(r.st.Dir(), "blocked")
	os.WriteFile(blocked, []byte("x"), 0o600)
	n := r.s.sb.armed.Load()
	if code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backup-targets", "secret", `{"name":"nas","kind":"local","config":{"path":"blocked"}}`); code != 201 {
		t.Fatalf("add a target: %d %s", code, text)
	}
	testkit.Eventually(t, "the wake of a target added", func() bool { return r.s.sb.armed.Load() > n }, jobWait)
	return func() { os.Remove(blocked) }
}

// advance moves the clock by d and waits for the loop's next timer.
func (r *sbRig) advance(t *testing.T, d time.Duration) {
	t.Helper()
	n := r.s.sb.armed.Load()
	r.clock.Add(d)
	testkit.Eventually(t, "the loop's next timer", func() bool { return r.s.sb.armed.Load() > n }, jobWait)
}

// makeLines are the server_backup lines of step make, newest first.
func (r *sbRig) makeLines(t *testing.T) (all, warn int) {
	t.Helper()
	for _, ev := range r.lines(t) {
		var e struct {
			Summary map[string]any `json:"summary"`
		}
		json.Unmarshal(ev.Data, &e)
		if e.Summary["step"] == "make" {
			all++
			if ev.Level == store.LevelWarn {
				warn++
			}
		}
	}
	return all, warn
}

// A retry due whose failure ends before the file, the key gone, is a try
// with its next try, and the loop waits for it: it does not spin.
func TestServerBackupLoopDoesNotSpin(t *testing.T) {
	t.Parallel()
	o := storeOptions(t, t.TempDir(), "secret")
	o.serverBackupZone = time.UTC
	now := time.Now()
	const id = "01a21e2c-0000-7000-8000-000000000001"
	if _, err := o.Store.SaveServerBackup(t.Context(), store.ServerBackup{UUID: id, Time: now, Trigger: "scheduled",
		State: store.ServerBackupFailed, Error: "x", Tries: 1, NextTry: now.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	r := startServer(t, withOptions(o))
	testkit.Eventually(t, "the retry", func() bool { b, _, _ := o.Store.ServerBackup(t.Context(), id); return b.Tries == 2 }, jobWait)
	if b, _, _ := o.Store.ServerBackup(t.Context(), id); !b.NextTry.After(now) || !strings.Contains(b.Error, "no key") {
		t.Fatalf("the retry not kept: %+v", b)
	}
	n := r.s.sb.armed.Load()
	time.Sleep(500 * time.Millisecond)
	if d := r.s.sb.armed.Load() - n; d > 1 {
		t.Fatalf("the loop spins: %d timers in 500 ms", d)
	}
}

// A scheduled backup that no target takes is retried after 5 min, 30 min
// and 2 h, its tries counted on, then warned of once and left for a
// retry by hand.
func TestServerBackupRetriesEnd(t *testing.T) {
	t.Parallel()
	r, _ := startSB(t, time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	testkit.Eventually(t, "the loop's first timer", func() bool { return r.s.sb.armed.Load() > 0 })
	r.blockedTarget(t)
	r.advance(t, 5*time.Minute)
	for i := 0; i < 8; i++ {
		bs := r.backups(t)
		if len(bs) != 1 || bs[0].NextTry.IsZero() {
			break
		}
		r.advance(t, bs[0].NextTry.Sub(r.clock.Now()))
	}
	bs := r.backups(t)
	if len(bs) != 1 || bs[0].Tries != 4 || !bs[0].NextTry.IsZero() || len(bs[0].Copies) != 0 {
		t.Fatalf("after the retries: %+v", bs)
	}
	testkit.Eventually(t, "the lines", func() bool { all, _ := r.makeLines(t); return all == 4 })
	if _, warn := r.makeLines(t); warn != 1 {
		t.Fatalf("%d warnings, want 1", warn)
	}
}

// A make that failed until its retries were spent is an alert, settled
// by the next backup made, whose line says so.
func TestServerBackupMakeAlertSettles(t *testing.T) {
	t.Parallel()
	var broken atomic.Bool
	broken.Store(true)
	clock := testkit.NewClock(time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	o := storeOptions(t, t.TempDir(), "secret")
	o.serverBackupClock, o.serverBackupZone = kitClock{clock}, time.UTC
	o.BackupTLSFiles = func() []serverbackup.TLSFile {
		if broken.Load() {
			return []serverbackup.TLSFile{{Role: "bad", Path: "/nonexistent"}}
		}
		return nil
	}
	k, _ := serverbackup.GenerateKey(serverbackup.KindX25519)
	o.Store.AddServerBackupKey(t.Context(), store.ServerBackupKey{Label: "l", Recipient: k.Recipient.Line, Fingerprint: k.Recipient.Fingerprint, Kind: string(k.Recipient.Kind)}, "", 0)
	r := &sbRig{rig: startServer(t, withOptions(o)), clock: clock, st: o.Store}
	testkit.Eventually(t, "the loop's first timer", func() bool { return r.s.sb.armed.Load() > 0 })
	r.advance(t, 5*time.Minute)
	for _, wait := range serverbackup.RetryDelays {
		r.advance(t, wait)
	}
	var alerts logAlertsView
	has := func() bool {
		getJSON(t, r.ts, "secret", "/api/v1/log/alerts", &alerts)
		for _, a := range alerts.Alerts {
			if a.Kind == "server_backup" {
				return true
			}
		}
		return false
	}
	testkit.Eventually(t, "the alert", has)
	broken.Store(false)
	r.advance(t, time.Date(2026, 11, 28, 3, 30, 0, 0, time.UTC).Sub(r.clock.Now()))
	if bs := r.backups(t); len(bs) != 1 || bs[0].State != store.ServerBackupMade {
		t.Fatalf("the next run: %+v", bs)
	}
	testkit.Eventually(t, "the alert settled", func() bool { return !has() })
}

// A backup made by hand whose copy failed is not retried by the loop; a
// retry by hand puts the same file again.
func TestServerBackupManualNotRetried(t *testing.T) {
	t.Parallel()
	r, _ := startSB(t, time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	testkit.Eventually(t, "the loop's first timer", func() bool { return r.s.sb.armed.Load() > 0 })
	r.advance(t, 5*time.Minute) // the missed run, to the local target
	for _, b := range r.backups(t) {
		r.st.DeleteServerBackup(t.Context(), b.UUID)
	}
	unblock := r.blockedTarget(t)
	if code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backups", "secret", `{"label":"by hand"}`); code != 201 {
		t.Fatalf("by hand: %d %s", code, text)
	}
	failedCopy := func() store.ServerBackupCopy {
		for _, c := range r.backups(t)[0].Copies {
			if c.State == store.CopyFailed {
				return c
			}
		}
		return store.ServerBackupCopy{}
	}
	if c := failedCopy(); c.Tries != 1 || !c.NextTry.IsZero() {
		t.Fatalf("the failed copy %+v", c)
	}
	before := len(r.lines(t))
	r.clock.Add(time.Hour)
	n := r.s.sb.armed.Load()
	r.s.wakeServerBackups()
	testkit.Eventually(t, "the loop's wake", func() bool { return r.s.sb.armed.Load() > n }, jobWait)
	if after := len(r.lines(t)); after != before || failedCopy().Tries != 1 {
		t.Fatalf("the loop retried a backup made by hand: %d lines, %+v", after-before, failedCopy())
	}
	unblock()
	b := r.backups(t)[0]
	if code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backups/"+b.UUID+"/retry", "secret", ""); code != 200 || strings.Count(text, `"state": "there"`) != 2 {
		t.Fatalf("a retry by hand: %d %s", code, text)
	}
	if got := r.backups(t); len(got) != 1 || got[0].UUID != b.UUID || got[0].SHA256 != b.SHA256 {
		t.Fatalf("a backup made anew: %+v", got)
	}
}

// A backup with no copy goes by DELETE; one with a copy there does not.
// A failed backup made by hand is not retried.
func TestServerBackupDelete(t *testing.T) {
	t.Parallel()
	r, _ := startSB(t, time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	ctx := t.Context()
	const manual, scheduled = "01a21e2c-0000-7000-8000-0000000000b1", "01a21e2c-0000-7000-8000-0000000000b2"
	for _, b := range []store.ServerBackup{{UUID: manual, Trigger: "manual"}, {UUID: scheduled, Trigger: "scheduled"}} {
		b.State, b.Error, b.Tries = store.ServerBackupFailed, "broken off", 1
		r.st.SaveServerBackup(ctx, b)
	}
	if code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backups/"+manual+"/retry", "secret", ""); code != 409 {
		t.Fatalf("a failed manual backup retried: %d %s", code, text)
	}
	if code, text := callAPI(t, r.ts, "DELETE", "/api/v1/server-backups/"+manual, "secret", ""); code != 200 {
		t.Fatalf("delete a failed one: %d %s", code, text)
	}
	if code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backups", "secret", ""); code != 201 {
		t.Fatalf("make: %d %s", code, text)
	}
	var made store.ServerBackup
	for _, b := range r.backups(t) {
		if b.State == store.ServerBackupMade {
			made = b
		}
	}
	if code, _ := callAPI(t, r.ts, "DELETE", "/api/v1/server-backups/"+made.UUID, "secret", ""); code != 409 {
		t.Fatalf("a backup with a copy deleted: %d", code)
	}
	if code, _ := callAPI(t, r.ts, "DELETE", "/api/v1/server-backups/nope", "secret", ""); code != 404 {
		t.Fatalf("no backup: %d", code)
	}
}

// A target's secret never reaches an audit row: not at the top of the
// body, and not inside its config, where the route refuses it, for a
// kind whose secret field secretFields does not name.
func TestServerBackupSecretNotAudited(t *testing.T) {
	t.Parallel()
	r, _ := startSB(t, time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	if code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backup-targets", "secret", `{"name":"v1","kind":"vaulttest","config":{"url":"https://vault.example","passphrase":"hunter2"}}`); code != 400 {
		t.Fatalf("a secret in config: %d %s", code, text)
	}
	if code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backup-targets", "secret", `{"name":"v2","kind":"vaulttest","config":{"url":"https://vault.example"},"secret":"hunter3"}`); code != 201 {
		t.Fatalf("a secret apart: %d %s", code, text)
	}
	rows, _ := r.st.AuditList(t.Context(), time.Time{}, 100, "")
	seen := 0
	for _, a := range rows {
		if a.Action == "serverbackup.target_add" {
			seen++
		}
		if strings.Contains(string(a.Detail), "hunter") {
			t.Fatalf("an audit row holds the secret: %s", a.Detail)
		}
	}
	if seen != 2 {
		t.Fatalf("%d audit rows of target_add", seen)
	}
	if got := redacted([]byte(`[{"x":{"passphrase":"p","password":"q"}},1.50]`)); strings.Contains(string(got), `"p"`) || strings.Contains(string(got), `"q"`) || !strings.Contains(string(got), "1.50") {
		t.Fatalf("redacted %s", got)
	}
}
