// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/password"
	"go.metaworx.ch/massimo/internal/store"
)

// The rooms' and devices' writes refuse what the protocol does not take
// with the code of its kind, the English text the protocol's own (AP
// ErrorCodes, Block 5): a temperature of a configuration and of a
// setpoint, a descaling day, a vacation's end, a day of the week
// programme, the radio address 000000.
func TestPassedRoomCodes(t *testing.T) {
	t.Parallel()
	ts := startServer(t, quiet()).ts
	const base = "/api/v1/cubes/KEQ0000100"
	checkRefusals(t, ts, []refusalCase{
		{"PUT", base + "/rooms/1/config", `{"comfort_temp": 31}`, nil, 400, "temp_invalid", `{"field": "comfort_temp"}`,
			"comfort_temp: temperature 31.0: must be between 4.5 and 30.5 °C"},
		{"PUT", base + "/devices/0a010a/config", `{"eco_temp": 17.2}`, nil, 400, "temp_invalid", `{"field": "eco_temp"}`,
			"eco_temp: temperature 17.20: must be a multiple of 0.5 °C"},
		{"PUT", base + "/rooms/1/setpoint", `{"mode": "manual", "temp": 19.25}`, nil, 400, "temp_invalid", `{"field": "temp"}`,
			"temperature 19.25: must be a multiple of 0.5 °C"},
		{"PUT", base + "/devices/0a010a/setpoint", `{"mode": "manual", "temp": 31}`, nil, 400, "temp_invalid", `{"field": "temp"}`,
			"temperature 31.0: must be between 4.5 and 30.5 °C"},
		{"PUT", base + "/devices/0a010a/config", `{"decalc_day": "funday"}`, nil, 400, "config_decalc_day_invalid", "",
			`decalc_day: unknown weekday "funday"`},
		{"PUT", base + "/rooms/1/setpoint", `{"mode": "vacation", "temp": 20, "until": "soon"}`, nil, 400, "setpoint_until_invalid", "",
			`until "soon": want YYYY-MM-DD HH:MM`},
		{"PUT", base + "/rooms/1/setpoint", `{"mode": "vacation", "temp": 20, "until": "2026-12-24 18:10"}`, nil, 400, "setpoint_until_invalid", "",
			"until time 1090 min: must be a multiple of 30 minutes within a day"},
		{"PUT", base + "/rooms/1/program", `{"monday": [{"until": "12:00", "temp": 20}]}`, nil, 400, "program_day_invalid",
			`{"day": "monday", "reason": "day program: last point must end at 24:00"}`, "monday: day program: last point must end at 24:00"},
		{"PUT", base + "/devices/000000/setpoint", `{"mode": "auto"}`, nil, 400, "device_zero_address", "", "radio address 000000 is no device"},
	})
}

