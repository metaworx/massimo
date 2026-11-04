// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// version is set at build time by the Makefile from
// `git describe --tags --always --dirty`, which yields exactly the tag when
// HEAD is tagged (v0.1.0), the tag plus distance and commit hash otherwise
// (v0.1.0-3-g1a2b3c4), and a -dirty suffix when the working tree has
// uncommitted changes. Without the Makefile, the version is derived from
// the VCS information Go records in the binary (hash and modified flag).
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
