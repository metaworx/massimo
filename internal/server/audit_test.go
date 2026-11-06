// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"net/http/httptest"
	"sort"
	"strings"
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
	callAPI(t, ts, "GET", "/api/v1/cubes/KEQ0537249/rooms", "secret", "")
	ticketFor(t, ts, "secret", "KEQ0537249")
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
		if want != "login.failed" && row.User != store.BootstrapUser || want == "login.failed" && row.User != "" {
			t.Errorf("%s %s: user %q", r.method, r.path, row.User)
		}
		if strings.Contains(r.path, "{sn}") && row.Cube != "KEQ0537249" {
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
	callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/rooms/6/setpoint", "secret", `{"mode":"auto"}`)
	callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/devices/1514B6/lock", "secret", `{"locked":false}`)
	callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/rooms/6/setpoint", viewer, `{"mode":"auto"}`)
	callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name":"Zed","password":"pw","role":"viewer"}`)
	callAPI(t, ts, "POST", "/api/v1/roles", "secret", `{"key":"night"}`)
	callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"Zed","password":"wrong"}`)
	callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"zed","password":"pw"}`)
	callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0537249/rooms/6/setpoint", "secret", `{"mode":"boost"}`)
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
		{"device.lock", store.BootstrapUser, "device:1514b6", store.ResultOK},
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
