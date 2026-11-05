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
	t.Setenv("MASSIMO_CUBE_IGNORE", "001867, 001F32")
	t.Setenv("MASSIMO_CUBE_KEQ0000002_IGNORE", "005c32")
	got, err := HiddenList("")
	if err != nil || len(got) != 2 || got[0] != 0x001867 || got[1] != 0x001f32 {
		t.Fatalf("plain list %v err %v", got, err)
	}
	got, err = HiddenList("keq0000002")
	if err != nil || len(got) != 1 || got[0] != 0x005c32 {
		t.Fatalf("per-cube list %v err %v", got, err)
	}
	if _, err := ParseList("MASSIMO_CUBE_IGNORE", "zz"); err == nil || !strings.Contains(err.Error(), "MASSIMO_CUBE_IGNORE") {
		t.Fatalf("bad entry accepted: %v", err)
	}
	md := &cube.Metadata{Devices: []cube.DeviceMeta{{RFAddress: 0x000002, RoomID: cube.HiddenRoom}}}
	h, err := HiddenSet("keq0000002", md, false)
	if err != nil || !h[0x005c32] || !h[0x000002] || len(h) != 2 {
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
	if a, err := ResolveAddr("test", "KEQ0000001", io.Discard); err != nil || a != "10.0.0.1" {
		t.Fatalf("address %q %v", a, err)
	}
	t.Setenv("MASSIMO_CUBE_KEQ0000001_PROXY", ln.Addr().String())
	if a, err := ResolveAddr("test", "KEQ0000001", io.Discard); err != nil || a != ln.Addr().String() {
		t.Fatalf("proxy %q %v", a, err)
	}
	if _, err := ResolveAddr("test", "", io.Discard); err == nil || !strings.Contains(err.Error(), "-addr is required") {
		t.Fatalf("no cube: %v", err)
	}
}

func TestKnown(t *testing.T) {
	for _, k := range []string{"MASSIMO_CUBE_KEY", "MASSIMO_CUBE_KEQ1_PROXY_SERIAL", "MASSIMO_LISTEN", "MASSIMO_API_TOKEN"} {
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
