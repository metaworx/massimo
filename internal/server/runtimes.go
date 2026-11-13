// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The runtimes and their proxy listeners come and go while the server
// runs: Run starts those of Options, and the Cube routes (cubes_api.go)
// add a Cube, change its configuration and remove it. Each runtime and
// each listener has a context of its own under Run's, so that one can
// be stopped without the others.

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/proxy"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// proxyRun is one running proxy listener.
type proxyRun struct {
	pl     ProxyListener
	rt     *runtime
	p      *proxy.Proxy
	cancel context.CancelFunc
	done   chan struct{}
}

// CubeFromRow is the configuration of a Cube row of the store.
func CubeFromRow(row store.Cube) (CubeConfig, error) {
	c := CubeConfig{Serial: row.Serial, Name: row.Name, Addr: row.Address, FollowDiscovery: row.FollowDiscovery, Source: row.Source,
		ProxyAddress: row.ProxyAddress, ProxySerial: row.ProxySerial, StoreID: row.ID}
	for _, a := range row.Hidden {
		rf, err := cube.ParseRFAddress(a)
		if err != nil {
			return CubeConfig{}, err
		}
		c.Hidden = append(c.Hidden, rf)
	}
	return c, nil
}

// hiddenStrings writes a hidden list as the API shows it.
func hiddenStrings(list []cube.RFAddress) []string {
	out := []string{}
	for _, rf := range list {
		out = append(out, rf.String())
	}
	return out
}

// config returns the runtime's configuration. Its Hidden slice is
// shared and never changed in place: setConfig installs a new one, so
// the views and the proxy, which ask for it per event and per message,
// get it without a copy.
func (rt *runtime) config() CubeConfig {
	rt.cfgMu.RLock()
	defer rt.cfgMu.RUnlock()
	return rt.cfg
}

// setConfig changes the runtime's configuration; a new hidden list is
// a new slice, never a change of the old one (see config).
func (rt *runtime) setConfig(change func(*CubeConfig)) {
	rt.cfgMu.Lock()
	defer rt.cfgMu.Unlock()
	change(&rt.cfg)
}

// newRuntime makes the runtime of a Cube whose client starts at addr
// ("" waits for discovery).
func (s *Server) newRuntime(c CubeConfig, addr string) *runtime {
	opts := s.opts
	rt := &runtime{evlog: s.opts.Logger, cfg: c, serial: strings.ToUpper(c.Serial), showHidden: opts.ShowHidden, last: map[cube.RFAddress]*view.State{}, subs: map[chan event]*subscriber{}, epoch: newEpoch(), push: map[chan struct{}]struct{}{},
		structure: make(chan struct{}, 1),
		follow:    newFollowUp(opts.FollowUpDelays, opts.FollowUpReconnectGap, opts.RefreshInterval > 0)}
	rt.client = cubelan.New(cubelan.Options{Addr: addr, PollInterval: opts.PollInterval, Logger: s.partLog("cube", rt.serial), EventLines: true})
	qopts := opts.Queue
	qopts.Observer = s.newObserver(rt)
	qopts.OnHold = rt.held
	// The queue's lines went to slog.Default() before: serve gave it no
	// logger of its own.
	if qopts.Logger == nil {
		qopts.Logger = s.partLog("queue", rt.serial)
	}
	rt.queue = engine.NewQueue(rt.client, qopts)
	rt.store = opts.Store
	if opts.Store != nil {
		rt.storeID.Store(c.StoreID)
		rt.loadOptions(context.Background())
	}
	return rt
}

// register adds a runtime to the Cubes the server serves, by serial,
// or by address while the serial is not known; a key that is taken
// already is refused, so that no runtime runs unreferenced.
func (s *Server) register(rt *runtime) error {
	key := strings.ToLower(rt.cubeSerial())
	if key == "" {
		key = strings.ToLower(rt.config().Addr)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, taken := s.cubes[key]; taken {
		return fmt.Errorf("cube %s is served already", key)
	}
	s.cubes[key] = rt
	s.order = append(s.order, key)
	return nil
}

// track counts a goroutine of the server in s.wg, unless Run has not
// started or its context is done: Run waits for s.wg after the end, and
// an Add then would race with that Wait. It returns Run's context, or
// nil when nothing may start.
func (s *Server) track() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.trackLocked()
}

// trackLocked is track with s.mu held.
func (s *Server) trackLocked() context.Context {
	if s.ctx == nil || s.ctx.Err() != nil {
		return nil
	}
	s.wg.Add(1)
	return s.ctx
}

