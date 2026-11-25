// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The listen address lives in drop-ins rather than in the units, so that
// installing a newer release, which replaces the units, keeps it. The
// helper's drop-in carries the health URL, whose scheme follows the
// settings file: https:// once the service has TLS.
const (
	ListenDropIn = UnitDir + "/massimo.service.d/listen.conf"
	HealthDropIn = UnitDir + "/massimo-update.service.d/listen.conf"
)

// splitListen checks a listen address of install's -listen (readListen).
func splitListen(addr string) (host, port string, err error) {
	if host, port, err = readListen(addr); err != nil {
		return "", "", fmt.Errorf("-listen %q: %w", addr, err)
	}
	return host, port, nil
}

// readListen reads a listen address: host:port or :port, the port a
// number from 1 to 65535, so not 0 and no service name; its error says
// what is wrong and adds no prefix of its own: the caller names the
// address and the setting.
func readListen(addr string) (host, port string, err error) {
	host, port, err = net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("%v (want host:port or :port)", err)
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return "", "", errors.New("bad port")
	}
	return host, port, nil
}

// listenDropIns returns the two drop-ins for a listen address: the
// address for serve (with the capability to bind a port below 1024,
// which the user _massimo lacks, for it or for the plain helper address
// httpAddr of -tls-http) and the matching /healthz URL for the update
// helper, https:// when the service has TLS.
func listenDropIns(addr string, tls bool, httpAddr string) (serve, health string, err error) {
	host, port, err := splitListen(addr)
	if err != nil {
		return "", "", err
	}
	head := "# Written by `massimo install` (-listen, -tls), kept by later installs. Delete\n# this file (and its pair) and run `systemctl daemon-reload` to go back to\n# http://:8099.\n[Service]\n"
	serve = head + "Environment=MASSIMO_LISTEN=" + addr + "\n"
	if portBelow1024(port) || httpAddr != "" && portBelow1024(hostPort(httpAddr)) {
		serve += capLine
	}
	health = head + "Environment=MASSIMO_HEALTH=" + healthURL(host, port, tls) + "\n"
	return serve, health, nil
}

// capLine is the listen drop-in's line of the capability to bind a port
// below 1024.
const capLine = "AmbientCapabilities=CAP_NET_BIND_SERVICE\n"

// portBelow1024 says whether a port needs the capability.
func portBelow1024(port string) bool {
	n, err := strconv.Atoi(port)
	return err == nil && n < 1024
}

// hostPort is the port of a listen address, "" for a bad one.
func hostPort(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return port
}

// listenFromDropIn reads the address a listen drop-in sets, "" when it
// has none.
func listenFromDropIn(b []byte) string {
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "Environment=MASSIMO_LISTEN="); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// HealthURL is the server's /healthz for a listen address (host:port or
// :port) by the rule of the drop-ins and the macOS plist, https:// when
// the server has TLS; serve uses it on Windows and under brew for the
// helper it starts. An address it cannot read, port 0 or a service name
// among them, has none: its error names serve's setting, "-listen or
// MASSIMO_LISTEN: …", and serve blocks updates from the API with it.
func HealthURL(listen string, tls bool) (string, error) {
	host, port, err := readListen(listen)
	if err != nil {
		return "", fmt.Errorf("-listen or MASSIMO_LISTEN: the update helper cannot ask %q for /healthz: %w", listen, err)
	}
	return healthURL(host, port, tls), nil
}

// healthURL is the server's /healthz for a listen address, asked from
// the box itself: an address on every interface is asked on the
// loopback, a specific one on itself. The helper does not verify the
// certificate of a loopback host (cmd/massimo), so a self-signed one
// does there.
func healthURL(host, port string, tls bool) string {
	switch ip := net.ParseIP(host); {
	case host == "" || ip != nil && ip.IsUnspecified() && ip.To4() != nil:
		host = "127.0.0.1"
	case ip != nil && ip.IsUnspecified():
		host = "::1"
	}
	scheme := "http"
	if tls {
		scheme = "https"
	}
	return scheme + "://" + net.JoinHostPort(host, port) + "/healthz"
}

// listen writes the drop-ins: the address for Options.Listen, which
// later installs keep, and the helper's health URL, which follows the
// address and the scheme (tls: the settings say the server has TLS;
// httpAddr: the plain helper of -tls-http, whose port below 1024 needs
// the capability too, which the listen drop-in keeps where the settings
// cannot be read, capKept). The health drop-in is written whenever the
// address or the scheme is not the unit's own, and rewritten when a
// kept one would be wrong.
func (in *installer) listen(tls bool, httpAddr string) error {
	listen, kept := in.Listen, false
	if listen == "" {
		if b, err := os.ReadFile(in.path(ListenDropIn)); err == nil {
			listen, kept = listenFromDropIn(b), true
			fmt.Fprintf(in.Out, "%s kept (%d bytes); change it with -listen\n", ListenDropIn, len(b))
		}
	}
	_, healthErr := os.Stat(in.path(HealthDropIn))
	keepCap := in.capKept()
	needCap := httpAddr != "" && portBelow1024(hostPort(httpAddr)) || keepCap
	if listen == "" && !tls && !needCap && healthErr != nil {
		return nil // the units' own values hold
	}
	addr := listen
	if addr == "" {
		addr = DefaultListen
	}
	serve, health, err := listenDropIns(addr, tls, httpAddr)
	if err != nil {
		return err
	}
	if keepCap && !strings.Contains(serve, capLine) {
		serve += capLine
	}
	var files [][3]string // path, content, what
	if in.Listen != "" || kept || needCap {
		files = append(files, [3]string{ListenDropIn, serve, "listen on " + addr})
	}
	if !in.NoUpdateHelper {
		files = append(files, [3]string{HealthDropIn, health, "the helper checks " + strings.TrimSuffix(strings.TrimPrefix(health[strings.LastIndex(health, "MASSIMO_HEALTH=")+len("MASSIMO_HEALTH="):], ""), "\n")})
	}
	for _, f := range files {
		dst := in.path(f[0])
		if old, err := os.ReadFile(dst); err == nil && string(old) == f[1] {
			fmt.Fprintf(in.Out, "%s is current\n", f[0])
			continue
		}
		in.say("write %s (%s)", f[0], f[2])
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
