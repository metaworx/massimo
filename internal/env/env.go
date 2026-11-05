// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package env reads the command line's configuration from the process
// environment and the .env file: the per-Cube variables, the hidden set
// and the address a command connects to. Every command, serve included,
// goes through it, so that a settings database can later replace it in
// one place.
//
// Each Cube variable has a per-Cube form with the serial number inserted
// (MASSIMO_CUBE_KEQ0123456_KEY for MASSIMO_CUBE_KEY), which wins for that
// Cube.
package env

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// Variable names.
const (
	Prefix  = "MASSIMO_CUBE_"
	Key     = Prefix + "KEY"
	IV      = Prefix + "IV"
	Address = Prefix + "ADDRESS"
	SN      = Prefix + "SN"
	// Ignore lists the hidden devices of a Cube; the name predates the
	// term "hidden" and is kept for compatibility.
	Ignore = Prefix + "IGNORE"
	// Proxy is the address of a running `massimo serve` that answers as
	// the Cube; commands try it before the Cube itself, and serve
	// listens on it. ProxySerial is the serial serve uses in UDP
	// discovery replies for the Cube.
	Proxy       = Prefix + "PROXY"
	ProxySerial = Prefix + "PROXY_SERIAL"
	Name        = Prefix + "NAME" // a user-friendly name for the Cube
	// FileVar names the env file to read instead of ./.env.
	FileVar = "MASSIMO_ENV_FILE"
)

// perCubeSuffixes are the variables that have a per-Cube form.
var perCubeSuffixes = []string{"_KEY", "_IV", "_ADDRESS", "_IGNORE", "_PROXY", "_PROXY_SERIAL", "_NAME"}

// CubePort is the Cube's TCP port, also the proxy's default.
const CubePort = "62910"

// proxyProbe bounds how long ResolveAddr waits for a proxy to answer.
const proxyProbe = 2 * time.Second

// PerCube returns the per-Cube name of a variable: MASSIMO_CUBE_KEY
// becomes MASSIMO_CUBE_<SN>_KEY.
func PerCube(name, sn string) string {
	return Prefix + strings.ToUpper(sn) + "_" + strings.TrimPrefix(name, Prefix)
}

// Get returns the value of name for the Cube sn: the per-Cube variable
// if set, else the plain one. With sn empty it is the plain one.
func Get(name, sn string) string {
	if sn != "" {
		if v := os.Getenv(PerCube(name, sn)); v != "" {
			return v
		}
	}
	return os.Getenv(name)
}

// Own returns the per-Cube value of name for the Cube sn, falling back
// to the plain variable only when single is set (one Cube configured) or
// sn is empty. It is for values two Cubes must not share, such as a
// listen address or a name.
func Own(name, sn string, single bool) string {
	if sn != "" {
		if v := os.Getenv(PerCube(name, sn)); v != "" || !single {
			return v
		}
	}
	return os.Getenv(name)
}

// Serials lists, sorted, the serial numbers that have the per-Cube form
// of name set to a non-empty value, with those values.
func Serials(name string) (serials []string, values map[string]string) {
	suffix := "_" + strings.TrimPrefix(name, Prefix)
	values = map[string]string{}
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		mid, ok := strings.CutPrefix(k, Prefix)
		if !ok || v == "" {
			continue
		}
		// A serial never contains "_".
		if sn, ok := strings.CutSuffix(mid, suffix); ok && sn != "" && !strings.Contains(sn, "_") {
			serials = append(serials, sn)
			values[sn] = v
		}
	}
	sort.Strings(serials)
	return serials, values
}

// Known reports whether a MASSIMO_* name is one the program reads.
func Known(k string) bool {
	switch k {
	case Key, IV, Address, SN, Ignore, Proxy, ProxySerial, Name, FileVar,
		"MASSIMO_API_TOKEN", "MASSIMO_LOG", "MASSIMO_LISTEN", "MASSIMO_UPDATE_REQUEST",
		// massimo update: the release channel, and update.TokenVar.
		"MASSIMO_UPDATE_CHANNEL", "MASSIMO_GITHUB_TOKEN":
		return true
	}
	mid, ok := strings.CutPrefix(k, Prefix)
	if !ok {
		return false
	}
	for _, suffix := range perCubeSuffixes {
		if sn, ok := strings.CutSuffix(mid, suffix); ok && sn != "" && !strings.Contains(sn, "_") {
			return true
		}
	}
	return false
}

// ParseList reads a comma, semicolon or space separated list of radio
// addresses; name is the variable, for the error message.
func ParseList(name, value string) ([]cube.RFAddress, error) {
	var out []cube.RFAddress
	for _, f := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == ';' }) {
		rf, err := cube.ParseRFAddress(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, rf)
	}
	return out, nil
}

// HiddenList returns the devices listed as hidden for the Cube sn in
// MASSIMO_CUBE_<SN>_IGNORE, else MASSIMO_CUBE_IGNORE.
func HiddenList(sn string) ([]cube.RFAddress, error) {
	name := Ignore
	if sn != "" && os.Getenv(PerCube(Ignore, sn)) != "" {
		name = PerCube(Ignore, sn)
	}
	return ParseList(name, os.Getenv(name))
}

// HiddenSet is the set of devices to leave out for the Cube sn: its
// hidden list plus the devices of cube.HiddenRoom in md, or nothing when
// show is set.
func HiddenSet(sn string, md *cube.Metadata, show bool) (view.Hidden, error) {
	if show {
		return nil, nil
	}
	list, err := HiddenList(sn)
	if err != nil {
		return nil, err
	}
	return view.NewHidden(list, md, false), nil
}

// WithPort adds the Cube's port to an address that has none.
func WithPort(addr string) string {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return net.JoinHostPort(addr, CubePort)
	}
	return addr
}

// ResolveAddr finds the address a command connects to for the Cube sn
// (sn may be empty). A proxy (MASSIMO_CUBE_<SN>_PROXY, a running
// `massimo serve`) is tried first, with a short timeout, because the
// Cube itself serves one client at a time and serve usually holds that
// connection; then MASSIMO_CUBE_<SN>_ADDRESS; then, with a serial, UDP
// discovery on the LAN. Progress goes to log; cmd names the command in
// errors.
func ResolveAddr(cmd, sn string, log io.Writer) (string, error) {
	if p := Get(Proxy, sn); p != "" {
		p = WithPort(p)
		if c, err := net.DialTimeout("tcp", p, proxyProbe); err == nil {
			c.Close()
			fmt.Fprintf(log, "using the massimo proxy at %s\n", p)
			return p, nil
		}
		fmt.Fprintf(log, "proxy %s does not answer, using the cube directly\n", p)
	}
	if a := Get(Address, sn); a != "" {
		return a, nil
	}
	if sn == "" {
		return "", fmt.Errorf("%s: -addr is required (or set %s, or -sn)", cmd, Address)
	}
	fmt.Fprintf(log, "looking for cube %s on the LAN ...\n", sn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cubes, err := cubelan.Discover(ctx, 3*time.Second)
	if err != nil {
		return "", err
	}
	for _, c := range cubes {
		if strings.EqualFold(c.Serial, sn) {
			fmt.Fprintf(log, "cube %s found at %s\n", c.Serial, c.IP)
			return c.IP.String(), nil
		}
	}
	return "", fmt.Errorf("%s: cube %s did not answer the discovery broadcast (set %s, or -addr; from WSL 2 broadcasts do not reach the LAN)", cmd, sn, PerCube(Address, sn))
}
