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

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/view"
)

// waitConnected waits until the Cube of serial is listed as connected.
func waitConnected(t *testing.T, ts *httptest.Server, serial string) map[string]any {
	t.Helper()
	var found map[string]any
	testkit.Eventually(t, "cube "+serial+" to connect", func() bool {
		var cubes []map[string]any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		for _, c := range cubes {
			if c["serial"] == serial && c["connected"] == true {
				found = c
				return true
			}
		}
		return false
	})
	return found
}

// greetingAt reads the H line of a proxy listening on addr.
func greetingAt(t *testing.T, addr string) cube.Message {
	t.Helper()
	var conn net.Conn
	var err error
	if !testkit.Within(5*time.Second, func() bool {
		conn, err = net.Dial("tcp", addr)
		return err == nil
	}) {
		t.Fatalf("no proxy on %s: %v", addr, err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(testkit.Scale(10 * time.Second)))
	m, err := cube.NewReader(conn).Read()
	if err != nil {
		t.Fatalf("greeting at %s: %v", addr, err)
	}
	return m
}

// PUT changes the row and applies it at once: the name, the hidden list
// (the device leaves the views), the proxy address (the listener moves).
func TestPutCube(t *testing.T) {
	t.Parallel()
	ports := &heldPorts{}
	r := startServer(t, stored(), listenOn(ports))
	ts, st := r.ts, r.st
	ctx := context.Background()

	var devs []view.Device
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices?devices=real", &devs)
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
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices?devices=real", &after)
	if len(after) != len(devs)-1 {
		t.Fatalf("hidden device still listed: %d of %d", len(after), len(devs))
	}
	if code := getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/"+hide, nil); code != 404 {
		t.Fatalf("hidden device answered %d", code)
	}

	// The proxy listener starts on its address, and moves with it.
	p1, p2 := ports.hold(t), ports.hold(t)
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

// DELETE stops serving the Cube and removes its row, which is kept with
// its history (D20-B); POST serves it again, as new.
func TestDeleteAndAddCube(t *testing.T) {
	r := startServer(t, stored())
	ts, st := r.ts, r.st
	ctx := context.Background()
	before, _, _ := st.Cube(ctx, "KEQ0000100")
	cubeAddr := before.Address
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100", ""); code != 200 || out["serial"] != "KEQ0000100" {
		t.Fatalf("delete %d %v", code, out)
	}
	if row, ok, _ := st.Cube(ctx, "KEQ0000100"); ok {
		t.Fatalf("a live row after delete %+v", row)
	}
	removed, _ := st.RemovedCubes(ctx, "KEQ0000100")
	if len(removed) != 1 || removed[0].ID != before.ID || removed[0].Configured || removed[0].Source != store.SourceUI {
		t.Fatalf("row after delete %+v", removed)
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
	row, _, _ := st.Cube(ctx, "KEQ0000100")
	if !row.Configured || row.Name != "Again" || row.ID == before.ID {
		t.Fatalf("row after post %+v", row)
	}
	if code, _ := do(t, ts, "POST", "/api/v1/cubes", `{"serial":"KEQ0000100"}`); code != 409 {
		t.Fatalf("second post %d", code)
	}
	if code, _ := do(t, ts, "POST", "/api/v1/cubes", `{"serial":"no/serial"}`); code != 400 {
		t.Fatalf("bad serial %d", code)
	}
}

// A removed Cube found again lists the removed rows of its serial, and
// discovery makes no row for it (D20-B); POST adds it as new, beside a
// removed row, or with "restore" revives the removed row it names, one
// of two, with its id and settings.
func TestRestoreRemovedCube(t *testing.T) {
	t.Parallel()
	other, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	greeting := recorded(t)
	second := &fakecube.Server{Greeting: greeting}
	addr2, err := second.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	_, port, _ := net.SplitHostPort(addr2)
	p, _ := strconv.Atoi(port)
	rg := startServer(t, stored(), withOptions(Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{other.Addr()}, DiscoveryWait: 200 * time.Millisecond, CubePort: p}))
	ts, st := rg.ts, rg.st
	ctx := context.Background()
	if code, _ := postDiscovery(t, ts); code != 200 {
		t.Fatal(code)
	}
	if code, out := do(t, ts, "POST", "/api/v1/cubes", `{"serial":"KEQ0000000","name":"First"}`); code != 201 {
		t.Fatalf("add %d %v", code, out)
	}
	first, _, _ := st.Cube(ctx, "KEQ0000000")
	found := func() map[string]any {
		t.Helper()
		var cubes []map[string]any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		for _, c := range cubes {
			if c["serial"] == "KEQ0000000" && c["configured"] == false {
				return c
			}
		}
		t.Fatalf("KEQ0000000 not found: %v", cubes)
		return nil
	}
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000000", ""); code != 200 {
		t.Fatalf("delete %d %v", code, out)
	}
	removed, _ := found()["removed"].([]any)
	if len(removed) != 1 {
		t.Fatalf("removed rows of the found cube: %v", found())
	}
	if r := removed[0].(map[string]any); r["uuid"] != first.UUID || r["name"] != "First" || r["removed_at"] == "" {
		t.Fatalf("the removed row: %v", r)
	}
	postDiscovery(t, ts)
	if row, ok, _ := st.Cube(ctx, "KEQ0000000"); ok {
		t.Fatalf("discovery made a row beside a removed one: %+v", row)
	}

	if code, out := do(t, ts, "POST", "/api/v1/cubes", `{"serial":"KEQ0000000","name":"Second"}`); code != 201 || out["name"] != "Second" {
		t.Fatalf("add as new %d %v", code, out)
	}
	fresh, _, _ := st.Cube(ctx, "KEQ0000000")
	if fresh.ID == first.ID || !fresh.Configured || fresh.DiscoveredAddress != "127.0.0.1" || fresh.RFAddress != "0a0b0c" {
		t.Fatalf("added as new: %+v", fresh)
	}
	waitConnected(t, ts, "KEQ0000000")
	if code, _ := do(t, ts, "POST", "/api/v1/cubes", `{"serial":"KEQ0000000","restore":"`+first.UUID+`"}`); code != 409 {
		t.Fatalf("restore of a served cube %d", code)
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000000", `{"restore":"`+first.UUID+`"}`); code != 400 {
		t.Fatalf("restore in a PUT %d", code)
	}
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000000", ""); code != 200 {
		t.Fatalf("delete %d %v", code, out)
	}
	if removed, _ := found()["removed"].([]any); len(removed) != 2 || removed[0].(map[string]any)["uuid"] != fresh.UUID {
		t.Fatalf("two removed rows, newest first: %v", removed)
	}
	if code, _ := do(t, ts, "POST", "/api/v1/cubes", `{"serial":"KEQ0000000","restore":"`+fresh.UUID+`x"}`); code != 404 {
		t.Fatalf("restore of no removed row %d", code)
	}
	code, out := do(t, ts, "POST", "/api/v1/cubes", `{"serial":"KEQ0000000","restore":"`+first.UUID+`"}`)
	if code != 201 || out["name"] != "First" {
		t.Fatalf("restore %d %v", code, out)
	}
	back, _, _ := st.Cube(ctx, "KEQ0000000")
	if back.ID != first.ID || back.Name != "First" || !back.Configured {
		t.Fatalf("restored: %+v", back)
	}
	waitConnected(t, ts, "KEQ0000000")
	if left, _ := st.RemovedCubes(ctx, "KEQ0000000"); len(left) != 1 || left[0].ID != fresh.ID {
		t.Fatalf("removed rows after the restore: %+v", left)
	}
}

