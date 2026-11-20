// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
)

// An integration's users are shown under its name (AP StoreRaise v1.2,
// Block 4): PUT sets the integration's name, its own user's display name,
// beside the one it reports; the users, /me, the audit and the log carry
// each user's chain, "<the integration's name> › <its display name>",
// with ", as <login name>" where it acts as a Massimo user; the filter by
// user finds the rows made with its rights and by it; and the keys of
// before stay, undocumented.
func TestIntegrationNamesShown(t *testing.T) {
	t.Parallel()
	rg := startServer(t, storedRooms(), quiet())
	ts, st := rg.ts, rg.st
	ctx := t.Context()
	i, key := registerHA(t, st, []string{"read", "setpoint"})
	owner := userToken(t, st, "boss", store.RoleOwner)
	if _, err := st.AddUser(ctx, store.User{Name: "anna", DisplayName: "Anna", Role: store.RoleOperator}, "", 0); err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/integrations/" + i.UUID

	// PUT sets the name the owner gives, and the answer says it.
	code, text := callAPI(t, ts, "PUT", base, owner, `{"name":"  Home "}`)
	var put map[string]any
	decodeInto(t, text, &put)
	if code != 200 || put["name"] != "Home" || put["reported_name"] != "Casa Leone" || put["uuid"] != i.UUID || put["key"] != i.UUID || put["user"] != i.UUID+":" {
		t.Fatalf("PUT name: %d %s", code, text)
	}
	got, _, _ := st.IntegrationByUUID(ctx, i.UUID)
	own, _, _ := st.User(ctx, i.UUID+":")
	if got.Name != "Casa Leone" || got.DisplayName != "Home" || own.DisplayName != "Home" {
		t.Fatalf("stored: the integration %q %q, its own user %q", got.Name, got.DisplayName, own.DisplayName)
	}
	for _, bad := range []string{`{"name":"  "}`, `{"name":"` + strings.Repeat("x", maxDisplayName+1) + `"}`} {
		if code, _ := callAPI(t, ts, "PUT", base, owner, bad); code != 400 {
			t.Fatalf("PUT %s: %d", bad, code)
		}
	}

	// A setpoint by Anna through the integration, then linked to anna.
	setpoint := "/api/v1/cubes/KEQ0000100/rooms/2/setpoint"
	if resp, text := callWith(t, ts, "PUT", setpoint, key, `{"mode":"auto"}`, as(haAnna, LevelControl)); resp.StatusCode != 200 {
		t.Fatalf("setpoint: %d %s", resp.StatusCode, text)
	}
	if code, text := callAPI(t, ts, "PUT", base+"/users/"+haAnna, owner, `{"user":"anna"}`); code != 200 {
		t.Fatalf("link: %d %s", code, text)
	}
	if resp, text := callWith(t, ts, "PUT", setpoint, key, `{"mode":"manual","temp":20}`, as(haAnna, LevelControl)); resp.StatusCode != 200 {
		t.Fatalf("setpoint, linked: %d %s", resp.StatusCode, text)
	}
	subject := i.UUID + ":" + haAnna
	const chain, linked = "Home › Anna", "Home › Anna, as anna"

	// The users list and /me.
	var list []userView
	_, text = callAPI(t, ts, "GET", "/api/v1/users", owner, "")
	decodeInto(t, text, &list)
	want := map[string]string{"anna": "Anna", "boss": "boss", i.UUID + ":": "Home", i.UUID + ":system": "Home › system", subject: linked}
	for _, u := range list {
		if u.Display != want[u.Name] {
			t.Errorf("%s shown as %q, want %q", u.Name, u.Display, want[u.Name])
		}
		if (u.Integration != nil) != strings.HasPrefix(u.Name, i.UUID) || u.Integration != nil && (u.Integration.UUID != i.UUID || u.Integration.Key != i.UUID) {
			t.Errorf("%s: integration %+v", u.Name, u.Integration)
		}
		if u.Name == "anna" && (len(u.Links) != 1 || u.Links[0].Display != chain) {
			t.Errorf("anna's links %+v", u.Links)
		}
	}
	if !strings.Contains(text, `"display_name": "Anna"`) {
		t.Errorf("display_name gone: %s", text)
	}
	resp, text := callWith(t, ts, "GET", "/api/v1/me", key, "", nil)
	var me meView
	decodeInto(t, text, &me)
	if resp.StatusCode != 200 || me.User.Display != "Home" || me.User.DisplayName != "Home" || me.User.Integration == nil || me.User.Integration.UUID != i.UUID ||
		me.Integration.Name != "Home" || me.Integration.ReportedName != "Casa Leone" || me.Integration.Key != i.UUID {
		t.Fatalf("me: %d %s", resp.StatusCode, text)
	}
	var det integrationView
	_, text = callAPI(t, ts, "GET", base, owner, "")
	decodeInto(t, text, &det)
	if det.Name != "Home" || len(det.Users) != 2 || det.Users[1].Display != linked || det.Users[0].Display != "Home › system" {
		t.Fatalf("the integration's users: %s", text)
	}

	// The audit: both setpoints for Anna's integration user, the linked
	// one for anna as well; user keeps the effective user's name.
	rows := auditRows(t, ts, owner, "?user="+subject)
	var sets []auditView
	for _, r := range rows {
		if r.Action == "setpoint" {
			sets = append(sets, r)
		}
	}
	if len(sets) != 2 {
		t.Fatalf("the audit of %s: %+v", subject, rows)
	}
	if r := sets[0]; r.User != "anna" || r.EffectiveUser == nil || r.EffectiveUser.Name != "anna" || r.EffectiveUser.Display != "Anna" ||
		r.RealUser == nil || r.RealUser.Name != subject || r.RealUser.Display != linked || r.RealUser.Integration != i.UUID {
		t.Fatalf("the linked setpoint: %+v %+v %+v", r, r.EffectiveUser, r.RealUser)
	}
	if r := sets[1]; r.User != subject || r.RealUser == nil || r.RealUser.Display != chain || r.EffectiveUser.Display != chain {
		t.Fatalf("the setpoint before the link: %+v %+v %+v", r, r.EffectiveUser, r.RealUser)
	}
	if rows := auditRows(t, ts, owner, "?user=anna"); len(rows) == 0 || rows[0].Action != "setpoint" || rows[0].RealUser.Name != subject {
		t.Fatalf("the audit of anna: %+v", rows)
	}

	// The log: the change lines, named alike, found by either user.
	if err := st.FlushEvents(ctx); err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{subject, "anna"} {
		var evs []map[string]any
		if code := getJSON(t, ts, owner, "/api/v1/log?kind=change&user="+who, &evs); code != 200 {
			t.Fatalf("the log of %s: %d", who, code)
		}
		found := false
		for _, ev := range evs {
			r, _ := ev["real_user"].(map[string]any)
			e, _ := ev["effective_user"].(map[string]any)
			by, _ := ev["by"].(map[string]any)
			if r != nil && r["display"] == linked && e != nil && e["name"] == "anna" && by != nil && by["name"] == "anna" {
				found = true
			}
		}
		if !found {
			t.Errorf("the log of %s: %+v", who, evs)
		}
	}
}

// The server's own lines name an integration's user by its chain, any
// other user by its login name.
func TestChainName(t *testing.T) {
	t.Parallel()
	plain := event{By: &eventUser{Name: "anna"}, RealUser: &eventUser{Name: "anna", Display: "Anna"}}
	through := event{By: &eventUser{Name: "anna"}, RealUser: &eventUser{Name: "x:y", Integration: "x", Display: "Home › Anna, as anna"}}
	if got := chainName(plain); got != "anna" {
		t.Errorf("a user: %q", got)
	}
	if got := chainName(through); got != "Home › Anna, as anna" {
		t.Errorf("an integration's user: %q", got)
	}
	if got := chainName(event{By: &eventUser{Name: "anna"}}); got != "anna" {
		t.Errorf("without the real user: %q", got)
	}
}
