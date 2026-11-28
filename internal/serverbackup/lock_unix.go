// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build unix

package serverbackup

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// tryLock takes f's exclusive flock without waiting; false where another
// open file holds it.
func tryLock(f *os.File) (bool, error) {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}

// unlock releases f's flock.
func unlock(f *os.File) { unix.Flock(int(f.Fd()), unix.LOCK_UN) }
