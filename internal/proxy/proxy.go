// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package proxy makes a Massimo server look like a MAX! Cube on the LAN.
// The real Cube serves a single TCP client, so every other program (the
// original software, FHEM, openHAB, the massimo commands) connects to
// the proxy instead: reads are answered from the server's snapshot of
// the Cube, writes go through the command queue to the real Cube and the
// Cube's own answers are relayed. The proxy also answers the UDP
// discovery request, so clients find it like a Cube.
package proxy

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// Options configure a Proxy.
type Options struct {
	Client *cubelan.Client
	Queue  *engine.Queue
	// DiscoverySerial is the serial used in UDP discovery replies; empty
	// means the Cube's real serial. The TCP greeting always carries the
	// real serial. The original software keeps only the first discovery
	// answer per serial, so a different serial here lets the user pick
	// the proxy next to the real Cube.
	DiscoverySerial string
	// MaxDutyPct is the duty cycle at which radio commands from clients
	// are answered as discarded without being sent (default 95).
	MaxDutyPct int
	// Subscribe, when set, returns a channel that receives a signal
	// whenever the Cube reported new device state, so connected clients
	// get an L line pushed as the real Cube does, and a function to end
	// the subscription.
	Subscribe func() (<-chan struct{}, func())
	// ReplyTimeout bounds how long a client waits for the Cube's answer
	// to a passed-through command (default one minute).
	ReplyTimeout time.Duration
	// Hidden lists devices hidden from clients, in addition to those of
	// cube.HiddenRoom: they are left out of the greeting, the L line and
	// the C lines, and put back, with their rooms, into every metadata
	// write a client sends, so the Cube keeps them exactly as they are.
	// Room 255 is hidden too; other rooms stay visible, empty.
	Hidden []cube.RFAddress
	// HiddenList, when set, is asked for the hidden list instead of
	// Hidden at every use, so that the list can change while the proxy
	// runs.
	HiddenList func() []cube.RFAddress
	// ShowHidden shows the hidden devices and room 255 instead.
	ShowHidden bool
	// OnWrite, when set, is called after each radio write (s:) of a
	// client that the Cube took, not discarded, with the command's
	// header, so that the server can follow the write up as it does
	// its own.
	OnWrite func(cube.RadioCommand)
	Logger  *slog.Logger
}

// Proxy serves Cube-protocol clients over TCP and discovery over UDP.
type Proxy struct {
	opts Options
	log  *slog.Logger

	mu    sync.Mutex
	conns map[*session]struct{}
}

type session struct {
	conn net.Conn
	wmu  sync.Mutex

	pmu       sync.Mutex
	stopWatch func() // ends this client's watch of a running pairing
}

// watchPairing replaces the client's pairing watch.
func (s *session) watchPairing(stop func()) {
	s.pmu.Lock()
	old := s.stopWatch
	s.stopWatch = stop
	s.pmu.Unlock()
	if old != nil {
		old()
	}
}

// New creates a proxy for one Cube.
func New(opts Options) *Proxy {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.MaxDutyPct == 0 {
		opts.MaxDutyPct = 95
	}
	if opts.ReplyTimeout == 0 {
		opts.ReplyTimeout = time.Minute
	}
	return &Proxy{opts: opts, log: opts.Logger, conns: map[*session]struct{}{}}
}

// Run listens on tcpAddr for Cube-protocol clients until ctx ends or
// the listener fails. Discovery is served by a Responder shared by every
// proxy of a host, see NewResponder.
func (p *Proxy) Run(ctx context.Context, tcpAddr string) error {
	ln, err := net.Listen("tcp", tcpAddr)
	if err != nil {
		return err
	}
	return p.ServeTCP(ctx, ln)
}

// ServeTCP accepts clients on ln until ctx ends.
func (p *Proxy) ServeTCP(ctx context.Context, ln net.Listener) error {
	if p.opts.Subscribe != nil {
		ch, cancel := p.opts.Subscribe()
		defer cancel()
		go p.pushLoop(ctx, ch)
	}
	go func() {
		<-ctx.Done()
		ln.Close()
		p.mu.Lock()
		for s := range p.conns {
			s.conn.Close()
		}
		p.mu.Unlock()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		go p.serve(ctx, conn)
	}
}

// pushLoop writes the current L line to every client whenever the Cube
// reported a state change.
func (p *Proxy) pushLoop(ctx context.Context, ch <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			snap := p.opts.Client.Snapshot()
			l, ok := snap.DeviceListLineExcluding(p.hidden(snap.Metadata()))
			if !ok {
				continue
			}
			p.mu.Lock()
			for s := range p.conns {
				s.write(l)
			}
			p.mu.Unlock()
		}
	}
}

