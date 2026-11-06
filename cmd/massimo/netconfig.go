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

Read or write a Cube's network configuration over UDP 23272 (c reads,
C writes), as the original software's network setup does.

Without -static or -dhcp it prints the configuration. -static writes a
static address with DHCP off; without -dns the DNS servers as read are
kept. -dhcp switches DHCP on and keeps the addresses as read. The
encryption flag is copied from the read, never changed. The ip and the
gateway must lie in the netmask's subnet. A write asks first (-yes skips
that) and re-reads the configuration afterwards; -dry-run prints the C
frame as hex and sends nothing.

A wrong configuration strands the Cube on an address nothing can reach.
The way back is -sn <serial> -broadcast -dhcp, which sends the write as a
broadcast that carries the serial, so it reaches that one Cube whatever
its address; with no read possible first, it sends zeros for the
addresses and the encryption flag off unless -encryption is given. If
that fails too, the Cube's factory fallback is 192.168.0.222/16; a Cube
that moved networks may need a power cycle. -broadcast goes to
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
	encryption := fs.Bool("encryption", false, "with -broadcast, send the encryption flag on (otherwise off; without -broadcast it is copied from the read)")
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

	// The current configuration: read, unless the Cube cannot be reached.
	var serial, dest string
	var cur *cubelan.NetConfig
	if *broadcast {
		serial, dest = strings.ToUpper(selectedSN), *addr
		if dest == "" {
			dest = net.IPv4bcast.String()
		}
	} else {
		s, c, err := cubelan.QueryNetConfig(ctx, *addr)
		if err != nil {
			return fmt.Errorf("netconfig: %w (a Cube stranded on a wrong address: netconfig -sn <serial> -broadcast -dhcp)", err)
		}
		if snExplicit && !strings.EqualFold(selectedSN, s) {
			return fmt.Errorf("netconfig: -sn %s given, but the cube at %s is %s", selectedSN, *addr, s)
		}
		serial, dest, cur = s, *addr, &c
		if !write {
			return printNetConfig(stdout, serial, c, *asJSON)
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

	info := stdout // with -json, stdout carries only the result
	if *asJSON {
		info = stderr
	}
	fmt.Fprintf(info, "cube     %s at %s\n", serial, dest)
	if cur != nil {
		fmt.Fprintf(info, "current  %s\n", summary(*cur))
	} else {
		fmt.Fprintln(info, "current  unknown (broadcast, nothing read)")
	}
	fmt.Fprintf(info, "new      %s\n", summary(next))
	if *dryRun {
		fmt.Fprintf(stdout, "frame    % x\n", frame)
		fmt.Fprintln(stdout, "dry run: nothing sent")
		return nil
	}
	fmt.Fprintf(stderr, "warning: a wrong configuration strands the Cube on an address nothing can reach. "+
		"The way back is massimo netconfig -sn %s -broadcast -dhcp and, after that, the factory fallback 192.168.0.222/16 (and a power cycle).\n", serial)
	if !*yes {
		fmt.Fprintf(stderr, "write the new configuration to cube %s? [y/N] ", serial)
		line, _ := bufio.NewReader(stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
			return errors.New("netconfig: cancelled, nothing sent")
		}
	}

	res, err := cubelan.SetNetConfig(ctx, dest, serial, next)
	if res.Answered {
		fmt.Fprintln(stderr, "the cube answered the write")
	}
	if err != nil {
		if errors.Is(err, cubelan.ErrNoReply) {
			fmt.Fprintf(stderr, "the write was sent, but the cube did not answer the read after it. It may be applying the change or "+
				"may have moved: wait a minute and try massimo discover, or massimo netconfig -addr <new ip>; a power cycle can help. "+
				"If it stays away: massimo netconfig -sn %s -broadcast -dhcp, then the factory fallback 192.168.0.222/16.\n", serial)
		}
		return fmt.Errorf("netconfig: %w", err)
	}
	fmt.Fprintln(info, "the cube now reports:")
	if err := printNetConfig(stdout, serial, res.Now, *asJSON); err != nil {
		return err
	}
	if !sameNetConfig(res.Now, next) {
		fmt.Fprintln(stderr, "warning: the cube reports a different configuration than the one written; it may apply it only after a restart")
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

// printNetConfig prints a Cube's configuration as text or JSON.
func printNetConfig(w io.Writer, serial string, c cubelan.NetConfig, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Serial string `json:"serial"`
			cubelan.NetConfig
		}{serial, c})
	}
	mode := "static"
	if c.DHCP {
		mode = "dhcp"
	}
	fmt.Fprintf(w, "serial      %s\nmode        %s\nip          %s\ngateway     %s\nnetmask     %s\ndns         %s\nencryption  %s\n",
		serial, mode, c.IP, c.Gateway, c.Netmask, dnsList(c.DNS), onOff(c.Encryption))
	return nil
}
