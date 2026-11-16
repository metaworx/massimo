// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package update

// The store after a rollback (AP StoreRaise v1.2, Block 1, D1-D): a
// newer binary may have migrated the store past what the binary put back
// reads, and that binary then refuses it. The binary set aside by the
// rollback (FailedPath after a failed health check, PrevPath after
// -rollback) is the newer one, so it knows the down parts: StoreDowngrade
// runs its `store downgrade`. StoreSchema asks the binary put back
// whether it opens the store. Both run the binary as Smoke does, reading
// no .env, on Windows through a name ending in .exe, and, where root
// runs them, as the store's owner: the helper runs as root and reads
// nothing the service's user can write, so it never opens the store
// itself.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// StoreDowngrade runs `<bin> -data <dir> store downgrade -to <to>` as the
// owner of the file owner (the store's massimo.db) and returns what it
// printed: the store goes back to what a binary for schema version to
// opens, by the down parts bin knows, else those the store keeps. The
// service must be stopped first.
func StoreDowngrade(ctx context.Context, bin, dir, owner string, to int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	out, err := runBinary(ctx, bin, owner, "-data", dir, "store", "downgrade", "-to", strconv.Itoa(to))
	if err != nil {
		return out, fmt.Errorf("%s store downgrade: %w", bin, err)
	}
	return out, nil
}

// schemaLine finds what `store info` says of a store: "this binary: N"
// where the binary reads it, "this one is for version N" in its refusal
// of one too new for it.
var schemaLine = regexp.MustCompile(`this binary: (\d+)\)|this one is for version (\d+)`)

// StoreSchema runs `<bin> -data <dir> store info` as the owner of the
// file owner and returns the schema version bin is built for and whether
// it opens the store; every binary with a store says both, also when it
// refuses the store as too new. It changes nothing.
func StoreSchema(ctx context.Context, bin, dir, owner string) (schema int, reads bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	out, err := runBinary(ctx, bin, owner, "-data", dir, "store", "info")
	m := schemaLine.FindStringSubmatch(out)
	if m == nil {
		if err == nil {
			err = errors.New("no schema version in its output")
		}
		return 0, false, fmt.Errorf("%s store info: %w: %s", bin, err, strings.TrimSpace(out))
	}
	schema, _ = strconv.Atoi(m[1] + m[2])
	return schema, m[1] != "", nil
}

// runBinary runs bin with args in an empty temporary directory, whose
// .env does not exist, as Smoke does, and returns its output, stdout and
// stderr together; where this process is root, as the owner of the file
// owner. On Windows, which runs a file by its extension, a kept binary
// (massimo.exe.failed, massimo.exe.prev) runs through a link beside it
// whose name ends in .exe, removed afterwards.
func runBinary(ctx context.Context, bin, owner string, args ...string) (string, error) {
	if _, err := os.Stat(bin); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "massimo-run-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	run := bin
	if runtime.GOOS == "windows" && !strings.EqualFold(filepath.Ext(bin), ".exe") {
		run = filepath.Join(filepath.Dir(bin), ".massimo.run.exe")
		os.Remove(run)
		if err := os.Link(bin, run); err != nil {
			if err := copyFile(bin, run); err != nil {
				return "", err
			}
		}
		defer os.Remove(run)
	}
	cmd := exec.CommandContext(ctx, run, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "MASSIMO_ENV_FILE="+filepath.Join(dir, ".env"))
	if err := runAsOwner(cmd, owner); err != nil {
		return "", err
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	return out.String(), err
}