// write sends one line to the client, serialised per connection.
func (s *session) write(m cube.Message) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := s.conn.Write([]byte(m.Wire()))
	return err
}

// serve handles one client connection: greeting, then one command per
// line until q: or the connection ends.
func (p *Proxy) serve(ctx context.Context, conn net.Conn) {
	s := &session{conn: conn}
	remote := conn.RemoteAddr().String()
	defer func() {
		conn.Close()
		s.watchPairing(nil)
		p.mu.Lock()
		delete(p.conns, s)
		p.mu.Unlock()
		p.log.Info("proxy: client disconnected", "remote", remote)
	}()
	// The greeting needs the Cube's data; wait a little for the first one.
	select {
	case <-p.opts.Client.Ready():
	case <-ctx.Done():
		return
	case <-time.After(15 * time.Second):
		p.log.Warn("proxy: cube not connected, refusing client", "remote", remote)
		return
	}
	p.log.Info("proxy: client connected", "remote", remote)
	for _, m := range p.greeting() {
		if err := s.write(m); err != nil {
			return
		}
	}
	p.mu.Lock()
	p.conns[s] = struct{}{}
	p.mu.Unlock()

	r := cube.NewReader(conn)
	for {
		conn.SetReadDeadline(time.Now().Add(24 * time.Hour))
		line, err := r.ReadRaw()
		if err != nil {
			return
		}
		m, err := cube.ParseLine(line)
		if err != nil {
			p.log.Debug("proxy: ignoring malformed line", "remote", remote, "error", err)
			continue
		}
		if m.Type == 'q' {
			// Let the client close first, as it expects from the Cube;
			// a server-side close while it still reads shows up as an
			// exception in the original software.
			conn.SetReadDeadline(time.Now().Add(time.Second))
			r.ReadRaw()
			return
		}
		if m.Type == 'n' || m.Type == 'x' {
			p.pairing(ctx, s, m, remote)
			continue
		}
		if reply, ok := p.local(m); ok {
			if reply.Type != 0 {
				if err := s.write(reply); err != nil {
					return
				}
			}
			continue
		}
		for _, reply := range p.passthrough(ctx, m, remote) {
			if err := s.write(reply); err != nil {
				return
			}
		}
	}
}

// greeting builds what a fresh connection to the Cube receives: H with
// a new connection id and the current budget and clock, then M, C… and L.
func (p *Proxy) greeting() []cube.Message {
	snap := p.opts.Client.Snapshot()
	md := snap.Metadata()
	lines := snap.LinesExcluding(p.hidden(md), p.hiddenRooms(md))
	for i, m := range lines {
		if m.Type == 'H' {
			lines[i] = p.hello(snap.Hello())
		}
	}
	return lines
}

// hello builds the H line for a new client from the Cube's greeting with
// a fresh connection id, the current budget and the local clock.
func (p *Proxy) hello(h *cube.Hello) cube.Message {
	f := append([]string(nil), h.Fields...)
	for len(f) < 9 {
		f = append(f, "")
	}
	var id [4]byte
	rand.Read(id[:])
	f[4] = hex.EncodeToString(id[:])
	duty, slots := h.DutyCycle, h.FreeMemorySlots
	if b := p.opts.Client.Budget(); !b.UpdatedAt.IsZero() {
		duty, slots = b.DutyCyclePct, b.FreeMemorySlots
	}
	f[5] = fmt.Sprintf("%02x", duty)
	f[6] = fmt.Sprintf("%02x", slots)
	now := time.Now()
	f[7] = fmt.Sprintf("%02x%02x%02x", now.Year()-2000, int(now.Month()), now.Day())
	f[8] = fmt.Sprintf("%02x%02x", now.Hour(), now.Minute())
	return cube.Message{Type: 'H', Data: strings.Join(f, ",")}
}

// local answers the commands that never need the Cube. The returned
// message has type 0 when nothing is to be written.
func (p *Proxy) local(m cube.Message) (cube.Message, bool) {
	snap := p.opts.Client.Snapshot()
	hidden := p.hidden(snap.Metadata())
	switch m.Type {
	case 'l':
		if l, ok := snap.DeviceListLineExcluding(hidden); ok {
			return l, true
		}
		return cube.Message{}, true
	case 'c':
		rf, err := cube.ParseRFAddress(strings.TrimSpace(m.Data))
		if err != nil || hidden[rf] {
			return cube.Message{}, true
		}
		if c, ok := snap.ConfigLine(rf); ok {
			return c, true
		}
	}
	return cube.Message{}, false
}

