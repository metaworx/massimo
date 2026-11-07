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

// A request written while the helper reads the last one: the helper's
// handle, opened as TakeRequest opens it, refuses a rename over the file
// on Windows until it is closed, a few milliseconds later, and the
// server answered the second POST /api/v1/update with 500.
func TestWriteRequestWhileTheHelperReadsIt(t *testing.T) {
	p := filepath.Join(t.TempDir(), "update-request")
	if err := WriteRequest(p, "alpha"); err != nil {
		t.Fatal(err)
	}
	f, err := openNoFollow(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	closed := time.AfterFunc(150*time.Millisecond, func() { f.Close() })
	defer closed.Stop()
	if err := WriteRequest(p, "beta"); err != nil {
		t.Fatalf("the request written while the helper reads the last one: %v", err)
	}
	if b, err := os.ReadFile(p); err != nil || string(b) != "beta\n" {
		t.Fatalf("the request holds %q, %v", b, err)
	}
}
