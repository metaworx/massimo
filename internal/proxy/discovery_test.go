// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package proxy

import (
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
)

// ipNet parses a CIDR keeping the host's address in it, as
// net.InterfaceAddrs gives it.
func ipNet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	ip, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatal(err)
	}
	n.IP = ip
	return n
}

func TestReplyTargets(t *testing.T) {
	nets := []*net.IPNet{
		ipNet(t, "192.0.2.10/24"),
		ipNet(t, "198.51.100.7/24"),
		ipNet(t, "203.0.113.4/31"),
		ipNet(t, "203.0.113.9/32"),
	}
	addr := func(s string) *net.UDPAddr {
		a, err := net.ResolveUDPAddr("udp4", s)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	for _, tc := range []struct {
		name      string
		src       string
		from      string
		port      int
		searching bool
		want      []string
	}{
		{"another host", "192.0.2.50:23272", "", 23272, false, []string{"192.0.2.50:23272"}},
		{"another host, another port", "192.0.2.50:5000", "", 23272, false, []string{"192.0.2.50:5000", "192.0.2.50:23272"}},
		{"this host", "192.0.2.10:23272", "", 23272, false, []string{"192.0.2.10:23272", "192.0.2.255:23272"}},
		{"this host, the responder on another port", "192.0.2.10:23999", "", 23999, false, []string{"192.0.2.10:23999", "192.0.2.10:23272", "192.0.2.255:23999"}},
		{"this host, a proxy on another address", "192.0.2.10:23272", "198.51.100.7", 23272, false, []string{"192.0.2.10:23272", "198.51.100.255:23272"}},
		{"this host, a proxy on the wildcard", "192.0.2.10:23272", "0.0.0.0", 23272, false, []string{"192.0.2.10:23272", "192.0.2.255:23272"}},
		{"this host, a proxy on an address without a network", "192.0.2.10:23272", "127.0.0.1", 23272, false, []string{"192.0.2.10:23272"}},
		{"this host, a /31", "203.0.113.4:23272", "", 23272, false, []string{"203.0.113.4:23272"}},
		{"this host, a /32", "203.0.113.9:23272", "", 23272, false, []string{"203.0.113.9:23272"}},
		{"this host, its own search running", "192.0.2.10:23272", "", 23272, true, []string{"192.0.2.10:23272"}},
		{"this host, no port known", "192.0.2.10:23272", "", 0, false, []string{"192.0.2.10:23272"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var from net.IP
			if tc.from != "" {
				from = net.ParseIP(tc.from)
			}
			var got []string
			for _, a := range replyTargets(addr(tc.src), from, tc.port, nets, tc.searching) {
				got = append(got, a.String())
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// A request from the host itself is also answered by the broadcast of
// its subnet, at the responder's port, where the original software on
// the same host listens beside serve; not while serve's own search runs.
func TestResponderAnswersItsOwnHostByBroadcast(t *testing.T) {
	_, p, _, ctx := setup(t)
	p.opts.DiscoverySerial = "XEQ0000100"
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var sent []string
	resp := NewResponder(nil)
	resp.hostNets = func() []*net.IPNet { return []*net.IPNet{ipNet(t, "127.0.0.1/8")} }
	resp.sent = func(to net.Addr) {
		mu.Lock()
		sent = append(sent, to.String())
		mu.Unlock()
	}
	if err := resp.Add(p, nil); err != nil {
		t.Fatal(err)
	}
	go resp.Serve(ctx, pc)
	client, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	bcast := (&net.UDPAddr{IP: net.IPv4(127, 255, 255, 255), Port: localPort(pc)}).String()
	broadcasts := func() int {
		mu.Lock()
		defer mu.Unlock()
		n := 0
		for _, s := range sent {
			if s == bcast {
				n++
			}
		}
		return n
	}
	// ask sends a request and waits until broadcasts reaches want.
	ask := func(want int) {
		t.Helper()
		req := append([]byte("eQ3Max*\x00**********"), 'I')
		if _, err := client.WriteTo(req, pc.LocalAddr()); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 256)
		client.SetReadDeadline(time.Now().Add(5 * time.Second))
		if _, _, err := client.ReadFrom(buf); err != nil {
			t.Fatal(err)
		}
		for deadline := time.Now().Add(5 * time.Second); broadcasts() < want; time.Sleep(5 * time.Millisecond) {
			if time.Now().After(deadline) {
				n := broadcasts()
				mu.Lock()
				defer mu.Unlock()
				t.Fatalf("%d broadcast(s) to %s, want %d; sent %v", n, bcast, want, sent)
			}
		}
	}
	ask(1)
	// A request during a search: its unicast reply comes, sent after the
	// responder looked at the search; the search ends after it.
	resp.mu.Lock()
	resp.searches++
	resp.mu.Unlock()
	client.WriteTo(append([]byte("eQ3Max*\x00**********"), 'I'), pc.LocalAddr())
	buf := make([]byte, 256)
	client.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, _, err := client.ReadFrom(buf); err != nil {
		t.Fatal(err)
	}
	resp.mu.Lock()
	resp.searches--
	resp.mu.Unlock()
	// The responder reads requests in turn: once the next one's broadcast
	// is out, the one during the search was answered without one.
	ask(2)
	if n := broadcasts(); n != 2 {
		t.Fatalf("%d broadcasts, want 2: one sent during a search", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if want := (&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: cubelan.DiscoveryPort}).String(); !slices.Contains(sent, want) {
		t.Errorf("no reply to the discovery port: %v", sent)
	}
}