// The store's sentinels a page meets are refused with their status and
// code, the English text the store's (storeRefusal), wrapped or not;
// any other error is none of them.
func TestStoreRefusalCodes(t *testing.T) {
	t.Parallel()
	seen := map[error]bool{}
	for _, m := range storeCodes {
		if seen[m.sentinel] {
			t.Errorf("%v mapped twice", m.sentinel)
		}
		seen[m.sentinel] = true
		for _, err := range []error{m.sentinel, fmt.Errorf("%w: KEQ0000000", m.sentinel)} {
			status, body := refusalBody(t, storeRefusal(err, "the store's words"))
			if want := fmt.Sprintf("{\n  \"code\": %q,\n  \"error\": \"the store's words\"\n}\n", m.code.name); status != m.status || body != want {
				t.Errorf("%v: %d %s, want %d %s", err, status, body, m.status, want)
			}
		}
	}
	if err := storeRefusal(errors.New("store: disk full"), "disk full"); err != nil {
		t.Errorf("an error of no sentinel: %v", err)
	}
	// userFail without "store: ", the integrations' with it, too many
	// requests waiting with its Retry-After, a bad provider with what is
	// bad as its reason, a store opened read-only without its OPAQUE keys
	for _, c := range []struct {
		err          error
		status       int
		code, params string
		msg          string
	}{
		{userFail(fmt.Errorf("%w: anna", store.ErrUserExists)), 409, "user_exists", "", "user exists: anna"},
		{userFail(store.ErrNoOpaque), 503, "opaque_no_keys", "", strings.TrimPrefix(store.ErrNoOpaque.Error(), "store: ")},
		{userFail(store.ErrLastOwner), 409, "user_last_owner", "", strings.TrimPrefix(store.ErrLastOwner.Error(), "store: ")},
		{providerFail(fmt.Errorf("%w: a client id is needed", store.ErrBadProvider)), 400, "provider_bad", `{"reason":"a client id is needed"}`,
			"bad provider: a client id is needed"},
		{providerFail(fmt.Errorf("%w: viewer2", store.ErrNoRole)), 400, "role_not_found", "", "no such role: viewer2"},
		{providerFail(store.ErrIdentityUsed), 409, "identity_used", "", "the identity was used for a sign-in"},
		{integrationFail(store.ErrNoIntegration), 404, "integration_not_found", "", store.ErrNoIntegration.Error()},
		{integrationFail(store.ErrNotPending), 410, "integration_not_pending", "", store.ErrNotPending.Error()},
		{integrationFail(store.ErrRequestGone), 410, "integration_request_gone", "", store.ErrRequestGone.Error()},
		{integrationFail(store.ErrBadLink), 400, "integration_link_bad", "", store.ErrBadLink.Error()},
		{integrationFail(store.ErrTooManyRequests), 429, "integration_requests_waiting", "", store.ErrTooManyRequests.Error()},
		{integrationFail(store.ErrBadInstance), 400, "", "", store.ErrBadInstance.Error()},
		{storeFail(fmt.Errorf("%w: KEQ0000000", store.ErrNotFound)), 404, "cube_row_not_found", "", "store: no such cube: KEQ0000000"},
		{storeFail(fmt.Errorf("%w: KEQ0000000", store.ErrExists)), 409, "cube_row_exists", "", "store: cube exists: KEQ0000000"},
	} {
		if status, body := refusalBody(t, c.err); !isRefusal(status, body, c.status, c.code, c.params, c.msg) {
			t.Errorf("%v: %d %s, want %d %s %s %q", c.err, status, body, c.status, c.code, c.params, c.msg)
		}
	}
	var he *httpError
	if errors.As(integrationFail(store.ErrTooManyRequests), &he); he.retryAfter != 60 {
		t.Errorf("too many requests waiting: Retry-After %d", he.retryAfter)
	}
}

// A new password that fails the rule names which, with its limit, in
// the rule's own words, and no word of its context.
func TestPasswordRefusalCodes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		err          error
		code, params string
	}{
		{password.ErrTooShort, "password_too_short", `{"min":8}`},
		{password.ErrTooLong, "password_too_long", `{"max":64}`},
		{password.ErrContext, "password_has_name", ""},
	} {
		if status, body := refusalBody(t, passwordRefusal(c.err)); !isRefusal(status, body, 400, c.code, c.params, c.err.Error()) {
			t.Errorf("%v: %d %s", c.err, status, body)
		}
	}
	if status, body := refusalBody(t, passwordRefusal(errors.New("password: something new"))); !isRefusal(status, body, 400, "", "", "password: something new") {
		t.Errorf("an error of no rule: %d %s", status, body)
	}
}

