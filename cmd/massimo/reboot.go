// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// The reboot of the computer from the API (POST /api/v1/reboot): who
// reboots, and how serve asks. Under systemd and under the LaunchDaemon
// of massimo install, serve writes the request file a root helper
// watches (packaging/systemd/massimo-reboot.path,
// packaging/launchd/ch.metaworx.massimo-reboot.plist) and waits for the
// helper to take it; the helper removes it and reboots, and reads
// nothing serve's user can write. A Windows service, which runs as
// LocalSystem, runs shutdown.exe itself. Nothing else reboots: a server
// in a terminal would reboot someone's desktop, a container cannot
// reboot its host, and Homebrew's service runs as a user. The owner's
// switch, -allow-reboot or MASSIMO_REBOOT, comes first (internal/server,
// reboot_api.go).

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/install"
)

// rebootVar is the variable behind serve's -allow-reboot.
const rebootVar = "MASSIMO_REBOOT"

// noRebootHelper is the reason a server under systemd or launchd gives
// while the helper is not installed.
const noRebootHelper = "no reboot helper installed: sudo massimo install -reboot-helper"

// How long serve waits for the helper to take its request, and how often
// it looks; variables for the tests.
var (
	rebootTake = 30 * time.Second
	rebootPoll = 200 * time.Millisecond
)

// parseSwitch reads an on/off variable: on, true, yes, 1, or off, false,
// no, 0 and empty.
func parseSwitch(name, v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on", "true", "yes", "1":
		return true, nil
	case "", "off", "false", "no", "0":
		return false, nil
	}
	return false, fmt.Errorf("%s %q: want on or off", name, v)
}

// rebootBy says who reboots this server's computer, or, with by empty,
// why nobody does; helper, where the system needs one, says at every
// request whether it is installed: on Linux the link that enabling its
// path unit makes, on macOS its LaunchDaemon, both in directories only
// root writes, looked for below root (the tests).
func rebootBy(p processInfo, root string) (by, blocked string, helper func() string) {
	marked := func(path string) func() string {
		return func() string {
			if _, err := os.Stat(filepath.Join(root, path)); err != nil {
				return noRebootHelper
			}
			return ""
		}
	}
	switch {
	case p.service:
		return byWindows, "", nil
	case p.goos == "linux" && systemdStarted(p):
		return bySystemd, "", marked(install.RebootPathWant)
	case p.goos == "darwin" && p.getenv("XPC_SERVICE_NAME") == homebrewLabel:
		return "", "Homebrew's service runs as a user, who cannot reboot the computer", nil
	case p.goos == "darwin" && (p.ppid == 1 || p.getenv("XPC_SERVICE_NAME") == install.Label):
		return byLaunchd, "", marked(install.PlistPath(install.RebootLabel))
	}
	return "", "only a server the system runs as a service reboots its computer: systemd's, launchd's or the Windows service", nil
}

// rebootRequester returns what asks the root helper for the reboot: the
// request file at path, written to a temporary file beside it and renamed,
// then up to rebootTake for the helper to take it. A request it did not
// take is removed, so that it cannot reboot the computer at some later
// start of the helper.
func rebootRequester(path string) func() error {
	return func() error {
		tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
		line := "massimo: reboot requested at " + time.Now().UTC().Format(time.RFC3339) + "\n"
		if err := os.WriteFile(tmp, []byte(line), 0o600); err != nil {
			return fmt.Errorf("writing the reboot request: %w", err)
		}
		if err := os.Rename(tmp, path); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("writing the reboot request: %w", err)
		}
		for deadline := time.Now().Add(rebootTake); time.Now().Before(deadline); time.Sleep(rebootPoll) {
			if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
				return nil
			}
		}
		os.Remove(path)
		return fmt.Errorf("the reboot helper did not take the request within %s", rebootTake)
	}
}

// removeStaleReboot removes a request file left from before this start:
// one serve wrote and the helper never took.
func removeStaleReboot(path string, log *slog.Logger) {
	if err := os.Remove(path); err == nil {
		log.Warn("a reboot request left from before removed", "file", path)
	}
}

// runCommand runs a command and returns what it printed; a variable for
// the tests, which must not reboot anything.
var runCommand = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// windowsShutdown is the command that reboots Windows at once (the
// server waited the delay): shutdown.exe of the system's directory, a
// planned reason for the event log (Application: Maintenance), and no /f,
// so that a signed-in user's programs are asked to close, not killed.
func windowsShutdown(systemRoot string) (string, []string) {
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	return systemRoot + `\System32\shutdown.exe`, []string{"/r", "/t", "0", "/d", "p:4:1", "/c", "Massimo: reboot requested from the web UI"}
}

// windowsRebooter returns what reboots Windows from the service.
func windowsRebooter() func() error {
	return func() error {
		name, args := windowsShutdown(os.Getenv("SystemRoot"))
		if out, err := runCommand(name, args...); err != nil {
			return fmt.Errorf("%s: %v: %s", name, err, strings.TrimSpace(string(out)))
		}
		return nil
	}
}
