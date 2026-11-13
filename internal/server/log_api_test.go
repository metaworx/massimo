// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
)

// logOf reads a page of the fake Cube's log.
func logOf(t *testing.T, ts *httptest.Server, query string) (int, []event) {
	t.Helper()
	var evs []event
	code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log"+query, &evs)
	return code, evs
}

// rawLog reads a page of the log as JSON objects, to see what a stored
// event holds.
func rawLog(t *testing.T, ts *httptest.Server, query string) []map[string]any {
	t.Helper()
	var evs []map[string]any
	if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log"+query, &evs); code != 200 {
		t.Fatalf("log%s: %d", query, code)
	}
	return evs
}

// The log of a Cube with a row is the store's: ids in order, pages by
// id and by time, filters by kind and device, the stored copy without
// the received times, a change of a device as a state event.
func TestLogFromStore(t *testing.T) {
	fc, ts, _, _ := startStored(t, Options{PollInterval: 50 * time.Millisecond})
	var all []event
	eventually(t, "the connect burst in the log", func() bool {
		_, all = logOf(t, ts, "?limit=1000")
		connected, states := false, 0
		for _, e := range all {
			connected = connected || e.Kind == "connected"
			if e.Kind == "state" {
				states++
			}
		}
		return connected && states >= 6
	})
	for i, e := range all {
		if e.ID == 0 || i > 0 && e.ID <= all[i-1].ID {
			t.Fatalf("event %d: id %d after %d", i, e.ID, all[max(i-1, 0)].ID)
		}
	}
	if code, page := logOf(t, ts, "?limit=5"); code != 200 || len(page) != 5 || page[4].ID != all[len(all)-1].ID || page[0].ID != all[len(all)-5].ID {
		t.Fatalf("the last five, oldest first: %d %v", code, ids(page))
	}

	// Paged back by id, every event once.
	seen := map[int64]bool{}
	for before := all[len(all)-1].ID + 1; ; {
		_, page := logOf(t, ts, "?limit=3&before="+strconv.FormatInt(before, 10))
		if len(page) == 0 {
			break
		}
		for _, e := range page {
			if seen[e.ID] || e.ID >= before {
				t.Fatalf("paging: %d before %d, seen %v", e.ID, before, seen[e.ID])
			}
			seen[e.ID] = true
		}
		before = page[0].ID
	}
	if len(seen) != len(all) {
		t.Fatalf("paged %d of %d events", len(seen), len(all))
	}

	// By kind, by device, by time.
	_, states := logOf(t, ts, "?kind=state")
	for _, e := range states {
		if e.Kind != "state" {
			t.Fatalf("kind=state answered %s", e.Kind)
		}
	}
	if len(states) < 6 {
		t.Fatalf("%d state events", len(states))
	}
	rf := states[0].Device
	_, mine := logOf(t, ts, "?device="+rf)
	for _, e := range mine {
		if e.Device != rf {
			t.Fatalf("device=%s answered %s", rf, e.Device)
		}
	}
	if len(mine) == 0 {
		t.Fatal("no event of the device")
	}
	if _, both := logOf(t, ts, "?kind=state&device="+rf); len(both) == 0 || len(both) > len(mine) {
		t.Fatalf("kind and device: %d of %d", len(both), len(mine))
	}
	// The burst's events share a millisecond, which is why the pages go
	// by id; a time bound takes the events before that millisecond.
	first, last := all[0], all[len(all)-1]
	if code, page := logOf(t, ts, "?limit=1000&before="+url.QueryEscape(first.Time.Format(time.RFC3339Nano))); code != 200 || len(page) != 0 {
		t.Fatalf("before the first event's time: %d %d", code, len(page))
	}
	if code, page := logOf(t, ts, "?limit=1000&before="+url.QueryEscape(last.Time.Add(time.Millisecond).Format(time.RFC3339Nano))); code != 200 || len(page) != len(all) {
		t.Fatalf("before a millisecond after the last event: %d %d of %d", code, len(page), len(all))
	}
	// since takes the events at or after a time, with before too.
	if code, page := logOf(t, ts, "?limit=1000&since="+url.QueryEscape(first.Time.Format(time.RFC3339Nano))); code != 200 || len(page) != len(all) {
		t.Fatalf("since the first event's time: %d %d of %d", code, len(page), len(all))
	}
	if code, page := logOf(t, ts, "?limit=1000&since="+url.QueryEscape(last.Time.Add(time.Millisecond).Format(time.RFC3339Nano))); code != 200 || len(page) != 0 {
		t.Fatalf("since a millisecond after the last event: %d %d", code, len(page))
	}
	if code, page := logOf(t, ts, "?limit=1000&since="+url.QueryEscape(last.Time.Format(time.RFC3339Nano))+
		"&before="+url.QueryEscape(last.Time.Add(time.Millisecond).Format(time.RFC3339Nano))); code != 200 || len(page) == 0 || page[len(page)-1].ID != last.ID {
		t.Fatalf("since and before around the last event: %d %v", code, ids(page))
	}
	for _, q := range []string{"?before=x", "?before=0", "?limit=0", "?limit=1001", "?limit=x", "?device=zz", "?since=x", "?since=4711",
		"?since=2026-11-06T21:00:00Z&before=2026-11-06T20:00:00Z"} {
		if code, _ := logOf(t, ts, q); code != 400 {
			t.Errorf("%s: %d", q, code)
		}
	}

	// A change of a device's values is a state event with its changes.
	greeting, _ := fakecube.LoadGreeting(fixture)
	l := greeting[len(greeting)-1]
	raw, _ := base64.StdEncoding.DecodeString(l.Data)
	copy(raw, []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12, 0x01, 30, 42, 0, 0, 0})
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	raw[7] = 55
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	eventually(t, "the change in the log", func() bool {
		_, page := logOf(t, ts, "?kind=state&device=0a010d&limit=1")
		return len(page) == 1 && page[0].Changes["valve_pct"].To == 55.0
	})
	// The device view has its received times; the stored copy not.
	var dev map[string]any
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/0a010d", &dev)
	if dev["received"] == nil {
		t.Fatalf("the device view without received: %v", dev)
	}
	for _, e := range rawLog(t, ts, "?kind=state&device=0a010d&limit=1000") {
		st, _ := e["state"].(map[string]any)
		if _, has := st["received"]; has {
			t.Fatalf("a stored state event with received: %v", e)
		}
	}
}

