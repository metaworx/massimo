// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// selectedSN is the Cube chosen with -sn (or MASSIMO_CUBE_SN), or the
// serial reported by the Cube that -addr pointed to; the per-Cube
// variables of internal/env are read for it. snExplicit says whether it
// came from -sn.
var (
	selectedSN string
	snExplicit bool
	// showHidden (-show-hidden; -no-ignore and -show-ghosts are older
	// names) shows the hidden devices for this run, so a backup or
	// restore can touch them.
	showHidden bool
)

// useDeviceSerial switches to the Cube that answered on the LAN. When a
// different Cube was selected with -sn, the device wins and a warning is
// printed.
func useDeviceSerial(serial string) {
	if serial == "" {
		return
	}
	if snExplicit && !strings.EqualFold(selectedSN, serial) {
		fmt.Fprintf(os.Stderr, "warning: -sn %s given, but the cube at that address is %s; using %s\n", selectedSN, serial, serial)
	}
	selectedSN = serial
}

// globalOptions parses options that precede the command name:
// -env <file>, -sn <serial> and -show-hidden. It returns the remaining
// arguments.
func globalOptions(args []string) ([]string, error) {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		name, value, hasValue := strings.Cut(strings.TrimLeft(args[0], "-"), "=")
		switch name {
		case "env", "sn":
		case "show-hidden", "no-ignore", "show-ghosts":
			showHidden = true
			args = args[1:]
			continue
		default:
			return args, nil // a command alias such as --version
		}
		args = args[1:]
		if !hasValue {
			if len(args) == 0 {
				return nil, fmt.Errorf("-%s needs a value", name)
			}
			value, args = args[0], args[1:]
		}
		switch name {
		case "env":
			os.Setenv(env.FileVar, value)
		case "sn":
			selectedSN, snExplicit = value, true
		}
	}
	return args, nil
}

// snFlag registers -sn on a command's flag set, so that both
// "massimo -sn X cube" and "massimo cube -sn X" work.
func snFlag(fs *flag.FlagSet) {
	fs.Func("sn", "serial number of the Cube to use (selects "+env.Prefix+"<SN>_KEY etc.; default "+env.SN+")", func(v string) error {
		selectedSN, snExplicit = v, true
		return nil
	})
}

// cubeEnv returns the value of a Cube variable for the selected Cube.
func cubeEnv(name string) string { return env.Get(name, selectedSN) }

// defaultAddr fills an empty -addr for the selected Cube, see
// env.ResolveAddr.
func defaultAddr(addr *string, cmd string) error {
	if *addr != "" {
		return nil
	}
	a, err := env.ResolveAddr(cmd, selectedSN, os.Stderr)
	if err != nil {
		return err
	}
	*addr = a
	return nil
}

// hiddenDevices returns the hidden list of the selected Cube, or nothing
// with -show-hidden.
func hiddenDevices() ([]cube.RFAddress, error) {
	if showHidden {
		return nil, nil
	}
	return env.HiddenList(selectedSN)
}

// hiddenSet is the set of devices to leave out for the selected Cube.
func hiddenSet(snap *cubelan.Snapshot) (view.Hidden, error) {
	return env.HiddenSet(selectedSN, snap.Metadata(), showHidden)
}
