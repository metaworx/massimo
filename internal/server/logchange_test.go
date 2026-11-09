// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The change lines (AP Logging v1.1, Block 6): every change made
// through the API is a line under Configuration, with who made it, what
// it set and what changed; a user, role or key with its ids and names as
// they were then; never a secret.

import (
	"encoding/json"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
)

// Every audited action says which line it writes, and the table names
// no action the routes do not have.
func TestEveryActionHasALine(t *testing.T) {
	seen := map[string]bool{}
	for route, action := range actions {
		if action == "" {
			continue
		}
		seen[action] = true
		line, ok := changeLines[action]
		if !ok {
			t.Errorf("%s (%s) says no line of the log", route, action)
		}
		if line != "" && line != "change" && line != "system" {
			t.Errorf("%s: line %q", action, line)
		}
	}
	for action := range changeLines {
		if !seen[action] {
			t.Errorf("changeLines names %q, which no route has", action)
		}
	}
}

// Naming a Cube is the Cube's change line, with the name from and to and
// who; a setpoint is a line of its device.
func TestCubeNamedIsLogged(t *testing.T) {
	_, ts, _, st := startStored(t, Options{})
	token := userToken(t, st, "anna", "owner")
	if code, body := callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", token, `{"name":"Upstairs"}`); code != 200 {
		t.Fatalf("name: %d %s", code, body)
	}
	var evs []event
	getJSON(t, ts, token, "/api/v1/cubes/KEQ0000100/log?kind=change", &evs)
	if len(evs) != 1 {
		t.Fatalf("%d change lines", len(evs))
	}
	e := evs[0]
	changes, _ := e.Summary["changes"].(map[string]any)
	name, _ := changes["name"].(map[string]any)
	if e.Source != "config" || e.Summary["action"] != "cube.update" || e.Summary["target"] != "cube:KEQ0000100" ||
		name["from"] != "" || name["to"] != "Upstairs" || e.By == nil || e.By.Name != "anna" {
		t.Fatalf("the line: %+v", e)
	}
	if code, body := callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a010d/setpoint", token, `{"mode":"manual","temp":21.5}`); code != 200 {
		t.Fatalf("setpoint: %d %s", code, body)
	}
	serial := serialsOf(t, ts, token)["0a010d"]
	getJSON(t, ts, token, "/api/v1/cubes/KEQ0000100/log?kind=change&device="+serial, &evs)
	if len(evs) != 1 || evs[0].Summary["action"] != "setpoint" || evs[0].Serial != serial || evs[0].Summary["target"] != "device:"+serial {
		t.Fatalf("the setpoint's line, by the device: %+v", evs)
	}
	if set, _ := evs[0].Summary["set"].(map[string]any); set["temp"] != 21.5 {
		t.Fatalf("what it set: %v", evs[0].Summary)
	}
}

// A user created is a line of the server's naming the user as created:
// ids and names; the password is in it nowhere.
func TestUserChangeCarriesIds(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	if code, body := callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name":"Zed","display_name":"Zed Z.","password":"pw-secret-123","role":"viewer"}`); code != 201 {
		t.Fatalf("add: %d %s", code, body)
	}
	u, _, _ := s.opts.Store.User(t.Context(), "zed")
	raw := []map[string]any{}
	getJSON(t, ts, "secret", "/api/v1/cubes/KEQ0000100/log?kind=change", &raw)
	if len(raw) != 1 {
		t.Fatalf("%d change lines", len(raw))
	}
	sum, _ := raw[0]["summary"].(map[string]any)
	subject, _ := sum["subject"].(map[string]any)
	if sum["action"] != "user.add" || sum["target"] != "user:zed" || subject["id"] != float64(u.ID) || subject["uuid"] != u.UUID ||
		subject["name"] != "zed" || subject["display_name"] != "Zed Z." || subject["role"] != "viewer" {
		t.Fatalf("the line: %v", raw[0])
	}
	// The bootstrap token is a key of the first owner, whom the line names
	// as the audit does.
	if by, _ := raw[0]["by"].(map[string]any); by["name"] != store.BootstrapUser {
		t.Fatalf("by: %v", raw[0]["by"])
	}
}

// No change line holds a secret: the passwords, the keys and the OPAQUE
// records are left out of what a request set.
func TestNoSecretInLine(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name":"zed","password":"pw-secret-123","role":"owner"}`)
	code, body := callAPI(t, ts, "POST", "/api/v1/users/zed/tokens", "secret", `{"label":"phone"}`)
	if code != 201 {
		t.Fatalf("token: %d %s", code, body)
	}
	if err := s.opts.Store.FlushEvents(t.Context()); err != nil {
		t.Fatal(err)
	}
	_, log := callAPI(t, ts, "GET", "/api/v1/cubes/KEQ0000100/log?kind=change&limit=100", "secret", "")
	if strings.Contains(log, "pw-secret-123") {
		t.Fatalf("a password in the log: %s", log)
	}
	var tok struct{ Token string }
	json.Unmarshal([]byte(body), &tok)
	if tok.Token == "" || strings.Contains(log, tok.Token) {
		t.Fatalf("the key in the log (%q): %s", tok.Token, log)
	}
	if !strings.Contains(log, `"token.add"`) || !strings.Contains(log, `"phone"`) {
		t.Fatalf("the key's line without its label: %s", log)
	}
}
