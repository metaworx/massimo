// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"os"
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

// TestHelperFailuresReachTheStatus: in helper mode, every error after the
// request was taken leaves "failed" in the status file, so that the UI
// does not show the previous run's result for a request that went
// nowhere.
func TestHelperFailuresReachTheStatus(t *testing.T) {
	oldVersion := version
	t.Cleanup(func() { version = oldVersion })
	t.Setenv("MASSIMO_UPDATE_CHANNEL", "")
	dir := t.TempDir()
	bin := filepath.Join(dir, "massimo")
	os.WriteFile(bin, []byte("binary"), 0o755)
	for _, c := range []struct{ version, want string }{
		{"", "development or modified build"},   // a build without a version
		{"v0.9.0-alpha.1", "no update channel"}, // a request without a channel, a build without one
	} {
		version = c.version
		req, status := filepath.Join(dir, "update-request"), filepath.Join(dir, "status.json")
		os.WriteFile(req, nil, 0o600)
		if err := update.WriteStatus(status, update.Status{State: "done", To: "v0.8.0-alpha.1"}); err != nil {
			t.Fatal(err)
		}
		err := runUpdate([]string{"-request", req, "-status", status, "-binary", bin})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("version %q: %v, want %q", c.version, err, c.want)
		}
		st, err := update.ReadStatus(status)
		if err != nil || st.State != "failed" || !strings.Contains(st.Message, c.want) {
			t.Errorf("version %q: status %+v, %v", c.version, st, err)
		}
	}
}
