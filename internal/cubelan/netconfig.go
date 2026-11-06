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
// it, N reports the addresses in use, R restarts the Cube. Confirmed on
// a Cube with firmware 1.1.3 (2026-11-06) against a capture of the
// original software, see docs/portal-protocol.md: C has the layout of
// the c reply and is answered with one status byte, 1 when the new
// configuration applies at once (sent from the new address), 2 when the
// Cube restarts by itself (a write that turns DHCP on). The software
// sends R right after C. c reports the stored static configuration and
// the flags, not a DHCP lease; N gives the current addresses.

const (
	udpSetConfig = byte('C')
	udpReboot    = byte('R')
	// netConfigLen is the length of the c reply's data and of the C
	// payload: ip, gateway, netmask, dns1, dns2 (four bytes each), the
	// DHCP flag and the encryption flag (one byte each).
	netConfigLen = 5*4 + 2
)

// Status bytes of the Cube's answer to C (and to R).
const (
	// SetStatusOK says the configuration applies at once.
	SetStatusOK byte = 1
	// SetStatusRestarting says the Cube restarts to apply it.
	SetStatusRestarting byte = 2
)

// The waits around a write, variables so that tests can shorten them.
var (
	// setReplyWait is how long SetNetConfig waits for the answer to C,
	// and then to R.
	setReplyWait = time.Second
	// setSettle is the pause between the write and the first re-read.
	setSettle = time.Second
	// rereadWait is how long the re-read keeps trying, and
	// restartWait how long when the Cube restarts.
	rereadWait  = 3 * time.Second
	restartWait = 20 * time.Second
	// askWait is how long one request waits for its answer.
	askWait = time.Second
)

// ErrNoReply says that the Cube did not answer.
var ErrNoReply = errors.New("the cube did not answer")

// NetConfig is a Cube's network configuration as the c reply carries it
// and the C request writes it. With DHCP on, the addresses are the
// stored static ones (192.168.0.222/16 on a Cube from the factory), not
// the lease; the addresses in use come from N, see NetState.
type NetConfig struct {
	IP      net.IP `json:"ip"`
	Gateway net.IP `json:"gateway"`
	Netmask net.IP `json:"netmask"`
	// DNS holds up to two servers; an all-zero entry is left out.
	DNS []net.IP `json:"dns"`
	// DHCP says whether the Cube takes its address by DHCP.
	DHCP bool `json:"dhcp"`
	// Encryption is the Cube's encryption flag; Massimo never sets it
	// on its own, a write copies it from a read.
	Encryption bool `json:"encryption"`
}

// NetAddrs are the addresses a Cube uses now, from its N reply.
type NetAddrs struct {
	IP      net.IP   `json:"ip"`
	Gateway net.IP   `json:"gateway,omitempty"`
	Netmask net.IP   `json:"netmask,omitempty"`
	DNS     []net.IP `json:"dns,omitempty"`
}

// NetState is what a read of a Cube's network learns.
type NetState struct {
	Serial string `json:"serial"`
	// Stored is the c reply: the stored static configuration and the
	// flags.
	Stored NetConfig `json:"stored"`
	// Current is the N reply, the addresses in use; nil when the Cube
	// did not answer N.
	Current *NetAddrs `json:"current,omitempty"`
	// From is the address the Cube answered from.
	From *net.UDPAddr `json:"-"`
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

// parseNetAddrs decodes the data of an N reply: ip, then, when present,
// gateway, netmask and two DNS servers (four bytes each).
func parseNetAddrs(data []byte) (NetAddrs, bool) {
	var ips []net.IP
	for off := 0; off+4 <= len(data) && len(ips) < 5; off += 4 {
		ips = append(ips, net.IPv4(data[off], data[off+1], data[off+2], data[off+3]).To4())
	}
	if len(ips) == 0 {
		return NetAddrs{}, false
	}
	a := NetAddrs{IP: ips[0]}
	if len(ips) >= 3 {
		a.Gateway, a.Netmask = ips[1], ips[2]
	}
	for _, ip := range ips[min(3, len(ips)):] {
		if !ip.Equal(net.IPv4zero) {
			a.DNS = append(a.DNS, ip)
		}
	}
	return a, true
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
// byte each, 0 or 1): 41 bytes, as the original software sends it
// (confirmed 2026-11-06, firmware 1.1.3).
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

// RebootFrame builds the R request that restarts the Cube with the
// given serial: "eQ3Max*\0", the serial padded with '*', 'R'.
func RebootFrame(serial string) []byte { return udpRequest(serial, udpReboot) }

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

// reply is one answer that exchange picked out.
type reply struct {
	serial string
	data   []byte
	from   *net.UDPAddr
}

// exchange sends req to each target and returns the first reply with
// opcode op (and the serial, when one is given) that arrives within
// wait.
func exchange(ctx context.Context, pc net.PacketConn, targets []*net.UDPAddr, req []byte, serial string, op byte, wait time.Duration) (reply, bool) {
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
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			return reply{}, false
		}
		s, _, o, data, ok := parseUDPReply(buf[:n])
		if ok && o == op && (serial == "" || s == serial) {
			u, _ := from.(*net.UDPAddr)
			return reply{serial: s, data: append([]byte(nil), data...), from: u}, true
		}
	}
	return reply{}, false
}

