// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
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
	if err != nil || b.State != store.ServerBackupMade || len(b.Copies) != 1 || b.Copies[0].State != store.CopyThere || len(outs) != 2 {
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
		at.Add(int64(wait))
		_, outs, _ := c.Retry(ctx, b3.UUID)
		if len(outs) != 1 || outs[0].Err == nil || outs[0].Tries != i+2 {
			t.Fatalf("retry %d: %+v", i+1, outs)
		}
		last = outs[0]
	}
	if !last.Spent || !last.NextTry.IsZero() {
		t.Fatalf("after the third retry: %+v", last)
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
	// A backup made by hand that fails is not retried.
	c.Version = "not\x00printable"
	if b, outs, _ := c.Make(ctx, MakeRequest{Trigger: TriggerManual}); !b.NextTry.IsZero() || outs[0].Spent {
		t.Fatalf("a manual one retried: %+v %+v", b, outs)
	}
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