// The errors passed on by the users', roles', providers', preferences'
// and settings' routes and a backup's id carry their codes over a store
// against the fake Cube.
func TestPassedStoreCodes(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, st := r.ts, r.s.opts.Store
	anna := userToken(t, st, "anna", store.RoleViewer)
	if _, err := st.AddRole(t.Context(), store.Role{Key: "keeper", Capabilities: []string{"read", "users"}}, 0); err != nil {
		t.Fatal(err)
	}
	userToken(t, st, "kim", "keeper")
	big := `{"x": "` + strings.Repeat("y", store.MaxPreferenceBytes) + `"}`
	for _, c := range []struct {
		method, path, token, body string
		status                    int
		code, params, msg         string
	}{
		{"POST", "/api/v1/users", "secret", `{"name": "bert", "role": "viewer", "password": "short"}`, 400, "password_too_short", `{"min":8}`, "password: at least 8 characters"},
		{"POST", "/api/v1/users", "secret", `{"name": "bert", "role": "viewer", "password": "` + strings.Repeat("x", 65) + `"}`, 400, "password_too_long", `{"max":64}`, ""},
		{"POST", "/api/v1/users", "secret", `{"name": "bert", "role": "viewer", "password": "bert-and-massimo"}`, 400, "password_has_name", "", ""},
		{"POST", "/api/v1/users", "secret", `{"name": "bert", "role": "nosuch"}`, 400, "role_not_found", "", ""},
		{"POST", "/api/v1/users", "secret", `{"name": "anna", "role": "viewer"}`, 409, "user_exists", "", ""},
		{"PUT", "/api/v1/users/anna", "secret", `{"expires_at": "tomorrow"}`, 400, "user_expires_at_bad", "", ""},
		{"PUT", "/api/v1/users/anna", "secret", `{"expires_at": 42}`, 400, "user_expires_at_bad", "", "expires_at: want a time such as 2026-12-24T18:00:00Z, or null"},
		{"POST", "/api/v1/roles", "secret", `{"key": "Bad Key", "capabilities": ["read"]}`, 400, "role_key_bad", "", ""},
		{"POST", "/api/v1/roles", "secret", `{"key": "keeper", "capabilities": ["read"]}`, 409, "role_exists", "", ""},
		{"DELETE", "/api/v1/roles/keeper", "secret", "", 409, "role_in_use", "", ""},
		{"POST", "/api/v1/providers", "secret", `{"key": "Cloud!", "name": "Cloud", "issuer": "https://idp.example.net", "client_id": "m", "enabled": false}`, 400, "provider_bad",
			`{"reason":"key \"cloud!\": lower case letters, digits, _ and -, starting with a letter"}`, ""},
		{"POST", "/api/v1/providers", "secret", `{"key": "cloud", "name": "Cloud", "issuer": "https://idp.example.net", "client_id": "m", "enabled": false, "auto_role": "owner"}`, 400,
			"provider_auto_role_owner", `{"role":"owner"}`, "auto_role: the role owner may not be given to users a provider creates"},
		{"POST", "/api/v1/providers", "secret", `{"key": "cloud", "name": "Cloud", "issuer": "https://idp.example.net", "client_id": "m", "enabled": false, "auto_role": "keeper"}`, 400,
			"provider_auto_role_admin", `{"capability":"users","role":"keeper"}`, ""},
		{"PUT", "/api/v1/me/preferences/notify", anna, `{"window_open": {"after_min": 100000}}`, 400, "preference_value_bad", "", ""},
		{"PUT", "/api/v1/me/preferences/ui", anna, big, 413, "preference_too_large", `{"max":` + strconv.Itoa(store.MaxPreferenceBytes) + `}`, ""},
		{"PUT", "/api/v1/settings", "secret", `{"update.auto_window": "night"}`, 400, "update_window_bad", `{"example":"` + store.DefaultUpdateWindow + `","window":"night"}`,
			`update.auto_window: "night": want a window HH:MM-HH:MM in the server's time, such as ` + store.DefaultUpdateWindow},
		{"PUT", "/api/v1/settings", "secret", `{"update.auto_window": "03:00-03:10"}`, 400, "update_window_short", `{"min":` + strconv.Itoa(minWindowMinutes) + `,"window":"03:00-03:10"}`, ""},
		{"GET", "/api/v1/cubes/KEQ0000100/backups/latest", "secret", "", 404, "backup_unknown", `{"backup":"latest","cube":"KEQ0000100"}`, `no backup "latest"`},
	} {
		status, body := callAPI(t, ts, c.method, c.path, c.token, c.body)
		got := answerRefusal(body)
		if status != c.status || got.Code != c.code || c.params != "" && string(got.Params) != c.params || c.msg != "" && got.Error != c.msg {
			t.Errorf("%s %s %.80s: %d %.300s\nwant %d %s %s %q", c.method, c.path, c.body, status, body, c.status, c.code, c.params, c.msg)
		}
		if c.code == "preference_value_bad" && !strings.Contains(string(got.Params), `"preference":"notify"`) {
			t.Errorf("%s: params %s name no preference", c.path, got.Params)
		}
	}
}

