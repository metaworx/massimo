// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	// The file too: the restore's own backup, made the same day, prunes
	// this one from the local target by its retention.
	made, _ := filepath.Glob(filepath.Join(dir, "backups", "*.zip"))
	if len(made) != 1 {
		t.Fatalf("backups %v", made)
	}
	file := filepath.Join(t.TempDir(), "backup.zip")
	if b, err := os.ReadFile(made[0]); err != nil || os.WriteFile(file, b, 0o600) != nil {
		t.Fatal(err)
	}
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
	// Beside a serve it did not find, which has the store open (s here),
	// the restore is refused, and so it is while another job runs.
	if err := c.restore(ctx, file, restoreArgs{scopes: []string{"users"}, keyFile: keyFile}); !errors.Is(err, store.ErrInUse) {
		t.Fatalf("beside an open store: %v", err)
	}
	unlock, err := serverbackup.LockJobs(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	c.lockWait = 100 * time.Millisecond
	if err := c.restore(ctx, file, restoreArgs{scopes: []string{"users"}, keyFile: keyFile}); err == nil || !strings.Contains(err.Error(), "another job") {
		t.Fatalf("beside another job: %v", err)
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
		case "GET /api/v1/server-backups":
			json.NewEncoder(w).Encode(map[string]any{"backups": []any{}})
		case "POST /api/v1/server-backups/upload":
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{"backup": map[string]any{"uuid": uuid, "copies": []any{map[string]any{"target": "t-local"}}}})
		case "DELETE /api/v1/server-backups/" + uuid + "/copies/t-local":
			json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case "POST /api/v1/server-backups/" + uuid + "/restore/plan":
			json.NewEncoder(w).Encode(map[string]any{"backup": map[string]any{"uuid": uuid, "schema": 100}, "identity": map[string]any{"this_box": false, "fingerprint": "SHA256:x", "answers": true},
				"scopes": []string{"users"}, "report": map[string]any{"tables": []any{map[string]any{"scope": "users", "table": "users", "restored": 2, "replaced": 1, "removed": 1}}}})
		case "POST /api/v1/server-backups/" + uuid + "/restore":
			if strings.Contains(string(body), `"cube:KEQ0000000"`) {
				w.WriteHeader(409)
				json.NewEncoder(w).Encode(map[string]any{"error": "store: the backup's rows name rows of other scopes", "code": "missing",
					"report": map[string]any{"missing": []any{map[string]any{"table": "users", "uuid": "u-1", "name": "bob", "by": "commands"}}}})
				return
			}
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
	if !strings.Contains(out.String(), "uploaded") || !strings.Contains(out.String(), "another box (SHA256:x), which still answers") || !strings.Contains(out.String(), "2 from the backup, 1 replaced, 1 removed") ||
		!strings.Contains(out.String(), "the upload removed again") {
		t.Fatalf("plan:\n%s", out.String())
	}
	out.Reset()
	if err := c.restore(ctx, backup, restoreArgs{scopes: []string{"cube:KEQ0000000"}, keyFile: keyFile}); err == nil || !strings.Contains(out.String(), "users bob (u-1), named by commands") {
		t.Fatalf("a refusal %v:\n%s", err, out.String())
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
	for _, want := range []string{"POST /api/v1/server-backups/upload a backup", `"take_identity":true`, `"scopes":["users"]`, `"key_file":"# created:`,
		"DELETE /api/v1/server-backups/" + uuid + "/copies/t-local"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %s in\n%s", want, got)
		}
	}
	if _, err := os.Stat(filepath.Join(c.dir, store.FileName)); err == nil {
		t.Fatal("a store made beside the serve")
	}
}

