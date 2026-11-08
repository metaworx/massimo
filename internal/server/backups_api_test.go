// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/store"
)

const backupsBase = "/api/v1/cubes/KEQ0000100/backups"

// historyOf reads GET …/backups.
func historyOf(t *testing.T, url string) (int, backupList) {
	t.Helper()
	resp, err := http.Get(url + backupsBase)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out backupList
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// reasons lists the backups' reasons, newest first.
func reasons(l backupList) string {
	var out []string
	for _, b := range l.Backups {
		out = append(out, b.Reason)
	}
	return strings.Join(out, ",")
}

// within reports whether cond holds before d has passed.
func within(d time.Duration, cond func() bool) bool {
	for deadline := time.Now().Add(d); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return true
		}
	}
	return cond()
}

// The server takes a Cube's first backup at once, none while nothing
// changes, one after a change once it has been quiet for
// backups.auto_min, and one backups.auto_max after the first change
// however long the quiet window.
func TestBackupAutomatic(t *testing.T) {
	_, ts, _, st := startStored(t, Options{BackupMinute: 20 * time.Millisecond})
	ctx := context.Background()
	st.SetSetting(ctx, store.BackupsAutoMinKey, 3)
	st.SetSetting(ctx, store.BackupsAutoMaxKey, 10)
	var l backupList
	eventually(t, "the first backup", func() bool {
		_, l = historyOf(t, ts.URL)
		return reasons(l) == "auto"
	})
	first := l.Backups[0]
	time.Sleep(300 * time.Millisecond)
	if _, l = historyOf(t, ts.URL); len(l.Backups) != 1 || l.Changed != nil {
		t.Fatalf("a backup without a change: %s, changed %v", reasons(l), l.Changed)
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6", `{"name":"Renamed"}`); code != 200 {
		t.Fatalf("rename: %d", code)
	}
	eventually(t, "the backup after the change", func() bool {
		_, l = historyOf(t, ts.URL)
		return reasons(l) == "auto,auto"
	})
	if l.Backups[0].SHA256 == first.SHA256 || l.Changed != nil {
		t.Fatalf("the backup after the change: %+v, changed %v", l.Backups[0], l.Changed)
	}
	// A quiet window of a thousand minutes: backups.auto_max decides.
	st.SetSetting(ctx, store.BackupsAutoMinKey, 1000)
	began := time.Now()
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6", `{"name":"Again"}`); code != 200 {
		t.Fatalf("rename: %d", code)
	}
	eventually(t, "the change seen", func() bool {
		_, l = historyOf(t, ts.URL)
		return l.Changed != nil && l.Due != nil || len(l.Backups) == 3
	})
	eventually(t, "the backup at backups.auto_max", func() bool {
		_, l = historyOf(t, ts.URL)
		return len(l.Backups) == 3
	})
	if took := time.Since(began); took > 5*time.Second {
		t.Fatalf("the latest backup came after %v", took)
	}
}

// Deleting the newest backup makes the one before it the reference: the
// configuration differs from it, so changed and due appear.
func TestBackupDeleteNewest(t *testing.T) {
	_, ts, s, st := startStored(t, Options{BackupMinute: 20 * time.Millisecond})
	ctx := context.Background()
	st.SetSetting(ctx, store.BackupsAutoMinKey, 3)
	st.SetSetting(ctx, store.BackupsAutoMaxKey, 10)
	var l backupList
	eventually(t, "the first backup", func() bool {
		_, l = historyOf(t, ts.URL)
		return reasons(l) == "auto"
	})
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6", `{"name":"Renamed"}`); code != 200 {
		t.Fatalf("rename: %d", code)
	}
	eventually(t, "the backup after the change", func() bool {
		_, l = historyOf(t, ts.URL)
		return reasons(l) == "auto,auto" && l.Changed == nil
	})
	// No automatic backup from here on but the one a Cube without any gets.
	st.SetSetting(ctx, store.BackupsAutoMinKey, 1000)
	st.SetSetting(ctx, store.BackupsAutoMaxKey, 1000)
	if code, _ := do(t, ts, "DELETE", backupsBase+"/"+strconv.FormatInt(l.Backups[0].ID, 10), ""); code != 204 {
		t.Fatalf("delete: %d", code)
	}
	// The configuration differs from the backup now newest.
	older, ok, err := st.NewestBackup(ctx, s.cubes["keq0000100"].storeID.Load())
	if err != nil || !ok || older.ID != l.Backups[1].ID {
		t.Fatalf("the newest after the delete: %+v %v %v", older, ok, err)
	}
	body, _, _ := st.BackupBody(ctx, older.CubeID, older.ID)
	f, err := backup.Unmarshal(body, false)
	if err != nil || backup.ConfigHash(f) == backup.ConfigHash(backupOf(t, ts)) {
		t.Fatalf("the configuration is the one of the backup now newest (%v)", err)
	}
	if !within(time.Second, func() bool {
		_, l = historyOf(t, ts.URL)
		return l.Changed != nil && l.Due != nil
	}) {
		t.Fatalf("after deleting the newest backup: %s, changed %v, due %v", reasons(l), l.Changed, l.Due)
	}
	if reasons(l) != "auto" {
		t.Fatalf("a backup before backups.auto_min: %s", reasons(l))
	}
}