func ids(evs []event) []int64 {
	out := []int64{}
	for _, e := range evs {
		out = append(out, e.ID)
	}
	return out
}

// The log survives a restart of the server on the same store: the
// first run's events come before the second's connected.
func TestLogSurvivesRestart(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer fc.Close()
	st := testStore(t, "")
	row, _ := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv})
	run := func() (*httptest.Server, func()) {
		cfg, _ := CubeFromRow(row)
		s := New(Options{Cubes: []CubeConfig{cfg}, Store: st, Queue: engine.Options{Gap: time.Millisecond}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { s.Run(ctx); close(done) }()
		ts := httptest.NewServer(s.Handler())
		waitConnected(t, ts, "KEQ0000100")
		return ts, func() { ts.Close(); cancel(); <-done }
	}
	ts, stop := run()
	eventually(t, "the first run's connected", func() bool {
		_, page := logOf(t, ts, "?kind=connected")
		return len(page) == 1
	})
	stop()
	ts, stop = run()
	defer stop()
	eventually(t, "the second run's connected", func() bool {
		_, page := logOf(t, ts, "?kind=connected")
		return len(page) == 2 && page[0].ID < page[1].ID && !page[0].Time.After(page[1].Time)
	})
	if _, all := logOf(t, ts, "?limit=1000"); len(all) < 4 {
		t.Fatalf("%d events after the restart", len(all))
	}
}

// What a goroutine of the server emits as Run ends, as a restore's
// restore_done does, is in the store once Run has returned: the writer
// stops after the server's goroutines, not with them.
func TestLogLastEvents(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer fc.Close()
	st := testStore(t, "")
	row, _ := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv})
	cfg, _ := CubeFromRow(row)
	s := New(Options{Cubes: []CubeConfig{cfg}, Store: st, Queue: engine.Options{Gap: time.Millisecond}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	waitConnected(t, ts, "KEQ0000100")
	tctx := s.track()
	if tctx == nil {
		t.Fatal("Run tracks nothing")
	}
	go func() {
		defer s.wg.Done()
		<-tctx.Done()
		time.Sleep(50 * time.Millisecond)
		for _, rt := range s.runtimes() {
			rt.emit(event{Kind: "restore_done", Time: time.Now()})
		}
	}()
	cancel()
	<-done
	page, err := st.Events(context.Background(), store.EventQuery{CubeID: row.ID, Kind: "restore_done"})
	if err != nil || len(page) != 1 {
		t.Fatalf("the event emitted as Run ended: %d, %v", len(page), err)
	}
}

// A Cube without a row keeps its log in the ring: no ids, the filters
// applied in memory, an id bound empty, a device and a time as on the
// store.
func TestLogWithoutStore(t *testing.T) {
	_, ts, _ := start(t, "")
	var all []event
	eventually(t, "the connect burst in the ring", func() bool {
		_, all = logOf(t, ts, "?limit=1000")
		return len(all) >= 6
	})
	for _, e := range all {
		if e.ID != 0 {
			t.Fatalf("a ring event with an id: %+v", e)
		}
	}
	_, states := logOf(t, ts, "?kind=state&limit=2")
	if len(states) != 2 || states[0].Kind != "state" || states[1].Kind != "state" {
		t.Fatalf("kind=state&limit=2: %+v", states)
	}
	if _, page := logOf(t, ts, "?before=5"); len(page) != 0 {
		t.Fatalf("an id bound on the ring: %d", len(page))
	}
	rf := states[0].Device
	_, mine := logOf(t, ts, "?limit=1000&device="+rf)
	if len(mine) == 0 {
		t.Fatalf("device=%s: nothing", rf)
	}
	for _, e := range mine {
		if e.Device != rf {
			t.Fatalf("device=%s: %+v", rf, e)
		}
	}
	last := all[len(all)-1].Time
	_, older := logOf(t, ts, "?limit=1000&before="+url.QueryEscape(last.Format(time.RFC3339Nano)))
	for _, e := range older {
		if !e.Time.Before(last) {
			t.Fatalf("before=%v on the ring: %+v", last, e)
		}
	}
	if len(older) == 0 && all[0].Time.Before(last) {
		t.Fatalf("before=%v on the ring: nothing", last)
	}
	_, newer := logOf(t, ts, "?limit=1000&since="+url.QueryEscape(last.Format(time.RFC3339Nano)))
	// The connect burst may have added events since the first read; of
	// those since its last one, count the ones it had.
	read := 0
	for _, e := range newer {
		if e.Time.Before(last) {
			t.Fatalf("since=%v on the ring: %+v", last, e)
		}
		if !e.Time.After(last) {
			read++
		}
	}
	if read == 0 || read+len(older) != len(all) {
		t.Fatalf("since=%v on the ring: %d of the first read (%d now), and %d before, of %d", last, read, len(newer), len(older), len(all))
	}
	if code, _ := logOf(t, ts, "?limit=1001"); code != http.StatusBadRequest {
		t.Fatalf("limit=1001: %d", code)
	}
	var raw json.RawMessage
	if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log?kind=nothing", &raw); code != 200 || string(raw) != "[]" {
		t.Fatalf("an empty page: %d %s", code, raw)
	}
}