// unregister removes a runtime under every key it has.
func (s *Server) unregister(rt *runtime) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.cubes {
		if v == rt {
			delete(s.cubes, k)
		}
	}
	kept := s.order[:0]
	for _, k := range s.order {
		if _, ok := s.cubes[k]; ok {
			kept = append(kept, k)
		}
	}
	s.order = kept
}

// startRuntime starts a runtime's connection, queue and loops under
// Run's context; before Run it does nothing, and Run starts it.
func (s *Server) startRuntime(rt *runtime) {
	parent := s.track()
	if parent == nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	rt.runMu.Lock()
	rt.cancel, rt.done = cancel, done
	rt.runMu.Unlock()
	var wg sync.WaitGroup
	run := func(f func()) {
		wg.Add(1)
		go func() { defer wg.Done(); f() }()
	}
	run(func() { rt.client.Run(ctx) })
	run(func() { rt.queue.Run(ctx) })
	run(func() { s.pump(ctx, rt) })
	run(func() { s.followUps(ctx, rt) })
	run(func() { s.backupLoop(ctx, rt) })
	if s.opts.RefreshInterval > 0 {
		run(func() { s.refresh(ctx, rt) })
	}
	go func() { defer s.wg.Done(); wg.Wait(); close(done) }()
}

// stopRuntime stops a runtime's goroutines and waits for them.
func (s *Server) stopRuntime(rt *runtime) {
	rt.runMu.Lock()
	cancel, done := rt.cancel, rt.done
	rt.runMu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

// ensureResponder returns the proxies' discovery socket, opening it on
// first use; nil when the discovery port is taken.
func (s *Server) ensureResponder() *proxy.Responder {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.responder != nil || s.ctx == nil || s.ctx.Err() != nil {
		return s.responder
	}
	pc, err := net.ListenPacket("udp4", s.discoveryAddr())
	if err != nil {
		s.log.Warn("proxy: discovery port not available, clients must be given the address", "port", cubelan.DiscoveryPort, "error", err)
		return nil
	}
	r := proxy.NewResponder(s.partLog("proxy", ""))
	s.responder = r
	ctx := s.trackLocked()
	go func() { defer s.wg.Done(); r.Serve(ctx, pc) }()
	return r
}

// startProxy starts one proxy listener and reports whether it listens.
// The port is bound here, so that a port in use is an error now, and
// only a listener that listens is in s.proxies and in the discovery
// replies; one that fails later is taken out of both.
func (s *Server) startProxy(pl ProxyListener) bool {
	rt, err := s.proxyRuntime(pl)
	if err != nil {
		s.log.Error("proxy listener not started", "addr", pl.Addr, "error", err)
		return false
	}
	ln, err := net.Listen("tcp", pl.Addr)
	if err != nil {
		s.log.Error("proxy listener not started", "addr", pl.Addr, "serial", rt.cubeSerial(), "error", err)
		return false
	}
	parent := s.track()
	if parent == nil {
		ln.Close()
		return false
	}
	p := proxy.New(proxy.Options{Client: rt.client, Queue: rt.queue, DiscoverySerial: pl.DiscoverySerial, HiddenList: func() []cube.RFAddress { return rt.config().Hidden },
		ShowHidden: rt.showHidden, MaxDutyPct: s.opts.Queue.MaxDutyPct, Subscribe: rt.subscribePush, OnWrite: rt.proxyWrote, Logger: s.partLog("proxy", rt.cubeSerial()),
		OnEvent: rt.proxyEvent, Command: s.proxyCommand(rt), Virtual: func() []proxy.Virtual { return s.proxyVirtuals(rt) }})
	var r *proxy.Responder
	if pl.Discovery {
		if r = s.ensureResponder(); r != nil {
			host, _, _ := net.SplitHostPort(pl.Addr)
			if err := r.Add(p, net.ParseIP(host)); err != nil {
				s.log.Error("proxy discovery not started", "addr", pl.Addr, "error", err)
				r = nil
			}
		}
	}
	ctx, cancel := context.WithCancel(parent)
	pr := &proxyRun{pl: pl, rt: rt, p: p, cancel: cancel, done: make(chan struct{})}
	s.proxyMu.Lock()
	s.proxies = append(s.proxies, pr)
	s.proxyMu.Unlock()
	s.log.Info("cube proxy listening", "addr", pl.Addr, "serial", rt.cubeSerial(), "discovery", pl.Discovery && r != nil, "discovery_serial", pl.DiscoverySerial)
	go func() {
		defer s.wg.Done()
		defer close(pr.done)
		if err := p.ServeTCP(ctx, ln); err != nil && ctx.Err() == nil {
			s.log.Error("proxy listener failed", "addr", pl.Addr, "error", err)
			s.dropProxy(pr)
		}
	}()
	return true
}

// dropProxy takes a listener that failed out of s.proxies and out of
// the discovery replies.
func (s *Server) dropProxy(pr *proxyRun) {
	s.proxyMu.Lock()
	kept := s.proxies[:0]
	for _, x := range s.proxies {
		if x != pr {
			kept = append(kept, x)
		}
	}
	s.proxies = kept
	s.proxyMu.Unlock()
	s.mu.RLock()
	r := s.responder
	s.mu.RUnlock()
	if r != nil {
		r.Remove(pr.p)
	}
	pr.cancel()
}

// stopProxies stops the proxy listeners of a runtime and waits until
// their ports are free.
func (s *Server) stopProxies(rt *runtime) {
	s.proxyMu.Lock()
	var stop []*proxyRun
	kept := s.proxies[:0]
	for _, pr := range s.proxies {
		if pr.rt == rt {
			stop = append(stop, pr)
		} else {
			kept = append(kept, pr)
		}
	}
	s.proxies = kept
	s.proxyMu.Unlock()
	s.mu.RLock()
	r := s.responder
	s.mu.RUnlock()
	for _, pr := range stop {
		pr.cancel()
		if r != nil {
			r.Remove(pr.p)
		}
		<-pr.done
		s.log.Info("cube proxy stopped", "addr", pr.pl.Addr, "serial", rt.cubeSerial())
	}
}

// proxyListening returns the addresses a runtime's proxies listen on.
func (s *Server) proxyListening(rt *runtime) []string {
	s.proxyMu.Lock()
	defer s.proxyMu.Unlock()
	var out []string
	for _, pr := range s.proxies {
		if pr.rt == rt {
			out = append(out, pr.pl.Addr)
		}
	}
	return out
}

// proxyAddress is the proxy address configured for a runtime.
func (s *Server) proxyAddress(rt *runtime) string { return rt.config().ProxyAddress }

// startCubeProxy starts the listener of a Cube configured through the
// API: on its proxy address, or on the Cube port when it is the only
// Cube and nothing else listens; with -proxy on the command line, the
// listeners are left as they are.
func (s *Server) startCubeProxy(rt *runtime) {
	if s.opts.ProxyFixed {
		return
	}
	cfg := rt.config()
	addr := cfg.ProxyAddress
	if addr == "" {
		s.proxyMu.Lock()
		others := len(s.proxies)
		s.proxyMu.Unlock()
		if others > 0 || len(s.runtimes()) != 1 {
			s.log.Warn("cube served without a proxy: other cubes are served and this one has no proxy address", "serial", rt.cubeSerial())
			return
		}
		addr = ":" + strconv.Itoa(cubelan.CubePort)
	}
	addr = withCubePort(addr)
	_, port, _ := net.SplitHostPort(addr)
	pl := ProxyListener{Serial: rt.cubeSerial(), Addr: addr, Discovery: port == strconv.Itoa(cubelan.CubePort), DiscoverySerial: cfg.ProxySerial}
	s.startProxy(pl)
}

// withCubePort adds the Cube's port to an address that has none.
func withCubePort(addr string) string {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return net.JoinHostPort(addr, strconv.Itoa(cubelan.CubePort))
	}
	return addr
}

// addCube starts serving a Cube row: its runtime, its proxy, and a
// search for its address when it has none.
func (s *Server) addCube(row store.Cube) (*runtime, error) {
	cfg, err := CubeFromRow(row)
	if err != nil {
		return nil, err
	}
	addr := cfg.Addr
	if addr == "" {
		if f, ok := s.foundInfo(row.Serial); ok && f.Address != "" {
			addr = net.JoinHostPort(f.Address, strconv.Itoa(s.cubePort()))
		}
	}
	rt := s.newRuntime(cfg, addr)
	if err := s.register(rt); err != nil {
		return nil, err
	}
	s.startRuntime(rt)
	s.startCubeProxy(rt)
	if addr == "" {
		s.wakeDiscovery()
	}
	return rt, nil
}

// removeCube stops serving a Cube.
func (s *Server) removeCube(rt *runtime) {
	s.stopProxies(rt)
	s.stopRuntime(rt)
	s.unregister(rt)
}

// cubePort is the TCP port of a Cube whose address comes from discovery.
func (s *Server) cubePort() int {
	if s.opts.CubePort != 0 {
		return s.opts.CubePort
	}
	return cubelan.CubePort
}
