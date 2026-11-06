// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package update

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PrevPath is where the binary before the last update is kept.
func PrevPath(target string) string { return target + ".prev" }

// FailedPath is where a binary that failed its health check is kept.
func FailedPath(target string) string { return target + ".failed" }

// stagedPath is where the download goes, next to target: .massimo.new,
// or .massimo.new.exe for massimo.exe, which Windows runs by extension.
func stagedPath(target string) string {
	base := filepath.Base(target)
	if ext := filepath.Ext(base); strings.EqualFold(ext, ".exe") {
		return filepath.Join(filepath.Dir(target), "."+strings.TrimSuffix(base, ext)+".new"+ext)
	}
	return filepath.Join(filepath.Dir(target), "."+base+".new")
}

// Stage creates the file a new binary is downloaded into, next to target
// so that the final rename stays on one file system. A leftover from an
// interrupted update is removed first.
func Stage(target string) (*os.File, error) {
	p := stagedPath(target)
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
}

// Smoke runs the staged binary's version command and checks that it
// starts on this machine (a binary for the wrong ARM version dies with
// "illegal instruction") and is the version expected. It runs with an
// empty environment and no .env file.
func Smoke(ctx context.Context, path, tag string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	// An empty directory to run in, whose .env does not exist: massimo
	// loads its .env before every command and fails on any other error,
	// such as "not a directory" for a path below /dev/null.
	dir, err := os.MkdirTemp("", "massimo-smoke-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	cmd := exec.CommandContext(ctx, path, "version")
	cmd.Dir = dir
	cmd.Env = []string{"MASSIMO_ENV_FILE=" + filepath.Join(dir, ".env")}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("the new binary does not run here: %v: %s", err, strings.TrimSpace(out.String()))
	}
	if f := strings.Fields(out.String()); len(f) < 2 || f[1] != tag {
		return fmt.Errorf("the new binary says %q, want %s", strings.TrimSpace(out.String()), tag)
	}
	return nil
}

// Swap installs the staged file as target and keeps the current target
// as PrevPath. target is replaced by a rename, so it is always either the
// old or the new binary, also for a process starting it meanwhile
// (except on Windows: see swapRenaming). A FailedPath left by an earlier
// failed update goes: a new binary replaces the one it was kept against,
// and an update that fails again keeps a fresh one.
func Swap(target string) error {
	staged := stagedPath(target)
	if err := os.Chmod(staged, 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		if err := swapRenaming(staged, target, PrevPath(target)); err != nil {
			return err
		}
		removeFailed(target)
		return nil
	}
	if err := keepAs(target, PrevPath(target)); err != nil {
		return err
	}
	if err := os.Rename(staged, target); err != nil {
		return err
	}
	removeFailed(target)
	return syncDir(filepath.Dir(target))
}

// removeFailed removes FailedPath; failing to is no reason to fail the
// update, the file is only kept for diagnosis.
func removeFailed(target string) {
	if err := os.Remove(FailedPath(target)); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "update:", err)
	}
}

// Rollback puts PrevPath back as target. The current target is kept as
// keep: PrevPath again for a manual rollback (so that a second one goes
// forward again), or FailedPath after a failed health check. A missing
// target is no obstacle: PrevPath is renamed into its place.
func Rollback(target, keep string) error {
	prev := PrevPath(target)
	if _, err := os.Stat(prev); err != nil {
		return fmt.Errorf("nothing to roll back to: %w", err)
	}
	swap := target + ".swap"
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		// The target is gone (an antivirus quarantined it after the swap):
		// put PrevPath in its place, there is nothing to keep.
		return os.Rename(prev, target)
	}
	if runtime.GOOS == "windows" {
		if err := swapRenaming(prev, target, swap); err != nil {
			return err
		}
		return os.Rename(swap, keep)
	}
	if err := keepAs(target, swap); err != nil {
		return err
	}
	if err := os.Rename(prev, target); err != nil {
		return err
	}
	if err := os.Rename(swap, keep); err != nil {
		return err
	}
	return syncDir(filepath.Dir(target))
}

// swapRenaming puts src in target's place and moves target to aside, for
// Windows, which lets a running .exe be renamed but not replaced. Unlike
// the rename over target elsewhere, there is a moment without target; a
// failure in it puts target back.
func swapRenaming(src, target, aside string) error {
	if err := os.Remove(aside); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(target, aside); err != nil {
		return err
	}
	if err := os.Rename(src, target); err != nil {
		if rerr := os.Rename(aside, target); rerr != nil {
			return fmt.Errorf("%v; and putting %s back failed: %v", err, target, rerr)
		}
		return err
	}
	return nil
}

// Discard removes a staged download.
func Discard(target string) { os.Remove(stagedPath(target)) }

// keepAs makes dst a copy of src without touching src: a hard link where
// the file system allows it, a copy otherwise, then renamed into place.
func keepAs(src, dst string) error {
	tmp := dst + ".tmp"
	os.Remove(tmp)
	if err := os.Link(src, tmp); err != nil {
		if err := copyFile(src, tmp); err != nil {
			return err
		}
	}
	return os.Rename(tmp, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fi.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	d.Sync() // not supported everywhere; the renames are done either way
	return nil
}
