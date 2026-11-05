// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build unix

package update

import (
	"os"
	"syscall"
)

// openNoFollow opens path for reading unless it is a symbolic link; with
// O_NONBLOCK a FIFO planted in its place does not block the open.
func openNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