// readState finds the Cube with serial through N at the targets and
// reads c from the address it answered from, trying until within has
// passed (and N at least once, c at least twice). A Cube that answers c
// but not N is read at the first target.
func readState(ctx context.Context, pc net.PacketConn, targets []*net.UDPAddr, serial string, within time.Duration) (NetState, error) {
	deadline := time.Now().Add(within)
	st := NetState{Serial: serial, From: targets[0]}
	for ctx.Err() == nil {
		if r, ok := exchange(ctx, pc, targets, udpRequest(serial, udpNetConfig), serial, udpNetConfig, askWait); ok {
			if a, ok := parseNetAddrs(r.data); ok {
				st.Current = &a
			}
			if r.from != nil {
				st.From = r.from
			}
			break
		}
		if time.Now().After(deadline) {
			break
		}
	}
	for try := 0; ; try++ {
		if r, ok := exchange(ctx, pc, []*net.UDPAddr{st.From}, udpRequest(serial, udpGetConfig), serial, udpGetConfig, askWait); ok {
			cfg, err := parseNetConfig(r.data)
			if err != nil {
				return st, err
			}
			st.Stored = cfg
			if r.from != nil {
				st.From = r.from
			}
			return st, nil
		}
		if ctx.Err() != nil {
			return st, fmt.Errorf("network configuration of %s: %w", serial, ctx.Err())
		}
		if try >= 1 && time.Now().After(deadline) {
			return st, fmt.Errorf("network configuration of %s: %w", serial, ErrNoReply)
		}
	}
}

// QueryNetConfig asks the Cube at addr (host or host:port, port 23272 by
// default) for its serial with I, for the addresses in use with N and
// for its stored configuration with c.
func QueryNetConfig(ctx context.Context, addr string) (NetState, error) {
	to, err := udpTarget(addr)
	if err != nil {
		return NetState{}, err
	}
	pc, err := listenDiscovery()
	if err != nil {
		return NetState{}, err
	}
	defer pc.Close()
	var id reply
	var ok bool
	for try := 0; try < 2 && !ok; try++ {
		id, ok = exchange(ctx, pc, []*net.UDPAddr{to}, udpRequest("", udpIdentify), "", udpIdentify, askWait)
	}
	if !ok {
		return NetState{}, fmt.Errorf("identify %s: %w", to, ErrNoReply)
	}
	return readState(ctx, pc, []*net.UDPAddr{to}, id.serial, askWait)
}

// SetResult is what SetNetConfig learned during and after the write.
type SetResult struct {
	// Status is the Cube's answer to C (SetStatusOK, SetStatusRestarting),
	// 0 when it did not answer.
	Status byte
	// RebootSent says whether R followed the C, and RebootStatus is the
	// Cube's answer to it, 0 when none came.
	RebootSent   bool
	RebootStatus byte
	// Now is what the Cube reports after the write.
	Now NetState
}

// Restarting says whether the Cube restarts after the write.
func (r SetResult) Restarting() bool {
	return r.Status == SetStatusRestarting || r.RebootStatus == SetStatusRestarting
}

// SetNetConfig writes cfg to the Cube with the given serial at addr
// (host or host:port, port 23272 by default; a broadcast address reaches
// the one Cube with that serial) as the original software does: C, and
// after a status 1 answer R, whose answer it waits for briefly. Status 2
// means the Cube restarts by itself, so no R follows. It then re-reads,
// allowing for the restart: N at addr, at the new address of a static
// write and, after a DHCP write or to a broadcast address, as a
// broadcast too, and c from the address that answered. The error wraps
// ErrNoReply when the re-read got no answer; the C was sent all the same.
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
	targets := []*net.UDPAddr{to}
	if !cfg.DHCP && cfg.IP != nil && !cfg.IP.Equal(to.IP) {
		targets = append(targets, &net.UDPAddr{IP: cfg.IP, Port: to.Port})
	}
	// R goes to where the Cube answered C from: a static write answers
	// from the new address.
	rebootTo := []*net.UDPAddr{to}
	if r, ok := exchange(ctx, pc, []*net.UDPAddr{to}, frame, serial, udpSetConfig, setReplyWait); ok && len(r.data) > 0 {
		res.Status = r.data[0]
		if r.from != nil {
			rebootTo = []*net.UDPAddr{r.from}
			if !r.from.IP.Equal(to.IP) && (cfg.IP == nil || !r.from.IP.Equal(cfg.IP)) {
				targets = append(targets, r.from)
			}
		}
	}
	if res.Status == SetStatusOK {
		res.RebootSent = true
		if r, ok := exchange(ctx, pc, rebootTo, RebootFrame(serial), serial, udpReboot, setReplyWait); ok && len(r.data) > 0 {
			res.RebootStatus = r.data[0]
		}
	}
	select {
	case <-ctx.Done():
		return res, fmt.Errorf("network configuration of %s: %w", serial, ctx.Err())
	case <-time.After(setSettle):
	}
	// After a DHCP write the lease may differ from the old address, so N
	// goes out as a broadcast too. A loopback target is a fake Cube, with
	// nothing to look for on the LAN.
	if (cfg.DHCP || to.IP.Equal(net.IPv4bcast)) && !to.IP.IsLoopback() {
		targets = append(targets, DiscoveryTargets()...)
	}
	within := rereadWait
	if res.Restarting() || res.RebootSent {
		within = restartWait
	}
	res.Now, err = readState(ctx, pc, targets, serial, within)
	return res, err
}
