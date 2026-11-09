// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"flag"
	"io"
	"strings"
	"testing"
)

// A switch is on or off in the usual spellings, enabled and disabled
// among them; anything else is refused with the variable's name.
func TestParseSwitch(t *testing.T) {
	for v, want := range map[string]bool{"on": true, "ON": true, "true": true, "yes": true, "1": true, " on ": true, "enabled": true, "Enabled": true,
		"": false, "off": false, "false": false, "no": false, "0": false, "disabled": false, "DISABLED": false} {
		if got, err := parseSwitch(rebootVar, v); err != nil || got != want {
			t.Errorf("%q: %v %v", v, got, err)
		}
	}
	if _, err := parseSwitch(rebootVar, "maybe"); err == nil || !strings.Contains(err.Error(), `MASSIMO_REBOOT "maybe": want on or off`) {
		t.Fatalf("maybe: %v", err)
	}
}

// A switch flag takes the same values, stands alone for on, and starts
// from its variable's value; an empty variable leaves it unset.
func TestSwitchFlag(t *testing.T) {
	parse := func(variable string, args ...string) (*switchFlag, error) {
		f, err := newSwitchFlag("announce", "MASSIMO_ANNOUNCE", variable)
		if err != nil {
			return f, err
		}
		fs := flag.NewFlagSet("serve", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		fs.Var(f, "announce", "")
		return f, fs.Parse(args)
	}
	for _, c := range []struct {
		variable string
		args     []string
		on, set  bool
	}{
		{"", nil, false, false},
		{"", []string{"-announce"}, true, true},
		{"", []string{"-announce=disabled"}, false, true},
		{"enabled", nil, true, true},
		{"off", []string{"-announce=YES"}, true, true},
	} {
		f, err := parse(c.variable, c.args...)
		if err != nil || f.on != c.on || f.set != c.set {
			t.Errorf("%q %v: on %v set %v, %v", c.variable, c.args, f.on, f.set, err)
		}
	}
	if _, err := parse("", "-announce=maybe"); err == nil || !strings.Contains(err.Error(), `-announce "maybe": want on or off`) {
		t.Errorf("-announce=maybe: %v", err)
	}
	if f, err := parse("maybe"); err == nil || !strings.Contains(err.Error(), `MASSIMO_ANNOUNCE "maybe"`) || f.set {
		t.Errorf("MASSIMO_ANNOUNCE=maybe: %v", err)
	}
}