// POST of a found Cube connects it at the address discovery saw, and it
// follows discovery from then on.
func TestAddFoundCube(t *testing.T) {
	t.Parallel()
	other, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	greeting := recorded(t)
	second := &fakecube.Server{Greeting: greeting}
	addr2, err := second.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	_, port, _ := net.SplitHostPort(addr2)
	p, _ := strconv.Atoi(port)
	r := startServer(t, stored(), withOptions(Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{other.Addr()}, DiscoveryWait: 200 * time.Millisecond, CubePort: p}))
	ts, st := r.ts, r.st
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

// configuredCubes is the store's configured rows, as serve takes them
// for the Cubes it serves at a start.
func configuredCubes(t *testing.T, st *store.Store) []CubeConfig {
	t.Helper()
	rows, err := st.Cubes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var out []CubeConfig
	for _, row := range rows {
		if !row.Configured {
			continue
		}
		cfg, err := CubeFromRow(row)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, cfg)
	}
	return out
}

// removedListed removes the Cube of serial through the API and reports
// how often it is listed then: never as a configured Cube, which the
// web UI would draw as one that is disconnected.
func removedListed(t *testing.T, ts *httptest.Server, serial string) int {
	t.Helper()
	if code, out := do(t, ts, "DELETE", "/api/v1/cubes/"+serial, ""); code != 200 {
		t.Fatalf("delete %d %v", code, out)
	}
	if code := getJSON(t, ts, "", "/api/v1/cubes/"+serial, nil); code != 404 {
		t.Fatalf("the removed cube answered %d", code)
	}
	return listedFound(t, ts, serial)
}

