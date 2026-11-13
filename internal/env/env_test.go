// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package env

import (
	"io"
	"net"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

func TestGetAndOwn(t *testing.T) {
	t.Setenv("MASSIMO_CUBE_KEY", "default")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_KEY", "one")
	t.Setenv("MASSIMO_CUBE_NAME", "plain")
	if v := Get(Key, "keq0000001"); v != "one" {
		t.Fatalf("per-cube key %q", v)
	}
	if v := Get(Key, "KEQ0000002"); v != "default" {
		t.Fatalf("fallback key %q", v)
	}
	if v := Get(Key, ""); v != "default" {
		t.Fatalf("plain key %q", v)
	}
	if v := Own(Name, "KEQ0000001", true); v != "plain" {
		t.Fatalf("single cube name %q", v)
	}
	if v := Own(Name, "KEQ0000001", false); v != "" {
		t.Fatalf("several cubes share the plain name: %q", v)
	}
	if v := Own(Name, "", false); v != "plain" {
		t.Fatalf("no serial %q", v)
	}
	if PerCube(ProxySerial, "keq1") != "MASSIMO_CUBE_KEQ1_PROXY_SERIAL" {
		t.Fatal(PerCube(ProxySerial, "keq1"))
	}
}

func TestSerials(t *testing.T) {
	t.Setenv("MASSIMO_CUBE_KEQ0000002_ADDRESS", "10.0.0.2")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	t.Setenv("MASSIMO_CUBE_KEQ0000003_ADDRESS", "")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_PROXY_SERIAL", "x")
	sns, addrs := Serials(Address)
	if len(sns) != 2 || sns[0] != "KEQ0000001" || addrs["KEQ0000002"] != "10.0.0.2" {
		t.Fatalf("serials %v %v", sns, addrs)
	}
	if sns, _ := Serials(Proxy); len(sns) != 0 {
		t.Fatalf("_PROXY_SERIAL read as _PROXY: %v", sns)
	}
}

func TestHiddenList(t *testing.T) {
	t.Setenv("MASSIMO_CUBE_IGNORE", "0a0101, 0A0201")
	t.Setenv("MASSIMO_CUBE_KEQ0000002_IGNORE", "0a0202")
	got, err := HiddenList("")
	if err != nil || len(got) != 2 || got[0] != 0x0a0101 || got[1] != 0x0a0201 {
		t.Fatalf("plain list %v err %v", got, err)
	}
	got, err = HiddenList("keq0000002")
	if err != nil || len(got) != 1 || got[0] != 0x0a0202 {
		t.Fatalf("per-cube list %v err %v", got, err)
	}
	if _, err := ParseList("MASSIMO_CUBE_IGNORE", "zz"); err == nil || !strings.Contains(err.Error(), "MASSIMO_CUBE_IGNORE") {
		t.Fatalf("bad entry accepted: %v", err)
	}
	md := &cube.Metadata{Devices: []cube.DeviceMeta{{RFAddress: 0x000002, RoomID: cube.HiddenRoom}}}
	h, err := HiddenSet("keq0000002", md, false)
	if err != nil || !h[0x0a0202] || !h[0x000002] || len(h) != 2 {
		t.Fatalf("hidden set %v %v", h, err)
	}
	if h, _ := HiddenSet("keq0000002", md, true); len(h) != 0 {
		t.Fatalf("shown: %v", h)
	}
}

func TestResolveAddrPrefersTheProxy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	t.Setenv("MASSIMO_CUBE_ADDRESS", "")
	t.Setenv("MASSIMO_CUBE_PROXY", "")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	if a, err := ResolveAddr("test", "KEQ0000001", ProxyFallback, io.Discard); err != nil || a != "10.0.0.1" {
		t.Fatalf("address %q %v", a, err)
	}
	t.Setenv("MASSIMO_CUBE_KEQ0000001_PROXY", ln.Addr().String())
	for _, mode := range []ProxyMode{ProxyFallback, ProxyRequired} {
		if a, err := ResolveAddr("test", "KEQ0000001", mode, io.Discard); err != nil || a != ln.Addr().String() {
			t.Fatalf("proxy, mode %d: %q %v", mode, a, err)
		}
	}
	// -direct: the Cube itself although the proxy answers.
	if a, err := ResolveAddr("test", "KEQ0000001", ProxySkip, io.Discard); err != nil || a != "10.0.0.1" {
		t.Fatalf("proxy skipped: %q %v", a, err)
	}
	if _, err := ResolveAddr("test", "", ProxyFallback, io.Discard); err == nil || !strings.Contains(err.Error(), "-addr is required") {
		t.Fatalf("no cube: %v", err)
	}
}

// deadAddr returns a local address nothing listens on.
func deadAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	a := ln.Addr().String()
	ln.Close()
	return a
}

