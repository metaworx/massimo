// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
)

// The role integration in the store is the ceiling the server knows; no
// regular user is given it, through the API or by a change of role.
func TestIntegrationRoleIsTheCeiling(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	r, ok, err := s.opts.Store.Role(t.Context(), store.RoleIntegration)
	if err != nil || !ok {
		t.Fatal(err)
	}
	want := slices.Clone(integrationCeiling)
	slices.Sort(want)
	if !slices.Equal(r.Capabilities, want) {
		t.Fatalf("the store's integration %v, the server's ceiling %v", r.Capabilities, want)
	}
	for _, c := range []string{"users", "passwords", "roles", "roles.disable", "capabilities", "auth", "integrations"} {
		if slices.Contains(integrationCeiling, c) {
			t.Errorf("the ceiling holds %s", c)
		}
	}
	if code, text := callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name":"ha","role":"integration"}`); code != 403 || errorOf(text) != "the role integration is an integration's and its users' alone" {
		t.Fatalf("a user with the role integration: %d %s", code, text)
	}
	callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name":"anna","role":"viewer"}`)
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/users/anna", "secret", `{"role":"integration"}`); code != 403 {
		t.Fatalf("anna given the role integration: %d", code)
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/roles/integration", "secret", `{"capabilities":["read"]}`); code < 400 {
		t.Fatalf("the role integration changed: %d", code)
	}
}

