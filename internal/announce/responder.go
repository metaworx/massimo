// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package announce

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"go.metaworx.ch/massimo/internal/sharedport"
)

// interval is how often the responder announces itself unasked.
const interval = 2 * time.Minute

// Responder answers for one service until it is stopped.
type Responder struct {
	svc   Service
	n     names
	log   *slog.Logger
	opts  options
	conns []*conn

	cancel  context.CancelFunc
	done    chan struct{} // closed once the goodbye is sent and the sockets closed
	readers sync.WaitGroup
}

// conn is one socket, IPv4 or IPv6, and the interfaces it joined the
// group on.
type conn struct {
	pc    net.PacketConn
	v6    bool
	p4    *ipv4.PacketConn
	p6    *ipv6.PacketConn
	group *net.UDPAddr

	mu     sync.Mutex // the multicast interface is set before each send
	joined map[int]net.Interface
}

// options are the responder's, for the tests.
type options struct {
	port     int           // the mDNS port, Port when 0
	interval time.Duration // between announcements, interval when 0
	// listen opens a family's socket; listenShared when nil.
	listen func(network string, port int) (net.PacketConn, error)
	// sendTo, where set, takes the announcements instead of the groups,
	// and nothing joins a group.
	sendTo []net.Addr
	// hostAddrs are the addresses for an asker no interface holds, and
	// for sendTo; hostAddresses when nil.
	hostAddrs func() []net.IP
}

// Start announces svc on every interface that takes multicast, answers
// the queries for it, and announces it again every two minutes until ctx
// ends or Stop is called; then it says goodbye. It fails when no socket
// can be opened or no interface joins the group, and the server then
// runs unannounced.
func Start(ctx context.Context, svc Service, log *slog.Logger) (*Responder, error) {
	return start(ctx, svc, log, options{})
}

func start(ctx context.Context, svc Service, log *slog.Logger, o options) (*Responder, error) {
	if log == nil {
		log = slog.Default()
	}
	if o.port == 0 {
		o.port = Port
	}
	if o.interval == 0 {
		o.interval = interval
	}
	if o.listen == nil {
		o.listen = listenShared
	}
	if o.hostAddrs == nil {
		o.hostAddrs = hostAddresses
	}
	n, err := namesOf(svc)
	if err != nil {
		return nil, err
	}
	r := &Responder{svc: svc, n: n, log: log, opts: o, done: make(chan struct{})}
	var errs []error
	for _, f := range []struct {
		network string
		v6      bool
		group   net.IP
	}{{"udp4", false, groupV4}, {"udp6", true, groupV6}} {
		pc, err := o.listen(f.network, o.port)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.network, err))
			continue
		}
		c := &conn{pc: pc, v6: f.v6, group: &net.UDPAddr{IP: f.group, Port: o.port}, joined: map[int]net.Interface{}}
		if f.v6 {
			c.p6 = ipv6.NewPacketConn(pc)
			c.p6.SetMulticastHopLimit(255)
			c.p6.SetMulticastLoopback(true)
		} else {
			c.p4 = ipv4.NewPacketConn(pc)
			c.p4.SetMulticastTTL(255)
			c.p4.SetMulticastLoopback(true)
		}
		r.conns = append(r.conns, c)
	}
	if len(r.conns) == 0 {
		return nil, fmt.Errorf("announce: no mDNS socket on port %d: %w", o.port, errors.Join(errs...))
	}
	if len(o.sendTo) == 0 && r.join() == 0 {
		r.close()
		return nil, errors.New("announce: no interface joined the mDNS group (no network with multicast)")
	}
	for _, err := range errs {
		log.Info("mdns: one address family not announced", "error", err)
	}
	ctx, r.cancel = context.WithCancel(ctx)
	for _, c := range r.conns {
		r.readers.Add(1)
		go r.serve(c)
	}
	go r.run(ctx)
	return r, nil
}

// Stop says goodbye and closes the sockets; it returns once they are
// closed. A second Stop does nothing.
func (r *Responder) Stop() {
	r.cancel()
	<-r.done
}

