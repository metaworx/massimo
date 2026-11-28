// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// catalogue is a catalogue over a test store with a key, on a clock the
// test moves, in UTC.
func catalogue(t *testing.T) (*Catalogue, *atomic.Int64) {
	t.Helper()
	st := testStore(t)
	r, _ := keyOf(t, KindX25519, "laptop")
	if _, err := st.AddServerBackupKey(context.Background(), store.ServerBackupKey{Label: r.Label, Recipient: r.Line, Fingerprint: r.Fingerprint, Kind: string(r.Kind)}, "", 0); err != nil {
		t.Fatal(err)
	}
	var at atomic.Int64
	at.Store(time.Date(2026, 11, 27, 3, 30, 0, 0, time.UTC).UnixNano())
	return &Catalogue{Store: st, Version: "v0.0.0-test", Location: time.UTC, Log: slog.New(slog.DiscardHandler), Now: func() time.Time { return time.Unix(0, at.Load()).UTC() }}, &at
}

// addLocal adds a local target in the data directory's dir.
func addLocal(t *testing.T, c *Catalogue, name, dir string, takes Takes, p Policy) store.ServerBackupTarget {
	t.Helper()
	ret, _ := json.Marshal(p)
	tg, err := c.Store.AddServerBackupTarget(context.Background(), store.ServerBackupTarget{Name: name, Kind: KindLocal, Config: map[string]string{"path": dir},
		Takes: string(takes), Retention: ret, Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

// onDisk lists the backups in the data directory's dir.
func onDisk(t *testing.T, c *Catalogue, dir string) []string {
	t.Helper()
	l, _ := NewLocal(filepath.Join(c.Store.Dir(), dir))
	es, err := l.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name)
	}
	return out
}

// No key, no backup; a catalogue without a target makes the local one.
func TestMakeWithoutKeyOrTarget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c, _ := catalogue(t)
	keys, _ := c.Store.ServerBackupKeys(ctx)
	c.Store.DeleteServerBackupKey(ctx, keys[0].UUID)
	if _, _, err := c.Make(ctx, MakeRequest{Trigger: TriggerScheduled}); !errors.Is(err, ErrNoKey) {
		t.Fatalf("no key: %v", err)
	}
	if bs, _ := c.Store.ServerBackups(ctx); len(bs) != 0 {
		t.Fatalf("a backup without a key: %+v", bs)
	}
	c2, _ := catalogue(t)
	b, outs, err := c2.Make(ctx, MakeRequest{Trigger: TriggerManual, UserID: 0, Label: "first"})
	if err != nil || b.State != store.ServerBackupMade || len(b.Copies) != 1 || b.Copies[0].State != store.CopyThere || len(outs) != 3 || outs[0].Step != "make" || outs[0].Err != nil {
		t.Fatalf("make %+v %+v %v", b, outs, err)
	}
	ts, _ := c2.Store.ServerBackupTargets(ctx)
	if len(ts) != 1 || ts[0].Name != DefaultLocalName || ts[0].Config["path"] != DefaultLocalDir {
		t.Fatalf("targets %+v", ts)
	}
	m, _, err := ParseManifest(Raw{Manifest: b.Manifest, Signature: []byte(b.Signature)})
	if err != nil || m.Label != "first" || m.UUID != b.UUID {
		t.Fatalf("the manifest kept %+v %v", m, err)
	}
	if got := onDisk(t, c2, DefaultLocalDir); len(got) != 1 || got[0] != FileName(m) {
		t.Fatalf("on disk %v", got)
	}
	if entries, _ := filepath.Glob(filepath.Join(c2.Store.Dir(), tempPrefix+"*")); len(entries) != 0 {
		t.Fatalf("temporaries left %v", entries)
	}
}

