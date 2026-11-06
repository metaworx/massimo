// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"

	"go.metaworx.ch/massimo/internal/cubelan"
)

const netconfigHelp = `usage: massimo netconfig -addr <cube-ip> [-json]
       massimo netconfig -addr <cube-ip> -static <ip> -gateway <gw> -netmask <mask> [-dns <a>[,<b>]] [-dry-run] [-yes]
       massimo netconfig -addr <cube-ip> -dhcp [-dry-run] [-yes]
       massimo netconfig -sn <serial> -broadcast (-dhcp [-encryption] | -static ...) [-dry-run] [-yes]

Read or write a Cube's network configuration over UDP 23272, as the
original software's network setup does: c reads the stored static
configuration and the DHCP flag, N the addresses in use (with DHCP on,
the lease; the stored ones are then only the fallback), C writes.

Without -static or -dhcp it prints both. -static writes a static address
with DHCP off; without -dns the DNS servers as read are kept. -dhcp
switches DHCP on and keeps the stored addresses as read. The encryption
flag is copied from the read, never changed. The ip and the gateway must
lie in the netmask's subnet. A write asks first (-yes skips that); a
static write applies at once and is followed by a restart (R) as the
original software does, a DHCP write makes the Cube restart by itself
and take a lease. The command then finds the Cube again and re-reads,
allowing for the restart; -dry-run prints the C frame as hex and sends
nothing.

A wrong configuration strands the Cube on an address nothing can reach.
The way back is -sn <serial> -broadcast -dhcp, which sends the requests
as broadcasts that carry the serial, so they reach that one Cube whatever
its address. It first asks for the stored configuration (c) that way;
when the Cube answers, only the DHCP flag changes and the stored
addresses and the encryption flag are kept. When no answer comes, it
sends zeros for the addresses and the encryption flag off unless
-encryption is given: the stored fallback addresses are then lost, and
the factory values are not restored. A Cube that moved networks may need
a power cycle. -broadcast goes to
255.255.255.255 unless -addr names another target, such as a directed
broadcast.

`

// runNetconfig reads or writes a Cube's network configuration.
func runNetconfig(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return netconfig(ctx, args, os.Stdout, os.Stderr, os.Stdin)
}

