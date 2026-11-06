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
	noHelper := fs.Bool("no-update-helper", false, "leave out the update helper (Linux and macOS: its units or LaunchDaemon; Windows: the service's -update-request); the web UI then cannot start updates")
	noStart := fs.Bool("no-start", false, "enable the service without starting it")
	listen := fs.String("listen", "", "serve the API and web UI on this address, such as :80 (default :8099; kept by later installs)")
	dryRun := fs.Bool("dry-run", false, "only show what would be done")
	remove := fs.Bool("remove", false, "stop and remove the service and the binary")
	purge := fs.Bool("purge", false, "with -remove: also remove the settings and the logs (Linux and macOS: the state and the user too)")
	root := fs.String("root", "", "Linux and macOS: install below this directory instead of / and run no commands (for images and tests)")
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
  domain networks; the server starts the update helper itself when the
  web UI asks for an update. Needs an administrator prompt.
- macOS: the binary as %s, the hidden user %s, %s, the
  LaunchDaemons %s and the update helper %s in
  %s, loaded with launchctl. Needs root.

See packaging/README.md.

`, install.BinPath, install.EnvPath, win.Bin, win.DataDir,
			install.BinPath, install.DarwinUser, install.EnvPath, install.Label, install.HelperLabel, install.DaemonDir)
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
		if *root != "" {
			return errors.New("-root is for Linux and macOS")
		}
		if *remove {
			return install.RemoveWindows(o, *purge)
		}
		return install.InstallWindows(o)
	case "darwin":
		if *remove {
			return install.RemoveDarwin(o, *purge)
		}
		return install.InstallDarwin(o)
	default:
		return fmt.Errorf("install does not support %s", runtime.GOOS)
	}
}
