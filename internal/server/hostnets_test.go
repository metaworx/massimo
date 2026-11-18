// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"net"
	"sync/atomic"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// nets parses CIDR networks for a test.
func nets(t *testing.T, cidrs ...string) []*net.IPNet {
	t.Helper()
	var out []*net.IPNet
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}

// An address in none of the host's networks is in another network, with
// or without its port; a host name, or a host whose networks are not
// known, is not judged.
func TestOtherNetwork(t *testing.T) {
	home := nets(t, "192.168.178.0/24", "127.0.0.0/8", "fd00::/64")
	for _, c := range []struct {
		addr string
		nets []*net.IPNet
		want bool
	}{
		{"192.168.0.222", home, true},
		{"192.168.0.222:62910", home, true},
		{"192.168.178.21", home, false},
		{"192.168.178.21:62910", home, false},
		{"127.0.0.1:62910", home, false},
		{"[fd00::21]:62910", home, false},
		{"[fd01::21]:62910", home, true},
		{"cube.example", home, false},
		{"cube.example:62910", home, false},
		{"", home, false},
		{"192.168.0.222", nil, false},
	} {
		if got := otherNetwork(c.nets, c.addr); got != c.want {
			t.Errorf("otherNetwork(%q, %d networks) = %v, want %v", c.addr, len(c.nets), got, c.want)
		}
	}
}

// A found Cube and a served one whose address lies outside the host's
// networks say so, in the discovery's answer, the Cube listing and the
// cube_found event; inside them, the field is absent. A proxy is never
// marked.
func TestFoundCubeInAnotherNetwork(t *testing.T) {
	other, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	// The fake Cubes answer from 127.0.0.1, outside this host's networks
	// as the test gives them, until the loopback network is among them.
	var host atomic.Pointer[[]*net.IPNet]
	lan := nets(t, "192.0.2.0/24")
	host.Store(&lan)
	r := startServer(t, discovering(Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{other.Addr()}, hostNets: func() []*net.IPNet { return *host.Load() }}))
	ts, s := r.ts, r.s

	code, out := postDiscovery(t, ts)
	found, _ := out["found"].([]any)
	if code != 200 || len(found) != 1 || found[0].(map[string]any)["other_network"] != true {
		t.Fatalf("discover %d %v", code, out)
	}
	var cubes []map[string]any
	if getJSON(t, ts, "", "/api/v1/cubes", &cubes) != 200 || len(cubes) != 2 {
		t.Fatalf("cubes %v", cubes)
	}
	if cubes[0]["serial"] != "KEQ0000100" || cubes[0]["other_network"] != true {
		t.Fatalf("the served Cube at 127.0.0.1 %v", cubes[0])
	}
	if cubes[1]["serial"] != "KEQ0000000" || cubes[1]["other_network"] != true {
		t.Fatalf("the found Cube at 127.0.0.1 %v", cubes[1])
	}
	var evs []event
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log", &evs)
	seen := false
	for _, e := range evs {
		if e.Kind == "cube_found" {
			seen = true
			if e.Cube == nil || !e.Cube.OtherNetwork {
				t.Fatalf("cube_found without other_network %+v", e.Cube)
			}
		}
	}
	if !seen {
		t.Fatal("no cube_found")
	}

	proxy := []foundCube{{Serial: "KEQ0000300", Address: "127.0.0.1", Kind: cubelan.KindProxy}}
	s.describeFound(proxy)
	if proxy[0].OtherNetwork {
		t.Fatal("a proxy marked as in another network")
	}

	both := nets(t, "192.0.2.0/24", "127.0.0.0/8")
	host.Store(&both)
	cubes = nil
	getJSON(t, ts, "", "/api/v1/cubes", &cubes)
	for _, c := range cubes {
		if _, ok := c["other_network"]; ok {
			t.Fatalf("other_network inside the host's networks %v", c)
		}
	}
}
