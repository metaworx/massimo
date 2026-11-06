// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/update"
)

// TestUpdateVariablesAreKnown keeps massimo config from flagging the
// variables massimo update reads as typos.
func TestUpdateVariablesAreKnown(t *testing.T) {
	for _, k := range []string{update.TokenVar, "MASSIMO_UPDATE_CHANNEL"} {
		if !env.Known(k) {
			t.Errorf("%s unknown to massimo config", k)
		}
	}
}

// TestSmokeRunsRealBinary builds massimo and runs the update's smoke test
// on it: the stand-in scripts of internal/update cannot show how the real
// binary starts (it loads its .env before every command).
func TestSmokeRunsRealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "massimo")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	out, err := exec.Command("go", "build", "-ldflags", "-X main.version=v9.8.7", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	if err := update.Smoke(context.Background(), bin, "v9.8.7"); err != nil {
		t.Fatal(err)
	}
	if err := update.Smoke(context.Background(), bin, "v9.8.8"); err == nil {
		t.Fatal("wrong version passed")
	}
}
