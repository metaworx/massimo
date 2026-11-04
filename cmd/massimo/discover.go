// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo discover [-wait 3s] [-json]\n\nFind Cubes on the local network with a UDP broadcast (port 23272).\nThe port must be free, and the broadcast must reach the Cube's network:\nfrom WSL 2 it does not, run the Windows binary there.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	cubes, err := cubelan.Discover(context.Background(), *wait)
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
