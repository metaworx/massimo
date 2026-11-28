// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/announce"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/sharedport"
	"go.metaworx.ch/massimo/internal/testkit"
)

// freeUDP returns a local UDP address that was free a moment ago, for
// the server's own discovery socket (Options.DiscoveryAddr), which the
// test also sends to: the server opens that socket by its address alone,
// its proxies' one or a run's own, so no socket the test holds can be
// handed on, and another program can take the port between
// (docs/testing.md, "Ports").
func freeUDP(t *testing.T, ip string) string {
	t.Helper()
	pc, err := net.ListenPacket("udp4", ip+":0")
	if err != nil {
		testkit.BindOrSkip(t, err)
	}
	defer pc.Close()
	return pc.LocalAddr().String()
}

// discovering is the start of the discovery's tests: the server's
// options o, a run's wait 200 ms where o sets none, and the wait from
// an answer for its N and c answers 500 ms, scaled, in place of the
// search's second: a proxy, the server's own or a fake's, answers the
// identify request alone, and a run waits that out for it; a fake
// Cube's N and c come at once, but on a starved runner later than its
// answer by more than a run's 200 ms.
func discovering(o Options) startOpt {
	if o.DiscoveryWait == 0 {
		o.DiscoveryWait = 200 * time.Millisecond
	}
	if o.discoveryLate == 0 {
		o.discoveryLate = testkit.Scale(500 * time.Millisecond)
	}
	return withOptions(o)
}

// postDiscovery asks for a run and fails the test where the answer's
// run began before the call: the API's call gets a run of its own, one
// that heard what the test changed just before it (discoverAfter). An
// answer without ran_at, an error, is returned as it is.
func postDiscovery(t *testing.T, ts *httptest.Server) (int, map[string]any) {
	t.Helper()
	called := time.Now()
	code, out := do(t, ts, "POST", "/api/v1/cubes/discover", "")
	at, ok := out["ran_at"].(string)
	if !ok {
		return code, out
	}
	ranAt, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		t.Fatalf("ran_at %q: %v", at, err)
	}
	if ranAt.Before(called) {
		t.Fatalf("the call got a run that began at %s, before the call at %s", at, called.Format(time.RFC3339Nano))
	}
	return code, out
}

// A call of the API while a run is going gets the run after it, which
// every call that came meanwhile shares; the discovery loop's call
// shares the run going.
func TestACallDuringARunGetsTheNext(t *testing.T) {
	t.Parallel()
	a, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	// A long wait, so that the calls below come while the first run goes.
	r := startServer(t, discovering(Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{a.Addr()}, DiscoveryWait: time.Second}))
	ts, s := r.ts, r.s
	ctx := context.Background()
	go s.discover(ctx)
	var going *discoveryRun
	testkit.Eventually(t, "a run going", func() bool {
		s.discMu.Lock()
		defer s.discMu.Unlock()
		going = s.discRun
		return going != nil
	})
	called := time.Now()
	joined, next := make(chan *discoveryRun, 1), make(chan *discoveryRun, 1)
	go func() { run, _ := s.discover(ctx); joined <- run }()
	go func() { run, _ := s.discoverAfter(ctx); next <- run }()
	_, out := postDiscovery(t, ts)
	if run := <-joined; run != going {
		t.Error("the loop's call did not share the run going")
	}
	run := <-next
	if run == going || run.ranAt.Before(called) || !going.ranAt.Before(called) {
		t.Fatalf("the run going began at %s, the call's at %s, the call at %s", going.ranAt, run.ranAt, called)
	}
	if at, _ := time.Parse(time.RFC3339Nano, out["ran_at"].(string)); !at.Equal(run.ranAt) {
		t.Errorf("two calls during a run got runs that began at %s and %s", at, run.ranAt)
	}
	if found, _ := out["found"].([]any); len(found) != 1 {
		t.Errorf("found %v", out["found"])
	}
}

