// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
)

// Two POSTs for one serial at the same time: one 201, one 409, and one
// runtime for the Cube.
func TestConcurrentAddCube(t *testing.T) {
	_, ts, s, st := startStored(t, Options{})
	before, _, _ := st.Cube(context.Background(), "KEQ0537249")
	if code, _ := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0537249", ""); code != 200 {
		t.Fatal(code)
	}
	body := `{"serial":"KEQ0537249","address":"` + before.Address + `"}`
	codes := make([]int, 2)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func() { defer wg.Done(); codes[i], _ = do(t, ts, "POST", "/api/v1/cubes", body) }()
	}
	wg.Wait()
	sort.Ints(codes)
	if codes[0] != 201 || codes[1] != 409 {
		t.Fatalf("codes %v", codes)
	}
	n := 0
	for _, rt := range s.runtimes() {
		if rt.cubeSerial() == "KEQ0537249" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d runtimes for the Cube", n)
	}
}

// A proxy address whose port is taken starts nothing: it is not listed
// as listening, and the server goes on.
func TestProxyPortInUse(t *testing.T) {
	_, ts, _, _ := startStored(t, Options{})
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249", `{"proxy_address":"`+taken.Addr().String()+`"}`)
	if code != 200 || out["proxy_listening"] != nil {
		t.Fatalf("put %d %v", code, out)
	}
	free := freeTCP(t)
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0537249", `{"proxy_address":"`+free+`"}`); code != 200 || len(out["proxy_listening"].([]any)) != 1 {
		t.Fatalf("put %d %v", code, out)
	}
	greetingAt(t, free)
}

// Once Run's context is done, nothing new starts.
func TestNothingStartsAfterRun(t *testing.T) {
	s := New(Options{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	for s.track() == nil {
		time.Sleep(time.Millisecond)
	}
	s.wg.Done() // the probe above was counted
	cancel()
	<-done
	if s.track() != nil {
		t.Fatal("a goroutine was counted after the end of Run")
	}
	if s.startProxy(ProxyListener{Addr: "127.0.0.1:0"}) {
		t.Fatal("a proxy started after the end of Run")
	}
}

// A Cube served by its address alone learns its serial when it connects,
// while a handler reads the serial (go test -race).
func TestSerialLearnedWhileRead(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	// The Cube comes up only once the handlers read: its address is
	// reserved, and the client's first dial fails.
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := free.Addr().String()
	free.Close()
	fc := &fakecube.Server{Greeting: greeting}
	defer fc.Close()
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Cubes = []CubeConfig{{Addr: addr}}
	opts.Queue = engine.Options{Gap: time.Millisecond}
	s := New(opts)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/cubes/"+addr, strings.NewReader("{}"))
				req.Header.Set("Authorization", "Bearer secret")
				if resp, err := http.DefaultClient.Do(req); err == nil {
					resp.Body.Close()
				}
			}
		}()
	}
	defer func() { close(stop); wg.Wait() }()
	time.Sleep(200 * time.Millisecond)
	if _, err := fc.Start(addr); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		var cubes []cubeView
		getJSON(t, ts, "secret", "/api/v1/cubes", &cubes)
		for _, c := range cubes {
			if c.Serial == "KEQ0537249" && c.Connected {
				return
			}
		}
	}
	t.Fatal("the serial was not learned")
}

// A Cube served by its address that reports a serial another runtime
// serves already, one added by serial through the API meanwhile, does
// not take that runtime's place: it stops, as register refuses a second
// runtime for a serial, and the one added keeps the serial.
func TestLearnedSerialServedAlready(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := free.Addr().String()
	free.Close()
	fc := &fakecube.Server{Greeting: greeting}
	defer fc.Close()
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Cubes = []CubeConfig{{Addr: addr}}
	opts.Queue = engine.Options{Gap: time.Millisecond}
	s := New(opts)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	lookup := func(key string) *runtime {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.cubes[key]
	}
	byAddr := lookup(strings.ToLower(addr))
	if code, text := callAPI(t, ts, "POST", "/api/v1/cubes", "secret", `{"serial":"KEQ0537249","address":"`+addr+`"}`); code != 201 {
		t.Fatalf("add: %d %s", code, text)
	}
	bySerial := lookup("keq0537249")
	if byAddr == nil || bySerial == nil || bySerial == byAddr {
		t.Fatalf("runtimes %p %p", byAddr, bySerial)
	}
	if _, err := fc.Start(addr); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if byAddr.cubeSerial() != "" && lookup(strings.ToLower(addr)) == nil {
			break
		}
	}
	if got := lookup("keq0537249"); got != bySerial {
		t.Fatalf("the serial's runtime is the one by address (%p), not the one added (%p)", got, bySerial)
	}
	if lookup(strings.ToLower(addr)) != nil {
		t.Fatal("the runtime by address still served")
	}
}