// The views of the reboot, the restart and the update say why a POST is
// refused now with its code and params beside blocked, which a POST
// then refuses with.
func TestBlockedCodes(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "secret")
	ts := startServer(t, withOptions(opts)).ts
	for _, c := range []struct{ view, post string }{
		{"/api/v1/reboot", `{"delay": 0}`}, {"/api/v1/restart", "{}"}, {"/api/v1/update", ""},
	} {
		_, text := callAPI(t, ts, "GET", c.view, "secret", "")
		var v struct {
			Blocked       string         `json:"blocked"`
			BlockedCode   string         `json:"blocked_code"`
			BlockedParams map[string]any `json:"blocked_params"`
		}
		if err := json.Unmarshal([]byte(text), &v); err != nil || v.Blocked == "" || v.BlockedCode == "" {
			t.Errorf("GET %s: %v %s", c.view, err, text)
			continue
		}
		status, body := callAPI(t, ts, "POST", c.view, "secret", c.post)
		got := refusalIn(t, body)
		var params map[string]any
		if got.Params != nil {
			params = got.Params.(map[string]any)
		}
		if status < 400 || got.Code != v.BlockedCode || got.Error != v.Blocked || !reflect.DeepEqual(params, v.BlockedParams) {
			t.Errorf("POST %s: %d %s, its view blocked %q %s %v", c.view, status, body, v.Blocked, v.BlockedCode, v.BlockedParams)
		}
	}
}

// The errors kept with a result carry their code: a step's in its
// failure and the log's line, and the event's err as setFailure keeps
// it, nothing for an error of no kind and for none.
func TestKeptFailures(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		err  error
		want failure
	}{
		{nil, failure{}},
		{fmt.Errorf("step 3 (x): %w", engine.ErrDiscarded), failure{Error: "step 3 (x): engine: command discarded by the cube", Code: "command_discarded"}},
		{fmt.Errorf("wakeup: %w", cubelan.ErrReplyTimeout), failure{Error: "wakeup: cubelan: no reply within the reply timeout", Code: "cube_no_reply"}},
		{refuse(http.StatusConflict, codeRoomNoThermostat, 2), failure{Error: "room 2 has no thermostat", Code: "room_no_thermostat", Params: map[string]any{"room": 2}}},
		{errors.New("exit status 1"), failure{Error: "exit status 1"}},
	} {
		if got := asFailure(c.err); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%v: %+v, want %+v", c.err, got, c.want)
		}
		var ev event
		ev.setFailure(c.err)
		if ev.Error != c.want.Error || ev.Code != c.want.Code || !reflect.DeepEqual(ev.Params, c.want.Params) {
			t.Errorf("event of %v: %q %q %v", c.err, ev.Error, ev.Code, ev.Params)
		}
	}
	// passed keeps a refusal, else gives the error's text its code and
	// reason; saying puts a field's name before a refusal's words
	if err := passed(400, refuse(409, codeRoomNoThermostat, 2), codeCommandFailed); err.Error() != "room 2 has no thermostat" {
		t.Errorf("passed over a refusal: %v", err)
	}
	if status, body := refusalBody(t, passed(400, errors.New("M: too many rooms or devices"), codeMetadataNotWritten)); !isRefusal(status, body, 400,
		"metadata_not_written", `{"reason":"M: too many rooms or devices"}`, "M: too many rooms or devices") {
		t.Errorf("passed: %d %s", status, body)
	}
	if status, body := refusalBody(t, saying("update.auto_window: ", refuse(400, codeUpdateWindowShort, "03:00-03:10", 30))); !isRefusal(status, body, 400,
		"update_window_short", `{"min":30,"window":"03:00-03:10"}`, `update.auto_window: "03:00-03:10": the window is shorter than 30 minutes`) {
		t.Errorf("saying: %d %s", status, body)
	}
}

