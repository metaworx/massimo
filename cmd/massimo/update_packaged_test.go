// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/update"
)

// TestUpdateRefusesPackaged: a binary a package manager owns is left to
// it, before any network, and the status file says so for the UI.
func TestUpdateRefusesPackaged(t *testing.T) {
	old := packaged
	packaged = func() (install.Manager, bool) {
		return install.Manager{Name: "apt", Command: "sudo apt install massimo"}, true
	}
	t.Cleanup(func() { packaged = old })
	status := filepath.Join(t.TempDir(), "status.json")
	for _, args := range [][]string{{"-yes"}, {"-rollback"}} {
		err := runUpdate(append(args, "-status", status))
		if err == nil || !strings.Contains(err.Error(), "managed by apt: sudo apt install massimo") {
			t.Fatalf("%v: got %v", args, err)
		}
	}
	st, err := update.ReadStatus(status)
	if err != nil || st.State != "failed" || !strings.Contains(st.Message, "managed by apt") {
		t.Fatalf("status %+v, %v", st, err)
	}
}
