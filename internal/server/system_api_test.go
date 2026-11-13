// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
	"time"
)

// GET /api/v1/system: the version and start /healthz has, how the server
// was installed, the computer, and the caller's address, the client a
// trusted proxy names.
func TestGetSystem(t *testing.T) {
	proxies, _ := ParseTrustedProxies("127.0.0.1,::1")
	_, ts, _ := startWith(t, Options{Token: "secret", Version: "v0.0.0-test (go1.26.0)", Install: "apt", TrustedProxies: proxies})
	get := func(forwarded string) systemView {
		t.Helper()
		req, _ := http.NewRequest("GET", ts.URL+"/api/v1/system", nil)
		req.Header.Set("Authorization", "Bearer secret")
		if forwarded != "" {
			req.Header.Set("X-Forwarded-For", forwarded)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status %d", resp.StatusCode)
		}
		var v systemView
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	v := get("")
	host, _ := os.Hostname()
	if v.Version != "v0.0.0-test (go1.26.0)" || v.StartedAt.IsZero() || v.Install != "apt" || v.OS != goruntime.GOOS ||
		v.Arch != goruntime.GOARCH || v.Hostname != host || v.ClientIP != "127.0.0.1" || v.TLS != nil {
		t.Fatalf("system %+v", v)
	}
	if v := get("192.0.2.10"); v.ClientIP != "192.0.2.10" {
		t.Fatalf("behind a trusted proxy: client %q", v.ClientIP)
	}
	resp, err := http.Get(ts.URL + "/api/v1/system")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("without a token: %d", resp.StatusCode)
	}
}

// A server that does not say how it was installed is "other".
func TestGetSystemInstallOther(t *testing.T) {
	_, ts, _ := startWith(t, Options{Token: "secret"})
	var v systemView
	if code := getJSON(t, ts, "secret", "/api/v1/system", &v); code != 200 || v.Install != InstallOther {
		t.Fatalf("%d %+v", code, v)
	}
}

