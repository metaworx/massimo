// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// searchOn runs Search on a loopback socket of its own against targets.
func searchOn(t *testing.T, o SearchOptions) []Discovered {
	t.Helper()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	found, err := Search(context.Background(), pc, o)
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func udpAddr(t *testing.T, s string) *net.UDPAddr {
	t.Helper()
	a, err := net.ResolveUDPAddr("udp4", s)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// A Cube answers N and c, a massimo proxy the identify request alone:
// the Cube's answer is a Cube's, with its addresses in use and its DHCP
// flag; the proxy's is unknown on the loopback, where the ARP cache has
// nothing, a proxy's with a MAC that is not eQ-3's, and a Cube's with
// eQ-3's.
func TestSearchMarksTheAnswers(t *testing.T) {
	cubeAt, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0107)
	if err != nil {
		t.Fatal(err)
	}
	defer cubeAt.Close()
	_, port, _ := net.SplitHostPort(cubeAt.Addr())
	proxyAt, err := fakecube.StartDiscovery("127.0.0.2:"+port, "KEQ0000000", 0x0a0107)
	if err != nil {
		t.Skipf("no second loopback address here: %v", err)
	}
	defer proxyAt.Close()
	proxyAt.SetIdentifyOnly(true)
	targets := []*net.UDPAddr{udpAddr(t, cubeAt.Addr()), udpAddr(t, proxyAt.Addr())}

	for _, tc := range []struct {
		name      string
		proxyMAC  string
		proxyKind Kind
	}{
		{"no MAC", "", KindUnknown},
		{"another card's MAC", "02:00:00:00:00:02", KindProxy},
		{"eQ-3's MAC", "00:1a:22:00:00:02", KindCube},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mac := func(ip net.IP) string {
				if ip.Equal(net.IPv4(127, 0, 0, 2)) {
					return tc.proxyMAC
				}
				return ""
			}
			found := searchOn(t, SearchOptions{Targets: targets, Wait: 300 * time.Millisecond, MAC: mac})
			byIP := map[string]Discovered{}
			for _, d := range found {
				byIP[d.IP.String()] = d
			}
			if len(found) != 2 || len(byIP) != 2 {
				t.Fatalf("answers %+v", found)
			}
			c := byIP["127.0.0.1"]
			if c.Kind != KindCube || c.Serial != "KEQ0000000" || c.RFAddress != 0x0a0107 || c.Firmware != "1.1.3" {
				t.Errorf("the Cube's answer %+v", c)
			}
			if c.DHCP == nil || !*c.DHCP || !c.Gateway.Equal(net.IPv4(127, 0, 0, 1)) || !c.Netmask.Equal(net.IPv4(255, 255, 255, 0)) {
				t.Errorf("the Cube's network: dhcp %v, gateway %v, netmask %v", c.DHCP, c.Gateway, c.Netmask)
			}
			p := byIP["127.0.0.2"]
			if p.Kind != tc.proxyKind || p.MAC != tc.proxyMAC || p.DHCP != nil || p.Gateway != nil {
				t.Errorf("the proxy's answer %+v, want kind %s", p, tc.proxyKind)
			}
		})
	}
}

// A search fed the datagrams another reader takes from the socket, as
// serve's responder does, finds what one reading the socket finds.
func TestSearchFromAFeed(t *testing.T) {
	fake, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0107)
	if err != nil {
		t.Fatal(err)
	}
	defer fake.Close()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	feed := make(chan Packet, 16)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			u, _ := from.(*net.UDPAddr)
			feed <- Packet{Data: append([]byte(nil), buf[:n]...), From: u}
		}
	}()
	found, err := Search(context.Background(), pc, SearchOptions{Targets: []*net.UDPAddr{udpAddr(t, fake.Addr())}, Wait: 200 * time.Millisecond, Packets: feed})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Kind != KindCube || found[0].DHCP == nil {
		t.Fatalf("answers %+v", found)
	}
}

// A Cube that answers just before the wait ends, and its N and c only
// after it, still has them: the search waits a second more.
func TestSearchWaitsForALateAnswer(t *testing.T) {
	cube, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer cube.Close()
	const wait = 300 * time.Millisecond
	reply := func(op byte, data []byte) []byte {
		b := append([]byte("eQ3MaxAp"), fmt.Sprintf("%-10.10s", "KEQ0000000")...)
		return append(append(b, '>', op), data...)
	}
	go func() {
		buf := make([]byte, 1024)
		identified := false
		for {
			n, from, err := cube.ReadFrom(buf)
			if err != nil {
				return
			}
			if n < 19 {
				continue
			}
			switch op := buf[18]; {
			case op == 'I' && !identified:
				identified = true
				time.Sleep(wait - 50*time.Millisecond)
				data := binary.BigEndian.AppendUint32(nil, 0x0a0107)
				cube.WriteTo(reply('I', binary.BigEndian.AppendUint16(data, 0x0113)), from)
			case op == 'N' || op == 'c':
				time.Sleep(200 * time.Millisecond) // past the wait
				if op == 'N' {
					cube.WriteTo(reply('N', []byte{127, 0, 0, 1, 127, 0, 0, 1, 255, 255, 255, 0}), from)
				} else {
					cfg := make([]byte, netConfigLen)
					cfg[20] = 1
					cube.WriteTo(reply('c', cfg), from)
				}
			}
		}
	}()
	found := searchOn(t, SearchOptions{Targets: []*net.UDPAddr{udpAddr(t, cube.LocalAddr().String())}, Wait: wait})
	if len(found) != 1 {
		t.Fatalf("answers %+v", found)
	}
	if d := found[0]; d.Kind != KindCube || d.DHCP == nil || !*d.DHCP || !d.Gateway.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Fatalf("the late Cube %+v", d)
	}
}
