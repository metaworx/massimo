// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
)

// freeUDP returns a local UDP address that was free a moment ago.
func freeUDP(t *testing.T, ip string) string {
	t.Helper()
	pc, err := net.ListenPacket("udp4", ip+":0")
	if err != nil {
		t.Skipf("cannot bind %s: %v", ip, err)
	}
	defer pc.Close()
	return pc.LocalAddr().String()
}

// startDiscovering starts a server over the fake Cube KEQ0000100 with the
// given options for discovery.
func startDiscovering(t *testing.T, opts Options) (*httptest.Server, *Server) {
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
	if opts.Cubes == nil {
		opts.Cubes = []CubeConfig{{Serial: "KEQ0000100", Addr: addr}}
	}
	opts.Queue = engine.Options{Gap: time.Millisecond}
	if opts.DiscoveryWait == 0 {
		opts.DiscoveryWait = 200 * time.Millisecond
	}
	s := New(opts)
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	return ts, s
}

func postDiscovery(t *testing.T, ts *httptest.Server) (int, map[string]any) {
	t.Helper()
	return do(t, ts, "POST", "/api/v1/cubes/discover", "")
}

func TestDiscoveryFindsUnconfiguredCubes(t *testing.T) {
	other, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	ts, _ := startDiscovering(t, Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{other.Addr()}})

	code, out := postDiscovery(t, ts)
	if code != 200 || out["ran_at"] == nil {
		t.Fatalf("discover %d %v", code, out)
	}
	found, _ := out["found"].([]any)
	if len(found) != 1 {
		t.Fatalf("found %v", out["found"])
	}
	f := found[0].(map[string]any)
	if f["serial"] != "KEQ0000000" || f["configured"] != false || f["address"] != "127.0.0.1" || f["rf_address"] != "0a0b0c" ||
		f["firmware"] != "1.1.3" || f["dhcp"] != true || f["found_at"] == nil || f["last_seen"] == nil {
		t.Fatalf("found cube %v", f)
	}

	// GET /cubes: the configured Cube, then the found one, in the same
	// object as the discovery answer.
	var cubes []map[string]any
	if getJSON(t, ts, "", "/api/v1/cubes", &cubes) != 200 || len(cubes) != 2 {
		t.Fatalf("cubes %v", cubes)
	}
	if cubes[0]["serial"] != "KEQ0000100" || cubes[0]["configured"] != true {
		t.Fatalf("configured cube %v", cubes[0])
	}
	for k := range cubes[1] {
		switch k {
		case "serial", "configured", "address", "mac", "rf_address", "firmware", "dhcp", "gateway", "netmask", "dns", "proxies", "found_at", "last_seen":
		default:
			t.Errorf("unexpected field %q in a found cube", k)
		}
	}
	if cubes[1]["serial"] != "KEQ0000000" || cubes[1]["configured"] != false {
		t.Fatalf("found cube in the list %v", cubes[1])
	}

	// cube_found once per new serial, not on later runs.
	postDiscovery(t, ts)
	var evs []event
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log", &evs)
	n := 0
	for _, e := range evs {
		if e.Kind == "cube_found" {
			n++
			if e.Cube == nil || e.Cube.Serial != "KEQ0000000" || e.Cube.Configured || e.Cube.DHCP == nil || !*e.Cube.DHCP {
				t.Fatalf("event %+v", e)
			}
		}
	}
	if n != 1 {
		t.Fatalf("%d cube_found events", n)
	}

	// Two calls during one run share its result.
	var wg sync.WaitGroup
	ran := make([]any, 2)
	for i := range ran {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, out := postDiscovery(t, ts)
			ran[i] = out["ran_at"]
		}()
	}
	wg.Wait()
	if ran[0] == nil || ran[0] != ran[1] {
		t.Fatalf("runs %v", ran)
	}
}

