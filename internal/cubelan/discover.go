// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// DiscoveryPort is the UDP port of the Cube's discovery and bootloader
// protocol. The Cube answers from this port to this port, so the client
// has to bind it.
const DiscoveryPort = 23272

var (
	udpSignature  = []byte("eQ3Max*\x00")
	udpAnySerial  = []byte("**********")
	udpReplyApp   = []byte("eQ3MaxAp")
	udpReplyBoot  = []byte("eQ3MaxBl")
	udpMulticast  = net.IPv4(224, 0, 0, 1)
	udpBroadcast  = net.IPv4bcast
	udpIdentify   = byte('I')
	udpNetConfig  = byte('N')
	udpGetConfig  = byte('c')
	udpReplyMinSz = 8 + 10 + 2
)

// Discovered describes a Cube that answered a discovery request.
type Discovered struct {
	Serial     string         `json:"serial"`
	IP         net.IP         `json:"ip"`
	RFAddress  cube.RFAddress `json:"rf_address"`
	Firmware   string         `json:"firmware"` // e.g. "1.1.3"
	Bootloader bool           `json:"bootloader"`
	// From the N reply, when the Cube answered it.
	Gateway net.IP   `json:"gateway,omitempty"`
	Netmask net.IP   `json:"netmask,omitempty"`
	DNS     []net.IP `json:"dns,omitempty"`
	// DHCP says whether the Cube takes its address by DHCP, from the c
	// reply, when the Cube answered it.
	DHCP *bool  `json:"dhcp,omitempty"`
	MAC  string `json:"mac,omitempty"` // from the ARP cache, when known
	// Kind says whether the answer is a Cube's or a massimo proxy's, as
	// Search tells them apart.
	Kind Kind `json:"kind"`
}

// udpRequest builds a discovery packet: signature, serial, opcode.
func udpRequest(serial string, op byte) []byte {
	s := []byte(serial)
	for len(s) < 10 {
		s = append(s, '*')
	}
	p := append([]byte(nil), udpSignature...)
	p = append(p, s[:10]...)
	return append(p, op)
}

// parseUDPReply decodes a reply: signature, serial, '>', opcode, data.
func parseUDPReply(b []byte) (serial string, boot bool, op byte, data []byte, ok bool) {
	if len(b) < udpReplyMinSz {
		return "", false, 0, nil, false
	}
	switch {
	case bytes.Equal(b[:8], udpReplyApp):
	case bytes.Equal(b[:8], udpReplyBoot):
		boot = true
	default:
		return "", false, 0, nil, false
	}
	if b[18] != '>' {
		return "", false, 0, nil, false
	}
	return string(b[8:18]), boot, b[19], b[20:], true
}

// Discover broadcasts an identify request and collects the Cubes that
// answer within wait, one entry per serial and address: a massimo proxy
// answers with the serial of the Cube it serves, so a serial can come
// twice, from the Cube and from the proxy. It runs Search on a socket of
// its own, so each answer has its addresses in use, its DHCP flag, its
// MAC and its Kind. With unicast addresses given, the request goes to
// those instead of being broadcast (for other subnets, or NAT setups
// such as WSL 2).
func Discover(ctx context.Context, wait time.Duration, unicast ...net.IP) ([]Discovered, error) {
	return discover(ctx, wait, DiscoveryPort, DiscoveryPort, unicast...)
}

// discover is Discover binding one port and asking on another, for the
// tests: a fake Cube on the loopback holds the port asked on, and
// Windows, unlike Linux, lets the wildcard bind it too and hands the
// fake its own answers.
func discover(ctx context.Context, wait time.Duration, bind, port int, unicast ...net.IP) ([]Discovered, error) {
	pc, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", bind))
	if err != nil {
		// Fall back to any port: a Cube that answers to the source port
		// is still found; one that answers to 23272 is not.
		pc, err = net.ListenPacket("udp4", ":0")
		if err != nil {
			return nil, fmt.Errorf("discover: cannot bind a UDP port: %w", err)
		}
	}
	defer pc.Close()
	ips := unicast
	if len(ips) == 0 {
		ips = append([]net.IP{udpBroadcast, udpMulticast}, interfaceBroadcasts()...)
	}
	targets := make([]*net.UDPAddr, 0, len(ips))
	for _, ip := range ips {
		targets = append(targets, &net.UDPAddr{IP: ip, Port: port})
	}
	return Search(ctx, pc, SearchOptions{Targets: targets, Wait: wait})
}

func firmwareString(v uint16) string {
	return fmt.Sprintf("%d.%d.%d", v>>8, v>>4&0xf, v&0xf)
}

// interfaceBroadcasts returns the directed broadcast address of every
// IPv4 interface, so discovery also works where 255.255.255.255 is not
// routed to the right interface.
func interfaceBroadcasts() []net.IP {
	var out []net.IP
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagBroadcast == 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok || n.IP.To4() == nil {
				continue
			}
			ip := n.IP.To4()
			mask := net.IP(n.Mask).To4()
			if mask == nil {
				continue
			}
			bc := make(net.IP, 4)
			for i := range bc {
				bc[i] = ip[i] | ^mask[i]
			}
			out = append(out, bc)
		}
	}
	return out
}
