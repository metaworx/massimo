// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
)

const (
	haInstance = "0123456789abcdef0123456789abcdef"
	haAnna     = "aaaaaaaa11112222333344445555aaaa"
	haBruno    = "bbbbbbbb11112222333344445555bbbb"
)

// The header's notation: parameters, a quoted value, a name in RFC 8187,
// the flag alone; what it may not say is refused.
func TestParseOnBehalf(t *testing.T) {
	ob, err := parseOnBehalf(`id=` + strings.ToUpper(haAnna) + `; level=Control; name*=UTF-8''J%C3%BCrg%20M; instance="` + haInstance + `"; system_generated`)
	if err != nil || ob.id != haAnna || ob.level != LevelControl || ob.name != "Jürg M" || ob.instance != haInstance || !ob.systemGenerated {
		t.Fatalf("%+v %v", ob, err)
	}
	if ob, err := parseOnBehalf(`id=system; level=admin; instance=` + haInstance); err != nil || ob.id != "system" || ob.name != "" {
		t.Fatalf("system: %+v %v", ob, err)
	}
	for _, bad := range []string{
		`id=anna; level=view; instance=` + haInstance,
		`id=` + haAnna + `; level=owner; instance=` + haInstance,
		`id=` + haAnna + `; level=view`,
		`id=` + haAnna + `; level=view; instance=` + haInstance + `; name*=latin1''x`,
	} {
		if _, err := parseOnBehalf(bad); err == nil {
			t.Errorf("taken: %s", bad)
		}
	}
}

// registerHA registers a Home Assistant installation with the maximum
// given and returns its integration and key.
func registerHA(t *testing.T, st *store.Store, caps []string) (store.Integration, string) {
	t.Helper()
	ctx := t.Context()
	req, err := st.AddIntegrationRequest(ctx, store.IntegrationRequest{Kind: store.KindHomeAssistant, Name: "Casa Leone", InstanceID: haInstance}, "ABCDEF")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ApproveIntegration(ctx, req.UUID, caps, "", 0); err != nil {
		t.Fatal(err)
	}
	key, i, err := st.CollectIntegrationKey(ctx, req.UUID, "ABCDEF")
	if err != nil {
		t.Fatal(err)
	}
	return i, key
}

// as is a call for a user of the integration at a level.
func as(id, level string) map[string]string {
	return map[string]string{OnBehalfHeader: "id=" + id + "; level=" + level + "; name=Anna; instance=" + haInstance}
}

