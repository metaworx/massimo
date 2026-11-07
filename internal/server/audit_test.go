// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// auditRows reads GET /api/v1/audit with a query.
func auditRows(t *testing.T, ts *httptest.Server, token, query string) []auditView {
	t.Helper()
	code, text := callAPI(t, ts, "GET", "/api/v1/audit"+query, token, "")
	if code != 200 {
		t.Fatalf("audit: %d %s", code, text)
	}
	var rows []auditView
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func detailOf(t *testing.T, row auditView) map[string]any {
	t.Helper()
	var d map[string]any
	if err := json.Unmarshal(row.Detail, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// Every route that changes something names an action and leaves a row
// with the caller, the Cube, the target and how it went; the reads and
// the stream ticket leave none.
func TestAuditOfEveryWrite(t *testing.T) {
	_, ts, _ := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	// A read and the stream ticket leave no row.
	callAPI(t, ts, "GET", "/api/v1/cubes/KEQ0000100/rooms", "secret", "")
	ticketFor(t, ts, "secret", "KEQ0000100")
	if n := len(auditRows(t, ts, "secret", "?limit=1000")); n != 0 {
		t.Fatalf("a read or a ticket left a row: %d", n)
	}
	routes := registeredRoutes(t)
	sort.SliceStable(routes, func(i, j int) bool {
		di, dj := routes[i].method == "DELETE" && routes[i].path == "/api/v1/cubes/{sn}", routes[j].method == "DELETE" && routes[j].path == "/api/v1/cubes/{sn}"
		return !di && dj
	})
	for _, r := range routes {
		action, ok := actions[r.method+" "+r.path]
		if r.method == "GET" {
			if ok {
				t.Errorf("GET %s has an audit action", r.path)
			}
			continue
		}
		if !ok {
			t.Errorf("%s %s has no audit action", r.method, r.path)
			continue
		}
		before := len(auditRows(t, ts, "secret", "?limit=1000"))
		body, want := "{}", action
		switch {
		case r.method == "POST" && r.path == "/api/v1/users":
			body = `{"role":"guest"}`
		case r.path == "/api/v1/login":
			body, want = `{"user":"nobody","password":"x"}`, "login.failed"
		}
		code, _ := callAPI(t, ts, r.method, fillRoute(r.path), "secret", body)
		rows := auditRows(t, ts, "secret", "?limit=1000")
		if action == "" {
			if len(rows) != before {
				t.Errorf("%s %s left a row", r.method, r.path)
			}
			continue
		}
		if len(rows) != before+1 {
			t.Errorf("%s %s: %d rows, want %d", r.method, r.path, len(rows), before+1)
			continue
		}
		row := rows[0]
		result := store.ResultFailed
		switch {
		case code < 300:
			result = store.ResultOK
		case code == 401 || code == 403 || code == 429:
			result = store.ResultRefused
		}
		if row.Action != want || row.Result != result || row.Remote != "127.0.0.1" {
			t.Errorf("%s %s (%d): row %+v, want action %s result %s", r.method, r.path, code, row, want, result)
		}
		// The sign-ins have no caller yet, whatever token they carry.
		anonymous := r.path == "/api/v1/login" || strings.HasPrefix(r.path, "/api/v1/login/")
		if !anonymous && row.User != store.BootstrapUser || anonymous && row.User != "" {
			t.Errorf("%s %s: user %q", r.method, r.path, row.User)
		}
		if strings.Contains(r.path, "{sn}") && row.Cube != "KEQ0000100" {
			t.Errorf("%s %s: cube %q", r.method, r.path, row.Cube)
		}
		d := detailOf(t, row)
		if d["method"] != r.method || d["path"] != fillRoute(r.path) || d["status"] != float64(code) {
			t.Errorf("%s %s: detail %v", r.method, r.path, d)
		}
		if code >= 400 && d["error"] == nil {
			t.Errorf("%s %s: no error in the detail of a %d", r.method, r.path, code)
		}
	}
}

// The rows' targets, the refused ones, the secrets removed, and the
// listing's filters.
func TestAuditDetails(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	viewer := userToken(t, st, "vi", store.RoleViewer)
	callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6/setpoint", "secret", `{"mode":"auto"}`)
	callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0A010D/lock", "secret", `{"locked":false}`)
	callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6/setpoint", viewer, `{"mode":"auto"}`)
	callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name":"Zed","password":"pw","role":"viewer"}`)
	callAPI(t, ts, "POST", "/api/v1/roles", "secret", `{"key":"night"}`)
	callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"Zed","password":"wrong"}`)
	callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"zed","password":"pw"}`)
	callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6/setpoint", "secret", `{"mode":"boost"}`)
	rows := auditRows(t, ts, "secret", "")
	if len(rows) != 8 {
		t.Fatalf("%d rows", len(rows))
	}
	for i, want := range []struct {
		action, user, target, result string
	}{
		{"setpoint", store.BootstrapUser, "room:6", store.ResultFailed},
		{"login", "", "user:zed", store.ResultOK},
		{"login.failed", "", "user:zed", store.ResultRefused},
		{"role.add", store.BootstrapUser, "role:night", store.ResultOK},
		{"user.add", store.BootstrapUser, "user:zed", store.ResultOK},
		{"setpoint", "vi", "room:6", store.ResultRefused},
		{"device.lock", store.BootstrapUser, "device:0a010d", store.ResultOK},
		{"setpoint", store.BootstrapUser, "room:6", store.ResultOK},
	} {
		r := rows[i]
		if r.Action != want.action || r.User != want.user || r.Target != want.target || r.Result != want.result {
			t.Errorf("row %d: %+v, want %+v", i, r, want)
		}
	}
	d := detailOf(t, rows[4])
	body, _ := d["body"].(map[string]any)
	if body["password"] != "…" || body["name"] != "Zed" || d["bytes"] == nil {
		t.Fatalf("user.add detail %v", d)
	}
	if d := detailOf(t, rows[0]); !strings.Contains(d["error"].(string), "boost") {
		t.Fatalf("failed setpoint detail %v", d)
	}
	if d := detailOf(t, rows[5]); !strings.Contains(d["error"].(string), "capability") {
		t.Fatalf("refused setpoint detail %v", d)
	}
	if mine := auditRows(t, ts, "secret", "?user=vi"); len(mine) != 1 || mine[0].Action != "setpoint" {
		t.Fatalf("by user: %+v", mine)
	}
	if page := auditRows(t, ts, "secret", "?limit=2"); len(page) != 2 || page[0].ID != rows[0].ID {
		t.Fatalf("limit: %+v", page)
	}
	if older := auditRows(t, ts, "secret", "?before="+rows[0].Time.UTC().Format(time.RFC3339Nano)); len(older) != 7 || older[0].ID != rows[1].ID {
		t.Fatalf("before: %d rows", len(older))
	}
	for _, q := range []string{"?before=yesterday", "?limit=0", "?limit=1001"} {
		if code, _ := callAPI(t, ts, "GET", "/api/v1/audit"+q, "secret", ""); code != 400 {
			t.Errorf("%s: %d", q, code)
		}
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/audit", viewer, ""); code != 403 {
		t.Fatalf("a viewer reads the audit: %d", code)
	}
	// A server without a store audits nothing and has no audit route.
	_, legacy, _ := start(t, "secret")
	if code, _ := callAPI(t, legacy, "GET", "/api/v1/audit", "secret", ""); code != 503 {
		t.Fatalf("audit without a store: %d", code)
	}
}

// countingBody counts the bytes the server reads of a request body.
type countingBody struct {
	r io.Reader
	n *atomic.Int64
}

func (b countingBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.n.Add(int64(n))
	return n, err
}

func (b countingBody) Close() error { return nil }

// Requests refused before anyone is known, without a token, with a
// wrong one or from a locked-out address, leave one audit row per
// address and minute with their count, written when the minute ends (here:
// when the server stops), and none of their bodies is read.
func TestRefusedRequestsAuditedPerMinute(t *testing.T) {
	opts := storeOptions(t, t.TempDir(), "secret")
	s := New(opts)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	var read atomic.Int64
	body := strings.Repeat(`{"mode":"auto"}`, 1000)
	codes := map[int]int{}
	for i := 0; i < 20; i++ {
		r := httptest.NewRequest("PUT", "/api/v1/cubes/KEQ0000100/rooms/6/setpoint", countingBody{strings.NewReader(body), &read})
		r.RemoteAddr = "192.0.2.10:4321"
		if i%2 == 1 {
			r.Header.Set("Authorization", "Bearer wrong")
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		codes[w.Code]++
	}
	if codes[401] != 10 || codes[429] != 10 {
		t.Fatalf("answers %v, want ten 401 and ten 429", codes)
	}
	if n := read.Load(); n != 0 {
		t.Errorf("%d bytes of refused bodies read", n)
	}
	if rows, _ := opts.Store.AuditList(t.Context(), time.Time{}, 100, ""); len(rows) > 1 {
		t.Errorf("%d audit rows within the minute, want at most one", len(rows))
	}
	cancel()
	<-done
	rows, err := opts.Store.AuditList(t.Context(), time.Time{}, 100, "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("%d audit rows, want one: %v", len(rows), err)
	}
	row := rows[0]
	var d map[string]any
	json.Unmarshal(row.Detail, &d)
	statuses, _ := d["statuses"].(map[string]any)
	if row.Result != store.ResultRefused || row.Remote != "192.0.2.10" || row.Action != "setpoint" || row.Cube != "KEQ0000100" || row.UserID != 0 ||
		d["count"] != float64(20) || statuses["401"] != float64(10) || statuses["429"] != float64(10) || d["body"] != nil || d["bytes"] != nil {
		t.Fatalf("row %+v, detail %v", row, d)
	}
}

// A row of refused requests is written at the address's next refusal
// after its minute, and when more addresses are open than the limit,
// the oldest first.
func TestRefusalAuditWindows(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 7, 8, 0, 0, 0, time.UTC)}
	var written []refusal
	var mu sync.Mutex
	ra := newRefusalAudit(func(f refusal) { mu.Lock(); written = append(written, f); mu.Unlock() })
	ra.now, ra.max = clock.now, 2
	defer ra.close()
	rows := func() []refusal {
		mu.Lock()
		defer mu.Unlock()
		return append([]refusal(nil), written...)
	}
	open := func() int {
		ra.mu.Lock()
		defer ra.mu.Unlock()
		return len(ra.open)
	}
	r := httptest.NewRequest("POST", "/api/v1/users", nil)
	for i := 0; i < 3; i++ {
		ra.note(r, "POST /api/v1/users", "user.add", "192.0.2.10", 401)
	}
	clock.t = clock.t.Add(refusalWindow)
	ra.note(r, "POST /api/v1/users", "user.add", "192.0.2.10", 429)
	if got := rows(); len(got) != 1 || got[0].count != 3 || got[0].status != 401 || got[0].remote != "192.0.2.10" {
		t.Fatalf("after the minute: %+v", got)
	}
	// Each address a second later than the one before, so that the
	// oldest is one.
	clock.t = clock.t.Add(time.Second)
	ra.note(r, "POST /api/v1/users", "user.add", "192.0.2.11", 401)
	clock.t = clock.t.Add(time.Second)
	ra.note(r, "POST /api/v1/users", "user.add", "192.0.2.12", 401) // a third address: the oldest goes
	if got := rows(); len(got) != 2 || got[1].remote != "192.0.2.10" || got[1].count != 1 || got[1].status != 429 || open() != 2 {
		t.Fatalf("over the limit: %+v, %d open", got, open())
	}
}
