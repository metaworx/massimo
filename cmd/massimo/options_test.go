// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import "testing"

func TestGlobalOptionsSelectTheCube(t *testing.T) {
	t.Setenv("MASSIMO_CUBE_KEY", "default")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_KEY", "one")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	t.Setenv("MASSIMO_CUBE_SN", "ignored-here")
	t.Setenv("MASSIMO_ENV_FILE", "")
	t.Cleanup(func() { selectedSN, snExplicit, showHidden = "", false, false })
	selectedSN = ""
	rest, err := globalOptions([]string{"-env", "x.env", "--sn=keq0000001", "-no-ignore", "cube", "-addr", "y"})
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
		t.Fatalf("hidden with -no-ignore %v %v", l, err)
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
