// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// lookupMAC returns the MAC address the operating system's ARP cache has
// for ip, or "". The Cube has just answered a packet, so the entry is
// usually present. Best effort; "" on any problem.
func lookupMAC(ip net.IP) string {
	switch runtime.GOOS {
	case "linux":
		f, err := os.Open("/proc/net/arp")
		if err != nil {
			return ""
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) >= 4 && fields[0] == ip.String() && fields[3] != "00:00:00:00:00:00" {
				return fields[3]
			}
		}
	case "windows", "darwin":
		out, err := exec.Command("arp", "-a").Output()
		if err != nil {
			return ""
		}
		want := ip.String()
		for _, line := range strings.Split(string(out), "\n") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if strings.Trim(f, "()") == want && i+1 < len(fields) {
					mac := strings.ReplaceAll(fields[i+1], "-", ":")
					if _, err := net.ParseMAC(mac); err == nil {
						return mac
					}
				}
			}
		}
	}
	return ""
}