// The distribution's name comes from os-release's PRETTY_NAME, quoted
// or not; none without the file.
func TestOSName(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct{ body, want string }{
		{"NAME=\"Debian GNU/Linux\"\nPRETTY_NAME=\"Debian GNU/Linux 13 (trixie)\"\nID=debian\n", "Debian GNU/Linux 13 (trixie)"},
		{"PRETTY_NAME=Alpine\n", "Alpine"},
		{"ID=nothing\n", ""},
	} {
		p := filepath.Join(dir, "os-release")
		if err := os.WriteFile(p, []byte(c.body), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := osName(p); got != c.want {
			t.Errorf("%q: %q, want %q", c.body, got, c.want)
		}
	}
	if got := osName(filepath.Join(dir, "none")); got != "" {
		t.Errorf("without the file: %q", got)
	}
}

// The interfaces a listener takes, with the API's addresses marked: one on
// every address takes each interface that is up but the loopback; one on
// an address, that address's interface; link-local addresses are left out.
func TestListenedInterfaces(t *testing.T) {
	ifs := []hostInterface{
		{Name: "lo", Up: true, Loopback: true, IPs: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}},
		{Name: "eth0", MAC: "02:00:00:00:00:01", Up: true, IPs: []net.IP{net.ParseIP("192.168.1.8"), net.ParseIP("192.168.1.31"), net.ParseIP("2001:db8::8"), net.ParseIP("fe80::1")}},
		{Name: "eth1", MAC: "02:00:00:00:00:02", Up: false, IPs: []net.IP{net.ParseIP("10.0.0.2")}},
		{Name: "wlan0", MAC: "02:00:00:00:00:03", Up: true},
	}
	listen := func(addrs ...string) []listener {
		var out []listener
		for i, a := range addrs {
			l, ok := parseListener(a)
			if !ok {
				t.Fatalf("listener %q", a)
			}
			l.api = i == 0
			out = append(out, l)
		}
		return out
	}
	show := func(v []interfaceView) string {
		s := ""
		for _, ifc := range v {
			s += ifc.Name + " " + ifc.MAC + ":"
			for _, a := range ifc.Addresses {
				s += " " + a.IP
				if a.API {
					s += "*"
				}
			}
			s += "; "
		}
		return s
	}
	for _, c := range []struct {
		name      string
		listeners []listener
		want      string
	}{
		{"the API everywhere", listen(":8099"), "eth0 02:00:00:00:00:01: 192.168.1.8* 192.168.1.31* 2001:db8::8*; "},
		{"the API on an address, a proxy on another", listen("192.168.1.8:8099", "192.168.1.31:62910"), "eth0 02:00:00:00:00:01: 192.168.1.8* 192.168.1.31 2001:db8::8; "},
		{"the API on IPv4's every address", listen("0.0.0.0:8099"), "eth0 02:00:00:00:00:01: 192.168.1.8* 192.168.1.31* 2001:db8::8; "},
		{"the API on the loopback, a proxy everywhere", listen("127.0.0.1:8099", ":62910"), "lo : 127.0.0.1* ::1; eth0 02:00:00:00:00:01: 192.168.1.8 192.168.1.31 2001:db8::8; "},
		{"no listener", nil, ""},
	} {
		if got := show(listenedInterfaces(ifs, c.listeners)); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}

// The API's listener and the proxies' running now are the server's
// listeners; GET /api/v1/system lists their interfaces and the API's port.
func TestGetSystemInterfaces(t *testing.T) {
	saved := hostInterfaces
	defer func() { hostInterfaces = saved }()
	hostInterfaces = func() ([]hostInterface, error) {
		return []hostInterface{{Name: "eth0", MAC: "02:00:00:00:00:01", Up: true, IPs: []net.IP{net.ParseIP("192.168.1.8")}}}, nil
	}
	_, ts, s := startWith(t, Options{Token: "secret"})
	var v systemView
	if code := getJSON(t, ts, "secret", "/api/v1/system", &v); code != 200 || len(v.Interfaces) != 0 || v.APIPort != 0 {
		t.Fatalf("before the API's address is known: %d %+v", code, v.Interfaces)
	}
	s.SetAPIAddr(&net.TCPAddr{Port: 8099})
	if code := getJSON(t, ts, "secret", "/api/v1/system", &v); code != 200 || v.APIPort != 8099 || len(v.Interfaces) != 1 ||
		v.Interfaces[0].MAC != "02:00:00:00:00:01" || !v.Interfaces[0].Addresses[0].API {
		t.Fatalf("%d %d %+v", code, v.APIPort, v.Interfaces)
	}
	if d := time.Since(v.Now); d < -time.Minute || d > time.Minute || v.TimeZone == "" {
		t.Fatalf("the clock %v in %q", v.Now, v.TimeZone)
	}
}

// The zone's name: TZ, a name or a path into zoneinfo, else what
// /etc/localtime links to, else the abbreviation.
func TestTimeZoneName(t *testing.T) {
	if got := timeZoneName(":Europe/Zurich", ""); got != "Europe/Zurich" {
		t.Errorf("TZ with a colon: %q", got)
	}
	if got := timeZoneName("/usr/share/zoneinfo/America/New_York", ""); got != "America/New_York" {
		t.Errorf("TZ as a path: %q", got)
	}
	link := filepath.Join(t.TempDir(), "localtime")
	if err := os.Symlink("/usr/share/zoneinfo/Europe/Berlin", link); err != nil {
		t.Skip(err)
	}
	if got := timeZoneName("", link); got != "Europe/Berlin" {
		t.Errorf("the link: %q", got)
	}
	if got := timeZoneName("", filepath.Join(t.TempDir(), "none")); got == "" {
		t.Error("no name at all without a link")
	}
}

// The full name is the canonical one, without its final dot; the short
// one where the lookup fails.
func TestFullName(t *testing.T) {
	saved := lookupCNAME
	defer func() { lookupCNAME = saved }()
	lookupCNAME = func(_ context.Context, host string) (string, error) { return host + ".example.net.", nil }
	if got := fullName(context.Background(), "pi"); got != "pi.example.net" {
		t.Errorf("%q", got)
	}
	lookupCNAME = func(context.Context, string) (string, error) { return "", errors.New("no such host") }
	if got := fullName(context.Background(), "pi"); got != "pi" {
		t.Errorf("without a canonical name: %q", got)
	}
}
