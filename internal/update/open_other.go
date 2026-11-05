// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !unix

package update

import (
	"fmt"
	"os"
)

// openNoFollow refuses a symbolic link; without O_NOFOLLOW the check
// comes before the open, which is enough where no helper runs as another
// user (the update helper is Linux only for now).
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
