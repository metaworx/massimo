// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// startStored starts a server whose one Cube, the fake Cube KEQ0000100,
// is a row of a store in a temporary directory.
func startStored(t *testing.T, opts Options) (*fakecube.Server, *httptest.Server, *Server, *store.Store) {
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
	st := testStore(t, "")
	row, err := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := CubeFromRow(row)
	if err != nil {
		t.Fatal(err)
	}
	opts.Cubes, opts.Store = []CubeConfig{cfg}, st
	opts.Queue = engine.Options{Gap: time.Millisecond}
	s := New(opts)
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	waitConnected(t, ts, "KEQ0000100")
	return fc, ts, s, st
}

// waitConnected waits until the Cube of serial is listed as connected.
func waitConnected(t *testing.T, ts *httptest.Server, serial string) map[string]any {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var cubes []map[string]any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		for _, c := range cubes {
			if c["serial"] == serial && c["connected"] == true {
				return c
			}
		}
	}
	t.Fatalf("cube %s never connected", serial)
	return nil
}

// greetingAt reads the H line of a proxy listening on addr.
func greetingAt(t *testing.T, addr string) cube.Message {
	t.Helper()
	var conn net.Conn
	var err error
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if conn, err = net.Dial("tcp", addr); err == nil {
			break
		}
	}
	if conn == nil {
		t.Fatalf("no proxy on %s: %v", addr, err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	m, err := cube.NewReader(conn).Read()
	if err != nil {
		t.Fatalf("greeting at %s: %v", addr, err)
	}
	return m
}

func freeTCP(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

// PUT changes the row and applies it at once: the name, the hidden list
// (the device leaves the views), the proxy address (the listener moves).
func TestPutCube(t *testing.T) {
	_, ts, _, st := startStored(t, Options{})
	ctx := context.Background()

	var devs []view.Device
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices", &devs)
	if len(devs) < 2 {
		t.Fatalf("devices %v", devs)
	}
	hide := devs[0].RFAddress

	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"name":"Cellar","hidden":["`+strings.ToUpper(hide)+`"]}`)
	if code != 200 || out["name"] != "Cellar" || out["source"] != "ui" || len(out["hidden"].([]any)) != 1 || out["hidden"].([]any)[0] != hide {
		t.Fatalf("put %d %v", code, out)
	}
	row, _, _ := st.Cube(ctx, "KEQ0000100")
	if row.Name != "Cellar" || row.Source != store.SourceUI || row.Version != 2 || len(row.Hidden) != 1 {
		t.Fatalf("row %+v", row)
	}
	var after []view.Device
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices", &after)
	if len(after) != len(devs)-1 {
		t.Fatalf("hidden device still listed: %d of %d", len(after), len(devs))
	}
	if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/"+hide, nil); code != 404 {
		t.Fatalf("hidden device answered %d", code)
	}

	// The proxy listener starts on its address, and moves with it.
	p1, p2 := freeTCP(t), freeTCP(t)
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_address":"`+p1+`","proxy_serial":"leq0000100"}`); code != 200 || out["proxy_address"] != p1 || out["proxy_serial"] != "LEQ0000100" {
		t.Fatalf("proxy %d %v", code, out)
	}
	if m := greetingAt(t, p1); m.Type != 'H' || !strings.HasPrefix(m.Data, "KEQ0000100,") {
		t.Fatalf("greeting %v", m)
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_address":"`+p2+`"}`); code != 200 {
		t.Fatal(code)
	}
	greetingAt(t, p2)
	if c, err := net.DialTimeout("tcp", p1, time.Second); err == nil {
		c.Close()
		t.Fatal("old proxy address still listens")
	}

	for _, bad := range []string{`{"address":"no such host:x"}`, `{"hidden":["zz"]}`, `{"nmae":"typo"}`, `{"proxy_serial":"a-b"}`, `{"serial":"KEQ1"}`, `{"name":"` + strings.Repeat("x", 65) + `"}`} {
		if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", bad); code != 400 {
			t.Errorf("%s: %d %v", bad, code, out)
		}
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000001", `{"name":"x"}`); code != 404 {
		t.Fatalf("unknown cube %d", code)
	}
}

