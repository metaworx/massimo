// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// FileTemplate returns the bytes of a file that is made once per test
// binary, for each test to write a copy of (WriteNew) in place of making
// it again: build makes the file name in a new temporary directory, the
// first time the returned function is called. A -wal or -shm beside the
// file once build returned (SQLite's write-ahead log and its index)
// would hold what a copy of the file alone lacks, and fails the
// template. check, where not nil, then reads the file in the directory,
// once its bytes are taken, and its error fails the template too, where
// the file is not what the copies need. The directory is removed before
// the function returns, so nothing of it is left once the binary ends;
// every later call returns the same bytes, or the same error.
//
// It is the half of a template that needs no package of the module, as
// the kit imports none: storekit's store and internal/store's own tests
// build on it.
func FileTemplate(name string, build, check func(dir string) error) func() ([]byte, error) {
	return sync.OnceValues(func() ([]byte, error) {
		dir, err := os.MkdirTemp("", "massimo-template-")
		if err != nil {
			return nil, fmt.Errorf("testkit: the template %s: %w", name, err)
		}
		defer os.RemoveAll(dir)
		if err := build(dir); err != nil {
			return nil, fmt.Errorf("testkit: the template %s: %w", name, err)
		}
		p := filepath.Join(dir, name)
		for _, side := range []string{"-wal", "-shm"} {
			if _, err := os.Stat(p + side); !errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("testkit: the template %s left %s%s behind, which its copies would lack (%v)", name, name, side, err)
			}
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("testkit: the template %s: %w", name, err)
		}
		if check != nil {
			if err := check(dir); err != nil {
				return nil, fmt.Errorf("testkit: the template %s: %w", name, err)
			}
		}
		return b, nil
	})
}

// WriteNew writes b to path, mode 0600, creating its directory (0750)
// where it is missing, as a copy of a template (FileTemplate); a file
// already at path is left as it is, and is no error.
func WriteNew(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("testkit: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("testkit: %w", err)
	}
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("testkit: %w", err)
	}
	return nil
}