// A store opened in its own directory, ".", as serve opens it when run
// there without -data, makes the local target and its copy all the
// same: the catalogue reads a target's relative path against the
// directory made absolute, which a target asks for.
func TestMakeInARelativeDirectory(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	t.Chdir(dir)
	st := storekit.Open(t, ".")
	r, _ := keyOf(t, KindX25519, "laptop")
	if _, err := st.AddServerBackupKey(ctx, store.ServerBackupKey{Label: r.Label, Recipient: r.Line, Fingerprint: r.Fingerprint, Kind: string(r.Kind)}, "", 0); err != nil {
		t.Fatal(err)
	}
	c := &Catalogue{Store: st, Version: "v0.0.0-test", Location: time.UTC, Log: slog.New(slog.DiscardHandler)}
	b, outs, err := c.Make(ctx, MakeRequest{Trigger: TriggerManual})
	if err != nil || len(b.Copies) != 1 || b.Copies[0].State != store.CopyThere {
		t.Fatalf("make %+v %+v %v", b, outs, err)
	}
	if got, _ := filepath.Glob(filepath.Join(dir, DefaultLocalDir, "massimo-*.zip")); len(got) != 1 {
		t.Fatalf("on disk %v", got)
	}
}

// Each target takes what it takes and keeps what its retention keeps; a
// copy that fails is retried, and a backup is pulled, scanned, checked,
// deleted.
func TestCatalogueJobs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c, at := catalogue(t)
	every := addLocal(t, c, "every", "every", TakesEvery, Policy{Last: 1})
	daily := addLocal(t, c, "daily", "daily", TakesDaily, Policy{Daily: 7})
	b1, _, err := c.Make(ctx, MakeRequest{Trigger: TriggerScheduled})
	if err != nil || len(b1.Copies) != 2 {
		t.Fatalf("first %+v %v", b1, err)
	}
	at.Add(int64(time.Hour))
	b2, outs, err := c.Make(ctx, MakeRequest{Trigger: TriggerManual})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := b2.Copy(daily.UUID); ok || len(b2.Copies) != 1 {
		t.Fatalf("the daily target took a second of the day: %+v", b2.Copies)
	}
	// every keeps the last one: b1 went from it, and stays on daily
	if got := onDisk(t, c, "every"); len(got) != 1 || !strings.Contains(got[0], b2.UUID) {
		t.Fatalf("every holds %v", got)
	}
	if b, _, _ := c.Store.ServerBackup(ctx, b1.UUID); len(b.Copies) != 1 || b.Copies[0].TargetUUID != daily.UUID {
		t.Fatalf("b1 %+v", b.Copies)
	}
	pruned := false
	for _, o := range outs {
		pruned = pruned || o.Step == "prune" && o.Removed == 1 && o.Target == "every"
	}
	if !pruned {
		t.Fatalf("outcomes %+v", outs)
	}

	// A copy that fails is retried after 5 min, 30 min and 2 h, then
	// told.
	blocked := filepath.Join(c.Store.Dir(), "blocked")
	if err := os.WriteFile(blocked, []byte("a file where the directory goes"), 0o600); err != nil {
		t.Fatal(err)
	}
	nas := addLocal(t, c, "nas", "blocked", TakesEvery, Policy{})
	at.Add(24 * int64(time.Hour))
	b3, outs, err := c.Make(ctx, MakeRequest{Trigger: TriggerScheduled})
	if err != nil {
		t.Fatal(err)
	}
	cp, _ := b3.Copy(nas.UUID)
	if cp.State != store.CopyFailed || cp.Tries != 1 || !cp.NextTry.Equal(c.now().Add(5*time.Minute)) {
		t.Fatalf("the failed copy %+v; %+v", cp, outs)
	}
	var last Outcome
	for i, wait := range []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour} {
		at.Add(int64(wait - time.Second))
		if _, outs, _ := c.RetryDue(ctx, b3.UUID, c.now()); len(outs) != 0 {
			t.Fatalf("retry %d before its time: %+v", i+1, outs)
		}
		at.Add(int64(time.Second))
		_, outs, _ := c.RetryDue(ctx, b3.UUID, c.now())
		if len(outs) != 1 || outs[0].Err == nil || outs[0].Tries != i+2 {
			t.Fatalf("retry %d: %+v", i+1, outs)
		}
		last = outs[0]
	}
	if !last.Spent || !last.NextTry.IsZero() {
		t.Fatalf("after the third retry: %+v", last)
	}
	// Spent, it waits for a retry by hand, which is told no more.
	at.Add(int64(24 * time.Hour))
	if _, outs, _ := c.RetryDue(ctx, b3.UUID, c.now()); len(outs) != 0 {
		t.Fatalf("a spent copy retried by the schedule: %+v", outs)
	}
	if _, outs, _ := c.Retry(ctx, b3.UUID); len(outs) != 1 || outs[0].Spent || !outs[0].NextTry.IsZero() || outs[0].Tries != 5 {
		t.Fatalf("a retry by hand of a spent copy: %+v", outs)
	}
	os.Remove(blocked)
	if b, outs, err := c.Retry(ctx, b3.UUID); err != nil || len(outs) != 2 || outs[0].Err != nil {
		t.Fatalf("retry by hand: %+v %v", outs, err)
	} else if cp, _ := b.Copy(nas.UUID); cp.State != store.CopyThere || cp.Tries != 0 {
		t.Fatalf("retried %+v", cp)
	}

	// Pulled from daily to the local target, b1 stays there whatever its
	// retention; there are three local targets, the first takes it.
	b, o, err := c.Pull(ctx, b1.UUID, "daily")
	if err != nil || o.Err != nil {
		t.Fatalf("pull %+v %v", o, err)
	}
	if cp, _ := b.Copy(every.UUID); cp.State != store.CopyThere || !cp.Pulled {
		t.Fatalf("pulled %+v", b.Copies)
	}
	if n, err := c.Prune(ctx, every); err != nil || n != 0 {
		t.Fatalf("a prune of the pulled one: %d %v", n, err)
	}

	// A target scanned: a file the catalogue lost is found again, with
	// its manifest; a copy gone from the disk is missing. b2 went from
	// every when b3 came, and with it from the catalogue.
	if _, ok, _ := c.Store.ServerBackup(ctx, b2.UUID); ok {
		t.Fatal("b2 stayed in the catalogue without a copy")
	}
	if err := c.Store.DeleteServerBackup(ctx, b3.UUID); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(c.Store.Dir(), "daily", FileName(mustManifest(t, b1))))
	added, skipped, err := c.ScanTarget(ctx, every)
	if err != nil || added != 1 || len(skipped) != 0 {
		t.Fatalf("scan every: %d %v %v", added, skipped, err)
	}
	found, ok, _ := c.Store.ServerBackup(ctx, b3.UUID)
	if !ok || found.SHA256 != "" || found.Trigger != string(TriggerScheduled) || found.UserID != 0 || len(found.Copies) != 1 {
		t.Fatalf("found %+v %v", found, ok)
	}
	if _, _, err := c.ScanTarget(ctx, daily); err != nil {
		t.Fatal(err)
	}
	if b, _, _ := c.Store.ServerBackup(ctx, b1.UUID); func() string { cp, _ := b.Copy(daily.UUID); return cp.State }() != store.CopyMissing {
		t.Fatalf("b1 on daily after its file went: %+v", b.Copies)
	}

	// A check reads it back and learns its hash; a changed copy fails.
	if got, outs, err := c.Check(ctx, b3.UUID, ""); err != nil || outs[0].Err != nil || got.SHA256 == "" {
		t.Fatalf("check %+v %+v %v", got, outs, err)
	}
	p := filepath.Join(c.Store.Dir(), "every", FileName(mustManifest(t, b3)))
	data, _ := os.ReadFile(p)
	data[len(data)/2] ^= 0xff
	os.WriteFile(p, data, 0o600)
	if got, outs, _ := c.Check(ctx, b3.UUID, "every"); outs[0].Err == nil {
		t.Fatalf("a changed copy checked: %+v", got)
	} else if cp, _ := got.Copy(every.UUID); cp.State != store.CopyFailed || cp.CheckedAt.IsZero() {
		t.Fatalf("the changed copy %+v", cp)
	}

	// A copy deleted takes the file; the backup goes with its last one,
	// b3's on daily, which the scan of daily found.
	if err := c.DeleteCopy(ctx, b3.UUID, "every"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the file stayed: %v", err)
	}
	if b, ok, _ := c.Store.ServerBackup(ctx, b3.UUID); !ok || len(b.Copies) != 1 || b.Copies[0].TargetUUID != daily.UUID {
		t.Fatalf("b3 after its copy on every went: %+v", b.Copies)
	}
	if err := c.DeleteCopy(ctx, b3.UUID, "daily"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := c.Store.ServerBackup(ctx, b3.UUID); ok {
		t.Fatal("b3 stayed without a copy")
	}
}

