// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/update"
)

// TestPackagedUpdate: the button goes through apt only with the massimo
// source and its suite as the channel, and while the helper is on; brew
// keeps its reason.
func TestPackagedUpdate(t *testing.T) {
	t.Parallel()
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

// aptRepo writes a local apt repository below dir with an index of the
// massimo versions given per suite, for arch, gzipped as publish.sh
// writes it.
func aptRepo(t *testing.T, dir, arch string, suites map[string][]string) {
	t.Helper()
	for suite, versions := range suites {
		var index strings.Builder
		for _, v := range versions {
			index.WriteString("Package: massimo\nVersion: " + v + "\nArchitecture: " + arch + "\n\n")
		}
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		zw.Write([]byte(index.String()))
		zw.Close()
		p := filepath.Join(dir, "dists", suite, "main", "binary-"+arch, "Packages.gz")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestPackagedUpdateSource: on a box apt updates, a release is offered
// once apt's index has it: the index of the channel's suite, which the
// helper switches the source to before it fetches, else the source's
// own; for the box's architecture, armhf for arm, and not another's.
func TestPackagedUpdateSource(t *testing.T) {
	t.Parallel()
	arch := update.AptArch(runtime.GOARCH)
	if arch == "" {
		t.Skip("the apt repository has no packages for", runtime.GOARCH)
	}
	// An architecture other than the box's: arm, or arm64 on an arm box.
	other := "arm"
	if arch == update.AptArch(other) {
		other = "arm64"
	}
	root, repo := t.TempDir(), t.TempDir()
	aptRepo(t, repo, arch, map[string][]string{
		"alpha": {"0.16.0~alpha.1", "0.16.0~alpha.2"},
		"devel": {"0.16.0~alpha.2", "0.16.0~alpha.2+3.gabc1234"},
	})
	aptRepo(t, repo, update.AptArch(other), map[string][]string{"alpha": {"0.16.0~alpha.3"}})
	// The URI as apt names a local repository, file:/srv/apt; on
	// Windows file:/C:/Users/….
	uri := filepath.ToSlash(repo)
	if !strings.HasPrefix(uri, "/") {
		uri = "/" + uri
	}
	source := filepath.Join(root, install.AptSource)
	os.MkdirAll(filepath.Dir(source), 0o755)
	os.WriteFile(source, []byte("Types: deb\nURIs: file:"+uri+"\nSuites: alpha\nComponents: main\n"), 0o644)
	upd := server.UpdateOptions{}
	packagedUpdate(&upd, install.Manager{Name: "apt"}, root, func(string) string { return "" })
	if upd.Source == nil {
		t.Fatalf("no source: %+v", upd)
	}
	ctx := t.Context()
	for channel, want := range map[string]string{"alpha": "v0.16.0-alpha.2", "devel": "v0.16.0-alpha.2-3-gabc1234", "main": "v0.16.0-alpha.2"} {
		if got, err := upd.Source(ctx, channel); err != nil || got != want {
			t.Errorf("channel %s: %q, %v; want %q", channel, got, err, want)
		}
	}
	if got, err := aptNewest(ctx, source, "alpha", other); err != nil || got != "v0.16.0-alpha.3" {
		t.Errorf("%s: %q, %v", other, got, err)
	}
	if _, err := aptNewest(ctx, source, "alpha", "riscv64"); err == nil {
		t.Error("an architecture without packages read")
	}
	if _, err := upd.Source(ctx, "beta"); err == nil {
		t.Error("a suite without an index read")
	}
}
