// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // Europe/Zurich for the change of summer time, on every system

	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// The schedule's runs in the box's time: the daily time, across both
// changes of summer time.
func TestServerBackupSlots(t *testing.T) {
	t.Parallel()
	zurich, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		t.Fatal(err)
	}
	at := func(s string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", s, zurich)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, c := range []struct {
		now, want string
		gap       time.Duration
	}{
		{"2026-11-27 01:00", "2026-11-27 03:30", 0},
		{"2026-11-27 03:30", "2026-11-28 03:30", 24 * time.Hour},
		{"2026-03-28 03:30", "2026-03-29 03:30", 23 * time.Hour}, // summer time: a night of 23 hours
		{"2026-10-24 03:30", "2026-10-25 03:30", 25 * time.Hour}, // and back: of 25
	} {
		got := slotAfter(at(c.now), 3, 30, zurich)
		if !got.Equal(at(c.want)) {
			t.Errorf("after %s: %s, want %s", c.now, got, c.want)
		}
		if c.gap != 0 && got.Sub(at(c.now)) != c.gap {
			t.Errorf("after %s: %s later, want %s", c.now, got.Sub(at(c.now)), c.gap)
		}
	}
	// 02:30 does not exist on the night summer time begins: it is 03:30.
	if got := slotAfter(at("2026-03-29 00:00"), 2, 30, zurich); got.Hour() != 3 || got.Minute() != 30 || got.Day() != 29 {
		t.Errorf("02:30 on the change: %s", got)
	}
	if got := slotBefore(at("2026-11-27 03:00"), 3, 30, zurich); !got.Equal(at("2026-11-26 03:30")) {
		t.Errorf("before 03:00: %s", got)
	}
	for _, bad := range []string{"3:30", "24:00", "03:60", "0330", "x"} {
		if _, _, err := parseClock(bad); err == nil {
			t.Errorf("%q taken", bad)
		}
	}
}

// sbRig is a server with a store, a key of the server's backups and the
// schedule's clock the test moves, in UTC.
type sbRig struct {
	*rig
	clock *testkit.Clock
	st    *store.Store
}

// startSB starts a server for the server's backups at at, with a
// classic key; the key's identity is returned.
func startSB(t *testing.T, at time.Time) (*sbRig, []byte) {
	t.Helper()
	clock := testkit.NewClock(at)
	o := storeOptions(t, t.TempDir(), "secret")
	o.serverBackupClock, o.serverBackupZone = kitClock{clock}, time.UTC
	st := o.Store
	k, err := serverbackup.GenerateKey(serverbackup.KindX25519)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddServerBackupKey(t.Context(), store.ServerBackupKey{Label: "laptop", Recipient: k.Recipient.Line, Fingerprint: k.Recipient.Fingerprint, Kind: string(k.Recipient.Kind)}, "", 0); err != nil {
		t.Fatal(err)
	}
	return &sbRig{rig: startServer(t, withOptions(o)), clock: clock, st: st}, k.File(at)
}