// mustManifest is a catalogued backup's manifest.
func mustManifest(t *testing.T, b store.ServerBackup) Manifest {
	t.Helper()
	m, _, err := ParseManifest(Raw{Manifest: b.Manifest, Signature: []byte(b.Signature)})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// A scheduled backup that fails is kept as failed with its retry, and
// made in its row by the retry.
func TestMakeFailedAndRetried(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c, at := catalogue(t)
	c.Version = "not\x00printable"
	b, outs, err := c.Make(ctx, MakeRequest{Trigger: TriggerScheduled})
	if err == nil || b.State != store.ServerBackupFailed || b.Tries != 1 || !b.NextTry.Equal(c.now().Add(5*time.Minute)) || len(outs) != 1 || outs[0].Spent {
		t.Fatalf("failed %+v %+v %v", b, outs, err)
	}
	c.Version = "v0.0.0-test"
	at.Add(int64(5 * time.Minute))
	got, _, err := c.Retry(ctx, b.UUID)
	if err != nil || got.UUID != b.UUID || got.State != store.ServerBackupMade || got.Tries != 0 || !got.NextTry.IsZero() || len(got.Copies) != 1 {
		t.Fatalf("retried %+v %v", got, err)
	}
	// A backup made by hand that fails is neither kept nor retried.
	c.Version = "not\x00printable"
	if b, outs, err := c.Make(ctx, MakeRequest{Trigger: TriggerManual}); err == nil || b.UUID != "" || len(outs) != 0 {
		t.Fatalf("a manual one kept: %+v %+v %v", b, outs, err)
	}
	if bs, _ := c.Store.ServerBackups(ctx); len(bs) != 1 {
		t.Fatalf("rows %+v", bs)
	}
}

// A scheduled backup that no target takes is retried in its row, its
// tries counted on across the file made again each time, its copies no
// copies; after the third retry it is told once and waits.
func TestMakeRetriesEnd(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c, at := catalogue(t)
	blocked := filepath.Join(c.Store.Dir(), "blocked")
	os.WriteFile(blocked, []byte("x"), 0o600)
	addLocal(t, c, "nas", "blocked", TakesEvery, Policy{})
	b, outs, err := c.Make(ctx, MakeRequest{Trigger: TriggerScheduled})
	if err == nil || b.State != store.ServerBackupFailed || b.Tries != 1 || len(b.Copies) != 0 || len(outs) != 1 || outs[0].Step != "make" {
		t.Fatalf("make %+v %+v %v", b, outs, err)
	}
	spent := 0
	for i := 0; i < 8; i++ {
		cur, _, _ := c.Store.ServerBackup(ctx, b.UUID)
		if cur.NextTry.IsZero() {
			break
		}
		at.Store(cur.NextTry.UnixNano())
		got, outs, _ := c.RetryDue(ctx, b.UUID, c.now())
		if got.UUID != b.UUID || got.Tries != cur.Tries+1 || len(got.Copies) != 0 || len(outs) != 1 {
			t.Fatalf("retry %d: %+v %+v", i+1, got, outs)
		}
		if outs[0].Spent {
			spent++
		}
	}
	if cur, _, _ := c.Store.ServerBackup(ctx, b.UUID); cur.Tries != 4 || !cur.NextTry.IsZero() || spent != 1 {
		t.Fatalf("after the retries: tries %d next %v, told %d times", cur.Tries, cur.NextTry, spent)
	}
}

// A retry of a failed scheduled backup that ends before its file, the
// key gone, is a try too: its next try moves on.
func TestMakeRetryWithoutKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c, at := catalogue(t)
	c.Version = "not\x00printable"
	b, _, _ := c.Make(ctx, MakeRequest{Trigger: TriggerScheduled})
	keys, _ := c.Store.ServerBackupKeys(ctx)
	c.Store.DeleteServerBackupKey(ctx, keys[0].UUID)
	at.Store(b.NextTry.UnixNano())
	got, outs, err := c.RetryDue(ctx, b.UUID, c.now())
	if !errors.Is(err, ErrNoKey) || got.Tries != 2 || !got.NextTry.After(c.now()) || len(outs) != 1 {
		t.Fatalf("retry without a key: %+v %+v %v", got, outs, err)
	}
	// A scheduled run with no key at all makes nothing, no row.
	c2, _ := catalogue(t)
	keys, _ = c2.Store.ServerBackupKeys(ctx)
	c2.Store.DeleteServerBackupKey(ctx, keys[0].UUID)
	if _, _, err := c2.Make(ctx, MakeRequest{Trigger: TriggerScheduled}); !errors.Is(err, ErrNoKey) {
		t.Fatal(err)
	}
	if bs, _ := c2.Store.ServerBackups(ctx); len(bs) != 0 {
		t.Fatalf("rows without a key: %+v", bs)
	}
}

