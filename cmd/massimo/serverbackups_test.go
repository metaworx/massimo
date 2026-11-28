// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// Without a running serve, store backup, backups, push and pull act on
// the store: a backup made to the local target the first one makes,
// listed, pushed to a second target and pulled back from it.
func TestServerBackupCommandsOnTheStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	st := storekit.Open(t, dir)
	k, err := serverbackup.GenerateKey(serverbackup.KindX25519)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddServerBackupKey(ctx, store.ServerBackupKey{Label: "laptop", Recipient: k.Recipient.Line, Fingerprint: k.Recipient.Fingerprint, Kind: string(k.Recipient.Kind)}, "", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddServerBackupTarget(ctx, store.ServerBackupTarget{Name: "local", Kind: serverbackup.KindLocal, Config: map[string]string{"path": "backups"}, Enabled: true}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddServerBackupTarget(ctx, store.ServerBackupTarget{Name: "second", Kind: serverbackup.KindLocal, Config: map[string]string{"path": "second"}, Takes: "monthly", Enabled: true}, 0); err != nil {
		t.Fatal(err)
	}
	st.Close()
	var out bytes.Buffer
	c := &backupCLI{dir: dir, out: &out, log: slog.New(slog.DiscardHandler)}
	if err := c.make(ctx, "local", "by hand"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "made ") || !strings.Contains(out.String(), "copy local: ok") || !strings.Contains(out.String(), "on the store in "+dir) {
		t.Fatalf("make:\n%s", out.String())
	}
	out.Reset()
	if err := c.list(ctx); err != nil {
		t.Fatal(err)
	}
	listed := out.String()
	if !strings.Contains(listed, "manual") || !strings.Contains(listed, "local: there") || !strings.Contains(listed, "by hand") {
		t.Fatalf("list:\n%s", listed)
	}
	id := strings.Fields(strings.Split(listed, "\n")[1])[0]
	out.Reset()
	if err := c.copy(ctx, id, "second", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "pushed ") || !strings.Contains(out.String(), "copy second: ok") {
		t.Fatalf("push:\n%s", out.String())
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "second"))
	if len(entries) != 1 {
		t.Fatalf("second holds %v", entries)
	}
	if err := c.copy(ctx, "nope-nope-nope", "second", false); err == nil || !strings.Contains(err.Error(), "no server backup") {
		t.Fatalf("an unknown id: %v", err)
	}
	out.Reset()
	if err := c.copy(ctx, id, "second", true); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	c.list(ctx)
	if !strings.Contains(out.String(), "local: there, pulled") {
		t.Fatalf("after the pull:\n%s", out.String())
	}
}

// With a running serve, the commands go through its API with the token,
// and print its answers; an error is the API's.
func TestServerBackupCommandsThroughServe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const uuid = "01a21e07-8cd9-72f9-a4b2-54277e08f80e"
	backup := map[string]any{"uuid": uuid, "time": "2026-11-27T02:30:00Z", "trigger": "manual", "state": "made", "size": 2048,
		"copies": []any{map[string]any{"target_name": "local", "state": "there"}}}
	var mu sync.Mutex
	var seen []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path+" "+strings.TrimSpace(string(body)))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"})
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/server-backups":
			json.NewEncoder(w).Encode(map[string]any{"backups": []any{backup}})
		case "POST /api/v1/server-backups", "PUT /api/v1/server-backups/" + uuid + "/copies/nas", "POST /api/v1/server-backups/" + uuid + "/copies/nas/pull":
			json.NewEncoder(w).Encode(map[string]any{"backup": backup, "steps": []any{map[string]any{"step": "copy", "target": "nas", "ok": true}}})
		default:
			w.WriteHeader(409)
			json.NewEncoder(w).Encode(map[string]string{"error": "no key is set"})
		}
	}))
	defer ts.Close()
	var out bytes.Buffer
	c := &backupCLI{dir: t.TempDir(), api: &serveAPI{base: ts.URL, token: "tok", client: ts.Client()}, out: &out}
	if err := c.make(ctx, "nas", "x"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "made 54277e08f80e") || !strings.Contains(out.String(), "through the server at "+ts.URL) {
		t.Fatalf("make:\n%s", out.String())
	}
	out.Reset()
	if err := c.list(ctx); err != nil || !strings.Contains(out.String(), "54277e08f80e") || !strings.Contains(out.String(), "local: there") {
		t.Fatalf("list %v:\n%s", err, out.String())
	}
	if err := c.copy(ctx, "54277e08f80e", "nas", false); err != nil {
		t.Fatal(err)
	}
	if err := c.copy(ctx, "54277e08f80e", "nas", true); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	got := strings.Join(seen, "\n")
	mu.Unlock()
	for _, want := range []string{
		`POST /api/v1/server-backups {"label":"x","target":"nas"}`,
		"PUT /api/v1/server-backups/" + uuid + "/copies/nas",
		"POST /api/v1/server-backups/" + uuid + "/copies/nas/pull",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %s in\n%s", want, got)
		}
	}
	c.api.token = "wrong"
	if err := c.list(ctx); err == nil || !strings.Contains(err.Error(), "401: authentication required") {
		t.Fatalf("a wrong token: %v", err)
	}
	if _, err := os.Stat(filepath.Join(c.dir, store.FileName)); err == nil {
		t.Fatal("a store made beside the serve")
	}
}
