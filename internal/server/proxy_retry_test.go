// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/testkit"
)

// quickRetry is the tests' wait between a proxy listener's tries.
var quickRetry = [2]time.Duration{5 * time.Millisecond, 20 * time.Millisecond}

// cubeListening reads the proxy addresses GET /api/v1/cubes lists for
// the fake Cube.
func cubeListening(t *testing.T, s *Server) []string {
	t.Helper()
	for _, rt := range s.runtimes() {
		if rt.cubeSerial() == "KEQ0000100" {
			return s.proxyListening(rt)
		}
	}
	t.Fatal("no runtime for KEQ0000100")
	return nil
}

// A proxy address whose port is taken is tried again, and listens once
// the port is free; the failure is logged once at Error, not per try.
func TestProxyRetriedUntilThePortIsFree(t *testing.T) {
	t.Parallel()
	log, logs := testkit.LogWatch(nil)
	ports := &heldPorts{}
	r := startServer(t, stored(), withOptions(Options{proxyRetry: quickRetry, Logger: log}), listenOn(ports))
	ts, s := r.ts, r.s
	addr, free := ports.taken(t)
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_address":"`+addr+`"}`)
	if code != 200 || out["proxy_listening"] != nil {
		t.Fatalf("put %d %v", code, out)
	}
	// Several tries go by while the port is held.
	testkit.Eventually(t, "a few tries", func() bool {
		s.proxyMu.Lock()
		defer s.proxyMu.Unlock()
		return len(s.proxies) == 1 && s.proxies[0].tries >= 3
	})
	free()
	testkit.Eventually(t, "the proxy to listen", func() bool { return len(cubeListening(t, s)) == 1 })
	if m := greetingAt(t, addr); m.Type != 'H' {
		t.Fatalf("greeting %q", m.Wire())
	}
	if got := logs.Since(0, testkit.MsgAt(slog.LevelError, "proxy listener not started", "addr", addr)); len(got) != 1 {
		t.Errorf("%d Error lines for the held port, want 1: %v", len(got), got)
	}
	listening := logs.Since(0, testkit.Msg("cube proxy listening", "addr", addr))
	withTries := 0
	for _, l := range listening {
		if _, ok := l.Attrs["tries"]; ok {
			withTries++
		}
	}
	if withTries != 1 {
		t.Errorf("no listening line with its tries: %v", listening)
	}
}

// A proxy address the host does not carry yet is tried again until the
// host has it; a new error is logged at Error again.
func TestProxyRetriedUntilTheAddressIsThere(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	failWith := errors.New("listen tcp 192.0.2.10:62910: bind: cannot assign requested address")
	ports := &heldPorts{}
	addr := ports.hold(t)
	listen := func(addr string) (net.Listener, error) {
		mu.Lock()
		defer mu.Unlock()
		if failWith != nil {
			return nil, failWith
		}
		return ports.listen(addr)
	}
	log, logs := testkit.LogWatch(nil)
	r := startServer(t, stored(), withOptions(Options{proxyRetry: quickRetry, proxyListen: listen, Logger: log}))
	ts, s := r.ts, r.s
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_address":"`+addr+`"}`); code != 200 || out["proxy_listening"] != nil {
		t.Fatalf("put %d %v", code, out)
	}
	tries := func(n int) func() bool {
		return func() bool {
			s.proxyMu.Lock()
			defer s.proxyMu.Unlock()
			return len(s.proxies) == 1 && s.proxies[0].tries >= n
		}
	}
	testkit.Eventually(t, "three tries", tries(3))
	mu.Lock()
	failWith = errors.New("listen tcp " + addr + ": bind: address already in use")
	mu.Unlock()
	testkit.Eventually(t, "six tries", tries(6))
	s.proxyMu.Lock()
	pr := *s.proxies[0]
	s.proxyMu.Unlock()
	if pr.listening || !strings.Contains(pr.err, "already in use") || pr.since.IsZero() || !pr.next.After(pr.since) {
		t.Fatalf("slot while failing: listening %v, err %q, since %v, next %v", pr.listening, pr.err, pr.since, pr.next)
	}
	mu.Lock()
	failWith = nil
	mu.Unlock()
	testkit.Eventually(t, "the proxy to listen", func() bool { return len(cubeListening(t, s)) == 1 })
	greetingAt(t, addr)
	if got := logs.Since(0, testkit.MsgAt(slog.LevelError, "proxy listener not started")); len(got) != 2 {
		t.Errorf("%d Error lines for two errors, want 2: %v", len(got), got)
	}
	s.proxyMu.Lock()
	defer s.proxyMu.Unlock()
	if p := s.proxies[0]; p.err != "" || p.tries != 0 || !p.since.IsZero() {
		t.Errorf("slot after listening: err %q, tries %d, since %v", p.err, p.tries, p.since)
	}
}

