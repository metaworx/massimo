// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNextVersion: packaging/next-version.sh, the tag make release <flavour>
// cuts next, for tag lists that hold rolling tags and tags that only look
// like a version beside the version tags.
func TestNextVersion(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	script := filepath.Join("..", "..", "packaging", "next-version.sh")
	noise := "release/devel\nrelease/nightly\nrelease/alpha\nverification-test\nv1.2\nv9.9.9-rc\n"
	alpha := noise + "v0.13.0-alpha.2\nv0.14.0-alpha.1\n"
	beta := alpha + "v0.14.0-beta.1\nv0.14.0-beta.2\n"
	stable := beta + "v0.14.0-rc.1\nv0.14.0\n"
	tens := "v0.9.0-alpha.10\nv0.10.0-alpha.9\nv0.10.0-alpha.10\n"
	for _, tc := range []struct {
		name, tags, args, want string
	}{
		{"alpha counts on", alpha, "alpha", "v0.14.0-alpha.2"},
		{"alpha, the next minor", alpha, "alpha minor", "v0.15.0-alpha.1"},
		{"the first beta", alpha, "beta", "v0.14.0-beta.1"},
		{"the first rc", alpha, "rc", "v0.14.0-rc.1"},
		{"stable", alpha, "stable", "v0.14.0"},
		{"a fix of the version", alpha, "stable patch", "v0.14.1"},
		{"beta counts on", beta, "beta", "v0.14.0-beta.3"},
		{"rc after beta", beta, "rc", "v0.14.0-rc.1"},
		{"alpha after beta, a new minor", beta, "alpha minor", "v0.15.0-alpha.1"},
		{"after stable, a new minor", stable, "alpha minor", "v0.15.0-alpha.1"},
		{"after stable, a patch", stable, "stable patch", "v0.14.1"},
		{"a major", stable, "beta major", "v1.0.0-beta.1"},
		{"numbers, not text", tens, "alpha", "v0.10.0-alpha.11"},
		{"alpha after beta", beta, "alpha", "!v0.14.0 has a beta release already"},
		{"beta after rc", stable + "v0.15.0-rc.1\n", "beta", "!v0.15.0 has a rc release already"},
		{"a pre-release after stable", stable, "rc", "!v0.14.0 is released as stable"},
		{"stable twice", stable, "stable", "!v0.14.0 is released as stable"},
		{"no version tag", noise, "alpha", "!no version tag"},
		{"a rolling flavour", alpha, "devel", "!usage"},
		{"an unknown bump", alpha, "alpha huge", "!usage"},
	} {
		cmd := exec.Command(bash, append([]string{script}, strings.Fields(tc.args)...)...)
		cmd.Stdin = strings.NewReader(tc.tags)
		out, err := cmd.CombinedOutput()
		got := strings.TrimSpace(string(out))
		if want, ok := strings.CutPrefix(tc.want, "!"); ok {
			if err == nil || !strings.Contains(got, want) {
				t.Errorf("%s: next-version.sh %s = %q, %v; want a refusal naming %q", tc.name, tc.args, got, err, want)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("%s: next-version.sh %s = %q, %v; want %q", tc.name, tc.args, got, err, tc.want)
		}
	}
}

// TestMakeReleaseRefuses: make release, which cuts a release since
// make build-release took the build, stops before it touches git where
// it has no flavour from the command line, the build's VERSION or
// RELEASE, a word that is no flavour, or two.
func TestMakeReleaseRefuses(t *testing.T) {
	makeCmd, err := exec.LookPath("make")
	if err != nil {
		t.Skip("no make")
	}
	for args, want := range map[string]string{
		"release":                     "make release <alpha|beta|rc|stable> cuts a release",
		"release VERSION=v0.14.0":     "the build of a release is make build-release",
		"release beta RELEASE=x":      "the build of a release is make build-release",
		"release devel":               "devel is not a flavour a release is cut as",
		"release bogus":               "bogus is not a flavour a release is cut as",
		"release beta FLAVOUR=rc":     "beta or FLAVOUR=rc, not both",
		"release beta rc":             "one flavour, not beta rc",
		"release-stable FLAVOUR=beta": "stable or FLAVOUR=beta, not both",
	} {
		cmd := exec.Command(makeCmd, append([]string{"-s", "--no-print-directory", "-C", filepath.Join("..", "..")}, strings.Fields(args)...)...)
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), want) {
			t.Errorf("make %s: %v: %s; want a stop naming %q", args, err, out, want)
		}
	}
}
