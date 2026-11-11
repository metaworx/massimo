// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// The update helper on a box whose massimo the Debian package installed,
// where the marker names apt: apt stays the owner of /usr/bin/massimo, so
// the helper runs apt instead of installing a release itself. Its
// commands are fixed and take nothing from the request, not even the
// channel: apt follows the suite of the massimo source, and updates from
// that source only. The package's postinst restarts massimo.service; the
// helper then waits until /healthz reports the version apt installed.
// Nothing is downgraded when it does not: an older binary would run on a
// store the newer one may have migrated, so the status names the way
// back instead (packaging/README.md, "Debian package").

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/update"
)

// The apt commands, run as they are: the massimo source's lists fetched
// (the other lists kept, so that apt still knows the dependencies), then
// massimo upgraded if it is installed, keeping a conffile the owner
// changed, and waiting for the dpkg lock up to two minutes.
var (
	aptUpdateArgs = []string{"update", "-q",
		"-o", "Dir::Etc::sourcelist=" + install.AptSource,
		"-o", "Dir::Etc::sourceparts=-",
		"-o", "APT::Get::List-Cleanup=0"}
	aptInstallArgs = []string{"install", "-y", "-q", "--only-upgrade",
		"-o", "Dpkg::Options::=--force-confdef",
		"-o", "Dpkg::Options::=--force-confold",
		"-o", "DPkg::Lock::Timeout=120",
		"massimo"}
)

// aptRun runs apt-get or dpkg-query and returns what it printed; a
// variable for the tests.
var aptRun = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive", "LC_ALL=C")
	return cmd.CombinedOutput()
}

// aptSourcePath is the massimo source the helper needs; a variable for
// the tests.
var aptSourcePath = install.AptSource

// aptLockWait is how long apt-get update is tried again while another
// apt run holds the lists lock, every aptLockEvery: apt-get update does
// not wait for it itself, even with DPkg::Lock::Timeout (apt 2.6 and 3.0
// measured), unlike apt-get install.
var aptLockWait, aptLockEvery = 2 * time.Minute, 5 * time.Second

// errAptBusy is what the helper reports when apt's lock stays taken.
var errAptBusy = errors.New("apt is busy (another apt run, perhaps unattended-upgrades, holds its lock); try again later")

// aptUpdate is the helper's run where apt owns massimo; health is the
// server's /healthz, empty to leave the check out.
func aptUpdate(ctx context.Context, status func(update.Status), health string, healthWait time.Duration) error {
	from := version
	fail := func(to string, err error) error {
		status(update.Status{State: "failed", From: from, To: to, Message: err.Error()})
		return err
	}
	if _, err := os.Stat(aptSourcePath); err != nil {
		return fail("", fmt.Errorf("no %s: the update helper upgrades massimo from that apt source only (packaging/README.md)", install.AptSource))
	}
	before, err := dpkgVersion(ctx)
	if err != nil {
		return fail("", err)
	}
	from = debTag(before)
	status(update.Status{State: "checking", From: from})
	if err := aptGetUpdate(ctx); err != nil {
		return fail("", err)
	}
	status(update.Status{State: "installing", From: from})
	if out, err := aptRun(ctx, "apt-get", aptInstallArgs...); err != nil {
		return fail("", aptError("apt-get install", out, err))
	}
	after, err := dpkgVersion(ctx)
	if err != nil {
		return fail("", err)
	}
	to := debTag(after)
	if after == before {
		fmt.Println("apt has nothing newer than", before)
		status(update.Status{State: "up-to-date", From: from, To: to, Message: "apt has nothing newer yet"})
		return nil
	}
	fmt.Printf("apt installed %s (was %s)\n", after, before)
	if health == "" {
		status(update.Status{State: "done", From: from, To: to, Message: "installed by apt; the server's version not checked"})
		return nil
	}
	status(update.Status{State: "restarting", From: from, To: to})
	if err := waitHealthy(ctx, health, to, healthWait); err != nil {
		return fail(to, fmt.Errorf("apt installed %s, but %v; the way back: sudo apt install massimo=%s", to, err, before))
	}
	status(update.Status{State: "done", From: from, To: to})
	fmt.Println("running", to)
	return nil
}

// aptGetUpdate fetches the massimo source's lists, again while another
// apt run holds the lock. A source apt could not fetch is an error even
// where apt-get exits 0 with a warning, so that a failed fetch is not
// reported as nothing newer.
func aptGetUpdate(ctx context.Context) error {
	deadline := time.Now().Add(aptLockWait)
	for {
		out, err := aptRun(ctx, "apt-get", aptUpdateArgs...)
		if err != nil && aptLocked(out) {
			if time.Now().Add(aptLockEvery).After(deadline) {
				return errAptBusy
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(aptLockEvery):
			}
			continue
		}
		if err == nil {
			if lines := aptLines(out, "W: Failed to fetch", "E: "); lines != "" {
				err = errors.New("the source could not be fetched")
			}
		}
		if err != nil {
			return aptError("apt-get update", out, err)
		}
		return nil
	}
}

// aptLocked says whether apt failed on a lock another process holds:
// the lists lock (apt-get update) or the dpkg frontend lock once its
// timeout ran out (apt-get install).
func aptLocked(out []byte) bool {
	return bytes.Contains(out, []byte("Could not get lock")) || bytes.Contains(out, []byte("Unable to acquire the dpkg frontend lock"))
}

// aptError is an apt command's failure, with apt's own error lines.
func aptError(what string, out []byte, err error) error {
	if aptLocked(out) {
		return errAptBusy
	}
	lines := aptLines(out, "E: ", "W: Failed to fetch")
	if lines == "" {
		lines = strings.TrimSpace(string(out))
		if i := strings.LastIndexByte(lines, '\n'); i >= 0 {
			lines = lines[i+1:]
		}
	}
	if lines == "" {
		return fmt.Errorf("%s: %v", what, err)
	}
	return fmt.Errorf("%s: %v: %s", what, err, lines)
}

// aptLines joins the lines of apt's output that start with one of the
// prefixes.
func aptLines(out []byte, prefixes ...string) string {
	var found []string
	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		for _, p := range prefixes {
			if strings.HasPrefix(line, p) {
				found = append(found, line)
				break
			}
		}
	}
	return strings.Join(found, "; ")
}

// dpkgVersion is the version of the massimo package dpkg has installed.
func dpkgVersion(ctx context.Context) (string, error) {
	out, err := aptRun(ctx, "dpkg-query", "-W", "--showformat=${Version}", "massimo")
	v := strings.TrimSpace(string(out))
	if err != nil {
		return "", fmt.Errorf("dpkg-query: %v: %s", err, v)
	}
	if v == "" {
		return "", errors.New("dpkg-query: the package massimo is not installed")
	}
	return v, nil
}

// debTag is the version the binary of a package reports: the Makefile's
// DEBVER read backwards, 0.13.0~alpha.1 for v0.13.0-alpha.1, and for a
// build between tags the part git describe adds behind the +,
// 0.13.0~alpha.1+3.gabc1234 for v0.13.0-alpha.1-3-gabc1234. A package of
// the older form, ~ throughout, reads as it did.
func debTag(v string) string {
	base, build, between := strings.Cut(v, "+")
	tag := "v" + strings.ReplaceAll(base, "~", "-")
	if between {
		tag += "-" + strings.ReplaceAll(build, ".", "-")
	}
	return tag
}
