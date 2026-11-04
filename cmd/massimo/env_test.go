// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import "testing"

func TestGlobalOptionsAndCubeEnv(t *testing.T) {
	t.Setenv("MASSIMO_CUBE_KEY", "default")
	t.Setenv("MASSIMO_CUBE_KEY_KEQ0000001", "one")
	t.Setenv("MASSIMO_CUBE_ADDRESS_KEQ0000001", "10.0.0.1")
	t.Setenv("MASSIMO_ENV_FILE", "")
	selectedSN = ""
	rest, err := globalOptions([]string{"-env", "x.env", "--sn=keq0000001", "cube", "-addr", "y"})
	if err != nil || len(rest) != 3 || rest[0] != "cube" {
		t.Fatalf("rest %v err %v", rest, err)
	}
	if selectedSN != "keq0000001" {
		t.Fatalf("sn %q", selectedSN)
	}
	if v := cubeEnv("MASSIMO_CUBE_KEY"); v != "one" {
		t.Fatalf("key %q", v)
	}
	if v := cubeEnv("MASSIMO_CUBE_IV"); v != "" {
		t.Fatalf("iv %q", v)
	}
	addr := ""
	if err := defaultAddr(&addr, "cube"); err != nil || addr != "10.0.0.1" {
		t.Fatalf("addr %q err %v", addr, err)
	}
	selectedSN = ""
	if v := cubeEnv("MASSIMO_CUBE_KEY"); v != "default" {
		t.Fatalf("key %q", v)
	}
	if rest, _ := globalOptions([]string{"--version"}); len(rest) != 1 || rest[0] != "--version" {
		t.Fatalf("version alias %v", rest)
	}
	if n := len(perCubeKeys()); n != 1 {
		t.Fatalf("per-cube keys %d", n)
	}
}