// passthrough sends one client command to the Cube through the queue and
// returns the Cube's replies.
func (p *Proxy) passthrough(ctx context.Context, m cube.Message, remote string) []cube.Message {
	if m.Type == 's' || m.Type == 'r' {
		if b := p.opts.Client.Budget(); !b.UpdatedAt.IsZero() && b.DutyCyclePct >= p.opts.MaxDutyPct {
			p.log.Warn("proxy: duty cycle exhausted, answering client with a discard", "remote", remote, "command", m.String(), "duty_pct", b.DutyCyclePct)
			return []cube.Message{{Type: 'S', Data: fmt.Sprintf("%02x,1,%02x", b.DutyCyclePct, b.FreeMemorySlots)}}
		}
	}
	lines := []cube.Message{m}
	if m.Type == 't' && !p.opts.ShowHidden {
		kept, ok := p.withoutHidden(m, remote)
		if !ok {
			// The Cube acknowledges t: with an empty A line.
			return []cube.Message{{Type: 'A'}}
		}
		lines = []cube.Message{kept}
	}
	if m.Type == 'm' && !p.opts.ShowHidden {
		if merged, ok := p.withHidden(m); ok {
			lines = merged
		}
	}
	p.log.Debug("proxy: passing command to the cube", "remote", remote, "command", m.String())
	wctx, cancel := context.WithTimeout(ctx, p.opts.ReplyTimeout)
	defer cancel()
	// A metadata write is mirrored into the snapshot, which the Cube
	// does not echo it to.
	w := cubeio.Writer{Client: p.opts.Client, Queue: p.opts.Queue}
	res := w.Do(wctx, engine.Command{Priority: engine.PriorityNormal, Passthrough: true, Lines: lines})
	if res.Err != nil {
		p.log.Warn("proxy: command failed", "remote", remote, "command", m.String(), "error", res.Err)
		return res.Replies
	}
	if m.Type == 's' && !res.Discarded && p.opts.OnWrite != nil {
		if rc, err := cube.ParseRadioCommand(m.Data); err == nil {
			p.opts.OnWrite(rc)
		} else {
			p.log.Debug("proxy: radio command not understood, not followed up", "remote", remote, "command", m.String(), "error", err)
		}
	}
	if m.Type == 'c' {
		// A device the server did not know yet, such as one just paired.
		for _, r := range res.Replies {
			if r.Type == 'C' {
				p.opts.Client.ApplyLocal(r)
			}
		}
	}
	// The client sent one line and expects one answer.
	if len(res.Replies) > 1 {
		return res.Replies[len(res.Replies)-1:]
	}
	return res.Replies
}

// pairing handles n: and x: from a client. Inclusion mode runs through
// the server (cubeio.Writer.StartPairing), so that every N line reaches
// the client however late the Cube sends it, and the server's own
// device list learns the new devices; a client that sends n: while
// pairing already runs joins it. x: ends pairing for everyone. Neither
// line has a reply of its own; the Cube's N lines, including the empty
// one at the end, are relayed as they come.
func (p *Proxy) pairing(ctx context.Context, s *session, m cube.Message, remote string) {
	w := cubeio.Writer{Client: p.opts.Client, Queue: p.opts.Queue}
	wctx, cancel := context.WithTimeout(ctx, p.opts.ReplyTimeout)
	defer cancel()
	if m.Type == 'x' {
		if err := w.StopPairing(wctx); err != nil {
			p.log.Warn("proxy: stopping pairing failed", "remote", remote, "error", err)
		}
		return
	}
	timeout := 60 * time.Second
	if secs, err := strconv.ParseUint(strings.TrimSpace(m.Data), 16, 16); err == nil && secs > 0 {
		timeout = time.Duration(secs) * time.Second
	}
	timeout = min(timeout, cubeio.MaxPairingTime)
	lines, stop, err := w.StartPairing(wctx, timeout)
	if errors.Is(err, cubelan.ErrInclusionActive) {
		lines, stop = p.opts.Client.WatchInclusion()
		err = nil
		p.log.Info("proxy: client joins the running pairing", "remote", remote)
	}
	if err != nil {
		p.log.Warn("proxy: pairing not started", "remote", remote, "error", err)
		return
	}
	p.log.Info("proxy: pairing for a client", "remote", remote, "seconds", int(timeout.Seconds()))
	s.watchPairing(stop)
	go func() {
		for l := range lines {
			if s.write(l) != nil {
				return
			}
		}
	}()
}