func TestDiscoveryFindsUnconfiguredCubes(t *testing.T) {
	t.Parallel()
	other, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	ts := startServer(t, discovering(Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{other.Addr()}})).ts

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
	// object as the discovery answer, with the backups the store keeps of
	// it, none here (AP BackupWatch v1.0, D5-A).
	var cubes []map[string]any
	if getJSON(t, ts, "", "/api/v1/cubes", &cubes) != 200 || len(cubes) != 2 {
		t.Fatalf("cubes %v", cubes)
	}
	if cubes[0]["serial"] != "KEQ0000100" || cubes[0]["configured"] != true {
		t.Fatalf("configured cube %v", cubes[0])
	}
	for k := range cubes[1] {
		switch k {
		case "serial", "kind", "serves", "host", "configured", "other_network", "address", "mac", "rf_address", "firmware", "dhcp", "gateway", "netmask", "dns", "proxies", "found_at", "last_seen",
			"backups":
		default:
			t.Errorf("unexpected field %q in a found cube", k)
		}
	}
	if cubes[1]["serial"] != "KEQ0000000" || cubes[1]["configured"] != false || cubes[1]["backups"] != 0.0 {
		t.Fatalf("found cube in the list %v", cubes[1])
	}
	if _, has := f["backups"]; has {
		t.Errorf("the discovery answer counts backups: %v", f)
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
	// Calls during a run share the run after it:
	// TestACallDuringARunGetsTheNext.
}

// TestDiscoveryDHCPFlag follows the flag through runs: false for a static
// address, absent for a Cube that never answered c, and kept when a
// later run gets no answer to c.
func TestDiscoveryDHCPFlag(t *testing.T) {
	t.Parallel()
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
	ts := startServer(t, discovering(Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{static.Addr(), mute.Addr()}})).ts

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
	t.Parallel()
	other, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	// The server's proxy answers discovery on own; the request goes to
	// itself as well, as a broadcast would.
	own := freeUDP(t, "127.0.0.1")
	ports := &heldPorts{}
	proxyAddr := ports.hold(t)
	// The start waits for the Cube, whose greeting data the proxy needs
	// to answer at all.
	ts := startServer(t, discovering(Options{DiscoveryAddr: own, DiscoveryTargets: []string{own, other.Addr()},
		Proxy: []ProxyListener{{Addr: proxyAddr, Discovery: true}}}), listenOn(ports)).ts
	_, out := postDiscovery(t, ts)
	found, _ := out["found"].([]any)
	if len(found) != 1 || found[0].(map[string]any)["serial"] != "KEQ0000000" {
		t.Fatalf("found %v", out["found"])
	}
}

// A served Cube on this host, as the fake Cube of the UI tests is, is
// heard: its answer is a Cube's, which no proxy of the server gives. The
// same serial from this host answering the identify request alone is the
// server's own proxy's, and left out.
func TestDiscoveryHearsAServedCubeOnThisHost(t *testing.T) {
	t.Parallel()
	atHost, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer atHost.Close()
	ts := startServer(t, discovering(Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{atHost.Addr()}}),
		withCubes(CubeConfig{Serial: "KEQ0000000", Addr: "127.0.0.1:9"})).ts
	if got := foundSerials(t, ts); len(got) != 1 || got[0] != "KEQ0000000" {
		t.Fatalf("found %v, want the served Cube KEQ0000000", got)
	}
	atHost.SetIdentifyOnly(true)
	if got := foundSerials(t, ts); len(got) != 0 {
		t.Fatalf("found %v, want none: a proxy's answer from this host", got)
	}
}

// foundSerials are the serials a discovery run found, configured or not.
func foundSerials(t *testing.T, ts *httptest.Server) []string {
	t.Helper()
	_, out := postDiscovery(t, ts)
	found, _ := out["found"].([]any)
	var serials []string
	for _, f := range found {
		serials = append(serials, f.(map[string]any)["serial"].(string))
	}
	return serials
}

