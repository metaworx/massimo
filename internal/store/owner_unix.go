// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build unix

package store

import (
	"os"
	"syscall"
)

// giveOwner gives path the owner and group of ref where this process is
// root: a copy the update helper makes in the service's data directory
// stays the service user's, as SQLite gives its journal files the
// database's owner.
func giveOwner(path, ref string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	fi, err := os.Stat(ref)
	if err != nil {
		return err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return os.Chown(path, int(st.Uid), int(st.Gid))
}