// withoutHidden removes the hidden devices from a client's t: (delete
// devices) command and logs the refusal: a client that never saw them
// must not delete them, which would also make the Cube send them a
// factory reset. It returns false when nothing is left to send; a t:
// that cannot be decoded is refused as a whole, since it might name a
// hidden device.
func (p *Proxy) withoutHidden(m cube.Message, remote string) (cube.Message, bool) {
	force, addrs, err := cube.ParseDeleteDevices(m.Data)
	if err != nil {
		p.log.Warn("proxy: delete command not understood, refused", "remote", remote, "command", m.String(), "error", err)
		return cube.Message{}, false
	}
	hidden := p.hidden(p.opts.Client.Snapshot().Metadata())
	var kept []cube.RFAddress
	var refused []string
	for _, rf := range addrs {
		if hidden[rf] {
			refused = append(refused, rf.String())
		} else {
			kept = append(kept, rf)
		}
	}
	if len(refused) == 0 {
		return m, true
	}
	p.log.Warn("proxy: refused to delete hidden devices", "remote", remote, "devices", strings.Join(refused, ","), "passed_on", len(kept))
	if len(kept) == 0 {
		return cube.Message{}, false
	}
	return cube.CmdDeleteDevices(force, kept...), true
}

// withHidden puts the hidden devices, and their rooms, back into a
// client's metadata write, so that a client that never saw them cannot
// remove them from the Cube. The result may need several m: parts.
func (p *Proxy) withHidden(m cube.Message) ([]cube.Message, bool) {
	idx, data, ok := strings.Cut(m.Data, ",")
	if !ok || idx != "00" {
		p.log.Warn("proxy: multi-part metadata write passed through unchanged; hidden devices may be lost", "index", idx)
		return nil, false
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, false
	}
	md, err := cube.DecodeMetadata(raw)
	if err != nil {
		p.log.Warn("proxy: metadata write not understood, passed through unchanged", "error", err)
		return nil, false
	}
	cur := p.opts.Client.Snapshot().Metadata()
	if cur == nil {
		return nil, false
	}
	added := cubeio.KeepHidden(md, cur, p.hidden(cur))
	if added == 0 {
		return nil, false
	}
	cmds, err := cube.WriteMetadataCommands(md)
	if err != nil {
		p.log.Warn("proxy: metadata write with hidden devices could not be encoded, passed through unchanged", "error", err)
		return nil, false
	}
	p.log.Info("proxy: hidden devices put back into a client's metadata write", "devices", added)
	return cmds, true
}

// hidden is the set of devices left out for clients.
func (p *Proxy) hidden(md *cube.Metadata) view.Hidden {
	list := p.opts.Hidden
	if p.opts.HiddenList != nil {
		list = p.opts.HiddenList()
	}
	return view.NewHidden(list, md, p.opts.ShowHidden)
}

// hiddenRooms lists the rooms left out of the greeting: room 255,
// when it holds only hidden devices. Other parking rooms stay visible,
// empty, so that a client allocating the lowest free id cannot reuse
// theirs.
func (p *Proxy) hiddenRooms(md *cube.Metadata) map[uint8]bool {
	if p.opts.ShowHidden {
		return nil
	}
	hidden := p.hidden(md).Rooms(md)
	if !hidden[cube.HiddenRoom] {
		return nil
	}
	return map[uint8]bool{cube.HiddenRoom: true}
}

// discoveryReply builds the reply to a discovery request for the
// serial it names ("" for any), or returns false when the request is
// not for this proxy's Cube.
func (p *Proxy) discoveryReply(serial string, op byte) ([]byte, bool) {
	h := p.opts.Client.Snapshot().Hello()
	if h == nil {
		return nil, false
	}
	mine := p.opts.DiscoverySerial
	if mine == "" {
		mine = h.Serial
	}
	if serial != "" && !strings.EqualFold(serial, h.Serial) && !strings.EqualFold(serial, mine) {
		return nil, false
	}
	if op != cubelan.UDPIdentify {
		p.log.Debug("proxy: discovery request not answered", "op", string(op), "serial", mine)
		return nil, false
	}
	fw, _ := strconv.ParseUint(h.Firmware, 16, 16)
	return cubelan.UDPReply(mine, cubelan.UDPIdentify, cubelan.UDPIdentifyData(h.RFAddress, uint16(fw))), true
}

