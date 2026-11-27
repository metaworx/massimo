// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/netip"
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

// oldBoxPassword is the password of the old box's owner.
const oldBoxPassword = "violet harbour quietly turns"

// oldBoxBackup is a backup of another box: its owner olga with a
// password of version 2, which only that box's opaque.key opens, and the
// fake Cube's row; the backup's file and the key file that opens it.
func oldBoxBackup(t *testing.T) (file string, keyFile string) {
	t.Helper()
	ctx := t.Context()
	old := testStore(t, "")
	if _, err := old.AddUserOpaque(ctx, store.User{Name: "olga", Role: store.RoleOwner}, oldBoxPassword, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := old.AddCube(ctx, store.Cube{Serial: "KEQ0000100", Name: "Upstairs", Configured: true, Source: store.SourceEnv}); err != nil {
		t.Fatal(err)
	}
	k, err := serverbackup.GenerateKey(serverbackup.KindX25519)
	if err != nil {
		t.Fatal(err)
	}
	file = filepath.Join(t.TempDir(), "old-box.zip")
	if _, err := serverbackup.Write(ctx, serverbackup.Inputs{Store: old, Trigger: serverbackup.TriggerManual, Version: "test"}, []serverbackup.Recipient{k.Recipient}, file); err != nil {
		t.Fatal(err)
	}
	return file, string(k.File(time.Now()))
}

// lanCall sends a request as a client of the local network that is not
// the box itself: through the loopback, which the options trust as a
// proxy, for the address 192.168.1.20.
func lanCall(t *testing.T, ts string, method, path, token string, body []byte) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, ts+path, bytes.NewReader(body))
	req.Header.Set("X-Forwarded-For", "192.168.1.20")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// jsonBody is v as a request's body.
