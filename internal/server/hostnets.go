// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"net"
	"sync"
	"time"
)

// hostNetsFor is how long the host's networks are kept before they are
// read again: every listing of the Cubes asks them.
const hostNetsFor = 30 * time.Second

// hostNets keeps the networks of this host's interfaces, read at most
// every hostNetsFor.
type hostNets struct {
	mu   sync.Mutex
	at   time.Time
	nets []*net.IPNet
}

// interfaceNets are the networks of this host's interface addresses,
// loopback among them.
func interfaceNets() []*net.IPNet {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []*net.IPNet
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			out = append(out, n)
		}
	}
	return out
}

// hostNetworks returns this host's networks: Options.hostNets in the
// tests, else the interfaces' as last read.
func (s *Server) hostNetworks() []*net.IPNet {
	if s.opts.hostNets != nil {
		return s.opts.hostNets()
	}
	s.nets.mu.Lock()
	defer s.nets.mu.Unlock()
	if s.nets.nets == nil || time.Since(s.nets.at) >= hostNetsFor {
		s.nets.nets, s.nets.at = interfaceNets(), time.Now()
	}
	return s.nets.nets
}

// otherNetwork reports whether addr, an IP address with or without a
// port, lies in none of nets: the server reaches it only through a
// router, if at all. A Cube with DHCP on and no lease answers discovery
// from its fallback address, 192.168.0.222/16 from the factory, and
// refuses a connection from another network (docs/cube-protocol.md). A
// host name is not judged, nor is anything where no network is known.
func otherNetwork(nets []*net.IPNet, addr string) bool {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	if ip == nil || len(nets) == 0 {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return false
		}
	}
	return true
}
