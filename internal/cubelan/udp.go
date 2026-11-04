// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"bytes"
	"encoding/binary"
	"strings"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// ParseUDPRequest decodes a discovery request as a client sends it:
// signature, serial (10 characters, '*' padded; all '*' means any Cube),
// opcode. serial is returned without the padding, empty for "any".
func ParseUDPRequest(b []byte) (serial string, op byte, ok bool) {
	if len(b) < len(udpSignature)+10+1 || !bytes.Equal(b[:len(udpSignature)], udpSignature) {
		return "", 0, false
	}
	s := b[len(udpSignature) : len(udpSignature)+10]
	if bytes.Equal(s, udpAnySerial) {
		return "", b[len(udpSignature)+10], true
	}
	return strings.TrimRight(string(s), "*"), b[len(udpSignature)+10], true
}

// UDPReply builds the answer a Cube gives to a discovery request:
// signature, serial, '>', opcode, data. For opcode 'I' data is the radio
// address (4 bytes) and the firmware (2 bytes), big endian.
func UDPReply(serial string, op byte, data []byte) []byte {
	s := []byte(serial)
	for len(s) < 10 {
		s = append(s, '*')
	}
	p := append([]byte(nil), udpReplyApp...)
	p = append(p, s[:10]...)
	p = append(p, '>', op)
	return append(p, data...)
}

// UDPIdentifyData encodes the payload of an 'I' reply.
func UDPIdentifyData(rf cube.RFAddress, firmware uint16) []byte {
	var d [6]byte
	binary.BigEndian.PutUint32(d[0:4], uint32(rf))
	binary.BigEndian.PutUint16(d[4:6], firmware)
	return d[:]
}

// UDPIdentify is the opcode of the identify request and reply.
const UDPIdentify = 'I'
