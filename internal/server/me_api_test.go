// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
)

// The preferences of the signed-in user: a round trip, each user's own,
// the capability for a write, the limits, the audit rows and no command.
func TestPreferencesRoutes(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	owner := "secret"
	prefs := "/api/v1/me/preferences"
	code, text := callAPI(t, ts, "PUT", prefs+"/ui", owner, `{"poll":5,"theme":"dark"}`)
	var v preferenceView
	json.Unmarshal([]byte(text), &v)
	if code != 200 || v.Key != "ui" || v.Version != 1 || v.UpdatedAt.IsZero() || !strings.Contains(strings.ReplaceAll(string(v.Value), " ", ""), `"poll":5`) {
		t.Fatalf("put: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "GET", prefs+"/ui", owner, ""); code != 200 || !strings.Contains(text, `"theme": "dark"`) {
		t.Fatalf("get: %d %s", code, text)
	}
	callAPI(t, ts, "PUT", prefs+"/schemes", owner, `{"format":"massimo-schemes/1","schemes":[]}`)
	code, text = callAPI(t, ts, "GET", prefs, owner, "")
	var all map[string]json.RawMessage
	json.Unmarshal([]byte(text), &all)
	if code != 200 || len(all) != 2 || all["ui"] == nil || all["schemes"] == nil {
		t.Fatalf("list: %d %s", code, text)
	}

	// Each user its own; a guest and an integration read, but write not.
	op := userToken(t, st, "op", store.RoleOperator)
	if code, text := callAPI(t, ts, "GET", prefs, op, ""); code != 200 || strings.TrimSpace(text) != "{}" {
		t.Fatalf("another user's list: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", prefs+"/ui", op, `{"poll":20}`); code != 200 {
		t.Fatalf("operator's put: %d", code)
	}
	if code, text := callAPI(t, ts, "GET", prefs+"/ui", owner, ""); code != 200 || strings.Contains(text, "20") {
		t.Fatalf("the operator's write reached the owner's: %s", text)
	}
	for _, role := range []string{store.RoleGuest, store.RoleIntegration} {
		tok := userToken(t, st, "u-"+role, role)
		if code, text := callAPI(t, ts, "PUT", prefs+"/ui", tok, `{"poll":20}`); code != 403 || errorOf(text) != `capability "preferences" required` {
			t.Errorf("%s's put: %d %s", role, code, text)
		}
		if code, _ := callAPI(t, ts, "DELETE", prefs+"/ui", tok, ""); code != 403 {
			t.Errorf("%s's delete: %d", role, code)
		}
		if code, _ := callAPI(t, ts, "GET", prefs, tok, ""); code != 200 {
			t.Errorf("%s's list: %d", role, code)
		}
	}

	// The limits.
	for _, c := range []struct {
		path, body string
		want       int
	}{
		{prefs + "/Bad", `{}`, 400},
		{prefs + "/ui", `nope`, 400},
		{prefs + "/ui", ``, 400},
		{prefs + "/ui", `"` + strings.Repeat("x", store.MaxPreferenceBytes) + `"`, 413},
	} {
		if code, text := callAPI(t, ts, "PUT", c.path, owner, c.body); code != c.want {
			t.Errorf("put %s with %d bytes: %d %s, want %d", c.path, len(c.body), code, text, c.want)
		}
	}
	if code, _ := callAPI(t, ts, "GET", prefs+"/nope", owner, ""); code != 404 {
		t.Fatalf("an unset key: %d", code)
	}

	// The removal, and the audit: a row per write, no command.
	if code, _ := callAPI(t, ts, "DELETE", prefs+"/ui", owner, ""); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", prefs+"/ui", owner, ""); code != 404 {
		t.Fatalf("after the delete: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", prefs+"/ui", owner, ""); code != 200 {
		t.Fatalf("delete twice: %d", code)
	}
	rows := auditRows(t, ts, owner, "?user=owner&limit=20")
	set, del := 0, 0
	for _, r := range rows {
		switch r.Action {
		case "preference.set":
			set++
		case "preference.delete":
			del++
		default:
			continue
		}
		if r.CommandID != 0 || !strings.HasPrefix(r.Target, "preference:") {
			t.Errorf("row %+v", r)
		}
	}
	if set < 2 || del != 2 {
		t.Fatalf("audit rows: %d set, %d delete", set, del)
	}
}

// Without a signed-in user there is no one to keep preferences for.
func TestPreferencesWithoutUser(t *testing.T) {
	_, open, _ := startWith(t, storeOptions(t, t.TempDir(), ""))
	for _, m := range []string{"GET", "PUT"} {
		if code, text := callAPI(t, open, m, "/api/v1/me/preferences/ui", "", `{}`); code != 409 || !strings.Contains(text, "no user signed in") {
			t.Errorf("open server %s: %d %s", m, code, text)
		}
	}
	_, legacy, _ := start(t, "secret")
	if code, _ := callAPI(t, legacy, "GET", "/api/v1/me/preferences", "secret", ""); code != 503 {
		t.Fatalf("no store: %d", code)
	}
}
