// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package safefile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// renameHeldWait is how long renameOver retries a rename over a file
// another process holds; a variable for the tests.
var renameHeldWait = 5 * time.Second

// trustedLink reports whether a link may be followed: on Windows always,
// where making one takes a privilege or developer mode.
func trustedLink(fs.FileInfo) bool { return true }

// meta is the replaced file's security descriptor: its owner, group and
// DACL.
type meta struct {
	sd        *windows.SECURITY_DESCRIPTOR
	protected bool  // the DACL inherits nothing from the directory
	err       error // why sd could not be read
}

const ownerAndGroup = windows.OWNER_SECURITY_INFORMATION | windows.GROUP_SECURITY_INFORMATION

// readMeta reads the replaced file's security descriptor.
func readMeta(path string, _ fs.FileInfo) (meta, error) {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, ownerAndGroup|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return meta{err: err}, nil
	}
	control, _, _ := sd.Control()
	return meta{sd: sd, protected: control&windows.SE_DACL_PROTECTED != 0}, nil
}

// apply gives the new file at tmp the owner and group as keep says, and,
// best effort, the DACL, protected from the directory's inheritance or
// not as the replaced file's was.
func (m meta) apply(_ *os.File, tmp string, keep *bool) error {
	if m.sd == nil {
		if keep != nil && *keep {
			return fmt.Errorf("reading the owner: %w", m.err)
		}
		return nil
	}
	dacl, _, _ := m.sd.DACL()
	info := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.UNPROTECTED_DACL_SECURITY_INFORMATION)
	if m.protected {
		info = windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION
	}
	if keep == nil || *keep {
		owner, _, _ := m.sd.Owner()
		group, _, _ := m.sd.Group()
		err := windows.SetNamedSecurityInfo(tmp, windows.SE_FILE_OBJECT, info|ownerAndGroup, owner, group, dacl, nil)
		if err == nil {
			return nil
		}
		if keep != nil {
			return fmt.Errorf("keeping owner and group: %w", err)
		}
	}
	windows.SetNamedSecurityInfo(tmp, windows.SE_FILE_OBJECT, info, nil, nil, dacl, nil)
	return nil
}

// renameOver renames tmp over path. Windows refuses that while another
// process holds path: a reader in the same moment (Go opens files without
// FILE_SHARE_DELETE), the image of a process that is still exiting, or a
// scanner's look; such a refusal is retried for renameHeldWait.
func renameOver(tmp, path string) error {
	deadline := time.Now().Add(renameHeldWait)
	for {
		err := os.Rename(tmp, path)
		if err == nil || !renameHeld(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func renameHeld(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

// syncDir does nothing: Windows has no directory to sync.
func syncDir(string) {}
