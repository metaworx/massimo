// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The lock of the catalogue's jobs, shared by serve and massimo store:
// one job at a time on a data directory, so that a command run beside a
// serve it did not reach never makes, copies or prunes while serve's
// schedule does. An advisory lock on a lock file in the data directory,
// as internal/safefile describes its locks: flock on Unix, which the
// kernel releases when the process dies, LockFileEx on Windows.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// JobLockName is the lock file of the jobs in the data directory.
const JobLockName = ".serverbackup.lock"

// ErrJobRunning is a lock that another job holds.
var ErrJobRunning = errors.New("serverbackup: another job of the server's backups runs on this data directory (serve's schedule, or a command)")

// lockPoll is how often LockJobs tries again while the lock is held.
const lockPoll = 100 * time.Millisecond

// LockJobs takes the lock of the jobs in the data directory dir,
// waiting while another holds it until ctx ends, then ErrJobRunning; the
// function returned releases it.
func LockJobs(ctx context.Context, dir string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(dir, JobLockName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("serverbackup: the jobs' lock: %w", err)
	}
	for {
		held, err := tryLock(f)
		switch {
		case err != nil:
			f.Close()
			return nil, fmt.Errorf("serverbackup: the jobs' lock: %w", err)
		case held:
			return func() { unlock(f); f.Close() }, nil
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, fmt.Errorf("%w: %w", ErrJobRunning, ctx.Err())
		case <-time.After(lockPoll):
		}
	}
}