func jsonBody(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// A new box started from another box's backup, from a client of the
// local network: the backup handed as an upload, a file that is none
// refused, the held one kept; a key that does not parse or does not
// open it refused; the plan, which changes nothing and offers the
// restart although the server is open; then the restore of every scope,
// no backup taken first, the server restarted, the backup handed gone
// and the key written nowhere, the routes closed at once. After the
// restart the old box's owner signs in with the old password, and the
// route says to her that the window has closed.
func TestNewBoxFromBackup(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	file, keyFile := oldBoxBackup(t)
	backup, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	o := storeOptions(t, dir, "")
	o.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	var restarts atomic.Int32
	o.Restart = RestartOptions{By: "systemd", Restart: func() error { restarts.Add(1); return nil }}
	st := o.Store
	r := startServer(t, withOptions(o))
	post := func(path string, body []byte, want int) map[string]any {
		t.Helper()
		code, text := lanCall(t, r.ts.URL, "POST", path, "", body)
		if code != want {
			t.Fatalf("POST %s: %d %s, want %d", path, code, text, want)
		}
		var v map[string]any
		json.Unmarshal([]byte(text), &v)
		return v
	}
	key := jsonBody(map[string]any{"key_file": keyFile})

	post("/api/v1/setup/backup/plan", key, 404)
	up := post("/api/v1/setup/backup", backup, 200)["backup"].(map[string]any)
	if up["uuid"] == "" || up["newer"] != false || up["counts"].(map[string]any)["users"] != 1.0 || len(up["recipients"].([]any)) != 1 {
		t.Fatalf("the backup handed %v", up)
	}
	post("/api/v1/setup/backup", []byte("no backup"), 400)
	post("/api/v1/setup/backup/plan", nil, 400)
	post("/api/v1/setup/backup/plan", jsonBody(map[string]any{"key_file": "AGE-SECRET-KEY-1NOPE"}), 400)
	post("/api/v1/setup/backup/plan", jsonBody(map[string]any{"key_file": keyFile, "scopes": []string{"users"}}), 400)
	other, _ := serverbackup.GenerateKey(serverbackup.KindX25519)
	post("/api/v1/setup/backup/plan", jsonBody(map[string]any{"key_file": string(other.File(time.Now()))}), 422)
	plan := post("/api/v1/setup/backup/plan", key, 200)
	restart := plan["restart"].(map[string]any)
	if plan["identity"].(map[string]any)["this_box"] != false || plan["refused"] != "" || len(plan["scopes"].([]any)) != 5 || restart["available"] != true {
		t.Fatalf("plan %v", plan)
	}
	if _, ok, _ := st.User(ctx, "olga"); ok {
		t.Fatal("the plan changed the store")
	}

	v := post("/api/v1/setup/backup/restore", key, 200)
	restart = v["restart"].(map[string]any)
	if restart["started"] != true || restart["by"] != "systemd" || restarts.Load() != 1 || v["before_backup"] != nil || len(v["scopes"].([]any)) != 5 {
		t.Fatalf("restore %v", v)
	}
	if bs, _ := st.ServerBackups(ctx); len(bs) != 0 {
		t.Errorf("a backup was taken first: %+v", bs)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".serverbackup-*")); len(left) > 0 {
		t.Errorf("left in the data directory: %v", left)
	}
	secret := strings.TrimSpace(keyFile[strings.LastIndex(strings.TrimSpace(keyFile), "\n")+1:])
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if b, err := os.ReadFile(p); err == nil && bytes.Contains(b, []byte(secret)) {
				t.Errorf("%s holds the key", p)
			}
		}
		return nil
	})
	// The store has users now: the routes ask for a session.
	if code, text := lanCall(t, r.ts.URL, "POST", "/api/v1/setup/backup", "", backup); code != 401 {
		t.Errorf("an upload once the store has users: %d %s", code, text)
	}

	// The restart: the store opened again, with the backup's opaque.key.
	r.stop()
	st.Close()
	r2 := startServer(t, withOptions(storeOptions(t, dir, "")))
	_, code, body := opaqueSignIn(t, r2.ts, "olga", oldBoxPassword, "")
	if code != 200 {
		t.Fatalf("olga's sign-in with the old password: %d %s", code, body)
	}
	var session struct {
		Token string `json:"token"`
	}
	json.Unmarshal([]byte(body), &session)
	if code, text := callAPI(t, r2.ts, "POST", "/api/v1/setup/backup/plan", session.Token, string(key)); code != 409 || !strings.Contains(text, "has users") {
		t.Errorf("the owner's plan once the server has users: %d %s", code, text)
	}
}