// TestDiscoveryDHCPFlag follows the flag through runs: false for a static
// address, absent for a Cube that never answered c, and kept when a
// later run gets no answer to c.
func TestDiscoveryDHCPFlag(t *testing.T) {
	static, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer static.Close()
	static.SetStatic(true)
	mute, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000001", 0x0a0b0d)
	if err != nil {
		t.Fatal(err)
	}
	defer mute.Close()
	mute.SetConfigSilent(true)
	ts, _ := startDiscovering(t, Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{static.Addr(), mute.Addr()}})

	flags := func(found []any) map[string]any {
		out := map[string]any{}
		for _, f := range found {
			m := f.(map[string]any)
			out[m["serial"].(string)] = m["dhcp"]
		}
		return out
	}
	listed := func() map[string]any {
		var cubes []any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		return flags(cubes[1:])
	}
	_, out := postDiscovery(t, ts)
	if got := flags(out["found"].([]any)); len(got) != 2 || got["KEQ0000000"] != false || got["KEQ0000001"] != nil {
		t.Fatalf("discovery answer %v", got)
	}
	if got := listed(); got["KEQ0000000"] != false || got["KEQ0000001"] != nil {
		t.Fatalf("listed %v", got)
	}
	var evs []event
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log", &evs)
	for _, e := range evs {
		if e.Kind != "cube_found" {
			continue
		}
		switch e.Cube.Serial {
		case "KEQ0000000":
			if e.Cube.DHCP == nil || *e.Cube.DHCP {
				t.Fatalf("event %+v", e.Cube)
			}
		case "KEQ0000001":
			if e.Cube.DHCP != nil {
				t.Fatalf("event %+v", e.Cube)
			}
		}
	}

	// The flag follows a change; a run without an answer keeps it.
	static.SetStatic(false)
	postDiscovery(t, ts)
	if got := listed(); got["KEQ0000000"] != true {
		t.Fatalf("after DHCP on %v", got)
	}
	static.SetConfigSilent(true)
	postDiscovery(t, ts)
	if got := listed(); got["KEQ0000000"] != true || got["KEQ0000001"] != nil {
		t.Fatalf("after a run without c %v", got)
	}
}

func TestDiscoveryLeavesOutTheProxysOwnAnswers(t *testing.T) {
	other, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	// The server's proxy answers discovery on own; the request goes to
	// itself as well, as a broadcast would.
	own := freeUDP(t, "127.0.0.1")
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxyAddr := tcp.Addr().String()
	tcp.Close()
	ts, _ := startDiscovering(t, Options{DiscoveryAddr: own, DiscoveryTargets: []string{own, other.Addr()},
		Proxy: []ProxyListener{{Addr: proxyAddr, Discovery: true}}})
	// Wait for the proxy's greeting data, without which it answers nothing.
	eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})
	_, out := postDiscovery(t, ts)
	found, _ := out["found"].([]any)
	if len(found) != 1 || found[0].(map[string]any)["serial"] != "KEQ0000000" {
		t.Fatalf("found %v", out["found"])
	}
}

func TestDiscoveryWithoutThePort(t *testing.T) {
	taken, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	ts, _ := startDiscovering(t, Options{DiscoveryAddr: taken.LocalAddr().String()})
	if code, out := postDiscovery(t, ts); code != http.StatusServiceUnavailable || out["error"] == nil {
		t.Fatalf("discover with the port taken: %d %v", code, out)
	}
}

// listedSerials are the serials of the found Cubes GET /cubes lists.
func listedSerials(t *testing.T, ts *httptest.Server) []string {
	t.Helper()
	var cubes []map[string]any
	getJSON(t, ts, "", "/api/v1/cubes", &cubes)
	var out []string
	for _, c := range cubes {
		if c["configured"] == false {
			out = append(out, c["serial"].(string))
		}
	}
	return out
}

// foundEvents counts the cube_found events per serial in the log of
// the configured Cube KEQ0000100.
func foundEvents(t *testing.T, ts *httptest.Server) map[string]int {
	t.Helper()
	var evs []event
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log", &evs)
	out := map[string]int{}
	for _, e := range evs {
		if e.Kind == "cube_found" && e.Cube != nil {
			out[e.Cube.Serial]++
		}
	}
	return out
}

// A search lists the Cubes that answered it: one that no longer answers
// leaves the list, its row keeping when it was last seen, and is found
// and announced again when it answers again.
func TestSearchListsOnlyCubesThatAnswered(t *testing.T) {
	a, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000001", 0x0a0b0d)
	if err != nil {
		t.Fatal(err)
	}
	bAddr := b.Addr()
	st := testStore(t, "")
	ts, _ := startDiscovering(t, Options{Store: st, DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{a.Addr(), bAddr}})

	postDiscovery(t, ts)
	if got := listedSerials(t, ts); len(got) != 2 || got[0] != "KEQ0000000" || got[1] != "KEQ0000001" {
		t.Fatalf("listed after the first search %v", got)
	}
	b.Close()
	_, out := postDiscovery(t, ts)
	if found, _ := out["found"].([]any); len(found) != 1 {
		t.Fatalf("the second search answered %v", out["found"])
	}
	if got := listedSerials(t, ts); len(got) != 1 || got[0] != "KEQ0000000" {
		t.Fatalf("listed once KEQ0000001 is gone %v", got)
	}
	if row, ok, err := st.Cube(t.Context(), "KEQ0000001"); err != nil || !ok || row.Configured || row.LastSeen.IsZero() {
		t.Fatalf("the gone cube's row %+v %v %v", row, ok, err)
	}
	if n := foundEvents(t, ts); n["KEQ0000000"] != 1 || n["KEQ0000001"] != 1 {
		t.Fatalf("cube_found events %v", n)
	}

	b, err = fakecube.StartDiscovery(bAddr, "KEQ0000001", 0x0a0b0d)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	postDiscovery(t, ts)
	if got := listedSerials(t, ts); len(got) != 2 {
		t.Fatalf("listed once KEQ0000001 is back %v", got)
	}
	if n := foundEvents(t, ts); n["KEQ0000000"] != 1 || n["KEQ0000001"] != 2 {
		t.Fatalf("cube_found events once it is back %v", n)
	}
}

