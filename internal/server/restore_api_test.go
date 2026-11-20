// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
)

const restoreBase = "/api/v1/cubes/KEQ0000100/restore"

// ecoLine is the 0x11 radiogram that sets 0a010d (room 6 of the fixture)
// to its fixture temperatures with eco 17.0 instead of 16.5.
const ecoLine = "s:AAARAAAACgENBisiPQkHGAM="

// backupOf downloads the Cube's backup through the API.
func backupOf(t *testing.T, ts *httptest.Server) *backup.File {
	t.Helper()
	resp, err := http.Get(ts.URL + "/api/v1/cubes/KEQ0000100/backup")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("backup: %d %v", resp.StatusCode, err)
	}
	f, err := backup.Unmarshal(data, false)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// backupWithEco sets the eco temperature of 0a010d in the backup.
func backupWithEco(f *backup.File, eco float64) *backup.File {
	for i := range f.Devices {
		if f.Devices[i].RFAddress == 0x0a010d {
			f.Devices[i].Thermostat.EcoTemp = eco
		}
	}
	return f
}

// withRoomName renames room 6 in the backup.
func withRoomName(f *backup.File, name string) *backup.File {
	for i := range f.Rooms {
		if f.Rooms[i].ID == 6 {
			f.Rooms[i].Name = name
		}
	}
	return f
}

