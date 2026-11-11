// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !linux && !darwin && !windows

package safefile

import (
	"io/fs"
	"os"
	"time"
)

// trustedLink follows no link on a platform this package does not know.
func trustedLink(fs.FileInfo) bool { return false }

// meta keeps nothing on a platform this package does not know.
type meta struct{}

func readMeta(string, fs.FileInfo) (meta, error)         { return meta{}, nil }
func (meta) apply(*os.File, string, *bool) error         { return nil }
func renameOver(tmp, path string, _ time.Duration) error { return os.Rename(tmp, path) }
func syncDir(string)                                     {}
