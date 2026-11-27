// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build unix

package serverbackup

import "golang.org/x/sys/unix"

// diskFree returns the bytes free for an unprivileged writer on the file
// system of dir.
func diskFree(dir string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