// A page by command holds that command's events alone, each naming it;
// an event of the audit visibility is on the page of a user with audit
// and on no other user's; command takes a positive id.
func TestLogByCommandAndVisibility(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	ctx := t.Context()
	viewer := userToken(t, st, "anna", store.RoleViewer)
	cmd, err := st.AddCommand(ctx, store.Command{Action: "setpoint"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s.logServer(event{Kind: "command", Source: "queue", Time: now, Command: cmd.ID})
	s.logServer(event{Kind: "login", Source: "server", Time: now, audit: true})
	s.logServer(event{Kind: "change", Source: "config", Time: now})

	var evs []event
	if code := getJSON(t, ts, "secret", "/api/v1/cubes/KEQ0000100/log?command="+strconv.FormatInt(cmd.ID, 10), &evs); code != 200 || len(evs) != 1 || evs[0].Kind != "command" || evs[0].Command != cmd.ID {
		t.Fatalf("the command's page: %d %+v", code, evs)
	}
	kinds := func(token string) map[string]bool {
		t.Helper()
		var page []event
		if code := getJSON(t, ts, token, "/api/v1/cubes/KEQ0000100/log?limit=1000", &page); code != 200 {
			t.Fatalf("log: %d", code)
		}
		seen := map[string]bool{}
		for _, e := range page {
			seen[e.Kind] = true
		}
		return seen
	}
	if seen := kinds("secret"); !seen["login"] || !seen["change"] {
		t.Errorf("the owner's page: %v", seen)
	}
	if seen := kinds(viewer); seen["login"] || !seen["change"] {
		t.Errorf("a viewer's page: %v", seen)
	}
	for _, bad := range []string{"0", "-1", "x"} {
		if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes/KEQ0000100/log?command="+bad, "secret", ""); code != 400 {
			t.Errorf("command=%s: %d", bad, code)
		}
	}
}

// A setpoint's lines are found by its command: the step queued and sent
// at debug, acknowledged at info, and its change line, each naming the
// command.
func TestSetpointLinesByCommand(t *testing.T) {
	_, ts, _ := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	if code, text := callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/6/setpoint", "secret", `{"mode":"auto"}`); code != 200 {
		t.Fatalf("setpoint: %d %s", code, text)
	}
	rows := auditRows(t, ts, "secret", "?limit=1")
	if len(rows) != 1 || rows[0].CommandID == 0 {
		t.Fatalf("audit row %+v", rows)
	}
	id := strconv.FormatInt(rows[0].CommandID, 10)
	want := []string{"command:queued:debug", "command:sent:debug", "command:acknowledged:info", "change::info"}
	var got []string
	eventually(t, "the setpoint's lines", func() bool {
		var evs []event
		getJSON(t, ts, "secret", "/api/v1/cubes/KEQ0000100/log?level=debug&command="+id, &evs)
		got = got[:0]
		for _, e := range evs {
			if e.Kind == "state" {
				continue // a device's new state, where the room's command reached one
			}
			state, _ := e.Summary["state"].(string)
			got = append(got, e.Kind+":"+state+":"+e.Level)
			if e.Command != rows[0].CommandID {
				t.Fatalf("a line of another command: %+v", e)
			}
		}
		return len(got) == len(want)
	})
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the setpoint's lines %v, want %v", got, want)
		}
	}
}

