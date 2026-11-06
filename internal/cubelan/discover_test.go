// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"encoding/binary"
	"net"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

func TestUDPIdentifyReply(t *testing.T) {
	// Captured on the wire from a Cube with firmware 1.1.3.
	pkt := []byte("eQ3MaxApKEQ0537249>I\x00\x0b\x95\x73\x01\x13")
	serial, boot, op, data, ok := parseUDPReply(pkt)
	if !ok || boot || op != 'I' || serial != "KEQ0537249" {
		t.Fatalf("parse: %q %v %c %v", serial, boot, op, ok)
	}
	if rf := binary.BigEndian.Uint32(data[0:4]); rf != 0x0b9573 {
		t.Fatalf("rf %06x", rf)
	}
	if fw := firmwareString(binary.BigEndian.Uint16(data[4:6])); fw != "1.1.3" {
		t.Fatalf("fw %s", fw)
	}
	if req := udpRequest("", 'I'); string(req) != "eQ3Max*\x00**********I" {
		t.Fatalf("request %q", req)
	}
	if req := udpRequest("KEQ0537249", 'N'); string(req) != "eQ3Max*\x00KEQ0537249N" {
		t.Fatalf("request %q", req)
	}
	if _, _, _, _, ok := parseUDPReply([]byte("eQ3Max*\x00**********I")); ok {
		t.Fatal("our own request parsed as a reply")
	}
}

func TestConfigRequestAndReply(t *testing.T) {
	if req := ConfigRequest("KEQ0000000"); string(req) != "eQ3Max*\x00KEQ0000000c" {
		t.Fatalf("request %q", req)
	}
	// The layout of a c reply from a Cube with firmware 1.1.3: the stored
	// ip, gateway, netmask, dns1, dns2, then the DHCP and encryption flags.
	cfg := "\xc0\xa8\x00\xde\xc0\xa8\x00\x01\xff\xff\x00\x00\xc0\xa8\x00\x01\x00\x00\x00\x00"
	for _, tc := range []struct {
		flags string
		dhcp  bool
	}{{"\x01\x01", true}, {"\x00\x01", false}} {
		serial, dhcp, ok := ParseConfigReply([]byte("eQ3MaxApKEQ0000000>c" + cfg + tc.flags))
		if !ok || serial != "KEQ0000000" || dhcp != tc.dhcp {
			t.Fatalf("flags %q: %q %v %v", tc.flags, serial, dhcp, ok)
		}
	}
	for _, b := range []string{"eQ3MaxApKEQ0000000>c" + cfg, "eQ3MaxApKEQ0000000>N" + cfg + "\x01\x01", "eQ3Max*\x00KEQ0000000c"} {
		if _, _, ok := ParseConfigReply([]byte(b)); ok {
			t.Fatalf("%q parsed as a c reply", b)
		}
	}
}

func TestQueryConfigReadsTheDHCPFlag(t *testing.T) {
	fake, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0b9573)
	if err != nil {
		t.Fatal(err)
	}
	defer fake.Close()
	to, err := net.ResolveUDPAddr("udp4", fake.Addr())
	if err != nil {
		t.Fatal(err)
	}
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	for _, static := range []bool{false, true} {
		fake.SetStatic(static)
		d := &Discovered{Serial: "KEQ0000000"}
		queryConfig(pc, to, d)
		if d.DHCP == nil || *d.DHCP == static {
			t.Fatalf("static %v: dhcp %v", static, d.DHCP)
		}
	}
	// Another serial gets no answer and no flag.
	d := &Discovered{Serial: "KEQ0000001"}
	queryConfig(pc, to, d)
	if d.DHCP != nil {
		t.Fatalf("other cube: dhcp %v", *d.DHCP)
	}
}
