// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// Two POSTs for one serial at the same time: one 201, one 409, and one
// runtime for the Cube.
func TestConcurrentAddCube(t *testing.T) {
	r := startServer(t, stored())
	ts, s, st := r.ts, r.s, r.st
	before, _, _ := st.Cube(context.Background(), "KEQ0000100")
	if code, _ := do(t, ts, "DELETE", "/api/v1/cubes/KEQ0000100", ""); code != 200 {
		t.Fatal(code)
	}
	body := `{"serial":"KEQ0000100","address":"` + before.Address + `"}`
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
		if rt.cubeSerial() == "KEQ0000100" {
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
	t.Parallel()
	ports := &heldPorts{}
	ts := startServer(t, stored(), listenOn(ports)).ts
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_address":"`+taken.Addr().String()+`"}`)
	if code != 200 || out["proxy_listening"] != nil {
		t.Fatalf("put %d %v", code, out)
	}
	free := ports.hold(t)
	if code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_address":"`+free+`"}`); code != 200 || len(out["proxy_listening"].([]any)) != 1 {
		t.Fatalf("put %d %v", code, out)
	}
	greetingAt(t, free)
}

// Once Run's context is done, nothing new starts.
func TestNothingStartsAfterRun(t *testing.T) {
	t.Parallel()
	s := New(Options{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	testkit.Eventually(t, "Run's start", func() bool { return s.track() != nil })
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
	t.Parallel()
	// The Cube comes up only once the handlers read: its port is held,
	// and the client's first dial waits for the greeting.
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	addr := held.Addr().String()
	fc := &fakecube.Server{Greeting: recorded(t)}
	defer fc.Close()
	ts := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")), withCubes(CubeConfig{Addr: addr})).ts
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
	fc.Serve(held)
	testkit.Eventually(t, "the serial learned", func() bool {
		var cubes []cubeView
		getJSON(t, ts, "secret", "/api/v1/cubes", &cubes)
		for _, c := range cubes {
			if c.Serial == "KEQ0000100" && c.Connected {
				return true
			}
		}
		return false
	})
}

// A Cube served by its address that reports a serial another runtime
// serves already, one added by serial through the API meanwhile, does
// not take that runtime's place: it stops, as register refuses a second
// runtime for a serial, and the one added keeps the serial.
func TestLearnedSerialServedAlready(t *testing.T) {
	t.Parallel()
	// The Cube comes up only once the serial was added: its port is
	// held, and the client's first dial waits for the greeting.
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	addr := held.Addr().String()
	fc := &fakecube.Server{Greeting: recorded(t)}
	defer fc.Close()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")), withCubes(CubeConfig{Addr: addr}))
	ts, s := r.ts, r.s
	lookup := func(key string) *runtime {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.cubes[key]
	}
	byAddr := lookup(strings.ToLower(addr))
	if code, text := callAPI(t, ts, "POST", "/api/v1/cubes", "secret", `{"serial":"KEQ0000100","address":"`+addr+`"}`); code != 201 {
		t.Fatalf("add: %d %s", code, text)
	}
	bySerial := lookup("keq0000100")
	if byAddr == nil || bySerial == nil || bySerial == byAddr {
		t.Fatalf("runtimes %p %p", byAddr, bySerial)
	}
	fc.Serve(held)
	testkit.Within(5*time.Second, func() bool {
		return byAddr.cubeSerial() != "" && lookup(strings.ToLower(addr)) == nil
	})
	if got := lookup("keq0000100"); got != bySerial {
		t.Fatalf("the serial's runtime is the one by address (%p), not the one added (%p)", got, bySerial)
	}
	if lookup(strings.ToLower(addr)) != nil {
		t.Fatal("the runtime by address still served")
	}
}

// A listener whose start found its runtime before removeCube took that
// runtime out does not start on it: the check and the listener's place
// in s.proxies are under proxyMu, where removeCube marks the runtime,
// so that no proxy listens for a Cube no longer served.
func TestProxyNotStartedOnRemovedCube(t *testing.T) {
	t.Parallel()
	ports := &heldPorts{}
	r := startServer(t, listenOn(ports))
	pl := ProxyListener{Serial: "KEQ0000100", Addr: ports.hold(t)}
	rt, err := r.s.proxyRuntime(pl) // the start's lookup, before the removal
	if err != nil || rt != r.rt {
		t.Fatalf("lookup: %p %v", rt, err)
	}
	r.s.removeCube(rt)
	if r.s.startProxyOn(rt, pl) {
		t.Fatal("a proxy started on a removed cube")
	}
	if st := r.s.proxyStatuses(); len(st) != 0 {
		t.Fatalf("listeners after the removal: %+v", st)
	}
}

// Run starts the proxy listeners before the Cubes' connections: the
// pump removes a runtime only once it has connected, so it finds that
// runtime's listeners to stop (plainServedAlready), where a listener
// started after the removal would fail its lookup. The listens and the
// dials are taken in their order through the server's hooks.
func TestRunListensBeforeDialing(t *testing.T) {
	t.Parallel()
	ports := &heldPorts{}
	addr := ports.hold(t)
	var mu sync.Mutex
	var order []string
	note := func(what string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, what)
	}
	startServer(t, withOptions(Options{Proxy: []ProxyListener{{Addr: addr}}}), editOptions(func(o *Options) {
		o.proxyListen = func(a string) (net.Listener, error) { note("listen " + a); return ports.listen(a) }
		o.cubeOptions = func(c *cubelan.Options) {
			c.Dialer = func(ctx context.Context, a string) (net.Conn, error) {
				note("dial")
				var d net.Dialer
				return d.DialContext(ctx, "tcp", a)
			}
		}
	}))
	mu.Lock()
	defer mu.Unlock()
	if len(order) < 2 || order[0] != "listen "+addr || order[1] != "dial" {
		t.Fatalf("listens and dials: %q", order)
	}
}
