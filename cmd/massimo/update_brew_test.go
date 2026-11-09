// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/update"
)

// brewBox is a Homebrew prefix with the formula massimo-alpha at the
// version before linked, and a stand-in for brew that links after on
// upgrade, or fails with upgradeErr.
type brewBox struct {
	prefix, before, after string
	upgradeErr            error
	calls                 []string
}

func (b *brewBox) keg(v string) string {
	return filepath.Join(b.prefix, "Cellar", "massimo-alpha", v)
}

func (b *brewBox) link(t *testing.T, v string) {
	t.Helper()
	opt := filepath.Join(b.prefix, "opt", "massimo-alpha")
	os.Remove(opt)
	if err := os.Symlink(filepath.Join("..", "Cellar", "massimo-alpha", v), opt); err != nil {
		t.Fatal(err)
	}
}

func (b *brewBox) run(t *testing.T) func(context.Context, string, ...string) ([]byte, error) {
	return func(_ context.Context, brew string, args ...string) ([]byte, error) {
		b.calls = append(b.calls, filepath.Base(brew)+" "+strings.Join(args, " "))
		if brew != filepath.Join(b.prefix, "bin", "brew") {
			return nil, errors.New("not the prefix's brew: " + brew)
		}
		if args[0] == "upgrade" {
			if b.upgradeErr != nil {
				return []byte("==> Upgrading massimo-alpha\nError: An exception occurred within a child process"), b.upgradeErr
			}
			os.MkdirAll(filepath.Join(b.keg(b.after), "bin"), 0o755)
			b.link(t, b.after)
		}
		return nil, nil
	}
}

// brewHelper sets up a box brew owns massimo on, run as a user.
func brewHelper(t *testing.T, b *brewBox) (request, status string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Homebrew does not run on Windows")
	}
	b.prefix = t.TempDir()
	for _, d := range []string{filepath.Join(b.keg(b.before), "bin"), filepath.Join(b.prefix, "bin"), filepath.Join(b.prefix, "opt")} {
		os.MkdirAll(d, 0o755)
	}
	os.WriteFile(filepath.Join(b.prefix, "bin", "brew"), []byte("#!/bin/sh\n"), 0o755)
	b.link(t, b.before)
	oldPackaged, oldRun, oldExe, oldEuid, oldVersion, oldHealth := packaged, brewRun, brewExecutable, geteuid, version, healthEvery
	t.Cleanup(func() {
		packaged, brewRun, brewExecutable, geteuid, version, healthEvery = oldPackaged, oldRun, oldExe, oldEuid, oldVersion, oldHealth
	})
	packaged = func() (install.Manager, bool) {
		return install.Manager{Name: "brew", Command: "brew upgrade massimo-alpha"}, true
	}
	brewRun = b.run(t)
	brewExecutable = func() (string, error) { return filepath.Join(b.keg(b.before), "bin", "massimo"), nil }
	oldRestart := restartService
	t.Cleanup(func() { restartService = oldRestart })
	restartService = func(name string) error {
		b.calls = append(b.calls, "restart "+name)
		return nil
	}
	geteuid = func() int { return 501 }
	version, healthEvery = "v"+b.before, 10*time.Millisecond
	t.Setenv("HOMEBREW_NO_AUTO_UPDATE", "")
	request, status = filepath.Join(b.prefix, "update-request"), filepath.Join(b.prefix, "status.json")
	if err := update.WriteRequest(request, "main"); err != nil {
		t.Fatal(err)
	}
	return request, status
}

