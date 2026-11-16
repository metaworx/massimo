// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build unix

package update

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// runAsOwner makes cmd run as the owner and group of the file owner, not
// followed where it is a link, when this process is root, and gives it
// its working directory: the helper runs as root, and what the service's
// user can write it leaves to that user. Anyone else runs cmd as
// itself.
func runAsOwner(cmd *exec.Cmd, owner string) error {
	if os.Geteuid() != 0 || owner == "" {
		return nil
	}
	fi, err := os.Lstat(owner)
	if err != nil {
		return err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("%s: no owner", owner)
	}
	if st.Uid == 0 {
		return nil
	}
	if err := os.Chown(cmd.Dir, int(st.Uid), int(st.Gid)); err != nil {
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: st.Uid, Gid: st.Gid}}
	return nil
}
