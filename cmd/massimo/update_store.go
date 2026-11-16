// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// The store after a rollback of the binary (AP StoreRaise v1.2, Block 1,
// D1-D; docs/store.md section 5, "Migrations"): where the newer binary
// migrated the store past what the binary put back reads, the store goes
// back too, with the service stopped, by the kept binary's `store
// downgrade`, else by the binary put back, which has the down parts the
// store keeps. The update runs as root, so it opens no store itself: the
// binaries do, as the store's owner (update.StoreSchema,
// update.StoreDowngrade).

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/update"
)

// stopService stops the systemd unit, on macOS the launchd job, or on
// Windows the service, for the store's downgrade, which needs the store
// alone; a variable for the tests.
var stopService = stopUnit

func stopUnit(name string) error {
	if name == "" {
		return nil
	}
	switch runtime.GOOS {
	case "windows":
		return install.StopService(name)
	case "darwin":
		return install.StopLaunchd(name)
	}
	out, err := exec.Command("systemctl", "stop", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl stop %s: %v: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// updateStoreDir is the store a rollback looks at: the one -data or
// MASSIMO_DATA names, else the service's on this host ("" where there is
// none), since the helper runs as root from /.
func updateStoreDir() string {
	if !implicitDataDir() {
		return dataDir()
	}
	return serviceStoreDir()
}

// rollbackStore takes the store back after update.Rollback put target
// back and kept the other binary as kept, where target refuses the store
// as too new. self says that target is this binary (the rollback after a
// failed health check by the binary that ran the update), which has
// `store downgrade` and so takes the store back by the store's own down
// parts where kept cannot. unit is stopped first and left stopped: the
// caller restarts it. It returns what it did for the update's status, ""
// where the store needed nothing.
func rollbackStore(ctx context.Context, target, kept string, self bool, unit string) (string, error) {
	dir := updateStoreDir()
	if dir == "" {
		return "", nil
	}
	file := store.Path(dir)
	if _, err := os.Lstat(file); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	reader, reads, err := update.StoreSchema(ctx, target, dir, file)
	if err != nil {
		return "", fmt.Errorf("the store: %s did not say whether it opens %s: %w", target, file, err)
	}
	if reads {
		return "", nil
	}
	fmt.Printf("%s is too new for %s (schema version %d): taking it back\n", file, target, reader)
	if err := stopService(unit); err != nil {
		return "", fmt.Errorf("%s is too new for %s, and the service did not stop for its downgrade: %w", file, target, err)
	}
	out, kerr := update.StoreDowngrade(ctx, kept, dir, file, reader)
	fmt.Print(out)
	if kerr == nil {
		return fmt.Sprintf("the store went back to what schema version %d reads, by %s", reader, kept), nil
	}
	if !self {
		return "", fmt.Errorf("%s is too new for %s and did not go back (%w); run %s -data %s store downgrade -to %d, or put back the copy %s.before-<N> made before the migration N that raised min_reader",
			file, target, kerr, kept, dir, reader, file)
	}
	fmt.Fprintln(os.Stderr, "update:", kerr, "- taking the store back by the down parts it keeps")
	out, err = update.StoreDowngrade(ctx, target, dir, file, reader)
	fmt.Print(out)
	if err != nil {
		return "", fmt.Errorf("%s is too new for %s and did not go back (%v; %w); run %s -data %s store downgrade -to %d, or put back the copy %s.before-<N> made before the migration N that raised min_reader",
			file, target, kerr, err, kept, dir, reader, file)
	}
	return fmt.Sprintf("the store went back to what schema version %d reads, by the down parts it keeps", reader), nil
}
