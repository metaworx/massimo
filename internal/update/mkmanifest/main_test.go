// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRefusesWhatNoReleaseIs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "massimo-linux-amd64"), []byte("bin"), 0o644); err != nil {
		t.Fatal(err)
	}
	// run writes the manifest to stdout, which the accepted cases send
	// to the null device.
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = null
	defer func() { null.Close(); os.Stdout = stdout }()

	for _, tc := range []struct {
		tag, flavour, release string
		refused               string // part of the error, "" when accepted
	}{
		{"v0.12.0-alpha.1", "alpha", "", ""},
		{"v0.12.0-alpha.1", "alpha", "v0.12.0-alpha.1", ""},
		{"v1.0.0", "stable", "", ""},
		{"v0.12.0-alpha.1-3-gabc1234", "devel", "release/devel", ""},
		{"v0.12.0-alpha.1-3-gabc1234", "nightly", "release/nightly", ""},
		{"v0.12.0-alpha.1-3-gabc1234", "alpha", "", "only a rolling flavour"},
		{"v0.12.0-alpha.1-3-gabc1234", "devel", "", "is released as release/devel"},
		{"v0.12.0-alpha.1-3-gabc1234", "devel", "release/nightly", "is released as release/devel"},
		{"v0.12.0-alpha.1", "alpha", "release/alpha", "under its tag"},
		{"v0.12.0-alpha.1-dirty", "alpha", "", "not a release's version"},
		{"v0.12.0-alpha.1-3-gabc1234-dirty", "devel", "release/devel", "not a release's version"},
		{"v0.12.0-alpha.1", "", "", "invalid flavour"},
	} {
		err := run(tc.tag, "c0ffee", tc.flavour, tc.release, dir, []string{"massimo-linux-amd64"})
		switch {
		case tc.refused == "" && err != nil:
			t.Errorf("%+v: %v", tc, err)
		case tc.refused != "" && (err == nil || !strings.Contains(err.Error(), tc.refused)):
			t.Errorf("%+v: got %v, want %q", tc, err, tc.refused)
		}
	}
}
