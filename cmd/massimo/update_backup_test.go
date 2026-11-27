// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// With a running serve, the backup before an update by hand is the
// server's, POST /api/v1/update/backup with the token: made, skipped
// without a key, or failed, which stops the update with the API's text.
func TestBackupBeforeUpdateThroughServe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const uuid = "01a21e07-8cd9-72f9-a4b2-54277e08f80e"
	answer := http.StatusCreated
	var paths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(answer)
		switch answer {
		case http.StatusCreated:
			json.NewEncoder(w).Encode(map[string]any{"backup": map[string]any{"uuid": uuid, "trigger": "before-update", "state": "made", "size": 2048},
				"steps": []any{map[string]any{"step": "copy", "target": "local", "ok": true}}})
		case http.StatusOK:
			json.NewEncoder(w).Encode(map[string]any{"skipped": "no_key"})
		default:
			json.NewEncoder(w).Encode(map[string]any{"error": "the backup of the server before the update failed, so the update did not start: disk full"})
		}
	}))
	defer ts.Close()
	var out bytes.Buffer
	c := &backupCLI{dir: t.TempDir(), api: &serveAPI{base: ts.URL, token: "tok", client: ts.Client()}, out: &out}
	if err := c.beforeUpdate(ctx); err != nil || !strings.Contains(out.String(), "backup before the update 54277e08f80e") || !strings.Contains(out.String(), "copy local: ok") {
		t.Fatalf("made: %v\n%s", err, out.String())
	}
	if len(paths) != 1 || paths[0] != "POST /api/v1/update/backup Bearer tok" {
		t.Fatalf("asked %v", paths)
	}
	out.Reset()
	answer = http.StatusOK
	if err := c.beforeUpdate(ctx); err != nil || !strings.Contains(out.String(), "no key is set") {
		t.Fatalf("no key: %v\n%s", err, out.String())
	}
	answer = http.StatusInternalServerError
	if err := c.beforeUpdate(ctx); err == nil || !strings.Contains(err.Error(), "disk full") || !strings.Contains(err.Error(), "before the update") {
		t.Fatalf("failed: %v", err)
	}
}

// Without a running serve, the backup is made on the store: local and
// pinned for 30 days; without a key, or without a store, the update goes
// on.
func TestBackupBeforeUpdateOnTheStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	st := storekit.Open(t, dir)
	st.Close()
	var out bytes.Buffer
	c := &backupCLI{dir: dir, out: &out, log: slog.New(slog.DiscardHandler)}
	if err := c.beforeUpdate(ctx); err != nil || !strings.Contains(out.String(), "no key is set") {
		t.Fatalf("no key: %v\n%s", err, out.String())
	}
	st = storekit.Open(t, dir)
	k, err := serverbackup.GenerateKey(serverbackup.KindX25519)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddServerBackupKey(ctx, store.ServerBackupKey{Label: "laptop", Recipient: k.Recipient.Line, Fingerprint: k.Recipient.Fingerprint, Kind: string(k.Recipient.Kind)}, "", 0); err != nil {
		t.Fatal(err)
	}
	st.Close()
	out.Reset()
	started := time.Now()
	if err := c.beforeUpdate(ctx); err != nil || !strings.Contains(out.String(), "backup before the update ") || !strings.Contains(out.String(), "on the store in "+dir) {
		t.Fatalf("made: %v\n%s", err, out.String())
	}
	st = storekit.Open(t, dir)
	bs, err := st.ServerBackups(ctx)
	if err != nil || len(bs) != 1 || bs[0].Trigger != string(serverbackup.TriggerBeforeUpdate) || len(bs[0].Copies) != 1 ||
		bs[0].PinnedUntil.Before(started.Add(serverbackup.PinBefore-time.Minute)) {
		t.Fatalf("the backup: %+v %v", bs, err)
	}
	st.Close()

	out.Reset()
	none := &backupCLI{dir: filepath.Join(t.TempDir(), "nothing"), out: &out}
	if err := none.beforeUpdate(ctx); err != nil || !strings.Contains(out.String(), "no store in") {
		t.Fatalf("no store: %v\n%s", err, out.String())
	}
	out.Reset()
	none.dir = ""
	if err := none.beforeUpdate(ctx); err != nil || !strings.Contains(out.String(), "no store on this host") {
		t.Fatalf("no service's store: %v\n%s", err, out.String())
	}
}