// A proxy started after the server, as one of a Cube added or changed
// through the API is (startCubeProxy), answers discovery with a serial of
// its own, which no configured Cube has: the server's own answer, never a
// Cube found.
func TestDiscoveryLeavesOutALaterProxysAnswers(t *testing.T) {
	t.Parallel()
	other, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	own := freeUDP(t, "127.0.0.1")
	ports := &heldPorts{}
	proxyAddr := ports.hold(t)
	r := startServer(t, discovering(Options{DiscoveryAddr: own, DiscoveryTargets: []string{own, other.Addr()}}), listenOn(ports))
	ts, s := r.ts, r.s
	if !s.startProxy(ProxyListener{Serial: "KEQ0000100", Addr: proxyAddr, Discovery: true, DiscoverySerial: "KEQ0000999"}) {
		t.Fatal("the proxy did not start")
	}
	if got := foundSerials(t, ts); len(got) != 1 || got[0] != "KEQ0000000" {
		t.Fatalf("found %v, want the other Cube alone", got)
	}
}

// A proxy's answer with the serial a configured Cube's proxy answers
// with is this server's from any address, as through NAT or from a
// container; a Cube's own answer with it is not.
func TestDiscoveryLeavesOutItsProxySerialFromAnywhere(t *testing.T) {
	t.Parallel()
	atProxy, err := fakecube.StartDiscovery("127.0.0.3:0", "KEQ0000999", 0x0a0b0c)
	if err != nil {
		testkit.BindOrSkip(t, err)
	}
	defer atProxy.Close()
	atProxy.SetIdentifyOnly(true)
	mac := func(ip net.IP) string {
		if ip.String() == "127.0.0.3" {
			return "02:00:00:00:00:03"
		}
		return ""
	}
	ts := startServer(t, discovering(Options{LookupMAC: mac, DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{atProxy.Addr()}}),
		withCubes(CubeConfig{Serial: "KEQ0000100", ProxySerial: "KEQ0000999"})).ts
	if got := foundSerials(t, ts); len(got) != 0 {
		t.Fatalf("found %v, want none", got)
	}
	// A Cube, which answers N and c as well, is never a proxy's answer.
	atProxy.SetIdentifyOnly(false)
	if got := foundSerials(t, ts); len(got) != 1 || got[0] != "KEQ0000999" {
		t.Fatalf("found %v, want the Cube KEQ0000999", got)
	}
}

