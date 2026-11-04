// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Environment variables. Each of them has a per-Cube variant with the
// serial number inserted (MASSIMO_CUBE_KEQ0123456_KEY), which wins when
// that Cube is selected with -sn, MASSIMO_CUBE_SN, or by the serial the
// Cube reports when -addr is given.
const (
	envPrefix = "MASSIMO_CUBE_"
	keyEnv    = envPrefix + "KEY"
	ivEnv     = envPrefix + "IV"
	addrEnv   = envPrefix + "ADDRESS"
	snEnv     = envPrefix + "SN"
	ignoreEnv = envPrefix + "IGNORE"
)

// selectedSN is the Cube chosen with -sn (or MASSIMO_CUBE_SN), or the
// serial reported by the Cube that -addr pointed to. snExplicit says
// whether it came from -sn.
var (
	selectedSN string
	snExplicit bool
)

// perCubeVar returns the per-Cube name of a variable: MASSIMO_CUBE_KEY
// becomes MASSIMO_CUBE_<SN>_KEY.
func perCubeVar(name, sn string) string {
	return envPrefix + strings.ToUpper(sn) + "_" + strings.TrimPrefix(name, envPrefix)
}

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
// -env <file> and -sn <serial>. It returns the remaining arguments.
func globalOptions(args []string) ([]string, error) {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		name, value, hasValue := strings.Cut(strings.TrimLeft(args[0], "-"), "=")
		switch name {
		case "env", "sn":
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
			os.Setenv("MASSIMO_ENV_FILE", value)
		case "sn":
			selectedSN, snExplicit = value, true
		}
	}
	return args, nil
}

// snFlag registers -sn on a command's flag set, so that both
// "massimo -sn X cube" and "massimo cube -sn X" work.
func snFlag(fs *flag.FlagSet) {
	fs.Func("sn", "serial number of the Cube to use (selects "+envPrefix+"<SN>_KEY etc.; default "+snEnv+")", func(v string) error {
		selectedSN, snExplicit = v, true
		return nil
	})
}

// cubeEnv returns the value of name for the selected Cube:
// MASSIMO_CUBE_<SN>_<name> if set, else the plain name.
func cubeEnv(name string) string {
	if selectedSN != "" {
		if v := os.Getenv(perCubeVar(name, selectedSN)); v != "" {
			return v
		}
	}
	return os.Getenv(name)
}

// defaultAddr fills an empty -addr from the environment, or, when a
// serial is selected, by discovering the Cube on the LAN.
func defaultAddr(addr *string, cmd string) error {
	if *addr == "" {
		*addr = cubeEnv(addrEnv)
	}
	if *addr == "" && selectedSN != "" {
		fmt.Fprintf(os.Stderr, "looking for cube %s on the LAN ...\n", selectedSN)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cubes, err := cubelan.Discover(ctx, 3*time.Second)
		if err != nil {
			return err
		}
		for _, c := range cubes {
			if strings.EqualFold(c.Serial, selectedSN) {
				*addr = c.IP.String()
				fmt.Fprintf(os.Stderr, "cube %s found at %s\n", c.Serial, *addr)
			}
		}
		if *addr == "" {
			return fmt.Errorf("%s: cube %s did not answer the discovery broadcast (set %s, or -addr; from WSL 2 broadcasts do not reach the LAN)", cmd, selectedSN, perCubeVar(addrEnv, selectedSN))
		}
	}
	if *addr == "" {
		return fmt.Errorf("%s: -addr is required (or set %s, or -sn)", cmd, addrEnv)
	}
	return nil
}

// ignoredDevices returns the devices listed in MASSIMO_CUBE_<SN>_IGNORE
// (or MASSIMO_CUBE_IGNORE) for the selected Cube: devices that are
// paired but never report, parked in a room of their own on the Cube
// and hidden from every view. See docs/portal-protocol.md.
func ignoredDevices() ([]cube.RFAddress, error) {
	name := ignoreEnv
	if selectedSN != "" && os.Getenv(perCubeVar(ignoreEnv, selectedSN)) != "" {
		name = perCubeVar(ignoreEnv, selectedSN)
	}
	return parseIgnoreList(name, os.Getenv(name))
}

// parseIgnoreList reads a comma or space separated list of radio
// addresses; name is the variable, for the error message.
func parseIgnoreList(name, value string) ([]cube.RFAddress, error) {
	var out []cube.RFAddress
	for _, f := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == ';' }) {
		rf, err := cube.ParseRFAddress(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, rf)
	}
	return out, nil
}

// perCubeKeys lists the serial numbers that have MASSIMO_CUBE_<SN>_KEY
// set in the environment.
func perCubeKeys() []string {
	var out []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		mid, ok := strings.CutPrefix(k, envPrefix)
		if !ok {
			continue
		}
		if sn, ok := strings.CutSuffix(mid, "_KEY"); ok && sn != "" {
			out = append(out, sn)
		}
	}
	return out
}
