// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// The restore's routes, the owner's: the plan, which changes nothing; a
// key that does not open the backup, none, or a scope that is none
// refused; the check with the key; then a restore of the users, a pinned
// backup taken first, a user added since gone, the server restarted, and
// the key in no audit row.
func TestServerRestoreAPI(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	clock := testkit.NewClock(at)
	o := storeOptions(t, t.TempDir(), "secret")
	o.serverBackupClock, o.serverBackupZone = kitClock{clock}, time.UTC
	var restarts atomic.Int32
	o.Restart = RestartOptions{By: "systemd", Restart: func() error { restarts.Add(1); return nil }}
	st := o.Store
	k, err := serverbackup.GenerateKey(serverbackup.KindX25519)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddServerBackupKey(t.Context(), store.ServerBackupKey{Label: "laptop", Recipient: k.Recipient.Line, Fingerprint: k.Recipient.Fingerprint, Kind: string(k.Recipient.Kind)}, "", 0); err != nil {
		t.Fatal(err)
	}
	r := startServer(t, withOptions(o))
	ctx := t.Context()
	keyFile := string(k.File(at))

	code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backups", "secret", "")
	if code != 201 {
		t.Fatalf("a backup: %d %s", code, text)
	}
	var made struct {
		Backup struct {
			UUID string `json:"uuid"`
		} `json:"backup"`
	}
	json.Unmarshal([]byte(text), &made)
	id := made.Backup.UUID
	if _, err := st.AddUser(ctx, store.User{Name: "late", Role: store.RoleViewer}, "x", 0); err != nil {
		t.Fatal(err)
	}
	body := func(v map[string]any) string {
		b, _ := json.Marshal(v)
		return string(b)
	}
	call := func(path, b string, want int) map[string]any {
		t.Helper()
		code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backups/"+id+path, "secret", b)
		if code != want {
			t.Fatalf("POST …%s: %d %s, want %d", path, code, text, want)
		}
		var v map[string]any
		json.Unmarshal([]byte(text), &v)
		return v
	}

	call("/restore/plan", "", 400)
	call("/restore/plan", body(map[string]any{"key_file": "AGE-SECRET-KEY-1NOPE"}), 400)
	call("/restore/plan", body(map[string]any{"key_file": keyFile, "scopes": []string{"everything"}}), 400)
	other, _ := serverbackup.GenerateKey(serverbackup.KindX25519)
	call("/restore/plan", body(map[string]any{"key_file": string(other.File(at))}), 422)
	plan := call("/restore/plan", body(map[string]any{"key_file": keyFile}), 200)
	if id := plan["identity"].(map[string]any); id["this_box"] != true || plan["refused"] != "" || len(plan["scopes"].([]any)) != 5 {
		t.Fatalf("plan %v", plan)
	}
	if _, ok, _ := st.User(ctx, "late"); !ok {
		t.Fatal("the plan changed the store")
	}
	if v := call("/check-key", body(map[string]any{"key_file": keyFile}), 200); v["ok"] != true || v["this_box"] != true {
		t.Fatalf("check-key %v", v)
	}

	v := call("/restore", body(map[string]any{"key_file": keyFile, "scopes": []string{"users"}}), 200)
	restart := v["restart"].(map[string]any)
	if restart["started"] != true || restart["by"] != "systemd" || v["before_backup"] == "" || restarts.Load() != 1 {
		t.Fatalf("restore %v", v)
	}
	if _, ok, _ := st.User(ctx, "late"); ok {
		t.Error("a user added since the backup stayed")
	}
	if b, ok, _ := st.ServerBackup(ctx, v["before_backup"].(string)); !ok || b.Trigger != string(serverbackup.TriggerBeforeRestore) || b.PinnedUntil.IsZero() {
		t.Errorf("the backup first %+v", b)
	}
	secret := strings.TrimSpace(keyFile[strings.LastIndex(strings.TrimSpace(keyFile), "\n")+1:])
	rows, _ := st.AuditList(ctx, time.Time{}, 100, "")
	n := 0
	for _, a := range rows {
		if strings.Contains(string(a.Detail), secret) {
			t.Fatalf("an audit row holds the key: %s %s", a.Action, a.Detail)
		}
		if strings.HasPrefix(a.Action, "serverbackup.restore") || a.Action == "serverbackup.check_key" {
			n++
		}
	}
	if n < 3 {
		t.Errorf("%d audit rows of the restore's routes", n)
	}
	// A second restore while the first one's restart is under way.
	call("/restore", body(map[string]any{"key_file": keyFile, "scopes": []string{"users"}}), 409)
}

// A server that cannot restart itself refuses a restore of users and
// sign-in, which takes the backup's opaque.key, and the plan says so; a
// restore of the settings goes, and its answer says the restart is the
// owner's.
func TestServerRestoreNeedsARestart(t *testing.T) {
	t.Parallel()
	r, keyFile := startSB(t, time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backups", "secret", "")
	if code != 201 {
		t.Fatalf("a backup: %d %s", code, text)
	}
	var made struct {
		Backup struct {
			UUID string `json:"uuid"`
		} `json:"backup"`
	}
	json.Unmarshal([]byte(text), &made)
	body := func(scopes ...string) string {
		b, _ := json.Marshal(map[string]any{"key_file": string(keyFile), "scopes": scopes})
		return string(b)
	}
	base := "/api/v1/server-backups/" + made.Backup.UUID
	code, text = callAPI(t, r.ts, "POST", base+"/restore/plan", "secret", body("users"))
	if code != 200 || !strings.Contains(text, "cannot restart itself") || !strings.Contains(text, `"available": false`) {
		t.Fatalf("plan %d %s", code, text)
	}
	if code, text = callAPI(t, r.ts, "POST", base+"/restore", "secret", body("users")); code != 409 || !strings.Contains(text, "cannot restart itself") {
		t.Fatalf("users without a restart: %d %s", code, text)
	}
	if bs := r.backups(t); len(bs) != 1 {
		t.Fatalf("the refused restore took a backup: %d", len(bs))
	}
	code, text = callAPI(t, r.ts, "POST", base+"/restore", "secret", body("settings"))
	if code != 200 || !strings.Contains(text, `"started": false`) || !strings.Contains(text, "no service manager") || !strings.Contains(text, `"restored_at"`) {
		t.Fatalf("settings without a restart: %d %s", code, text)
	}
}
