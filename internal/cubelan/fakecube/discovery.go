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
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// netConfigLen is the length of the c reply's data and of the C payload.
const netConfigLen = 22

// DiscoveryResponder answers UDP discovery requests like a Cube. A real
// Cube answers to port 23272; this one answers to the request's source
// address, so that a test needs no fixed port. It answers I, N and c,
// and takes C and R as a Cube with firmware 1.1.3 does: C is answered
// with status 1 for a static configuration and 2 for DHCP, after which
// it restarts; R is answered with 2 and restarts it. While restarting it
// answers nothing. It never changes its own address.
type DiscoveryResponder struct {
	Serial    string
	RFAddress cube.RFAddress
	Firmware  uint16 // e.g. 0x0113

	pc       net.PacketConn
	silent   atomic.Bool
	noConfig atomic.Bool

	mu sync.Mutex
	// netConfig is what the c reply carries and a C request replaces:
	// ip, gateway, netmask, dns1, dns2 (four bytes each), the DHCP flag
	// and the encryption flag (one byte each).
	netConfig [netConfigLen]byte
	// lease is what N reports while DHCP is on: ip, gateway, netmask,
	// dns1, dns2.
	lease   [20]byte
	writes  int
	reboots int
	// restartUntil is when a running restart ends, restartFor how long
	// one takes.
	restartUntil time.Time
	restartFor   time.Duration
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

// Reboots counts the R requests the fake has taken.
func (d *DiscoveryResponder) Reboots() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.reboots
}

// SetRestartDelay sets how long a restart after C or R keeps the fake
// silent (300 ms by default).
func (d *DiscoveryResponder) SetRestartDelay(t time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.restartFor = t
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
	d := &DiscoveryResponder{Serial: serial, RFAddress: rf, Firmware: 0x0113, pc: pc, restartFor: 300 * time.Millisecond}
	// As a Cube from the factory: DHCP on, the stored static fallback
	// 192.168.0.222/16 with gateway and DNS 192.168.0.1, encryption on.
	copy(d.netConfig[:], []byte{192, 168, 0, 222, 192, 168, 0, 1, 255, 255, 0, 0, 192, 168, 0, 1, 0, 0, 0, 0, 1, 1})
	// The lease: the fake's own address in a /24, gateway and DNS .1.
	ip := net.IPv4(127, 0, 0, 1).To4()
	if u, ok := pc.LocalAddr().(*net.UDPAddr); ok && u.IP.To4() != nil && !u.IP.IsUnspecified() {
		ip = u.IP.To4()
	}
	gw := []byte{ip[0], ip[1], ip[2], 1}
	lease := append(append([]byte(nil), ip...), gw...)
	lease = append(append(lease, 255, 255, 255, 0), gw...)
	copy(d.lease[:], lease)
	go d.serve()
	return d, nil
}

// Addr is the address requests go to.
func (d *DiscoveryResponder) Addr() string { return d.pc.LocalAddr().String() }

// SetSilent stops or resumes the answers, as a Cube that left or joined
// the network.
func (d *DiscoveryResponder) SetSilent(silent bool) { d.silent.Store(silent) }

// SetConfigSilent stops or resumes the answers to c only, as a Cube
// whose answer gets lost.
func (d *DiscoveryResponder) SetConfigSilent(silent bool) { d.noConfig.Store(silent) }

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
		// '*' for any Cube), opcode, payload.
		b := buf[:n]
		if n < 19 || !bytes.Equal(b[:8], []byte("eQ3Max*\x00")) || d.silent.Load() {
			continue
		}
		op := b[18]
		want := strings.TrimRight(string(b[8:18]), "*")
		if want != "" && want != d.Serial {
			continue
		}
		data, ok := d.answer(op, want == d.Serial, b[19:])
		if !ok {
			continue
		}
		// Reply: "eQ3MaxAp", serial, '>', opcode, data.
		reply := append([]byte("eQ3MaxAp"), fmt.Sprintf("%-10.10s", d.Serial)...)
		reply = append(reply, '>', op)
		reply = append(reply, data...)
		d.pc.WriteTo(reply, from)
	}
}

// answer handles one request with opcode op and payload p; named says
// whether it carried this Cube's serial rather than the any-Cube one.
// It returns the reply's data and whether to answer at all.
func (d *DiscoveryResponder) answer(op byte, named bool, p []byte) ([]byte, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if time.Now().Before(d.restartUntil) {
		return nil, false
	}
	switch op {
	case 'I': // the radio address (4 bytes) and the firmware (2 bytes), big endian
		out := binary.BigEndian.AppendUint32(nil, uint32(d.RFAddress))
		return binary.BigEndian.AppendUint16(out, d.Firmware), true
	case 'N': // the addresses in use: the lease with DHCP, else the stored ones
		if d.netConfig[20] != 0 {
			return append([]byte(nil), d.lease[:]...), true
		}
		return append([]byte(nil), d.netConfig[:20]...), true
	case 'c': // the stored configuration and the flags
		if d.noConfig.Load() {
			return nil, false
		}
		return append([]byte(nil), d.netConfig[:]...), true
	case 'C': // a write names its Cube; the payload has the c reply's layout
		if !named || len(p) < netConfigLen {
			return nil, false
		}
		copy(d.netConfig[:], p[:netConfigLen])
		d.writes++
		if d.netConfig[20] != 0 {
			d.restartUntil = time.Now().Add(d.restartFor)
			return []byte{2}, true
		}
		return []byte{1}, true
	case 'R':
		if !named {
			return nil, false
		}
		d.reboots++
		d.restartUntil = time.Now().Add(d.restartFor)
		return []byte{2}, true
	}
	return nil, false
}
