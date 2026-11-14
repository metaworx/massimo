// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The log's unsettled warnings and errors (AP LogAlerts v1.0, Block 1):
// each kind settled by its counterpart for the same Cube, a hold by its
// end, one without a counterpart left; a device's flag while it is set.

import (
	"encoding/base64"
	"slices"
	"strconv"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
)

// Every kind of settleRules, and every kind that settles one, is a kind
// the log keeps.
func TestSettleRulesKnowTheirKinds(t *testing.T) {
	for kind, rule := range settleRules {
		if !slices.Contains(eventKinds, kind) {
			t.Errorf("%s: not a kind of the log", kind)
		}
		for _, by := range rule.by {
			if !slices.Contains(eventKinds, by) {
				t.Errorf("%s: settled by %s, not a kind of the log", kind, by)
			}
		}
	}
}

// A warning or an error is settled by a later counterpart of the same
// Cube below warn; a hold by its end; a kind without a counterpart only
// by a mark.
func TestUnsettled(t *testing.T) {
	now := time.Now()
	ev := func(id int64, kind, level, cube string, summary map[string]any) event {
		return event{ID: id, Kind: kind, Level: level, CubeSerial: cube, Summary: summary, Time: now.Add(time.Duration(id-100) * time.Second)}
	}
	cmd := func(state string) map[string]any { return map[string]any{"state": state, "key": "temp:0a0101"} }
	lines := []event{
		ev(1, "disconnected", "warn", "KEQ0000100", nil),
		ev(3, "disconnected", "warn", "KEQ0000100", nil),
		ev(5, "command", "warn", "KEQ0000100", cmd("retried")),
		ev(7, "command", "warn", "KEQ0000100", map[string]any{"state": "failed", "key": "mode:0a0102"}),
		ev(8, "hold", "warn", "KEQ0000100", map[string]any{"reason": "duty_cycle", "until": now.Add(-time.Minute).UTC().Format(time.RFC3339)}),
		ev(9, "hold", "warn", "KEQ0000100", map[string]any{"reason": "duty_cycle", "until": now.Add(time.Hour).UTC().Format(time.RFC3339)}),
		ev(10, "restart", "error", "", nil),
		ev(12, "backup", "warn", "KEQ0000100", nil),
		ev(13, "backup", "warn", "KEQ0000100", nil),
		ev(14, "proxy_delete_refused", "warn", "KEQ0000100", nil),
	}
	lines[2].Device, lines[3].Device = "0a0101", "0a0102"
	ack := ev(6, "command", "info", "KEQ0000100", cmd("acknowledged"))
	ack.Device = "0a0101"
	settling := []event{
		ev(2, "connected", "info", "KEQ0000100", nil),
		ev(4, "connected", "info", "KEQ0000200", nil), // another Cube's
		ack,
		ev(11, "server_started", "info", "", nil),
	}
	var got []int64
	for _, a := range unsettled(lines, settling, now) {
		got = append(got, a.ID)
	}
	if want := []int64{3, 7, 9, 12, 13, 14}; !slices.Equal(got, want) {
		t.Fatalf("unsettled %v, want %v", got, want)
	}
}

