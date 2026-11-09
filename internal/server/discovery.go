// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Discovery of Cubes on the LAN. serve holds the discovery port for its
// proxies' answers, and the Cubes answer to that port, so a run goes
// through the proxy responder's socket, or through a socket of its own
// when serve answers no discovery. The answers of serve's own proxies
// come back as well and are left out: an answer from one of this host's
// addresses with one of serve's serials. A Cube that is not configured
// is listed as found while it answers: from its first answer to the end
// of the first complete run it does not answer. cubeFound and cubeGone
// are the one place each where a Cube becomes found and gone. One
// configured by serial only follows the address it answers from.

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/proxy"
	"go.metaworx.ch/massimo/internal/store"
)

// Defaults of discovery.
const (
	// DefaultDiscoveryWait is how long a run waits for answers.
	DefaultDiscoveryWait = 3 * time.Second
	// addressRetry is how often discovery runs while a Cube configured by
	// serial has no address yet.
	addressRetry = time.Minute
)

// foundState is where a found Cube stands with discovery, in memory
// only.
type foundState int

const (
	// foundLoaded is a Cube read from the store at the start that this
	// process has not heard yet: not listed, and not announced when it
	// answers, since it was found before.
	foundLoaded foundState = iota
	// foundPresent is a Cube that answered the latest complete run, or
	// the run going.
	foundPresent
	// foundGone is a Cube that did not answer the latest complete run:
	// not listed, and announced again when it answers.
	foundGone
)

// foundCube is a Cube that answered discovery.
type foundCube struct {
	Serial    string `json:"serial"`
	Address   string `json:"address"` // the IP it answered from
	MAC       string `json:"mac,omitempty"`
	RFAddress string `json:"rf_address"`
	Firmware  string `json:"firmware"`
	// DHCP says whether the Cube takes its address by DHCP, from its
	// answer to c in this process's latest discovery that had one; nil
	// until then. It is kept in memory only, not in the store.
	DHCP *bool `json:"dhcp,omitempty"`
	// Configured says whether the server serves the Cube when the copy
	// is handed out. It is set on the copies only, never kept on the
	// entry, since a Cube is added and removed without discovery
	// hearing it: a flag kept from the last answer, or from the store at
	// the start, listed a removed Cube as configured.
	Configured bool      `json:"configured"`
	FoundAt    time.Time `json:"found_at"`
	LastSeen   time.Time `json:"last_seen"`
	// state is the entry's: kept on the map's entry, and in a copy only
	// as it was when copied.
	state foundState
}

// discoveryRun is one run of discovery; callers that come while it
// runs share its result.
type discoveryRun struct {
	done  chan struct{}
	ranAt time.Time
	found []foundCube
	err   error
}

// errNoDiscoverySocket is the error of a run without the discovery port.
var errNoDiscoverySocket = errors.New("the discovery port is in use by another program")

// discoveryAddr is the local address of a discovery socket of its own.
func (s *Server) discoveryAddr() string {
	if s.opts.DiscoveryAddr != "" {
		return s.opts.DiscoveryAddr
	}
	return ":" + strconv.Itoa(cubelan.DiscoveryPort)
}

// discoveryTargets are where a request goes.
func (s *Server) discoveryTargets() []*net.UDPAddr {
	if len(s.opts.DiscoveryTargets) == 0 {
		return cubelan.DiscoveryTargets()
	}
	var out []*net.UDPAddr
	for _, t := range s.opts.DiscoveryTargets {
		if a, err := net.ResolveUDPAddr("udp4", t); err == nil {
			out = append(out, a)
		}
	}
	return out
}

// runtimes returns every configured Cube once.
func (s *Server) runtimes() []*runtime {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*runtime
	seen := map[*runtime]bool{}
	for _, key := range s.order {
		if rt := s.cubes[key]; !seen[rt] {
			seen[rt] = true
			out = append(out, rt)
		}
	}
	return out
}

// configured returns the runtime of a serial, or nil.
func (s *Server) configured(serial string) *runtime {
	for _, rt := range s.runtimes() {
		if strings.EqualFold(rt.cubeSerial(), serial) || strings.EqualFold(rt.config().Serial, serial) {
			return rt
		}
	}
	return nil
}

// ownAnswer reports whether an answer is one of this server's proxies':
// from one of this host's addresses, with a serial the server answers
// discovery with.
func (s *Server) ownAnswer(d cubelan.Discovered, local map[string]bool) bool {
	if !local[d.IP.String()] {
		return false
	}
	if s.configured(d.Serial) != nil {
		return true
	}
	for _, pl := range s.opts.Proxy {
		if strings.EqualFold(pl.DiscoverySerial, d.Serial) {
			return true
		}
	}
	return false
}

// localIPs are this host's addresses.
func localIPs() map[string]bool {
	out := map[string]bool{}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			out[n.IP.String()] = true
		}
	}
	return out
}