// A found entry says what it is: a Cube, a massimo proxy by its answer,
// or one at an announced server's address, which is a proxy too; a proxy
// names the Cube with its radio address and the server it runs on, a
// Cube the server one of its proxies runs on. The servers are asked at
// the discovery targets' addresses, as long as the run waits.
func TestFoundEntriesNameTheirKindAndServer(t *testing.T) {
	t.Parallel()
	start := func(addr, serial string, rf cube.RFAddress, identifyOnly bool) *fakecube.DiscoveryResponder {
		d, err := fakecube.StartDiscovery(addr, serial, rf)
		if err != nil {
			testkit.BindOrSkip(t, err)
		}
		t.Cleanup(func() { d.Close() })
		d.SetIdentifyOnly(identifyOnly)
		return d
	}
	atCube := start("127.0.0.4:0", "KEQ0000201", 0x0a0101, false)
	atProxy := start("127.0.0.3:0", "KEQ0000202", 0x0a0101, true)
	atServer := start("127.0.0.5:0", "KEQ0000203", 0x0a0102, true)
	mac := func(ip net.IP) string {
		if ip.String() == "127.0.0.3" {
			return "02:00:00:00:00:03"
		}
		return ""
	}
	var asked atomic.Value
	browse := func(_ context.Context, wait time.Duration, unicast ...net.IP) ([]announce.Found, error) {
		var ips []string
		for _, ip := range unicast {
			ips = append(ips, ip.String())
		}
		asked.Store(strings.Join(ips, " ") + " " + wait.String())
		return []announce.Found{
			{Name: "box-a", Host: "massimo-1.local.", Port: 8443, From: net.IPv4(192, 0, 2, 7), Addrs: []net.IP{net.IPv4(127, 0, 0, 3)}, TLS: true,
				TXT: map[string]string{"url": "https://box-a.example:8443"}},
			{Host: "box-b.local.", Port: 8080, From: net.IPv4(127, 0, 0, 5)},
		}, nil
	}
	ts := startServer(t, discovering(Options{LookupMAC: mac, BrowseServers: browse, DiscoveryAddr: "127.0.0.1:0",
		DiscoveryTargets: []string{atProxy.Addr(), atCube.Addr(), atServer.Addr()}})).ts
	_, out := postDiscovery(t, ts)
	if got, want := asked.Load(), "127.0.0.3 127.0.0.4 127.0.0.5 200ms"; got != want {
		t.Errorf("servers asked at %v, want %q", got, want)
	}
	check := func(where string, found []map[string]any) {
		t.Helper()
		by := map[string]map[string]any{}
		for _, f := range found {
			by[f["serial"].(string)] = f
		}
		want := map[string]string{
			"KEQ0000201": "cube  box-a https://box-a.example:8443",
			"KEQ0000202": "proxy KEQ0000201 box-a https://box-a.example:8443",
			"KEQ0000203": "proxy  box-b http://127.0.0.5:8080",
		}
		for serial, w := range want {
			f := by[serial]
			if f == nil {
				t.Errorf("%s: %s not found", where, serial)
				continue
			}
			host, _ := f["host"].(map[string]any)
			serves, _ := f["serves"].(string)
			if got := fmt.Sprintf("%v %s %v %v", f["kind"], serves, host["name"], host["url"]); got != w {
				t.Errorf("%s: %s is %q, want %q", where, serial, got, w)
			}
		}
	}
	var answer []map[string]any
	for _, f := range out["found"].([]any) {
		answer = append(answer, f.(map[string]any))
	}
	check("the run's answer", answer)
	var cubes []map[string]any
	getJSON(t, ts, "", "/api/v1/cubes", &cubes)
	check("GET /cubes", cubes)
}

func TestDiscoveryWithoutThePort(t *testing.T) {
	t.Parallel()
	taken, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	ts := startServer(t, discovering(Options{DiscoveryAddr: taken.LocalAddr().String()})).ts
	if code, out := postDiscovery(t, ts); code != http.StatusServiceUnavailable || out["error"] == nil {
		t.Fatalf("discover with the port taken: %d %v", code, out)
	}
}

// While serve's proxies answer discovery, another program on the host
// can still bind the discovery port with the port shared, as the
// original software's MulticastSocket does; a plain bind, the command
// line's, is refused and falls back to another port.
func TestDiscoveryPortShared(t *testing.T) {
	t.Parallel()
	own := freeUDP(t, "127.0.0.1")
	ports := &heldPorts{}
	proxyAddr := ports.hold(t)
	s := startServer(t, discovering(Options{DiscoveryAddr: own, Proxy: []ProxyListener{{Addr: proxyAddr, Discovery: true}}}), listenOn(ports)).s
	testkit.Eventually(t, "the proxies' discovery socket", func() bool {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.responder != nil
	})
	pc, err := sharedport.ListenPacket("udp4", own)
	if err != nil {
		t.Fatalf("a shared bind beside serve: %v", err)
	}
	pc.Close()
	if pc, err := net.ListenPacket("udp4", own); err == nil {
		pc.Close()
		t.Fatal("a plain bind beside serve")
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
	t.Parallel()
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
	ts := startServer(t, discovering(Options{Store: st, DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{a.Addr(), bAddr}})).ts

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
	t.Parallel()
	a, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	r := startServer(t, discovering(Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{a.Addr()}}))
	ts, s := r.ts, r.s
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
	t.Parallel()
	// The first run at the start finds it; no interval is needed for that.
	m := startMovingCube(t, 0)
	// It moves: the next run finds it at 127.0.0.3 and reconnects.
	moved := m.move()
	postDiscovery(t, m.ts)
	m.movedTo3(moved)
	var cubes []map[string]any
	getJSON(t, m.ts, "", "/api/v1/cubes", &cubes)
	if len(cubes) != 1 {
		t.Fatalf("a configured cube also listed as found: %v", cubes)
	}
	if cubes[0]["configured"] != true {
		t.Fatalf("cube %v", cubes[0])
	}
}

