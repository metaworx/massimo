// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// GET /api/v1/system: what System settings → System Info shows about the
// server and the computer it runs on, beside what /healthz tells anyone
// (the version, the start, the certificate): how it was installed, the
// operating system, the host's name and the caller's address as the
// server sees it.

import (
	"bufio"
	"net/http"
	"os"
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
	}
	writeJSON(w, http.StatusOK, v)
	return nil
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