// Responder answers discovery requests for several proxies on one UDP
// socket. A discovery reply carries no address: the client takes the
// packet's source address, so a proxy that listens on a specific IP
// answers from that IP, which lets one host present several Cubes.
//
// The same socket also runs discovery for the server (Discover): the
// Cubes answer to the discovery port, which this socket holds, so the
// request goes out from it and the answers are taken out of what it
// reads.
type Responder struct {
	log     *slog.Logger
	mu      sync.Mutex
	members []responderMember

	pc         net.PacketConn
	serving    chan struct{} // closed once Serve has its socket
	collectors map[chan cubelan.Discovered]struct{}
}

type responderMember struct {
	p    *Proxy
	from *net.UDPConn // bound to the proxy's IP; nil answers from the shared socket
}

// NewResponder creates an empty responder; Add registers proxies.
func NewResponder(log *slog.Logger) *Responder {
	if log == nil {
		log = slog.Default()
	}
	return &Responder{log: log, serving: make(chan struct{}), collectors: map[chan cubelan.Discovered]struct{}{}}
}

// Discover sends a discovery request from the responder's socket to each
// target and returns the answers that arrive within wait, one per serial
// and address, in the order they came. Answers of this host's own proxies
// are among them; the caller tells them apart. It waits for Serve to have
// its socket.
func (r *Responder) Discover(ctx context.Context, targets []*net.UDPAddr, wait time.Duration) ([]cubelan.Discovered, error) {
	select {
	case <-r.serving:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	ch := make(chan cubelan.Discovered, 64)
	r.mu.Lock()
	pc := r.pc
	r.collectors[ch] = struct{}{}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.collectors, ch)
		r.mu.Unlock()
	}()
	req := cubelan.IdentifyRequest()
	for i := 0; i < 2; i++ {
		for _, t := range targets {
			pc.WriteTo(req, t)
		}
		time.Sleep(50 * time.Millisecond)
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	seen := map[string]bool{}
	var out []cubelan.Discovered
	for {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-timer.C:
			return out, nil
		case d := <-ch:
			key := d.Serial + "@" + d.IP.String()
			if !seen[key] {
				seen[key] = true
				out = append(out, d)
			}
		}
	}
}

// Add registers a proxy. With ip set, replies for it are sent from a
// socket bound to that IP.
func (r *Responder) Add(p *Proxy, ip net.IP) error {
	m := responderMember{p: p}
	if ip != nil && !ip.IsUnspecified() {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: ip})
		if err != nil {
			return fmt.Errorf("discovery replies from %s: %w", ip, err)
		}
		m.from = c
	}
	r.mu.Lock()
	r.members = append(r.members, m)
	r.mu.Unlock()
	return nil
}

// Remove unregisters a proxy and closes its reply socket.
func (r *Responder) Remove(p *Proxy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.members[:0]
	for _, m := range r.members {
		if m.p == p {
			if m.from != nil {
				m.from.Close()
			}
			continue
		}
		kept = append(kept, m)
	}
	r.members = kept
}

// Serve answers requests on pc until ctx ends, and hands the Cubes'
// answers to a running Discover.
func (r *Responder) Serve(ctx context.Context, pc net.PacketConn) error {
	r.mu.Lock()
	r.pc = pc
	r.mu.Unlock()
	close(r.serving)
	go func() {
		<-ctx.Done()
		pc.Close()
		r.mu.Lock()
		for _, m := range r.members {
			if m.from != nil {
				m.from.Close()
			}
		}
		r.mu.Unlock()
	}()
	buf := make([]byte, 1024)
	for {
		n, src, err := pc.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, net.ErrClosed) {
				return err
			}
			continue
		}
		u, _ := src.(*net.UDPAddr)
		serial, op, ok := cubelan.ParseUDPRequest(buf[:n])
		if !ok {
			if u != nil {
				if d, ok := cubelan.ParseIdentifyReply(buf[:n], u.IP); ok {
					r.mu.Lock()
					for ch := range r.collectors {
						select {
						case ch <- d:
						default:
						}
					}
					r.mu.Unlock()
				}
			}
			continue
		}
		r.mu.Lock()
		members := append([]responderMember(nil), r.members...)
		r.mu.Unlock()
		for _, m := range members {
			reply, ok := m.p.discoveryReply(serial, op)
			if !ok {
				continue
			}
			send := func(to net.Addr) {
				if m.from != nil {
					m.from.WriteTo(reply, to)
				} else {
					pc.WriteTo(reply, to)
				}
			}
			send(src)
			// Clients such as the original software listen on the fixed port.
			if u != nil && u.Port != cubelan.DiscoveryPort {
				send(&net.UDPAddr{IP: u.IP, Port: cubelan.DiscoveryPort})
			}
			r.log.Debug("proxy: answered discovery", "from", src.String())
		}
	}
}
