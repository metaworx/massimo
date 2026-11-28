// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The log's unsettled warnings and errors (AP LogAlerts v1.0, Block 1):
// each kind settled by its counterpart for the same Cube, a hold by its
// end, one without a counterpart left; a device's flag while it is set.

import (
	"encoding/base64"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// Every kind of settleRules, and every kind that settles one, is a kind
// the log keeps.
func TestSettleRulesKnowTheirKinds(t *testing.T) {
	t.Parallel()
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
// Cube below warn, or at any level where its rule says any; a hold by
// its end; a kind without a counterpart only by a mark.
func TestUnsettled(t *testing.T) {
	t.Parallel()
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
		// a value the next start refuses: settled by the file read again,
		// a warning too, the newest the alert, or by a start
		ev(15, "settings_read", "warn", "", nil),
		ev(17, "settings_read", "warn", "", nil),
		ev(20, "settings_read", "warn", "", nil),
		ev(21, "settings_read", "warn", "", nil),
	}
	lines[2].Device, lines[3].Device = "0a0101", "0a0102"
	ack := ev(6, "command", "info", "KEQ0000100", cmd("acknowledged"))
	ack.Device = "0a0101"
	settling := []event{
		ev(2, "connected", "info", "KEQ0000100", nil),
		ev(4, "connected", "info", "KEQ0000200", nil), // another Cube's
		ack,
		ev(11, "server_started", "info", "", nil),
		ev(16, "settings_read", "info", "", nil),
		ev(17, "settings_read", "warn", "", nil),
		ev(19, "server_started", "info", "", nil),
		ev(20, "settings_read", "warn", "", nil),
		ev(21, "settings_read", "warn", "", nil),
	}
	var got []int64
	for _, a := range unsettled(lines, settling, now) {
		got = append(got, a.ID)
	}
	if want := []int64{3, 7, 9, 12, 13, 14, 21}; !slices.Equal(got, want) {
		t.Fatalf("unsettled %v, want %v", got, want)
	}
}

// A battery's low level is settled by a later battery event of the same
// device at ok, not by another device's, nor by a weak one.
func TestBatterySettles(t *testing.T) {
	t.Parallel()
	now := time.Now()
	ev := func(id int64, device, level, to string) event {
		return event{ID: id, Kind: "battery", Level: level, CubeSerial: "KEQ0000100", Device: device, Summary: map[string]any{"level": to},
			Time: now.Add(time.Duration(id-100) * time.Second)}
	}
	lines := []event{ev(1, "0a0101", "warn", "low"), ev(3, "0a0102", "warn", "low"), ev(6, "0a0101", "warn", "low")}
	settling := []event{
		ev(2, "0a0101", "info", "ok"),
		ev(4, "0a0103", "info", "ok"),   // another device's
		ev(5, "0a0102", "info", "weak"), // not ok
	}
	var got []int64
	for _, a := range unsettled(lines, settling, now) {
		got = append(got, a.ID)
	}
	if want := []int64{3, 6}; !slices.Equal(got, want) {
		t.Fatalf("unsettled %v, want %v", got, want)
	}
}

// A wall thermostat's mode reset is settled by a later agreement of its
// room, also as the store reads the room back, not by another room's,
// nor by a room check's other outcome; a line without a room only by a
// mark.
func TestWallResetSettles(t *testing.T) {
	t.Parallel()
	now := time.Now()
	ev := func(id int64, kind, level string, summary map[string]any) event {
		return event{ID: id, Kind: kind, Level: level, CubeSerial: "KEQ0000100", Summary: summary, Time: now.Add(time.Duration(id-100) * time.Second)}
	}
	reset := func(id int64, room any) event {
		summary := map[string]any{"mode": "auto", "was_mode": "manual"}
		if room != nil {
			summary["room"] = room
		}
		e := ev(id, "wall_reset", "warn", summary)
		e.Device = "0a0201"
		return e
	}
	agreed := func(id int64, room any, outcome string) event {
		return ev(id, "disagree", "info", map[string]any{"room": room, "outcome": outcome})
	}
	lines := []event{reset(1, uint8(1)), reset(3, uint8(2)), reset(5, nil), reset(7, uint8(1))}
	settling := []event{
		agreed(2, uint8(1), "agree"),
		agreed(4, float64(3), "agree"),      // another room's
		agreed(6, float64(2), "not_resent"), // not agreeing
		agreed(8, float64(1), "agree"),      // as the store reads it back
	}
	var got []int64
	for _, a := range unsettled(lines, settling, now) {
		got = append(got, a.ID)
	}
	if want := []int64{3, 5}; !slices.Equal(got, want) {
		t.Fatalf("unsettled %v, want %v", got, want)
	}
}

// GET /api/v1/log/alerts over a device's battery: its level's event at
// low is an alert, at warn; another device's level back at ok leaves it,
// the device's own settles it; a weak level's event, at info, is none.
func TestLogAlertsBatteryLevel(t *testing.T) {
	t.Parallel()
	r := startServer(t, stored(), withOptions(Options{PollInterval: 100 * time.Millisecond}))
	ts, st := r.ts, r.st
	ctx := t.Context()
	row, _, _ := st.Cube(ctx, "KEQ0000100")
	add := func(device, level, to string) {
		t.Helper()
		ev := event{Kind: "battery", Source: "device", Device: device, Level: level, Time: time.Now(), Summary: map[string]any{"level": to}}
		ev.settle()
		if err := st.InsertEvent(ctx, storedEvent(row.ID, ev)); err != nil {
			t.Fatal(err)
		}
	}
	alerts := func() []logAlert {
		var out logAlertsView
		if code := getJSON(t, ts, "", "/api/v1/log/alerts", &out); code != 200 {
			t.Fatalf("GET the alerts: %d", code)
		}
		return out.Alerts
	}
	add("0a010d", "info", "weak")
	add("0a010d", "warn", "low")
	got := alerts()
	if len(got) != 1 || got[0].Kind != "battery" || got[0].Device != "0a010d" || got[0].Level != "warn" {
		t.Fatalf("the low level: %+v, want its battery event alone", got)
	}
	add("0a0108", "info", "ok")
	if got := alerts(); len(got) != 1 {
		t.Fatalf("another device ok: %+v, want the low level still", got)
	}
	add("0a010d", "info", "ok")
	if got := alerts(); len(got) != 0 {
		t.Fatalf("the device ok again: %+v, want none", got)
	}
}

// settings_read's line holds all that its read found, so that a later
// one settles it, a warning too, and the newest is the alert while it
// warns; a start settles it, and the start's own notes, a warning after
// its server_started, are the alert (AP StartConfiguration, D11-A).
// Another kind's lines settle it not, at any level: any keeps to by.
func TestSettingsReadSettles(t *testing.T) {
	t.Parallel()
	now := time.Now()
	ev := func(id int64, kind, level string) event {
		return event{ID: id, Kind: kind, Level: level, Source: "system", Time: now.Add(time.Duration(id-100) * time.Second)}
	}
	warned := func(id int64) event { return ev(id, "settings_read", "warn") }
	read := func(id int64) event { return ev(id, "settings_read", "info") }
	started := func(id int64) event { return ev(id, "server_started", "info") }
	rotated := func(id int64, level string) event { return ev(id, "log_rotated", level) }
	for _, c := range []struct {
		name string
		log  []event // oldest first
		want []int64
	}{
		{"two warnings in a row", []event{warned(1), warned(2)}, []int64{2}},
		{"three warnings in a row", []event{warned(1), warned(2), warned(3)}, []int64{3}},
		{"a warning, then an info line", []event{warned(1), read(2)}, nil},
		{"a warning, then a start", []event{warned(1), started(2)}, nil},
		{"a warning, then a start that writes its notes", []event{warned(1), started(2), warned(3)}, []int64{3}},
		{"the start's notes, then a read that warns", []event{started(1), warned(2), warned(3)}, []int64{3}},
		{"the start's notes, then a read without a finding", []event{started(1), warned(2), read(3)}, nil},
		{"an info line, then a warning", []event{read(1), warned(2)}, []int64{2}},
		{"a warning, then another kind's warning and its counterpart", []event{warned(1), rotated(2, "warn"), rotated(3, "info")}, []int64{1}},
	} {
		// As logAlerts reads them: the warnings and errors, and every
		// line, whatever its level, of the kinds that settle one of them.
		var lines, settling []event
		kinds := map[string]bool{}
		for _, e := range c.log {
			if !belowWarn(&e) {
				lines = append(lines, e)
				for _, k := range settleRules[e.Kind].by {
					kinds[k] = true
				}
			}
		}
		for _, e := range c.log {
			if kinds[e.Kind] {
				settling = append(settling, e)
			}
		}
		var got []int64
		for _, a := range unsettled(lines, settling, now) {
			got = append(got, a.ID)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: unsettled %v, want %v", c.name, got, c.want)
		}
	}
}

// A communication error's alert names, of the radiograms that failed
// while the flag stayed set, the most severe, of two as severe the
// oldest (D181): the state lines from the one that set the flag on, as
// the store gives them back, and the device's state now; a line before
// the flag's counts not.
func TestFlagRadiogram(t *testing.T) {
	t.Parallel()
	line := func(id int64, code int) event {
		st := map[string]any{"error": true}
		if code >= 0 {
			st["failed_radiogram"] = map[string]any{"code": float64(code)}
		}
		return event{ID: id, Kind: "state", Device: "0a0101", State: map[string]any{"rf_address": "0a0101", "state": st}}
	}
	run := func(codes ...int) []event {
		var out []event
		for i, c := range codes {
			out = append(out, line(int64(10+i), c))
		}
		return out
	}
	rt := func(r cube.RadiogramType) *cube.RadiogramType { return &r }
	for _, c := range []struct {
		name   string
		states []event
		from   int // the index of the line that set the flag
		now    *cube.RadiogramType
		want   *cube.RadiogramType
	}{
		{"the time push, then a setpoint", run(0x03, 0x40), 0, rt(0x40), rt(cube.RadiogramControlMode)},
		{"a setpoint, then a removal's link", run(0x40, 0x21), 0, rt(0x21), rt(cube.RadiogramRemoveLinkPartner)},
		{"two setpoints, the oldest", run(0x41, 0x40), 0, rt(0x40), rt(cube.RadiogramSetPointTemperature)},
		{"a configuration over a control", run(0x40, 0x11, 0x44), 0, rt(0x44), rt(cube.RadiogramConfigTemperatures)},
		{"a code the list lacks, then the time push, the oldest", run(0xf6, 0x03), 0, rt(0x03), rt(0xf6)},
		{"a line before the flag's counts not", run(0x21, 0x03, 0x40), 1, rt(0x40), rt(cube.RadiogramControlMode)},
		{"the state now, not yet in a line", run(0x03), 0, rt(0x10), rt(cube.RadiogramConfigWeekTemperatureProfile)},
		{"lines without one, the state now's", run(-1, -1), 0, rt(0x03), rt(cube.RadiogramTimeInformation)},
		{"none", run(-1), 0, nil, nil},
	} {
		got := flagRadiogram(c.states, &c.states[c.from], c.now)
		if (got == nil) != (c.want == nil) || got != nil && *got != *c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// GET /api/v1/log/alerts: a disconnect settled by the next connect, an
// error of the server's left, a device's transmit error while it is set,
// at the line that set it; after leaves out the older ones.
func TestLogAlertsRoute(t *testing.T) {
	t.Parallel()
	r := startServer(t, stored(), withOptions(Options{PollInterval: 100 * time.Millisecond}))
	fc, ts, st := r.fc, r.ts, r.st
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
	testkit.Eventually(t, "the restart's error", func() bool {
		return getJSON(t, ts, "", "/api/v1/log/alerts", &out) == 200 && len(out.Alerts) == 1
	})
	if a := out.Alerts[0]; a.Kind != "restart" || a.Level != "error" || out.Level != "error" || a.CubeSerial != "" {
		t.Fatalf("the alerts: %+v", out)
	}
	restart := out.Alerts[0].ID

	greeting := recorded(t)
	last := greeting[len(greeting)-1]
	raw, _ := base64.StdEncoding.DecodeString(last.Data)
	flagged := append([]byte(nil), raw...)
	// 0a010d: a transmit error, and its battery flag, which is no alert:
	// the battery's alerts are its level's (TestLogAlertsBatteryLevel)
	copy(flagged, []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12 | 0x08, 0x01 | 0x80, 30, 42, 0, 0, 0})
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(flagged)})
	testkit.Eventually(t, "the transmit error", func() bool {
		return getJSON(t, ts, "", "/api/v1/log/alerts", &out) == 200 && len(out.Alerts) >= 2
	})
	if len(out.Alerts) != 2 {
		t.Fatalf("the transmit error and the battery flag: %+v, want the restart's and the error's alone", out.Alerts)
	}
	a := out.Alerts[0]
	if a.Kind != "state" || a.Flag != "error" || a.Device != "0a010d" || a.Level != "warn" || a.CubeSerial != "KEQ0000100" || a.ID <= restart || out.Level != "error" {
		t.Fatalf("the transmit error: %+v", out)
	}
	getJSON(t, ts, "", "/api/v1/log/alerts?after="+strconv.FormatInt(restart, 10), &out)
	if len(out.Alerts) != 1 || out.Alerts[0].Flag != "error" || out.Level != "warn" {
		t.Fatalf("after the restart's line: %+v", out)
	}

	fc.SetDeviceList(last)
	testkit.Eventually(t, "the error cleared", func() bool {
		return getJSON(t, ts, "", "/api/v1/log/alerts", &out) == 200 && len(out.Alerts) == 1
	})
	if code, _ := callAPI(t, ts, "GET", "/api/v1/log/alerts?after=x", "", ""); code != 400 {
		t.Fatalf("a bad after: %d", code)
	}
}

