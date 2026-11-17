// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// recorder is a test that records its failure instead of failing: its
// Fatalf ends the goroutine it runs in, as the real one does.
type recorder struct {
	testing.TB
	failed string
}

func (r *recorder) Helper() {}

func (r *recorder) Fatalf(format string, args ...any) {
	r.failed = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// run calls f with a recorder in a goroutine of its own and returns the
// failure f met, if any.
func run(t *testing.T, f func(tb testing.TB)) string {
	t.Helper()
	r := &recorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		f(r)
	}()
	<-done
	return r.failed
}

// after returns a condition that holds from the n-th check on.
func after(n int) func() bool {
	var calls atomic.Int64
	return func() bool { return calls.Add(1) >= int64(n) }
}

// The scale is 1 without a value, the number given otherwise, and
// nothing that is not a positive number.
func TestParseScale(t *testing.T) {
	for _, c := range []struct {
		v    string
		want float64
	}{{"", 1}, {"1", 1}, {"2", 2}, {"4", 4}, {"0.5", 0.5}, {"1.5", 1.5}} {
		if got, err := parseScale(c.v); err != nil || got != c.want {
			t.Errorf("%q: %v, %v; want %v", c.v, got, err, c.want)
		}
	}
	for _, v := range []string{"0", "-1", "two", "NaN", "Inf", "+Inf", " 2"} {
		if got, err := parseScale(v); err == nil {
			t.Errorf("%q: %v and no error", v, got)
		}
	}
}

// Scale multiplies by the scale this run was given.
func TestScale(t *testing.T) {
	want := 1.0
	if v := os.Getenv(ScaleVar); v != "" {
		var err error
		if want, err = strconv.ParseFloat(v, 64); err != nil {
			t.Fatal(err)
		}
	}
	if got := Scale(time.Second); got != time.Duration(want*float64(time.Second)) {
		t.Errorf("Scale(1s) = %v with %s=%q", got, ScaleVar, os.Getenv(ScaleVar))
	}
}

// Eventually returns once the condition holds, and fails with what it
// waited for and how long where it does not.
func TestEventually(t *testing.T) {
	if f := run(t, func(tb testing.TB) { Eventually(tb, "the third check", after(3), Every(time.Millisecond)) }); f != "" {
		t.Errorf("a condition that comes to hold: %q", f)
	}
	f := run(t, func(tb testing.TB) {
		Eventually(tb, "nothing", func() bool { return false }, For(20*time.Millisecond), Every(time.Millisecond))
	})
	if want := fmt.Sprintf("timed out after %v waiting for nothing", Scale(20*time.Millisecond)); f != want {
		t.Errorf("a condition that never holds: %q, want %q", f, want)
	}
}

// Eventually checks the condition once more after its time is up, so
// that a condition met during the last tick counts.
func TestEventuallyChecksAtTheEnd(t *testing.T) {
	start := time.Now()
	d := Scale(30 * time.Millisecond)
	f := run(t, func(tb testing.TB) {
		Eventually(tb, "the end", func() bool { return time.Since(start) >= d }, For(30*time.Millisecond), Every(time.Hour))
	})
	if f != "" {
		t.Errorf("met during the last tick: %q", f)
	}
}

// Within says whether the condition held in time, and fails nothing.
func TestWithin(t *testing.T) {
	if !Within(time.Second, after(2)) {
		t.Error("a condition that comes to hold")
	}
	if Within(20*time.Millisecond, func() bool { return false }) {
		t.Error("a condition that never holds")
	}
}

// Never passes where the condition stays false until the end, and fails
// where it holds first, where it holds at the end, or where the end does
// not come.
func TestNever(t *testing.T) {
	if f := run(t, func(tb testing.TB) {
		Never(tb, "a second line", func() bool { return false }, after(3), Every(time.Millisecond))
	}); f != "" {
		t.Errorf("nothing before the end: %q", f)
	}
	if f := run(t, func(tb testing.TB) {
		Never(tb, "a second line", after(2), after(5), Every(time.Millisecond))
	}); f != "came, and was not to: a second line" {
		t.Errorf("the thing before the end: %q", f)
	}
	// cond false at the first check, true at the second: the one after the end held
	var checks atomic.Int64
	if f := run(t, func(tb testing.TB) {
		Never(tb, "a second line", func() bool { return checks.Add(1) >= 2 }, func() bool { return true })
	}); f != "came, and was not to: a second line" {
		t.Errorf("the thing as the end comes: %q", f)
	}
	f := run(t, func(tb testing.TB) {
		Never(tb, "a second line", func() bool { return false }, func() bool { return false }, For(20*time.Millisecond), Every(time.Millisecond))
	})
	if want := fmt.Sprintf("timed out after %v waiting for the end of the check for a second line", Scale(20*time.Millisecond)); f != want {
		t.Errorf("no end: %q, want %q", f, want)
	}
}

// kitImport is the package's import path.
const kitImport = "go.metaworx.ch/massimo/internal/testkit"

// The kit is test code: no file of the module but a test imports it or a
// package under it, so that nothing shipped waits as a test does.
func TestOnlyTestsImportTheKit(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("no module root at %s: %v", root, err)
	}
	here, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// As the go tool: no directory whose name starts with . or _,
			// no testdata (other worktrees live under .claude), and not
			// the guidelines, which hold no Go of the module.
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "node_modules" || name == "GUIDELINES") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || filepath.Dir(path) == here {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		seen++
		for _, imp := range f.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); p == kitImport || strings.HasPrefix(p, kitImport+"/") {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s imports %s, which only tests may", rel, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen < 100 {
		t.Fatalf("only %d files read under %s: the walk missed the module", seen, root)
	}
}
