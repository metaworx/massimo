// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package env

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// A line that does not parse, or one too long to be read, is named by
// its number, at the start as at a reload, and never quoted: a value
// wrapped onto a line of its own is half a secret, and a long line may
// be a whole one. Neither sets anything.
func TestParseErrorQuotesNoLine(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ what, text, want string }{
		{"a line without =", "MASSIMO_TEST_Q=placeholder-token-\nwrapped-secret-9\n", ":2: expected KEY=VALUE"},
		{"a line too long", "MASSIMO_TEST_Q=placeholder-token-1\nMASSIMO_TEST_Q2=" + strings.Repeat("s", bufio.MaxScanTokenSize) + "\n",
			":2: " + bufio.ErrTooLong.Error()},
	} {
		p := filepath.Join(t.TempDir(), ".env")
		if err := os.WriteFile(p, []byte(c.text), 0o600); err != nil {
			t.Fatal(err)
		}
		want := p + c.want
		changed, reloadErr := ReloadFile(p)
		for _, f := range []struct {
			what string
			err  error
		}{{"LoadFile", LoadFile(p)}, {"ReloadFile", reloadErr}} {
			if f.err == nil || f.err.Error() != want {
				t.Errorf("%s, %s: %v, want %s", c.what, f.what, f.err, want)
			}
		}
		_, set1 := os.LookupEnv("MASSIMO_TEST_Q")
		_, set2 := os.LookupEnv("MASSIMO_TEST_Q2")
		if set1 || set2 || changed != nil {
			t.Errorf("%s: set %v %v, changed %v", c.what, set1, set2, changed)
		}
	}
}

// File is MASSIMO_ENV_FILE, else ./.env, or the default file
// SetDefaultFile named, which MASSIMO_ENV_FILE still overrides.
func TestFileDefault(t *testing.T) {
	t.Cleanup(func() { SetDefaultFile(".env") })
	t.Setenv(FileVar, "")
	if f := File(); f != ".env" {
		t.Fatalf("without MASSIMO_ENV_FILE: %q", f)
	}
	SetDefaultFile("/etc/massimo/env")
	if f := File(); f != "/etc/massimo/env" {
		t.Fatalf("with a default file: %q", f)
	}
	t.Setenv(FileVar, "given.env")
	if f := File(); f != "given.env" {
		t.Fatalf("MASSIMO_ENV_FILE over the default file: %q", f)
	}
}

// clean gives the test a record of sources of its own, and registers
// the names its files set, so that they are unset again after it.
func clean(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		t.Setenv(n, "")
		os.Unsetenv(n)
	}
	sourcesMu.Lock()
	saved := sources
	sources = map[string]string{}
	sourcesMu.Unlock()
	t.Cleanup(func() {
		sourcesMu.Lock()
		sources = saved
		sourcesMu.Unlock()
	})
}

// A file read again while the program runs changes what it set, sets
// what it names now, unsets what it dropped, and leaves what the
// environment set before it; a parse error or a missing file changes
// nothing.
func TestReloadFile(t *testing.T) {
	clean(t, "MASSIMO_TEST_R1", "MASSIMO_TEST_R2", "MASSIMO_TEST_R3", "MASSIMO_TEST_R4")
	t.Setenv("MASSIMO_TEST_RENV", "from-the-environment")
	p := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(p, []byte("MASSIMO_TEST_R1=one\nMASSIMO_TEST_R2=two\nMASSIMO_TEST_RENV=from-the-file\n"), 0o600)
	if err := LoadFile(p); err != nil {
		t.Fatal(err)
	}
	if changed, err := ReloadFile(p); err != nil || len(changed) != 0 {
		t.Fatalf("an unchanged file: %v %v", changed, err)
	}
	os.WriteFile(p, []byte("MASSIMO_TEST_R1=uno\nMASSIMO_TEST_R3=three\nMASSIMO_TEST_R3=ignored\nMASSIMO_TEST_RENV=again-the-file\n"), 0o600)
	changed, err := ReloadFile(p)
	if err != nil || strings.Join(changed, ",") != "MASSIMO_TEST_R1,MASSIMO_TEST_R2,MASSIMO_TEST_R3" {
		t.Fatalf("changed %v %v", changed, err)
	}
	if v, ok := os.LookupEnv("MASSIMO_TEST_R2"); ok {
		t.Errorf("a dropped variable is still set: %q", v)
	}
	if os.Getenv("MASSIMO_TEST_R1") != "uno" || os.Getenv("MASSIMO_TEST_R3") != "three" || os.Getenv("MASSIMO_TEST_RENV") != "from-the-environment" {
		t.Errorf("got %q %q %q", os.Getenv("MASSIMO_TEST_R1"), os.Getenv("MASSIMO_TEST_R3"), os.Getenv("MASSIMO_TEST_RENV"))
	}
	if Source("MASSIMO_TEST_R3") != "file" || !strings.HasPrefix(Source("MASSIMO_TEST_RENV"), "environment, overrides") {
		t.Errorf("sources %q %q", Source("MASSIMO_TEST_R3"), Source("MASSIMO_TEST_RENV"))
	}
	if changed, _ := ReloadFile(p); len(changed) != 0 {
		t.Errorf("a name given twice changes again: %v", changed)
	}
	os.WriteFile(p, []byte("MASSIMO_TEST_R4=four\ngarbage\n"), 0o600)
	if changed, err := ReloadFile(p); err == nil || changed != nil || os.Getenv("MASSIMO_TEST_R4") != "" || os.Getenv("MASSIMO_TEST_R1") != "uno" {
		t.Errorf("a parse error: %v %v", changed, err)
	}
	os.Remove(p)
	if changed, err := ReloadFile(p); !errors.Is(err, os.ErrNotExist) || changed != nil || os.Getenv("MASSIMO_TEST_R1") != "uno" {
		t.Errorf("a missing file: %v %v", changed, err)
	}
}
