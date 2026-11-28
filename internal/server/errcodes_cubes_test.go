// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"go.metaworx.ch/massimo/internal/testkit"
)

// refusalCodeParams reads the code and the params of an error body, its
// JSON text or the map do decodes it into: "" for no code, the params as
// compact JSON with their keys in order, "null" for none.
func refusalCodeParams(t *testing.T, body any) (string, string) {
	t.Helper()
	m, ok := body.(map[string]any)
	if text, isText := body.(string); isText {
		if err := json.Unmarshal([]byte(text), &m); err != nil {
			t.Fatalf("no JSON error body: %q", text)
		}
		ok = true
	}
	if !ok {
		t.Fatalf("no error body: %v", body)
	}
	code, _ := m["code"].(string)
	params, _ := json.Marshal(m["params"])
	return code, string(params)
}

// refusedWithCode checks a refusal: its status, its code and its params
// as compact JSON with their keys in order ("null" for none).
func refusedWithCode(t *testing.T, what string, status int, body any, wantStatus int, code, params string) {
	t.Helper()
	if c, p := refusalCodeParams(t, body); status != wantStatus || c != code || p != params {
		t.Errorf("%s: %d %s %s, want %d %s %s (%v)", what, status, c, p, wantStatus, code, params, body)
	}
}

// refusalHas reports whether an error body, as refusalCodeParams reads
// it, has the code and the params (compact JSON, "null" for none).
func refusalHas(t *testing.T, body any, code, params string) bool {
	t.Helper()
	c, p := refusalCodeParams(t, body)
	return c == code && p == params
}

// A Cube served for this run only has no row to change or remove
// (cube_not_stored), and one that has not said its serial waits for it
// (cube_serial_not_known_yet), in the change's own words.
func TestCubeRowRefusalCodes(t *testing.T) {
	t.Parallel()
	ts := startServer(t, editOptions(func(o *Options) { o.Store = testStore(t, "") })).ts
	for _, c := range [][3]string{{"PUT", "/api/v1/cubes/KEQ0000100", `{"name":"x"}`}, {"DELETE", "/api/v1/cubes/KEQ0000100", ""}} {
		code, out := do(t, ts, c[0], c[1], c[2])
		refusedWithCode(t, c[0]+" of a Cube of the run", code, out, http.StatusConflict, "cube_not_stored", `{"cube":"KEQ0000100"}`)
	}
	nowhere := discovering(Options{Store: testStore(t, ""), DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{"127.0.0.1:9"}})
	ts = startServer(t, nowhere, withCubes(CubeConfig{Addr: "127.0.0.1:9"})).ts
	code, out := do(t, ts, "PUT", "/api/v1/cubes/127.0.0.1:9", `{"name":"x"}`)
	refusedWithCode(t, "PUT without a serial", code, out, http.StatusConflict, "cube_serial_not_known_yet", "null")
	if out["error"] != "the cube has not reported its serial yet" {
		t.Errorf("PUT without a serial: %v", out)
	}
}

// Without a store GET /api/v1/store says so (server_no_store), and
// without what gathers them GET /api/v1/config does not show the
// settings (config_not_shown).
func TestServerFactsRefusalCodes(t *testing.T) {
	t.Parallel()
	ts := startServer(t, withCubes()).ts
	code, text := callAPI(t, ts, "GET", "/api/v1/store", "", "")
	refusedWithCode(t, "GET /api/v1/store", code, text, http.StatusServiceUnavailable, "server_no_store", "null")
	code, text = callAPI(t, ts, "GET", "/api/v1/config", "", "")
	refusedWithCode(t, "GET /api/v1/config", code, text, http.StatusServiceUnavailable, "config_not_shown", "null")
}

