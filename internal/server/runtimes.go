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
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

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
		ProxyAddress: row.ProxyAddress, ProxySerial: row.ProxySerial}
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

// config returns a copy of the runtime's configuration.
func (rt *runtime) config() CubeConfig {
	rt.cfgMu.RLock()
	defer rt.cfgMu.RUnlock()
	c := rt.cfg
	c.Hidden = append([]cube.RFAddress(nil), rt.cfg.Hidden...)
	return c
}

// setConfig changes the runtime's configuration.
func (rt *runtime) setConfig(change func(*CubeConfig)) {
	rt.cfgMu.Lock()
	defer rt.cfgMu.Unlock()
	change(&rt.cfg)
}

// newRuntime makes the runtime of a Cube whose client starts at addr
// ("" waits for discovery).
func (s *Server) newRuntime(c CubeConfig, addr string) *runtime {
	opts := s.opts
	rt := &runtime{cfg: c, serial: strings.ToUpper(c.Serial), showHidden: opts.ShowHidden, last: map[cube.RFAddress]*view.State{}, subs: map[chan event]struct{}{}, push: map[chan struct{}]struct{}{},
		follow: newFollowUp(opts.FollowUpDelays, opts.FollowUpReconnectGap, opts.RefreshInterval > 0)}
	rt.client = cubelan.New(cubelan.Options{Addr: addr, PollInterval: opts.PollInterval, Logger: opts.Logger})
	rt.queue = engine.NewQueue(rt.client, opts.Queue)
	return rt
}

// register adds a runtime to the Cubes the server serves, by serial,
// or by address while the serial is not known.
func (s *Server) register(rt *runtime) {
	key := strings.ToLower(rt.serial)
	if key == "" {
		key = strings.ToLower(rt.config().Addr)
	}
	s.mu.Lock()
	s.cubes[key] = rt
	s.order = append(s.order, key)
	s.mu.Unlock()
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
	s.mu.RLock()
	parent := s.ctx
	s.mu.RUnlock()
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
	if s.opts.RefreshInterval > 0 {
		run(func() { s.refresh(ctx, rt) })
	}
	s.wg.Add(1)
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
	if s.responder != nil || s.ctx == nil {
		return s.responder
	}
	pc, err := net.ListenPacket("udp4", s.discoveryAddr())
	if err != nil {
		s.log.Warn("proxy: discovery port not available, clients must be given the address", "port", cubelan.DiscoveryPort, "error", err)
		return nil
	}
	r := proxy.NewResponder(s.log)
	s.responder = r
	ctx := s.ctx
	s.wg.Add(1)
	go func() { defer s.wg.Done(); r.Serve(ctx, pc) }()
	return r
}

// startProxy starts one proxy listener.
func (s *Server) startProxy(pl ProxyListener) {
	rt, err := s.proxyRuntime(pl)
	if err != nil {
		s.log.Error("proxy listener not started", "addr", pl.Addr, "error", err)
		return
	}
	s.mu.RLock()
	parent := s.ctx
	s.mu.RUnlock()
	if parent == nil {
		return
	}
	p := proxy.New(proxy.Options{Client: rt.client, Queue: rt.queue, DiscoverySerial: pl.DiscoverySerial, HiddenList: func() []cube.RFAddress { return rt.config().Hidden },
		ShowHidden: rt.showHidden, MaxDutyPct: s.opts.Queue.MaxDutyPct, Subscribe: rt.subscribePush, Logger: s.log})
	if pl.Discovery {
		if r := s.ensureResponder(); r != nil {
			host, _, _ := net.SplitHostPort(pl.Addr)
			if err := r.Add(p, net.ParseIP(host)); err != nil {
				s.log.Error("proxy discovery not started", "addr", pl.Addr, "error", err)
			}
		}
	}
	ctx, cancel := context.WithCancel(parent)
	pr := &proxyRun{pl: pl, rt: rt, p: p, cancel: cancel, done: make(chan struct{})}
	s.proxyMu.Lock()
	s.proxies = append(s.proxies, pr)
	s.proxyMu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(pr.done)
		if err := p.Run(ctx, pl.Addr); err != nil && ctx.Err() == nil {
			s.log.Error("proxy listener failed", "addr", pl.Addr, "error", err)
		}
	}()
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
		s.log.Info("cube proxy stopped", "addr", pr.pl.Addr, "serial", rt.serial)
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
			return
		}
		addr = ":" + strconv.Itoa(cubelan.CubePort)
	}
	addr = withCubePort(addr)
	_, port, _ := net.SplitHostPort(addr)
	pl := ProxyListener{Serial: rt.serial, Addr: addr, Discovery: port == strconv.Itoa(cubelan.CubePort), DiscoverySerial: cfg.ProxySerial}
	s.startProxy(pl)
	s.log.Info("cube proxy listening", "addr", pl.Addr, "serial", pl.Serial, "discovery", pl.Discovery, "discovery_serial", pl.DiscoverySerial)
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
	s.register(rt)
	s.startRuntime(rt)
	s.startCubeProxy(rt)
	if addr == "" {
		go s.awaitAddress(rt)
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

// awaitAddress runs discovery every addressRetry until a Cube added
// without an address has one, or is removed.
func (s *Server) awaitAddress(rt *runtime) {
	rt.runMu.Lock()
	done := rt.done
	rt.runMu.Unlock()
	if done == nil {
		return
	}
	for rt.client.Address() == "" {
		s.mu.RLock()
		ctx := s.ctx
		s.mu.RUnlock()
		if _, err := s.discover(ctx); err != nil && ctx.Err() == nil {
			s.log.Warn("discovery failed", "error", err)
		}
		if rt.client.Address() != "" {
			return
		}
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-time.After(addressRetry):
		}
	}
}
