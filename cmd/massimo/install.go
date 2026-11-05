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
	settings := fs.String("settings", "", "copy this settings file (a .env) to "+install.EnvPath+" when there is none yet")
	noHelper := fs.Bool("no-update-helper", false, "leave out the update helper units (the web UI then cannot start updates)")
	noStart := fs.Bool("no-start", false, "enable the service without starting it")
	dryRun := fs.Bool("dry-run", false, "only show what would be done")
	remove := fs.Bool("remove", false, "stop and remove the service, its units and the binary")
	purge := fs.Bool("purge", false, "with -remove: also remove the settings, the state, the logs and the user")
	root := fs.String("root", "", "install below this directory instead of / and run no commands (for images and tests)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `usage: massimo install [-settings <.env>] [-no-update-helper] [-no-start] [-dry-run]
       massimo install -remove [-purge]

Install this binary as %s and massimo serve as a service: on Linux with
systemd the user massimo, %s, the units massimo.service and the update
helper massimo-update.path/.service, and the log rotation, as
packaging/README.md describes. Run it again to update the binary and the
units; a unit edited by hand is kept as .bak. Needs root.

`, install.BinPath, install.EnvPath)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "linux":
	case "darwin":
		return errors.New("install does not support macOS yet (it would write a LaunchDaemon); run massimo serve by hand or under launchd yourself")
	case "windows":
		return errors.New("install does not support Windows yet (it needs massimo to run as a Windows service); run massimo serve by hand")
	default:
		return fmt.Errorf("install does not support %s", runtime.GOOS)
	}
	o := install.Options{Root: *root, Settings: *settings, NoUpdateHelper: *noHelper, NoStart: *noStart, DryRun: *dryRun}
	if *remove {
		return install.Remove(o, *purge)
	}
	if *purge {
		return errors.New("-purge goes with -remove")
	}
	return install.Install(o)
}
