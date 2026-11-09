// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// The update helper where Homebrew owns massimo (the marker names brew)
// and brew services runs the server as its user's job: brew refuses to
// run as root, so there is no root helper; the server starts this one
// itself, detached, as its own user (serve_brew_darwin.go). It takes the
// request (whose channel it ignores: brew follows the formula of the
// tap) and runs brew: brew update, unless HOMEBREW_NO_AUTO_UPDATE says
// not to, as brew upgrade itself would, then brew upgrade <formula>,
// which builds the release from source. It then restarts the job, whose
// plist runs the formula's opt binary, now the new one, and waits until
// /healthz reports it. Nothing is downgraded (packaging/README.md,
// "Homebrew").

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/update"
)

// brewKeg is where Homebrew keeps this binary:
// <prefix>/Cellar/<formula>/<version>/bin/massimo.
type brewKeg struct {
	prefix, formula, version string
}

// brewKegOf reads the keg from the binary's resolved path.
func brewKegOf(exe string) (brewKeg, error) {
	bin := filepath.Dir(exe)
	ver := filepath.Dir(bin)
	formula := filepath.Dir(ver)
	cellar := filepath.Dir(formula)
	if filepath.Base(bin) != "bin" || filepath.Base(cellar) != "Cellar" {
		return brewKeg{}, fmt.Errorf("%s is not in a Homebrew Cellar", exe)
	}
	return brewKeg{prefix: filepath.Dir(cellar), formula: filepath.Base(formula), version: filepath.Base(ver)}, nil
}

// brew is the brew of the keg's prefix.
func (k brewKeg) brew() string { return filepath.Join(k.prefix, "bin", "brew") }

// linked is the version brew links as opt/<formula>, the one the job
// runs.
func (k brewKeg) linked() (string, error) {
	t, err := os.Readlink(filepath.Join(k.prefix, "opt", k.formula))
	if err != nil {
		return "", fmt.Errorf("the formula's opt link: %w", err)
	}
	return filepath.Base(t), nil
}

// brewTag is the release tag of a formula version: the formula builds
// with -X main.version=v<version>.
func brewTag(v string) string { return "v" + v }

// brewExecutable is this binary's resolved path; a variable for the
// tests.
var brewExecutable = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// brewRun runs brew with the prefix's bin first on PATH (launchd gives a
// job a bare one) and returns what it printed; a variable for the tests.
var brewRun = func(ctx context.Context, brew string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, brew, args...)
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(brew)+":/usr/bin:/bin:/usr/sbin:/sbin", "HOMEBREW_NO_ENV_HINTS=1")
	return cmd.CombinedOutput()
}

// geteuid is os.Geteuid; a variable for the tests.
var geteuid = os.Geteuid

// errBrewRoot is why a server brew runs as root updates nothing itself.
func errBrewRoot(formula string) error {
	return fmt.Errorf("brew does not run as root: brew upgrade %s, then sudo brew services restart %s", formula, formula)
}

// brewUpdate is the helper's run where brew owns massimo; restart is the
// job to restart (gui/<uid>/<label>), health the server's /healthz,
// either empty to leave it out.
func brewUpdate(ctx context.Context, status func(update.Status), restart, health string, healthWait time.Duration) error {
	from := version
	fail := func(to string, err error) error {
		status(update.Status{State: "failed", From: from, To: to, Message: err.Error()})
		return err
	}
	exe, err := brewExecutable()
	if err != nil {
		return fail("", err)
	}
	k, err := brewKegOf(exe)
	if err != nil {
		return fail("", err)
	}
	if geteuid() == 0 {
		return fail("", errBrewRoot(k.formula))
	}
	from = brewTag(k.version)
	status(update.Status{State: "checking", From: from})
	if os.Getenv("HOMEBREW_NO_AUTO_UPDATE") == "" {
		if out, err := brewRun(ctx, k.brew(), "update", "--quiet"); err != nil {
			return fail("", brewError("brew update", out, err))
		}
	}
	status(update.Status{State: "installing", From: from})
	if out, err := brewRun(ctx, k.brew(), "upgrade", k.formula); err != nil {
		return fail("", brewError("brew upgrade "+k.formula, out, err))
	}
	after, err := k.linked()
	if err != nil {
		return fail("", err)
	}
	to := brewTag(after)
	if after == k.version {
		fmt.Println("brew has nothing newer than", k.version)
		status(update.Status{State: "up-to-date", From: from, To: to, Message: "brew has nothing newer yet"})
		return nil
	}
	fmt.Printf("brew installed %s (was %s)\n", after, k.version)
	if restart == "" {
		status(update.Status{State: "done", From: from, To: to, Message: "installed by brew; restart the service to run it"})
		return nil
	}
	status(update.Status{State: "restarting", From: from, To: to})
	if err := restartService(restart); err != nil {
		return fail(to, fmt.Errorf("brew installed %s, but %v; restart it: brew services restart %s", to, err, k.formula))
	}
	if health != "" {
		if err := waitHealthy(ctx, health, to, healthWait); err != nil {
			return fail(to, fmt.Errorf("brew installed %s, but %v", to, err))
		}
	}
	status(update.Status{State: "done", From: from, To: to})
	fmt.Println("running", to)
	return nil
}

// brewError is a brew command's failure, with its last lines of output,
// where brew says what went wrong.
func brewError(what string, out []byte, err error) error {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var keep []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, "Error:") {
			keep = append(keep, l)
		}
	}
	if len(keep) == 0 && len(lines) > 0 && lines[len(lines)-1] != "" {
		keep = lines[len(lines)-1:]
	}
	if len(keep) == 0 {
		return fmt.Errorf("%s: %v", what, err)
	}
	return fmt.Errorf("%s: %v: %s", what, err, strings.Join(keep, "; "))
}

// homebrewJob says whether a launchd job label is brew services' for
// massimo: homebrew.mxcl.massimo, or that of a channel's formula such as
// homebrew.mxcl.massimo-alpha.
func homebrewJob(label string) bool {
	return label == homebrewLabel || strings.HasPrefix(label, homebrewLabel+"-")
}

// brewHelperPlan says whether this server can start the brew helper: as
// brew services' job of a user, with the binary in a keg and brew beside
// it; else the reason.
func brewHelperPlan(exe string, euid int, job string) (brewKeg, error) {
	k, err := brewKegOf(exe)
	if err != nil {
		return brewKeg{}, err
	}
	switch {
	case euid == 0:
		return k, errBrewRoot(k.formula)
	case !homebrewJob(job):
		return k, fmt.Errorf("the server is not brew services' job: brew services start %s", k.formula)
	}
	if _, err := os.Stat(k.brew()); err != nil {
		return k, errors.New("no brew beside the keg, " + k.brew())
	}
	return k, nil
}