// hangTarget is a target whose put waits for its context's end.
type hangTarget struct{}

func (hangTarget) Put(ctx context.Context, _ string, _ io.Reader, _ int64) error {
	hangPut <- struct{}{}
	<-ctx.Done()
	return ctx.Err()
}
func (hangTarget) Open(context.Context, string) (File, int64, error) { return nil, 0, io.EOF }
func (hangTarget) List(context.Context) ([]Entry, error)             { return nil, nil }
func (hangTarget) Delete(context.Context, string) error              { return nil }
func (hangTarget) Check(context.Context) error                       { return nil }

// hangPut says a hangTarget's put has begun.
var hangPut = make(chan struct{}, 4)

func init() {
	Register(Kind{Name: "hangtest", New: func(map[string]string, []byte) (Target, error) { return hangTarget{}, nil }})
}

// A stop in the middle of the copies, its context ended: a scheduled
// backup stays a failed row with its retry, never one made without a
// file, and can be deleted; one made by hand goes.
func TestMakeBrokenOff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c, _ := catalogue(t)
	if _, err := c.Store.AddServerBackupTarget(ctx, store.ServerBackupTarget{Name: "remote", Kind: "hangtest", Enabled: true}, 0); err != nil {
		t.Fatal(err)
	}
	for _, trigger := range []Trigger{TriggerScheduled, TriggerManual} {
		stop, cancel := context.WithCancel(ctx)
		go func() { <-hangPut; cancel() }()
		_, _, err := c.Make(stop, MakeRequest{Trigger: trigger})
		if err == nil {
			t.Fatalf("%s: made", trigger)
		}
	}
	bs, _ := c.Store.ServerBackups(ctx)
	if len(bs) != 1 || bs[0].State != store.ServerBackupFailed || bs[0].Trigger != string(TriggerScheduled) || bs[0].NextTry.IsZero() || len(bs[0].Copies) != 0 {
		t.Fatalf("rows %+v", bs)
	}
	if err := c.Delete(ctx, bs[0].UUID); err != nil {
		t.Fatal(err)
	}
}

