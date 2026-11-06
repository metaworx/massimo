// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net"
	"sort"
	"sync"
	"testing"
	"time"
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
		if rt.serial == "KEQ0537249" {
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