// A Cube without a row keeps no backups (backup_no_history); a stored
// backup that is not there is backup_unknown wherever it is asked for;
// a backup needs the Cube connected (cube_not_connected, a 503 in the
// backup's own words).
func TestBackupRefusalCodes(t *testing.T) {
	t.Parallel()
	ts := startServer(t, editOptions(func(o *Options) { o.Store = testStore(t, "") })).ts
	for _, c := range [][2]string{{"GET", backupsBase}, {"POST", backupsBase}} {
		code, out := do(t, ts, c[0], c[1], "")
		refusedWithCode(t, c[0]+" without a row", code, out, http.StatusServiceUnavailable, "backup_no_history", "null")
	}
	code, out := postBackup(t, ts, restoreBase+"/plan?backup=1", "", nil)
	refusedWithCode(t, "a plan of a stored backup without a row", code, out, http.StatusServiceUnavailable, "backup_no_history", "null")

	r := startServer(t, stored(), quiet())
	ts = r.ts
	code, out = do(t, ts, "POST", backupsBase, "")
	if code != http.StatusCreated {
		t.Fatalf("take: %d %v", code, out)
	}
	id := strconv.FormatInt(int64(out["id"].(float64)), 10)
	unknown := `{"backup":999,"cube":"KEQ0000100"}`
	for _, c := range [][2]string{{"GET", backupsBase + "/999"}, {"GET", backupsBase + "/999/diff?against=" + id}, {"GET", backupsBase + "/" + id + "/diff?against=999"},
		{"DELETE", backupsBase + "/999"}, {"POST", restoreBase + "/plan?backup=999"}} {
		code, out := do(t, ts, c[0], c[1], "")
		refusedWithCode(t, c[0]+" "+c[1], code, out, http.StatusNotFound, "backup_unknown", unknown)
	}

	r.fc.Close()
	testkit.Eventually(t, "the disconnect", func() bool { return !r.rt.client.Connected() }, awayWait)
	code, out = do(t, ts, "POST", backupsBase, "")
	refusedWithCode(t, "a backup without the Cube", code, out, http.StatusServiceUnavailable, "cube_not_connected", "null")
	if out["error"] != "cube not connected" {
		t.Errorf("a backup without the Cube: %v", out)
	}
}

// The reboot of the computer waits for a restart under way
// (restart_under_way), as the restart waits for the reboot.
func TestRebootWaitsForARestart(t *testing.T) {
	t.Parallel()
	f := newRebootFake(nil)
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Reboot = rebootOptions(f)
	r := startServer(t, withOptions(opts))
	r.s.restartGate.asked.Store(true)
	code, text := callAPI(t, r.ts, "POST", "/api/v1/reboot", "secret", `{"delay": 0}`)
	refusedWithCode(t, "a reboot during a restart", code, text, http.StatusConflict, "restart_under_way", "null")
	if errorOf(text) != "a restart is under way" {
		t.Errorf("a reboot during a restart: %s", text)
	}
	f.none(t)
}

// Why a box starts no update and why not now, where the other tests of
// the route do not look: no channel (update_no_channel), a build of no
// release (update_dev_build), no release on the channel
// (update_no_release), and an answer while none is offered
// (update_none_offered).
func TestUpdateRefusalCodes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	serve := func(o UpdateOptions) *httptest.Server {
		t.Helper()
		o.Request, o.Status = filepath.Join(dir, "update-request"), filepath.Join(dir, "status.json")
		ts := httptest.NewServer(New(Options{Token: "secret", Update: o}).Handler())
		t.Cleanup(ts.Close)
		return ts
	}
	for _, c := range []struct {
		what   string
		o      UpdateOptions
		status int
		code   string
		params string
	}{
		{"no channel", UpdateOptions{Version: "v0.1.0-alpha.9", Finder: &fakeFinder{tag: "v0.1.0-alpha.10"}}, http.StatusForbidden, "update_no_channel", "null"},
		{"a modified build", UpdateOptions{Version: "v0.1.0-alpha.9-dirty", Channel: "alpha", Finder: &fakeFinder{tag: "v0.1.0-alpha.10"}},
			http.StatusForbidden, "update_dev_build", "null"},
		{"a build of no release", UpdateOptions{Version: "v0.0.0-dev+0123abc", Channel: "alpha", Finder: &fakeFinder{tag: "v0.1.0-alpha.10"}},
			http.StatusForbidden, "update_dev_build", "null"},
		{"no release", UpdateOptions{Version: "v0.1.0-alpha.9", Channel: "alpha", Finder: &fakeFinder{}}, http.StatusConflict, "update_no_release", `{"channel":"alpha"}`},
	} {
		code, out := call(t, serve(c.o), "POST", "secret")
		refusedWithCode(t, c.what, code, out, c.status, c.code, c.params)
	}

	b := newAnswerBox(t, &fakeFinder{tag: "v0.1.0-alpha.9"})
	code, text := b.answer(t, b.anna, "v0.1.0-alpha.10", "tonight")
	refusedWithCode(t, "an answer while none is offered", code, text, http.StatusConflict, "update_none_offered", `{"version":"v0.1.0-alpha.10"}`)
	if want := `"v0.1.0-alpha.10" is not the version offered (none)`; errorOf(text) != want {
		t.Errorf("an answer while none is offered: %s, want %q", text, want)
	}
}
