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
// configured by serial only follows the address it answers from. A call
// of the API gets a run begun after it; the discovery loop shares a run
// going. A lost Cube is searched for at the loss and while it lasts
// (searchLost), since it may have taken another address.

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
	"go.metaworx.ch/massimo/internal/sharedport"
	"go.metaworx.ch/massimo/internal/store"
)

// Defaults of discovery.
const (
	// DefaultDiscoveryWait is how long a run waits for answers.
	DefaultDiscoveryWait = 3 * time.Second
	// addressRetry is how often discovery runs while a Cube configured by
	// serial has no address yet, or a Cube searchLost takes is lost; the
	// default of Options.AddressRetry.
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
	// Gateway, Netmask and DNS are the addresses the Cube uses, from its
	// answer to N in this process's latest discovery that had one; kept
	// in memory only, as DHCP.
	Gateway string   `json:"gateway,omitempty"`
	Netmask string   `json:"netmask,omitempty"`
	DNS     []string `json:"dns,omitempty"`
	// Proxies are the other addresses that answered for the Cube's
	// serial in the latest run, massimo proxies (docs/proxy.md); in
	// memory only.
	Proxies []string `json:"proxies,omitempty"`
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
// from one of this host's addresses with a configured Cube's serial, or,
// from any address, with a serial of its own that one of its proxies
// answers with (proxySerial), unless a Cube gave it (cubelan.KindCube).
// A configured Cube's serial from elsewhere is left in: the Cube itself,
// or another server's proxy of it, which the Cube may follow.
func (s *Server) ownAnswer(d cubelan.Discovered, local map[string]bool) bool {
	if s.configured(d.Serial) != nil {
		return local[d.IP.String()]
	}
	return d.Kind != cubelan.KindCube && s.proxySerial(d.Serial)
}

// proxySerial reports whether one of this server's proxies answers
// discovery with serial: a proxy running now, whenever it started (a Cube
// added or changed through the API has one serve did not start with), or
// a configured Cube's proxy, which may be starting again.
func (s *Server) proxySerial(serial string) bool {
	if serial == "" {
		return false
	}
	s.proxyMu.Lock()
	for _, pr := range s.proxies {
		if strings.EqualFold(pr.pl.DiscoverySerial, serial) {
			s.proxyMu.Unlock()
			return true
		}
	}
	s.proxyMu.Unlock()
	for _, rt := range s.runtimes() {
		if strings.EqualFold(rt.config().ProxySerial, serial) {
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
// comes while a run is going waits for it and gets its result: the
// discovery loop, whose runs need not begin after it asks.
func (s *Server) discover(ctx context.Context) (*discoveryRun, error) {
	s.discMu.Lock()
	run := s.discRun
	if run == nil {
		run = s.startRun(nil)
	}
	s.discMu.Unlock()
	return waitRun(ctx, run)
}

// discoverAfter is discover for a caller that wants a run begun after
// its call, the API's: a run going sent its requests before the call,
// and does not hear a Cube plugged in or moved just before it. While a
// run is going it waits for the run after it, which starts as that one
// ends and which every such caller that came meanwhile shares.
func (s *Server) discoverAfter(ctx context.Context) (*discoveryRun, error) {
	s.discMu.Lock()
	var run *discoveryRun
	switch {
	case s.discRun == nil:
		run = s.startRun(nil)
	case s.discNext != nil:
		run = s.discNext
	default:
		run = &discoveryRun{done: make(chan struct{})}
		s.discNext = run
	}
	s.discMu.Unlock()
	return waitRun(ctx, run)
}

// startRun starts run as the run going, a new one where it is nil, and
// returns it; its end starts the run waiting after it (discNext). discMu
// is held.
func (s *Server) startRun(run *discoveryRun) *discoveryRun {
	if run == nil {
		run = &discoveryRun{done: make(chan struct{})}
	}
	run.ranAt = time.Now()
	s.discRun = run
	s.mu.RLock()
	runCtx := s.ctx
	s.mu.RUnlock()
	if runCtx == nil {
		runCtx = context.Background()
	}
	go func() {
		// Not tied to a caller's request: others may be waiting.
		run.found, run.err = s.discoverOnce(runCtx)
		s.discMu.Lock()
		s.discRun = nil
		if next := s.discNext; next != nil {
			s.discNext = nil
			s.startRun(next)
		}
		s.discMu.Unlock()
		close(run.done)
	}()
	return run
}

// waitRun waits for run to end, or for ctx.
func waitRun(ctx context.Context, run *discoveryRun) (*discoveryRun, error) {
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
		// the discovery port if it is free, since the Cubes answer to it;
		// shared, as the responder's is (ensureResponder).
		pc, err := sharedport.ListenPacket("udp4", s.discoveryAddr())
		if err != nil {
			// The Cubes answer to this port only; another one finds none.
			s.log.Warn("discovery port in use, no discovery", "port", cubelan.DiscoveryPort, "error", err)
			return nil, errNoDiscoverySocket
		}
		rctx, cancel := context.WithCancel(ctx)
		defer cancel()
		r = proxy.NewResponder(s.partLog("proxy", ""))
		go r.Serve(rctx, pc)
	}
	answers, err := r.Discover(ctx, cubelan.SearchOptions{Targets: s.discoveryTargets(), Wait: wait, MAC: s.opts.LookupMAC})
	if err != nil && len(answers) == 0 {
		return nil, err
	}
	local := localIPs()
	now := time.Now()
	// A serial can come from several addresses, the Cube's own and a
	// proxy's: the run's answers are taken per serial, in the order the
	// serials came.
	groups := map[string][]cubelan.Discovered{}
	var serials []string
	for _, d := range answers {
		if s.ownAnswer(d, local) {
			continue
		}
		serial := strings.ToUpper(strings.TrimSpace(d.Serial))
		if _, ok := groups[serial]; !ok {
			serials = append(serials, serial)
		}
		groups[serial] = append(groups[serial], d)
	}
	var out []foundCube
	answered := map[string]bool{}
	for _, serial := range serials {
		f := s.noteSerial(ctx, groups[serial], now)
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

// note records one answer, as noteSerial does a run's answers of one
// serial.
func (s *Server) note(ctx context.Context, d cubelan.Discovered, now time.Time) foundCube {
	return s.noteSerial(ctx, []cubelan.Discovered{d}, now)
}

// noteSerial records a run's answers of one serial, in the order they
// came: a Cube new to the server, or back after a run it did not answer,
// is found (cubeFound); the entry and the store's row take the Cube's own
// answer (ownOf); a configured Cube follows its address when it was
// configured by serial only, at most one move a run (followAddress). A
// found Cube at a new address is the change of f.Address here.
func (s *Server) noteSerial(ctx context.Context, answers []cubelan.Discovered, now time.Time) foundCube {
	d := ownOf(answers)
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
	if d.MAC != "" {
		f.MAC = d.MAC
	}
	if d.DHCP != nil {
		dhcp := *d.DHCP
		f.DHCP = &dhcp
	}
	if d.Gateway != nil || d.Netmask != nil {
		f.Gateway, f.Netmask, f.DNS = ipString(d.Gateway), ipString(d.Netmask), nil
		for _, ip := range d.DNS {
			f.DNS = append(f.DNS, ip.String())
		}
	}
	f.Proxies = nil
	for _, a := range answers {
		if a.IP.String() != f.Address {
			f.Proxies = append(f.Proxies, a.IP.String())
		}
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
	for _, a := range answers {
		if a.Kind != cubelan.KindProxy && a.IP.String() == host {
			// The Cube itself answers at its client's address: a
			// connection that does not come is refused (Client.Link).
			rt.client.Heard()
			break
		}
	}
	if cfg := rt.config(); cfg.Addr != "" && !cfg.FollowDiscovery {
		if ip := net.ParseIP(host); ip != nil && !answeredFrom(answers, host) {
			s.log.Warn("cube answers discovery from another address than configured", "serial", serial, "configured", cfg.Addr, "found", cp.Address)
		}
		return cp
	}
	to := followAddress(host, answers)
	if to == "" || to == host {
		return cp
	}
	had := host != ""
	port := cubelan.CubePort
	if s.opts.CubePort != 0 {
		port = s.opts.CubePort
	}
	if rt.client.SetAddress(net.JoinHostPort(to, strconv.Itoa(port))) {
		rt.emit(event{Kind: "address_changed", Source: "cube", Serial: serial, Time: now, Summary: map[string]any{"from": host, "to": to}})
		if had {
			// Connected, or trying, to the old address: reconnect now.
			go rt.queue.Enqueue(engine.Command{Key: "refresh", Priority: engine.PriorityNormal, Reconnect: true, Moved: true}).Wait(ctx)
		}
	}
	return cp
}

// ownOf picks the Cube's own answer among a serial's: the first the
// search took for a Cube's, else the first it could not tell, else the
// first, a proxy's.
func ownOf(answers []cubelan.Discovered) cubelan.Discovered {
	for _, kind := range []cubelan.Kind{cubelan.KindCube, cubelan.KindUnknown} {
		for _, d := range answers {
			if d.Kind == kind {
				return d
			}
		}
	}
	return answers[0]
}

// ipString is ip's text, "" for none.
func ipString(ip net.IP) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}

// answeredFrom reports whether one of answers came from host.
func answeredFrom(answers []cubelan.Discovered, host string) bool {
	for _, d := range answers {
		if d.IP.String() == host {
			return true
		}
	}
	return false
}

// followAddress is where a Cube followed by discovery goes, given the
// run's answers of its serial and the address it has, "" for none yet:
// that address while it is among them, else the first proxy's, since a
// proxy answers only while its serve holds the Cube and the Cube's one
// LAN slot is then taken, else the Cube's own. An answer the search
// could not tell is taken only by a Cube without an address; "" keeps
// what it has.
func followAddress(current string, answers []cubelan.Discovered) string {
	if current != "" && answeredFrom(answers, current) {
		return current
	}
	for _, kind := range []cubelan.Kind{cubelan.KindProxy, cubelan.KindCube} {
		for _, d := range answers {
			if d.Kind == kind {
				return d.IP.String()
			}
		}
	}
	if current == "" {
		return answers[0].IP.String()
	}
	return ""
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

// searchLost reports whether a Cube is searched for while its
// connection is lost: one following discovery, which only discovery can
// move, and any where periodic discovery is on, where a fixed address
// that moved is warned of (noteSerial). Without an interval, discovery
// runs on request only, but for the Cubes only it can connect.
func (s *Server) searchLost(rt *runtime) bool {
	cfg := rt.config()
	return cfg.Addr == "" || cfg.FollowDiscovery || s.opts.DiscoverInterval > 0
}

// needSearch reports whether a Cube needs discovery now: one without an
// address, or one searchLost takes that is lost.
func (s *Server) needSearch() bool {
	for _, rt := range s.runtimes() {
		if rt.client.Address() == "" || rt.client.Lost() && s.searchLost(rt) {
			return true
		}
	}
	return false
}

// discoveryLoop runs discovery at the start and every DiscoverInterval,
// every AddressRetry (a minute) while needSearch, and when wakeDiscovery
// asks: a Cube added without an address, or one lost (pump); it is the
// only periodic discovery.
func (s *Server) discoveryLoop(ctx context.Context) {
	retry := s.opts.AddressRetry
	if retry <= 0 {
		retry = addressRetry
	}
	// next < 0 waits for wakeDiscovery only: no periodic discovery and
	// no Cube in need of it.
	next := time.Duration(0)
	if s.opts.DiscoverInterval <= 0 && !s.needSearch() {
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
		case s.needSearch():
			// One run a minute for every Cube without an address or lost
			// together.
			next = retry
		case s.opts.DiscoverInterval > 0:
			next = s.opts.DiscoverInterval
		default:
			next = -1
		}
	}
}

// wakeDiscovery runs the discovery loop now, for a Cube added or changed
// without an address, or one lost; the loop then repeats every
// AddressRetry while needSearch.
func (s *Server) wakeDiscovery() {
	select {
	case s.discWake <- struct{}{}:
	default: // a run is asked for already
	}
}

// postDiscover handles POST /api/v1/cubes/discover: one run now, or the
// one after the run going (discoverAfter); the answer lists every Cube
// that answered, configured or not.
func (s *Server) postDiscover(w http.ResponseWriter, r *http.Request) error {
	run, err := s.discoverAfter(r.Context())
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
