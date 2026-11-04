// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"strings"
	"testing"
)

func TestIgnoredDevices(t *testing.T) {
	t.Setenv("MASSIMO_CUBE_IGNORE", "001867, 001F32")
	t.Setenv("MASSIMO_CUBE_KEQ0000002_IGNORE", "005c32")
	selectedSN, snExplicit = "", false
	got, err := ignoredDevices()
	if err != nil || len(got) != 2 || got[0] != 0x001867 || got[1] != 0x001f32 {
		t.Fatalf("plain list %v err %v", got, err)
	}
	selectedSN = "keq0000002"
	got, err = ignoredDevices()
	if err != nil || len(got) != 1 || got[0] != 0x005c32 {
		t.Fatalf("per-cube list %v err %v", got, err)
	}
	selectedSN = ""
	if _, err := parseIgnoreList("MASSIMO_CUBE_IGNORE", "zz"); err == nil || !strings.Contains(err.Error(), "MASSIMO_CUBE_IGNORE") {
		t.Fatalf("bad entry accepted: %v", err)
	}
}