// requestCounter is a discovery target that answers nothing and counts
// the requests it gets.
func requestCounter(t *testing.T) (addr string, count func() int) {
	t.Helper()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	var n atomic.Int64
	go func() {
		buf := make([]byte, 512)
		for {
			if _, _, err := pc.ReadFrom(buf); err != nil {
				return
			}
			n.Add(1)
		}
	}()
	return pc.LocalAddr().String(), func() int { return int(n.Load()) }
}

// noRunGoing reports whether no discovery run is going.
func noRunGoing(s *Server) bool {
	s.discMu.Lock()
	defer s.discMu.Unlock()
	return s.discRun == nil
}

// movingCube is a Cube configured by serial only, connected at
// 127.0.0.2 with no run going, which can move to 127.0.0.3 on the same
// port: its two homes, their discovery responders (the one at 127.0.0.3
// silent), a request counter among the targets, and the server's log
// at Debug, where its client's tries are. The client's backoff is 50
// ms in place of a second: a dial refused around the move doubles it,
// and from a second the next tries came two and four seconds later,
// past the wait for the move on a loaded host.
type movingCube struct {
	t             *testing.T
	ts            *httptest.Server
	s             *Server
	first, second *fakecube.Server
	at2, at3      *fakecube.DiscoveryResponder
	port          string
	requests      func() int
	log           *testkit.Watch[testkit.Record]
}

func startMovingCube(t *testing.T, retry time.Duration) *movingCube {
	t.Helper()
	greeting := recorded(t)
	m := &movingCube{t: t, first: &fakecube.Server{Greeting: greeting}, second: &fakecube.Server{Greeting: greeting}}
	addr, err := m.first.Start("127.0.0.2:0")
	if err != nil {
		testkit.BindOrSkip(t, err)
	}
	t.Cleanup(func() { m.first.Close() })
	_, m.port, _ = net.SplitHostPort(addr)
	port, _ := strconv.Atoi(m.port)
	if _, err := m.second.Start("127.0.0.3:" + m.port); err != nil {
		testkit.BindOrSkip(t, err)
	}
	t.Cleanup(func() { m.second.Close() })
	if m.at2, err = fakecube.StartDiscovery("127.0.0.2:0", "KEQ0000100", 0x0a0107); err != nil {
		testkit.BindOrSkip(t, err)
	}
	t.Cleanup(func() { m.at2.Close() })
	if m.at3, err = fakecube.StartDiscovery("127.0.0.3:0", "KEQ0000100", 0x0a0107); err != nil {
		testkit.BindOrSkip(t, err)
	}
	t.Cleanup(func() { m.at3.Close() })
	m.at3.SetSilent(true)
	counter, requests := requestCounter(t)
	m.requests = requests
	r := startServer(t, withLog(slog.LevelDebug), discovering(Options{CubePort: port, AddressRetry: retry,
		DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{m.at2.Addr(), m.at3.Addr(), counter},
		cubeOptions: func(o *cubelan.Options) { o.Backoff = 50 * time.Millisecond }}), withCubes(CubeConfig{Serial: "KEQ0000100"}))
	m.ts, m.s, m.log = r.ts, r.s, r.log
	testkit.Eventually(t, "the cube at 127.0.0.2", func() bool { return m.at(addr) })
	testkit.Eventually(t, "the run at the start to end", func() bool { return noRunGoing(m.s) })
	return m
}