// netconfig is runNetconfig with its streams given, for tests.
func netconfig(ctx context.Context, args []string, stdout, stderr io.Writer, stdin io.Reader) error {
	fs := flag.NewFlagSet("netconfig", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "", "the Cube's address, host[:port] (port 23272); with -broadcast the target to send to (default 255.255.255.255)")
	asJSON := fs.Bool("json", false, "print JSON instead of text")
	static := fs.String("static", "", "write this static IPv4 address, DHCP off (needs -gateway and -netmask)")
	gateway := fs.String("gateway", "", "the gateway for -static")
	netmask := fs.String("netmask", "", "the netmask for -static, such as 255.255.255.0")
	dns := fs.String("dns", "", "one or two DNS servers for -static, comma separated (default: as read)")
	dhcp := fs.Bool("dhcp", false, "write DHCP on, keeping the addresses as read")
	broadcast := fs.Bool("broadcast", false, "send the write as a broadcast carrying the serial (-sn), without reading first: the recovery for a Cube on a wrong address")
	encryption := fs.Bool("encryption", false, "with -broadcast and no answer to the read, send the encryption flag on (otherwise off); a flag that was read is always copied")
	dryRun := fs.Bool("dry-run", false, "print the C frame as hex and send nothing")
	yes := fs.Bool("yes", false, "write without asking")
	snFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(stderr, netconfigHelp)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("netconfig: unexpected argument %q", fs.Arg(0))
	}
	write := *static != "" || *dhcp
	switch {
	case *static != "" && *dhcp:
		return errors.New("netconfig: -static and -dhcp exclude each other")
	case *static == "" && (*gateway != "" || *netmask != "" || *dns != ""):
		return errors.New("netconfig: -gateway, -netmask and -dns go with -static")
	case *encryption && !*broadcast:
		return errors.New("netconfig: -encryption only with -broadcast; otherwise the flag is copied from the Cube")
	case *broadcast && !write:
		return errors.New("netconfig: -broadcast only writes; give -dhcp or -static")
	case *broadcast && selectedSN == "":
		return errors.New("netconfig: -broadcast needs the Cube's serial (-sn)")
	case !*broadcast && *addr == "":
		return errors.New("netconfig: -addr is required (massimo discover lists the Cubes)")
	}

	// The static flags are checked before anything is sent.
	if *static != "" {
		if _, err := staticConfig(cubelan.NetConfig{}, *static, *gateway, *netmask, *dns); err != nil {
			return err
		}
	}
	info := stdout // with -json, stdout carries only the result
	if *asJSON {
		info = stderr
	}

	// The current configuration: read, at the address or by broadcast.
	var serial, dest string
	var cur *cubelan.NetConfig
	var inUse *cubelan.NetAddrs
	lost := false // no read: zeros replace the stored fallback
	if *broadcast {
		serial, dest = strings.ToUpper(selectedSN), *addr
		if dest == "" {
			dest = net.IPv4bcast.String()
		}
		if st, err := cubelan.FindNetConfig(ctx, dest, serial); err == nil {
			cur, inUse = &st.Stored, st.Current
			fmt.Fprintf(info, "found    cube %s answered the broadcast read from %s; its stored addresses and encryption flag are kept\n", serial, st.From.IP)
		} else if errors.Is(err, cubelan.ErrNoReply) {
			lost = true
			fmt.Fprintf(info, "found    no answer to the broadcast read: zeros will be sent for the addresses, so the stored fallback addresses are lost and the factory values are not restored\n")
		} else {
			return fmt.Errorf("netconfig: %w", err)
		}
	} else {
		st, err := cubelan.QueryNetConfig(ctx, *addr)
		if err != nil {
			return fmt.Errorf("netconfig: %w (a Cube stranded on a wrong address: netconfig -sn <serial> -broadcast -dhcp)", err)
		}
		if snExplicit && !strings.EqualFold(selectedSN, st.Serial) {
			return fmt.Errorf("netconfig: -sn %s given, but the cube at %s is %s", selectedSN, *addr, st.Serial)
		}
		serial, dest, cur, inUse = st.Serial, *addr, &st.Stored, st.Current
		if !write {
			return printNetState(stdout, st, *asJSON)
		}
	}

	// The new configuration.
	var next cubelan.NetConfig
	if cur != nil {
		next = *cur
	} else {
		zero := net.IPv4(0, 0, 0, 0).To4()
		next = cubelan.NetConfig{IP: zero, Gateway: zero, Netmask: zero, Encryption: *encryption}
	}
	if *dhcp {
		next.DHCP = true
	} else {
		var err error
		if next, err = staticConfig(next, *static, *gateway, *netmask, *dns); err != nil {
			return err
		}
	}
	frame, err := cubelan.NetConfigFrame(serial, next)
	if err != nil {
		return fmt.Errorf("netconfig: %w", err)
	}

	fmt.Fprintf(info, "cube     %s at %s\n", serial, dest)
	if cur != nil {
		fmt.Fprintf(info, "stored   %s\n", summary(*cur))
		fmt.Fprintf(info, "in use   %s\n", addrSummary(inUse))
	} else {
		fmt.Fprintln(info, "stored   unknown (no answer to the read)")
	}
	fmt.Fprintf(info, "new      %s\n", summary(next))
	if *dryRun {
		fmt.Fprintf(stdout, "frame    % x\n", frame)
		fmt.Fprintln(stdout, "dry run: nothing sent")
		return nil
	}
	fmt.Fprintf(stderr, "warning: a wrong configuration strands the Cube on an address nothing can reach. "+
		"The way back is massimo netconfig -sn %s -broadcast -dhcp and, after that, the stored fallback addresses (192.168.0.222/16 on a Cube from the factory) and a power cycle.\n", serial)
	if lost {
		fmt.Fprintln(stderr, "warning: nothing was read, so this write sends zeros for the addresses: the Cube's stored fallback addresses are lost and the factory values are not restored.")
	}
	if !*yes {
		fmt.Fprintf(stderr, "write the new configuration to cube %s? [y/N] ", serial)
		line, _ := bufio.NewReader(stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
			return errors.New("netconfig: cancelled, nothing sent")
		}
	}

	res, err := cubelan.SetNetConfig(ctx, dest, serial, next)
	fmt.Fprintf(info, "write    %s\n", statusText(res.Status))
	if res.RebootSent {
		fmt.Fprintf(info, "restart  sent R: %s\n", statusText(res.RebootStatus))
	}
	if err != nil {
		if errors.Is(err, cubelan.ErrNoReply) {
			fmt.Fprintf(stderr, "the write was sent, but the cube did not answer the read after it. It may be applying the change or "+
				"may have moved: wait a minute and try massimo discover, or massimo netconfig -addr <new ip>; a power cycle can help. "+
				"If it stays away: massimo netconfig -sn %s -broadcast -dhcp.\n", serial)
		}
		return fmt.Errorf("netconfig: %w", err)
	}
	fmt.Fprintf(info, "the cube now answers from %s and reports:\n", res.Now.From.IP)
	if err := printNetState(stdout, res.Now, *asJSON); err != nil {
		return err
	}
	if !sameNetConfig(res.Now.Stored, next) {
		fmt.Fprintln(stderr, "warning: the cube reports a different stored configuration than the one written")
	}
	return nil
}

