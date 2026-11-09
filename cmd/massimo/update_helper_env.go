// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"strings"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/update"
)

// helperEnv is the server's environment for a helper it starts itself
// (Windows, Homebrew), without the MASSIMO_ variables, which hold the
// settings file's Cube keys and API token the helper does not need,
// except the GitHub token and the test-only API base, and with a
// settings file that does not exist.
func helperEnv(environ []string) []string {
	var out []string
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		switch {
		case strings.EqualFold(name, update.TokenVar), strings.EqualFold(name, update.APIVar):
		case len(name) >= 8 && strings.EqualFold(name[:8], "MASSIMO_"):
			continue
		}
		out = append(out, kv)
	}
	return append(out, env.FileVar+"="+noEnvFile)
}
