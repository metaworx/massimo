// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The local target: a directory of this box, by default backups/ in the
// data directory. A copy there does not survive the disk it is on; the
// card says so (AP ServerBackup, the analysis's section 3.3).

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// KindLocal is the local target's kind, and DefaultLocalDir its
// directory where none is given, relative to the data directory.
const (
	KindLocal       = "local"
	DefaultLocalDir = "backups"
)

// partSuffix ends the temporary name of a put, beside the final one and
// hidden: ".<name>.<random>.part". One a crash left is removed by the
// next put after a day.
const (
	partSuffix = ".part"
	partStale  = 24 * time.Hour
)

func init() {
	Register(Kind{
		Name:   KindLocal,
		Fields: []Field{{Key: "path", Type: FieldPath, Default: DefaultLocalDir}},
		New: func(cfg map[string]string, _ []byte) (Target, error) {
			return NewLocal(cfg["path"])
		},
	})
}

// Local is a target in a directory of this box.
type Local struct {
	dir string
}

// NewLocal returns the local target of dir, which must be absolute; it is
// made, mode 0700, at the first put or check.
func NewLocal(dir string) (*Local, error) {
	if dir == "" || !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("serverbackup: the local target's directory %q is not absolute", dir)
	}
	return &Local{dir: filepath.Clean(dir)}, nil
}

// Dir is the target's directory.
func (l *Local) Dir() string { return l.dir }

// Put writes r, size bytes, to name: to a hidden temporary file beside
// it, mode 0600, synced, then renamed to name, so that a put broken off
// leaves nothing under name. Fewer or more bytes than size are refused.
func (l *Local) Put(ctx context.Context, name string, r io.Reader, size int64) error {
	if err := CheckName(name); err != nil {
		return err
	}
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return fmt.Errorf("serverbackup: local target: %w", err)
	}
	l.removeStaleParts(time.Now())
	f, err := os.CreateTemp(l.dir, "."+name+".*"+partSuffix)
	if err != nil {
		return fmt.Errorf("serverbackup: local target: %w", err)
	}
	tmp := f.Name()
	done := false
	defer func() {
		if !done {
			f.Close()
			os.Remove(tmp)
		}
	}()
	n, err := io.Copy(f, io.LimitReader(ctxReader{ctx, r}, size+1))
	switch {
	case err != nil:
		return fmt.Errorf("serverbackup: local target: %s: %w", name, err)
	case n != size:
		return fmt.Errorf("serverbackup: local target: %s: %d bytes, not %d", name, n, size)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("serverbackup: local target: %s: %w", name, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("serverbackup: local target: %s: %w", name, err)
	}
	if err := os.Rename(tmp, filepath.Join(l.dir, name)); err != nil {
		return fmt.Errorf("serverbackup: local target: %w", err)
	}
	done = true
	return nil
}

// removeStaleParts removes the temporary files of puts a crash left,
// older than a day.
func (l *Local) removeStaleParts(now time.Time) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") || !strings.HasSuffix(e.Name(), partSuffix) {
			continue
		}
		if fi, err := e.Info(); err == nil && fi.Mode().IsRegular() && now.Sub(fi.ModTime()) > partStale {
			os.Remove(filepath.Join(l.dir, e.Name()))
		}
	}
}

// Open opens name for ranged reads.
func (l *Local) Open(_ context.Context, name string) (File, int64, error) {
	if err := CheckName(name); err != nil {
		return nil, 0, err
	}
	f, err := os.Open(filepath.Join(l.dir, name))
	if err != nil {
		return nil, 0, fmt.Errorf("serverbackup: local target: %w", err)
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		f.Close()
		return nil, 0, fmt.Errorf("serverbackup: local target: %s is no regular file", name)
	}
	return f, fi.Size(), nil
}

// List lists the backups of the directory: its regular files whose
// names a backup may have; a directory not made yet holds none.
func (l *Local) List(_ context.Context) ([]Entry, error) {
	entries, err := os.ReadDir(l.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("serverbackup: local target: %w", err)
	}
	out := []Entry{}
	for _, e := range entries {
		if CheckName(e.Name()) != nil {
			continue
		}
		fi, err := e.Info()
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		out = append(out, Entry{Name: e.Name(), Size: fi.Size(), Time: fi.ModTime()})
	}
	return out, nil
}

// Delete removes name; one not there is no error.
func (l *Local) Delete(_ context.Context, name string) error {
	if err := CheckName(name); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(l.dir, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("serverbackup: local target: %w", err)
	}
	return nil
}

// Check makes the directory where it is missing and writes and removes
// a file in it.
func (l *Local) Check(_ context.Context) error {
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return fmt.Errorf("serverbackup: local target: %w", err)
	}
	f, err := os.CreateTemp(l.dir, ".check-*"+partSuffix)
	if err != nil {
		return fmt.Errorf("serverbackup: local target: %s is not writable: %w", l.dir, err)
	}
	_, werr := f.WriteString("massimo\n")
	cerr := f.Close()
	os.Remove(f.Name())
	if err := errors.Join(werr, cerr); err != nil {
		return fmt.Errorf("serverbackup: local target: %s: %w", l.dir, err)
	}
	return nil
}

// ctxReader stops a read once its context is done.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

// Read reads from r while the context lasts.
func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