// A retry keeps the request: a backup made by hand that failed is not
// made again (ErrNotRetried); a failed copy of one is the same file put
// again, by hand alone, its pin kept, no backup made anew.
func TestRetryKeepsTheRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c, at := catalogue(t)
	blocked := filepath.Join(c.Store.Dir(), "blocked")
	os.WriteFile(blocked, []byte("x"), 0o600)
	addLocal(t, c, "local", "local", TakesEvery, Policy{})
	nas := addLocal(t, c, "nas", "blocked", TakesEvery, Policy{})
	b, _, err := c.Make(ctx, MakeRequest{Trigger: TriggerBeforeUpdate, Local: true, PinFor: PinBefore})
	if err != nil {
		t.Fatal(err)
	}
	cp, _ := b.Copy(nas.UUID)
	if cp.State != store.CopyFailed || !cp.NextTry.IsZero() || !b.Pinned(c.now()) {
		t.Fatalf("made %+v", b)
	}
	at.Add(int64(time.Hour))
	if _, outs, _ := c.RetryDue(ctx, b.UUID, c.now()); len(outs) != 0 {
		t.Fatalf("the schedule retried a copy of a backup before an update: %+v", outs)
	}
	os.Remove(blocked)
	got, outs, err := c.Retry(ctx, b.UUID)
	if err != nil || len(outs) != 2 || outs[0].Err != nil || got.UUID != b.UUID || got.SHA256 != b.SHA256 || !got.Pinned(c.now()) {
		t.Fatalf("retry by hand %+v %+v %v", got, outs, err)
	}
	if bs, _ := c.Store.ServerBackups(ctx); len(bs) != 1 {
		t.Fatalf("a backup made anew: %d", len(bs))
	}
	f, err := os.ReadFile(filepath.Join(c.Store.Dir(), "blocked", FileName(mustManifest(t, b))))
	if err != nil || int64(len(f)) != b.Size {
		t.Fatalf("the copy pushed: %d %v", len(f), err)
	}
	// A failed backup made by hand is not retried.
	if _, err := c.Store.SaveServerBackup(ctx, store.ServerBackup{UUID: "01a21e2c-0000-7000-8000-0000000000aa", Trigger: string(TriggerManual), State: store.ServerBackupFailed, Error: "x", Tries: 1}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Retry(ctx, "01a21e2c-0000-7000-8000-0000000000aa"); !errors.Is(err, ErrNotRetried) {
		t.Fatalf("a failed manual backup retried: %v", err)
	}
	if err := c.Delete(ctx, b.UUID); !errors.Is(err, ErrHasCopies) {
		t.Fatalf("a backup with copies deleted: %v", err)
	}
}