// An integration asks without a key, the owner approves it with its
// maximum, and the integration collects its key with its code; revoked,
// the key is refused.
func TestIntegrationRegistrationRoutes(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	body := `{"kind":"homeassistant","name":"Casa Leone","instance_id":"0123456789ABCDEF0123456789abcdef","url":"http://ha:8123","version":"2026.11.1"}`
	code, text := callAPI(t, ts, "POST", "/api/v1/integrations/requests", "", body)
	if code != 201 {
		t.Fatalf("request: %d %s", code, text)
	}
	var req struct{ ID, Code string }
	decodeInto(t, text, &req)
	if len(req.Code) != 6 || req.ID == "" {
		t.Fatalf("request %+v", req)
	}
	collect := "/api/v1/integrations/requests/" + req.ID + "?code=" + req.Code
	if code, _ := callAPI(t, ts, "GET", collect, "", ""); code != 202 {
		t.Fatalf("collect while waiting: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/integrations/requests/"+req.ID+"?code=XXXXXX", "", ""); code != 404 {
		t.Fatalf("a wrong code: %d", code)
	}
	code, text = callAPI(t, ts, "GET", "/api/v1/integrations", "secret", "")
	if code != 200 || !strings.Contains(text, `"request_code": "`+req.Code+`"`) || !strings.Contains(text, `"state": "pending"`) {
		t.Fatalf("the list: %d %s", code, text)
	}
	approve := "/api/v1/integrations/" + req.ID + "/approve"
	if code, _ := callAPI(t, ts, "POST", approve, "secret", `{"max_caps":["read","users"]}`); code != 400 {
		t.Fatalf("a maximum beyond the role integration: %d", code)
	}
	if code, text := callAPI(t, ts, "POST", approve, "secret", `{"max_caps":["read","setpoint"]}`); code != 200 || !strings.Contains(text, `"key": "casa-leone"`) {
		t.Fatalf("approve: %d %s", code, text)
	}
	code, text = callAPI(t, ts, "GET", collect, "", "")
	var got struct{ Key string }
	decodeInto(t, text, &got)
	if code != 200 || len(got.Key) != 64 {
		t.Fatalf("collect: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "GET", "/api/v1/me", got.Key, ""); code != 200 || !strings.Contains(text, `"casa-leone"`) {
		t.Fatalf("the key: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "GET", collect, "", ""); code != 410 {
		t.Fatalf("collect after the key's use: %d", code)
	}
	if code, _ := callAPI(t, ts, "DELETE", "/api/v1/integrations/"+req.ID, "secret", ""); code != 200 {
		t.Fatalf("revoke: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", got.Key, ""); code != 401 {
		t.Fatalf("the revoked key: %d", code)
	}

	// The setting off refuses requests; an address asking too often waits.
	if err := s.opts.Store.SetSetting(t.Context(), IntegrationRequestsKey, "off"); err != nil {
		t.Fatal(err)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/integrations/requests", "", body); code != 403 {
		t.Fatalf("with the setting off: %d", code)
	}
	s.opts.Store.SetSetting(t.Context(), IntegrationRequestsKey, "on")
	limited := 0
	for n := 0; n < requestBurst+1; n++ {
		if code, _ := callAPI(t, ts, "POST", "/api/v1/integrations/requests", "", body); code == 429 {
			limited++
		}
	}
	if limited == 0 {
		t.Fatal("no request refused for its address")
	}
}

// decodeInto reads a JSON answer into v.
func decodeInto(t *testing.T, text string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(text), v); err != nil {
		t.Fatalf("%v: %s", err, text)
	}
}

// The owner links two of an integration's users to one Massimo user,
// proposed by name: their calls then act as that user, within its role
// (the chain's third link), and the users list shows the links; unlinked,
// the integration's user acts as itself again.
func TestIntegrationLinking(t *testing.T) {
	_, ts, _, st := startRoomsOpts(t, roomsOpts{slow: true})
	ctx := t.Context()
	i, key := registerHA(t, st, []string{"read", "setpoint", "lock"})
	owner := userToken(t, st, "boss", store.RoleOwner)
	manager := userToken(t, st, "mgr", store.RoleManager)
	anna, err := st.AddUser(ctx, store.User{Name: "anna", DisplayName: "Anna", Role: store.RoleViewer}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	phone := map[string]string{OnBehalfHeader: "id=" + haBruno + "; level=control; name=\"Anna's phone\"; instance=" + haInstance}
	setpoint := "/api/v1/cubes/KEQ0000100/rooms/2/setpoint"
	for _, h := range []map[string]string{as(haAnna, LevelControl), phone} {
		if resp, text := callWith(t, ts, "PUT", setpoint, key, `{"mode":"auto"}`, h); resp.StatusCode != 200 {
			t.Fatalf("a setpoint before the link: %d %s", resp.StatusCode, text)
		}
	}
	base := "/api/v1/integrations/" + i.UUID + "/users"
	var got integrationView
	code, text := callAPI(t, ts, "GET", "/api/v1/integrations/"+i.UUID, owner, "")
	decodeInto(t, text, &got)
	users := got.Users
	if code != 200 || got.Key != "casa-leone" || len(users) != 3 || users[0].ID != "system" || users[1].Name != "Anna" || users[1].Proposal != "anna" || users[2].Proposal != "" {
		t.Fatalf("the users %d %s", code, text)
	}

	for _, sub := range []string{haAnna, haBruno} {
		if code, text := callAPI(t, ts, "PUT", base+"/"+sub, owner, `{"user":"anna"}`); code != 200 || !strings.Contains(text, `"linked": true`) {
			t.Fatalf("link %s: %d %s", sub, code, text)
		}
	}
	if rows := auditRows(t, ts, owner, "?limit=1"); len(rows) == 0 || rows[0].Action != "integration.link" || rows[0].Target != "integration:"+i.UUID {
		t.Fatalf("the audit %+v", rows)
	}
	// Anna is a viewer: no setpoint through the integration either.
	for _, h := range []map[string]string{as(haAnna, LevelControl), phone} {
		if resp, _ := callWith(t, ts, "PUT", setpoint, key, `{"mode":"auto"}`, h); resp.StatusCode != 403 {
			t.Fatalf("a setpoint after the link: %d", resp.StatusCode)
		}
	}
	if resp, text := callWith(t, ts, "GET", "/api/v1/me", key, "", phone); resp.StatusCode != 200 || !strings.Contains(text, `"name": "anna"`) || strings.Contains(text, `"setpoint"`) {
		t.Fatalf("me, linked: %d %s", resp.StatusCode, text)
	}
	var list []userView
	_, text = callAPI(t, ts, "GET", "/api/v1/users", owner, "")
	decodeInto(t, text, &list)
	for _, u := range list {
		switch {
		case u.Name == "anna" && len(u.Links) != 2:
			t.Fatalf("anna's links %+v", u.Links)
		case u.Name == "casa-leone" && (u.Integration == nil || u.Integration.ID != ""):
			t.Fatalf("the integration's user %+v", u.Integration)
		case u.Name == "casa-leone."+haAnna[:8] && (u.Integration == nil || u.Integration.LinkedTo != "anna" || u.Integration.ID != haAnna):
			t.Fatalf("Anna's integration user %+v", u.Integration)
		}
	}

	// Refused: the system user, an integration's user, a user the caller
	// does not outrank, a user unknown.
	for _, c := range []struct {
		name, token, sub, body string
		status                 int
	}{
		{"system", owner, "system", `{"user":"anna"}`, 400},
		{"an integration's user", owner, haAnna, `{"user":"casa-leone.` + haBruno[:8] + `"}`, 400},
		{"the owner by a manager", manager, haAnna, `{"user":"boss"}`, 403},
		{"nobody", owner, haAnna, `{"user":"nobody"}`, 404},
		{"an unknown id", owner, "cccccccc11112222333344445555cccc", `{"user":"anna"}`, 404},
	} {
		if code, text := callAPI(t, ts, "PUT", base+"/"+c.sub, c.token, c.body); code != c.status {
			t.Fatalf("%s: %d %s", c.name, code, text)
		}
	}

	if code, text := callAPI(t, ts, "PUT", base+"/"+haAnna, owner, `{"user":null}`); code != 200 || !strings.Contains(text, `"linked": false`) {
		t.Fatalf("unlink: %d %s", code, text)
	}
	if resp, text := callWith(t, ts, "PUT", setpoint, key, `{"mode":"auto"}`, as(haAnna, LevelControl)); resp.StatusCode != 200 {
		t.Fatalf("a setpoint after the unlink: %d %s", resp.StatusCode, text)
	}
	_ = anna
}
