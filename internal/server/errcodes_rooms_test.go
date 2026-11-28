// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// answered is an error answer as a client reads it: its English text,
// its code and its params, nil where it has none.
type answered struct {
	Error  string `json:"error"`
	Code   string `json:"code"`
	Params any    `json:"params"`
}

// refusalIn reads an error answer's body.
func refusalIn(t *testing.T, body string) answered {
	t.Helper()
	var r answered
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("no JSON answer: %v\n%s", err, body)
	}
	return r
}

// paramsOf decodes params written as JSON, nil for "".
func paramsOf(t *testing.T, params string) any {
	t.Helper()
	if params == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(params), &v); err != nil {
		t.Fatalf("params %s: %v", params, err)
	}
	return v
}

// refusalCase is a request a refusal answers: its status, its code, its
// params as JSON ("" for none) and, where given, its English text.
type refusalCase struct {
	method, path, body string
	header             map[string]string
	status             int
	code, params, text string
}

// checkRefusals sends each case and checks its answer.
func checkRefusals(t *testing.T, ts *httptest.Server, cases []refusalCase) {
	t.Helper()
	for _, c := range cases {
		status, body := sendWith(t, ts, c.method, c.path, c.body, c.header)
		got := refusalIn(t, body)
		if status != c.status || got.Code != c.code || !reflect.DeepEqual(got.Params, paramsOf(t, c.params)) || c.text != "" && got.Error != c.text {
			t.Errorf("%s %s %s: %d %s\nwant %d %s %s %q", c.method, c.path, c.body, status, body, c.status, c.code, c.params, c.text)
		}
	}
}

