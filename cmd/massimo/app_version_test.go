// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestAppVersion: packaging/app-version.sh, the Android app's versionName
// and versionCode (AP MobileApp, D21-A), in a repository of its own whose
// history holds version tags, tags that only look like one and a rolling
// tag: what a nightly build is called, what make release writes for the
// release to come, and what check accepts.
func TestAppVersion(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("a release is cut on Linux or macOS")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "packaging", "app-version.sh"))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	repo := filepath.Join(home, "repo")
	// No configuration of the machine's: its commits and tags may be
	// signed with a key the test cannot reach.
	env := []string{"HOME=" + home, "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid"}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") && !strings.HasPrefix(kv, "HOME=") {
			env = append(env, kv)
		}
	}
	command := func(dir, name string, args ...string) (string, error) {
		cmd := exec.Command(name, args...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	git := func(args ...string) string {
		t.Helper()
		out, err := command(repo, "git", args...)
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
		return out
	}
	commit := func(n int) {
		t.Helper()
		for range n {
			git("commit", "-q", "--allow-empty", "-m", "a change")
		}
	}
	tag := func(name string) { t.Helper(); git("tag", "-a", "-m", name, name) }
	if err := os.MkdirAll(filepath.Join(repo, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	git("init", "-q", "-b", "main")
	commit(1)
	tag("v0.1.0-rc") // looks like one, and is no version tag
	commit(1)
	tag("v0.1.0-alpha.1")
	commit(3)
	tag("release/nightly")
	tag("v1.2")
	alpha1 := git("rev-parse", "v0.1.0-alpha.1^{commit}")
	mid := git("rev-parse", "HEAD")
	commit(1)
	tag("v0.1.0-alpha.2")

	type want struct{ args, out string }
	check := func(cases []want) {
		t.Helper()
		for _, tc := range cases {
			out, err := command(repo, bash, append([]string{script}, strings.Fields(tc.args)...)...)
			if refusal, ok := strings.CutPrefix(tc.out, "!"); ok {
				if err == nil || !strings.Contains(out, refusal) {
					t.Errorf("app-version.sh %s = %q, %v; want a refusal naming %q", tc.args, out, err, refusal)
				}
				continue
			}
			if err != nil || out != tc.out {
				t.Errorf("app-version.sh %s = %q, %v; want %q", tc.args, out, err, tc.out)
			}
		}
	}
	short := func(c string) string { return git("rev-parse", "--short", c) }
	check([]want{
		{"describe " + alpha1, "0.1.0-alpha.1 1000000"},
		{"describe " + mid, "0.1.0-alpha.1-3-g" + short(mid) + " 1000003"},
		{"describe", "0.1.0-alpha.2 1000004"},
		{"release v0.1.0-beta.1", "1000005"},
		{"release v0.1.0 2", "1000006"},
		{"release v0.2.0-alpha.1 2", "2000000"},
		{"release v0.1.0-alpha.2", "!exists already"},
		{"release 0.2.0", "!not a version tag"},
		{"release v0.2.0-Beta.1", "!not a version tag"},
		{"release v21.0.0", "!the major is at most 20"},
		{"release v0.100.0", "!the minor is at most 99"},
		{"release v0.1.100", "!the patch is at most 99"},
		{"release v0.2.0 x", "!usage"},
		{"check", "!app/version.json is missing"},
		{"bogus", "!usage"},
	})

	// The release make release cuts: the file, its commit, its tag.
	check([]want{{"write v0.1.0-beta.1", "app-version: app/version.json is 0.1.0-beta.1, 1000005"}})
	git("add", "app/version.json")
	commit(1)
	tag("v0.1.0-beta.1")
	check([]want{
		{"check", "app-version: app/version.json is 0.1.0-beta.1, 1000005"},
		{"check v0.1.0-beta.1", "app-version: app/version.json is 0.1.0-beta.1, 1000005"},
		{"check v0.1.0", "!names 0.1.0-beta.1, not v0.1.0"},
		{"describe", "0.1.0-beta.1 1000005"},
	})
	// A versionCode that is not the tag's, and a version without a tag.
	for body, refusal := range map[string]string{
		"{\n  \"version\": \"0.1.0-beta.1\",\n  \"versionCode\": 1000004\n}\n": "its tag's is 1000005",
		`{"version": "0.1.0-beta.1", "versionCode": 1000005}`:                  `one "version"`,
		"{\n  \"version\": \"0.1.0-beta.2\",\n  \"versionCode\": 1000006\n}\n": "there is no tag v0.1.0-beta.2",
		"{\n  \"version\": \"0.1.0-beta.1\"\n}\n":                              `one "versionCode"`,
	} {
		if err := os.WriteFile(filepath.Join(repo, "app", "version.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		check([]want{{"check", "!" + refusal}})
	}

	// A shallow clone cannot count.
	shallow := filepath.Join(home, "shallow")
	if out, err := command(home, "git", "clone", "-q", "--depth", "1", "file://"+repo, shallow); err != nil {
		t.Fatalf("git clone --depth 1: %v: %s", err, out)
	}
	if out, err := command(shallow, bash, script, "describe"); err == nil || !strings.Contains(out, "a shallow clone") {
		t.Errorf("app-version.sh describe in a shallow clone = %q, %v; want a refusal", out, err)
	}
}