// at reports whether the Cube is connected at addr.
func (m *movingCube) at(addr string) bool {
	var cubes []map[string]any
	getJSON(m.t, m.ts, "", "/api/v1/cubes", &cubes)
	return len(cubes) == 1 && cubes[0]["connected"] == true && cubes[0]["address"] == addr
}

// moveMark is what came before a move: the connections the new home
// had taken, and the server's log.
type moveMark struct {
	conns int
	log   testkit.Cursor
}

// mark is the mark before a move the test makes itself.
func (m *movingCube) mark() moveMark {
	return moveMark{conns: m.second.Connections(), log: m.log.Mark()}
}

// move moves the Cube from 127.0.0.2 to 127.0.0.3: the old home falls
// silent and closes, the new one answers discovery. It returns the mark
// before it.
func (m *movingCube) move() moveMark {
	mark := m.mark()
	m.at2.SetSilent(true)
	m.at3.SetSilent(false)
	m.first.Close()
	return mark
}

// movedTo3 waits until the Cube is connected at 127.0.0.3, the new
// home having taken a connection since the mark: the connections since
// the move, not one in all, since a client that connected there twice,
// as after a reply timeout on a loaded host, is there all the same.
// The failure says what the API showed, the connections to each home,
// and the server's lines of the Cube since the mark, the client's
// tries among them, so that a slow reconnect is told from a second one.
func (m *movingCube) movedTo3(mark moveMark) {
	m.t.Helper()
	addr := "127.0.0.3:" + m.port
	var seen string
	if testkit.Within(testkit.Wait, func() bool {
		at := m.at(addr)
		since := m.second.Connections() - mark.conns
		seen = fmt.Sprintf("connected there %v, %d connection(s) to 127.0.0.3 since the move, %d to 127.0.0.2 in all", at, since, m.first.Connections())
		return at && since > 0
	}) {
		return
	}
	var lines []string
	for _, rec := range m.log.Since(mark.log, testkit.Contains("source=cube")) {
		lines = append(lines, rec.String())
	}
	m.t.Fatalf("timed out after %v waiting for the cube at %s: %s; the cube's lines since the move:\n%s",
		testkit.Scale(testkit.Wait), addr, seen, strings.Join(lines, "\n"))
}

// A Cube configured by serial that is lost is searched for at once,
// and followed to its new address without a call of the API: the minute
// between searches is left as it is.
func TestALostCubeIsSearchedForAtOnce(t *testing.T) {
	t.Parallel()
	m := startMovingCube(t, 0)
	m.movedTo3(m.move())
}

// A Cube still lost after the search at the loss is searched for again
// every AddressRetry until it answers.
func TestALostCubeIsSearchedForWhileLost(t *testing.T) {
	t.Parallel()
	m := startMovingCube(t, 200*time.Millisecond)
	m.at2.SetSilent(true)
	before, moved := m.requests(), m.mark()
	m.first.Close()
	// The search at the loss hears nobody: both homes are silent.
	testkit.Eventually(t, "a search after the loss", func() bool { return m.requests() > before && noRunGoing(m.s) })
	if m.at("127.0.0.3:" + m.port) {
		t.Fatal("followed to a silent address")
	}
	m.at3.SetSilent(false)
	m.movedTo3(moved)
	// Back, it is no longer searched for.
	testkit.Eventually(t, "the last run to end", func() bool { return noRunGoing(m.s) })
	time.Sleep(600 * time.Millisecond) // a run asked for before it was back
	settled := m.requests()
	time.Sleep(500 * time.Millisecond)
	if n := m.requests(); n != settled {
		t.Errorf("%d searches after the cube was back", n-settled)
	}
}

