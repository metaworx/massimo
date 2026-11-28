// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !unix && !windows

package serverbackup

import "os"

// tryLock takes no lock on a system without one; the jobs of one
// process are still one at a time (internal/server).
func tryLock(*os.File) (bool, error) { return true, nil }

// unlock releases nothing.
func unlock(*os.File) {}
