// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/server"
)

// TestPackagedUpdate: the button goes through apt only with the massimo
// source and its suite as the channel, and while the helper is on; brew
// keeps its reason.
func TestPackagedUpdate(t *testing.T) {
	apt := install.Manager{Name: "apt", Command: "sudo apt update && sudo apt install massimo"}
	env := func(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }
	root := t.TempDir()

	upd := server.UpdateOptions{Channel: "main"}
	packagedUpdate(&upd, install.Manager{Name: "brew", Command: "brew upgrade massimo-alpha"}, root, env(nil))
	if upd.Managed != "managed by brew: brew upgrade massimo-alpha" || upd.Manager != "" || upd.Helper != nil {
		t.Fatalf("brew: %+v", upd)
	}

	upd = server.UpdateOptions{Channel: "alpha"}
	packagedUpdate(&upd, apt, root, env(nil))
	if !strings.Contains(upd.Managed, install.AptSource+", which is missing") || upd.Manager != "apt" {
		t.Fatalf("no source: %+v", upd)
	}

	source := filepath.Join(root, install.AptSource)
	os.MkdirAll(filepath.Dir(source), 0o755)
	os.WriteFile(source, []byte("Types: deb\nURIs: https://massimo.example.invalid/apt\nSuites: alpha\nComponents: main\n"), 0o644)
	upd = server.UpdateOptions{Channel: "main"}
	packagedUpdate(&upd, apt, root, env(map[string]string{"MASSIMO_UPDATE_CHANNEL": "main"}))
	if !strings.Contains(upd.Managed, "apt follows the suite alpha") || !strings.Contains(upd.Managed, "MASSIMO_UPDATE_CHANNEL=main") {
		t.Fatalf("another channel: %+v", upd)
	}

	// The build's own channel gives way to the suite.
	upd = server.UpdateOptions{Channel: "main"}
	packagedUpdate(&upd, apt, root, env(map[string]string{"MASSIMO_UPDATE_CHANNEL": "alpha"}))
	if upd.Managed != "" || upd.Manager != "apt" || upd.Channel != "alpha" || upd.Helper == nil {
		t.Fatalf("apt: %+v", upd)
	}
	// Without the variable the channel is the suite, read at each check:
	// the update helper switches it.
	upd = server.UpdateOptions{}
	packagedUpdate(&upd, apt, root, env(nil))
	if upd.Managed != "" || upd.Channel != "" || upd.Suite == nil || upd.Suite() != "alpha" {
		t.Fatalf("apt without the variable: %+v", upd)
	}
	os.WriteFile(source, []byte("Types: deb\nURIs: https://massimo.example.invalid/apt\nSuites: devel\nComponents: main\n"), 0o644)
	if s := upd.Suite(); s != "devel" {
		t.Fatalf("the suite after a switch: %q", s)
	}
	if why := upd.Helper(); !strings.Contains(why, "the update helper is off") {
		t.Fatalf("helper off: %q", why)
	}
	want := filepath.Join(root, install.UpdatePathWant)
	os.MkdirAll(filepath.Dir(want), 0o755)
	// The link systemctl enable makes; a file does here, also on Windows.
	if err := os.WriteFile(want, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if why := upd.Helper(); why != "" {
		t.Fatalf("helper on: %q", why)
	}
}
