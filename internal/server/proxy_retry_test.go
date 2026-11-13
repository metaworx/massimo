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
)

// quickRetry is the tests' wait between a proxy listener's tries.
var quickRetry = [2]time.Duration{5 * time.Millisecond, 20 * time.Millisecond}

// countLines counts the log lines that carry every one of parts.
func countLines(log string, parts ...string) int {
	n := 0
	for _, l := range strings.Split(log, "\n") {
		all := l != ""
		for _, p := range parts {
			all = all && strings.Contains(l, p)
		}
		if all {
			n++
		}
	}
	return n
}

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
	var logs syncBuffer
	_, ts, s, _ := startStored(t, Options{proxyRetry: quickRetry, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := taken.Addr().String()
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_address":"`+addr+`"}`)
	if code != 200 || out["proxy_listening"] != nil {
		taken.Close()
		t.Fatalf("put %d %v", code, out)
	}
	// Several tries go by while the port is held.
	eventually(t, "a few tries", func() bool {
		s.proxyMu.Lock()
		defer s.proxyMu.Unlock()
		return len(s.proxies) == 1 && s.proxies[0].tries >= 3
	})
	taken.Close()
	eventually(t, "the proxy to listen", func() bool { return len(cubeListening(t, s)) == 1 })
	if m := greetingAt(t, addr); m.Type != 'H' {
		t.Fatalf("greeting %q", m.Wire())
	}
	log := logs.String()
	if n := countLines(log, "level=ERROR", `msg="proxy listener not started"`, "addr="+addr); n != 1 {
		t.Errorf("%d Error lines for the held port, want 1:\n%s", n, log)
	}
	if n := countLines(log, `msg="cube proxy listening"`, "addr="+addr, "tries="); n != 1 {
		t.Errorf("no listening line with its tries:\n%s", log)
	}
}

// A proxy address the host does not carry yet is tried again until the
// host has it; a new error is logged at Error again.
func TestProxyRetriedUntilTheAddressIsThere(t *testing.T) {
	var mu sync.Mutex
	failWith := errors.New("listen tcp 192.0.2.10:62910: bind: cannot assign requested address")
	listen := func(addr string) (net.Listener, error) {
		mu.Lock()
		defer mu.Unlock()
		if failWith != nil {
			return nil, failWith
		}
		return net.Listen("tcp", addr)
	}
	var logs syncBuffer
	_, ts, s, _ := startStored(t, Options{proxyRetry: quickRetry, proxyListen: listen, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	addr := freeTCP(t)
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
	eventually(t, "three tries", tries(3))
	mu.Lock()
	failWith = errors.New("listen tcp " + addr + ": bind: address already in use")
	mu.Unlock()
	eventually(t, "six tries", tries(6))
	s.proxyMu.Lock()
	pr := *s.proxies[0]
	s.proxyMu.Unlock()
	if pr.listening || !strings.Contains(pr.err, "already in use") || pr.since.IsZero() || !pr.next.After(pr.since) {
		t.Fatalf("slot while failing: listening %v, err %q, since %v, next %v", pr.listening, pr.err, pr.since, pr.next)
	}
	mu.Lock()
	failWith = nil
	mu.Unlock()
	eventually(t, "the proxy to listen", func() bool { return len(cubeListening(t, s)) == 1 })
	greetingAt(t, addr)
	if n := countLines(logs.String(), "level=ERROR", `msg="proxy listener not started"`); n != 2 {
		t.Errorf("%d Error lines for two errors, want 2:\n%s", n, logs.String())
	}
	s.proxyMu.Lock()
	defer s.proxyMu.Unlock()
	if p := s.proxies[0]; p.err != "" || p.tries != 0 || !p.since.IsZero() {
		t.Errorf("slot after listening: err %q, tries %d, since %v", p.err, p.tries, p.since)
	}
}

// A slot that is trying stops with a new proxy address and with the
// Cube's removal: nothing listens on its address afterwards.
func TestProxyRetryStops(t *testing.T) {
	_, ts, s, _ := startStored(t, Options{proxyRetry: quickRetry})
	for _, step := range []struct{ method, body string }{
		{"PUT", `{"proxy_address":"%s"}`},
		{"DELETE", ""},
	} {
		taken, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := taken.Addr().String()
		if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_address":"`+addr+`"}`); code != 200 {
			t.Fatalf("put %d %v", code, out)
		}
		body := step.body
		if body != "" {
			body = strings.Replace(body, "%s", freeTCP(t), 1)
		}
		if code, out := do(t, ts, step.method, "/api/v1/cubes/KEQ0000100", body); code != 200 {
			t.Fatalf("%s %d %v", step.method, code, out)
		}
		taken.Close()
		time.Sleep(5 * quickRetry[1])
		if conn, err := net.Dial("tcp", addr); err == nil {
			conn.Close()
			t.Fatalf("after %s, the old address %s listens", step.method, addr)
		}
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
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := taken.Addr().String()
	_, _, s := startWith(t, Options{proxyRetry: quickRetry, ProxyFixed: true, Proxy: []ProxyListener{{Serial: "KEQ0000100", Addr: addr}}})
	if l := cubeListening(t, s); len(l) != 0 {
		taken.Close()
		t.Fatalf("listening on a taken port: %v", l)
	}
	taken.Close()
	greetingAt(t, addr)
}