// backups lists the catalogue.
func (r *sbRig) backups(t *testing.T) []store.ServerBackup {
	t.Helper()
	bs, err := r.st.ServerBackups(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return bs
}

// lines are the server's server_backup lines, newest first.
func (r *sbRig) lines(t *testing.T) []store.Event {
	t.Helper()
	evs, err := r.st.Events(t.Context(), store.EventQuery{Server: true, Kind: "server_backup", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return evs
}

// A run the schedule missed is made five minutes after the start; then
// the daily time, not before; a copy that fails is retried after 5 min,
// 30 min and 2 h, then a warning that is an alert until a retry by hand
// succeeds.
func TestServerBackupSchedule(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	r, _ := startSB(t, start)
	// The clock moves only while the loop waits on its timer: after each
	// step that wakes it, the test waits for its next timer.
	armed := func() int64 { return r.s.sb.armed.Load() }
	rearmed := func(what string, n int64) {
		t.Helper()
		testkit.Eventually(t, what, func() bool { return armed() > n }, jobWait)
	}
	rearmed("the loop's first timer", 0)
	r.clock.Add(4 * time.Minute)
	time.Sleep(50 * time.Millisecond)
	if bs := r.backups(t); len(bs) != 0 {
		t.Fatalf("made before the five minutes: %+v", bs)
	}
	n := armed()
	r.clock.Add(time.Minute)
	rearmed("the missed run", n)
	first := r.backups(t)
	if len(first) != 1 || first[0].Trigger != "scheduled" || len(first[0].Copies) != 1 || first[0].Copies[0].State != store.CopyThere {
		t.Fatalf("the missed run %+v", first)
	}

	// A target whose directory is a file: its copy fails.
	blocked := filepath.Join(r.st.Dir(), "blocked")
	os.WriteFile(blocked, []byte("x"), 0o600)
	n = armed()
	if code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backup-targets", "secret", `{"name":"nas","kind":"local","config":{"path":"blocked"},"retention":{"last":5}}`); code != 201 {
		t.Fatalf("add a target: %d %s", code, text)
	}
	rearmed("the wake of a target added", n)
	// 03:30 the next day, not before.
	r.clock.Add(time.Date(2026, 11, 28, 3, 29, 0, 0, time.UTC).Sub(r.clock.Now()))
	time.Sleep(50 * time.Millisecond)
	if bs := r.backups(t); len(bs) != 1 {
		t.Fatalf("made before 03:30: %d", len(bs))
	}
	n = armed()
	r.clock.Add(time.Minute)
	rearmed("the 03:30 run", n)
	if bs := r.backups(t); len(bs) != 2 || !bs[0].Time.Equal(time.Date(2026, 11, 28, 3, 30, 0, 0, time.UTC)) {
		t.Fatalf("the 03:30 run: %+v", bs)
	}
	failedCopy := func() store.ServerBackupCopy {
		for _, c := range r.backups(t)[0].Copies {
			if c.State != store.CopyThere {
				return c
			}
		}
		return store.ServerBackupCopy{}
	}
	if c := failedCopy(); c.Tries != 1 {
		t.Fatalf("the failed copy %+v", c)
	}
	for i, wait := range serverbackup.RetryDelays {
		n := armed()
		r.clock.Add(wait)
		rearmed("a retry", n)
		if c := failedCopy(); c.Tries != i+2 {
			t.Fatalf("retry %d: %+v", i+1, c)
		}
	}
	if c := failedCopy(); !c.NextTry.IsZero() {
		t.Fatalf("after the third retry, another: %+v", c)
	}
	testkit.Eventually(t, "the warning", func() bool {
		evs := r.lines(t)
		return len(evs) > 0 && evs[0].Level == 4
	})
	var alerts struct {
		Alerts []struct {
			Kind string `json:"kind"`
		} `json:"alerts"`
	}
	hasAlert := func() bool {
		getJSON(t, r.ts, "secret", "/api/v1/log/alerts", &alerts)
		for _, a := range alerts.Alerts {
			if a.Kind == "server_backup" {
				return true
			}
		}
		return false
	}
	testkit.Eventually(t, "the alert", hasAlert)

	// A retry by hand of a holder of backup.retry, the manager, once the
	// target is fixed: the copy is there, and the alert settles.
	os.Remove(blocked)
	manager := userToken(t, r.st, "anna", store.RoleManager)
	if code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backups/"+r.backups(t)[0].UUID+"/retry", manager, ""); code != 200 {
		t.Fatalf("retry: %d %s", code, text)
	}
	if c := failedCopy(); c.TargetUUID != "" {
		t.Fatalf("still failed: %+v", c)
	}
	testkit.Eventually(t, "the alert settled", func() bool { return !hasAlert() })
	if code, _ := callAPI(t, r.ts, "POST", "/api/v1/server-backups/"+r.backups(t)[0].UUID+"/retry", manager, ""); code != 409 {
		t.Fatalf("a retry of nothing failed: %d", code)
	}
}

// Only a holder of backup.retry, or of backup.server which includes it,
// is told of a failure.
func TestServerBackupNotifyNeeds(t *testing.T) {
	t.Parallel()
	r, _ := startSB(t, time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	ctx := t.Context()
	r.st.AddRole(ctx, store.Role{Key: "backer", Capabilities: []string{"read", "backup.server"}}, 0)
	for name, c := range map[string]struct {
		role string
		want bool
	}{"o": {store.RoleOwner, true}, "m": {store.RoleManager, true}, "b": {"backer", true}, "p": {store.RoleOperator, false}} {
		u, err := r.st.AddUser(ctx, store.User{Name: name, Role: c.role}, "", 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := r.s.userHolds(ctx, u.ID, notifyNeeds[notify.KindServerBackupFailed]); got != c.want {
			t.Errorf("%s: %v", c.role, got)
		}
	}
}

// Each route of the server's backups: who passes and who is refused.
func TestServerBackupRoutesRights(t *testing.T) {
	t.Parallel()
	r, _ := startSB(t, time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	ctx := t.Context()
	r.st.AddRole(ctx, store.Role{Key: "backer", Capabilities: []string{"read", "backup.server"}}, 0)
	r.st.AddRole(ctx, store.Role{Key: "retrier", Capabilities: []string{"read", "backup.retry"}}, 0)
	tokens := map[string]string{
		"owner": "secret", "backer": userToken(t, r.st, "b", "backer"), "retrier": userToken(t, r.st, "r", "retrier"),
		"manager": userToken(t, r.st, "m", store.RoleManager), "integration": userToken(t, r.st, "i", store.RoleIntegration),
	}
	passes := map[string]map[string]bool{
		"backup.retry":  {"owner": true, "backer": true, "retrier": true, "manager": true},
		"backup.server": {"owner": true, "backer": true},
		capOwner:        {"owner": true},
	}
	n := 0
	for _, rt := range registeredRoutes(t) {
		if !strings.Contains(rt.path, "/server-backup") {
			continue
		}
		n++
		for who, tok := range tokens {
			code, text := callAPI(t, r.ts, rt.method, strings.NewReplacer("{id}", "nope", "{uuid}", "nope", "{target}", "nope").Replace(rt.path), tok, "")
			refused := code == 403
			if want := passes[rt.capability][who]; want == refused {
				t.Errorf("%s %s (%s) for %s: %d %s", rt.method, rt.path, rt.capability, who, code, text)
			}
		}
	}
	if n != 21 {
		t.Fatalf("%d routes", n)
	}
}

// The keys: one made and confirmed by its last six characters, a mix of
// kinds refused with why, a secret key refused, the identity in no audit
// row; then backups made, copied, pinned, checked, downloaded, uploaded
// and found by a target's scan.
func TestServerBackupAPI(t *testing.T) {
	t.Parallel()
	r, _ := startSB(t, time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	ctx := t.Context()
	keys, _ := r.st.ServerBackupKeys(ctx)
	r.st.DeleteServerBackupKey(ctx, keys[0].UUID)
	post := func(path, body string, want int) map[string]any {
		t.Helper()
		code, text := callAPI(t, r.ts, "POST", path, "secret", body)
		if code != want {
			t.Fatalf("POST %s %s: %d %s, want %d", path, body, code, text, want)
		}
		var v map[string]any
		json.Unmarshal([]byte(text), &v)
		return v
	}
	post("/api/v1/server-backups", "", 409) // no key
	made := post("/api/v1/server-backup-keys", `{"label":"safe","make":"classic"}`, 201)
	identity, _ := made["identity"].(string)
	key, _ := made["key"].(map[string]any)
	if !strings.HasPrefix(identity, "AGE-SECRET-KEY-1") || !strings.Contains(made["key_file"].(string), identity) || key["pending"] != true {
		t.Fatalf("made %v", made)
	}
	uuid := key["uuid"].(string)
	post("/api/v1/server-backups", "", 409) // pending: no key yet
	post("/api/v1/server-backup-keys/"+uuid+"/confirm", `{"last_six":"AAAAAA"}`, 422)
	post("/api/v1/server-backup-keys/"+uuid+"/confirm", `{"last_six":"`+strings.ToLower(identity[len(identity)-6:])+`"}`, 200)
	pq, _ := serverbackup.GenerateKey(serverbackup.KindPostQuantum)
	if v := post("/api/v1/server-backup-keys", `{"label":"pq","recipient":"`+pq.Recipient.Line+`"}`, 409); v["code"] != "mixed_kinds" {
		t.Fatalf("a mix: %v", v)
	}
	post("/api/v1/server-backup-keys", `{"recipient":"`+identity+`"}`, 400)
	rows, _ := r.st.AuditList(ctx, time.Time{}, 100, "")
	for _, a := range rows {
		if strings.Contains(string(a.Detail), identity) || strings.Contains(string(a.Detail), identity[len(identity)-6:]) {
			t.Fatalf("an audit row holds the key: %s %s", a.Action, a.Detail)
		}
	}

	// A backup now, to the local target made for it, labelled.
	b1 := post("/api/v1/server-backups", `{"label":"before the move"}`, 201)["backup"].(map[string]any)
	if b1["label"] != "before the move" || b1["trigger"] != "manual" || b1["by"] != store.BootstrapUser || b1["this_box"] != true {
		t.Fatalf("made %v", b1)
	}
	id1 := b1["uuid"].(string)
	var kinds []kindView
	if getJSON(t, r.ts, "secret", "/api/v1/server-backup-targets/kinds", &kinds); len(kinds) == 0 || kinds[0].Name != "local" || kinds[0].Fields[0].Type != "path" {
		t.Fatalf("kinds %+v", kinds)
	}

	// A second target; the next backup goes there alone.
	second := post("/api/v1/server-backup-targets", `{"name":"Second","kind":"local","config":{"path":"second"},"takes":"daily","retention":{"last":2}}`, 201)
	sid := second["target"].(map[string]any)["uuid"].(string)
	post("/api/v1/server-backup-targets", `{"name":"second","kind":"local"}`, 409)
	post("/api/v1/server-backup-targets", `{"name":"x","kind":"tape"}`, 400)
	post("/api/v1/server-backup-targets", `{"name":"x","kind":"local","secret":"no"}`, 400)
	if v := post("/api/v1/server-backup-targets/"+sid+"/test", "", 200); v["ok"] != true {
		t.Fatalf("test %v", v)
	}
	b2 := post("/api/v1/server-backups", `{"target":"second"}`, 201)["backup"].(map[string]any)
	if cs := b2["copies"].([]any); len(cs) != 1 || cs[0].(map[string]any)["target"] != sid {
		t.Fatalf("to second alone: %v", cs)
	}
	if code, text := callAPI(t, r.ts, "PUT", "/api/v1/server-backup-targets/"+sid, "secret", `{"takes":"weekly"}`); code != 200 || !strings.Contains(text, `"takes": "weekly"`) {
		t.Fatalf("put %d %s", code, text)
	}

	// Pinned and unpinned; checked; pushed to second and deleted there.
	if code, text := callAPI(t, r.ts, "PUT", "/api/v1/server-backups/"+id1+"/pin", "secret", `{"pinned":true}`); code != 200 || !strings.Contains(text, `"pinned_until": "9999-`) {
		t.Fatalf("pin %d %s", code, text)
	}
	callAPI(t, r.ts, "PUT", "/api/v1/server-backups/"+id1+"/pin", "secret", `{"pinned":false}`)
	if v := post("/api/v1/server-backups/"+id1+"/check", "", 200); v["steps"].([]any)[0].(map[string]any)["ok"] != true {
		t.Fatalf("check %v", v)
	}
	if code, text := callAPI(t, r.ts, "PUT", "/api/v1/server-backups/"+id1+"/copies/second", "secret", ""); code != 200 || strings.Count(text, `"state": "there"`) != 2 {
		t.Fatalf("push %d %s", code, text)
	}
	if code, text := callAPI(t, r.ts, "DELETE", "/api/v1/server-backups/"+id1+"/copies/"+sid, "secret", ""); code != 200 {
		t.Fatalf("delete a copy %d %s", code, text)
	}
	// Pulled from local, it goes to the other local target, Second, and
	// stays there; deleted again.
	if v := post("/api/v1/server-backups/"+id1+"/copies/local/pull", "", 200); !strings.Contains(fmt.Sprint(v), "pulled:true") {
		t.Fatalf("pull %v", v)
	}
	callAPI(t, r.ts, "DELETE", "/api/v1/server-backups/"+id1+"/copies/"+sid, "secret", "")

	// The file, whole and by a range.
	b, _, _ := r.st.ServerBackup(ctx, id1)
	req, _ := http.NewRequest("GET", r.ts.URL+"/api/v1/server-backups/"+id1+"/file", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	file, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || int64(len(file)) != b.Size || !strings.Contains(resp.Header.Get("Content-Disposition"), "massimo-") {
		t.Fatalf("file %d %d %v", resp.StatusCode, len(file), resp.Header)
	}
	req.Header.Set("Range", "bytes=0-3")
	resp, _ = http.DefaultClient.Do(req)
	part, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 206 || !bytes.Equal(part, file[:4]) {
		t.Fatalf("range %d %q", resp.StatusCode, part)
	}

	// Uploaded: junk refused, a backup of its own taken again; deleted
	// from local and uploaded, it is back, kept there.
	if code, _ := callAPI(t, r.ts, "POST", "/api/v1/server-backups/upload", "secret", "no zip"); code != 400 {
		t.Fatalf("junk uploaded: %d", code)
	}
	callAPI(t, r.ts, "DELETE", "/api/v1/server-backups/"+id1+"/copies/local", "secret", "")
	if _, ok, _ := r.st.ServerBackup(ctx, id1); ok {
		t.Fatal("b1 stayed without a copy")
	}
	upReq, _ := http.NewRequest("POST", r.ts.URL+"/api/v1/server-backups/upload", bytes.NewReader(file))
	upReq.Header.Set("Authorization", "Bearer secret")
	upReq.Header.Set("Content-Type", "application/zip")
	resp, err = http.DefaultClient.Do(upReq)
	if err != nil {
		t.Fatal(err)
	}
	upText, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 201 || !strings.Contains(string(upText), `"trigger": "uploaded"`) || !strings.Contains(string(upText), `"pulled": true`) {
		t.Fatalf("upload %d %s", resp.StatusCode, upText)
	}

	// A target added over a directory that holds a backup finds it.
	other := filepath.Join(r.st.Dir(), "other")
	os.MkdirAll(other, 0o700)
	os.WriteFile(filepath.Join(other, "massimo-copy.zip"), file, 0o600)
	callAPI(t, r.ts, "DELETE", "/api/v1/server-backups/"+id1+"/copies/local", "secret", "")
	if v := post("/api/v1/server-backup-targets", `{"name":"other","kind":"local","config":{"path":"other"}}`, 201); v["found"] != float64(1) {
		t.Fatalf("scan %v", v)
	}
	if b, ok, _ := r.st.ServerBackup(ctx, id1); !ok || len(b.Copies) != 1 || b.Copies[0].Path != "massimo-copy.zip" {
		t.Fatalf("found %+v %v", b, ok)
	}

	// The list: each target's state, the time, the next run.
	var list struct {
		Backups []serverBackupView `json:"backups"`
		Targets []targetState      `json:"targets"`
		Keys    int                `json:"keys"`
		Time    string             `json:"time"`
	}
	if getJSON(t, r.ts, "secret", "/api/v1/server-backups", &list); list.Keys != 1 || list.Time != "03:30" || len(list.Targets) != 3 || len(list.Backups) != 2 {
		t.Fatalf("list %+v", list)
	}
	if code, text := callAPI(t, r.ts, "PUT", "/api/v1/settings", "secret", `{"serverbackup.time":"04:15"}`); code != 200 || !strings.Contains(text, `"04:15"`) {
		t.Fatalf("the time %d %s", code, text)
	}
	if code, _ := callAPI(t, r.ts, "PUT", "/api/v1/settings", "secret", `{"serverbackup.time":"25:00"}`); code != 400 {
		t.Fatalf("25:00: %d", code)
	}
	if code, _ := callAPI(t, r.ts, "DELETE", "/api/v1/server-backup-keys/"+uuid, "secret", ""); code != 200 {
		t.Fatalf("delete the key: %d", code)
	}
	if code, _ := callAPI(t, r.ts, "DELETE", "/api/v1/server-backup-targets/"+sid, "secret", ""); code != 200 {
		t.Fatalf("delete a target: %d", code)
	}

}
