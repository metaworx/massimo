// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"

	"go.metaworx.ch/massimo/internal/install"
)

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	settings := fs.String("settings", "", "copy this settings file (a .env) as the service's settings when there are none yet")
	noHelper := fs.Bool("no-update-helper", false, "Linux: leave out the update helper units (the web UI then cannot start updates)")
	noStart := fs.Bool("no-start", false, "enable the service without starting it")
	listen := fs.String("listen", "", "serve the API and web UI on this address, such as :80 (default :8099; kept by later installs)")
	dryRun := fs.Bool("dry-run", false, "only show what would be done")
	remove := fs.Bool("remove", false, "stop and remove the service and the binary")
	purge := fs.Bool("purge", false, "with -remove: also remove the settings and the logs (Linux: the state and the user too)")
	root := fs.String("root", "", "Linux: install below this directory instead of / and run no commands (for images and tests)")
	fs.Usage = func() {
		win := install.WindowsPaths("", "")
		fmt.Fprintf(os.Stderr, `usage: massimo install [-settings <.env>] [-listen <addr>] [-no-update-helper] [-no-start] [-dry-run]
       massimo install -remove [-purge]

Install this binary and massimo serve as a service, or update both when
run again (existing settings are kept):

- Linux with systemd: the binary as %s, the user massimo, %s,
  the units massimo.service and the update helper
  massimo-update.path/.service, and the log rotation; a unit edited by
  hand is kept as .bak. Needs root.
- Windows: the binary as %s, settings and log in %s
  (SYSTEM and Administrators only), the service "massimo" (automatic
  start, restarted on failure) and a firewall rule for private and
  domain networks. Needs an administrator prompt.

See packaging/README.md.

`, install.BinPath, install.EnvPath, win.Bin, win.DataDir)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *purge && !*remove {
		return errors.New("-purge goes with -remove")
	}
	o := install.Options{Root: *root, Settings: *settings, NoUpdateHelper: *noHelper, NoStart: *noStart, DryRun: *dryRun, Listen: *listen}
	switch runtime.GOOS {
	case "linux":
		if *remove {
			return install.Remove(o, *purge)
		}
		return install.Install(o)
	case "windows":
		if *root != "" || *noHelper {
			return errors.New("-root and -no-update-helper are for Linux")
		}
		if *remove {
			return install.RemoveWindows(o, *purge)
		}
		return install.InstallWindows(o)
	case "darwin":
		// It would take a LaunchDaemon in /Library/LaunchDaemons loaded
		// with launchctl bootstrap, a service user (dscl) or root, and an
		// update helper; not before there is a Mac to test on.
		return errors.New("install does not support macOS yet; run massimo serve by hand or under launchd yourself")
	default:
		return fmt.Errorf("install does not support %s", runtime.GOOS)
	}
}