// GET /api/v1/log/alerts: a disconnect settled by the next connect, an
// error of the server's left, a device's transmit error while it is set,
// at the line that set it; after leaves out the older ones.
func TestLogAlertsRoute(t *testing.T) {
	fc, ts, _, st := startStored(t, Options{PollInterval: 100 * time.Millisecond})
	ctx := t.Context()
	row, _, _ := st.Cube(ctx, "KEQ0000100")
	add := func(ev event, cubeID int64) {
		t.Helper()
		ev.settle()
		if err := st.InsertEvent(ctx, storedEvent(cubeID, ev)); err != nil {
			t.Fatal(err)
		}
	}
	add(event{Kind: "disconnected", Time: time.Now()}, row.ID)
	add(event{Kind: "connected", Time: time.Now(), Summary: map[string]any{"cause": "loss"}}, row.ID)
	add(event{Kind: "restart", Source: "system", Level: "error", Error: "no helper", Time: time.Now()}, 0)
	var out logAlertsView
	eventually(t, "the restart's error", func() bool {
		return getJSON(t, ts, "", "/api/v1/log/alerts", &out) == 200 && len(out.Alerts) == 1
	})
	if a := out.Alerts[0]; a.Kind != "restart" || a.Level != "error" || out.Level != "error" || a.CubeSerial != "" {
		t.Fatalf("the alerts: %+v", out)
	}
	restart := out.Alerts[0].ID

	greeting, _ := fakecube.LoadGreeting(fixture)
	last := greeting[len(greeting)-1]
	raw, _ := base64.StdEncoding.DecodeString(last.Data)
	flagged := append([]byte(nil), raw...)
	copy(flagged, []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12 | 0x08, 0x01, 30, 42, 0, 0, 0}) // 0a010d: a transmit error
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(flagged)})
	eventually(t, "the transmit error", func() bool {
		return getJSON(t, ts, "", "/api/v1/log/alerts", &out) == 200 && len(out.Alerts) == 2
	})
	a := out.Alerts[0]
	if a.Kind != "state" || a.Flag != "error" || a.Device != "0a010d" || a.Level != "warn" || a.CubeSerial != "KEQ0000100" || a.ID <= restart || out.Level != "error" {
		t.Fatalf("the transmit error: %+v", out)
	}
	getJSON(t, ts, "", "/api/v1/log/alerts?after="+strconv.FormatInt(restart, 10), &out)
	if len(out.Alerts) != 1 || out.Alerts[0].Flag != "error" || out.Level != "warn" {
		t.Fatalf("after the restart's line: %+v", out)
	}

	fc.SetDeviceList(last)
	eventually(t, "the error cleared", func() bool {
		return getJSON(t, ts, "", "/api/v1/log/alerts", &out) == 200 && len(out.Alerts) == 1
	})
	if code, _ := callAPI(t, ts, "GET", "/api/v1/log/alerts?after=x", "", ""); code != 400 {
		t.Fatalf("a bad after: %d", code)
	}
}

// A person's marks (Block 2): all hides the alerts up to its id, a
// line's mark that line alone; another person's alerts stay; a document
// that is not marks is refused.
func TestLogAlertsSeenMarks(t *testing.T) {
	_, ts, _, st := startStored(t, Options{})
	ctx := t.Context()
	anna := userToken(t, st, "anna", store.RoleOwner)
	ben := userToken(t, st, "ben", store.RoleOwner)
	var ids []int64
	for i := 0; i < 3; i++ {
		ev := event{Kind: "restart", Source: "system", Level: "error", Time: time.Now()}
		ev.settle()
		if err := st.InsertEvent(ctx, storedEvent(0, ev)); err != nil {
			t.Fatal(err)
		}
	}
	alerts := func(token string) []int64 {
		t.Helper()
		var out logAlertsView
		if code := getJSON(t, ts, token, "/api/v1/log/alerts", &out); code != 200 {
			t.Fatalf("GET: %d", code)
		}
		var got []int64
		for _, a := range out.Alerts {
			got = append(got, a.ID)
		}
		return got
	}
	ids = alerts(anna)
	if len(ids) != 3 {
		t.Fatalf("the alerts: %v", ids)
	}
	newest, middle, oldest := ids[0], ids[1], ids[2]
	put := func(body string) int {
		t.Helper()
		code, _ := callAPI(t, ts, "PUT", "/api/v1/me/preferences/log.seen", anna, body)
		return code
	}
	if code := put(`{"all": ` + strconv.FormatInt(oldest, 10) + `, "lines": [` + strconv.FormatInt(newest, 10) + `]}`); code != 200 {
		t.Fatalf("PUT: %d", code)
	}
	if got := alerts(anna); !slices.Equal(got, []int64{middle}) {
		t.Fatalf("anna's alerts after her marks: %v, want %d", got, middle)
	}
	if got := alerts(ben); len(got) != 3 {
		t.Fatalf("ben's alerts: %v", got)
	}
	for _, bad := range []string{`{"all": -1}`, `{"all": 1, "lines": [0]}`, `{"seen": 1}`, `[1]`} {
		if code := put(bad); code != 400 {
			t.Errorf("PUT %s: %d", bad, code)
		}
	}
}
