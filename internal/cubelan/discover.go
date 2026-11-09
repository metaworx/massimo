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
// twice, from the Cube and from the proxy. Each one is then asked for its
// network settings. With unicast addresses given, the request goes to
// those instead of being broadcast (for other subnets, or NAT setups
// such as WSL 2).
func Discover(ctx context.Context, wait time.Duration, unicast ...net.IP) ([]Discovered, error) {
	return discover(ctx, wait, DiscoveryPort, unicast...)
}

// discover is Discover on another port than the Cube's, for the tests.
func discover(ctx context.Context, wait time.Duration, port int, unicast ...net.IP) ([]Discovered, error) {
	pc, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		// Fall back to any port: a Cube that answers to the source port
		// is still found; one that answers to 23272 is not.
		pc, err = net.ListenPacket("udp4", ":0")
		if err != nil {
			return nil, fmt.Errorf("discover: cannot bind a UDP port: %w", err)
		}
	}
	defer pc.Close()
	req := udpRequest("", udpIdentify)
	targets := unicast
	if len(targets) == 0 {
		targets = []net.IP{udpBroadcast, udpMulticast}
		for _, ip := range interfaceBroadcasts() {
			targets = append(targets, ip)
		}
	}
	for i := 0; i < 2; i++ {
		for _, ip := range targets {
			pc.WriteTo(req, &net.UDPAddr{IP: ip, Port: port})
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
		ip := from.(*net.UDPAddr).IP
		key := serial + "|" + ip.String()
		if _, seen := found[key]; seen {
			continue
		}
		d := &Discovered{
			Serial:     serial,
			IP:         ip,
			RFAddress:  cube.RFAddress(binary.BigEndian.Uint32(data[0:4])),
			Firmware:   firmwareString(binary.BigEndian.Uint16(data[4:6])),
			Bootloader: boot,
		}
		found[key] = d
		order = append(order, key)
	}
	var out []Discovered
	for _, s := range order {
		d := found[s]
		// A massimo proxy answers neither N nor c; skip the second wait.
		if queryNetwork(pc, d, port) {
			queryConfig(pc, &net.UDPAddr{IP: d.IP, Port: port}, d)
		}
		d.MAC = lookupMAC(d.IP)
		out = append(out, *d)
	}
	return out, nil
}

// ask sends the request op to one Cube at to and returns the data of
// its reply, waiting a second at most.
func ask(pc net.PacketConn, to net.Addr, serial string, op byte) ([]byte, bool) {
	pc.WriteTo(udpRequest(serial, op), to)
	buf := make([]byte, 1024)
	pc.SetReadDeadline(time.Now().Add(time.Second))
	for {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			return nil, false
		}
		s, _, o, data, ok := parseUDPReply(buf[:n])
		if ok && o == op && s == serial {
			return data, true
		}
	}
}

// queryConfig sends c to one Cube and fills in its DHCP flag. The reply
// carries ip, gateway, netmask, dns1, dns2 (four bytes each), then the
// DHCP flag and the encryption flag, one byte each (confirmed on a Cube
// with firmware 1.1.3); the addresses are the stored static ones, not a
// DHCP lease, so only the flag is taken here.
func queryConfig(pc net.PacketConn, to net.Addr, d *Discovered) {
	data, ok := ask(pc, to, d.Serial, udpGetConfig)
	if !ok || len(data) < 21 {
		return
	}
	dhcp := data[20] != 0
	d.DHCP = &dhcp
}

// queryNetwork sends N to one Cube and fills in what it answers: the
// original software reads the IP only; other clients report ip, gateway,
// netmask and two DNS servers. It reports whether the Cube answered.
func queryNetwork(pc net.PacketConn, d *Discovered, port int) bool {
	data, ok := ask(pc, &net.UDPAddr{IP: d.IP, Port: port}, d.Serial, udpNetConfig)
	if !ok {
		return false
	}
	if a, ok := parseNetAddrs(data); ok {
		d.Gateway, d.Netmask, d.DNS = a.Gateway, a.Netmask, a.DNS
	}
	return true
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