// A run that fails cannot tell a Cube gone from one not heard: the list
// stays as it was.
func TestFailedRunChangesNothing(t *testing.T) {
	a, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	ts, s := startDiscovering(t, Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{a.Addr()}})
	postDiscovery(t, ts)
	a.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.discoverOnce(ctx); err == nil {
		t.Fatal("a cancelled run did not fail")
	}
	if got := listedSerials(t, ts); len(got) != 1 || got[0] != "KEQ0000000" {
		t.Fatalf("listed after a failed run %v", got)
	}
}

// A Cube configured by serial only is connected at the address it
// answers discovery from, and follows it when it changes.
func TestCubeConfiguredBySerialFollowsItsAddress(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	// Two homes for the same Cube on one port, 127.0.0.2 and 127.0.0.3
	// (other addresses than this host's own, which mean its own proxy).
	first := &fakecube.Server{Greeting: greeting}
	addr, err := first.Start("127.0.0.2:0")
	if err != nil {
		t.Skipf("cannot listen on 127.0.0.2: %v", err)
	}
	defer first.Close()
	_, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)
	second := &fakecube.Server{Greeting: greeting}
	if _, err := second.Start("127.0.0.3:" + portStr); err != nil {
		t.Skipf("cannot listen on 127.0.0.3: %v", err)
	}
	defer second.Close()
	at2, err := fakecube.StartDiscovery("127.0.0.2:0", "KEQ0000100", 0x0a0107)
	if err != nil {
		t.Skip(err)
	}
	defer at2.Close()
	at3, err := fakecube.StartDiscovery("127.0.0.3:0", "KEQ0000100", 0x0a0107)
	if err != nil {
		t.Skip(err)
	}
	defer at3.Close()
	at3.SetSilent(true)

	ts, _ := startDiscovering(t, Options{Cubes: []CubeConfig{{Serial: "KEQ0000100"}}, CubePort: port,
		DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{at2.Addr(), at3.Addr()}})
	// The first run at the start finds it; no interval is needed for that.
	cube := func() map[string]any {
		var cubes []map[string]any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		if len(cubes) == 0 {
			return nil
		}
		return cubes[0]
	}
	eventually(t, "the cube at 127.0.0.2", func() bool {
		c := cube()
		return c != nil && c["connected"] == true && c["address"] == addr
	})
	// It moves: the next run finds it at 127.0.0.3 and reconnects.
	at2.SetSilent(true)
	at3.SetSilent(false)
	first.Close()
	postDiscovery(t, ts)
	eventually(t, "the cube at 127.0.0.3", func() bool {
		c := cube()
		return c != nil && c["connected"] == true && c["address"] == "127.0.0.3:"+portStr && second.Connections() == 1
	})
	if c := cube(); c["configured"] != true {
		t.Fatalf("cube %v", c)
	}
	var cubes []json.RawMessage
	getJSON(t, ts, "", "/api/v1/cubes", &cubes)
	if len(cubes) != 1 {
		t.Fatalf("a configured cube also listed as found: %s", cubes)
	}
}

// homes starts a fake Cube's TCP side on 127.0.0.2 and 127.0.0.3, one
// port, and a discovery responder on each address; the one on 127.0.0.3
// answers as a massimo proxy, the identify request alone. The loopback
// has no ARP entries, so mac gives the MACs the search sees.
func homes(t *testing.T) (port int, cubeTCP, proxyTCP *fakecube.Server, atCube, atProxy *fakecube.DiscoveryResponder) {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	cubeTCP = &fakecube.Server{Greeting: greeting}
	addr, err := cubeTCP.Start("127.0.0.2:0")
	if err != nil {
		t.Skipf("cannot listen on 127.0.0.2: %v", err)
	}
	t.Cleanup(func() { cubeTCP.Close() })
	_, portStr, _ := net.SplitHostPort(addr)
	port, _ = strconv.Atoi(portStr)
	proxyTCP = &fakecube.Server{Greeting: greeting}
	if _, err := proxyTCP.Start("127.0.0.3:" + portStr); err != nil {
		t.Skipf("cannot listen on 127.0.0.3: %v", err)
	}
	t.Cleanup(func() { proxyTCP.Close() })
	if atCube, err = fakecube.StartDiscovery("127.0.0.2:0", "KEQ0000100", 0x0a0107); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { atCube.Close() })
	if atProxy, err = fakecube.StartDiscovery("127.0.0.3:0", "KEQ0000100", 0x0a0107); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { atProxy.Close() })
	atProxy.SetIdentifyOnly(true)
	return port, cubeTCP, proxyTCP, atCube, atProxy
}

