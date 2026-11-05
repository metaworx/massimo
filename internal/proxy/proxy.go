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
	// Ignore lists devices hidden from clients, in addition to those of
	// cube.GhostRoom: they are left out of the greeting, the L line and
	// the C lines, and put back, with their rooms, into every metadata
	// write a client sends, so the Cube keeps them exactly as they are.
	// The ghost room is hidden too; other rooms stay visible, empty.
	Ignore []cube.RFAddress
	// ShowGhosts shows the ignored devices and the ghost room instead.
	ShowGhosts bool
	Logger     *slog.Logger
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
	if m.Type == 'm' && !p.opts.ShowGhosts {
		if merged, ok := p.withIgnored(m); ok {
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
	// The client sent one line and expects one answer.
	if len(res.Replies) > 1 {
		return res.Replies[len(res.Replies)-1:]
	}
	return res.Replies
}

// withIgnored puts the hidden devices, and their rooms, back into a
// client's metadata write, so that a client that never saw them cannot
// remove them from the Cube. The result may need several m: parts.
func (p *Proxy) withIgnored(m cube.Message) ([]cube.Message, bool) {
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
	return view.NewHidden(p.opts.Ignore, md, p.opts.ShowGhosts)
}

// hiddenRooms lists the rooms left out of the greeting: the ghost room,
// when it holds only hidden devices. Other parking rooms stay visible,
// empty, so that a client allocating the lowest free id cannot reuse
// theirs.
func (p *Proxy) hiddenRooms(md *cube.Metadata) map[uint8]bool {
	if p.opts.ShowGhosts {
		return nil
	}
	hidden := p.hidden(md).Rooms(md)
	if !hidden[cube.GhostRoom] {
		return nil
	}
	return map[uint8]bool{cube.GhostRoom: true}
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
type Responder struct {
	log     *slog.Logger
	mu      sync.Mutex
	members []responderMember
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
	return &Responder{log: log}
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

// Serve answers requests on pc until ctx ends.
func (r *Responder) Serve(ctx context.Context, pc net.PacketConn) error {
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
		serial, op, ok := cubelan.ParseUDPRequest(buf[:n])
		if !ok {
			continue
		}
		u, _ := src.(*net.UDPAddr)
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