// DELETE stops serving the Cube and keeps its row; POST serves it again.
func TestDeleteAndAddCube(t *testing.T) {
	_, ts, _, st := startStored(t, Options{})
	ctx := context.Background()
	before, _, _ := st.Cube(ctx, "KEQ0000100")
	cubeAddr := before.Address
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100", ""); code != 200 || out["serial"] != "KEQ0000100" {
		t.Fatalf("delete %d %v", code, out)
	}
	row, ok, _ := st.Cube(ctx, "KEQ0000100")
	if !ok || row.Configured || row.Source != store.SourceUI {
		t.Fatalf("row after delete %+v %v", row, ok)
	}
	var cubes []map[string]any
	getJSON(t, ts, "", "/api/v1/cubes", &cubes)
	if len(cubes) != 0 {
		t.Fatalf("cubes after delete %v", cubes)
	}
	if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", nil); code != 404 {
		t.Fatalf("removed cube answered %d", code)
	}

	code, out := do(t, ts, "POST", "/api/v1/cubes", `{"serial":"keq0000100","address":"`+cubeAddr+`","name":"Again"}`)
	if code != 201 || out["serial"] != "KEQ0000100" || out["name"] != "Again" || out["source"] != "ui" {
		t.Fatalf("post %d %v", code, out)
	}
	c := waitConnected(t, ts, "KEQ0000100")
	if c["address"] != cubeAddr {
		t.Fatalf("address %v", c)
	}
	row, _, _ = st.Cube(ctx, "KEQ0000100")
	if !row.Configured || row.Name != "Again" {
		t.Fatalf("row after post %+v", row)
	}
	if code, _ := do(t, ts, "POST", "/api/v1/cubes", `{"serial":"KEQ0000100"}`); code != 409 {
		t.Fatalf("second post %d", code)
	}
	if code, _ := do(t, ts, "POST", "/api/v1/cubes", `{"serial":"no/serial"}`); code != 400 {
		t.Fatalf("bad serial %d", code)
	}
}

// POST of a found Cube connects it at the address discovery saw, and it
// follows discovery from then on.
func TestAddFoundCube(t *testing.T) {
	other, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	greeting, _ := fakecube.LoadGreeting(fixture)
	second := &fakecube.Server{Greeting: greeting}
	addr2, err := second.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	_, port, _ := net.SplitHostPort(addr2)
	p, _ := strconv.Atoi(port)
	_, ts, _, st := startStored(t, Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{other.Addr()}, DiscoveryWait: 200 * time.Millisecond, CubePort: p})
	if code, _ := postDiscovery(t, ts); code != 200 {
		t.Fatal(code)
	}
	code, out := do(t, ts, "POST", "/api/v1/cubes", `{"serial":"KEQ0000000"}`)
	if code != 201 || out["address"] != addr2 || out["follow_discovery"] != true {
		t.Fatalf("post found %d %v", code, out)
	}
	row, _, _ := st.Cube(context.Background(), "KEQ0000000")
	if !row.Configured || !row.FollowDiscovery || row.Source != store.SourceUI || row.DiscoveredAddress != "127.0.0.1" {
		t.Fatalf("row %+v", row)
	}
	var cubes []map[string]any
	getJSON(t, ts, "", "/api/v1/cubes", &cubes)
	for _, c := range cubes {
		if c["configured"] == false {
			t.Fatalf("the added cube is still listed as found: %v", cubes)
		}
	}
}

// Without a store the Cube and settings routes say so.
func TestCubeRoutesNeedAStore(t *testing.T) {
	_, ts, _ := start(t, "")
	for _, r := range [][3]string{{"PUT", "/api/v1/cubes/KEQ0000100", `{"name":"x"}`}, {"DELETE", "/api/v1/cubes/KEQ0000100", ""}, {"POST", "/api/v1/cubes", `{"serial":"KEQ0000001"}`}, {"GET", "/api/v1/settings", ""}} {
		if code, _ := do(t, ts, r[0], r[1], r[2]); code != 503 {
			t.Errorf("%s %s: %d", r[0], r[1], code)
		}
	}
}

