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
	"time"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// Without a running serve, store restore acts on the store: its plan
// changes nothing, the restore takes a backup first and removes a user
// added since; a file restores as an id does.
func TestStoreRestoreOnTheStore(t *testing.T) {
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
	if _, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOwner}, "x", 0); err != nil {
		t.Fatal(err)
	}
	st.Close()
	keyFile := filepath.Join(t.TempDir(), "key.txt")
	os.WriteFile(keyFile, k.File(time.Now()), 0o600)
	var out bytes.Buffer
	c := &backupCLI{dir: dir, out: &out, log: slog.New(slog.DiscardHandler)}
	if err := c.make(ctx, "", ""); err != nil {
		t.Fatal(err)
	}
	id := strings.Fields(out.String())[1]
	reopen := func() *store.Store {
		s, err := store.Open(dir, store.Options{Version: "test", Logger: slog.New(slog.DiscardHandler)})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := reopen()
	s.AddUser(ctx, store.User{Name: "late", Role: store.RoleViewer}, "x", 0)
	s.Close()

	out.Reset()
	if err := c.restore(ctx, id, restoreArgs{scopes: []string{"users"}, keyFile: keyFile, plan: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "made by this box") || !strings.Contains(out.String(), "users") {
		t.Fatalf("plan:\n%s", out.String())
	}
	s = reopen()
	if _, ok, _ := s.User(ctx, "late"); !ok {
		t.Fatal("the plan changed the store")
	}
	s.Close()
	out.Reset()
	if err := c.restore(ctx, id, restoreArgs{scopes: []string{"users"}, keyFile: keyFile}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "restored users") || !strings.Contains(out.String(), "pinned for 30 days") || !strings.Contains(out.String(), "start it") {
		t.Fatalf("restore:\n%s", out.String())
	}
	s = reopen()
	defer s.Close()
	if _, ok, _ := s.User(ctx, "late"); ok {
		t.Fatal("a user added since stayed")
	}
	if err := c.restore(ctx, id, restoreArgs{scopes: []string{"bogus"}, keyFile: keyFile}); err == nil || !strings.Contains(err.Error(), "no scope") {
		t.Fatalf("a bogus scope: %v", err)
	}
	if err := c.restore(ctx, id, restoreArgs{scopes: []string{"users"}, keyFile: filepath.Join(dir, "none")}); err == nil {
		t.Fatal("no key file")
	}
}

// With a running serve, store restore goes through its API: a file
// uploaded first, then its plan and its restore, the key in the body;
// the server's restart is said.
func TestStoreRestoreThroughServe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const uuid = "01a21e07-8cd9-72f9-a4b2-54277e08f80e"
	var mu sync.Mutex
	var seen []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path+" "+strings.TrimSpace(string(body)))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v1/server-backups/upload":
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{"backup": map[string]any{"uuid": uuid}})
		case "POST /api/v1/server-backups/" + uuid + "/restore/plan":
			json.NewEncoder(w).Encode(map[string]any{"backup": map[string]any{"uuid": uuid, "schema": 100}, "identity": map[string]any{"this_box": false, "fingerprint": "SHA256:x", "answers": true},
				"scopes": []string{"users"}, "report": map[string]any{"tables": []any{map[string]any{"scope": "users", "table": "users", "restored": 2, "replaced": 1, "removed": 1}}}})
		case "POST /api/v1/server-backups/" + uuid + "/restore":
			json.NewEncoder(w).Encode(map[string]any{"restart": map[string]any{"started": true, "by": "systemd"}, "before_backup": uuid, "scopes": []string{"users"}})
		default:
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]string{"error": "no such route"})
		}
	}))
	defer ts.Close()
	k, _ := serverbackup.GenerateKey(serverbackup.KindX25519)
	d := t.TempDir()
	keyFile, backup := filepath.Join(d, "key.txt"), filepath.Join(d, "backup.zip")
	os.WriteFile(keyFile, k.File(time.Now()), 0o600)
	os.WriteFile(backup, []byte("a backup"), 0o600)
	var out bytes.Buffer
	c := &backupCLI{dir: t.TempDir(), api: &serveAPI{base: ts.URL, token: "tok", client: ts.Client()}, out: &out}
	if err := c.restore(ctx, backup, restoreArgs{scopes: []string{"users"}, keyFile: keyFile, plan: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "uploaded") || !strings.Contains(out.String(), "another box (SHA256:x), which still answers") || !strings.Contains(out.String(), "2 from the backup, 1 replaced, 1 removed") {
		t.Fatalf("plan:\n%s", out.String())
	}
	out.Reset()
	if err := c.restore(ctx, backup, restoreArgs{scopes: []string{"users"}, keyFile: keyFile, identity: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "the server restarts (systemd)") {
		t.Fatalf("restore:\n%s", out.String())
	}
	mu.Lock()
	got := strings.Join(seen, "\n")
	mu.Unlock()
	for _, want := range []string{"POST /api/v1/server-backups/upload a backup", `"take_identity":true`, `"scopes":["users"]`, `"key_file":"# created:`} {
		if !strings.Contains(got, want) {
			t.Errorf("no %s in\n%s", want, got)
		}
	}
	if _, err := os.Stat(filepath.Join(c.dir, store.FileName)); err == nil {
		t.Fatal("a store made beside the serve")
	}
}