// fixedCube starts a server over a Cube with a fixed address at
// 127.0.0.1, which a responder at 127.0.0.3 answers discovery for, and
// returns the Cube, the watch of the server's log, where the warnings
// that it answers from another address are (elsewhereMsg), and the
// request count.
func fixedCube(t *testing.T, interval time.Duration) (s *Server, fc *fakecube.Server, connected func() bool, logs *testkit.Watch[testkit.Record], requests func() int) {
	t.Helper()
	elsewhere, err := fakecube.StartDiscovery("127.0.0.3:0", "KEQ0000100", 0x0a0107)
	if err != nil {
		testkit.BindOrSkip(t, err)
	}
	t.Cleanup(func() { elsewhere.Close() })
	counter, requests := requestCounter(t)
	r := startServer(t, withLog(nil), discovering(Options{DiscoverInterval: interval, AddressRetry: 100 * time.Millisecond,
		DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{elsewhere.Addr(), counter}}))
	connected = func() bool {
		var cubes []map[string]any
		getJSON(t, r.ts, "", "/api/v1/cubes", &cubes)
		return len(cubes) == 1 && cubes[0]["connected"] == true
	}
	testkit.Eventually(t, "no run going", func() bool { return noRunGoing(r.s) })
	return r.s, r.fc, connected, r.log, requests
}

// Without periodic discovery, a lost Cube with a fixed address is not
// searched for: discovery runs on request only, and cannot move it.
func TestALostFixedCubeWithoutAnIntervalIsNotSearchedFor(t *testing.T) {
	t.Parallel()
	_, fc, connected, logs, requests := fixedCube(t, 0)
	fc.Close()
	testkit.Eventually(t, "the cube lost", func() bool { return !connected() }, awayWait)
	time.Sleep(500 * time.Millisecond) // five AddressRetry
	if n, w := requests(), logs.Count(0, testkit.Msg(elsewhereMsg)); n != 0 || w != 0 {
		t.Fatalf("%d searches, %d warnings, want none", n, w)
	}
}

// With periodic discovery, a lost Cube with a fixed address is searched
// for, and one that answers from another address is warned of, once
// however many searches find it there (noteElsewhere).
func TestALostFixedCubeWithAnIntervalIsSearchedFor(t *testing.T) {
	t.Parallel()
	s, fc, connected, logs, requests := fixedCube(t, time.Hour)
	// The run at the start may have warned already, where its wait heard
	// the answer; the searches after the loss repeat until one does.
	testkit.Eventually(t, "the run at the start", func() bool { return requests() >= 1 && noRunGoing(s) })
	before := requests()
	fc.Close()
	testkit.Eventually(t, "the cube lost", func() bool { return !connected() }, awayWait)
	logs.Await(t, 0, "a warning after the loss", testkit.Msg(elsewhereMsg))
	testkit.Eventually(t, "searches after the warning", func() bool { return requests() >= before+3 })
	if n := logs.Count(0, testkit.Msg(elsewhereMsg)); n != 1 {
		t.Errorf("%d warnings over the searches, want 1", n)
	}
}

