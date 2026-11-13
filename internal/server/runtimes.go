// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The runtimes and their proxy listeners come and go while the server
// runs: Run starts those of Options, and the Cube routes (cubes_api.go)
// add a Cube, change its configuration and remove it. Each runtime and
// each listener has a context of its own under Run's, so that one can
// be stopped without the others. A listener that cannot listen - an
// address the host does not carry yet, a port in use - is tried again
// with a backoff until it listens or is stopped (docs/proxy.md).

import (
	"context"
	"fmt"
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

// proxyRun is one proxy listener, from its first try until it is
// stopped: listening, or trying again after a failure. The fields after
// done are under s.proxyMu.
type proxyRun struct {
	pl     ProxyListener
	rt     *runtime
	p      *proxy.Proxy
	cancel context.CancelFunc
	done   chan struct{}

	listening bool
	err       string    // the last failure while not listening
	since     time.Time // the first failure of those since it last listened
	next      time.Time // the next try
	tries     int       // the failed tries since it last listened
}

// The waits between a proxy listener's tries: the first, doubled after
// each failure up to the longest (Options.proxyRetry for the tests).
const (
	proxyRetryFirst = time.Second
	proxyRetryMax   = 30 * time.Second
)

// listenProxy opens a proxy's listener: net.Listen, or the tests'
// Options.proxyListen, which fails as an address the host does not
// carry does.
func (s *Server) listenProxy(addr string) (net.Listener, error) {
	if s.opts.proxyListen != nil {
		return s.opts.proxyListen(addr)
	}
	return net.Listen("tcp", addr)
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
	copts := cubelan.Options{Addr: addr, PollInterval: opts.PollInterval, Hold: opts.CubeHold, Logger: s.partLog("cube", rt.serial), EventLines: true}
	if opts.cubeOptions != nil {
		opts.cubeOptions(&copts)
	}
	rt.client = cubelan.New(copts)
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

// startProxy starts one proxy listener and reports whether it listens
// now. The port is bound here once, so that the caller learns at once
// whether it listens; a listener that cannot listen yet is in s.proxies
// all the same, and tried again (runProxy). Only one that listens is in
// the discovery replies and among the listening ones (proxyListening).
func (s *Server) startProxy(pl ProxyListener) bool {
	rt, err := s.proxyRuntime(pl)
	if err != nil {
		s.log.Error("proxy listener not started", "addr", pl.Addr, "error", err)
		return false
	}
	parent := s.track()
	if parent == nil {
		return false
	}
	p := proxy.New(proxy.Options{Client: rt.client, Queue: rt.queue, DiscoverySerial: pl.DiscoverySerial, HiddenList: func() []cube.RFAddress { return rt.config().Hidden },
		ShowHidden: rt.showHidden, MaxDutyPct: s.opts.Queue.MaxDutyPct, Subscribe: rt.subscribePush, OnWrite: rt.proxyWrote, Logger: s.partLog("proxy", rt.cubeSerial()),
		OnEvent: rt.proxyEvent, Command: s.proxyCommand(rt), Virtual: func() []proxy.Virtual { return s.proxyVirtuals(rt) }})
	ctx, cancel := context.WithCancel(parent)
	pr := &proxyRun{pl: pl, rt: rt, p: p, cancel: cancel, done: make(chan struct{})}
	s.proxyMu.Lock()
	s.proxies = append(s.proxies, pr)
	s.proxyMu.Unlock()
	first, _ := s.proxyRetry()
	ln, err := s.listenProxy(pl.Addr)
	if err != nil {
		s.proxyFailed(pr, "proxy listener not started", err, first)
	} else {
		s.proxyUp(pr)
	}
	go s.runProxy(ctx, pr, ln)
	return err == nil
}

// proxyRetry returns the first and the longest wait between a proxy
// listener's tries.
func (s *Server) proxyRetry() (first, longest time.Duration) {
	first, longest = s.opts.proxyRetry[0], s.opts.proxyRetry[1]
	if first <= 0 {
		first = proxyRetryFirst
	}
	if longest <= 0 {
		longest = proxyRetryMax
	}
	return first, max(first, longest)
}

// runProxy serves a proxy listener while it listens, and tries to
// listen again after a failure, with a wait that doubles from the first
// to the longest, until ctx ends. ln is the listener of startProxy's
// try, nil where it failed.
func (s *Server) runProxy(ctx context.Context, pr *proxyRun, ln net.Listener) {
	defer s.wg.Done()
	defer close(pr.done)
	first, longest := s.proxyRetry()
	wait := first
	for {
		if ln != nil {
			err := s.serveProxy(ctx, pr, ln)
			if ctx.Err() != nil {
				return
			}
			wait = first
			s.proxyFailed(pr, "proxy listener failed", err, wait)
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
		var err error
		if ln, err = s.listenProxy(pr.pl.Addr); err != nil {
			wait = min(wait*2, longest)
			s.proxyFailed(pr, "proxy listener not started", err, wait)
			continue
		}
		if ctx.Err() != nil {
			ln.Close()
			return
		}
		s.proxyUp(pr)
	}
}

// proxyUp marks a proxy listener as listening, adds it to the discovery
// replies where it answers discovery, and logs it.
func (s *Server) proxyUp(pr *proxyRun) {
	discovery := false
	if pr.pl.Discovery {
		if r := s.ensureResponder(); r != nil {
			host, _, _ := net.SplitHostPort(pr.pl.Addr)
			if err := r.Add(pr.p, net.ParseIP(host)); err != nil {
				s.log.Error("proxy discovery not started", "addr", pr.pl.Addr, "error", err)
			} else {
				discovery = true
			}
		}
	}
	s.proxyMu.Lock()
	tries := pr.tries
	pr.listening, pr.err, pr.since, pr.next, pr.tries = true, "", time.Time{}, time.Time{}, 0
	s.proxyMu.Unlock()
	attrs := []any{"addr", pr.pl.Addr, "serial", pr.rt.cubeSerial(), "discovery", discovery, "discovery_serial", pr.pl.DiscoverySerial}
	if tries > 0 {
		attrs = append(attrs, "tries", tries+1)
	}
	s.log.Info("cube proxy listening", attrs...)
}

// serveProxy serves a listening proxy listener until ctx ends or the
// listener fails, then takes it out of the discovery replies.
func (s *Server) serveProxy(ctx context.Context, pr *proxyRun, ln net.Listener) error {
	sctx, cancel := context.WithCancel(ctx)
	err := pr.p.ServeTCP(sctx, ln)
	cancel()
	s.proxyMu.Lock()
	pr.listening = false
	s.proxyMu.Unlock()
	s.mu.RLock()
	r := s.responder
	s.mu.RUnlock()
	if r != nil {
		r.Remove(pr.p)
	}
	return err
}

// proxyFailed records a proxy listener's failure and when it is tried
// again, and logs it: at Error the first failure and one whose error
// differs from the last, at Debug a failure that repeats the last.
func (s *Server) proxyFailed(pr *proxyRun, msg string, err error, wait time.Duration) {
	now := time.Now()
	text := err.Error()
	s.proxyMu.Lock()
	repeat := pr.tries > 0 && pr.err == text
	if pr.tries == 0 {
		pr.since = now
	}
	pr.tries++
	pr.err, pr.next = text, now.Add(wait)
	tries := pr.tries
	s.proxyMu.Unlock()
	if repeat {
		s.log.Debug("proxy listener retry", "addr", pr.pl.Addr, "serial", pr.rt.cubeSerial(), "error", err, "tries", tries, "retry_in", wait)
		return
	}
	s.log.Error(msg, "addr", pr.pl.Addr, "serial", pr.rt.cubeSerial(), "error", err, "retry_in", wait)
}

// stopProxies stops the proxy listeners of a runtime, those trying to
// listen too, and waits until their ports are free.
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
		if pr.rt == rt && pr.listening {
			out = append(out, pr.pl.Addr)
		}
	}
	return out
}

// proxyFailure is a proxy listener that does not listen, as a Cube in
// the API shows it: its address, the last error, since when it fails,
// when it is tried next, and the tries so far.
type proxyFailure struct {
	Addr    string    `json:"addr"`
	Error   string    `json:"error"`
	Since   time.Time `json:"since"`
	NextTry time.Time `json:"next_try"`
	Tries   int       `json:"tries"`
}

// proxyStatus is a proxy listener as GET /api/v1/system shows it: its
// Cube, its address, whether it listens, and, where it does not, the
// fields of proxyFailure.
type proxyStatus struct {
	Serial    string     `json:"serial,omitempty"`
	Addr      string     `json:"addr"`
	Listening bool       `json:"listening"`
	Error     string     `json:"error,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	NextTry   *time.Time `json:"next_try,omitempty"`
	Tries     int        `json:"tries,omitempty"`
}

// failure returns a slot's failure, with s.proxyMu held; false while it
// listens, or before its first try has ended.
func (pr *proxyRun) failure() (proxyFailure, bool) {
	if pr.listening || pr.tries == 0 {
		return proxyFailure{}, false
	}
	return proxyFailure{Addr: pr.pl.Addr, Error: pr.err, Since: pr.since, NextTry: pr.next, Tries: pr.tries}, true
}

// proxyFailing returns a runtime's proxy listeners that do not listen.
func (s *Server) proxyFailing(rt *runtime) []proxyFailure {
	s.proxyMu.Lock()
	defer s.proxyMu.Unlock()
	var out []proxyFailure
	for _, pr := range s.proxies {
		if f, ok := pr.failure(); ok && pr.rt == rt {
			out = append(out, f)
		}
	}
	return out
}

// proxyStatuses returns every proxy listener, listening or not.
func (s *Server) proxyStatuses() []proxyStatus {
	s.proxyMu.Lock()
	defer s.proxyMu.Unlock()
	out := []proxyStatus{}
	for _, pr := range s.proxies {
		st := proxyStatus{Serial: pr.rt.cubeSerial(), Addr: pr.pl.Addr, Listening: pr.listening}
		if f, ok := pr.failure(); ok {
			st.Error, st.Since, st.NextTry, st.Tries = f.Error, &f.Since, &f.NextTry, f.Tries
		}
		out = append(out, st)
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
