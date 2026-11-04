// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"go.metaworx.ch/massimo/internal/cubelan"
)

// firewallHints explains why discovery may have got no answer. It only
// reads firewall state, never changes it.
func firewallHints() []string {
	port := fmt.Sprintf("%d/udp", cubelan.DiscoveryPort)
	var hints []string
	switch runtime.GOOS {
	case "linux":
		if out, err := exec.Command("ufw", "status").Output(); err == nil {
			s := string(out)
			switch {
			case strings.Contains(s, "Status: active") && !strings.Contains(s, port):
				hints = append(hints, "ufw is active and has no rule for "+port+"; the Cube answers to that port. Allow it with: sudo ufw allow "+port)
			case strings.Contains(s, "Status: active"):
				hints = append(hints, "ufw is active with a rule for "+port)
			}
		} else if _, err := exec.LookPath("ufw"); err == nil {
			hints = append(hints, "could not read the ufw status (run 'sudo ufw status'); a firewall must allow incoming "+port)
		}
		if _, err := exec.LookPath("nft"); err == nil && len(hints) == 0 {
			hints = append(hints, "if a firewall is active, it must allow incoming "+port)
		}
		hints = append(hints, "from WSL 2 or another NAT, broadcasts do not reach the LAN; run this on a machine on the Cube's network")
	case "windows":
		hints = append(hints, "Windows Defender Firewall must allow this program to receive UDP on port "+fmt.Sprint(cubelan.DiscoveryPort)+" (private network); the original MAX! software must not be running")
	default:
		hints = append(hints, "a firewall must allow incoming "+port)
	}
	return hints
}
