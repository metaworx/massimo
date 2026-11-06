// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !unix

package update

import (
	"fmt"
	"os"
)

// openNoFollow refuses a symbolic link; without O_NOFOLLOW the check
// comes before the open, which is enough where the helper runs as the
// same user as the server (on Windows both run as LocalSystem, and the
// request file lies in a folder only SYSTEM and the Administrators can
// write).
func openNoFollow(path string) (*os.File, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s is a symbolic link", path)
	}
	return os.Open(path)
}