// A sign-in, a failed one and a lockout are the server's lines, which a
// user with audit sees and a viewer does not, on the log and the stream.
func TestSignInLinesForAuditorsAlone(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	viewer := userToken(t, s.opts.Store, "anna", store.RoleViewer)
	s.logSignIn("login", "anna", 0, "192.0.2.1")
	s.logSignIn("login_failed", "bob", 0, "192.0.2.2")
	s.logLockout("192.0.2.2", "logins")
	kinds := func(token string) map[string]string {
		t.Helper()
		var page []event
		getJSON(t, ts, token, "/api/v1/cubes/KEQ0000100/log?limit=1000&source=server", &page)
		seen := map[string]string{}
		for _, e := range page {
			seen[e.Kind] = e.Level
		}
		return seen
	}
	eventually(t, "the owner's sign-in lines", func() bool { return len(kinds("secret")) == 3 })
	if seen := kinds("secret"); seen["login"] != "info" || seen["login_failed"] != "warn" || seen["locked_out"] != "warn" {
		t.Errorf("the owner's lines: %v", seen)
	}
	if seen := kinds(viewer); len(seen) != 0 {
		t.Errorf("a viewer's lines: %v", seen)
	}
	rt := s.runtimes()[0]
	sees := func(sub *subscriber) int {
		rt.subsMu.Lock()
		defer rt.subsMu.Unlock()
		n := 0
		for _, p := range rt.replay {
			if p.ev.audit && sub.sees(p.ev, p.user) {
				n++
			}
		}
		return n
	}
	if n := sees(&subscriber{user: 2}); n != 0 {
		t.Errorf("a viewer's stream sees %d sign-in lines", n)
	}
	if n := sees(&subscriber{audit: true}); n != 3 {
		t.Errorf("an auditor's stream sees %d sign-in lines, want 3", n)
	}
}