// Deleting all of a Cube's backups leaves it without one, so it gets one
// at once.
func TestBackupDeleteAll(t *testing.T) {
	_, ts, _, st := startStored(t, Options{BackupMinute: 20 * time.Millisecond})
	ctx := context.Background()
	var l backupList
	eventually(t, "the first backup", func() bool {
		_, l = historyOf(t, ts.URL)
		return reasons(l) == "auto"
	})
	first := l.Backups[0].ID
	st.SetSetting(ctx, store.BackupsAutoMinKey, 1000)
	st.SetSetting(ctx, store.BackupsAutoMaxKey, 1000)
	if code, _ := do(t, ts, "DELETE", backupsBase+"/"+strconv.FormatInt(first, 10), ""); code != 204 {
		t.Fatalf("delete: %d", code)
	}
	if !within(time.Second, func() bool {
		_, l = historyOf(t, ts.URL)
		return reasons(l) == "auto" && l.Backups[0].ID != first
	}) {
		t.Fatalf("after deleting every backup: %q, changed %v", reasons(l), l.Changed)
	}
}

// With backups.keep_newest 0 a prune can remove a Cube's only backup,
// and the Cube then gets one at once, as after a delete.
func TestBackupPruneNewest(t *testing.T) {
	_, ts, s, st := startStored(t, Options{BackupMinute: 20 * time.Millisecond})
	ctx := context.Background()
	var l backupList
	eventually(t, "the first backup", func() bool {
		_, l = historyOf(t, ts.URL)
		return reasons(l) == "auto"
	})
	first := l.Backups[0]
	st.SetSetting(ctx, store.BackupsAutoMinKey, 1000)
	st.SetSetting(ctx, store.BackupsAutoMaxKey, 1000)
	st.SetSetting(ctx, store.BackupsKeepNewestKey, 0)
	st.SetSetting(ctx, store.BackupsKeepDaysAutoKey, 1)
	s.pruneBackups(ctx, st, time.Now().Add(48*time.Hour))
	if _, ok, err := st.StoredBackup(ctx, s.cubes["keq0000100"].storeID.Load(), first.ID); ok || err != nil {
		t.Fatalf("the first backup is not pruned (%v)", err)
	}
	// The prune removes the row, so the next may get its id: the time tells them apart.
	if !within(time.Second, func() bool {
		_, l = historyOf(t, ts.URL)
		return reasons(l) == "auto" && l.Backups[0].TakenAt.After(first.TakenAt)
	}) {
		t.Fatalf("after pruning the only backup: %q, changed %v", reasons(l), l.Changed)
	}
}