// TestBrewUpdate: the helper runs brew update and brew upgrade with the
// prefix's brew, and reports the version brew linked once the server
// runs it.
func TestBrewUpdate(t *testing.T) {
	b := &brewBox{before: "0.12.0-alpha.1", after: "0.13.0-alpha.1"}
	req, status := brewHelper(t, b)
	if err := runUpdate([]string{"-request", req, "-status", status, "-restart", "gui/501/homebrew.mxcl.massimo-alpha", "-health", healthServer(t, "v0.13.0-alpha.1")}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(b.calls, "\n"); got != "brew update --quiet\nbrew upgrade massimo-alpha\nrestart gui/501/homebrew.mxcl.massimo-alpha" {
		t.Fatalf("ran\n%s", got)
	}
	st := readStatus(t, status)
	if st.State != "done" || st.From != "v0.12.0-alpha.1" || st.To != "v0.13.0-alpha.1" {
		t.Fatalf("status %+v", st)
	}
}

// TestBrewUpdateRespectsNoAutoUpdate: HOMEBREW_NO_AUTO_UPDATE keeps brew
// update out, as brew upgrade would.
func TestBrewUpdateRespectsNoAutoUpdate(t *testing.T) {
	b := &brewBox{before: "0.12.0-alpha.1", after: "0.12.0-alpha.1"}
	req, status := brewHelper(t, b)
	t.Setenv("HOMEBREW_NO_AUTO_UPDATE", "1")
	if err := runUpdate([]string{"-request", req, "-status", status}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(b.calls, "\n"); got != "brew upgrade massimo-alpha" {
		t.Fatalf("ran\n%s", got)
	}
	if st := readStatus(t, status); st.State != "up-to-date" || !strings.Contains(st.Message, "brew has nothing newer") {
		t.Fatalf("status %+v", st)
	}
}

// TestBrewUpdateFailures: brew's error line reaches the status; root and
// a server on the old version are refused or reported.
func TestBrewUpdateFailures(t *testing.T) {
	b := &brewBox{before: "0.12.0-alpha.1", after: "0.13.0-alpha.1", upgradeErr: errors.New("exit status 1")}
	req, status := brewHelper(t, b)
	err := runUpdate([]string{"-request", req, "-status", status})
	if err == nil || !strings.Contains(err.Error(), "brew upgrade massimo-alpha: exit status 1: Error: An exception occurred") {
		t.Fatalf("upgrade: %v", err)
	}
	if st := readStatus(t, status); st.State != "failed" || !strings.Contains(st.Message, "Error: An exception") {
		t.Fatalf("status %+v", st)
	}

	b = &brewBox{before: "0.12.0-alpha.1", after: "0.13.0-alpha.1"}
	req, status = brewHelper(t, b)
	geteuid = func() int { return 0 }
	if err := runUpdate([]string{"-request", req, "-status", status}); err == nil || !strings.Contains(err.Error(), "brew does not run as root") {
		t.Fatalf("root: %v", err)
	}
	if len(b.calls) != 0 {
		t.Fatalf("ran as root: %v", b.calls)
	}

	b = &brewBox{before: "0.12.0-alpha.1", after: "0.13.0-alpha.1"}
	req, status = brewHelper(t, b)
	err = runUpdate([]string{"-request", req, "-status", status, "-restart", "gui/501/homebrew.mxcl.massimo-alpha", "-health", healthServer(t, "v0.12.0-alpha.1"), "-health-timeout", "50ms"})
	if err == nil || !strings.Contains(err.Error(), "brew installed v0.13.0-alpha.1, but") {
		t.Fatalf("health: %v", err)
	}
}

// TestBrewKegOf: the keg is read with filepath, so on Windows, where the
// test runs too, the prefix comes back with its separators.
func TestBrewKegOf(t *testing.T) {
	k, err := brewKegOf("/opt/homebrew/Cellar/massimo-alpha/0.13.0-alpha.1/bin/massimo")
	if err != nil || k.prefix != filepath.FromSlash("/opt/homebrew") || k.formula != "massimo-alpha" || k.version != "0.13.0-alpha.1" {
		t.Fatalf("%+v, %v", k, err)
	}
	if _, err := brewKegOf("/usr/local/bin/massimo"); err == nil {
		t.Fatal("a binary outside a Cellar read as a keg")
	}
}

func TestHomebrewJob(t *testing.T) {
	for label, want := range map[string]bool{
		"homebrew.mxcl.massimo": true, "homebrew.mxcl.massimo-alpha": true,
		"homebrew.mxcl.massimoX": false, "ch.metaworx.massimo": false, "": false,
	} {
		if got := homebrewJob(label); got != want {
			t.Errorf("homebrewJob(%q) = %v", label, got)
		}
	}
}

// TestBrewHelperPlan: only brew services' job of a user, with brew
// beside the keg, starts the helper.
func TestBrewHelperPlan(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Homebrew does not run on Windows")
	}
	prefix := t.TempDir()
	exe := filepath.Join(prefix, "Cellar", "massimo-alpha", "0.13.0-alpha.1", "bin", "massimo")
	if _, err := brewHelperPlan(exe, 501, "homebrew.mxcl.massimo-alpha"); err == nil || !strings.Contains(err.Error(), "no brew") {
		t.Fatalf("no brew: %v", err)
	}
	os.MkdirAll(filepath.Join(prefix, "bin"), 0o755)
	os.WriteFile(filepath.Join(prefix, "bin", "brew"), nil, 0o755)
	if k, err := brewHelperPlan(exe, 501, "homebrew.mxcl.massimo-alpha"); err != nil || k.formula != "massimo-alpha" {
		t.Fatalf("brew services: %+v, %v", k, err)
	}
	if _, err := brewHelperPlan(exe, 0, "homebrew.mxcl.massimo-alpha"); err == nil || !strings.Contains(err.Error(), "sudo brew services restart massimo-alpha") {
		t.Fatalf("root: %v", err)
	}
	if _, err := brewHelperPlan(exe, 501, ""); err == nil || !strings.Contains(err.Error(), "brew services start massimo-alpha") {
		t.Fatalf("by hand: %v", err)
	}
}
