// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// version is set at build time from the git tag, e.g.
//
//	go build -ldflags "-X main.version=v0.1.0" ./cmd/massimo
//
// The Makefile does this with `git describe --tags`. Without it the version
// is derived from the VCS information Go records in the binary.
var version = ""

func versionString() string {
	v := version
	if v == "" {
		v = "v0.0.0-dev"
		if bi, ok := debug.ReadBuildInfo(); ok {
			var rev, modified string
			for _, s := range bi.Settings {
				switch s.Key {
				case "vcs.revision":
					rev = s.Value
				case "vcs.modified":
					if s.Value == "true" {
						modified = "-dirty"
					}
				}
			}
			if rev != "" {
				v += "+" + rev[:min(12, len(rev))] + modified
			}
		}
	}
	return fmt.Sprintf("massimo %s (%s, %s/%s)", strings.TrimSpace(v), runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
