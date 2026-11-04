// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"encoding/binary"
	"testing"
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
