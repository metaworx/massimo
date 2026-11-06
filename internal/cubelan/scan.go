// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"encoding/binary"
	"net"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Pieces of discovery for a program that holds the discovery port
// itself, such as serve, whose LAN proxy answers discovery on it: the
// Cubes answer to port 23272 whatever the request's source port, so the
// request has to go out from that socket and the answers are picked out
// of what it reads.

// CubePort is the Cube's TCP port.
const CubePort = 62910

// IdentifyRequest is the discovery request every Cube answers.
func IdentifyRequest() []byte { return udpRequest("", udpIdentify) }

// ParseIdentifyReply decodes a Cube's answer to IdentifyRequest that
// came from the address from; ok is false for anything else, requests
// included.
func ParseIdentifyReply(b []byte, from net.IP) (Discovered, bool) {
	serial, boot, op, data, ok := parseUDPReply(b)
	if !ok || op != udpIdentify || len(data) < 6 {
		return Discovered{}, false
	}
	return Discovered{
		Serial:     serial,
		IP:         from,
		RFAddress:  cube.RFAddress(binary.BigEndian.Uint32(data[0:4])),
		Firmware:   firmwareString(binary.BigEndian.Uint16(data[4:6])),
		Bootloader: boot,
	}, true
}

// ConfigRequest asks the Cube with serial for its stored network
// configuration and flags (c), a read that sends nothing over the radio.
func ConfigRequest(serial string) []byte { return udpRequest(serial, udpGetConfig) }

// ParseConfigReply decodes a Cube's answer to ConfigRequest: the serial
// it names and its DHCP flag; ok is false for anything else.
func ParseConfigReply(b []byte) (serial string, dhcp bool, ok bool) {
	serial, _, op, data, ok := parseUDPReply(b)
	if !ok || op != udpGetConfig {
		return "", false, false
	}
	cfg, err := parseNetConfig(data)
	if err != nil {
		return "", false, false
	}
	return serial, cfg.DHCP, true
}

// DiscoveryTargets are the addresses a discovery request goes to: the
// limited broadcast, the multicast group and every interface's directed
// broadcast, on DiscoveryPort.
func DiscoveryTargets() []*net.UDPAddr {
	ips := append([]net.IP{udpBroadcast, udpMulticast}, interfaceBroadcasts()...)
	out := make([]*net.UDPAddr, 0, len(ips))
	for _, ip := range ips {
		out = append(out, &net.UDPAddr{IP: ip, Port: DiscoveryPort})
	}
	return out
}

// LookupMAC returns the MAC address the operating system's ARP cache
// has for ip, or "" (best effort).
func LookupMAC(ip net.IP) string { return lookupMAC(ip) }
