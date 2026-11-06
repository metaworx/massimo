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