// A backup taken by hand with a note, downloaded as stored and in the
// other format, deleted; a bad request, an unknown id and a Cube without
// a row in the store are refused.
func TestBackupsAPI(t *testing.T) {
	_, ts, _, _ := startStored(t, Options{})
	if code, l := historyOf(t, ts.URL); code != 200 || l.Backups == nil || len(l.Backups) != 0 {
		t.Fatalf("empty: %d %+v", code, l)
	}
	code, out := do(t, ts, "POST", backupsBase, `{"note":"before the holidays"}`)
	if code != 201 || out["reason"] != "manual" || out["note"] != "before the holidays" || out["format"] != "json" || out["size"].(float64) < 1000 {
		t.Fatalf("take: %d %v", code, out)
	}
	id := strconv.FormatInt(int64(out["id"].(float64)), 10)
	if code, _ := do(t, ts, "POST", backupsBase, ""); code != 201 {
		t.Fatalf("take without a body: %d", code)
	}
	for _, bad := range []string{`{"name":"x"}`, `{"note":"` + strings.Repeat("x", maxBackupNote+1) + `"}`, `[`} {
		if code, _ := do(t, ts, "POST", backupsBase, bad); code != 400 {
			t.Fatalf("take %s: %d", bad, code)
		}
	}
	get := func(path string) (int, string, []byte) {
		t.Helper()
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header.Get("Content-Type"), data
	}
	code, ctype, data := get(backupsBase + "/" + id)
	f, err := backup.Unmarshal(data, false)
	if code != 200 || ctype != "application/json" || err != nil || f.Cube.Serial != "KEQ0000100" {
		t.Fatalf("download: %d %s %v", code, ctype, err)
	}
	code, ctype, data = get(backupsBase + "/" + id + "?format=yaml")
	if y, err := backup.Unmarshal(data, true); code != 200 || ctype != "application/yaml" || err != nil || backup.ConfigHash(y) != backup.ConfigHash(f) {
		t.Fatalf("as YAML: %d %s %v", code, ctype, err)
	}
	for path, want := range map[string]int{"/" + id + "?format=xml": 400, "/999": 404, "/abc": 404} {
		if code, _, _ := get(backupsBase + path); code != want {
			t.Fatalf("%s: %d, want %d", path, code, want)
		}
	}
	if code, _ := do(t, ts, "DELETE", backupsBase+"/"+id, ""); code != 204 {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := do(t, ts, "DELETE", backupsBase+"/"+id, ""); code != 404 {
		t.Fatalf("delete again: %d", code)
	}
	if _, l := historyOf(t, ts.URL); reasons(l) != "manual" {
		t.Fatalf("after the delete: %s", reasons(l))
	}

	_, plain, _ := start(t, "")
	if code, _ := historyOf(t, plain.URL); code != http.StatusServiceUnavailable {
		t.Fatalf("a Cube without a row: %d", code)
	}
}

// A restore keeps the Cube's configuration before it and the file it was
// sent; a stored backup is restored by its id, without an upload; after
// a restore the Cube is as the file says, so no automatic backup follows.
func TestRestoreKeepsBackups(t *testing.T) {
	_, ts, _, st := startStored(t, Options{BackupMinute: 20 * time.Millisecond})
	ctx := context.Background()
	st.SetSetting(ctx, store.BackupsAutoMinKey, 2)
	st.SetSetting(ctx, store.BackupsAutoMaxKey, 4)
	var l backupList
	eventually(t, "the first backup", func() bool {
		_, l = historyOf(t, ts.URL)
		return reasons(l) == "auto"
	})
	f := withRoomName(backupOf(t, ts), "Restored")
	hash := plan(t, ts, "", f)["plan"].(string)
	if code, out := postBackup(t, ts, restoreBase+"?plan="+hash, "application/json", marshal(t, f)); code != 202 {
		t.Fatalf("restore: %d %v", code, out)
	}
	waitFor(t, "the restore", func() bool {
		_, run := do(t, ts, "GET", restoreBase, "")
		return run["state"] == "done"
	})
	_, l = historyOf(t, ts.URL)
	if reasons(l) != "upload,before_restore,auto" {
		t.Fatalf("after a restore from a file: %s", reasons(l))
	}
	before := l.Backups[1].ID
	time.Sleep(300 * time.Millisecond)
	if _, l = historyOf(t, ts.URL); len(l.Backups) != 3 {
		t.Fatalf("a backup after a full restore: %s", reasons(l))
	}

	// The stored backup from before the restore, by its id.
	q := "?backup=" + strconv.FormatInt(before, 10)
	code, p := postBackup(t, ts, restoreBase+"/plan"+q, "", nil)
	if code != 200 || p["steps"] == nil {
		t.Fatalf("plan by id: %d %v", code, p)
	}
	if code, out := postBackup(t, ts, restoreBase+q+"&plan="+p["plan"].(string), "", nil); code != 202 {
		t.Fatalf("restore by id: %d %v", code, out)
	}
	waitFor(t, "the restore by id", func() bool {
		_, run := do(t, ts, "GET", restoreBase, "")
		return run["state"] == "done" && run["plan"] == p["plan"]
	})
	time.Sleep(300 * time.Millisecond)
	if _, l = historyOf(t, ts.URL); reasons(l) != "before_restore,upload,before_restore,auto" {
		t.Fatalf("after a restore by id: %s", reasons(l))
	}
	for _, bad := range []string{"?backup=999", "?backup=x"} {
		if code, _ := postBackup(t, ts, restoreBase+"/plan"+bad, "", nil); code != 404 && code != 400 {
			t.Fatalf("plan%s: %d", bad, code)
		}
	}
}

// The five settings in the API, auto_min never above auto_max.
func TestBackupSettingsAPI(t *testing.T) {
	_, ts, _, _ := startStored(t, Options{})
	var all map[string]settingView
	getJSON(t, ts, "", "/api/v1/settings", &all)
	for key, def := range map[string]string{store.BackupsAutoMinKey: "60", store.BackupsAutoMaxKey: "120", store.BackupsKeepDaysAutoKey: "90",
		store.BackupsKeepDaysManualKey: "365", store.BackupsKeepNewestKey: "10"} {
		if all[key].Value != def || all[key].Default != def || all[key].Set {
			t.Fatalf("%s: %+v", key, all[key])
		}
	}
	steps := []struct {
		body string
		want int
	}{
		{`{"backups.auto_min": 200}`, 400},
		{`{"backups.auto_min": 30, "backups.auto_max": 40}`, 200},
		{`{"backups.auto_max": 20}`, 400},
		{`{"backups.auto_min": null, "backups.auto_max": null}`, 200},
		{`{"backups.keep_newest": -1}`, 400},
		{`{"backups.keep_days_auto": 0, "backups.keep_newest": 3}`, 200},
	}
	for _, s := range steps {
		if code, out := do(t, ts, "PUT", "/api/v1/settings", s.body); code != s.want {
			t.Fatalf("%s: %d %v", s.body, code, out)
		}
	}
	getJSON(t, ts, "", "/api/v1/settings", &all)
	if all[store.BackupsAutoMinKey].Set || all[store.BackupsKeepNewestKey].Value != "3" || all[store.BackupsKeepDaysAutoKey].Value != "0" {
		t.Fatalf("after the changes: %+v %+v", all[store.BackupsAutoMinKey], all[store.BackupsKeepNewestKey])
	}
}

// The diff of two stored backups is what a restore of the one would send
// to a Cube as the other found it: nothing against itself; a name, a
// setting and a programme against an edited copy; an unknown id or a
// missing against are refused.
func TestBackupDiff(t *testing.T) {
	_, ts, s, st := startStored(t, Options{})
	code, out := do(t, ts, "POST", backupsBase, "")
	if code != 201 {
		t.Fatalf("take: %d %v", code, out)
	}
	a := int64(out["id"].(float64))
	f := withRoomName(backupWithEco(backupOf(t, ts), 17.5), "Edited")
	for i := range f.Devices {
		if f.Devices[i].RFAddress == 0x1514b6 {
			f.Devices[i].Thermostat.WeekProgram[0][0].Temp += 1
		}
	}
	b, err := st.AddBackup(context.Background(), store.Backup{CubeID: s.cubes["keq0000100"].storeID.Load(), Reason: store.ReasonManual, Format: "json"}, marshal(t, f))
	if err != nil {
		t.Fatal(err)
	}
	diff := func(id, against int64) (int, backupDiff) {
		t.Helper()
		var d backupDiff
		code := getJSON(t, ts, "", backupsBase+"/"+strconv.FormatInt(id, 10)+"/diff?against="+strconv.FormatInt(against, 10), &d)
		return code, d
	}
	if code, d := diff(a, a); code != 200 || len(d.Steps) != 0 || d.Steps == nil {
		t.Fatalf("against itself: %d %+v", code, d)
	}
	code, d := diff(b.ID, a)
	if code != 200 {
		t.Fatalf("against the edited copy: %d", code)
	}
	var whats []string
	for _, st := range d.Steps {
		whats = append(whats, st.What)
	}
	joined := strings.Join(whats, "; ")
	// The names in one step without radio; the device's settings and its
	// programme in one with two radio commands.
	if len(d.Steps) != 2 || d.RadioCommands != 2 || !strings.Contains(joined, `"Edited"`) || !strings.Contains(joined, "eco 17.5") || !strings.Contains(joined, "program saturday") {
		t.Fatalf("against the edited copy: %d radio, %s", d.RadioCommands, joined)
	}
	if code, _ := diff(a, b.ID); code != 200 {
		t.Fatalf("the other way: %d", code)
	}
	if code, _ := diff(999, a); code != 404 {
		t.Fatalf("an unknown id: %d", code)
	}
	if code := getJSON(t, ts, "", backupsBase+"/"+strconv.FormatInt(a, 10)+"/diff", &d); code != 400 {
		t.Fatalf("without against: %d", code)
	}
}
