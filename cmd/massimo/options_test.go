// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"net"
	"strings"
	"testing"
)

func TestGlobalOptionsSelectTheCube(t *testing.T) {
	t.Setenv("MASSIMO_CUBE_KEY", "default")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_KEY", "one")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	t.Setenv("MASSIMO_CUBE_SN", "ignored-here")
	t.Setenv("MASSIMO_ENV_FILE", "")
	t.Cleanup(func() { selectedSN, snExplicit, showHidden = "", false, false })
	selectedSN = ""
	rest, err := globalOptions([]string{"-env", "x.env", "--sn=keq0000001", "-show-hidden", "cube", "-addr", "y"})
	if err != nil || len(rest) != 3 || rest[0] != "cube" {
		t.Fatalf("rest %v err %v", rest, err)
	}
	if selectedSN != "keq0000001" || !snExplicit || !showHidden {
		t.Fatalf("sn %q explicit %v show %v", selectedSN, snExplicit, showHidden)
	}
	if v := cubeEnv("MASSIMO_CUBE_KEY"); v != "one" {
		t.Fatalf("key %q", v)
	}
	addr := ""
	if err := defaultAddr(&addr, "cube"); err != nil || addr != "10.0.0.1" {
		t.Fatalf("addr %q err %v", addr, err)
	}
	addr = "given"
	if err := defaultAddr(&addr, "cube"); err != nil || addr != "given" {
		t.Fatalf("given addr %q err %v", addr, err)
	}
	if l, err := hiddenDevices(); err != nil || l != nil {
		t.Fatalf("hidden with -show-hidden %v %v", l, err)
	}
	if rest, _ := globalOptions([]string{"--version"}); len(rest) != 1 || rest[0] != "--version" {
		t.Fatalf("version alias %v", rest)
	}
	selectedSN, snExplicit = "", false
	useDeviceSerial("KEQ0000001")
	if v := cubeEnv("MASSIMO_CUBE_KEY"); v != "one" {
		t.Fatalf("key after device serial %q", v)
	}
}

func TestDirectAllowsTheCubeWhenTheProxyIsDown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := ln.Addr().String()
	ln.Close()
	t.Setenv("MASSIMO_CUBE_ADDRESS", "")
	t.Setenv("MASSIMO_CUBE_PROXY", "")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_PROXY", dead)
	t.Cleanup(func() { selectedSN, snExplicit, direct = "", false, false })
	selectedSN, snExplicit, direct = "KEQ0000001", true, false
	addr := ""
	if err := defaultAddr(&addr, "backup"); err == nil || !strings.Contains(err.Error(), "-direct") {
		t.Fatalf("without -direct: addr %q err %v", addr, err)
	}
	addr = "given"
	if err := defaultAddr(&addr, "backup"); err != nil || addr != "given" {
		t.Fatalf("-addr wins: %q %v", addr, err)
	}
	rest, err := globalOptions([]string{"-direct", "backup"})
	if err != nil || len(rest) != 1 || rest[0] != "backup" || !direct {
		t.Fatalf("rest %v err %v direct %v", rest, err, direct)
	}
	addr = ""
	if err := defaultAddr(&addr, "backup"); err != nil || addr != "10.0.0.1" {
		t.Fatalf("with -direct: addr %q err %v", addr, err)
	}
}
