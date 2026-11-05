// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

// The listen address lives in drop-ins rather than in the units, so that
// installing a newer release, which replaces the units, keeps it.
const (
	ListenDropIn = UnitDir + "/massimo.service.d/listen.conf"
	HealthDropIn = UnitDir + "/massimo-update.service.d/listen.conf"
)

// listenDropIns returns the two drop-ins for a listen address: the
// address for serve (with the capability to bind a port below 1024,
// which the user massimo lacks) and the matching /healthz URL for the
// update helper.
func listenDropIns(addr string) (serve, health string, err error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("-listen %q: %v (want host:port or :port)", addr, err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", "", fmt.Errorf("-listen %q: bad port", addr)
	}
	head := "# Written by `massimo install -listen`, kept by later installs. Delete this\n# file (and its pair) and run `systemctl daemon-reload` to go back to :8099.\n[Service]\n"
	serve = head + "Environment=MASSIMO_LISTEN=" + addr + "\n"
	if n < 1024 {
		serve += "AmbientCapabilities=CAP_NET_BIND_SERVICE\n"
	}
	health = head + "Environment=MASSIMO_HEALTH=" + healthURL(host, port) + "\n"
	return serve, health, nil
}

// healthURL is the server's /healthz for a listen address, asked from
// the box itself: an address on every interface is asked on the
// loopback, a specific one on itself.
func healthURL(host, port string) string {
	switch ip := net.ParseIP(host); {
	case host == "" || ip != nil && ip.IsUnspecified() && ip.To4() != nil:
		host = "127.0.0.1"
	case ip != nil && ip.IsUnspecified():
		host = "::1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/healthz"
}

// listen writes the drop-ins for Options.Listen, or with none given
// reports the ones a former install left.
func (in *installer) listen() error {
	if in.Listen == "" {
		if b, err := os.ReadFile(in.path(ListenDropIn)); err == nil {
			fmt.Fprintf(in.Out, "%s kept (%d bytes); change it with -listen\n", ListenDropIn, len(b))
		}
		return nil
	}
	serve, health, err := listenDropIns(in.Listen)
	if err != nil {
		return err
	}
	files := [][2]string{{ListenDropIn, serve}}
	if !in.NoUpdateHelper {
		files = append(files, [2]string{HealthDropIn, health})
	}
	for _, f := range files {
		dst := in.path(f[0])
		if old, err := os.ReadFile(dst); err == nil && string(old) == f[1] {
			fmt.Fprintf(in.Out, "%s is current\n", f[0])
			continue
		}
		in.say("write %s (listen on %s)", f[0], in.Listen)
		if in.DryRun {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := writeAtomic(dst, []byte(f[1]), 0o644); err != nil {
			return err
		}
	}
	return nil
}
