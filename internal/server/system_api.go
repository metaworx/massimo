// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// GET /api/v1/system: what System settings → System Info shows about the
// server and the computer it runs on, beside what /healthz tells anyone
// (the version, the start, the certificate): how it was installed, the
// operating system, the server's clock and zone, the host's full name, the
// interfaces the server listens on, the Cube proxies' listeners, the
// caller's address as the server sees it, and the proxy in front of it
// (proxied.go).

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/certs"
)

// Install kinds beside a package manager's name: what runs the server
// where no package owns it, as Options.Install names it.
const (
	InstallWindowsService = "windows-service" // the Windows service of massimo install
	InstallSystemd        = "systemd"         // a systemd unit, massimo install's or another
	InstallLaunchd        = "launchd"         // a launchd job, massimo install's or another
	InstallOther          = "other"           // a terminal, a container, a supervisor massimo does not know
)

// systemView is the answer of GET /api/v1/system.
type systemView struct {
	Version   string        `json:"version"`
	StartedAt time.Time     `json:"started_at"`
	TLS       *certs.Status `json:"tls,omitempty"`
	Install   string        `json:"install"`
	OS        string        `json:"os"`
	OSName    string        `json:"os_name,omitempty"`
	Arch      string        `json:"arch"`
	Hostname  string        `json:"hostname,omitempty"`
	ClientIP  string        `json:"client_ip"`
	// Now is the server's clock in its own zone, TimeZone the zone's name
	// (timeZoneName).
	Now      time.Time `json:"now"`
	TimeZone string    `json:"time_zone,omitempty"`
	// FQDN is the host's full name, as hostname -f gives it (fullName).
	FQDN string `json:"fqdn,omitempty"`
	// Interfaces are those the API or a Cube proxy listens on
	// (listenedInterfaces), APIPort the API's port.
	Interfaces []interfaceView `json:"interfaces"`
	APIPort    int             `json:"api_port,omitempty"`
	// CubeProxies are the Cube proxies' listeners, those that do not
	// listen with why (proxyStatuses).
	CubeProxies []proxyStatus `json:"cube_proxies"`
	// Proxy is the proxy the request came through, where a trusted one
	// says so (proxied.go).
	Proxy *proxyView `json:"proxy,omitempty"`
}

// interfaceView is a network interface the server listens on: its name,
// its MAC address, and its addresses, those the API listens on marked.
type interfaceView struct {
	Name      string        `json:"name"`
	MAC       string        `json:"mac,omitempty"`
	Addresses []addressView `json:"addresses"`
}

// addressView is an address of an interface.
type addressView struct {
	IP  string `json:"ip"`
	API bool   `json:"api,omitempty"`
}

// getSystem handles GET /api/v1/system.
func (s *Server) getSystem(w http.ResponseWriter, r *http.Request) error {
	v := systemView{Version: s.opts.Version, StartedAt: s.started, Install: s.opts.Install,
		OS: goruntime.GOOS, OSName: osName(osReleasePath), Arch: goruntime.GOARCH, ClientIP: s.clientIP(r)}
	if v.Install == "" {
		v.Install = InstallOther
	}
	if s.opts.TLS != nil {
		st := s.opts.TLS.Status()
		v.TLS = &st
	}
	if h, err := os.Hostname(); err == nil {
		v.Hostname = h
		v.FQDN = fullName(r.Context(), h)
	}
	v.Now = time.Now()
	v.TimeZone = timeZoneName(os.Getenv("TZ"), localtimePath)
	listeners := s.listeners()
	if ifs, err := hostInterfaces(); err == nil {
		v.Interfaces = listenedInterfaces(ifs, listeners)
	}
	if v.Interfaces == nil {
		v.Interfaces = []interfaceView{}
	}
	v.CubeProxies = s.proxyStatuses()
	for _, l := range listeners {
		if l.api {
			v.APIPort = l.port
		}
	}
	v.Proxy = s.proxied(r)
	writeJSON(w, http.StatusOK, v)
	return nil
}

// SetAPIAddr tells the server the address its API listens on, which the
// program serving it opens after the server is made (internal/serve).
func (s *Server) SetAPIAddr(addr net.Addr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apiAddr = addr
}

// listener is an address the server listens on: its IP, nil for every
// address, and whether it is the API's.
type listener struct {
	ip   net.IP
	port int
	api  bool
}

// listeners are the API's listener and the Cube proxies' listening now.
func (s *Server) listeners() []listener {
	var out []listener
	s.mu.RLock()
	api := s.apiAddr
	s.mu.RUnlock()
	if api != nil {
		if l, ok := parseListener(api.String()); ok {
			l.api = true
			out = append(out, l)
		}
	}
	s.proxyMu.Lock()
	for _, pr := range s.proxies {
		if !pr.listening {
			continue
		}
		if l, ok := parseListener(pr.pl.Addr); ok {
			out = append(out, l)
		}
	}
	s.proxyMu.Unlock()
	return out
}

