// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
)

// runDiscover lists the Cubes on the local network.
func runDiscover(args []string) error {
	fs := flag.NewFlagSet("discover", flag.ContinueOnError)
	wait := fs.Duration("wait", 3*time.Second, "how long to wait for answers")
	asJSON := fs.Bool("json", false, "print JSON instead of a table")
	addrs := fs.String("addr", "", "comma-separated IPs to ask directly instead of broadcasting")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo discover [-wait 3s] [-json] [-addr ip,ip]\n\nFind Cubes on the local network with a UDP broadcast (port 23272).\nThe broadcast must reach the Cube's network: from WSL 2 in NAT mode it\ndoes not; use the Windows binary, mirrored networking, or -addr.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	var unicast []net.IP
	for _, a := range strings.Split(*addrs, ",") {
		if a = strings.TrimSpace(a); a != "" {
			ip := net.ParseIP(a)
			if ip == nil {
				return fmt.Errorf("discover: bad address %q", a)
			}
			unicast = append(unicast, ip)
		}
	}
	cubes, err := cubelan.Discover(context.Background(), *wait, unicast...)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cubes)
	}
	if len(cubes) == 0 {
		fmt.Println("no cube answered")
		return nil
	}
	fmt.Printf("%-12s %-15s %-18s %-8s %-8s %s\n", "SERIAL", "IP", "MAC", "RF", "FW", "NETWORK")
	for _, c := range cubes {
		fw := c.Firmware
		if c.Bootloader {
			fw += " (bootloader)"
		}
		var netinfo []string
		if c.Gateway != nil {
			netinfo = append(netinfo, "gw "+c.Gateway.String())
		}
		if c.Netmask != nil {
			netinfo = append(netinfo, "mask "+c.Netmask.String())
		}
		for _, d := range c.DNS {
			netinfo = append(netinfo, "dns "+d.String())
		}
		mac := c.MAC
		if mac == "" {
			mac = "-"
		}
		fmt.Printf("%-12s %-15s %-18s %-8s %-8s %s\n", c.Serial, c.IP, mac, c.RFAddress, fw, strings.Join(netinfo, " "))
	}
	return nil
}
