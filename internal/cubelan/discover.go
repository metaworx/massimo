// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"bytes"
	"context"
	"encoding/binary"
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
	MAC     string   `json:"mac,omitempty"` // from the ARP cache, when known
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
// answer within wait. Each one is then asked for its network settings.
func Discover(ctx context.Context, wait time.Duration) ([]Discovered, error) {
	pc, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", DiscoveryPort))
	if err != nil {
		return nil, fmt.Errorf("discover: cannot bind UDP port %d (is the original MAX! software running?): %w", DiscoveryPort, err)
	}
	defer pc.Close()
	req := udpRequest("", udpIdentify)
	targets := []net.IP{udpBroadcast, udpMulticast}
	for _, ip := range interfaceBroadcasts() {
		targets = append(targets, ip)
	}
	for i := 0; i < 2; i++ {
		for _, ip := range targets {
			pc.WriteTo(req, &net.UDPAddr{IP: ip, Port: DiscoveryPort})
		}
		time.Sleep(50 * time.Millisecond)
	}
	found := map[string]*Discovered{}
	var order []string
	deadline := time.Now().Add(wait)
	buf := make([]byte, 1024)
	for {
		if ctx.Err() != nil {
			break
		}
		pc.SetReadDeadline(deadline)
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			break
		}
		serial, boot, op, data, ok := parseUDPReply(buf[:n])
		if !ok || op != udpIdentify || len(data) < 6 {
			continue
		}
		if _, seen := found[serial]; seen {
			continue
		}
		d := &Discovered{
			Serial:     serial,
			IP:         from.(*net.UDPAddr).IP,
			RFAddress:  cube.RFAddress(binary.BigEndian.Uint32(data[0:4])),
			Firmware:   firmwareString(binary.BigEndian.Uint16(data[4:6])),
			Bootloader: boot,
		}
		found[serial] = d
		order = append(order, serial)
	}
	var out []Discovered
	for _, s := range order {
		d := found[s]
		queryNetwork(pc, d)
		d.MAC = lookupMAC(d.IP)
		out = append(out, *d)
	}
	return out, nil
}

// queryNetwork sends N to one Cube and fills in what it answers: the
// original software reads the IP only; other clients report ip, gateway,
// netmask and two DNS servers.
func queryNetwork(pc net.PacketConn, d *Discovered) {
	pc.WriteTo(udpRequest(d.Serial, udpNetConfig), &net.UDPAddr{IP: d.IP, Port: DiscoveryPort})
	buf := make([]byte, 1024)
	pc.SetReadDeadline(time.Now().Add(time.Second))
	for {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		serial, _, op, data, ok := parseUDPReply(buf[:n])
		if !ok || op != udpNetConfig || serial != d.Serial {
			continue
		}
		ips := make([]net.IP, 0, 5)
		for off := 0; off+4 <= len(data) && len(ips) < 5; off += 4 {
			ips = append(ips, net.IPv4(data[off], data[off+1], data[off+2], data[off+3]))
		}
		if len(ips) >= 3 {
			d.Gateway, d.Netmask = ips[1], ips[2]
		}
		for _, ip := range ips[min(3, len(ips)):] {
			if !ip.Equal(net.IPv4zero) {
				d.DNS = append(d.DNS, ip)
			}
		}
		return
	}
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