// A call's capabilities are what every link of the chain leaves: the
// integration's maximum, the integration user's role and the level; and
// each refusal says which it is.
func TestOnBehalfChain(t *testing.T) {
	fc, ts, s, st := startRoomsOpts(t, roomsOpts{slow: true})
	_ = fc
	i, key := registerHA(t, st, []string{"read", "setpoint", "lock", "structure"})
	owner := userToken(t, st, "boss", store.RoleOwner)
	me := func(h map[string]string) (int, string) {
		resp, text := callWith(t, ts, "GET", "/api/v1/me", key, "", h)
		return resp.StatusCode, text
	}
	// The key alone: the integration, its maximum within the ceiling.
	if code, text := me(nil); code != 200 || !strings.Contains(text, `"key": "casa-leone"`) || !strings.Contains(text, `"structure"`) || strings.Contains(text, `"configure"`) {
		t.Fatalf("the key alone: %d %s", code, text)
	}
	// view: reading alone.
	code, text := me(as(haAnna, LevelView))
	if code != 200 || !strings.Contains(text, `"acts_for"`) || strings.Contains(text, `"setpoint"`) || !strings.Contains(text, `"read"`) {
		t.Fatalf("view: %d %s", code, text)
	}
	setpoint := "/api/v1/cubes/KEQ0000100/rooms/2/setpoint"
	if resp, text := callWith(t, ts, "PUT", setpoint, key, `{"mode":"manual","temp":21}`, as(haAnna, LevelView)); resp.StatusCode != 403 || !strings.Contains(text, `"code": "forbidden"`) {
		t.Fatalf("a setpoint at view: %d %s", resp.StatusCode, text)
	}
	// control: the setpoint goes, and the audit names the user and the way.
	if resp, text := callWith(t, ts, "PUT", setpoint, key, `{"mode":"manual","temp":21}`, as(haAnna, LevelControl)); resp.StatusCode != 200 {
		t.Fatalf("a setpoint at control: %d %s", resp.StatusCode, text)
	}
	if rc := lastRadio(t, fc); rc.Command != cube.RFSetTemperature || rc.Target.RoomID != 2 {
		t.Fatalf("sent %+v", rc)
	}
	rows := auditRows(t, ts, owner, "?limit=1")
	if len(rows) == 0 || rows[0].User != "casa-leone."+haAnna[:8] || !strings.Contains(string(rows[0].Detail), `"via": "casa-leone"`) || !strings.Contains(string(rows[0].Detail), haAnna) {
		t.Fatalf("the audit %+v %s", rows, rows[0].Detail)
	}
	// admin adds nothing beyond the maximum and the user's role.
	if resp, _ := callWith(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/2/config", key, `{"comfort_temp":21}`, as(haAnna, LevelAdmin)); resp.StatusCode != 403 {
		t.Fatalf("configure beyond the maximum: %d", resp.StatusCode)
	}
	// The owner narrows Anna's user: the same call is refused.
	if _, err := st.UpdateUser(t.Context(), "casa-leone."+haAnna[:8], func(u *store.User) error { u.Role = store.RoleViewer; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	if resp, _ := callWith(t, ts, "PUT", setpoint, key, `{"mode":"auto"}`, as(haAnna, LevelControl)); resp.StatusCode != 403 {
		t.Fatalf("a narrowed user's setpoint: %d", resp.StatusCode)
	}
	// Another user is made at its first call; system is there already.
	if code, text := me(as(haBruno, LevelControl)); code != 200 || !strings.Contains(text, `"setpoint"`) {
		t.Fatalf("Bruno: %d %s", code, text)
	}
	if code, text := me(as("system", LevelAdmin)); code != 200 || !strings.Contains(text, `"name": "casa-leone.system"`) {
		t.Fatalf("system: %d %s", code, text)
	}

	// The refusals, each with its code.
	refused := func(name, token string, h map[string]string, status int, code string) {
		t.Helper()
		resp, text := callWith(t, ts, "GET", "/api/v1/me", token, "", h)
		if resp.StatusCode != status || !strings.Contains(text, `"code": "`+code+`"`) {
			t.Fatalf("%s: %d %s", name, resp.StatusCode, text)
		}
	}
	refused("another key", owner, as(haAnna, LevelView), 403, "not_integration")
	refused("a bad header", key, map[string]string{OnBehalfHeader: "id=x"}, 400, "bad_header")
	other := map[string]string{OnBehalfHeader: "id=" + haAnna + "; level=view; instance=fedcba9876543210fedcba9876543210"}
	refused("another installation", key, other, 403, "instance_pending")
	if got, _, _ := st.IntegrationByUUID(t.Context(), i.UUID); got.PendingInstance != "fedcba9876543210fedcba9876543210" {
		t.Fatalf("the new installation id not noted: %+v", got)
	}
	if _, err := st.UpdateUser(t.Context(), "casa-leone."+haBruno[:8], func(u *store.User) error { u.Disabled = true; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	refused("a disabled user", key, as(haBruno, LevelView), 403, "user_disabled")
	if _, err := st.SetIntegration(t.Context(), i.UUID, func(i *store.Integration) error { i.State = store.IntegrationDisabled; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	refused("a disabled integration", key, nil, 403, "integration_disabled")
	_ = s
}

var _ = httptest.NewServer
var _ fakecube.State