// A proxy client's setpoint is a command of the store, with no user and
// the client's address, whose step and the device's next state are found
// by ?command=; the client's connection is a line of source proxy.
func TestProxyClientsCommandInTheLog(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxyAddr := ln.Addr().String()
	ln.Close()
	fc, ts, _, st := startStored(t, Options{PollInterval: 50 * time.Millisecond, Proxy: []ProxyListener{{Addr: proxyAddr}}})
	conn, r := proxyClient(t, proxyAddr)
	m, err := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a010d}, 21, cube.ModeManual, cube.Until{})
	if err != nil {
		t.Fatal(err)
	}
	// The S reply, past the L lines the proxy pushes as the polls come.
	conn.Write([]byte(m.Wire()))
	for {
		reply, err := r.Read()
		if err != nil {
			t.Fatal(err)
		}
		if reply.Type == 'S' {
			break
		}
		if reply.Type != 'L' {
			t.Fatalf("%s answered %v", m, reply)
		}
	}
	var id int64
	eventually(t, "the proxy client's command acknowledged", func() bool {
		var evs []event
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log?source=queue", &evs)
		for _, e := range evs {
			if e.Kind == "command" && e.Summary["state"] == store.StateAcknowledged {
				id = e.Command
			}
		}
		return id != 0
	})
	c, ok, err := st.Command(t.Context(), id)
	if err != nil || !ok || c.Action != "proxy.radio" || c.Target != "device:0a010d" || c.UserID != 0 || len(c.Steps) != 1 || !strings.Contains(string(c.Request), `"remote":"127.0.0.1:`) {
		t.Fatalf("the proxy client's command: %+v %v %v", c, ok, err)
	}
	eventually(t, "the command finished", func() bool {
		c, _, _ = st.Command(t.Context(), id)
		return c.State == store.StateAcknowledged
	})

	// The device's new state names the command.
	greeting, _ := fakecube.LoadGreeting(fixture)
	raw, _ := base64.StdEncoding.DecodeString(greeting[len(greeting)-1].Data)
	copy(raw, []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12, 0x01, 30, 42, 0, 0, 0})
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	eventually(t, "the state line of the command", func() bool {
		var evs []event
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log?level=debug&command="+strconv.FormatInt(id, 10), &evs)
		kinds := map[string]bool{}
		for _, e := range evs {
			kinds[e.Kind] = true
		}
		return kinds["command"] && kinds["state"]
	})
	var evs []event
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log?source=proxy", &evs)
	if len(evs) == 0 || evs[0].Kind != "proxy_connected" || !strings.HasPrefix(fmt.Sprint(evs[0].Summary["remote"]), "127.0.0.1:") {
		t.Fatalf("the proxy's lines: %+v", evs)
	}
}

// startTwoCubes starts a server with the fake Cube KEQ0000100 and a
// second Cube, KEQ0000200, that never connects; with stored, both are
// rows of a store, returned with the second Cube's row id.
func startTwoCubes(t *testing.T, stored bool) (*httptest.Server, *Server, *store.Store, int64) {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{Queue: engine.Options{Gap: time.Millisecond}}
	var st *store.Store
	var second int64
	if stored {
		st = testStore(t, "")
		ctx := context.Background()
		for _, c := range []store.Cube{{Serial: "KEQ0000100", Address: addr}, {Serial: "KEQ0000200", Address: "127.0.0.1:9"}} {
			c.Configured, c.Source = true, store.SourceEnv
			row, err := st.AddCube(ctx, c)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := CubeFromRow(row)
			if err != nil {
				t.Fatal(err)
			}
			opts.Cubes, second = append(opts.Cubes, cfg), row.ID
		}
		opts.Store = st
	} else {
		opts.Cubes = []CubeConfig{{Serial: "KEQ0000100", Addr: addr}, {Serial: "KEQ0000200", Addr: "127.0.0.1:9"}}
	}
	s := New(opts)
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	waitConnected(t, ts, "KEQ0000100")
	return ts, s, st, second
}

// allLog reads a page of every Cube's log.
func allLog(t *testing.T, ts *httptest.Server, query string) []event {
	t.Helper()
	var evs []event
	if code := getJSON(t, ts, "", "/api/v1/log"+query, &evs); code != 200 {
		t.Fatalf("log%s: %d", query, code)
	}
	return evs
}

