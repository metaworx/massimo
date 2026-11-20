// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A template is made once, checked where its bytes were taken, and its
// directory removed; every call returns the same bytes.
func TestFileTemplate(t *testing.T) {
	var dirs []string
	checked := ""
	template := FileTemplate("t.db", func(dir string) error {
		dirs = append(dirs, dir)
		return os.WriteFile(filepath.Join(dir, "t.db"), []byte("made"), 0o600)
	}, func(dir string) error {
		b, err := os.ReadFile(filepath.Join(dir, "t.db"))
		checked = string(b)
		return err
	})
	for range 2 {
		if b, err := template(); err != nil || string(b) != "made" {
			t.Fatalf("%q %v", b, err)
		}
	}
	if len(dirs) != 1 || checked != "made" {
		t.Fatalf("made %d times, checked %q", len(dirs), checked)
	}
	if _, err := os.Stat(dirs[0]); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the template's directory is left: %v", err)
	}
}

// A build that leaves a -wal or a -shm beside the file, one that fails,
// and a check that fails each fail the template.
func TestFileTemplateRefused(t *testing.T) {
	write := func(names ...string) func(string) error {
		return func(dir string) error {
			for _, n := range names {
				if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
					return err
				}
			}
			return nil
		}
	}
	for _, c := range []struct {
		name         string
		build, check func(string) error
		want         string
	}{
		{"a -wal left", write("t.db", "t.db-wal"), nil, "left t.db-wal behind"},
		{"a -shm left", write("t.db", "t.db-shm"), nil, "left t.db-shm behind"},
		{"no file", write(), nil, "t.db"},
		{"the build failed", func(string) error { return errors.New("no store") }, nil, "the template t.db: no store"},
		{"the check failed", write("t.db"), func(string) error { return errors.New("not migrated") }, "the template t.db: not migrated"},
	} {
		if _, err := FileTemplate("t.db", c.build, c.check)(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
	}
}

// WriteNew makes the file and its directory, mode 0600, and leaves a
// file already there as it is.
func TestWriteNew(t *testing.T) {
	p := filepath.Join(t.TempDir(), "data", "t.db")
	if err := WriteNew(p, []byte("copy")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600) {
		t.Fatalf("%v %v", fi, err)
	}
	if err := WriteNew(p, []byte("another")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "copy" {
		t.Fatalf("a file there became %q", b)
	}
}