// A command the Cube discards keeps its code across a restart of the
// server, its store opened again: its row, the answer's code, and its
// step, the result's (GET /api/v1/commands/{id}), the replay of its
// Idempotency-Key and the log's line of its step; a line kept before the
// codes keeps its English error alone.
func TestKeptErrorsAcrossARestart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	opts := storeOptions(t, dir, "secret")
	st := opts.Store
	first := startServer(t, withOptions(opts), withFake(func(fc *fakecube.Server) { fc.Discard = true }))
	const path, body = "/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint", `{"mode": "manual", "temp": 20}`
	send := func(ts string) (int, string, string) {
		req, _ := http.NewRequest("PUT", ts+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set("Idempotency-Key", "kept-1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, resp.Header.Get("Idempotent-Replayed"), string(b)
	}
	// the room's members hold no state on the fake Cube, which names them
	status, _, answer := send(first.ts.URL)
	if status != http.StatusBadGateway || answerCode(answer) != "command_discarded_no_state" {
		t.Fatalf("the setpoint: %d %s", status, answer)
	}
	cubeRow, _, _ := st.Cube(t.Context(), "KEQ0000100")
	old, _ := json.Marshal(map[string]any{"kind": "restore_done", "level": "warn", "error": "step 1 (names): engine: command discarded by the cube",
		"time": time.Now().UTC().Format(time.RFC3339)})
	if err := st.InsertEvent(t.Context(), store.Event{CubeID: cubeRow.ID, Time: time.Now(), Kind: "restore_done", Source: "system", Level: store.LevelWarn, Data: old}); err != nil {
		t.Fatal(err)
	}
	first.stop()
	st.Close()

	second := startServer(t, withOptions(storeOptions(t, dir, "secret")))
	ts := second.ts
	status, replayed, again := send(ts.URL)
	if status != http.StatusBadGateway || replayed != "true" || again != answer {
		t.Fatalf("the replay after the restart: %d %q %s\nwant %s", status, replayed, again, answer)
	}
	var lines []event
	getJSON(t, ts, "secret", "/api/v1/cubes/KEQ0000100/log?level=warn&limit=1000", &lines)
	var step, before *event
	for i := range lines {
		switch {
		case lines[i].Kind == "command" && lines[i].Error != "":
			step = &lines[i]
		case lines[i].Kind == "restore_done":
			before = &lines[i]
		}
	}
	if step == nil || step.Code != "command_discarded" || step.Command == 0 {
		t.Fatalf("the step's line after the restart: %+v of %d lines", step, len(lines))
	}
	if before == nil || before.Error == "" || before.Code != "" || before.Params != nil {
		t.Fatalf("the line kept before the codes: %+v", before)
	}
	_, text := callAPI(t, ts, "GET", "/api/v1/commands/"+strconv.FormatInt(step.Command, 10), "secret", "")
	var c struct {
		State  string          `json:"state"`
		Result json.RawMessage `json:"result"`
		Steps  []struct {
			State  string          `json:"state"`
			Result json.RawMessage `json:"result"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(text), &c); err != nil || len(c.Steps) == 0 {
		t.Fatalf("the command: %v %s", err, text)
	}
	if answerCode(string(c.Result)) != "command_discarded_no_state" || answerCode(string(c.Steps[len(c.Steps)-1].Result)) != "command_discarded" {
		t.Fatalf("the command's row and step after the restart: %s", text)
	}
}
