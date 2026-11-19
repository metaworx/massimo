// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// writeTests writes each of files, a name and its source, into a
// directory of its own, and returns the directory.
func writeTests(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// The guard passes a package whose tests keep the rule, and names each
// place that breaks it: a test neither parallel nor listed, one parallel
// later than first, a listed test that is parallel, a stale entry, an
// entry without a reason, and Parallel() in a helper or in a function
// literal that is no subtest's. A subtest's own Parallel() is no test's,
// and a file no other system builds is read too.
func TestParallelProblems(t *testing.T) {
	good := writeTests(t, map[string]string{
		"a_test.go": `package p

import "testing"

func TestParallel(t *testing.T) {
	t.Parallel()
	t.Run("sub", func(t *testing.T) { t.Parallel() })
}

func TestSerial(t *testing.T) {
	t.Run("sub", func(t *testing.T) { t.Parallel() })
}

func helper(tb testing.TB) {}

func Testlower(t *testing.T) {}

func BenchmarkX(b *testing.B) {}
`,
		"b_windows_test.go": `package p

import "testing"

func TestOnWindows(c *testing.T) {
	c.Parallel()
}
`,
	})
	problems, err := parallelProblems(good, map[string]string{"TestSerial": "swaps a hook"})
	if err != nil || len(problems) != 0 {
		t.Errorf("a package that keeps the rule: %q %v", problems, err)
	}

	bad := writeTests(t, map[string]string{
		"a_test.go": `package p

import "testing"

func TestNeither(t *testing.T) {
	t.Helper()
}

func TestLate(t *testing.T) {
	t.Helper()
	t.Parallel()
}

func TestListedButParallel(t *testing.T) {
	t.Parallel()
}

func TestNoReason(t *testing.T) {}

func helper(t *testing.T) {
	t.Parallel()
}

func TestInALiteral(t *testing.T) {
	t.Parallel()
	f := func() { t.Parallel() }
	f()
}
`,
	})
	problems, err = parallelProblems(bad, map[string]string{
		"TestListedButParallel": "binds a fixed port",
		"TestNoReason":          " ",
		"TestGone":              "was renamed",
	})
	if err != nil {
		t.Fatal(err)
	}
	at := filepath.Join(bad, "a_test.go")
	want := []string{
		at + ":21:2: Parallel() outside a test's body and a subtest's function literal",
		at + ":26:16: Parallel() outside a test's body and a subtest's function literal",
		at + ":5:1: TestNeither neither calls t.Parallel() first nor is on the serial list with its reason",
		at + ":9:1: TestLate neither calls t.Parallel() first nor is on the serial list with its reason",
		at + ":14:1: TestListedButParallel is on the serial list (binds a fixed port) and calls t.Parallel()",
		"the serial list names TestGone, which is no test of the package",
		"the serial list names TestNoReason without a reason",
	}
	if !slices.Equal(problems, want) {
		t.Errorf("the problems:\n%q\nwant\n%q", problems, want)
	}

	empty := t.TempDir()
	if _, err := parallelProblems(empty, nil); err == nil || err.Error() != "no test files in "+empty {
		t.Errorf("a directory without test files: %v", err)
	}
}

// Swapping and t.Parallel exclude each other, in either order: a swap
// in a parallel test panics, and so does a parallel test that swapped.
func TestSwappingForbidsParallel(t *testing.T) {
	panics := func(f func()) (p any) {
		defer func() { p = recover() }()
		f()
		return nil
	}
	t.Run("swap after parallel", func(t *testing.T) {
		t.Parallel()
		if p := panics(func() { Swapping(t) }); p == nil {
			t.Error("Swapping in a parallel test did not panic")
		}
	})
	t.Run("parallel after swap", func(t *testing.T) {
		Swapping(t)
		if v := os.Getenv(SwapVar); v != "1" {
			t.Errorf("%s=%q while swapping", SwapVar, v)
		}
		if p := panics(t.Parallel); p == nil {
			t.Error("t.Parallel after Swapping did not panic")
		}
	})
}
