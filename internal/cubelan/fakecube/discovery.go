// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync/atomic"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// DiscoveryResponder answers UDP discovery requests like a Cube. A real
// Cube answers to port 23272; this one answers to the request's source
// address, so that a test needs no fixed port.
type DiscoveryResponder struct {
	Serial    string
	RFAddress cube.RFAddress
	Firmware  uint16 // e.g. 0x0113

	pc     net.PacketConn
	silent atomic.Bool
	static atomic.Bool
}

// SetStatic makes the c reply say the Cube has a static address instead
// of DHCP, the factory default.
func (d *DiscoveryResponder) SetStatic(static bool) { d.static.Store(static) }

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// StartDiscovery listens on addr ("127.0.0.1:0" for a random port).
func StartDiscovery(addr, serial string, rf cube.RFAddress) (*DiscoveryResponder, error) {
	pc, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return nil, err
	}
	d := &DiscoveryResponder{Serial: serial, RFAddress: rf, Firmware: 0x0113, pc: pc}
	go d.serve()
	return d, nil
}

// Addr is the address requests go to.
func (d *DiscoveryResponder) Addr() string { return d.pc.LocalAddr().String() }

// SetSilent stops or resumes the answers, as a Cube that left or joined
// the network.
func (d *DiscoveryResponder) SetSilent(silent bool) { d.silent.Store(silent) }

// Close stops the responder.
func (d *DiscoveryResponder) Close() error { return d.pc.Close() }

func (d *DiscoveryResponder) serve() {
	buf := make([]byte, 1024)
	for {
		n, from, err := d.pc.ReadFrom(buf)
		if err != nil {
			return
		}
		// Request: "eQ3Max*\0", ten characters of serial ('*' padded, all
		// '*' for any Cube), opcode. The fake answers I and c.
		b := buf[:n]
		if n < 19 || !bytes.Equal(b[:8], []byte("eQ3Max*\x00")) || (b[18] != 'I' && b[18] != 'c') || d.silent.Load() {
			continue
		}
		if want := strings.TrimRight(string(b[8:18]), "*"); want != "" && want != d.Serial {
			continue
		}
		// Reply: "eQ3MaxAp", serial, '>', opcode, data. For I the radio
		// address (4 bytes) and the firmware (2 bytes), big endian; for c
		// ip, gateway, netmask, dns1, dns2 (4 bytes each, zero here), the
		// DHCP flag and the encryption flag (one byte each).
		reply := append([]byte("eQ3MaxAp"), fmt.Sprintf("%-10.10s", d.Serial)...)
		reply = append(reply, '>', b[18])
		if b[18] == 'I' {
			reply = binary.BigEndian.AppendUint32(reply, uint32(d.RFAddress))
			reply = binary.BigEndian.AppendUint16(reply, d.Firmware)
		} else {
			reply = append(reply, make([]byte, 20)...)
			reply = append(reply, boolByte(!d.static.Load()), 0)
		}
		d.pc.WriteTo(reply, from)
	}
}