// discover runs one discovery and records what it found. A caller that
// comes while a run is going waits for it and gets its result.
func (s *Server) discover(ctx context.Context) (*discoveryRun, error) {
	s.discMu.Lock()
	run := s.discRun
	if run == nil {
		run = &discoveryRun{done: make(chan struct{}), ranAt: time.Now()}
		s.discRun = run
		s.mu.RLock()
		runCtx := s.ctx
		s.mu.RUnlock()
		if runCtx == nil {
			runCtx = context.Background()
		}
		go func() {
			// Not tied to the caller's request: others may be waiting.
			run.found, run.err = s.discoverOnce(runCtx)
			s.discMu.Lock()
			s.discRun = nil
			s.discMu.Unlock()
			close(run.done)
		}()
	}
	s.discMu.Unlock()
	select {
	case <-run.done:
		return run, run.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// discoverOnce is one run.
func (s *Server) discoverOnce(ctx context.Context) ([]foundCube, error) {
	wait := s.opts.DiscoveryWait
	if wait <= 0 {
		wait = DefaultDiscoveryWait
	}
	s.mu.RLock()
	r := s.responder
	s.mu.RUnlock()
	if r == nil {
		// serve answers no discovery: a socket of its own for this run, on
		// the discovery port if it is free, since the Cubes answer to it.
		pc, err := net.ListenPacket("udp4", s.discoveryAddr())
		if err != nil {
			// The Cubes answer to this port only; another one finds none.
			s.log.Warn("discovery port in use, no discovery", "port", cubelan.DiscoveryPort, "error", err)
			return nil, errNoDiscoverySocket
		}
		rctx, cancel := context.WithCancel(ctx)
		defer cancel()
		r = proxy.NewResponder(s.log)
		go r.Serve(rctx, pc)
	}
	answers, err := r.Discover(ctx, s.discoveryTargets(), wait)
	if err != nil && len(answers) == 0 {
		return nil, err
	}
	local := localIPs()
	now := time.Now()
	var out []foundCube
	answered := map[string]bool{}
	for _, d := range answers {
		if s.ownAnswer(d, local) {
			continue
		}
		f := s.note(ctx, d, now)
		answered[f.Serial] = true
		out = append(out, f)
	}
	if err == nil {
		// A complete run: who did not answer it is gone. One cut short
		// cannot tell a Cube gone from one not heard yet.
		s.settle(answered)
	}
	return out, nil
}

// note records one answer: a Cube new to the server, or back after a
// run it did not answer, is found (cubeFound); a configured one follows
// its address when it was configured by serial only. A found Cube at a
// new address is the change of f.Address here.
func (s *Server) note(ctx context.Context, d cubelan.Discovered, now time.Time) foundCube {
	serial := strings.ToUpper(strings.TrimSpace(d.Serial))
	rt := s.configured(serial)
	s.foundMu.Lock()
	f, known := s.found[serial]
	if !known {
		f = &foundCube{Serial: serial, FoundAt: now}
		s.found[serial] = f
	}
	arrived := !known || f.state == foundGone
	f.state = foundPresent
	f.Address, f.RFAddress, f.Firmware, f.LastSeen = d.IP.String(), d.RFAddress.String(), d.Firmware, now
	if mac := cubelan.LookupMAC(d.IP); mac != "" {
		f.MAC = mac
	}
	if d.DHCP != nil {
		dhcp := *d.DHCP
		f.DHCP = &dhcp
	}
	cp := *f
	cp.Configured = rt != nil
	s.foundMu.Unlock()
	if s.opts.Store != nil {
		if _, _, err := s.opts.Store.NoteDiscovered(ctx, store.Discovered{Serial: serial, Address: cp.Address, MAC: cp.MAC, RFAddress: cp.RFAddress, Firmware: cp.Firmware, At: now}); err != nil {
			s.log.Warn("store: found cube not recorded", "serial", serial, "error", err)
		}
	}

	if arrived {
		s.cubeFound(cp)
	}
	if rt == nil {
		return cp
	}
	host, _, _ := net.SplitHostPort(rt.client.Address())
	if host == cp.Address {
		return cp
	}
	if cfg := rt.config(); cfg.Addr != "" && !cfg.FollowDiscovery {
		if ip := net.ParseIP(host); ip != nil {
			s.log.Warn("cube answers discovery from another address than configured", "serial", serial, "configured", cfg.Addr, "found", cp.Address)
		}
		return cp
	}
	had := host != ""
	port := cubelan.CubePort
	if s.opts.CubePort != 0 {
		port = s.opts.CubePort
	}
	if rt.client.SetAddress(net.JoinHostPort(cp.Address, strconv.Itoa(port))) {
		s.log.Info("cube address from discovery", "serial", serial, "ip", cp.Address, "was", host)
		rt.emit(event{Kind: "address_changed", Source: "cube", Serial: serial, Time: now, Summary: map[string]any{"from": host, "to": cp.Address}})
		if had {
			// Connected, or trying, to the old address: reconnect now.
			go rt.queue.Enqueue(engine.Command{Key: "refresh", Priority: engine.PriorityNormal, Reconnect: true, Moved: true}).Wait(ctx)
		}
	}
	return cp
}

// settle ends a complete run, answered holding the serials that
// answered it: a Cube that answered the run before and not this one is
// gone (cubeGone), and one read from the store that this process has
// not heard is gone without a word, or a Cube gone long ago would be
// reported at every start.
func (s *Server) settle(answered map[string]bool) {
	var gone []foundCube
	s.foundMu.Lock()
	for serial, f := range s.found {
		if answered[serial] {
			continue
		}
		if f.state == foundPresent {
			gone = append(gone, *f)
		}
		f.state = foundGone
	}
	s.foundMu.Unlock()
	sort.Slice(gone, func(i, j int) bool { return gone[i].Serial < gone[j].Serial })
	for _, f := range gone {
		f.Configured = s.configured(f.Serial) != nil
		s.cubeGone(f)
	}
}

// cubeFound is the one place where a Cube becomes found: new to the
// server, or back after a complete run it did not answer. One that is
// not configured is logged and announced, an event of the server's, kept
// once and sent on every Cube's stream (cube_found). It runs without
// foundMu held.
func (s *Server) cubeFound(f foundCube) {
	if f.Configured {
		return
	}
	s.log.Info("cube found on the LAN, not configured", "serial", f.Serial, "ip", f.Address, "mac", f.MAC)
	s.logServer(event{Kind: "cube_found", Source: "cube", Serial: f.Serial, Cube: &f, Time: f.LastSeen})
	s.pushNotify(notify.Event{Kind: "cube_found", Cube: f.Serial, Values: map[string]any{"address": f.Address}, Time: f.LastSeen})
}

// cubeGone is the one place where a Cube becomes gone: it answered the
// run before and not the latest complete one. One that is not
// configured leaves the found list, and is logged; its row in the store
// keeps when it was last seen. It runs without foundMu held.
func (s *Server) cubeGone(f foundCube) {
	if f.Configured {
		return
	}
	s.log.Info("cube gone from the LAN, not configured", "serial", f.Serial, "ip", f.Address, "mac", f.MAC)
	s.logServer(event{Kind: "cube_gone", Source: "cube", Serial: f.Serial, Cube: &f, Time: time.Now()})
}

// unconfigured lists the Cubes found but not configured, by serial:
// those that answered the latest complete run, or the run going.
func (s *Server) unconfigured() []foundCube {
	s.foundMu.Lock()
	defer s.foundMu.Unlock()
	var out []foundCube
	for _, f := range s.found {
		if f.state == foundPresent && s.configured(f.Serial) == nil {
			c := *f
			c.Configured = false
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Serial < out[j].Serial })
	return out
}

// foundInfo returns what discovery knows about a configured Cube.
func (s *Server) foundInfo(serial string) (foundCube, bool) {
	s.foundMu.Lock()
	defer s.foundMu.Unlock()
	f, ok := s.found[strings.ToUpper(serial)]
	if !ok {
		return foundCube{}, false
	}
	return *f, true
}

// discoveryLoop runs discovery at the start and every DiscoverInterval,
// every minute while a Cube configured by serial has no address, and
// when wakeDiscovery asks; it is the only periodic discovery.
func (s *Server) discoveryLoop(ctx context.Context) {
	needAddress := func() bool {
		for _, rt := range s.runtimes() {
			if rt.client.Address() == "" {
				return true
			}
		}
		return false
	}
	// next < 0 waits for wakeDiscovery only: no periodic discovery and
	// every Cube has its address.
	next := time.Duration(0)
	if s.opts.DiscoverInterval <= 0 && !needAddress() {
		next = -1
	}
	for {
		var tick <-chan time.Time
		if next >= 0 {
			tick = time.After(next)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick:
		case <-s.discWake:
		}
		if _, err := s.discover(ctx); err != nil && ctx.Err() == nil {
			s.log.Warn("discovery failed", "error", err)
		}
		switch {
		case needAddress():
			// One run a minute for every Cube without an address together.
			next = addressRetry
		case s.opts.DiscoverInterval > 0:
			next = s.opts.DiscoverInterval
		default:
			next = -1
		}
	}
}

// wakeDiscovery runs the discovery loop now, for a Cube added or changed
// without an address; the loop then repeats every addressRetry until
// every Cube has one.
func (s *Server) wakeDiscovery() {
	select {
	case s.discWake <- struct{}{}:
	default: // a run is asked for already
	}
}

// postDiscover handles POST /api/v1/cubes/discover: one run now, or the
// one already going; the answer lists every Cube that answered,
// configured or not.
func (s *Server) postDiscover(w http.ResponseWriter, r *http.Request) error {
	run, err := s.discover(r.Context())
	if errors.Is(err, errNoDiscoverySocket) {
		return fail(http.StatusServiceUnavailable, "discovery: %v", err)
	}
	if err != nil {
		return fail(http.StatusInternalServerError, "discovery: %v", err)
	}
	found := run.found
	if found == nil {
		found = []foundCube{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ran_at": run.ranAt, "found": found})
	return nil
}
