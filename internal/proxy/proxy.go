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
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
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
	Logger       *slog.Logger
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

// Run listens on tcpAddr for Cube-protocol clients and, with discovery
// set, answers discovery requests on UDP port cubelan.DiscoveryPort. It
// returns when ctx ends or the TCP listener fails. A failure to bind the
// UDP port is logged and otherwise ignored.
func (p *Proxy) Run(ctx context.Context, tcpAddr string, discovery bool) error {
	ln, err := net.Listen("tcp", tcpAddr)
	if err != nil {
		return err
	}
	if discovery {
		pc, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", cubelan.DiscoveryPort))
		if err != nil {
			p.log.Warn("proxy: discovery port not available, clients must be given the address", "port", cubelan.DiscoveryPort, "error", err)
		} else {
			go p.ServeUDP(ctx, pc)
		}
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
			l, ok := p.opts.Client.Snapshot().DeviceListLine()
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

func (s *session) write(m cube.Message) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := s.conn.Write([]byte(m.Wire()))
	return err
}

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
	lines := snap.Lines()
	for i, m := range lines {
		if m.Type == 'H' {
			lines[i] = p.hello(snap.Hello())
		}
	}
	return lines
}

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
	switch m.Type {
	case 'l':
		if l, ok := snap.DeviceListLine(); ok {
			return l, true
		}
		return cube.Message{}, true
	case 'c':
		rf, err := cube.ParseRFAddress(strings.TrimSpace(m.Data))
		if err != nil {
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
	p.log.Debug("proxy: passing command to the cube", "remote", remote, "command", m.String())
	wctx, cancel := context.WithTimeout(ctx, p.opts.ReplyTimeout)
	defer cancel()
	res := p.opts.Queue.Enqueue(engine.Command{Priority: engine.PriorityNormal, Passthrough: true, Lines: []cube.Message{m}}).Wait(wctx)
	if res.Err != nil {
		p.log.Warn("proxy: command failed", "remote", remote, "command", m.String(), "error", res.Err)
		return res.Replies
	}
	if m.Type == 'm' {
		p.applyMetadata(m)
	}
	return res.Replies
}

// applyMetadata updates the snapshot after a metadata write, which the
// Cube acknowledges without echoing the new M lines.
func (p *Proxy) applyMetadata(m cube.Message) {
	idx, data, ok := strings.Cut(m.Data, ",")
	if !ok || idx != "00" {
		p.log.Warn("proxy: multi-part metadata write not mirrored into the snapshot", "index", idx)
		return
	}
	p.opts.Client.ApplyLocal(cube.Message{Type: 'M', Data: "00,01," + data})
}

// ServeUDP answers discovery requests on pc until ctx ends.
func (p *Proxy) ServeUDP(ctx context.Context, pc net.PacketConn) error {
	go func() {
		<-ctx.Done()
		pc.Close()
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
		h := p.opts.Client.Snapshot().Hello()
		if h == nil {
			continue
		}
		mine := p.opts.DiscoverySerial
		if mine == "" {
			mine = h.Serial
		}
		if serial != "" && !strings.EqualFold(serial, h.Serial) && !strings.EqualFold(serial, mine) {
			continue
		}
		if op != cubelan.UDPIdentify {
			p.log.Debug("proxy: discovery request not answered", "op", string(op), "from", src.String())
			continue
		}
		fw, _ := strconv.ParseUint(h.Firmware, 16, 16)
		reply := cubelan.UDPReply(mine, cubelan.UDPIdentify, cubelan.UDPIdentifyData(h.RFAddress, uint16(fw)))
		pc.WriteTo(reply, src)
		// Clients such as the original software listen on the fixed port.
		if u, ok := src.(*net.UDPAddr); ok && u.Port != cubelan.DiscoveryPort {
			pc.WriteTo(reply, &net.UDPAddr{IP: u.IP, Port: cubelan.DiscoveryPort})
		}
		p.log.Debug("proxy: answered discovery", "from", src.String(), "serial", mine)
	}
}