// A slot that is trying stops with a new proxy address and with the
// Cube's removal: it tries its address no more afterwards. The test
// holds the address throughout, so that no other program takes it, and
// counts the slot's listens there.
func TestProxyRetryStops(t *testing.T) {
	t.Parallel()
	ports := &heldPorts{}
	r := startServer(t, stored(), withOptions(Options{proxyRetry: quickRetry}), listenOn(ports))
	ts, s := r.ts, r.s
	for _, step := range []struct{ method, body string }{
		{"PUT", `{"proxy_address":"%s"}`},
		{"DELETE", ""},
	} {
		addr, _ := ports.taken(t) // never given: each listen there fails
		if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_address":"`+addr+`"}`); code != 200 {
			t.Fatalf("put %d %v", code, out)
		}
		testkit.Eventually(t, "the slot to try "+addr+" again", func() bool { return ports.listens(addr) >= 2 })
		body := step.body
		if body != "" {
			body = strings.Replace(body, "%s", ports.hold(t), 1)
		}
		if code, out := do(t, ts, step.method, "/api/v1/cubes/KEQ0000100", body); code != 200 {
			t.Fatalf("%s %d %v", step.method, code, out)
		}
		// The end is a fixed time here, where Never's end is otherwise an
		// event: the stop waits for the slot's end before the answer comes
		// (stopProxies), and the test holds the address, so no listen
		// belongs there after the answer at all. The window only catches a
		// stop that does not wait, whose next try comes within a few of
		// the slot's longest waits; it cannot fail a slow runner.
		tried, after := ports.listens(addr), time.Now().Add(5*quickRetry[1])
		testkit.Never(t, "a listen at "+addr+" after "+step.method, func() bool { return ports.listens(addr) > tried }, func() bool {
			return time.Now().After(after) && getJSON(t, ts, "", "/api/v1/cubes", nil) == 200
		})
		s.proxyMu.Lock()
		for _, pr := range s.proxies {
			if pr.pl.Addr == addr {
				t.Errorf("after %s, a slot for %s is left", step.method, addr)
			}
		}
		s.proxyMu.Unlock()
	}
}

// A listener of -proxy (Options.Proxy) is tried again as well.
func TestProxyFlagListenerRetried(t *testing.T) {
	t.Parallel()
	ports := &heldPorts{}
	addr, free := ports.taken(t)
	s := startServer(t, withOptions(Options{proxyRetry: quickRetry, ProxyFixed: true, Proxy: []ProxyListener{{Serial: "KEQ0000100", Addr: addr}}}), listenOn(ports)).s
	if l := cubeListening(t, s); len(l) != 0 {
		t.Fatalf("listening on a taken port: %v", l)
	}
	free()
	greetingAt(t, addr)
}

// The API shows a listener that does not listen: the Cube's
// proxy_failing, from the PUT's answer on, and System Info's
// cube_proxies; both clear once it listens.
func TestProxyFailingInTheAPI(t *testing.T) {
	t.Parallel()
	ports := &heldPorts{}
	r := startServer(t, stored(), withOptions(Options{proxyRetry: quickRetry}), listenOn(ports))
	ts, s := r.ts, r.s
	addr, free := ports.taken(t)
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_address":"`+addr+`"}`)
	if code != 200 {
		t.Fatalf("put %d %v", code, out)
	}
	failing, _ := out["proxy_failing"].([]any)
	if len(failing) != 1 {
		t.Fatalf("proxy_failing %v", out["proxy_failing"])
	}
	// At least the PUT's own try: the slot's next ones, 5 ms apart, can
	// come before the answer on a loaded runner, as System Info's below.
	f := failing[0].(map[string]any)
	if tries, _ := f["tries"].(float64); f["addr"] != addr || f["error"] == "" || f["since"] == nil || f["next_try"] == nil || tries < 1 {
		t.Fatalf("proxy_failing[0] %v", f)
	}
	var sys systemView
	getJSON(t, ts, "", "/api/v1/system", &sys)
	if len(sys.CubeProxies) != 1 {
		t.Fatalf("cube_proxies %+v", sys.CubeProxies)
	}
	if p := sys.CubeProxies[0]; p.Serial != "KEQ0000100" || p.Addr != addr || p.Listening || p.Error == "" || p.Since == nil || p.NextTry == nil || p.Tries < 1 {
		t.Fatalf("cube_proxies[0] %+v", p)
	}
	free()
	testkit.Eventually(t, "the proxy to listen", func() bool { return len(cubeListening(t, s)) == 1 })
	_, out = do(t, ts, "GET", "/api/v1/cubes/KEQ0000100", "")
	if out["proxy_failing"] != nil {
		t.Errorf("proxy_failing once listening: %v", out["proxy_failing"])
	}
	sys = systemView{}
	getJSON(t, ts, "", "/api/v1/system", &sys)
	if p := sys.CubeProxies[0]; !p.Listening || p.Error != "" || p.Since != nil || p.NextTry != nil || p.Tries != 0 {
		t.Errorf("cube_proxies[0] once listening %+v", p)
	}
}

// System Info names the serial a proxy answers discovery with, the
// Cube's proxy serial where one is set, and none for a listener that
// answers no discovery.
func TestProxyDiscoverySerialInSystem(t *testing.T) {
	t.Parallel()
	ports := &heldPorts{}
	answering, silent := ports.hold(t), ports.hold(t)
	ts := startServer(t, withOptions(Options{ProxyFixed: true, DiscoveryAddr: "127.0.0.1:0", Proxy: []ProxyListener{
		{Serial: "KEQ0000100", Addr: answering, Discovery: true, DiscoverySerial: "KEQ0000999"},
		{Serial: "KEQ0000100", Addr: silent}}}), listenOn(ports)).ts
	var sys systemView
	getJSON(t, ts, "", "/api/v1/system", &sys)
	got := map[string]string{}
	for _, p := range sys.CubeProxies {
		got[p.Addr] = p.DiscoverySerial
	}
	if got[answering] != "KEQ0000999" || got[silent] != "" || len(got) != 2 {
		t.Fatalf("discovery serials %v, want KEQ0000999 at %s and none at %s", got, answering, silent)
	}
}