// listedFound reports how often GET /cubes lists serial, failing on an
// entry that is not a found Cube.
func listedFound(t *testing.T, ts *httptest.Server, serial string) int {
	t.Helper()
	var cubes []map[string]any
	getJSON(t, ts, "", "/api/v1/cubes", &cubes)
	n := 0
	for _, c := range cubes {
		if c["serial"] != serial {
			continue
		}
		n++
		if _, has := c["connected"]; has || c["configured"] != false {
			t.Fatalf("the removed cube is listed as configured: %v", c)
		}
	}
	if n > 1 {
		t.Fatalf("the removed cube is listed %d times: %v", n, cubes)
	}
	return n
}

// A removed Cube is listed as found, never as a configured Cube without
// a connection, whichever way it was configured: from the discovery
// list, from its own variable in the settings file, or from the plain
// MASSIMO_CUBE_ADDRESS. Its found entry had known it as served: from
// the store after a restart, or from a discovery answer while it was
// served. That answer comes from 192.0.2.10 straight into note: the
// Cube's address is fixed, so nothing is sent there.
func TestRemovedCubeIsListedAsFound(t *testing.T) {
	ctx := context.Background()
	lan := cubelan.Discovered{Serial: "KEQ0000100", IP: net.ParseIP("192.0.2.10"), RFAddress: 0x0a0b0c, Firmware: "1.1.3", Kind: cubelan.KindCube}

	t.Run("discovery list", func(t *testing.T) {
		disc, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000100", 0x0a0b0c)
		if err != nil {
			t.Fatal(err)
		}
		defer disc.Close()
		greeting := recorded(t)
		fc := &fakecube.Server{Greeting: greeting}
		addr, err := fc.Start("127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer fc.Close()
		_, port, _ := net.SplitHostPort(addr)
		p, _ := strconv.Atoi(port)
		opts := Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{disc.Addr()}, DiscoveryWait: 200 * time.Millisecond, CubePort: p}
		st := testStore(t, "")
		r := startServer(t, onStore(st), withOptions(opts), withCubes(configuredCubes(t, st)...))
		ts := r.ts
		if code, _ := postDiscovery(t, ts); code != 200 {
			t.Fatal(code)
		}
		if code, out := do(t, ts, "POST", "/api/v1/cubes", `{"serial":"KEQ0000100"}`); code != 201 {
			t.Fatalf("add the found cube %d %v", code, out)
		}
		waitConnected(t, ts, "KEQ0000100")
		r.stop()

		ts = startServer(t, onStore(st), withOptions(opts), withCubes(configuredCubes(t, st)...)).ts
		removedListed(t, ts, "KEQ0000100")
		postDiscovery(t, ts)
		if n := listedFound(t, ts, "KEQ0000100"); n != 1 {
			t.Fatalf("the removed cube is not found after a search: %d", n)
		}
	})

	t.Run("settings file", func(t *testing.T) {
		r := startServer(t, stored(), withOptions(Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{testkit.ListenPacket(t).LocalAddr().String()}}))
		ts, s := r.ts, r.s
		s.note(ctx, lan, time.Now())
		if n := removedListed(t, ts, "KEQ0000100"); n != 1 {
			t.Fatalf("the removed cube is listed %d times", n)
		}
	})

	t.Run("plain address", func(t *testing.T) {
		greeting := recorded(t)
		fc := &fakecube.Server{Greeting: greeting}
		addr, err := fc.Start("127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer fc.Close()
		st := testStore(t, "")
		plain := store.FileCube{Values: map[string]store.FileValue{store.FieldAddress: {Variable: "MASSIMO_CUBE_ADDRESS", Value: addr}}}
		r := startServer(t, onStore(st), withOptions(Options{FileCubes: map[string]store.FileCube{"": plain},
			DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{testkit.ListenPacket(t).LocalAddr().String()}}),
			withCubes(append([]CubeConfig{{Addr: addr, Source: store.SourceEnv, SeedFromFile: true}}, configuredCubes(t, st)...)...))
		ts, s := r.ts, r.s
		testkit.Eventually(t, "the plain address's cube seeded", func() bool {
			row, ok, _ := st.Cube(ctx, "KEQ0000100")
			return ok && row.Configured
		})
		s.note(ctx, lan, time.Now())
		if n := removedListed(t, ts, "KEQ0000100"); n != 1 {
			t.Fatalf("the removed cube is listed %d times", n)
		}
	})
}

// Without a store the Cube and settings routes say so.
func TestCubeRoutesNeedAStore(t *testing.T) {
	t.Parallel()
	ts := startServer(t).ts
	for _, r := range [][3]string{{"PUT", "/api/v1/cubes/KEQ0000100", `{"name":"x"}`}, {"DELETE", "/api/v1/cubes/KEQ0000100", ""}, {"POST", "/api/v1/cubes", `{"serial":"KEQ0000001"}`}, {"GET", "/api/v1/settings", ""}} {
		if code, _ := do(t, ts, r[0], r[1], r[2]); code != 503 {
			t.Errorf("%s %s: %d", r[0], r[1], code)
		}
	}
}

