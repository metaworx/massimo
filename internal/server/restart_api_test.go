// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/update"
)

// restartCounter is a RestartOptions.Restart that counts its calls and
// fails with err.
type restartCounter struct {
	calls atomic.Int32
	err   error
}

func (c *restartCounter) restart() error {
	c.calls.Add(1)
	return c.err
}

func decodeRestart(t *testing.T, text string) restartView {
	t.Helper()
	var v restartView
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatalf("%v: %s", err, text)
	}
	return v
}

// With users, a holder of restart restarts the server once: the answer
// is 202, the restart is asked for, the request is an audit row and an
// acknowledged command, its retry is replayed, a second one is refused
// while the first is under way, and the Cube is sent nothing.
func TestRestartRoutes(t *testing.T) {
	c := &restartCounter{}
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Restart = RestartOptions{By: "systemd", Restart: c.restart}
	fc, ts, s := startWith(t, opts)
	stores[ts] = s.opts.Store
	st := s.opts.Store
	manager := userToken(t, st, "manager1", store.RoleManager)
	operator := userToken(t, st, "operator1", store.RoleOperator)
	path := "/api/v1/restart"
	code, text := callAPI(t, ts, "GET", path, "secret", "")
	if v := decodeRestart(t, text); code != 200 || !v.Available || v.By != "systemd" || v.Blocked != "" {
		t.Fatalf("GET as owner: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "GET", path, manager, ""); code != 200 || !decodeRestart(t, text).Available {
		t.Fatalf("GET as manager: %d %s", code, text)
	}
	for _, m := range []string{"GET", "POST"} {
		if code, text := callAPI(t, ts, m, path, operator, "{}"); code != 403 || errorOf(text) != `capability "restart" required` {
			t.Fatalf("%s as operator: %d %s", m, code, text)
		}
		if code, text := callAPI(t, ts, m, path, "", "{}"); code != 401 {
			t.Fatalf("%s without a key: %d %s", m, code, text)
		}
	}
	before := len(fc.Received())
	headers := map[string]string{"Idempotency-Key": "restart-1"}
	resp, text := callWith(t, ts, "POST", path, manager, "{}", headers)
	var answer struct {
		OK bool   `json:"ok"`
		By string `json:"by"`
	}
	if json.Unmarshal([]byte(text), &answer); resp.StatusCode != 202 || !answer.OK || answer.By != "systemd" {
		t.Fatalf("POST as manager: %d %s", resp.StatusCode, text)
	}
	if n := c.calls.Load(); n != 1 {
		t.Fatalf("restart asked %d times", n)
	}
	rows := auditRows(t, ts, "secret", "?limit=1")
	if len(rows) != 1 || rows[0].Action != "server.restart" || rows[0].User != "manager1" {
		t.Fatalf("audit %+v", rows)
	}
	if cmd := commandOfRow(t, ts, "secret", rows[0]); cmd.Action != "server.restart" || cmd.State != store.StateAcknowledged || cmd.Status != 202 {
		t.Fatalf("the restart's command %+v", cmd)
	}
	if resp, text := callWith(t, ts, "POST", path, manager, "{}", headers); resp.StatusCode != 202 || resp.Header.Get("Idempotent-Replayed") != "true" || c.calls.Load() != 1 {
		t.Fatalf("the retry: %d %s, %d restarts", resp.StatusCode, text, c.calls.Load())
	}
	if code, text := callAPI(t, ts, "POST", path, "secret", "{}"); code != 409 || errorOf(text) != "a restart is under way" {
		t.Fatalf("a second restart: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "GET", path, "secret", ""); code != 200 || decodeRestart(t, text).Available || decodeRestart(t, text).Blocked != "a restart is under way" {
		t.Fatalf("GET during the restart: %d %s", code, text)
	}
	if got := fc.Received()[before:]; len(got) != 0 {
		t.Fatalf("the restart sent the Cube %v", got)
	}
}

// Without a service manager that starts the server again, while an
// update runs, and when the restart cannot be started, the API says why
// and restarts nothing.
func TestRestartBlocked(t *testing.T) {
	opts := storeOptions(t, t.TempDir(), "secret")
	_, ts, _ := startWith(t, opts)
	path := "/api/v1/restart"
	code, text := callAPI(t, ts, "GET", path, "secret", "")
	if v := decodeRestart(t, text); code != 200 || v.Available || v.By != "" || v.Blocked != "no service manager would start this server again" {
		t.Fatalf("no restart configured: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "POST", path, "secret", "{}"); code != 409 || errorOf(text) != "no service manager would start this server again" {
		t.Fatalf("POST without a restart: %d %s", code, text)
	}

	opts = storeOptions(t, t.TempDir(), "secret")
	opts.Restart = RestartOptions{Blocked: "the restart is switched off (-restart off)", Restart: func() error { t.Error("restarted"); return nil }}
	_, ts, _ = startWith(t, opts)
	if code, text := callAPI(t, ts, "POST", path, "secret", "{}"); code != 409 || errorOf(text) != "the restart is switched off (-restart off)" {
		t.Fatalf("switched off: %d %s", code, text)
	}

	status := filepath.Join(t.TempDir(), "status.json")
	if err := update.WriteStatus(status, update.Status{State: "installing", To: "v0.1.0-alpha.10", Time: time.Now()}); err != nil {
		t.Fatal(err)
	}
	c := &restartCounter{}
	opts = storeOptions(t, t.TempDir(), "secret")
	opts.Restart = RestartOptions{By: "launchd", Restart: c.restart}
	opts.Update = UpdateOptions{Status: status}
	_, ts, _ = startWith(t, opts)
	if code, text := callAPI(t, ts, "POST", path, "secret", "{}"); code != 409 || errorOf(text) != "an update is running, and it restarts the server itself" || c.calls.Load() != 0 {
		t.Fatalf("during an update: %d %s", code, text)
	}

	c = &restartCounter{err: errors.New("start C:\\Program Files\\Massimo\\massimo.exe: access denied")}
	opts = storeOptions(t, t.TempDir(), "secret")
	opts.Restart = RestartOptions{By: "windows", Restart: c.restart}
	_, ts, _ = startWith(t, opts)
	for i := 1; i <= 2; i++ {
		if code, text := callAPI(t, ts, "POST", path, "secret", "{}"); code != 500 || !strings.Contains(errorOf(text), "access denied") || c.calls.Load() != int32(i) {
			t.Fatalf("a restart that fails, %d: %d %s", i, code, text)
		}
	}
}

// An open server restarts only for a request from the box itself: a
// loopback peer without a forwarding header. Anyone else, the LAN and a
// reverse proxy's client, is refused with the reason, and another
// site's page is refused before that; through a proxy it does not
// trust, the open server answers nothing at all.
func TestRestartOpenServer(t *testing.T) {
	c := &restartCounter{}
	opts := storeOptions(t, t.TempDir(), "")
	opts.Restart = RestartOptions{By: "exit", Restart: c.restart}
	_, ts, s := startWith(t, opts)
	trusted, err := ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	opts = storeOptions(t, t.TempDir(), "")
	opts.Restart = RestartOptions{By: "exit", Restart: c.restart}
	opts.TrustedProxies = trusted
	_, proxied, _ := startWith(t, opts)
	path := "/api/v1/restart"
	for _, h := range []map[string]string{{"X-Forwarded-For": "192.168.1.20"}, {"X-Real-IP": "192.168.1.20"}, {"Forwarded": "for=192.168.1.20"}} {
		for _, m := range []string{"GET", "POST"} {
			if code, text := sendWith(t, ts, m, path, "{}", h); code != 403 || errorOf(text) != openRefused {
				t.Errorf("%s %v, a proxy not trusted: %d %s", m, h, code, text)
			}
		}
		if code, text := sendWith(t, proxied, "GET", path, "", h); code != 200 || decodeRestart(t, text).Available || decodeRestart(t, text).Blocked != openRestartRefused {
			t.Errorf("GET %v, a trusted proxy: %d %s", h, code, text)
		}
		if code, text := sendWith(t, proxied, "POST", path, "{}", h); code != 403 || errorOf(text) != openRestartRefused {
			t.Errorf("POST %v, a trusted proxy: %d %s", h, code, text)
		}
	}
	if code, text := sendWith(t, ts, "POST", path, "{}", map[string]string{"Sec-Fetch-Site": "cross-site", "Content-Type": "text/plain"}); code != 403 || errorOf(text) != "cross-origin request refused" {
		t.Errorf("another site's page: %d %s", code, text)
	}
	req := httptest.NewRequest("POST", path, strings.NewReader("{}"))
	req.RemoteAddr = "192.168.1.20:50000"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != 403 || errorOf(rec.Body.String()) != openRestartRefused {
		t.Errorf("from the LAN: %d %s", rec.Code, rec.Body.String())
	}
	if c.calls.Load() != 0 {
		t.Fatalf("restarted %d times by refused requests", c.calls.Load())
	}
	if code, text := sendWith(t, ts, "POST", path, "{}", nil); code != 202 || c.calls.Load() != 1 {
		t.Fatalf("from the box: %d %s", code, text)
	}
}

// A request from the box itself has a loopback peer and no forwarding
// header.
func TestFromTheBox(t *testing.T) {
	for addr, want := range map[string]bool{"127.0.0.1:5000": true, "[::1]:5000": true, "[::ffff:127.0.0.1]:5000": true, "127.0.0.53:5000": true,
		"192.168.1.20:5000": false, "10.0.0.1:5000": false, "[fe80::1]:5000": false, "nonsense": false} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = addr
		if got := fromTheBox(r); got != want {
			t.Errorf("%s: %v", addr, got)
		}
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:5000"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	if fromTheBox(r) {
		t.Error("a proxied request counted as the box's")
	}
}

// POST /api/v1/reload reads the settings file again through what
// cmd/massimo hands the server: for a holder of restart it answers what
// changed and what waits for the next start, GET /api/v1/restart then
// names the file and when it was read, the request is an audit row and
// no command, and the Cube is sent nothing; without restart it is
// refused, and a server without a reloader says it reads no file.
func TestReloadRoute(t *testing.T) {
	opts := storeOptions(t, t.TempDir(), "secret")
	fc, ts, s := startWith(t, opts)
	stores[ts] = s.opts.Store
	st := s.opts.Store
	manager := userToken(t, st, "manager1", store.RoleManager)
	operator := userToken(t, st, "operator1", store.RoleOperator)
	path := "/api/v1/reload"
	if code, text := callAPI(t, ts, "POST", path, "secret", "{}"); code != 409 || errorOf(text) != "this server reads no settings file" {
		t.Fatalf("without a reloader: %d %s", code, text)
	}
	at := time.Date(2026, 11, 8, 19, 52, 0, 0, time.UTC)
	calls := 0
	s.SetReloader("/etc/massimo/env", func(context.Context) (ReloadResult, error) {
		calls++
		s.SetSettingsRead(at)
		return ReloadResult{Changed: []string{"MASSIMO_LISTEN"}, Pending: []string{"MASSIMO_LISTEN"}, ReadAt: at}, nil
	})
	before := len(fc.Received())
	code, text := callAPI(t, ts, "POST", path, manager, "{}")
	var res ReloadResult
	if err := json.Unmarshal([]byte(text), &res); err != nil || code != 200 || calls != 1 ||
		strings.Join(res.Changed, ",") != "MASSIMO_LISTEN" || strings.Join(res.Pending, ",") != "MASSIMO_LISTEN" || !res.ReadAt.Equal(at) {
		t.Fatalf("POST as manager: %d %s (%d calls)", code, text, calls)
	}
	_, text = callAPI(t, ts, "GET", "/api/v1/restart", "secret", "")
	if v := decodeRestart(t, text); v.File != "/etc/massimo/env" || v.ReadAt == nil || !v.ReadAt.Equal(at) {
		t.Fatalf("GET /api/v1/restart: %s", text)
	}
	rows := auditRows(t, ts, "secret", "?limit=1")
	if len(rows) != 1 || rows[0].Action != "server.reload" || rows[0].User != "manager1" {
		t.Fatalf("audit %+v", rows)
	}
	if code, text := callAPI(t, ts, "POST", path, operator, "{}"); code != 403 || errorOf(text) != `capability "restart" required` {
		t.Fatalf("as operator: %d %s", code, text)
	}
	s.SetReloader("/etc/massimo/env", func(context.Context) (ReloadResult, error) {
		return ReloadResult{}, errors.New("permission denied")
	})
	if code, text := callAPI(t, ts, "POST", path, "secret", "{}"); code != 500 || !strings.Contains(errorOf(text), "permission denied") {
		t.Fatalf("a file that cannot be read: %d %s", code, text)
	}
	if got := fc.Received()[before:]; len(got) != 0 {
		t.Fatalf("the reload sent the Cube %v", got)
	}
}