// One job at a time on a data directory: a second lock waits, and is
// refused when its context ends first.
func TestLockJobs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	unlock, err := LockJobs(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, err := LockJobs(short, dir); !errors.Is(err, ErrJobRunning) {
		t.Fatalf("a second lock: %v", err)
	}
	unlock()
	again, err := LockJobs(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	again()
}

// An upload is refused where it is no backup or was changed, and taken
// to the local target, kept there, as uploaded.
func TestUpload(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c, _ := catalogue(t)
	other := testStore(t)
	r, _ := keyOf(t, KindX25519, "laptop")
	path, m := write(t, other, Inputs{Trigger: TriggerScheduled}, r)
	junk := filepath.Join(t.TempDir(), "junk.zip")
	os.WriteFile(junk, []byte("no zip"), 0o600)
	if _, _, err := c.Upload(ctx, junk, 0); !errors.Is(err, ErrNotABackup) {
		t.Fatalf("junk: %v", err)
	}
	b, o, err := c.Upload(ctx, path, 0)
	if err != nil || o.Err != nil || b.UUID != m.UUID || b.Trigger != string(TriggerUploaded) || len(b.Copies) != 1 || !b.Copies[0].Pulled || b.SHA256 == "" {
		t.Fatalf("upload %+v %+v %v", b, o, err)
	}
	if mm := mustManifest(t, b); mm.Trigger != TriggerScheduled {
		t.Fatalf("the manifest's trigger %s", mm.Trigger)
	}
	if _, o, err := c.Upload(ctx, path, 0); err != nil || o.Err != nil {
		t.Fatalf("again: %+v %v", o, err)
	}
	f, size, name, err := c.OpenCopy(ctx, b.UUID)
	if err != nil || name != FileName(m) || size != b.Size {
		t.Fatalf("open %s %d %v", name, size, err)
	}
	f.Close()
}

// A backup no target takes, its targets switched off, failed: the file
// is gone with the temporary directory, and the backup is retried.
func TestMakeWithoutATargetOn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c, _ := catalogue(t)
	off := addLocal(t, c, "off", "off", TakesEvery, Policy{})
	if _, err := c.Store.UpdateServerBackupTarget(ctx, off.UUID, func(t *store.ServerBackupTarget) error { t.Enabled = false; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	b, outs, err := c.Make(ctx, MakeRequest{Trigger: TriggerScheduled})
	if err == nil || b.State != store.ServerBackupFailed || b.Tries != 1 || b.NextTry.IsZero() || len(outs) == 0 || outs[len(outs)-1].Step != "make" {
		t.Fatalf("make %+v %+v %v", b, outs, err)
	}
}
