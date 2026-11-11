// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package safefile writes a file whole: to a temporary file beside it,
// synced, given what the replaced file carried, and renamed over it, so
// that a reader sees the old content or the new, never part of either,
// and a crash or a full disk leaves the old file as it was.
//
// A link at the path is followed when the writing process's user or root
// owns it, as Linux's protected_symlinks does in a sticky directory, so a
// link the owner made keeps pointing where it did; a link another user
// planted is replaced, as a rename does, so that a privileged writer
// cannot be sent elsewhere. A link to nothing is refused. On Windows a
// link is followed.
//
// The new file keeps the replaced file's mode unless Options.Mode sets
// one, its owner and group as Options.KeepOwner says, and, best effort,
// its extended attributes on Linux and macOS (a POSIX ACL is one on
// Linux; macOS keeps ACLs behind acl(3), which a build without cgo cannot
// call) or its DACL on Windows. Hard links are not kept: the path names a
// new file.
//
// Where the directory takes no new file but the file itself is writable,
// Write writes it in place, without truncating first, then truncates to
// the new length, and logs that at Warn: a reader or a crash can then see
// mixed content, but the file stays the same, its owner, ACLs and hard
// links untouched. Any other error never falls back.
//
// Reads go through nothing here: a file every writer replaces whole is
// read whole with os.ReadFile.
//
// Locks, for when two processes come to write one file (a settings
// editor in the web UI would be the first): an advisory lock on a lock
// file beside the target, never on the target itself, which a rename
// replaces with another file; flock(LOCK_EX) on Unix, which the kernel
// releases when the process dies, so that no stale lock outlives its
// holder, unlike lock files judged stale by their age; LockFileEx on
// Windows, which is mandatory, on that lock file alone, so that readers of
// the target are not refused. The lock is held across the read and the
// write of a read-modify-write, and the write is Write's. golang.org/x/sys
// has both calls.
package safefile

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options say how a file is written.
type Options struct {
	// Mode is the file's mode; 0 keeps the replaced file's, and gives a
	// new file 0600.
	Mode os.FileMode
	// KeepOwner says whether the new file gets the replaced file's owner
	// and group (on Windows its owner and group SIDs): nil tries, and
	// where that fails the process's ownership, or the directory's
	// group, is taken to be fine; true requires it, and the write fails,
	// the old file untouched; false leaves ownership alone.
	KeepOwner *bool
	// Logger takes the Warn line of a write in place; nil is
	// slog.Default().
	Logger *slog.Logger
}

// staleAge is how old a temporary file of this package's must be before
// a later write removes it: an interrupted write's leftover.
const staleAge = time.Hour

// maxLinks bounds the links followed for one path.
const maxLinks = 40

// The seams the tests replace.
var (
	createTemp = os.CreateTemp
	trustLink  = trustedLink
)

// Write replaces the file at path with data, whole (see the package
// comment).
func Write(path string, data []byte, o Options) error {
	return commit(path, o, data, func(f *os.File) (*os.File, error) {
		_, err := f.Write(data)
		return f, err
	})
}

// Replace replaces the file at path with what fill writes to the empty
// temporary file it is handed by name, as SQLite's VACUUM INTO needs a
// path; nothing changes at path when fill fails. It has no in-place
// fallback.
func Replace(path string, o Options, fill func(tmp string) error) error {
	return commit(path, o, nil, func(f *os.File) (*os.File, error) {
		name := f.Name()
		if err := f.Close(); err != nil {
			return f, err
		}
		if err := fill(name); err != nil {
			return f, err
		}
		g, err := os.OpenFile(name, os.O_RDWR, 0)
		if err != nil {
			return f, err
		}
		return g, nil
	})
}

