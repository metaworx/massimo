// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The swap waits for a binary another process holds for a moment. CI's
// Windows service job had the rename of the massimo.exe just installed
// refused, "being used by another process", seconds after the install,
// as a virus scanner reads a new .exe, and the update failed at once.
func TestSwapWaitsForAHeldTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "massimo.exe")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := Stage(target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("new"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	held, err := os.Open(target) // without FILE_SHARE_DELETE, as a scanner may
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(300*time.Millisecond, func() { held.Close() })
	if err := Swap(target); err != nil {
		t.Fatalf("the swap of a held binary: %v", err)
	}
	for path, want := range map[string]string{target: "new", PrevPath(target): "old"} {
		if b, err := os.ReadFile(path); err != nil || string(b) != want {
			t.Errorf("%s holds %q, %v; want %q", filepath.Base(path), b, err, want)
		}
	}
}
