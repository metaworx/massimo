// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// Search is discovery on one socket, for massimo discover and serve
// alike: the identify request to every target, then N and c to each
// Cube that answers, all from that socket, since a Cube answers to the
// discovery port alone. massimo discover runs it on a socket of its own
// (Discover); serve on its responder's, whose reader keeps answering
// other hosts' requests and hands the search the rest
// (SearchOptions.Packets): two readers on one socket would split its
// datagrams between them.

// Kind says what an answer to the identify request came from.
type Kind string

const (
	// KindCube is a Cube: it answered N or c, has eQ-3's MAC, or
	// answered from its bootloader.
	KindCube Kind = "cube"
	// KindProxy is a massimo proxy, which answers the identify request
	// alone: no N or c, and a MAC that is not eQ-3's.
	KindProxy Kind = "proxy"
	// KindUnknown is neither: no answer to N or c, and no MAC in the ARP
	// cache, as on the loopback.
	KindUnknown Kind = "unknown"
)

// Packet is a datagram a socket read, and where it came from.
type Packet struct {
	Data []byte
	From *net.UDPAddr
}

// SearchOptions say how Search runs.
type SearchOptions struct {
	// Targets are where the identify request goes; nil is
	// DiscoveryTargets.
	Targets []*net.UDPAddr
	// Wait is how long answers to it are taken.
	Wait time.Duration
	// Packets are the datagrams the socket reads, when another reader
	// owns it, as serve's responder does; nil has Search read the socket
	// itself.
	Packets <-chan Packet
	// MAC gives the MAC of an address; nil is LookupMAC.
	MAC func(net.IP) string
}

// lateWait is how long Search waits past its wait for the N and c
// replies of a Cube that answered late.
const lateWait = time.Second

// pending is one answer Search is completing.
type pending struct {
	d        Discovered
	asked    time.Time // when N and c went out
	net, cfg bool      // N and c answered
}

// Search sends the identify request from pc to every target and returns
// the answers that arrive within the wait, one per serial and address,
// in the order they came: a massimo proxy answers with the serial of the
// Cube it serves, so a serial can come twice. Each answer is asked N and
// c at once, from pc, and its addresses in use and DHCP flag are filled
// in from the replies; one that answered late gets a second more for
// them. Then each answer gets its MAC and its Kind.
func Search(ctx context.Context, pc net.PacketConn, o SearchOptions) ([]Discovered, error) {
	targets := o.Targets
	if targets == nil {
		targets = DiscoveryTargets()
	}
	mac := o.MAC
	if mac == nil {
		mac = LookupMAC
	}
	packets := o.Packets
	if packets == nil {
		ch := make(chan Packet, 64)
		done := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			readPackets(pc, ch, done)
		}()
		defer func() {
			close(done)
			wg.Wait()
		}()
		packets = ch
	}

	req := udpRequest("", udpIdentify)
	for i := 0; i < 2; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(50 * time.Millisecond):
			}
		}
		for _, t := range targets {
			pc.WriteTo(req, t)
		}
	}

	timer := time.NewTimer(o.Wait)
	defer timer.Stop()
	index := map[string]int{} // serial@ip to its place in found
	var found []*pending
	for {
		select {
		case <-ctx.Done():
			return finish(found, mac), ctx.Err()
		case <-timer.C:
			// An answer asked shortly before the end gets lateWait for
			// its N and c replies; the others are done.
			var last time.Time
			for _, p := range found {
				if !(p.net && p.cfg) && p.asked.After(last) {
					last = p.asked
				}
			}
			if more := time.Until(last.Add(lateWait)); more > 0 {
				timer.Reset(more)
				continue
			}
			return finish(found, mac), nil
		case pkt, ok := <-packets:
			if !ok {
				return finish(found, mac), nil
			}
			if pkt.From == nil {
				continue
			}
			serial, _, op, data, ok := parseUDPReply(pkt.Data)
			if !ok {
				continue
			}
			key := serial + "@" + pkt.From.IP.String()
			i, seen := index[key]
			switch op {
			case udpIdentify:
				d, ok := ParseIdentifyReply(pkt.Data, pkt.From.IP)
				if seen || !ok {
					continue
				}
				index[key] = len(found)
				found = append(found, &pending{d: d, asked: time.Now()})
				// The Cube answers from the port it takes requests on.
				pc.WriteTo(udpRequest(serial, udpNetConfig), pkt.From)
				pc.WriteTo(udpRequest(serial, udpGetConfig), pkt.From)
			case udpNetConfig:
				if !seen || found[i].net {
					continue
				}
				found[i].net = true
				if a, ok := parseNetAddrs(data); ok {
					found[i].d.Gateway, found[i].d.Netmask, found[i].d.DNS = a.Gateway, a.Netmask, a.DNS
				}
			case udpGetConfig:
				if !seen || found[i].cfg {
					continue
				}
				found[i].cfg = true
				if cfg, err := parseNetConfig(data); err == nil {
					dhcp := cfg.DHCP
					found[i].d.DHCP = &dhcp
				}
			}
		}
	}
}

// finish gives each answer its MAC and Kind.
func finish(found []*pending, mac func(net.IP) string) []Discovered {
	out := make([]Discovered, 0, len(found))
	for _, p := range found {
		d := p.d
		d.MAC = mac(d.IP)
		switch {
		case p.net || p.cfg || d.Bootloader || IsEQ3MAC(d.MAC):
			d.Kind = KindCube
		case d.MAC != "":
			d.Kind = KindProxy
		default:
			d.Kind = KindUnknown
		}
		out = append(out, d)
	}
	return out
}

// readPackets hands what pc reads to ch until done is closed.
func readPackets(pc net.PacketConn, ch chan<- Packet, done <-chan struct{}) {
	buf := make([]byte, 1024)
	for {
		select {
		case <-done:
			return
		default:
		}
		pc.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		u, _ := from.(*net.UDPAddr)
		select {
		case ch <- Packet{Data: append([]byte(nil), buf[:n]...), From: u}:
		case <-done:
			return
		}
	}
}
