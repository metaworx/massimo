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
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/announce"
	"go.metaworx.ch/massimo/internal/cubelan"
)

// runDiscover lists the Cubes on the local network, and the massimo
// servers that announce themselves there.
func runDiscover(args []string) error {
	fs := flag.NewFlagSet("discover", flag.ContinueOnError)
	wait := fs.Duration("wait", 3*time.Second, "how long to wait for answers")
	asJSON := fs.Bool("json", false, "print JSON instead of the tables: an object with the cubes and the servers")
	addrs := fs.String("addr", "", "comma-separated IPs to ask directly instead of broadcasting")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo discover [-wait 3s] [-json] [-addr ip,ip]\n\nFind Cubes on the local network with a UDP broadcast (port 23272), and\nthe massimo servers that announce themselves by mDNS (_massimo._tcp).\nA Cube that is a massimo serve's proxy is marked proxy: it answers neither\nN nor c, and its MAC is not eQ-3's or its address is an announced server's.\nThe broadcast must reach the Cube's network: from WSL 2 in NAT mode it does\nnot; use the Windows binary, mirrored networking, or -addr.\n\n")
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
	ctx := context.Background()
	var (
		cubes      []cubelan.Discovered
		servers    []announce.Found
		cerr, serr error
		wg         sync.WaitGroup
	)
	wg.Go(func() { cubes, cerr = cubelan.Discover(ctx, *wait, unicast...) })
	wg.Go(func() { servers, serr = announce.Browse(ctx, *wait, unicast...) })
	wg.Wait()
	if cerr != nil {
		return cerr
	}
	if serr != nil {
		fmt.Fprintln(os.Stderr, "warning: servers not looked for:", serr)
	}
	found := markProxies(cubes, servers)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(discovery{Cubes: found, Servers: nonNilFound(servers)})
	}
	printCubes(found)
	fmt.Println()
	printServers(servers)
	return nil
}

// foundCube is a Cube that answered, and whether it is a massimo proxy.
type foundCube struct {
	cubelan.Discovered
	Proxy bool `json:"proxy"`
}

// discovery is what discover -json prints.
type discovery struct {
	Cubes   []foundCube      `json:"cubes"`
	Servers []announce.Found `json:"servers"`
}

func nonNilFound(s []announce.Found) []announce.Found {
	if s == nil {
		return []announce.Found{}
	}
	return s
}

// markProxies marks the Cubes that are a massimo serve's proxy: those
// the search took for one (cubelan.KindProxy: no answer to N or c, a MAC
// that is not eQ-3's), and those it could not tell whose address is an
// announced server's. An answer the search took for a Cube's stays one.
func markProxies(cubes []cubelan.Discovered, servers []announce.Found) []foundCube {
	out := []foundCube{}
	for _, c := range cubes {
		proxy := c.Kind == cubelan.KindProxy
		for _, s := range servers {
			proxy = proxy || (c.Kind != cubelan.KindCube && s.Has(c.IP))
		}
		out = append(out, foundCube{Discovered: c, Proxy: proxy})
	}
	return out
}

// printCubes prints the Cube table, or why it is empty.
func printCubes(cubes []foundCube) {
	if len(cubes) == 0 {
		fmt.Println("no cube answered")
		for _, hint := range firewallHints() {
			fmt.Println("hint:", hint)
		}
		return
	}
	fmt.Printf("%-12s %-15s %-18s %-8s %-8s %s\n", "SERIAL", "IP", "MAC", "RF", "FW", "NETWORK")
	for _, c := range cubes {
		fw := c.Firmware
		if c.Bootloader {
			fw += " (bootloader)"
		}
		var netinfo []string
		if c.Proxy {
			netinfo = append(netinfo, "proxy")
		}
		if c.DHCP != nil {
			netinfo = append(netinfo, map[bool]string{true: "dhcp", false: "static"}[*c.DHCP])
		}
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
}

// printServers prints the table of the announced servers, or that none
// announced itself.
func printServers(servers []announce.Found) {
	if len(servers) == 0 {
		fmt.Println("no massimo server announced itself")
		return
	}
	fmt.Printf("%-20s %-22s %-4s %-24s %s\n", "NAME", "ADDRESS", "TLS", "VERSION", "NODE")
	for _, s := range servers {
		tls := "no"
		if s.TLS {
			tls = "yes"
		}
		version := s.Version
		if version == "" {
			version = "-"
		}
		fmt.Printf("%-20s %-22s %-4s %-24s %s\n", s.Name, s.Address(), tls, version, s.Node)
	}
}