func TestResolveAddrWhenTheProxyDoesNotAnswer(t *testing.T) {
	dead := deadAddr(t)
	t.Setenv("MASSIMO_CUBE_ADDRESS", "")
	t.Setenv("MASSIMO_CUBE_PROXY", "")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_PROXY", dead)
	// Default: the Cube itself, with a warning.
	var log strings.Builder
	if a, err := ResolveAddr("status", "KEQ0000001", ProxyFallback, &log); err != nil || a != "10.0.0.1" {
		t.Fatalf("fallback: %q %v", a, err)
	}
	for _, s := range []string{"warning:", dead + " does not answer", "connecting to the cube directly", "only connection", "-no-direct"} {
		if !strings.Contains(log.String(), s) {
			t.Fatalf("warning %q lacks %q", log.String(), s)
		}
	}
	// -no-direct: an error naming -direct and -addr.
	_, err := ResolveAddr("status", "KEQ0000001", ProxyRequired, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "status: the massimo proxy at "+dead+" does not answer") || !strings.Contains(err.Error(), "-direct") || !strings.Contains(err.Error(), "-addr") {
		t.Fatalf("-no-direct: %v", err)
	}
	// -direct: no probe, no warning.
	log.Reset()
	if a, err := ResolveAddr("status", "KEQ0000001", ProxySkip, &log); err != nil || a != "10.0.0.1" || log.Len() != 0 {
		t.Fatalf("-direct: %q %v log %q", a, err, log.String())
	}
}

func TestKnown(t *testing.T) {
	for _, k := range []string{"MASSIMO_CUBE_ADDRESS", "MASSIMO_CUBE_KEQ1_PROXY_SERIAL", "MASSIMO_LISTEN", "MASSIMO_API_TOKEN", "MASSIMO_DATA", "MASSIMO_LOG_LEVEL", "MASSIMO_UPDATE_CHANNEL", "MASSIMO_UPDATE_API", "MASSIMO_GITHUB_TOKEN", "MASSIMO_REBOOT"} {
		if !Known(k) {
			t.Errorf("%s unknown", k)
		}
	}
	for _, k := range []string{"MASSIMO_CUBE_KEQ1_KYE", "MASSIMO_CUBE_A_B_KEY", "MASSIMO_FOO"} {
		if Known(k) {
			t.Errorf("%s known", k)
		}
	}
}

// The Cube keys and IVs of the portal commands, gone (AP PortalDrop
// v1.0): neither known nor unknown, but removed, named as ignored.
func TestRemoved(t *testing.T) {
	for _, k := range []string{"MASSIMO_CUBE_KEY", "MASSIMO_CUBE_IV", "MASSIMO_CUBE_KEQ1_KEY", "MASSIMO_CUBE_KEQ1_IV"} {
		if !Removed(k) || Known(k) {
			t.Errorf("%s: removed %v, known %v", k, Removed(k), Known(k))
		}
	}
	for _, k := range []string{"MASSIMO_TLS_KEY", "MASSIMO_CUBE_A_B_KEY", "MASSIMO_CUBE_ADDRESS", "MASSIMO_CUBE_KEQ1_ADDRESS"} {
		if Removed(k) {
			t.Errorf("%s removed", k)
		}
	}
}

// A serial written in lower case in a variable's name, as Linux keeps
// it: Serials lists it in upper case, and the per-Cube lookups find it.
func TestLowerCaseSerial(t *testing.T) {
	t.Setenv("MASSIMO_CUBE_keq0000000_ADDRESS", "192.0.2.5")
	t.Setenv("MASSIMO_CUBE_keq0000000_IGNORE", "0abcde")
	sns, addrs := Serials(Address)
	if len(sns) != 1 || sns[0] != "KEQ0000000" || addrs["KEQ0000000"] != "192.0.2.5" {
		t.Fatalf("serials %v %v", sns, addrs)
	}
	if v := Get(Address, "KEQ0000000"); v != "192.0.2.5" {
		t.Fatalf("Get: %q", v)
	}
	if v := Own(Address, "keq0000000", false); v != "192.0.2.5" {
		t.Fatalf("Own: %q", v)
	}
	if list, err := HiddenList("KEQ0000000"); err != nil || len(list) != 1 || list[0].String() != "0abcde" {
		t.Fatalf("hidden %v %v", list, err)
	}
}

// PerCubeValue names the variable as it is set, for massimo config and
// the store's file view.
func TestPerCubeValue(t *testing.T) {
	t.Setenv("MASSIMO_CUBE_keq0000000_ADDRESS", "192.0.2.5")
	if name, v := PerCubeValue(Address, "KEQ0000000"); name != "MASSIMO_CUBE_keq0000000_ADDRESS" || v != "192.0.2.5" {
		t.Fatalf("PerCubeValue: %s=%q", name, v)
	}
	if name, v := PerCubeValue(Address, "KEQ0000001"); name != "MASSIMO_CUBE_KEQ0000001_ADDRESS" || v != "" {
		t.Fatalf("PerCubeValue of a Cube without one: %s=%q", name, v)
	}
}
