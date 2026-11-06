// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

// The Cube's network configuration over UDP 23272: c reads it, C writes
// it. The c reply is confirmed on a Cube with firmware 1.1.3 for the
// DHCP case (2026-11-06); the C payload mirrors the c reply and is not
// yet confirmed on hardware, see docs/portal-protocol.md.

const (
	udpSetConfig = byte('C')
	// netConfigLen is the length of the c reply's data and of the C
	// payload: ip, gateway, netmask, dns1, dns2 (four bytes each), the
	// DHCP flag and the encryption flag (one byte each).
	netConfigLen = 5*4 + 2
)

// The pauses around a write, variables so that tests can shorten them.
var (
	// setReplyWait is how long SetNetConfig waits for an answer to C.
	setReplyWait = time.Second
	// setSettle is the pause between the C and the c that re-reads.
	setSettle = time.Second
	// askWait is how long one c or I request waits for its answer.
	askWait = time.Second
)

// ErrNoReply says that the Cube did not answer.
var ErrNoReply = errors.New("the cube did not answer")

// NetConfig is a Cube's network configuration as the c reply carries it
// and the C request writes it.
type NetConfig struct {
	IP      net.IP `json:"ip"`
	Gateway net.IP `json:"gateway"`
	Netmask net.IP `json:"netmask"`
	// DNS holds up to two servers; an all-zero entry is left out.
	DNS []net.IP `json:"dns"`
	// DHCP says whether the Cube takes its address by DHCP; the
	// addresses are then the ones it fell back to or was given.
	DHCP bool `json:"dhcp"`
	// Encryption is the Cube's encryption flag; Massimo never sets it
	// on its own, a write copies it from a read.
	Encryption bool `json:"encryption"`
}

// parseNetConfig decodes the data of a c reply.
func parseNetConfig(data []byte) (NetConfig, error) {
	if len(data) < netConfigLen {
		return NetConfig{}, fmt.Errorf("network configuration: %d bytes, want %d", len(data), netConfigLen)
	}
	ip := func(off int) net.IP { return net.IPv4(data[off], data[off+1], data[off+2], data[off+3]).To4() }
	cfg := NetConfig{IP: ip(0), Gateway: ip(4), Netmask: ip(8), DHCP: data[20] != 0, Encryption: data[21] != 0}
	for _, off := range []int{12, 16} {
		if d := ip(off); !d.Equal(net.IPv4zero) {
			cfg.DNS = append(cfg.DNS, d)
		}
	}
	return cfg, nil
}

// appendIPv4 appends ip as four bytes; nil is 0.0.0.0.
func appendIPv4(b []byte, ip net.IP, what string) ([]byte, error) {
	if ip == nil {
		return append(b, 0, 0, 0, 0), nil
	}
	v4 := ip.To4()
	if v4 == nil {
		return nil, fmt.Errorf("network configuration: %s %s is not an IPv4 address", what, ip)
	}
	return append(b, v4...), nil
}

// NetConfigFrame builds the C request that writes cfg to the Cube with
// the given serial: "eQ3Max*\0", the serial padded with '*' to ten
// characters, 'C', then ip, gateway, netmask, dns1, dns2 (four bytes
// each, missing ones zero), the DHCP flag and the encryption flag (one
// byte each, 0 or 1): 41 bytes. The payload mirrors the c reply; it is
// not yet confirmed on hardware.
func NetConfigFrame(serial string, cfg NetConfig) ([]byte, error) {
	if serial == "" || len(serial) > 10 {
		return nil, fmt.Errorf("network configuration: serial %q, want one to ten characters", serial)
	}
	if len(cfg.DNS) > 2 {
		return nil, fmt.Errorf("network configuration: %d DNS servers, at most two", len(cfg.DNS))
	}
	b := udpRequest(serial, udpSetConfig)
	var err error
	fields := []struct {
		ip   net.IP
		what string
	}{{cfg.IP, "ip"}, {cfg.Gateway, "gateway"}, {cfg.Netmask, "netmask"}, {nil, "dns1"}, {nil, "dns2"}}
	for i, d := range cfg.DNS {
		fields[3+i].ip = d
	}
	for _, f := range fields {
		if b, err = appendIPv4(b, f.ip, f.what); err != nil {
			return nil, err
		}
	}
	return append(b, boolByte(cfg.DHCP), boolByte(cfg.Encryption)), nil
}

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// udpTarget resolves host or host:port, the port defaulting to
// DiscoveryPort.
func udpTarget(addr string) (*net.UDPAddr, error) {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, strconv.Itoa(DiscoveryPort))
	}
	a, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("network configuration: address %q: %w", addr, err)
	}
	return a, nil
}