// The settings: defaults, a change that applies at once, the checks.
func TestSettingsAPI(t *testing.T) {
	_, ts, srv, st := startStored(t, Options{PollInterval: 2 * time.Minute})
	code, out := do(t, ts, "GET", "/api/v1/settings", "")
	stale, _ := out["stale_after"].(map[string]any)
	poll, _ := out["poll_interval"].(map[string]any)
	node, _ := out["node.id"].(map[string]any)
	if code != 200 || stale["value"] != "24h" || stale["set"] != false || stale["applies"] != "now" || poll["in_effect"] != "2m" || poll["applies"] != "restart" || node["value"] != st.NodeID() || node["read_only"] != true {
		t.Fatalf("get %d %v", code, out)
	}
	code, out = do(t, ts, "PUT", "/api/v1/settings", `{"stale_after":"36h","refresh_interval":"0"}`)
	stale, _ = out["stale_after"].(map[string]any)
	refresh, _ := out["refresh_interval"].(map[string]any)
	if code != 200 || stale["value"] != "36h" || stale["set"] != true || stale["in_effect"] != "36h" || refresh["value"] != "0" {
		t.Fatalf("put %d %v", code, out)
	}
	if srv.StaleAfter() != 36*time.Hour {
		t.Fatalf("stale threshold in effect %v", srv.StaleAfter())
	}
	if d, ok, _ := st.SettingDuration(context.Background(), store.StaleAfterKey); !ok || d != 36*time.Hour {
		t.Fatalf("stored %v %v", d, ok)
	}
	for _, bad := range []string{`{"stale_after":"10m"}`, `{"poll_interval":"0"}`, `{"node.id":"x"}`, `{"colour":"red"}`, `{}`, `{"stale_after":"soon"}`, `{"stale_after":"36h","discover_interval":"1s"}`} {
		if code, out := do(t, ts, "PUT", "/api/v1/settings", bad); code != 400 {
			t.Errorf("%s: %d %v", bad, code, out)
		}
	}
	// A refused body writes nothing, also not its valid part.
	if v, _, _ := st.SettingDuration(context.Background(), store.StaleAfterKey); v != 36*time.Hour {
		t.Fatalf("a refused body changed stale_after to %v", v)
	}
	code, out = do(t, ts, "PUT", "/api/v1/settings", `{"stale_after":null}`)
	stale, _ = out["stale_after"].(map[string]any)
	if code != 200 || stale["set"] != false || stale["value"] != "24h" || srv.StaleAfter() != view.StaleAfter {
		t.Fatalf("back to the default %d %v", code, out)
	}
	// The log's retention: 30 days unset, 0 keeps everything, at most 3650.
	keep := func(out map[string]any) map[string]any { m, _ := out["log.keep_days"].(map[string]any); return m }
	if code, out := do(t, ts, "GET", "/api/v1/settings", ""); code != 200 || keep(out)["value"] != "30" || keep(out)["default"] != "30" || keep(out)["set"] != false || keep(out)["applies"] != "now" {
		t.Fatalf("log.keep_days unset: %d %v", code, keep(out))
	}
	if code, out := do(t, ts, "PUT", "/api/v1/settings", `{"log.keep_days": 0}`); code != 200 || keep(out)["value"] != "0" || keep(out)["set"] != true {
		t.Fatalf("log.keep_days 0: %d %v", code, keep(out))
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/settings", `{"log.keep_days": 3651}`); code != 400 {
		t.Fatalf("log.keep_days 3651: %d", code)
	}
	if code, out := do(t, ts, "PUT", "/api/v1/settings", `{"log.keep_days": null}`); code != 200 || keep(out)["value"] != "30" || keep(out)["set"] != false {
		t.Fatalf("log.keep_days back to the default: %d %v", code, keep(out))
	}
}
