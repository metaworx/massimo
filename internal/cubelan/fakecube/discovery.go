// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// netConfigLen is the length of the c reply's data and of the C payload.
const netConfigLen = 22

// DiscoveryResponder answers UDP discovery requests like a Cube. A real
// Cube answers to port 23272; this one answers to the request's source
// address, so that a test needs no fixed port.
type DiscoveryResponder struct {
	Serial    string
	RFAddress cube.RFAddress
	Firmware  uint16 // e.g. 0x0113

	pc     net.PacketConn
	silent atomic.Bool

	mu sync.Mutex
	// netConfig is what the c reply carries and a C request replaces:
	// ip, gateway, netmask, dns1, dns2 (four bytes each), the DHCP flag
	// and the encryption flag (one byte each).
	netConfig [netConfigLen]byte
	writes    int
}

// SetStatic makes the c reply say the Cube has a static address instead
// of DHCP, the factory default, by changing the stored DHCP flag.
func (d *DiscoveryResponder) SetStatic(static bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.netConfig[20] = boolByte(!static)
}

// NetConfig returns the stored network configuration, in the layout of
// the c reply, and how many C requests have replaced it.
func (d *DiscoveryResponder) NetConfig() ([]byte, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]byte(nil), d.netConfig[:]...), d.writes
}

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
	d.netConfig[20] = 1 // DHCP, as a Cube from the factory
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
		// '*' for any Cube), opcode, payload. The fake answers I and c,
		// and takes C without an answer: whether a Cube answers C is not
		// known.
		b := buf[:n]
		if n < 19 || !bytes.Equal(b[:8], []byte("eQ3Max*\x00")) || d.silent.Load() {
			continue
		}
		op := b[18]
		if op != 'I' && op != 'c' && op != 'C' {
			continue
		}
		want := strings.TrimRight(string(b[8:18]), "*")
		if want != "" && want != d.Serial {
			continue
		}
		if op == 'C' {
			// A write names its Cube; the payload has the c reply's layout.
			if want == d.Serial && n >= 19+netConfigLen {
				d.mu.Lock()
				copy(d.netConfig[:], b[19:19+netConfigLen])
				d.writes++
				d.mu.Unlock()
			}
			continue
		}
		// Reply: "eQ3MaxAp", serial, '>', opcode, data. For I the radio
		// address (4 bytes) and the firmware (2 bytes), big endian; for c
		// the stored network configuration.
		reply := append([]byte("eQ3MaxAp"), fmt.Sprintf("%-10.10s", d.Serial)...)
		reply = append(reply, '>', op)
		if op == 'I' {
			reply = binary.BigEndian.AppendUint32(reply, uint32(d.RFAddress))
			reply = binary.BigEndian.AppendUint16(reply, d.Firmware)
		} else {
			d.mu.Lock()
			reply = append(reply, d.netConfig[:]...)
			d.mu.Unlock()
		}
		d.pc.WriteTo(reply, from)
	}
}