// Every Cube's log is each Cube's rows, with the Cube's S/N, and the
// server's once, without one, oldest first; paged back by id across the
// Cubes, every row once; a device by its S/N or by its radio address on
// every Cube; a Cube's own log carries no Cube's S/N.
func TestLogOfEveryCube(t *testing.T) {
	ts, s, st, second := startTwoCubes(t, true)
	eventually(t, "the fake Cube's burst in the store", func() bool {
		_, own := logOf(t, ts, "?kind=state")
		return len(own) >= 6
	})
	at := time.Now().UTC()
	data, _ := json.Marshal(event{Kind: "state", Device: "0a0909", Serial: "MEQ0000909", Time: at})
	st.AddEvent(store.Event{CubeID: second, Time: at, Kind: "state", Source: "device", Device: "0a0909", Serial: "MEQ0000909", Data: data})
	s.logServer(systemEvent("settings_read", 0, nil))

	all := allLog(t, ts, "?limit=1000")
	servers, seconds := map[string]int{}, 0
	for i, e := range all {
		if i > 0 && e.ID <= all[i-1].ID {
			t.Fatalf("not oldest first at %d: %v", i, ids(all))
		}
		switch {
		case e.CubeSerial == "":
			if e.Source != "system" && e.Source != "server" {
				t.Fatalf("a Cube's line without its Cube: %+v", e)
			}
			servers[e.Kind]++
		case e.Serial == "MEQ0000909":
			seconds++
			if e.CubeSerial != "KEQ0000200" {
				t.Fatalf("the second Cube's line: %+v", e)
			}
		case e.CubeSerial != "KEQ0000100" && e.CubeSerial != "KEQ0000200":
			t.Fatalf("a Cube's line: %+v", e)
		}
	}
	if servers["server_started"] != 1 || servers["settings_read"] != 1 || seconds != 1 {
		t.Fatalf("the server's lines %v, the second Cube's %d", servers, seconds)
	}

	seen := map[int64]bool{}
	for before := all[len(all)-1].ID + 1; ; {
		page := allLog(t, ts, "?limit=4&before="+strconv.FormatInt(before, 10))
		if len(page) == 0 {
			break
		}
		for _, e := range page {
			if seen[e.ID] || e.ID >= before {
				t.Fatalf("paging: %d before %d", e.ID, before)
			}
			seen[e.ID] = true
		}
		before = page[0].ID
	}
	if len(seen) != len(all) {
		t.Fatalf("paged %d of %d", len(seen), len(all))
	}

	for _, device := range []string{"MEQ0000909", "0a0909"} {
		page := allLog(t, ts, "?device="+device)
		if len(page) != 1 || page[0].CubeSerial != "KEQ0000200" {
			t.Fatalf("device=%s: %+v", device, page)
		}
	}
	for _, e := range rawLog(t, ts, "?limit=1000") {
		if _, ok := e["cube_serial"]; ok {
			t.Fatalf("a Cube's own log names the Cube: %v", e)
		}
	}
}

// Without a store, every Cube's log merges the Cubes' rings by time,
// each Cube's events with its S/N and the server's once, though every
// ring keeps them.
func TestLogOfEveryCubeWithoutStore(t *testing.T) {
	ts, s, _, _ := startTwoCubes(t, false)
	eventually(t, "the fake Cube's burst in its ring", func() bool {
		_, own := logOf(t, ts, "?kind=state")
		return len(own) >= 6
	})
	s.logServer(systemEvent("settings_read", 0, nil))
	all := allLog(t, ts, "?limit=1000")
	servers, cubes := map[string]int{}, map[string]int{}
	for i, e := range all {
		if i > 0 && e.Time.Before(all[i-1].Time) {
			t.Fatalf("not by time at %d", i)
		}
		if e.CubeSerial == "" {
			if e.Source != "system" && e.Source != "server" {
				t.Fatalf("a Cube's line without its Cube: %+v", e)
			}
			servers[e.Kind]++
		}
		cubes[e.CubeSerial]++
	}
	if servers["server_started"] != 1 || servers["settings_read"] != 1 {
		t.Fatalf("the server's lines, each once: %v", servers)
	}
	if cubes["KEQ0000100"] < 6 || cubes["KEQ0000200"] == 0 || len(cubes) != 3 {
		t.Fatalf("the lines by Cube: %v", cubes)
	}
}
