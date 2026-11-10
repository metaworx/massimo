// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"net"
	"testing"

	"go.metaworx.ch/massimo/internal/announce"
	"go.metaworx.ch/massimo/internal/cubelan"
)

// A Cube is marked proxy when the search took it for one, or when the
// search could not tell and its address is an announced server's; an
// answer the search took for a Cube's stays one, even from a server's
// address. The real Cube and the proxy that answers with its serial are
// both listed.
func TestMarkProxies(t *testing.T) {
	cube := cubelan.Discovered{Serial: "KEQ0000000", IP: net.ParseIP("192.0.2.20"), MAC: "00:1a:22:01:02:03", Kind: cubelan.KindCube}
	byKind := cubelan.Discovered{Serial: "KEQ0000000", IP: net.ParseIP("192.0.2.30"), MAC: "02:00:00:01:02:03", Kind: cubelan.KindProxy}
	byAddress := cubelan.Discovered{Serial: "KEQ0000001", IP: net.ParseIP("192.0.2.40"), Kind: cubelan.KindUnknown}
	unknown := cubelan.Discovered{Serial: "KEQ0000002", IP: net.ParseIP("192.0.2.50"), Kind: cubelan.KindUnknown}
	cubeAtServer := cubelan.Discovered{Serial: "KEQ0000003", IP: net.ParseIP("192.0.2.40"), Kind: cubelan.KindCube}
	servers := []announce.Found{{Name: "box", From: net.ParseIP("192.0.2.41"), Addrs: []net.IP{net.ParseIP("192.0.2.40")}}}
	got := markProxies([]cubelan.Discovered{cube, byKind, byAddress, unknown, cubeAtServer}, servers)
	want := []bool{false, true, true, false, false}
	if len(got) != len(want) {
		t.Fatalf("%d cubes", len(got))
	}
	for i, w := range want {
		if got[i].Proxy != w {
			t.Errorf("%s at %s: proxy %v, want %v", got[i].Serial, got[i].IP, got[i].Proxy, w)
		}
	}
	if markProxies(nil, nil) == nil {
		t.Error("no cubes: nil, which -json prints as null")
	}
}

// The announcement's default: off on a loopback address, on otherwise.
func TestLoopbackListen(t *testing.T) {
	for listen, want := range map[string]bool{"127.0.0.1:8099": true, "[::1]:8099": true, "localhost:8099": true, ":8099": false, "0.0.0.0:8099": false, "192.0.2.5:80": false, "box.example.net:8099": false} {
		if got := loopbackListen(listen); got != want {
			t.Errorf("%s: %v", listen, got)
		}
	}
}