// A new box, a store without a regular user and without a key, restores
// on the store without a backup of itself first, says so on standard
// error, and takes the old box's users; a store with a key and still no
// user takes its backup first, as one with users does, and says nothing
// of a new box.
func TestStoreRestoreNewBox(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	old := storekit.Open(t, "")
	if _, err := old.AddUser(ctx, store.User{Name: "olga", Role: store.RoleOwner}, "x", 0); err != nil {
		t.Fatal(err)
	}
	k, err := serverbackup.GenerateKey(serverbackup.KindX25519)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "old-box.zip")
	if _, err := serverbackup.Write(ctx, serverbackup.Inputs{Store: old, Trigger: serverbackup.TriggerManual, Version: "test"}, []serverbackup.Recipient{k.Recipient}, file); err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(t.TempDir(), "key.txt")
	os.WriteFile(keyFile, k.File(time.Now()), 0o600)
	restore := func(dir string) (string, string, error) {
		var out, notes bytes.Buffer
		c := &backupCLI{dir: dir, out: &out, notesTo: &notes, log: slog.New(slog.DiscardHandler)}
		err := c.restore(ctx, file, restoreArgs{scopes: []string{"all"}, keyFile: keyFile})
		return out.String(), notes.String(), err
	}
	look := func(dir string) (bool, int) {
		st, err := store.Open(dir, store.Options{Version: "test", Logger: slog.New(slog.DiscardHandler)})
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		_, ok, _ := st.User(ctx, "olga")
		bs, _ := st.ServerBackups(ctx)
		return ok, len(bs)
	}

	fresh := t.TempDir()
	storekit.Open(t, fresh).Close()
	out, notes, err := restore(fresh)
	if err != nil || !strings.Contains(notes, newBoxNote) || !strings.Contains(out, "restored settings, users") || strings.Contains(out, "pinned") {
		t.Fatalf("a new box: %v\n%s\nnotes: %s", err, out, notes)
	}
	if olga, backups := look(fresh); !olga || backups != 0 {
		t.Errorf("a new box: olga %v, %d backups of the store", olga, backups)
	}

	keyed := t.TempDir()
	st := storekit.Open(t, keyed)
	own, _ := serverbackup.GenerateKey(serverbackup.KindX25519)
	if _, err := st.AddServerBackupKey(ctx, store.ServerBackupKey{Label: "own", Recipient: own.Recipient.Line, Fingerprint: own.Recipient.Fingerprint, Kind: string(own.Recipient.Kind)}, "", 0); err != nil {
		t.Fatal(err)
	}
	st.Close()
	out, notes, err = restore(keyed)
	if err != nil || notes != "" || !strings.Contains(out, "pinned for 30 days") {
		t.Fatalf("a store with a key: %v\n%s\nnotes: %s", err, out, notes)
	}
	if olga, backups := look(keyed); !olga || backups != 1 {
		t.Errorf("a store with a key: olga %v, %d backups of the store", olga, backups)
	}
}

// Through a new box's serve, open and without a key, store restore goes
// by the first owner's window's routes: the file handed, its restore of
// every scope without scopes in the body, the note on standard error; a
// backup's id and fewer scopes are refused before anything is sent.
func TestStoreRestoreNewBoxThroughServe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var mu sync.Mutex
	var seen []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path+" "+strings.TrimSpace(string(body)))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/me":
			json.NewEncoder(w).Encode(map[string]any{"mode": "open"})
		case "GET /api/v1/server-backups":
			json.NewEncoder(w).Encode(map[string]any{"backups": []any{}, "keys": 0})
		case "POST /api/v1/setup/backup":
			json.NewEncoder(w).Encode(map[string]any{"backup": map[string]any{"uuid": "01a21e07-8cd9-72f9-a4b2-54277e08f80e"}})
		case "POST /api/v1/setup/backup/restore":
			json.NewEncoder(w).Encode(map[string]any{"restart": map[string]any{"started": true, "by": "systemd"}, "scopes": serverbackup.Scopes})
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
	var out, notes bytes.Buffer
	c := &backupCLI{dir: t.TempDir(), api: &serveAPI{base: ts.URL, client: ts.Client()}, out: &out, notesTo: &notes}
	if err := c.restore(ctx, backup, restoreArgs{scopes: []string{"users"}, keyFile: keyFile}); err == nil || !strings.Contains(err.Error(), "-scope all") {
		t.Fatalf("fewer scopes: %v", err)
	}
	if err := c.restore(ctx, "01a21e07", restoreArgs{scopes: []string{"all"}, keyFile: keyFile}); err == nil || !strings.Contains(err.Error(), "give the file") {
		t.Fatalf("a backup's id: %v", err)
	}
	if err := c.restore(ctx, backup, restoreArgs{scopes: []string{"all"}, keyFile: keyFile}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "handed") || !strings.Contains(out.String(), "the server restarts (systemd)") || !strings.Contains(notes.String(), newBoxNote) {
		t.Fatalf("restore:\n%s\nnotes: %s", out.String(), notes.String())
	}
	mu.Lock()
	got := strings.Join(seen, "\n")
	mu.Unlock()
	if !strings.Contains(got, "POST /api/v1/setup/backup a backup") || !strings.Contains(got, `POST /api/v1/setup/backup/restore {"env":false,"key_file":"# created:`) ||
		strings.Contains(got, `"scopes"`) || strings.Contains(got, "/api/v1/server-backups/upload") || strings.Count(got, "POST /api/v1/setup/backup ") != 1 {
		t.Errorf("requests:\n%s", got)
	}
}