// listenDiscovery binds the discovery port, which a real Cube answers
// to, or any port when it is taken (a fake Cube answers to the source).
func listenDiscovery() (net.PacketConn, error) {
	pc, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", DiscoveryPort))
	if err != nil {
		if pc, err = net.ListenPacket("udp4", ":0"); err != nil {
			return nil, fmt.Errorf("network configuration: cannot bind a UDP port: %w", err)
		}
	}
	return pc, nil
}

// exchange sends req to each target and returns the serial and data of
// the first reply with opcode op (and the serial, when one is given)
// that arrives within wait.
func exchange(ctx context.Context, pc net.PacketConn, targets []*net.UDPAddr, req []byte, serial string, op byte, wait time.Duration) (string, []byte, bool) {
	for _, to := range targets {
		pc.WriteTo(req, to)
	}
	deadline := time.Now().Add(wait)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	buf := make([]byte, 1024)
	pc.SetReadDeadline(deadline)
	for ctx.Err() == nil {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			return "", nil, false
		}
		s, _, o, data, ok := parseUDPReply(buf[:n])
		if ok && o == op && (serial == "" || s == serial) {
			return s, append([]byte(nil), data...), true
		}
	}
	return "", nil, false
}

// readNetConfig sends c for serial to the targets, twice at most.
func readNetConfig(ctx context.Context, pc net.PacketConn, targets []*net.UDPAddr, serial string) (NetConfig, error) {
	for try := 0; try < 2; try++ {
		if _, data, ok := exchange(ctx, pc, targets, udpRequest(serial, udpGetConfig), serial, udpGetConfig, askWait); ok {
			return parseNetConfig(data)
		}
	}
	return NetConfig{}, fmt.Errorf("network configuration of %s: %w", serial, ErrNoReply)
}

// QueryNetConfig asks the Cube at addr (host or host:port, port 23272 by
// default) for its serial with I and then for its network configuration
// with c.
func QueryNetConfig(ctx context.Context, addr string) (serial string, cfg NetConfig, err error) {
	to, err := udpTarget(addr)
	if err != nil {
		return "", NetConfig{}, err
	}
	pc, err := listenDiscovery()
	if err != nil {
		return "", NetConfig{}, err
	}
	defer pc.Close()
	targets := []*net.UDPAddr{to}
	var ok bool
	for try := 0; try < 2 && !ok; try++ {
		serial, _, ok = exchange(ctx, pc, targets, udpRequest("", udpIdentify), "", udpIdentify, askWait)
	}
	if !ok {
		return "", NetConfig{}, fmt.Errorf("identify %s: %w", to, ErrNoReply)
	}
	cfg, err = readNetConfig(ctx, pc, targets, serial)
	return serial, cfg, err
}

// SetResult is what SetNetConfig learned after the write.
type SetResult struct {
	// Answered says whether the Cube answered the C itself, and Reply
	// is the configuration that answer carried, when it carried one.
	Answered bool
	Reply    *NetConfig
	// Now is what the Cube reports to a c read after the write.
	Now NetConfig
}

// SetNetConfig sends the C request for cfg to the Cube with the given
// serial at addr (host or host:port, port 23272 by default; a broadcast
// address reaches the one Cube with that serial). Whether a Cube answers
// C is unknown: it waits briefly for an answer, then pauses and re-reads
// with c, at addr and, for a static configuration, at the new address
// too. The error wraps ErrNoReply when the re-read got no answer; the
// C was sent all the same.
func SetNetConfig(ctx context.Context, addr, serial string, cfg NetConfig) (SetResult, error) {
	frame, err := NetConfigFrame(serial, cfg)
	if err != nil {
		return SetResult{}, err
	}
	to, err := udpTarget(addr)
	if err != nil {
		return SetResult{}, err
	}
	pc, err := listenDiscovery()
	if err != nil {
		return SetResult{}, err
	}
	defer pc.Close()
	var res SetResult
	if _, data, ok := exchange(ctx, pc, []*net.UDPAddr{to}, frame, serial, udpSetConfig, setReplyWait); ok {
		res.Answered = true
		if c, err := parseNetConfig(data); err == nil {
			res.Reply = &c
		}
	}
	select {
	case <-ctx.Done():
		return res, fmt.Errorf("network configuration of %s: %w", serial, ctx.Err())
	case <-time.After(setSettle):
	}
	targets := []*net.UDPAddr{to}
	if !cfg.DHCP && cfg.IP != nil && !cfg.IP.Equal(to.IP) {
		targets = append(targets, &net.UDPAddr{IP: cfg.IP, Port: to.Port})
	}
	res.Now, err = readNetConfig(ctx, pc, targets, serial)
	return res, err
}
