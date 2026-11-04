// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// Environment variables. Each of them may carry a per-Cube variant with
// the serial number appended (MASSIMO_CUBE_KEY_KEQ0123456), which wins
// when that Cube is selected with -sn or MASSIMO_CUBE_SN.
const (
	keyEnv  = "MASSIMO_CUBE_KEY"
	ivEnv   = "MASSIMO_CUBE_IV"
	addrEnv = "MASSIMO_CUBE_ADDRESS"
	snEnv   = "MASSIMO_CUBE_SN"
)

// selectedSN is the Cube chosen with -sn (or MASSIMO_CUBE_SN).
var selectedSN string

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
			selectedSN = value
		}
	}
	return args, nil
}

// snFlag registers -sn on a command's flag set, so that both
// "massimo -sn X cube" and "massimo cube -sn X" work.
func snFlag(fs *flag.FlagSet) {
	fs.Func("sn", "serial number of the Cube to use (selects "+keyEnv+"_<SN> etc.; default "+snEnv+")", func(v string) error {
		selectedSN = v
		return nil
	})
}

// cubeEnv returns the value of name for the selected Cube: name_<SN> if
// set, else name.
func cubeEnv(name string) string {
	if selectedSN != "" {
		if v := os.Getenv(name + "_" + strings.ToUpper(selectedSN)); v != "" {
			return v
		}
	}
	return os.Getenv(name)
}

// defaultAddr fills an empty -addr from the environment.
func defaultAddr(addr *string, cmd string) error {
	if *addr == "" {
		*addr = cubeEnv(addrEnv)
	}
	if *addr == "" {
		return fmt.Errorf("%s: -addr is required (or set %s)", cmd, addrEnv)
	}
	return nil
}

// perCubeKeys lists the serial numbers that have MASSIMO_CUBE_KEY_<SN>
// set in the environment.
func perCubeKeys() []string {
	var out []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if sn, ok := strings.CutPrefix(k, keyEnv+"_"); ok && sn != "" {
			out = append(out, sn)
		}
	}
	return out
}