// staticConfig returns base with the static addresses of the flags,
// DHCP off, after checking that the ip and the gateway lie in the
// netmask's subnet. Without dns, base's DNS servers are kept.
func staticConfig(base cubelan.NetConfig, ip, gateway, netmask, dns string) (cubelan.NetConfig, error) {
	parse := func(what, s string) (net.IP, error) {
		v := net.ParseIP(strings.TrimSpace(s)).To4()
		if v == nil {
			return nil, fmt.Errorf("netconfig: %s %q is not an IPv4 address", what, s)
		}
		return v, nil
	}
	var err error
	cfg := base
	cfg.DHCP = false
	if cfg.IP, err = parse("-static", ip); err != nil {
		return cfg, err
	}
	if gateway == "" || netmask == "" {
		return cfg, errors.New("netconfig: -static needs -gateway and -netmask")
	}
	if cfg.Gateway, err = parse("-gateway", gateway); err != nil {
		return cfg, err
	}
	if cfg.Netmask, err = parse("-netmask", netmask); err != nil {
		return cfg, err
	}
	if dns != "" {
		cfg.DNS = nil
		for _, d := range strings.Split(dns, ",") {
			v, err := parse("-dns", d)
			if err != nil {
				return cfg, err
			}
			cfg.DNS = append(cfg.DNS, v)
		}
		if len(cfg.DNS) > 2 {
			return cfg, errors.New("netconfig: -dns takes at most two servers")
		}
	}
	return cfg, checkSubnet(cfg.IP, cfg.Gateway, cfg.Netmask)
}

// checkSubnet checks that mask is a contiguous netmask, that ip and gw
// lie in its subnet, and that ip is a host address other than gw.
func checkSubnet(ip, gw, mask net.IP) error {
	m := net.IPMask(mask.To4())
	ones, bits := m.Size()
	if bits == 0 || ones == 0 {
		return fmt.Errorf("netconfig: -netmask %s is not a netmask", mask)
	}
	network := ip.Mask(m)
	if !gw.Mask(m).Equal(network) {
		return fmt.Errorf("netconfig: the gateway %s is outside the subnet %s/%d of %s", gw, network, ones, ip)
	}
	if ip.Equal(gw) {
		return fmt.Errorf("netconfig: the ip and the gateway are both %s", ip)
	}
	if ones < 31 {
		bcast := make(net.IP, 4)
		for i := range bcast {
			bcast[i] = network[i] | ^m[i]
		}
		if ip.Equal(network) || ip.Equal(bcast) {
			return fmt.Errorf("netconfig: %s is the network or broadcast address of %s/%d", ip, network, ones)
		}
	}
	return nil
}

// summary is a one-line form of cfg.
func summary(c cubelan.NetConfig) string {
	mode := "static"
	if c.DHCP {
		mode = "dhcp"
	}
	return fmt.Sprintf("%s ip %s gw %s mask %s dns %s encryption %s", mode, c.IP, c.Gateway, c.Netmask, dnsList(c.DNS), onOff(c.Encryption))
}

func dnsList(d []net.IP) string {
	if len(d) == 0 {
		return "-"
	}
	s := make([]string, len(d))
	for i, ip := range d {
		s[i] = ip.String()
	}
	return strings.Join(s, ",")
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// sameNetConfig compares two configurations by value.
func sameNetConfig(a, b cubelan.NetConfig) bool { return summary(a) == summary(b) }

// statusText explains a status byte of the Cube's answer to C or R.
func statusText(b byte) string {
	switch b {
	case 0:
		return "no answer"
	case cubelan.SetStatusOK:
		return "status 1, applied"
	case cubelan.SetStatusRestarting:
		return "status 2, the cube restarts"
	}
	return fmt.Sprintf("status %d (unknown)", b)
}

// addrSummary is a one-line form of the addresses in use.
func addrSummary(a *cubelan.NetAddrs) string {
	if a == nil {
		return "unknown (no answer to N)"
	}
	return fmt.Sprintf("ip %s gw %s mask %s dns %s", a.IP, a.Gateway, a.Netmask, dnsList(a.DNS))
}

// printNetState prints what a read learned, as text or JSON: the
// addresses in use (N) apart from the stored static configuration (c).
func printNetState(w io.Writer, st cubelan.NetState, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(st)
	}
	c := st.Stored
	mode, what := "static", "in use"
	if c.DHCP {
		mode, what = "dhcp", "the fallback, not in use while DHCP is on"
	}
	fmt.Fprintf(w, "serial      %s\nmode        %s\nencryption  %s\n", st.Serial, mode, onOff(c.Encryption))
	fmt.Fprintf(w, "in use      %s\n", addrSummary(st.Current))
	fmt.Fprintf(w, "stored      ip %s gw %s mask %s dns %s (static configuration, %s)\n", c.IP, c.Gateway, c.Netmask, dnsList(c.DNS), what)
	return nil
}