// marshal encodes the backup as JSON.
func marshal(t *testing.T, f *backup.File) []byte {
	t.Helper()
	data, err := f.Marshal(false)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// postBackup sends body to path with the content type and decodes the
// JSON answer.
func postBackup(t *testing.T, ts *httptest.Server, path, contentType string, body []byte) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("POST", ts.URL+path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// plan asks for the plan of f and returns it, failing unless it is 200.
func plan(t *testing.T, ts *httptest.Server, query string, f *backup.File) map[string]any {
	t.Helper()
	code, out := postBackup(t, ts, restoreBase+"/plan"+query, "application/json", marshal(t, f))
	if code != 200 {
		t.Fatalf("plan%s: %d %v", query, code, out)
	}
	return out
}

// noReads leaves out the lines that only read: the polls, and the NTP
// reads of the backups the tests take.
var noReads = cubewatch.Except('l', 'f')

func TestRestorePlan(t *testing.T) {
	t.Parallel()
	r := startServer(t)
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	f := backupOf(t, ts)

	// An unchanged backup gives an empty plan with a stable hash.
	p := plan(t, ts, "", f)
	if len(p["steps"].([]any)) != 0 || p["radio_commands"] != 0.0 || p["duty_cost_pct"] != 0.0 || len(p["notes"].([]any)) != 0 || p["plan"] == "" {
		t.Fatalf("plan of an unchanged backup %v", p)
	}
	empty := p["plan"].(string)
	if again := plan(t, ts, "", f); again["plan"] != empty {
		t.Fatalf("hash changed between two plans: %v, %v", empty, again["plan"])
	}
	if b, ok := p["budget"].(map[string]any); !ok || b["free_memory_slots"] != 50.0 {
		t.Fatalf("budget %v", p["budget"])
	}

	// One changed eco temperature is one radio step to the device,
	// after a wakeup of its room.
	backupWithEco(f, 17)
	p = plan(t, ts, "", f)
	steps := p["steps"].([]any)
	if len(steps) != 1 || p["radio_commands"] != 1.0 || p["duty_cost_pct"] != 1.25 {
		t.Fatalf("plan with one change %v", p)
	}
	st := steps[0].(map[string]any)
	what, _ := st["what"].(string)
	if !strings.HasPrefix(what, `0a010d "`) || !strings.Contains(what, "temperatures") || !strings.Contains(what, "eco 17.0") ||
		st["radio"] != true || st["device"] != "0a010d" || st["room"] != 6.0 || st["result"] != nil {
		t.Fatalf("step %v", st)
	}
	changed := p["plan"].(string)
	if changed == empty {
		t.Fatal("hash did not change with the backup")
	}
	if plan(t, ts, "", f)["plan"] != changed {
		t.Fatal("hash of the changed backup not stable")
	}

	// YAML by Content-Type, and by the body alone.
	yml, err := f.Marshal(true)
	if err != nil {
		t.Fatal(err)
	}
	for _, ct := range []string{"application/yaml", "text/yaml; charset=utf-8", ""} {
		if code, out := postBackup(t, ts, restoreBase+"/plan", ct, yml); code != 200 || out["plan"] != changed {
			t.Fatalf("YAML plan with Content-Type %q: %d %v", ct, code, out)
		}
	}

	// Options as the command's flags.
	if p := plan(t, ts, "?skip=settings,clock", f); len(p["steps"].([]any)) != 0 {
		t.Fatalf("skip=settings still plans %v", p["steps"])
	}
	if p := plan(t, ts, "?state&portal=false", f); len(p["steps"].([]any)) != 1 {
		t.Fatalf("state and portal change the plan: %v", p["steps"])
	}
	for _, q := range []string{"?skip=bogus", "?portal=maybe", "?state=2"} {
		if code, _ := postBackup(t, ts, restoreBase+"/plan"+q, "application/json", marshal(t, f)); code != 400 {
			t.Fatalf("plan%s: %d, want 400", q, code)
		}
	}

	// A device that is not paired is a note.
	g := backupOf(t, ts)
	g.Devices = append(g.Devices, backup.Device{RFAddress: 0x123456, Type: cube.DeviceRadiatorThermostat, Serial: "MEQ0000000", Name: "Neu", RoomID: 1})
	if p := plan(t, ts, "", g); len(p["steps"].([]any)) != 0 || len(p["notes"].([]any)) != 1 || !strings.Contains(p["notes"].([]any)[0].(string), "123456") {
		t.Fatalf("plan with an unpaired device %v", p)
	}

	// Refusals: another Cube's backup, no serial, bad files, too large.
	g = backupOf(t, ts)
	g.Cube.Serial = "KEQ0000000"
	if code, out := postBackup(t, ts, restoreBase+"/plan", "application/json", marshal(t, g)); code != 400 || !strings.Contains(out["error"].(string), "KEQ0000000") {
		t.Fatalf("other cube's backup: %d %v", code, out)
	}
	g.Cube.Serial = ""
	if code, _ := postBackup(t, ts, restoreBase+"/plan", "application/json", marshal(t, g)); code != 400 {
		t.Fatalf("backup without serial: %d", code)
	}
	for _, body := range []string{"", `{"version": 99}`, `{"cube": [}`, "not: [a backup", "[1, 2]"} {
		if code, _ := postBackup(t, ts, restoreBase+"/plan", "", []byte(body)); code != 400 {
			t.Fatalf("body %q: %d, want 400", body, code)
		}
	}
	big := append(marshal(t, f), bytes.Repeat([]byte(" "), maxBackupBytes)...)
	if code, _ := postBackup(t, ts, restoreBase+"/plan", "application/json", big); code != 413 {
		t.Fatalf("body of %d bytes: %d, want 413", len(big), code)
	}
	// Planning writes nothing.
	if got := lineStrings(received.Since(0, noReads)); len(got) != 0 {
		t.Fatalf("planning sent %v", got)
	}
	if code, out := do(t, ts, "GET", restoreBase, ""); code != 200 || out["state"] != "none" {
		t.Fatalf("status before any restore: %d %v", code, out)
	}
}

func TestRestoreRunSendsThePlan(t *testing.T) {
	t.Parallel()
	r := startServer(t)
	fc, ts, s := r.fc, r.ts, r.s
	rt := s.cubes["keq0000100"]
	f := backupWithEco(backupOf(t, ts), 17)
	hash := plan(t, ts, "", f)["plan"].(string)

	ch := make(chan event, 64)
	rt.subsMu.Lock()
	rt.subs[ch] = &subscriber{gone: make(chan struct{})}
	rt.subsMu.Unlock()
	events := testkit.ChanWatch(t, ch, eventText)
	received := cubewatch.Lines(fc)
	before := received.Mark()
	code, out := postBackup(t, ts, restoreBase+"?plan="+hash, "application/json", marshal(t, f))
	if code != 202 || out["id"] == "" || out["steps"] != 1.0 {
		t.Fatalf("restore: %d %v", code, out)
	}
	id := out["id"].(string)
	var st map[string]any
	testkit.Eventually(t, "the restore", func() bool {
		_, st = do(t, ts, "GET", restoreBase, "")
		return st["state"] == "done"
	})
	// Exactly the wakeup of room 6 and the 0x11 radiogram.
	if got := lineStrings(received.Since(before, noReads)); len(got) != 2 || got[0] != "z:1e,G,06" || got[1] != ecoLine {
		t.Fatalf("restore sent %v", got)
	}
	// The state has the step with its result.
	steps, _ := st["steps"].([]any)
	if st["id"] != id || st["plan"] != hash || st["sent"] != 1.0 || st["radio_commands"] != 1.0 || st["finished"] == nil || len(steps) != 1 {
		t.Fatalf("state %v", st)
	}
	res, _ := steps[0].(map[string]any)["result"].(map[string]any)
	if res["ok"] != true || res["discarded"] != nil || res["error"] != nil || res["budget"] == nil {
		t.Fatalf("step result %v", res)
	}
	// The stream got the step and the end.
	// restore_started is the system's line, without the run's progress.
	isRun := func(ev event) bool { return ev.Kind == "restore_step" || ev.Kind == "restore_done" }
	var kinds []string
	for c := testkit.Cursor(0); len(kinds) < 2; {
		var ev event
		ev, c = events.Await(t, c, "the restore's step and end on the stream", isRun)
		kinds = append(kinds, ev.Kind)
		if ev.Restore == nil || ev.Restore.ID != id || ev.Restore.Steps != 1 || !ev.Restore.OK {
			t.Fatalf("event %+v", ev)
		}
		switch ev.Kind {
		case "restore_step":
			if ev.Restore.Step != 1 || ev.Device != "0a010d" || !strings.Contains(ev.Restore.What, "eco 17.0") {
				t.Fatalf("step event %+v %+v", ev, ev.Restore)
			}
		case "restore_done":
			if ev.Restore.State != "done" || ev.Restore.Sent != 1 {
				t.Fatalf("done event %+v", ev.Restore)
			}
		}
	}
	if strings.Join(kinds, ",") != "restore_step,restore_done" {
		t.Fatalf("events %v", kinds)
	}
	// And the log remembers them.
	var evs []event
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log", &evs)
	logged := map[string]bool{}
	for _, e := range evs {
		logged[e.Kind] = true
	}
	if !logged["restore_step"] || !logged["restore_done"] {
		t.Fatalf("log kinds %v", logged)
	}
	rt.subsMu.Lock()
	delete(rt.subs, ch)
	rt.subsMu.Unlock()
}

func TestRestoreFailsWhenTheCubeDiscards(t *testing.T) {
	t.Parallel()
	ts := startServer(t, withFake(func(fc *fakecube.Server) { fc.Discard = true })).ts
	f := backupWithEco(backupOf(t, ts), 17)
	hash := plan(t, ts, "", f)["plan"].(string)
	if code, out := postBackup(t, ts, restoreBase+"?plan="+hash, "application/json", marshal(t, f)); code != 202 {
		t.Fatalf("restore: %d %v", code, out)
	}
	var st map[string]any
	testkit.Eventually(t, "the restore", func() bool {
		_, st = do(t, ts, "GET", restoreBase, "")
		return st["state"] != "running"
	})
	res, _ := st["steps"].([]any)[0].(map[string]any)["result"].(map[string]any)
	if st["state"] != "failed" || st["sent"] != 0.0 || st["error"] == nil || res["ok"] != false || res["discarded"] != true {
		t.Fatalf("state %v", st)
	}
	var evs []event
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log", &evs)
	var done *event
	for i := range evs {
		if evs[i].Kind == "restore_done" {
			done = &evs[i]
		}
	}
	if done == nil || done.Restore.State != "failed" || done.Restore.OK || done.Error == "" {
		t.Fatalf("done event %+v", done)
	}
}

func TestRestoreRefusals(t *testing.T) {
	t.Parallel()
	// The queue pauses half a second between commands, so a restore of
	// two steps runs long enough for a second one to be refused.
	r := startServer(t, withOptions(Options{Queue: engine.Options{Gap: 500 * time.Millisecond}}))
	fc, ts, s := r.fc, r.ts, r.s
	received := cubewatch.Lines(fc)
	rt := s.cubes["keq0000100"]
	f := backupWithEco(backupOf(t, ts), 17)
	hash := plan(t, ts, "", f)["plan"].(string)

	if code, _ := postBackup(t, ts, restoreBase, "application/json", marshal(t, f)); code != 400 {
		t.Fatalf("restore without plan: %d, want 400", code)
	}
	if code, out := postBackup(t, ts, restoreBase+"?plan=0123abcd", "application/json", marshal(t, f)); code != 409 {
		t.Fatalf("restore with another hash: %d %v", code, out)
	}
	g := backupWithEco(backupOf(t, ts), 17)
	g.Cube.Serial = "KEQ0000000"
	if code, _ := postBackup(t, ts, restoreBase+"?plan="+hash, "application/json", marshal(t, g)); code != 400 {
		t.Fatalf("restore of another cube's backup: %d, want 400", code)
	}
	big := append(marshal(t, f), bytes.Repeat([]byte(" "), maxBackupBytes)...)
	if code, _ := postBackup(t, ts, restoreBase+"?plan="+hash, "application/json", big); code != 413 {
		t.Fatalf("restore with a body of %d bytes: %d, want 413", len(big), code)
	}
	if got := lineStrings(received.Since(0, noReads)); len(got) != 0 {
		t.Fatalf("refused restores sent %v", got)
	}

	// A second restore while one runs.
	withRoomName(f, "Renamed")
	hash = plan(t, ts, "", f)["plan"].(string)
	before := received.Mark()
	code, out := postBackup(t, ts, restoreBase+"?plan="+hash, "application/json", marshal(t, f))
	if code != 202 || out["steps"] != 2.0 {
		t.Fatalf("restore: %d %v", code, out)
	}
	if code, out := postBackup(t, ts, restoreBase+"?plan="+hash, "application/json", marshal(t, f)); code != 409 || !strings.Contains(out["error"].(string), "running") {
		t.Fatalf("second restore: %d %v", code, out)
	}
	if code, st := do(t, ts, "GET", restoreBase, ""); code != 200 || st["state"] != "running" || st["id"] != out["id"] {
		t.Fatalf("state while running: %d %v", code, st)
	}
	testkit.Eventually(t, "the restore", func() bool {
		_, st := do(t, ts, "GET", restoreBase, "")
		return st["state"] == "done"
	})
	got := lineStrings(received.Since(before, noReads))
	if len(got) != 3 || !strings.HasPrefix(got[0], "m:00,") || got[1] != "z:1e,G,06" || got[2] != ecoLine {
		t.Fatalf("restore sent %v", got)
	}
	// The names are mirrored into the snapshot, so a plan from the same
	// backup lists only the radio step again: the Cube changes the
	// device's configuration once the device acknowledged.
	if p := plan(t, ts, "", f); len(p["steps"].([]any)) != 1 {
		t.Fatalf("plan after the restore %v", p["steps"])
	}

	// A Cube that is gone.
	fc.Close()
	testkit.Eventually(t, "the disconnect", func() bool { return !rt.client.Connected() }, awayWait)
	if code, _ := postBackup(t, ts, restoreBase+"/plan", "application/json", marshal(t, f)); code != 503 {
		t.Fatalf("plan without the cube: %d, want 503", code)
	}
}

func TestRestoreRefusesHighDutyCycle(t *testing.T) {
	t.Parallel()
	// The greeting's H line reports a duty cycle of 85 %.
	greeting := recorded(t)
	greeting[0].Data = strings.Replace(greeting[0].Data, ",00,32,", ",55,32,", 1)
	r := startServer(t, withGreeting(greeting))
	ts, received := r.ts, r.lines
	f := backupWithEco(backupOf(t, ts), 17)
	p := plan(t, ts, "", f)
	if b := p["budget"].(map[string]any); b["duty_cycle_pct"] != 85.0 {
		t.Fatalf("budget %v", b)
	}
	hash := p["plan"].(string)
	code, out := postBackup(t, ts, restoreBase+"?plan="+hash, "application/json", marshal(t, f))
	if code != 409 || !strings.Contains(out["error"].(string), "85 %") {
		t.Fatalf("restore at 85 %%: %d %v", code, out)
	}
	// Without radio commands the budget does not matter.
	g := withRoomName(backupOf(t, ts), "Renamed")
	hash = plan(t, ts, "", g)["plan"].(string)
	if code, out := postBackup(t, ts, restoreBase+"?plan="+hash, "application/json", marshal(t, g)); code != 202 {
		t.Fatalf("names-only restore at 85 %%: %d %v", code, out)
	}
	testkit.Eventually(t, "the restore", func() bool {
		_, st := do(t, ts, "GET", restoreBase, "")
		return st["state"] == "done"
	})
	if got := lineStrings(received.Since(0, noReads)); len(got) != 1 || !strings.HasPrefix(got[0], "m:00,") {
		t.Fatalf("restore sent %v", got)
	}
}
