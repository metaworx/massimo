// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package safefile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// leftovers lists the entries of dir other than names.
func leftovers(t *testing.T, dir string, names ...string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	keep := map[string]bool{}
	for _, n := range names {
		keep[n] = true
	}
	var out []string
	for _, e := range entries {
		if !keep[e.Name()] {
			out = append(out, e.Name())
		}
	}
	return out
}

func readString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A new file and a replaced one hold what was written, a new one with
// mode 0600, and nothing else is left in the directory.
func TestWriteReplacesWhole(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings")
	for _, text := range []string{"MASSIMO_LOG=a\n", "MASSIMO_LOG=bb\n", ""} {
		if err := Write(p, []byte(text), Options{}); err != nil {
			t.Fatal(err)
		}
		if got := readString(t, p); got != text {
			t.Fatalf("content %q, want %q", got, text)
		}
		if l := leftovers(t, dir, "settings"); len(l) != 0 {
			t.Fatalf("left behind: %v", l)
		}
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v, want 0600", fi.Mode().Perm())
		}
	}
}

// Replace hands fill an empty file by name and puts what fill wrote in
// place; a fill that fails changes nothing and leaves nothing behind.
func TestReplaceFill(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "copy.db")
	os.WriteFile(p, []byte("old"), 0o600)
	if err := Replace(p, Options{}, func(tmp string) error {
		if fi, err := os.Stat(tmp); err != nil || fi.Size() != 0 {
			t.Errorf("handed %s: %v %v", tmp, fi, err)
		}
		return os.WriteFile(tmp, []byte("new"), 0o600)
	}); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, p); got != "new" {
		t.Fatalf("content %q", got)
	}
	failed := errors.New("vacuum failed")
	err := Replace(p, Options{}, func(tmp string) error {
		os.WriteFile(tmp, []byte("half"), 0o600)
		return failed
	})
	if !errors.Is(err, failed) || readString(t, p) != "new" {
		t.Fatalf("after a failed fill: %v, content %q", err, readString(t, p))
	}
	if l := leftovers(t, dir, "copy.db"); len(l) != 0 {
		t.Fatalf("left behind: %v", l)
	}
}

// Only a permission error on the directory falls back to a write in
// place; another error, a full disk, fails and changes nothing, and
// Replace never falls back.
func TestNoFallbackForOtherErrors(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings")
	os.WriteFile(p, []byte("old"), 0o600)
	full := &os.PathError{Op: "open", Path: dir, Err: syscall.ENOSPC}
	createTemp = func(string, string) (*os.File, error) { return nil, full }
	t.Cleanup(func() { createTemp = os.CreateTemp })
	if err := Write(p, []byte("new"), Options{}); !errors.Is(err, syscall.ENOSPC) || readString(t, p) != "old" {
		t.Fatalf("a full disk: %v, content %q", err, readString(t, p))
	}
	denied := &os.PathError{Op: "open", Path: dir, Err: os.ErrPermission}
	createTemp = func(string, string) (*os.File, error) { return nil, denied }
	if err := Replace(p, Options{}, func(string) error { return nil }); !errors.Is(err, os.ErrPermission) || readString(t, p) != "old" {
		t.Fatalf("Replace without a writable directory: %v, content %q", err, readString(t, p))
	}
}

// A write removes this package's temporary files beside the target that
// an interrupted write left an hour ago or more, and the writers' older
// names, but not a fresh one, which may be another write's.
func TestStaleTempRemoved(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "status.json")
	old := time.Now().Add(-2 * time.Hour)
	stale := []string{".status.json.123.tmp", ".status.json.tmp", "status.json.tmp"}
	for _, n := range append(stale, ".status.json.456.tmp") {
		os.WriteFile(filepath.Join(dir, n), nil, 0o600)
	}
	for _, n := range stale {
		os.Chtimes(filepath.Join(dir, n), old, old)
	}
	if err := Write(p, []byte("{}"), Options{}); err != nil {
		t.Fatal(err)
	}
	if l := leftovers(t, dir, "status.json"); strings.Join(l, " ") != ".status.json.456.tmp" {
		t.Fatalf("left: %v", l)
	}
}

// A link to nothing is refused, and nothing is made where it points.
func TestDanglingLinkRefused(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "env")
	if err := os.Symlink(filepath.Join(dir, "missing"), link); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	if err := Write(link, []byte("x"), Options{}); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("a link to nothing: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the link's target made: %v", err)
	}
}
