// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"flag"
	"fmt"
	"strings"
)

// parseSwitch reads an on/off setting, a variable's or a flag's: on,
// true, yes, 1 or enabled, and off, false, no, 0, disabled or nothing, in
// any case.
func parseSwitch(name, v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on", "true", "yes", "1", "enabled":
		return true, nil
	case "", "off", "false", "no", "0", "disabled":
		return false, nil
	}
	return false, fmt.Errorf("%s %q: want on or off", name, v)
}

// switchHelp is how a switch's help says what it takes: the flag alone
// is on, and a value follows its "=", since "-name off" would end the
// flags at "off".
func switchHelp(name string) string {
	return "on or off: also true or false, yes or no, 1 or 0, enabled or disabled, in any case; -" + name + " alone is on, a value goes after =, as -" + name + "=off"
}

// strayHint is what an argument flag parsing stopped at needs, where it
// is a switch's value given after a space: "-announce off" sets the
// switch on and leaves "off", and every flag after it, unread.
func strayHint(fs *flag.FlagSet, args []string, stray string) string {
	if _, err := parseSwitch("", stray); err != nil || strings.TrimSpace(stray) == "" {
		return ""
	}
	for i := 1; i < len(args); i++ {
		if args[i] != stray {
			continue
		}
		name := strings.TrimLeft(args[i-1], "-")
		if f := fs.Lookup(name); f != nil {
			if _, ok := f.Value.(*switchFlag); ok {
				return fmt.Sprintf(": write -%s=%s, a switch takes its value after =", name, stray)
			}
		}
	}
	return ""
}

// switchFlag is an on/off flag read by parseSwitch; given alone, as
// -name, it is on. set says whether the flag or its variable set it.
type switchFlag struct {
	name    string
	on, set bool
}

// newSwitchFlag is the flag name with its variable's value as the
// default: nothing leaves it unset, a value it does not take is
// returned as the error, for serve to refuse unless the flag is given.
func newSwitchFlag(name, variable string, value string) (*switchFlag, error) {
	f := &switchFlag{name: name}
	if strings.TrimSpace(value) == "" {
		return f, nil
	}
	on, err := parseSwitch(variable, value)
	if err != nil {
		return f, err
	}
	f.on, f.set = on, true
	return f, nil
}

func (f *switchFlag) String() string {
	if f != nil && f.on {
		return "on"
	}
	return "off"
}

func (f *switchFlag) Set(v string) error {
	on, err := parseSwitch("-"+f.name, v)
	if err != nil {
		return err
	}
	f.on, f.set = on, true
	return nil
}

// IsBoolFlag lets -name stand alone for on.
func (f *switchFlag) IsBoolFlag() bool { return true }