// run announces the service at the start, a second later and then every
// interval, and says goodbye when ctx ends.
func (r *Responder) run(ctx context.Context) {
	defer close(r.done)
	r.announce(false)
	t := time.NewTimer(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			r.announce(true)
			r.close()
			r.readers.Wait()
			return
		case <-t.C:
			if len(r.opts.sendTo) == 0 {
				r.join() // an interface that came up since
			}
			r.announce(false)
			t.Reset(r.opts.interval)
		}
	}
}

func (r *Responder) close() {
	for _, c := range r.conns {
		c.pc.Close()
	}
}

// join joins the group on every interface that takes multicast and has
// an address of the socket's family, and returns how many interfaces are
// joined.
func (r *Responder) join() int {
	ifs, _ := net.Interfaces()
	total := 0
	for _, c := range r.conns {
		c.mu.Lock()
		for _, ifi := range ifs {
			if _, ok := c.joined[ifi.Index]; ok || ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagMulticast == 0 || ifi.Flags&net.FlagLoopback != 0 || !hasFamily(ifi, c.v6) {
				continue
			}
			var err error
			if c.v6 {
				err = c.p6.JoinGroup(&ifi, &net.UDPAddr{IP: groupV6})
			} else {
				err = c.p4.JoinGroup(&ifi, &net.UDPAddr{IP: groupV4})
			}
			if err != nil {
				r.log.Debug("mdns: group not joined", "interface", ifi.Name, "error", err)
				continue
			}
			c.joined[ifi.Index] = ifi
		}
		total += len(c.joined)
		c.mu.Unlock()
	}
	return total
}

// set is the service's records for these addresses: legacy for an
// answer to a legacy unicast query, goodbye with TTL 0.
func (r *Responder) set(addrs []net.IP, legacy, goodbye bool) recordSet {
	return recordSet{n: r.n, svc: r.svc, addrs: addrs, flush: !legacy, ttl: func(long bool) uint32 {
		switch {
		case goodbye:
			return 0
		case legacy:
			return ttlLegacy
		case long:
			return ttlLong
		}
		return ttlShort
	}}
}

// announce sends every record unasked, with TTL 0 for the goodbye, on
// every joined interface with that interface's addresses.
func (r *Responder) announce(goodbye bool) {
	r.multicast(func(addrs []net.IP) records { return r.set(addrs, false, goodbye).all() })
}

// multicast sends the records rs gives for each interface's addresses to
// the group on that interface, or to sendTo.
func (r *Responder) multicast(rs func(addrs []net.IP) records) {
	if len(r.opts.sendTo) > 0 {
		set := rs(r.opts.hostAddrs())
		if len(set.answers) == 0 {
			return
		}
		msg, err := build(0, nil, set)
		if err != nil {
			r.log.Error("mdns: response not built", "error", err)
			return
		}
		for _, to := range r.opts.sendTo {
			r.conns[0].pc.WriteTo(msg, to)
		}
		return
	}
	for _, c := range r.conns {
		c.mu.Lock()
		for _, ifi := range c.joined {
			set := rs(interfaceAddresses(ifi))
			if len(set.answers) == 0 {
				continue
			}
			msg, err := build(0, nil, set)
			if err != nil {
				r.log.Error("mdns: response not built", "error", err)
				continue
			}
			if c.v6 {
				err = c.p6.SetMulticastInterface(&ifi)
			} else {
				err = c.p4.SetMulticastInterface(&ifi)
			}
			if err == nil {
				_, err = c.pc.WriteTo(msg, c.group)
			}
			if err != nil {
				r.log.Debug("mdns: not sent", "interface", ifi.Name, "error", err)
			}
		}
		c.mu.Unlock()
	}
}

// serve reads the queries that reach c until it is closed.
func (r *Responder) serve(c *conn) {
	defer r.readers.Done()
	buf := make([]byte, 9000)
	for {
		n, from, err := c.pc.ReadFrom(buf)
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			continue // a Windows socket reports an earlier send's ICMP error here
		}
		if src, ok := from.(*net.UDPAddr); ok {
			r.handle(c, buf[:n], src)
		}
	}
}