// parseListener reads a listener's host:port; an empty or unspecified
// host is every address.
func parseListener(addr string) (listener, bool) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return listener{}, false
	}
	p, _ := strconv.Atoi(port)
	l := listener{port: p}
	if host != "" {
		l.ip = net.ParseIP(host)
		if l.ip == nil {
			return listener{}, false // a name: not resolved here
		}
	}
	return l, true
}

// everywhere reports whether a listener is on every address.
func (l listener) everywhere() bool { return l.ip == nil || l.ip.IsUnspecified() }

// covers reports whether a listener takes ip: one on every address takes
// an IPv4 address where it is 0.0.0.0, and any where it is :: or empty,
// as Go listens on both; one on an address takes that address.
func (l listener) covers(ip net.IP) bool {
	switch {
	case l.ip == nil || l.ip.Equal(net.IPv6unspecified):
		return true
	case l.ip.Equal(net.IPv4zero):
		return ip.To4() != nil
	}
	return l.ip.Equal(ip)
}

// hostInterface is a network interface of this host, as net.Interfaces
// gives it.
type hostInterface struct {
	Name     string
	MAC      string
	Up       bool
	Loopback bool
	IPs      []net.IP
}

// hostInterfaces reads this host's interfaces; a variable for tests.
var hostInterfaces = func() ([]hostInterface, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]hostInterface, 0, len(ifs))
	for _, ifc := range ifs {
		h := hostInterface{Name: ifc.Name, MAC: ifc.HardwareAddr.String(), Up: ifc.Flags&net.FlagUp != 0, Loopback: ifc.Flags&net.FlagLoopback != 0}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				h.IPs = append(h.IPs, n.IP)
			}
		}
		out = append(out, h)
	}
	return out, nil
}

// listenedInterfaces are the interfaces a listener takes an address of,
// each with its addresses but link-local ones, those the API's listener
// takes marked. A listener on every address takes no loopback and no
// interface that is down; one on an address takes that address's
// interface, the loopback too.
func listenedInterfaces(ifs []hostInterface, ls []listener) []interfaceView {
	var out []interfaceView
	for _, ifc := range ifs {
		if !ifc.Up {
			continue
		}
		var addrs []addressView
		taken := false
		for _, ip := range ifc.IPs {
			if ip.IsLinkLocalUnicast() {
				continue
			}
			a := addressView{IP: ip.String()}
			for _, l := range ls {
				if ifc.Loopback && l.everywhere() || !l.covers(ip) {
					continue
				}
				taken = true
				a.API = a.API || l.api
			}
			addrs = append(addrs, a)
		}
		if taken {
			out = append(out, interfaceView{Name: ifc.Name, MAC: ifc.MAC, Addresses: addrs})
		}
	}
	return out
}

// lookupCNAME finds a name's canonical name; a variable for tests.
var lookupCNAME = net.DefaultResolver.LookupCNAME

// fullName is the host's full name as hostname -f gives it: its canonical
// name, which Go's resolver takes from /etc/hosts first, then from DNS,
// within a second; the short name where there is none.
func fullName(ctx context.Context, host string) string {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	name, err := lookupCNAME(ctx, host)
	name = strings.TrimSuffix(name, ".")
	if err != nil || name == "" {
		return host
	}
	return name
}

// localtimePath is the link to the host's zone on Linux and macOS.
var localtimePath = "/etc/localtime"

// timeZoneName names the server's zone: TZ where it is set (a zone's name,
// or a path into a zoneinfo directory), else the zone /etc/localtime links
// to, else the zone's abbreviation, as on Windows.
func timeZoneName(tz, localtime string) string {
	if tz = strings.TrimPrefix(tz, ":"); tz != "" {
		return zoneOfPath(tz)
	}
	if p, err := os.Readlink(localtime); err == nil {
		if name := zoneOfPath(filepath.ToSlash(p)); name != p {
			return name
		}
	}
	name, _ := time.Now().Zone()
	return name
}

// zoneOfPath is the zone's name in a path into a zoneinfo directory,
// .../zoneinfo/Europe/Zurich, else the path as it is.
func zoneOfPath(p string) string {
	if _, name, ok := strings.Cut(p, "zoneinfo/"); ok && name != "" {
		return name
	}
	return p
}

// osReleasePath is the file that names a Linux distribution.
var osReleasePath = "/etc/os-release"

// osName is the distribution's PRETTY_NAME from an os-release file, or
// "" where there is none, as on Windows and macOS.
func osName(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "PRETTY_NAME=")
		if !ok {
			continue
		}
		if u, err := strconv.Unquote(v); err == nil {
			return u
		}
		return strings.Trim(v, `"'`)
	}
	return ""
}