// GET /api/v1/log/alerts over the settings file read again and again
// (AP StartConfiguration, D11-A): of the reads that warn in a row, the
// newest alone is the alert, and the answer's level, the log's sign, is
// warn with it; a read without a finding leaves neither; another kind's
// warning and its counterpart leave it as it is; a start settles a
// read's warning, and its own notes, after its server_started, are the
// alert; a start without notes leaves none.
func TestLogAlertsSettingsReadInARow(t *testing.T) {
	t.Parallel()
	refused := []Refusal{{Settings: []string{"MASSIMO_HSTS"}, Error: `-hsts or MASSIMO_HSTS "-1": want a number of days, 0 or more`}}
	notes := []string{"MASSIMO_HSTS is given on lines 3 and 4; the first wins"}
	// newest is the newest settings_read's id, with the store's written.
	newest := func(r *rig) int64 {
		t.Helper()
		if err := r.st.FlushEvents(t.Context()); err != nil {
			t.Fatal(err)
		}
		rows, err := r.st.Events(t.Context(), store.EventQuery{Kind: "settings_read", Limit: 1})
		if err != nil || len(rows) != 1 {
			t.Fatalf("the newest settings_read: %v %v", rows, err)
		}
		return rows[0].ID
	}
	// read writes a read's settings_read, a warning where it found a
	// refusal or a note, and answers its id.
	read := func(r *rig, refused []Refusal, notes []string) int64 {
		t.Helper()
		r.s.LogSettingsRead(t.Context(), "the settings file changed", nil, nil, refused, notes)
		return newest(r)
	}
	// sign checks the alerts, with the store's written: want's, newest
	// first, settings_read at warn, and the level warn with one, "" with
	// none.
	sign := func(r *rig, what string, want ...int64) {
		t.Helper()
		if err := r.st.FlushEvents(t.Context()); err != nil {
			t.Fatal(err)
		}
		var out logAlertsView
		if code := getJSON(t, r.ts, "", "/api/v1/log/alerts", &out); code != 200 {
			t.Fatalf("%s: GET: %d", what, code)
		}
		var got []int64
		for _, a := range out.Alerts {
			if a.Kind != "settings_read" || a.Level != "warn" {
				t.Fatalf("%s: an alert %+v", what, a)
			}
			got = append(got, a.ID)
		}
		level := ""
		if len(want) > 0 {
			level = "warn"
		}
		if !slices.Equal(got, want) || out.Level != level {
			t.Fatalf("%s: the alerts %v at %q, want %v at %q", what, got, out.Level, want, level)
		}
	}
	first := startServer(t, stored())
	sign(first, "the start")
	sign(first, "a refusal", read(first, refused, nil))
	sign(first, "a note after it", read(first, nil, notes))
	sign(first, "both after it", read(first, refused, notes))
	read(first, nil, nil)
	sign(first, "a read without a finding")
	last := read(first, refused, nil)
	sign(first, "a refusal again", last)
	first.s.LogRotated("size", "serve.log.1", 1, nil, errors.New("log not rotated: permission denied"))
	first.s.LogRotated("size", "serve.log.1", 1, nil, nil)
	sign(first, "a failed rotation and a good one after it", last)
	first.stop()

	second := startServer(t, again(first), withOptions(Options{SettingsNotes: notes}))
	start := newest(second)
	if start <= last {
		t.Fatalf("the start's notes: no settings_read after %d", last)
	}
	sign(second, "the start's notes", start)
	sign(second, "a read after the start's notes", read(second, refused, notes))
	second.stop()

	third := startServer(t, again(second))
	sign(third, "a start without notes")
}

// A person's marks (Block 2): all hides the alerts up to its id, a
// line's mark that line alone; another person's alerts stay; a document
// that is not marks is refused.
func TestLogAlertsSeenMarks(t *testing.T) {
	t.Parallel()
	r := startServer(t, stored())
	ts, st := r.ts, r.st
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