// checkNothing checks a repair with nothing to send: its preview says
// why with code and params, and its run is refused with 409 and the same.
func checkNothing(t *testing.T, ts *httptest.Server, path, code, params string) {
	t.Helper()
	status, p, body := previewOf(t, ts, path)
	want := paramsOf(t, params)
	var got any
	if p.Params != nil {
		got = paramsOf(t, mustJSON(t, p.Params))
	}
	if status != 200 || p.Nothing == "" || p.Code != code || !reflect.DeepEqual(got, want) {
		t.Errorf("GET %s: %d %s\nwant nothing with %s %s", path, status, body, code, params)
		return
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	status, body = sendWith(t, ts, "POST", path+sep+"plan=0", "", nil)
	if r := refusalIn(t, body); status != http.StatusConflict || r.Code != code || r.Error != p.Nothing || !reflect.DeepEqual(r.Params, want) {
		t.Errorf("POST %s: %d %s\nwant 409 %s %s %q", path, status, body, code, params, p.Nothing)
	}
}

// mustJSON is v as JSON.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The refusals of rooms and devices, their configuration, the week
// programme's tag, the virtual wall thermostats and the pairing answer
// with their codes and params (AP ErrorCodes, Block 2), over the
// recorded greeting: rooms 1 to 6, one radiator thermostat each, room
// 1's 0a010a and room 2's 0a0109; room 2's virtual wall thermostat has
// the address 000002.
func TestRoomRefusalCodes(t *testing.T) {
	t.Parallel()
	ts := startServer(t, quiet()).ts
	const base = "/api/v1/cubes/KEQ0000100"
	long := `{"name": "` + strings.Repeat("x", maxNameBytes+1) + `"}`
	stale := map[string]string{"If-Match": `"stale"`}
	checkRefusals(t, ts, []refusalCase{
		{"PUT", base + "/rooms/1", `{"name": "  "}`, nil, 400, "name_empty", "", "name must not be empty"},
		{"PUT", base + "/rooms/1", long, nil, 400, "name_too_long", `{"max": 32}`, "name must be valid text of at most 32 bytes"},
		{"PUT", base + "/devices/0a010a", long, nil, 400, "name_too_long", `{"max": 32}`, ""},
		{"DELETE", base + "/rooms/1", "", nil, 409, "room_not_empty", `{"room": 1, "device": "0a010a"}`, "room 1 still has device 0a010a; move it first"},
		{"PUT", base + "/rooms/1/setpoint", `{"mode": "eco"}`, nil, 400, "setpoint_mode_invalid", "", "mode must be auto, manual, vacation or boost"},
		{"PUT", base + "/devices/0a010a/config", `{"temp_offset": 4}`, nil, 400, "config_temp_offset_invalid", "", "temp_offset: must be a multiple of 0.5 between -3.5 and 3.5"},
		{"PUT", base + "/devices/0a010a/config", `{"window_open_minutes": 7}`, nil, 400, "config_window_minutes_invalid", "", "window_open_minutes: must be 0..60 in steps of 5"},
		{"PUT", base + "/rooms/1/config", `{"boost_minutes": 35}`, nil, 400, "config_boost_minutes_invalid", "", "boost_minutes: must be 0..30 in steps of 5, or 60"},
		{"PUT", base + "/rooms/1/config", `{"boost_valve_pct": 3}`, nil, 400, "config_boost_valve_invalid", "", "boost_valve_pct: must be 0..100 in steps of 5"},
		{"PUT", base + "/rooms/1/config", `{"decalc_hour": 24}`, nil, 400, "config_decalc_hour_invalid", "", "decalc_hour: must be 0..23"},
		{"PUT", base + "/rooms/1/config", `{"valve_offset_pct": 101}`, nil, 400, "config_valve_pct_invalid", `{"field": "valve_offset_pct"}`, "valve_offset_pct: must be 0..100"},
		{"PUT", base + "/rooms/1/program", `{"monday": [{"until": "24:00", "temp": 20}]}`, stale, 412, "program_changed", "",
			"the week programme changed since it was read (or a write to it is still on its way): read it again, then write"},
		{"PUT", base + "/devices/0a010a/program", `{"monday": [{"until": "24:00", "temp": 20}]}`, stale, 412, "program_changed", "", ""},
		{"PUT", base + "/devices/room-1/lock", `{"locked": true}`, nil, 409, "vwt_lock_no_wall_thermostat", `{"room": 1}`,
			"room 1 has no wall thermostat whose keys could be locked; lock its radiator thermostats one by one"},
		{"PUT", base + "/devices/room-1", `{"name": "Wand"}`, nil, 409, "vwt_rename_with_room", `{"room": 1}`,
			"the virtual wall thermostat of room 1 goes with its room: rename the room (PUT …/rooms/1)"},
		{"DELETE", base + "/devices/room-1", "", nil, 409, "vwt_remove_with_room", `{"room": 1}`,
			"the virtual wall thermostat of room 1 goes with its room: it is removed with the room"},
		{"PUT", base + "/devices/room-1/config", `{"temp_method": "max"}`, nil, 400, "vwt_temp_method_invalid", "", "temp_method: median or mean"},
		{"PUT", base + "/devices/room-1/config", `{"rf_address": "0a01"}`, nil, 400, "vwt_address_invalid", "", `rf_address: six hex digits, or "" for the room number`},
		{"PUT", base + "/devices/room-1/config", `{"rf_address": "0A0109"}`, nil, 409, "vwt_address_real", `{"device": "0a0109"}`, "rf_address 0a0109 is a real device's"},
		{"PUT", base + "/devices/room-1/config", `{"rf_address": "000002"}`, nil, 409, "vwt_address_taken", `{"address": "000002", "room": 2}`,
			"rf_address 000002 is the virtual wall thermostat's of room 2"},
		{"GET", base + "/devices/room-9", "", nil, 404, "room_unknown", `{"room": 9}`, "no room 9"},
		{"GET", base + "/devices/room-009", "", nil, 404, "room_unknown", `{"room": 9}`, "no room 009"},
		{"POST", base + "/pairing", `{"seconds": 0}`, nil, 400, "pairing_seconds_invalid", `{"max": 255}`, "seconds must be 1..255"},
	})
	// The tag refused carries the current one beside the code.
	if _, body := sendWith(t, ts, "PUT", base+"/rooms/1/program", `{"monday": [{"until": "24:00", "temp": 20}]}`, stale); !strings.Contains(body, `"etag": "`) {
		t.Errorf("412 without the current tag: %s", body)
	}
}

// The repairs with nothing to send say why with a code and its params,
// in the preview and the run's 409 alike, over the recorded greeting as
// each case changes it, and over the repairs' own (repairing).
func TestRepairRefusalCodes(t *testing.T) {
	t.Parallel()
	const base = "/api/v1/cubes/KEQ0000100"
	type nothing struct{ path, code, params string }
	for _, tc := range []struct {
		name   string
		start  []startOpt
		change func(*testing.T, []cube.Message)
		cases  []nothing
	}{
		{"recorded", nil, nil, []nothing{
			{base + "/repair/reset", "repair_none_flagged", ""},
			{base + "/rooms/1/repair/reset", "repair_room_none_flagged", `{"room": 1}`},
			{base + "/rooms/1/repair/master", "repair_master_valid", `{"room": 1, "device": "0a010a"}`},
			{base + "/rooms/1/repair/relink", "repair_link_too_few", `{"room": 1}`},
			{base + "/devices/0a010a/repair/place", "repair_placed_already", `{"device": "0a010a", "room": 1}`},
			{base + "/devices/0a010a/repair/list", "repair_listed_already", `{"device": "0a010a", "room": 1}`},
		}},
		{"room 2's thermostat in no room", nil, func(t *testing.T, g []cube.Message) { withConfigRoom(t, g, 0x0a0109, 0) }, []nothing{
			{base + "/rooms/2/repair/master", "repair_master_none", `{"room": 2}`},
			{base + "/rooms/2/repair/relink", "repair_room_disagrees", `{"room": 2, "devices": ["0a0109"]}`},
			{base + "/devices/0a0109/repair/config", "repair_device_no_room", `{"device": "0a0109"}`},
			{base + "/devices/0a0109/repair/list", "repair_device_no_room", `{"device": "0a0109"}`},
		}},
		{"room 2's thermostat in room 1", nil, func(t *testing.T, g []cube.Message) { withConfigRoom(t, g, 0x0a0109, 1) }, []nothing{
			{base + "/devices/0a0109/repair/config", "repair_rooms_differ", `{"device": "0a0109", "room": 1, "list_room": 2}`},
		}},
		{"room 2's thermostat in the hidden room", nil, func(t *testing.T, g []cube.Message) { withConfigRoom(t, g, 0x0a0109, cube.HiddenRoom) }, []nothing{
			{base + "/devices/0a0109/repair/place", "repair_hidden_room", `{"device": "0a0109", "room": 255}`},
		}},
		{"repairing", []startOpt{repairing()}, nil, []nothing{
			{base + "/devices/0a0b0c/repair/place", "repair_device_unlisted", `{"device": "0a0b0c"}`},
			{base + "/devices/0a010a/repair/config", "repair_config_same", `{"device": "0a010a", "room": 1, "thermostats": 2}`},
		}},
		{"listed in no room", []startOpt{repairing()}, func(t *testing.T, g []cube.Message) {
			listIn(t, g, copyNew, cube.DeviceRadiatorThermostat, "TEQ0000001", 0)
		}, []nothing{
			{base + "/devices/0a0b0c/repair/place", "repair_list_no_room", `{"device": "0a0b0c"}`},
		}},
		{"a second wall thermostat", []startOpt{repairing()}, func(t *testing.T, g []cube.Message) {
			// The list has copyWall and another wall thermostat in room 1,
			// the Cube copyWall in room 6.
			withConfigRoom(t, g, copyWall, 6)
			listIn(t, g, 0x0a0d0f, cube.DeviceWallThermostat, "NEQ0000005", 1)
		}, []nothing{
			{base + "/devices/0c0d0e/repair/place", "room_wall_thermostat_taken", `{"room": 1, "device": "0a0d0f"}`},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := append([]startOpt{quiet()}, tc.start...)
			if tc.change != nil {
				opts = append(opts, withFake(func(fc *fakecube.Server) { tc.change(t, fc.Greeting) }))
			}
			ts := startServer(t, opts...).ts
			for _, c := range tc.cases {
				checkNothing(t, ts, c.path, c.code, c.params)
			}
		})
	}
}

// A run refused or stopped answers with its code: a plan that is not the
// preview's, and a step the Cube discarded, the result's refusal its
// reason beside the steps done.
func TestRepairRunCodes(t *testing.T) {
	t.Parallel()
	const path = "/api/v1/cubes/KEQ0000100/rooms/1/repair/relink"
	r := startServer(t, repairing(), quiet(), withFake(func(fc *fakecube.Server) {
		withContactInRoom1(t, fc.Greeting)
		fc.DiscardIf = func(m cube.Message) bool { return m.Type == 's' }
	}))
	status, body := sendWith(t, r.ts, "POST", path+"?plan=0", "", nil)
	if got := refusalIn(t, body); status != http.StatusConflict || got.Code != "repair_changed" || got.Params != nil ||
		!strings.HasPrefix(got.Error, "the repair changed since its preview (plan 0, now ") {
		t.Errorf("another plan: %d %s", status, body)
	}
	code, p, body := previewOf(t, r.ts, path)
	if code != 200 || p.Plan == "" {
		t.Fatalf("preview: %d %s", code, body)
	}
	status, body = sendWith(t, r.ts, "POST", path+"?plan="+p.Plan, "", nil)
	got := refusalIn(t, body)
	want := paramsOf(t, `{"step": 1, "steps": 3, "reason": {"code": "command_discarded", "error": "engine: command discarded by the cube"}}`)
	if status != http.StatusBadGateway || got.Code != "repair_stopped" || !reflect.DeepEqual(got.Params, want) ||
		got.Error != "relink: stopped at step 1 of 3: engine: command discarded by the cube" || !strings.Contains(body, `"steps": [`) {
		t.Errorf("a discarded step: %d %s", status, body)
	}
}

// A run's budget refusals carry their figures (repairRefused): the duty
// cycle with the repair's cost at the queue's hold, and slots too few.
func TestRepairBudgetCodes(t *testing.T) {
	t.Parallel()
	rg := startServer(t, quiet())
	now := time.Now()
	for _, tc := range []struct {
		v      repairPlan
		code   string
		params string
	}{
		{repairPlan{Radiograms: 2, DutyCostPct: 2.5, Budget: cubelan.Budget{DutyCyclePct: 93, FreeMemorySlots: 100, UpdatedAt: now}},
			"repair_duty_cycle", `{"duty": 93, "cost": 2.5, "max": 95}`},
		{repairPlan{Radiograms: 2, Held: 4, Budget: cubelan.Budget{DutyCyclePct: 10, FreeMemorySlots: 8, UpdatedAt: now}},
			"repair_memory_slots", `{"free": 8, "held": 4, "min": 5}`},
	} {
		var he *httpError
		if err := rg.s.repairRefused(tc.v); !errors.As(err, &he) {
			t.Fatalf("%s: %v", tc.code, err)
		}
		got := refusalIn(t, mustJSON(t, he.body()))
		if he.status != http.StatusConflict || got.Code != tc.code || !reflect.DeepEqual(got.Params, paramsOf(t, tc.params)) {
			t.Errorf("%s: %d %+v", tc.code, he.status, got)
		}
	}
	if err := rg.s.repairRefused(repairPlan{Radiograms: 2, Budget: cubelan.Budget{DutyCyclePct: 10, FreeMemorySlots: 100, UpdatedAt: now}}); err != nil {
		t.Errorf("within the budget: %v", err)
	}
}

// The Cube's structure lock held by another change past the request's
// time refuses with structure_change_running.
func TestStructureBusyCode(t *testing.T) {
	t.Parallel()
	rg := startServer(t, quiet())
	unlock, err := rg.rt.lockStructure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = rg.rt.lockStructure(ctx)
	var he *httpError
	if !errors.As(err, &he) || he.status != http.StatusServiceUnavailable || he.code != codeStructureBusy ||
		he.msg != "another change of the cube's rooms and devices is still running" {
		t.Fatalf("lock held: %v", err)
	}
}

// With the Cube away, what needs its connection is refused with 503 and
// cube_not_connected: a pairing, a device's removal and a repair's run.
func TestNotConnectedCodes(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet(), withFake(func(fc *fakecube.Server) { withErrorFlag(t, fc.Greeting, 0x0a010a, cube.RFRemoveLink) }))
	const base = "/api/v1/cubes/KEQ0000100"
	code, p, body := previewOf(t, r.ts, base+"/rooms/1/repair/reset")
	if code != 200 || p.Plan == "" {
		t.Fatalf("preview: %d %s", code, body)
	}
	r.fc.Close()
	testkit.Eventually(t, "the Cube away", func() bool { return !r.rt.client.Connected() }, awayWait)
	checkRefusals(t, r.ts, []refusalCase{
		{"POST", base + "/pairing", `{"seconds": 30}`, nil, 503, "cube_not_connected", "", "cube not connected"},
		{"DELETE", base + "/devices/0a0109", "", nil, 503, "cube_not_connected", "", "cube not connected"},
		{"POST", base + "/rooms/1/repair/reset?plan=" + p.Plan, "", nil, 503, "cube_not_connected", "", "cube not connected"},
	})
}