// The settings: defaults, a change that applies at once, the checks.
// The poll interval's default is 10 s; a flag that decides a setting for
// the run is its fixed_by.
func TestSettingsAPI(t *testing.T) {
	t.Parallel()
	r := startServer(t, stored(), withOptions(Options{PollInterval: 2 * time.Minute, SettingsFixedBy: map[string]string{store.PollIntervalKey: "-poll"}}))
	ts, srv, st := r.ts, r.s, r.st
	code, out := do(t, ts, "GET", "/api/v1/settings", "")
	stale, _ := out["stale_after"].(map[string]any)
	poll, _ := out["poll_interval"].(map[string]any)
	refresh, _ := out["refresh_interval"].(map[string]any)
	node, _ := out["node.id"].(map[string]any)
	if code != 200 || stale["value"] != "50h" || stale["set"] != false || stale["applies"] != "now" || node["value"] != st.NodeID() || node["read_only"] != true {
		t.Fatalf("get %d %v", code, out)
	}
	// The wall thermostats' own threshold, 24 h, set at once (AP LegacyLogs, D136-A).
	if wall, _ := out["stale_after_wall"].(map[string]any); wall["value"] != "24h" || wall["applies"] != "now" {
		t.Fatalf("stale_after_wall %v", wall)
	}
	if code, out := do(t, ts, "PUT", "/api/v1/settings", `{"stale_after_wall":"12h"}`); code != 200 || srv.StaleAfterWall() != 12*time.Hour || srv.StaleAfter() != view.StaleAfter {
		t.Fatalf("stale_after_wall set %d %v", code, out)
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/settings", `{"stale_after_wall":null}`); code != 200 || srv.StaleAfterWall() != view.StaleAfterWall {
		t.Fatalf("stale_after_wall back to the default: %d", code)
	}
	if poll["value"] != "10s" || poll["default"] != "10s" || poll["in_effect"] != "2m" || poll["applies"] != "restart" || poll["fixed_by"] != "-poll" {
		t.Fatalf("poll_interval %v", poll)
	}
	if _, ok := refresh["fixed_by"]; ok {
		t.Fatalf("refresh_interval fixed without a flag: %v", refresh)
	}
	code, out = do(t, ts, "PUT", "/api/v1/settings", `{"stale_after":"36h","refresh_interval":"0"}`)
	stale, _ = out["stale_after"].(map[string]any)
	refresh, _ = out["refresh_interval"].(map[string]any)
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
	if code != 200 || stale["set"] != false || stale["value"] != "50h" || srv.StaleAfter() != view.StaleAfter {
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

// The setting announce: its value, this run's announcement as in_effect,
// what fixes it, at the next start; auto, on and off taken, null back to
// auto, anything else refused.
func TestAnnounceSettingAPI(t *testing.T) {
	t.Parallel()
	r := startServer(t, stored(), withOptions(Options{Announce: AnnounceState{On: true, FixedBy: "MASSIMO_ANNOUNCE"}}))
	ts, st := r.ts, r.st
	code, out := do(t, ts, "GET", "/api/v1/settings", "")
	ann, _ := out[store.AnnounceKey].(map[string]any)
	if code != 200 || ann["value"] != "auto" || ann["default"] != "auto" || ann["set"] != false || ann["in_effect"] != "on" || ann["applies"] != "restart" ||
		ann["fixed_by"] != "MASSIMO_ANNOUNCE" {
		t.Fatalf("get %d %v", code, ann)
	}
	code, out = do(t, ts, "PUT", "/api/v1/settings", `{"announce":"off"}`)
	ann, _ = out[store.AnnounceKey].(map[string]any)
	if code != 200 || ann["value"] != "off" || ann["set"] != true || ann["in_effect"] != "on" {
		t.Fatalf("put off %d %v", code, ann)
	}
	if v, _ := st.SettingString(context.Background(), store.AnnounceKey); v != "off" {
		t.Fatalf("stored %q", v)
	}
	for _, bad := range []string{`{"announce":"yes"}`, `{"announce":true}`, `{"announce":""}`} {
		if code, out := do(t, ts, "PUT", "/api/v1/settings", bad); code != 400 {
			t.Errorf("%s: %d %v", bad, code, out)
		}
	}
	if code, _ := do(t, ts, "PUT", "/api/v1/settings", `{"announce":null}`); code != 200 {
		t.Fatalf("null: %d", code)
	}
	if v, _ := st.SettingString(context.Background(), store.AnnounceKey); v != "" {
		t.Fatalf("not cleared: %q", v)
	}
}
