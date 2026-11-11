// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build linux || darwin

package safefile

import (
	"bytes"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// The replaced file's mode is kept unless Options.Mode sets one.
func TestModeKeptOrSet(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings")
	os.WriteFile(p, []byte("a"), 0o600)
	os.Chmod(p, 0o640)
	if err := Write(p, []byte("b"), Options{}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v, want the kept 0640", fi.Mode().Perm())
	}
	if err := Write(p, []byte("c"), Options{Mode: 0o600}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, want the set 0600", fi.Mode().Perm())
	}
}

// KeepOwner: unset tries and takes a failure for fine, true requires it,
// false leaves ownership alone. A test that is not root cannot give a
// file to another user, so chown is the seam, and the owner another one.
func TestOwnerTriState(t *testing.T) {
	var calls int
	refused := &os.PathError{Op: "chown", Err: syscall.EPERM}
	chown = func(*os.File, int, int) error { calls++; return refused }
	t.Cleanup(func() { chown = func(f *os.File, uid, gid int) error { return f.Chown(uid, gid) } })
	f, err := os.CreateTemp(t.TempDir(), "new")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	other := meta{uid: os.Geteuid() + 1, gid: os.Getegid() + 1}
	yes, no := true, false
	for _, c := range []struct {
		name    string
		keep    *bool
		calls   int
		failing bool
	}{{"unset", nil, 1, false}, {"required", &yes, 1, true}, {"left alone", &no, 0, false}} {
		calls = 0
		err := other.apply(f, f.Name(), c.keep)
		if calls != c.calls || (err != nil) != c.failing {
			t.Errorf("%s: %d chown calls, error %v", c.name, calls, err)
		}
		if c.failing && !errors.Is(err, syscall.EPERM) {
			t.Errorf("%s: error %v, want the refusal", c.name, err)
		}
	}
	// The process's own file is not chowned at all.
	calls = 0
	fi, _ := f.Stat()
	st := fi.Sys().(*syscall.Stat_t)
	if err := (meta{uid: int(st.Uid), gid: int(st.Gid)}).apply(f, f.Name(), &yes); err != nil || calls != 0 {
		t.Errorf("own file: %d chown calls, error %v", calls, err)
	}
	// Required and refused, a write fails and leaves the old file.
	p := filepath.Join(t.TempDir(), "env")
	os.WriteFile(p, []byte("old"), 0o600)
	readMetaOwner = func(m meta) meta { m.uid, m.gid = os.Geteuid()+1, os.Getegid()+1; return m }
	t.Cleanup(func() { readMetaOwner = nil })
	if err := Write(p, []byte("new"), Options{KeepOwner: &yes}); !errors.Is(err, syscall.EPERM) || readString(t, p) != "old" {
		t.Fatalf("required owner refused: %v, content %q", err, readString(t, p))
	}
	if err := Write(p, []byte("new"), Options{}); err != nil || readString(t, p) != "new" {
		t.Fatalf("owner tried: %v, content %q", err, readString(t, p))
	}
}

// A link the process's user made is followed, the link kept and its
// target replaced; one another user made is replaced itself, its target
// untouched.
func TestLinkFollowedOrReplaced(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real-env")
	link := filepath.Join(dir, "env")
	os.WriteFile(real, []byte("old"), 0o600)
	if err := os.Symlink("real-env", link); err != nil {
		t.Fatal(err)
	}
	if err := Write(link, []byte("through"), Options{}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&fs.ModeSymlink == 0 || readString(t, real) != "through" {
		t.Fatalf("own link: link %v, target %q", fi.Mode(), readString(t, real))
	}
	trustLink = func(fs.FileInfo) bool { return false }
	t.Cleanup(func() { trustLink = trustedLink })
	if err := Write(link, []byte("replaced"), Options{}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); !fi.Mode().IsRegular() || readString(t, link) != "replaced" || readString(t, real) != "through" {
		t.Fatalf("another's link: link %v %q, target %q", fi.Mode(), readString(t, link), readString(t, real))
	}
}

// The replaced file's extended attributes are kept, a POSIX ACL among
// them on Linux.
func TestExtendedAttributesKept(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings")
	os.WriteFile(p, []byte("a"), 0o600)
	name := "user.massimo.test"
	if runtime.GOOS == "darwin" {
		name = "ch.metaworx.massimo.test"
	}
	if err := unix.Setxattr(p, name, []byte("kept"), 0); err != nil {
		t.Skipf("no extended attributes here: %v", err)
	}
	if err := Write(p, []byte("b"), Options{}); err != nil {
		t.Fatal(err)
	}
	v, err := xattrValue(p, name)
	if err != nil || string(v) != "kept" {
		t.Fatalf("attribute after the write: %q %v", v, err)
	}
}

// Where the directory takes no new file but the file is writable, it is
// written in place and that is logged; it stays the same file.
func TestInPlaceFallback(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes any directory")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "settings")
	os.WriteFile(p, []byte("a longer old content\n"), 0o600)
	before, _ := os.Stat(p)
	os.Chmod(dir, 0o500)
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	var log bytes.Buffer
	if err := Write(p, []byte("new\n"), Options{Logger: slog.New(slog.NewTextHandler(&log, nil))}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(p)
	if got := readString(t, p); got != "new\n" || !os.SameFile(before, after) {
		t.Fatalf("content %q, same file %v", got, os.SameFile(before, after))
	}
	if !strings.Contains(log.String(), "level=WARN") || !strings.Contains(log.String(), "written in place") {
		t.Fatalf("log: %s", log.String())
	}
}

// As root, a replaced file another user owns keeps its owner and group,
// and a link that user planted is not followed.
func TestOwnerKeptAsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("only root gives a file to another user")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "env")
	os.WriteFile(p, []byte("old"), 0o600)
	os.Chown(p, 4321, 4321)
	if err := Write(p, []byte("new"), Options{KeepOwner: new(bool)}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Sys().(*syscall.Stat_t).Uid != 0 {
		t.Fatalf("KeepOwner false: owner %d, want root's", fi.Sys().(*syscall.Stat_t).Uid)
	}
	os.Chown(p, 4321, 4321)
	if err := Write(p, []byte("newer"), Options{}); err != nil {
		t.Fatal(err)
	}
	if st := mustStat(t, p); st.Uid != 4321 || st.Gid != 4321 || readString(t, p) != "newer" {
		t.Fatalf("owner %d group %d, content %q", st.Uid, st.Gid, readString(t, p))
	}
	secret := filepath.Join(dir, "secret")
	os.WriteFile(secret, []byte("root's"), 0o600)
	link := filepath.Join(dir, "planted")
	os.Symlink(secret, link)
	os.Lchown(link, 4321, 4321)
	if err := Write(link, []byte("redirected?"), Options{}); err != nil {
		t.Fatal(err)
	}
	if readString(t, secret) != "root's" {
		t.Fatalf("a link another user planted was followed: %q", readString(t, secret))
	}
}

func mustStat(t *testing.T, p string) *syscall.Stat_t {
	t.Helper()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Sys().(*syscall.Stat_t)
}
