// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
)

// callWith sends a request with extra headers and returns the response.
func callWith(t *testing.T, ts *httptest.Server, method, path, token, body string, headers map[string]string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func commandOfRow(t *testing.T, ts *httptest.Server, token string, row auditView) commandView {
	t.Helper()
	c, _, _ := commandByID(t, ts, token, row)
	return c
}

func commandByID(t *testing.T, ts *httptest.Server, token string, row auditView) (commandView, int, string) {
	t.Helper()
	st := serverStore(t, ts)
	c, ok, err := st.Command(t.Context(), row.CommandID)
	if err != nil || !ok {
		t.Fatalf("command %d of the audit row: %v %v", row.CommandID, ok, err)
	}
	code, text := callAPI(t, ts, "GET", "/api/v1/commands/"+c.UUID, token, "")
	var v commandView
	json.Unmarshal([]byte(text), &v)
	return v, code, text
}

var stores = map[*httptest.Server]*store.Store{}

func serverStore(t *testing.T, ts *httptest.Server) *store.Store {
	t.Helper()
	st := stores[ts]
	if st == nil {
		t.Fatal("no store for the server")
	}
	return st
}

func countRadio(fc *fakecube.Server) int {
	n := 0
	for _, m := range fc.Received() {
		if m.Type == 's' {
			n++
		}
	}
	return n
}

// A write is a command with its steps; the audit row names it; a retry
// with the Idempotency-Key answers the same and sends nothing twice.
func TestCommandsOfWrites(t *testing.T) {
	fc, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	stores[ts] = s.opts.Store
	setpoint := "/api/v1/cubes/KEQ0000100/rooms/6/setpoint"
	resp, text := callWith(t, ts, "PUT", setpoint, "secret", `{"mode":"auto"}`, map[string]string{"Idempotency-Key": "k1"})
	if resp.StatusCode != 200 || resp.Header.Get("Idempotent-Replayed") != "" {
		t.Fatalf("setpoint: %d %s", resp.StatusCode, text)
	}
	sent := countRadio(fc)
	rows := auditRows(t, ts, "secret", "?limit=1")
	if len(rows) != 1 || rows[0].CommandID == 0 {
		t.Fatalf("audit row %+v", rows)
	}
	c := commandOfRow(t, ts, "secret", rows[0])
	if c.State != store.StateAcknowledged || c.Status != 200 || c.Action != "setpoint" || c.Target != "room:6" || c.Cube != "KEQ0000100" || c.User != store.BootstrapUser || c.FinishedAt == nil || len(c.Steps) != 1 {
		t.Fatalf("command %+v", c)
	}
	step := c.Steps[0]
	var req struct {
		Key   string `json:"key"`
		Lines int    `json:"lines"`
		Radio int    `json:"radio"`
	}
	json.Unmarshal(step.Request, &req)
	if step.State != store.StateAcknowledged || step.Target != "" && !strings.HasPrefix(step.Target, "room:") || req.Key != "setpoint:room:6" || req.Lines != 1 || req.Radio != 1 || step.FinishedAt == nil {
		t.Fatalf("step %+v %+v", step, req)
	}
	if !strings.Contains(string(c.Result), `"ok": true`) && !strings.Contains(string(c.Result), `"ok":true`) {
		t.Fatalf("result %s", c.Result)
	}

	// The retry: the same answer, nothing sent, no new rows.
	resp2, text2 := callWith(t, ts, "PUT", setpoint, "secret", `{"mode":"auto"}`, map[string]string{"Idempotency-Key": "k1"})
	if resp2.StatusCode != 200 || resp2.Header.Get("Idempotent-Replayed") != "true" || text2 != text {
		t.Fatalf("replay: %d %q %q (first %q)", resp2.StatusCode, resp2.Header.Get("Idempotent-Replayed"), text2, text)
	}
	if countRadio(fc) != sent {
		t.Fatal("the replay sent a radio command")
	}
	if again := auditRows(t, ts, "secret", "?limit=10"); len(again) != 1 {
		t.Fatalf("the replay left an audit row: %d", len(again))
	}
	// Another request under the key, a key too long, and a key of
	// another user.
	if resp, _ := callWith(t, ts, "PUT", setpoint, "secret", `{"mode":"manual","temp":20}`, map[string]string{"Idempotency-Key": "k1"}); resp.StatusCode != 422 {
		t.Fatalf("another request under the key: %d", resp.StatusCode)
	}
	if resp, _ := callWith(t, ts, "PUT", setpoint, "secret", `{"mode":"auto"}`, map[string]string{"Idempotency-Key": strings.Repeat("x", 129)}); resp.StatusCode != 400 {
		t.Fatalf("a long key: %d", resp.StatusCode)
	}
	op := userToken(t, s.opts.Store, "op", store.RoleOperator)
	if resp, _ := callWith(t, ts, "PUT", setpoint, op, `{"mode":"auto"}`, map[string]string{"Idempotency-Key": "k1"}); resp.StatusCode != 200 || countRadio(fc) != sent+1 {
		t.Fatalf("another user's key: %d, radio %d", resp.StatusCode, countRadio(fc))
	}

	// A refused request is no command, a failed one a dropped command.
	viewer := userToken(t, s.opts.Store, "vi", store.RoleViewer)
	callAPI(t, ts, "PUT", setpoint, viewer, `{"mode":"auto"}`)
	if rows := auditRows(t, ts, "secret", "?limit=1"); rows[0].Result != store.ResultRefused || rows[0].CommandID != 0 {
		t.Fatalf("refused request's row %+v", rows[0])
	}
	callAPI(t, ts, "PUT", setpoint, "secret", `{"mode":"boost"}`)
	rows = auditRows(t, ts, "secret", "?limit=1")
	if c := commandOfRow(t, ts, "secret", rows[0]); c.State != store.StateDropped || c.Status != 400 || len(c.Steps) != 0 {
		t.Fatalf("failed request's command %+v", c)
	}

	// A move is several steps; a user without audit sees own commands only.
	callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a010d", "secret", `{"room_id":5}`)
	rows = auditRows(t, ts, "secret", "?limit=1")
	c, code, _ := commandByID(t, ts, "secret", rows[0])
	if code != 200 || c.Action != "device.update" || len(c.Steps) < 2 {
		t.Fatalf("move: %d %+v", code, c)
	}
	if _, code, _ := commandByID(t, ts, op, rows[0]); code != 404 {
		t.Fatalf("another user's command: %d", code)
	}
	callAPI(t, ts, "PUT", setpoint, op, `{"mode":"auto"}`)
	rows = auditRows(t, ts, "secret", "?limit=1")
	if _, code, _ := commandByID(t, ts, op, rows[0]); code != 200 {
		t.Fatalf("own command: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/commands/nope", "secret", ""); code != 404 {
		t.Fatalf("unknown command: %d", code)
	}
}

// A restore's command is sent with its 202 and finished by its run.
func TestRestoreCommand(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	stores[ts] = s.opts.Store
	code0, data := callAPI(t, ts, "GET", "/api/v1/cubes/KEQ0000100/backup", "secret", "")
	f, err := backup.Unmarshal([]byte(data), false)
	if code0 != 200 || err != nil {
		t.Fatalf("backup: %d %v", code0, err)
	}
	body, _ := f.Marshal(false)
	code, text := callAPI(t, ts, "POST", restoreBase+"/plan", "secret", string(body))
	var plan restorePlan
	json.Unmarshal([]byte(text), &plan)
	if code != 200 || plan.Plan == "" {
		t.Fatalf("plan: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "POST", restoreBase+"?plan="+plan.Plan, "secret", string(body)); code != 202 {
		t.Fatalf("restore: %d %s", code, text)
	}
	rows := auditRows(t, ts, "secret", "?limit=1")
	var c commandView
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if c = commandOfRow(t, ts, "secret", rows[0]); store.Final(c.State) {
			break
		}
	}
	if c.Action != "restore.start" || c.State != store.StateAcknowledged || c.Status != 202 || !strings.Contains(string(c.Result), `"state"`) {
		t.Fatalf("restore command %+v", c)
	}
}

// Rows left queued or sent by a stopped server are dropped at the next
// start; commands.keep_days is a setting.
func TestCommandsAtStart(t *testing.T) {
	dir := t.TempDir()
	st := testStore(t, dir)
	if _, err := st.AddCommand(t.Context(), store.Command{Action: "setpoint"}); err != nil {
		t.Fatal(err)
	}
	st.Close()
	_, ts, s := startWith(t, storeOptions(t, dir, "secret"))
	if pending, _ := s.opts.Store.Pending(t.Context()); len(pending) != 0 {
		t.Fatalf("pending after the start: %+v", pending)
	}
	code, text := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", `{"commands.keep_days": 30}`)
	if code != 200 || !strings.Contains(text, `"commands.keep_days"`) {
		t.Fatalf("set keep_days: %d %s", code, text)
	}
	var view map[string]settingView
	json.Unmarshal([]byte(text), &view)
	if v := view[store.CommandsKeepDaysKey]; v.Value != "30" || !v.Set || v.Default != "0" || v.Applies != "now" {
		t.Fatalf("keep_days view %+v", v)
	}
	if d, _ := s.opts.Store.CommandsKeepDays(t.Context()); d != 30 {
		t.Fatalf("keep_days %d", d)
	}
	for _, body := range []string{`{"commands.keep_days": -1}`, `{"commands.keep_days": "many"}`, `{"commands.keep_days": 99999}`} {
		if code, _ := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", body); code != 400 {
			t.Errorf("%s: %d", body, code)
		}
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", `{"commands.keep_days": "7"}`); code != 200 {
		t.Fatalf("keep_days as a string: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", `{"commands.keep_days": null}`); code != 200 {
		t.Fatalf("keep_days default: %d", code)
	}
	if d, _ := s.opts.Store.CommandsKeepDays(t.Context()); d != 0 {
		t.Fatalf("keep_days after the default %d", d)
	}
}

// A new API key is shown once: neither the command row of its creation,
// nor GET /api/v1/commands/{id}, nor a replay of the request with its
// Idempotency-Key gives the secret out again.
func TestTokenCreationKeepsNoSecret(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	stores[ts] = s.opts.Store
	userToken(t, s.opts.Store, "anna", store.RoleViewer)
	headers := map[string]string{"Idempotency-Key": "key-1"}
	resp, text := callWith(t, ts, "POST", "/api/v1/users/anna/tokens", "secret", `{"label":"laptop"}`, headers)
	var made struct {
		Token string `json:"token"`
		ID    int64  `json:"id"`
	}
	if json.Unmarshal([]byte(text), &made); resp.StatusCode != 201 || made.Token == "" || made.ID == 0 {
		t.Fatalf("token: %d %s", resp.StatusCode, text)
	}
	rows := auditRows(t, ts, "secret", "?limit=1")
	if len(rows) != 1 || rows[0].Action != "token.add" || rows[0].CommandID == 0 {
		t.Fatalf("audit row %+v", rows)
	}
	c, ok, err := s.opts.Store.Command(t.Context(), rows[0].CommandID)
	if err != nil || !ok {
		t.Fatalf("command row: %v %v", ok, err)
	}
	if strings.Contains(string(c.Result), made.Token) {
		t.Errorf("the command row keeps the secret: %s", c.Result)
	}
	if _, code, body := commandByID(t, ts, "secret", rows[0]); code != 200 || strings.Contains(body, made.Token) {
		t.Errorf("GET /api/v1/commands/{id} gives the secret out: %d %s", code, body)
	}
	resp2, text2 := callWith(t, ts, "POST", "/api/v1/users/anna/tokens", "secret", `{"label":"laptop"}`, headers)
	var again struct {
		Token *string `json:"token"`
		ID    int64   `json:"id"`
	}
	json.Unmarshal([]byte(text2), &again)
	if resp2.StatusCode != 201 || resp2.Header.Get("Idempotent-Replayed") != "true" || again.Token != nil || again.ID != made.ID {
		t.Errorf("replay: %d %q %s", resp2.StatusCode, resp2.Header.Get("Idempotent-Replayed"), text2)
	}
}

// The step observer never makes the queue wait on the store: with the
// store's write lock held by another connection, as an events batch or
// a slow handler holds it, its calls still return at once, and the step
// rows are written once the lock is free.
func TestStepObserverDoesNotWaitOnTheStore(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	stores[ts] = s.opts.Store
	st := s.opts.Store
	parent, err := st.AddCommand(t.Context(), store.Command{Action: "setpoint"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := sql.Open("sqlite", "file:"+filepath.ToSlash(st.File())+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	conn, err := other.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(t.Context(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	o := s.newObserver()
	cmd := engine.Command{ID: parent.UUID, Seq: 1, Lines: []cube.Message{{Type: 's', Data: "AARAAAAACgENBms="}}}
	start := time.Now()
	o.Enqueued(cmd)
	o.Started(cmd)
	o.Finished(cmd, engine.Result{})
	took := time.Since(start)
	conn.ExecContext(t.Context(), "ROLLBACK")
	conn.Close()
	if took > 500*time.Millisecond {
		t.Fatalf("the observer waited %s on the store's lock", took)
	}
	var c store.Command
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if c, _, _ = st.Command(t.Context(), parent.ID); len(c.Steps) == 1 && c.Steps[0].State == store.StateAcknowledged {
			return
		}
	}
	t.Fatalf("steps after the lock: %+v", c.Steps)
}

// A retry whose first request was in flight when the server stopped,
// its row dropped at the start with no status answered, gets 503 with
// what the row says, not a panic.
func TestReplayOfADroppedCommand(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	owner, _, _ := st.User(t.Context(), store.BootstrapUser)
	path, body := "/api/v1/cubes/KEQ0000100/rooms/6/setpoint", `{"mode":"auto"}`
	fp := fingerprint(&auditInfo{method: "PUT", path: path, body: []byte(body)}, "")
	req, _ := json.Marshal(map[string]any{"method": "PUT", "path": path, "query": "", "hash": fp})
	if _, err := st.AddCommand(t.Context(), store.Command{UserID: owner.ID, Key: "key-1", Action: "setpoint", Request: req}); err != nil {
		t.Fatal(err)
	}
	if n, err := st.DropUnfinished(t.Context(), "server restarted"); err != nil || n != 1 {
		t.Fatalf("dropped %d: %v", n, err)
	}
	req2, _ := http.NewRequest("PUT", ts.URL+path, strings.NewReader(body))
	req2.Header.Set("Authorization", "Bearer secret")
	req2.Header.Set("Idempotency-Key", "key-1")
	resp, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("the replay: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 503 || resp.Header.Get("Idempotent-Replayed") != "true" || !strings.Contains(string(b), "server restarted") {
		t.Fatalf("the replay: %d %q %s", resp.StatusCode, resp.Header.Get("Idempotent-Replayed"), b)
	}
}

// A replay answers what the first request answered: a restore's 202
// with its id and steps, also once its run has finished the command.
func TestReplayOfARestoreKeepsItsAnswer(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	stores[ts] = s.opts.Store
	_, data := callAPI(t, ts, "GET", "/api/v1/cubes/KEQ0000100/backup", "secret", "")
	f, err := backup.Unmarshal([]byte(data), false)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := f.Marshal(false)
	_, text := callAPI(t, ts, "POST", restoreBase+"/plan", "secret", string(body))
	var plan restorePlan
	json.Unmarshal([]byte(text), &plan)
	headers := map[string]string{"Idempotency-Key": "restore-1"}
	resp, first := callWith(t, ts, "POST", restoreBase+"?plan="+plan.Plan, "secret", string(body), headers)
	if resp.StatusCode != 202 {
		t.Fatalf("restore: %d %s", resp.StatusCode, first)
	}
	rows := auditRows(t, ts, "secret", "?limit=1")
	var c commandView
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if c = commandOfRow(t, ts, "secret", rows[0]); store.Final(c.State) {
			break
		}
	}
	if !store.Final(c.State) || !strings.Contains(string(c.Result), `"state"`) {
		t.Fatalf("the run did not finish the command: %+v", c)
	}
	resp2, again := callWith(t, ts, "POST", restoreBase+"?plan="+plan.Plan, "secret", string(body), headers)
	var a, b map[string]any
	json.Unmarshal([]byte(first), &a)
	json.Unmarshal([]byte(again), &b)
	if resp2.StatusCode != 202 || resp2.Header.Get("Idempotent-Replayed") != "true" || len(b) != len(a) || b["id"] != a["id"] || b["steps"] != a["steps"] {
		t.Fatalf("the replay: %d %s, first %s", resp2.StatusCode, again, first)
	}
}

// firstAnswer replays an answer that was not JSON as its text, a run's
// 202 as its answer, and any other result as it is.
func TestFirstAnswer(t *testing.T) {
	for _, c := range []struct {
		status       int
		result, want string
	}{
		{500, `{"answer":"plain text"}`, "plain text"},
		{202, `{"answer":{"id":"r1","steps":2},"id":"r1","state":"done"}`, `{"id":"r1","steps":2}`},
		{200, `{"ok":true}`, `{"ok":true}`},
		{200, `{"answer":{"x":1},"other":2}`, `{"answer":{"x":1},"other":2}`},
	} {
		if got := string(firstAnswer(store.Command{Status: c.status, Result: json.RawMessage(c.result)})); got != c.want {
			t.Errorf("%d %s: %s, want %s", c.status, c.result, got, c.want)
		}
	}
}