// commit writes through a temporary file beside the real target of path:
// fill writes the temporary file it is given and returns it open, or
// another handle on it. data is what Write writes, for its in-place
// fallback, nil for Replace.
func commit(path string, o Options, data []byte, fill func(*os.File) (*os.File, error)) error {
	dst, err := target(path)
	if err != nil {
		return err
	}
	dir, base := filepath.Dir(dst), filepath.Base(dst)
	old, err := os.Lstat(dst)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		old = nil
	case err != nil:
		return fmt.Errorf("safefile: %w", err)
	case !old.Mode().IsRegular():
		old = nil // another user's link, replaced; nothing of it is kept
	}
	removeStale(dir, base)
	f, err := createTemp(dir, "."+base+".*.tmp")
	if err != nil {
		if data != nil && old != nil && errors.Is(err, fs.ErrPermission) {
			return inPlace(dst, data, o, err)
		}
		return fmt.Errorf("safefile: %w", err)
	}
	tmp := f.Name()
	done := false
	defer func() {
		if !done {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if f, err = fill(f); err != nil {
		return err
	}
	if old != nil {
		meta, err := readMeta(dst, old)
		if err != nil {
			return fmt.Errorf("safefile: %s: %w", dst, err)
		}
		if err := meta.apply(f, tmp, o.KeepOwner); err != nil {
			return fmt.Errorf("safefile: %s: %w", dst, err)
		}
	}
	mode := o.Mode
	if mode == 0 {
		mode = 0o600
		if old != nil {
			mode = old.Mode().Perm()
		}
	}
	// After the attributes: a copied POSIX ACL carries mode bits too,
	// and the mode asked for is the last word.
	if err := f.Chmod(mode); err != nil {
		return fmt.Errorf("safefile: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("safefile: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("safefile: %w", err)
	}
	if err := renameOver(tmp, dst); err != nil {
		return fmt.Errorf("safefile: %w", err)
	}
	done = true
	syncDir(dir)
	return nil
}

// target is the file a write at path replaces: path, or where the links
// at it lead that a trusted user made; a link to nothing is refused.
func target(path string) (string, error) {
	p := path
	followed := false
	for range maxLinks {
		fi, err := os.Lstat(p)
		if errors.Is(err, fs.ErrNotExist) {
			if followed {
				return "", fmt.Errorf("safefile: %s is a link to %s, which does not exist", path, p)
			}
			return p, nil
		}
		if err != nil {
			return "", fmt.Errorf("safefile: %w", err)
		}
		if fi.Mode()&fs.ModeSymlink == 0 || !trustLink(fi) {
			return p, nil
		}
		dest, err := os.Readlink(p)
		if err != nil {
			return "", fmt.Errorf("safefile: %w", err)
		}
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(filepath.Dir(p), dest)
		}
		p, followed = dest, true
	}
	return "", fmt.Errorf("safefile: %s: more than %d links", path, maxLinks)
}

// inPlace writes data over the file at dst, which exists: without
// truncating first, synced, then truncated to its length. err is why no
// temporary file could be made.
func inPlace(dst string, data []byte, o Options, err error) error {
	f, ferr := os.OpenFile(dst, os.O_WRONLY, 0)
	if ferr != nil {
		return fmt.Errorf("safefile: %w", err)
	}
	logger := o.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn("file written in place: its directory takes no new file", "path", dst, "error", err)
	if _, err := f.WriteAt(data, 0); err != nil {
		f.Close()
		return fmt.Errorf("safefile: %w", err)
	}
	if err := f.Truncate(int64(len(data))); err != nil {
		f.Close()
		return fmt.Errorf("safefile: %w", err)
	}
	if o.Mode != 0 {
		f.Chmod(o.Mode) // best effort: the file may be another user's
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("safefile: %w", err)
	}
	return f.Close()
}

// removeStale removes this package's temporary files beside base that are
// older than staleAge, and the names the writers used before it
// (".base.tmp", "base.tmp").
func removeStale(dir, base string) {
	names, _ := filepath.Glob(filepath.Join(dir, "."+escapeGlob(base)+".*.tmp"))
	names = append(names, filepath.Join(dir, "."+base+".tmp"), filepath.Join(dir, base+".tmp"))
	for _, n := range names {
		if fi, err := os.Lstat(n); err == nil && fi.Mode().IsRegular() && time.Since(fi.ModTime()) > staleAge {
			os.Remove(n)
		}
	}
}

// escapeGlob quotes the characters filepath.Match treats specially.
func escapeGlob(s string) string {
	r := strings.NewReplacer(`*`, `\*`, `?`, `\?`, `[`, `\[`)
	if filepath.Separator == '\\' {
		// Match takes no escapes on Windows; such a name is rare there.
		r = strings.NewReplacer(`*`, `?`, `[`, `?`)
	}
	return r.Replace(s)
}
