// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"slices"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
)

// Each of the start's checks reads the variables its entry names and no
// other, so that a reload runs it where one of them changed: the Cubes'
// check, whose prefix stands for every Cube line, reads the selected
// Cube through get and the lines from the environment itself.
func TestStartChecksReadTheirVariables(t *testing.T) {
	t.Parallel()
	for _, c := range startChecks {
		var read []string
		c.check(func(k string) string {
			if !slices.Contains(read, k) {
				read = append(read, k)
			}
			return ""
		})
		slices.Sort(read)
		if len(c.vars) == 1 && strings.HasSuffix(c.vars[0], "_") {
			for _, k := range read {
				if !strings.HasPrefix(k, c.vars[0]) {
					t.Errorf("the check of %s* reads %s", c.vars[0], k)
				}
			}
			continue
		}
		if want := slices.Sorted(slices.Values(c.vars)); !slices.Equal(read, want) {
			t.Errorf("the check of %v reads %v", c.vars, read)
		}
	}
}

// hideSecrets names the value of MASSIMO_TLS_KEY by the variable, as it
// is and as %q escapes it, in one pass: a value the name holds, a letter
// and a Windows path come out named once, and other values stay.
func TestHideSecrets(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ key, text, want string }{
		{"KEY", "tls: stat KEY: no such file or directory", "tls: stat MASSIMO_TLS_KEY: no such file or directory"},
		{"S", "tls: stat S: no such file or directory", "tls: stat MASSIMO_TLS_KEY: no such file or directory"},
		{`C:\ssl\key.pem`, `tls: open C:\ssl\key.pem: denied; "C:\\ssl\\key.pem"`, `tls: open MASSIMO_TLS_KEY: denied; "MASSIMO_TLS_KEY"`},
		{"/etc/ssl/key.pem", "tls: open /etc/ssl/cert.pem: denied", "tls: open /etc/ssl/cert.pem: denied"},
		{"", "tls: stat key.pem: no such file or directory", "tls: stat key.pem: no such file or directory"},
	} {
		values := map[string]string{env.TLSKey: c.key, env.TLSCert: "/etc/ssl/cert.pem"}
		if got := hideSecrets(c.text, func(k string) string { return values[k] }); got != c.want {
			t.Errorf("key %q: %q, want %q", c.key, got, c.want)
		}
	}
}