// homes starts a fake Cube's TCP side on 127.0.0.2 and 127.0.0.3, one
// port, and a discovery responder on each address; the one on 127.0.0.3
// answers as a massimo proxy, the identify request alone. The loopback
// has no ARP entries, so mac gives the MACs the search sees.
func homes(t *testing.T) (port int, cubeTCP, proxyTCP *fakecube.Server, atCube, atProxy *fakecube.DiscoveryResponder) {
	t.Helper()
	greeting := recorded(t)
	cubeTCP = &fakecube.Server{Greeting: greeting}
	addr, err := cubeTCP.Start("127.0.0.2:0")
	if err != nil {
		testkit.BindOrSkip(t, err)
	}
	t.Cleanup(func() { cubeTCP.Close() })
	_, portStr, _ := net.SplitHostPort(addr)
	port, _ = strconv.Atoi(portStr)
	proxyTCP = &fakecube.Server{Greeting: greeting}
	if _, err := proxyTCP.Start("127.0.0.3:" + portStr); err != nil {
		testkit.BindOrSkip(t, err)
	}
	t.Cleanup(func() { proxyTCP.Close() })
	if atCube, err = fakecube.StartDiscovery("127.0.0.2:0", "KEQ0000100", 0x0a0107); err != nil {
		testkit.BindOrSkip(t, err)
	}
	t.Cleanup(func() { atCube.Close() })
	if atProxy, err = fakecube.StartDiscovery("127.0.0.3:0", "KEQ0000100", 0x0a0107); err != nil {
		testkit.BindOrSkip(t, err)
	}
	t.Cleanup(func() { atProxy.Close() })
	atProxy.SetIdentifyOnly(true)
	return port, cubeTCP, proxyTCP, atCube, atProxy
}

// A Cube heard from its own address and from a proxy is one found Cube,
// at its own address; configured by serial, it follows the proxy, whose
// serve holds the Cube's one LAN slot, and stays there run after run.
func TestOneSerialFromACubeAndAProxy(t *testing.T) {
	t.Parallel()
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
	log, logs := testkit.LogWatch(nil)
	ts := startServer(t, discovering(Options{CubePort: port, LookupMAC: mac, Logger: log,
		DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{atCube.Addr(), atProxy.Addr()}}), withCubes(CubeConfig{Serial: "KEQ0000100"})).ts
	proxyAddr := "127.0.0.3:" + strconv.Itoa(port)
	testkit.Eventually(t, "the cube through the proxy", func() bool {
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
	// Where discovery found it, beside where the server connects.
	if cubes[0]["found_address"] != "127.0.0.2" {
		t.Fatalf("found_address %v, want the Cube's own 127.0.0.2", cubes[0]["found_address"])
	}
	if changes := logs.Since(0, testkit.Msg("cube address from discovery")); len(changes) != 1 {
		t.Errorf("%d address changes in three runs, want 1: %v", len(changes), changes)
	}
	if cubeTCP.Connections() != 0 || proxyTCP.Connections() != 1 {
		t.Errorf("connections: %d to the Cube, %d to the proxy", cubeTCP.Connections(), proxyTCP.Connections())
	}
}

// An answer the search cannot tell, a proxy's without a MAC, is not
// followed away from the address a Cube has, even when the Cube's own
// answer is missing.
func TestAnUnknownAnswerIsNotFollowed(t *testing.T) {
	t.Parallel()
	port, cubeTCP, proxyTCP, atCube, atProxy := homes(t)
	atProxy.SetSilent(true)
	log, logs := testkit.LogWatch(nil)
	ts := startServer(t, discovering(Options{CubePort: port, Logger: log,
		DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{atCube.Addr(), atProxy.Addr()}}), withCubes(CubeConfig{Serial: "KEQ0000100"})).ts
	cubeAddr := "127.0.0.2:" + strconv.Itoa(port)
	address := func() any {
		var cubes []map[string]any
		getJSON(t, ts, "", "/api/v1/cubes", &cubes)
		if len(cubes) != 1 || cubes[0]["connected"] != true {
			return nil
		}
		return cubes[0]["address"]
	}
	testkit.Eventually(t, "the cube at its own address", func() bool { return address() == cubeAddr })
	atCube.SetSilent(true)
	atProxy.SetSilent(false)
	postDiscovery(t, ts)
	if a := address(); a != cubeAddr {
		t.Fatalf("followed an unknown answer: %v", a)
	}
	if n := logs.Count(0, testkit.Msg("cube address from discovery")); n != 1 || proxyTCP.Connections() != 0 || cubeTCP.Connections() != 1 {
		t.Errorf("%d address changes, %d connections to the proxy, %d to the Cube", n, proxyTCP.Connections(), cubeTCP.Connections())
	}
}
