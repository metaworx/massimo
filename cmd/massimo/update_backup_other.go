// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !windows

package main

import (
	"os"
	"syscall"
)

// storeOfAnother reports whether this process is root and the store's
// file, path, belongs to another user: the service's, whose files root
// does not write (update_backup.go).
func storeOfAnother(path string) bool { return ownedByAnother(path, euid()) }

// ownedByAnother reports whether uid is root's and path's owner is not;
// a file that cannot be read counts as another's.
func ownedByAnother(path string, uid int) bool {
	if uid != 0 {
		return false
	}
	fi, err := os.Stat(path)
	if err != nil {
		return true
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	return !ok || st.Uid != 0
}