// The routes answer while the server is open alone: on a server with
// users a request without a session is 401, one of a role that is not
// the owner 403, and the owner's 409, the window closed. A backup handed
// waits for at most newBoxHeld; then the plan says to hand it again.
func TestNewBoxOnlyWhileOpen(t *testing.T) {
	t.Parallel()
	file, keyFile := oldBoxBackup(t)
	backup, _ := os.ReadFile(file)
	key := string(jsonBody(map[string]any{"key_file": keyFile}))

	users := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	viewer := userToken(t, users.st, "vera", store.RoleViewer)
	for _, path := range []string{"/api/v1/setup/backup", "/api/v1/setup/backup/list", "/api/v1/setup/backup/fetch", "/api/v1/setup/backup/plan", "/api/v1/setup/backup/restore"} {
		if code, text := callAPI(t, users.ts, "POST", path, "", key); code != 401 {
			t.Errorf("%s without a session: %d %s", path, code, text)
		}
		if code, text := callAPI(t, users.ts, "POST", path, viewer, key); code != 403 {
			t.Errorf("%s of a viewer: %d %s", path, code, text)
		}
		if code, text := callAPI(t, users.ts, "POST", path, "secret", key); code != 409 || !strings.Contains(text, "has users") {
			t.Errorf("%s of the owner: %d %s", path, code, text)
		}
	}

	clock := testkit.NewClock(time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	o := storeOptions(t, t.TempDir(), "")
	o.serverBackupClock = kitClock{clock}
	open := startServer(t, withOptions(o))
	if code, text := callAPI(t, open.ts, "POST", "/api/v1/setup/backup", "", string(backup)); code != 200 {
		t.Fatalf("hand the backup: %d %s", code, text)
	}
	clock.Add(newBoxHeld + time.Minute)
	if code, text := callAPI(t, open.ts, "POST", "/api/v1/setup/backup/plan", "", key); code != 404 || !strings.Contains(text, "hand it again") {
		t.Errorf("a plan past the backup's time: %d %s", code, text)
	}
	if left, _ := filepath.Glob(filepath.Join(o.Store.Dir(), ".serverbackup-*")); len(left) > 0 {
		t.Errorf("left in the data directory: %v", left)
	}
}

// From a target described in the request and stored nowhere: its
// backups listed by their manifests, a file that is none counted apart;
// the one picked fetched and held, then planned; a name the target does
// not list refused, and a kind that makes its own key (SFTP) too.
func TestNewBoxFromTarget(t *testing.T) {
	t.Parallel()
	file, keyFile := oldBoxBackup(t)
	src := t.TempDir()
	b, _ := os.ReadFile(file)
	if err := os.WriteFile(filepath.Join(src, "massimo-old.zip"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "notes.zip"), []byte("no backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	o := storeOptions(t, t.TempDir(), "")
	o.Restart = RestartOptions{By: "systemd", Restart: func() error { return nil }}
	r := startServer(t, withOptions(o))
	target := map[string]any{"kind": "local", "config": map[string]string{"path": src}}
	code, text := callAPI(t, r.ts, "POST", "/api/v1/setup/backup/list", "", string(jsonBody(target)))
	var list struct {
		Backups []struct {
			Name string `json:"name"`
			UUID string `json:"uuid"`
		} `json:"backups"`
		Skipped int `json:"skipped"`
	}
	json.Unmarshal([]byte(text), &list)
	if code != 200 || len(list.Backups) != 1 || list.Backups[0].Name != "massimo-old.zip" || list.Skipped != 1 {
		t.Fatalf("list: %d %s", code, text)
	}
	ts, _ := o.Store.ServerBackupTargets(t.Context())
	for _, tg := range ts {
		if tg.Config["path"] == src {
			t.Errorf("the target was stored: %+v", tg)
		}
	}
	fetch := func(name string) (int, string) {
		return callAPI(t, r.ts, "POST", "/api/v1/setup/backup/fetch", "", string(jsonBody(map[string]any{"kind": "local", "config": map[string]string{"path": src}, "name": name})))
	}
	if code, text := fetch("../old-box.zip"); code != 404 {
		t.Errorf("a name the target does not list: %d %s", code, text)
	}
	if code, text := fetch("massimo-old.zip"); code != 200 || !strings.Contains(text, list.Backups[0].UUID) {
		t.Fatalf("fetch: %d %s", code, text)
	}
	if code, text := callAPI(t, r.ts, "POST", "/api/v1/setup/backup/plan", "", string(jsonBody(map[string]any{"key_file": keyFile}))); code != 200 || !strings.Contains(text, `"refused": ""`) {
		t.Errorf("plan of the backup fetched: %d %s", code, text)
	}
	// The kinds say which a new box cannot read from, for the card.
	var kinds []kindView
	if code := getJSON(t, r.ts, "", "/api/v1/server-backup-targets/kinds", &kinds); code != 200 {
		t.Fatalf("kinds: %d", code)
	}
	for _, k := range kinds {
		if k.MakesSecret != (k.Name == serverbackup.KindSFTP) {
			t.Errorf("kind %s: makes_secret %v", k.Name, k.MakesSecret)
		}
	}
	sftp := map[string]any{"kind": "sftp", "config": map[string]string{"host": "nas.example.com", "user": "massimo"}}
	if code, text := callAPI(t, r.ts, "POST", "/api/v1/setup/backup/list", "", string(jsonBody(sftp))); code != 400 || !strings.Contains(text, "start from the file") {
		t.Errorf("an SFTP target: %d %s", code, text)
	}
}