// handle answers a query about the service: a legacy unicast query
// (from another port than mDNS's) to its sender with its ID and
// questions, a query asking for a unicast answer to its sender, any
// other to the group.
func (r *Responder) handle(c *conn, msg []byte, src *net.UDPAddr) {
	var p dnsmessage.Parser
	h, err := p.Start(msg)
	if err != nil || h.Response || h.OpCode != 0 {
		return
	}
	qs, err := p.AllQuestions()
	if err != nil {
		return
	}
	legacy := src.Port != r.opts.port
	unicast := legacy
	var asked []dnsmessage.Question
	for _, q := range qs {
		qu := q.Class&unicastResponse != 0
		q.Class &^= unicastResponse
		if q.Class != dnsmessage.ClassINET && q.Class != dnsmessage.ClassANY {
			continue
		}
		if len(r.set(nil, false, false).answer(q).answers) == 0 && !r.asksHost(q) {
			continue
		}
		asked = append(asked, q)
		unicast = unicast || qu
	}
	if len(asked) == 0 {
		return
	}
	answer := func(addrs []net.IP) records {
		var rs records
		for _, q := range asked {
			a := r.set(addrs, legacy, false).answer(q)
			rs.answers = append(rs.answers, a.answers...)
			rs.additionals = append(rs.additionals, a.additionals...)
		}
		return rs
	}
	if !unicast {
		r.multicast(answer)
		return
	}
	rs := answer(r.addressesFor(src.IP))
	if len(rs.answers) == 0 {
		return
	}
	id, questions := uint16(0), []dnsmessage.Question(nil)
	if legacy {
		id, questions = h.ID, asked
	}
	out, err := build(id, questions, rs)
	if err != nil {
		r.log.Error("mdns: response not built", "error", err)
		return
	}
	c.pc.WriteTo(out, src)
}

// asksHost says whether q asks for the host name's addresses, which the
// record set answers only once it has some.
func (r *Responder) asksHost(q dnsmessage.Question) bool {
	return strings.EqualFold(q.Name.String(), r.n.host.String())
}

// addressesFor are the addresses of the interface whose network holds
// ip, else the host's.
func (r *Responder) addressesFor(ip net.IP) []net.IP {
	if len(r.opts.sendTo) == 0 {
		ifs, _ := net.Interfaces()
		for _, ifi := range ifs {
			addrs, _ := ifi.Addrs()
			for _, a := range addrs {
				if n, ok := a.(*net.IPNet); ok && ifi.Flags&net.FlagLoopback == 0 && n.Contains(ip) {
					return interfaceAddresses(ifi)
				}
			}
		}
	}
	return r.opts.hostAddrs()
}

// interfaceAddresses are an interface's unicast addresses.
func interfaceAddresses(ifi net.Interface) []net.IP {
	addrs, _ := ifi.Addrs()
	var out []net.IP
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() && !n.IP.IsMulticast() {
			out = append(out, n.IP)
		}
	}
	return out
}

// hostAddresses are the unicast addresses of every interface that is up
// and has a link, but the loopback's.
func hostAddresses() []net.IP {
	ifs, _ := net.Interfaces()
	var out []net.IP
	for _, ifi := range ifs {
		if ifi.Flags&net.FlagUp != 0 && ifi.Flags&net.FlagRunning != 0 && ifi.Flags&net.FlagLoopback == 0 {
			out = append(out, interfaceAddresses(ifi)...)
		}
	}
	return out
}

// hasFamily says whether an interface has an address of the family.
func hasFamily(ifi net.Interface, v6 bool) bool {
	for _, ip := range interfaceAddresses(ifi) {
		if (ip.To4() == nil) == v6 {
			return true
		}
	}
	return false
}

// listenShared opens the mDNS port of the family with the address and
// port shared, since the system's responder holds the port too.
func listenShared(network string, port int) (net.PacketConn, error) {
	lc := net.ListenConfig{Control: sharedport.Control}
	return lc.ListenPacket(context.Background(), network, fmt.Sprintf(":%d", port))
}