// A Cube heard from its own address and from a proxy is one found Cube,
// at its own address; configured by serial, it follows the proxy, whose
// serve holds the Cube's one LAN slot, and stays there run after run.
func TestOneSerialFromACubeAndAProxy(t *testing.T) {
	port, cubeTCP, proxyTCP, atCube, atProxy := homes(t)
	mac := func(ip net.IP) string {
		switch ip.String() {
		case "127.0.0.2":
			return "00:1a:22:00:00:02"
		case "127.0.0.3":
			return "02:00:00:00:00:03"
		}
		return ""
	}
	var log syncBuffer
	ts, _ := startDiscovering(t, Options{Cubes: []CubeConfig{{Serial: "KEQ0000100"}}, CubePort: port, LookupMAC: mac,
		Logger:        slog.New(slog.NewTextHandler(&log, nil)),
		DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{atCube.Addr(), atProxy.Addr()}})
	proxyAddr := "127.0.0.3:" + strconv.Itoa(port)
	eventually(t, "the cube through the proxy", func() bool {
		var cubes []map[string]any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		return len(cubes) == 1 && cubes[0]["connected"] == true && cubes[0]["address"] == proxyAddr
	})
	for i := 0; i < 2; i++ {
		code, body := postDiscovery(t, ts)
		found, _ := body["found"].([]any)
		if code != http.StatusOK || len(found) != 1 {
			t.Fatalf("run %d: %d %v", i, code, body)
		}
		if f := found[0].(map[string]any); f["address"] != "127.0.0.2" || f["mac"] != "00:1a:22:00:00:02" || f["dhcp"] != true {
			t.Fatalf("run %d: the found cube %v", i, f)
		}
		// Its proxies, and the addresses it uses from its answer to N:
		// the fake's lease, a /24 with the gateway at .1.
		f := found[0].(map[string]any)
		if p, _ := f["proxies"].([]any); len(p) != 1 || p[0] != "127.0.0.3" || f["gateway"] != "127.0.0.1" || f["netmask"] != "255.255.255.0" {
			t.Fatalf("run %d: proxies and addresses %v", i, f)
		}
	}
	var cubes []map[string]any
	getJSON(t, ts, "", "/api/v1/cubes", &cubes)
	if len(cubes) != 1 || cubes[0]["address"] != proxyAddr || cubes[0]["connected"] != true {
		t.Fatalf("cubes %v", cubes)
	}
	if n := strings.Count(log.String(), `msg="cube address from discovery"`); n != 1 {
		t.Errorf("%d address changes in three runs, want 1:\n%s", n, log.String())
	}
	if cubeTCP.Connections() != 0 || proxyTCP.Connections() != 1 {
		t.Errorf("connections: %d to the Cube, %d to the proxy", cubeTCP.Connections(), proxyTCP.Connections())
	}
}

// An answer the search cannot tell, a proxy's without a MAC, is not
// followed away from the address a Cube has, even when the Cube's own
// answer is missing.
func TestAnUnknownAnswerIsNotFollowed(t *testing.T) {
	port, cubeTCP, proxyTCP, atCube, atProxy := homes(t)
	atProxy.SetSilent(true)
	var log syncBuffer
	ts, _ := startDiscovering(t, Options{Cubes: []CubeConfig{{Serial: "KEQ0000100"}}, CubePort: port,
		Logger:        slog.New(slog.NewTextHandler(&log, nil)),
		DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{atCube.Addr(), atProxy.Addr()}})
	cubeAddr := "127.0.0.2:" + strconv.Itoa(port)
	address := func() any {
		var cubes []map[string]any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		if len(cubes) != 1 || cubes[0]["connected"] != true {
			return nil
		}
		return cubes[0]["address"]
	}
	eventually(t, "the cube at its own address", func() bool { return address() == cubeAddr })
	atCube.SetSilent(true)
	atProxy.SetSilent(false)
	postDiscovery(t, ts)
	if a := address(); a != cubeAddr {
		t.Fatalf("followed an unknown answer: %v", a)
	}
	if n := strings.Count(log.String(), `msg="cube address from discovery"`); n != 1 || proxyTCP.Connections() != 0 || cubeTCP.Connections() != 1 {
		t.Errorf("%d address changes, %d connections to the proxy, %d to the Cube", n, proxyTCP.Connections(), cubeTCP.Connections())
	}
}
