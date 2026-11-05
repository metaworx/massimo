// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(p, []byte("# comment\n\nMASSIMO_TEST_A=\"k0011\"\nexport MASSIMO_TEST_B = 'x y'\nMASSIMO_TEST_C=plain\n"), 0o600)
	t.Setenv("MASSIMO_TEST_C", "already")
	if err := LoadFile(p); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("MASSIMO_TEST_A") != "k0011" || os.Getenv("MASSIMO_TEST_B") != "x y" || os.Getenv("MASSIMO_TEST_C") != "already" {
		t.Fatalf("got %q %q %q", os.Getenv("MASSIMO_TEST_A"), os.Getenv("MASSIMO_TEST_B"), os.Getenv("MASSIMO_TEST_C"))
	}
	if err := LoadFile(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("missing file must not be an error: %v", err)
	}
	os.WriteFile(p, []byte("garbage\n"), 0o600)
	if err := LoadFile(p); err == nil {
		t.Fatal("want error for malformed line")
	}
}
