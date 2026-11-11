// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build linux || darwin

package safefile

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// The seams for the tests: a test that is not root cannot give a file
// to another user, so chown is os.File.Chown replaced, and readMetaOwner,
// nil but in a test, changes the owner readMeta read.
var (
	chown         = func(f *os.File, uid, gid int) error { return f.Chown(uid, gid) }
	readMetaOwner func(meta) meta
)

// trustedLink reports whether a link may be followed: the writing
// process's user or root owns it.
func trustedLink(fi fs.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && (int(st.Uid) == os.Geteuid() || st.Uid == 0)
}

// meta is what the replaced file carried that the new one keeps.
type meta struct {
	uid, gid int
	xattrs   map[string][]byte
}

// readMeta reads the replaced file's owner, group and extended
// attributes; an attribute that cannot be read is left out.
func readMeta(path string, fi fs.FileInfo) (meta, error) {
	m := meta{uid: -1, gid: -1}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		m.uid, m.gid = int(st.Uid), int(st.Gid)
	}
	if readMetaOwner != nil {
		m = readMetaOwner(m)
	}
	names, err := xattrNames(path)
	if err != nil {
		return m, nil // a file system without them
	}
	m.xattrs = map[string][]byte{}
	for _, n := range names {
		if v, err := xattrValue(path, n); err == nil {
			m.xattrs[n] = v
		}
	}
	return m, nil
}

// apply gives f, the new file, the owner and group as keep says, then the
// extended attributes, best effort: one the process may not set, such as
// an SELinux label, is left as the directory gives it.
func (m meta) apply(f *os.File, _ string, keep *bool) error {
	if m.uid >= 0 && (keep == nil || *keep) && !owned(f, m.uid, m.gid) {
		if err := chown(f, m.uid, m.gid); err != nil && keep != nil {
			return fmt.Errorf("keeping owner %d and group %d: %w", m.uid, m.gid, err)
		}
	}
	for n, v := range m.xattrs {
		unix.Fsetxattr(int(f.Fd()), n, v, 0)
	}
	return nil
}

// owned reports whether f already has owner uid and group gid: a new
// file takes the process's user, and its group from the process on Linux
// but from the directory on macOS.
func owned(f *os.File, uid, gid int) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == uid && int(st.Gid) == gid
}

// xattrNames lists the extended attributes of the file at path.
func xattrNames(path string) ([]string, error) {
	n, err := unix.Listxattr(path, nil)
	if err != nil || n == 0 {
		return nil, err
	}
	buf := make([]byte, n)
	if n, err = unix.Listxattr(path, buf); err != nil {
		return nil, err
	}
	var names []string
	start := 0
	for i, b := range buf[:n] {
		if b == 0 {
			if i > start {
				names = append(names, string(buf[start:i]))
			}
			start = i + 1
		}
	}
	return names, nil
}

// xattrValue reads one extended attribute of the file at path.
func xattrValue(path, name string) ([]byte, error) {
	n, err := unix.Getxattr(path, name, nil)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, n)
	if n, err = unix.Getxattr(path, name, buf); err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// renameOver renames tmp over path.
func renameOver(tmp, path string) error { return os.Rename(tmp, path) }

// syncDir syncs the directory, so that the rename survives a power cut;
// a file system that cannot is left as it is.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	d.Sync()
	d.Close()
}
