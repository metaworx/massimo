// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Command tally says what a connected run of the emulator suite ran
// (tools/app-emulator/README.md, "The results"): the tests of Gradle's
// JUnit results (every TEST-*.xml under the directory it is given), each
// with how it went, against those the suite's sources declare (every
// @Test of the androidTest's Kotlin, narrowed by the run's class filter
// as the instrumentation's -e class takes it). Gradle passes a run whose
// tests were skipped, or that ran fewer than declared; with -strict, as
// CI runs it, either fails here.
//
// usage: tally -what <label> -tests <androidTest's kotlin dir> [-class <filter>] [-strict] <results dir>
//
// It exits 0 where the run is as it should be or -strict is not given, 1
// where -strict finds a test skipped, failed or not run, none run, or the
// results unreadable, and 2 for a wrong call.
package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The outcomes of a test, as the results give them.
const (
	passed  = "passed"
	skipped = "skipped"
	failed  = "failed"
)

// result is a test that ran: its key (package.Class#method) and outcome.
type result struct {
	key, outcome string
}

// testcase is a JUnit testcase element; a skipped child marks a skip, a
// failure or error child a failure.
type testcase struct {
	Name     string    `xml:"name,attr"`
	Class    string    `xml:"classname,attr"`
	Skipped  *struct{} `xml:"skipped"`
	Failures []string  `xml:"failure"`
	Errors   []string  `xml:"error"`
}

// readResults reads the tests of every TEST-*.xml under dir, in the
// order the files and their cases come, testcase elements at any depth.
func readResults(dir string) ([]result, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasPrefix(d.Name(), "TEST-") && strings.HasSuffix(d.Name(), ".xml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("the results: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no TEST-*.xml under %s", dir)
	}
	sort.Strings(files)
	var out []result
	for _, f := range files {
		rs, err := readFile(f)
		if err != nil {
			return nil, err
		}
		out = append(out, rs...)
	}
	return out, nil
}

// readFile reads the testcase elements of one results file.
func readFile(path string) ([]result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []result
	d := xml.NewDecoder(f)
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "testcase" {
			continue
		}
		var c testcase
		if err := d.DecodeElement(&c, &start); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		r := result{key: c.Class + "#" + c.Name, outcome: passed}
		switch {
		case len(c.Failures) > 0 || len(c.Errors) > 0:
			r.outcome = failed
		case c.Skipped != nil:
			r.outcome = skipped
		}
		out = append(out, r)
	}
}

var (
	packageLine = regexp.MustCompile(`(?m)^package\s+([\w.]+)`)
	classLine   = regexp.MustCompile(`^\s*(?:(?:public|internal|private|open|abstract|final|data)\s+)*class\s+(\w+)`)
	testLine    = regexp.MustCompile(`^\s*@Test\b`)
	funLine     = regexp.MustCompile(`\bfun\s+(\w+)\s*\(`)
)

// declared lists the tests the Kotlin under src declares, sorted, each
// the first fun after an @Test that opens a line, in the class declared
// last before it. A test of an abstract class counts under that class,
// which no run names; the suite has none.
func declared(src string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".kt") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		pkg := ""
		if m := packageLine.FindSubmatch(b); m != nil {
			pkg = string(m[1]) + "."
		}
		class, pending := "", false
		for _, line := range strings.Split(string(b), "\n") {
			if m := classLine.FindStringSubmatch(line); m != nil {
				class = m[1]
			}
			if testLine.MatchString(line) {
				pending = true
			}
			if m := funLine.FindStringSubmatch(line); m != nil && pending {
				out = append(out, pkg+class+"#"+m[1])
				pending = false
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("the suite's sources: %w", err)
	}
	sort.Strings(out)
	return out, nil
}

// narrow keeps the tests the class filter names: a comma-separated list
// of classes (package.Class, every test of it) and tests
// (package.Class#method); an empty filter keeps them all.
func narrow(tests []string, filter string) []string {
	if strings.TrimSpace(filter) == "" {
		return tests
	}
	var out []string
	for _, t := range tests {
		for _, f := range strings.Split(filter, ",") {
			f = strings.TrimSpace(f)
			if f != "" && (t == f || (!strings.Contains(f, "#") && strings.HasPrefix(t, f+"#"))) {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

// short is a test's key as the lines show it: Class.method.
func short(key string) string {
	class, method, _ := strings.Cut(key, "#")
	if i := strings.LastIndex(class, "."); i >= 0 {
		class = class[i+1:]
	}
	return class + "." + method
}

// report writes what ran under the label what, each test with its
// outcome, then the declared tests that did not run; it reports whether
// the run is as -strict wants it: tests ran, none skipped or failed, and
// every declared one among them.
func report(w io.Writer, what string, got []result, want []string) bool {
	n := map[string]int{}
	ran := map[string]bool{}
	for _, r := range got {
		n[r.outcome]++
		ran[r.key] = true
	}
	var missing []string
	for _, t := range want {
		if !ran[t] {
			missing = append(missing, t)
		}
	}
	fmt.Fprintf(w, "app-emulator: %s: %d tests, %d skipped, %d failed; %d of the %d the sources declare not run\n",
		what, len(got), n[skipped], n[failed], len(missing), len(want))
	declaredSet := map[string]bool{}
	for _, t := range want {
		declaredSet[t] = true
	}
	for _, r := range got {
		note := ""
		if !declaredSet[r.key] {
			note = " (not among the tests the sources declare)"
		}
		fmt.Fprintf(w, "app-emulator:   %-8s %s%s\n", r.outcome, short(r.key), note)
	}
	for _, t := range missing {
		fmt.Fprintf(w, "app-emulator:   %-8s %s\n", "not run", short(t))
	}
	return len(got) > 0 && n[skipped] == 0 && n[failed] == 0 && len(missing) == 0
}

// run is the command, its exit code returned.
func run(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("tally", flag.ContinueOnError)
	fl.SetOutput(stderr)
	what := fl.String("what", "the run", "the run's name in the lines")
	tests := fl.String("tests", "", "the androidTest's Kotlin, whose @Test funs are the tests declared")
	class := fl.String("class", "", "the run's class filter, as the instrumentation's -e class takes it")
	strict := fl.Bool("strict", false, "fail where a test was skipped, failed or not run, or none ran")
	if err := fl.Parse(args); err != nil || fl.NArg() != 1 || *tests == "" {
		fmt.Fprintln(stderr, "usage: tally -what <label> -tests <androidTest's kotlin dir> [-class <filter>] [-strict] <results dir>")
		return 2
	}
	mode := "it does not fail the run (APP_EMU_STRICT=1, or CI, makes it)"
	if *strict {
		mode = "strict, as in CI or with APP_EMU_STRICT=1"
	}
	want, err := declared(*tests)
	if err == nil {
		want = narrow(want, *class)
		var got []result
		got, err = readResults(fl.Arg(0))
		if err == nil {
			if report(stdout, *what, got, want) {
				return 0
			}
			fmt.Fprintf(stderr, "app-emulator: %s: not every test the sources declare ran and passed; %s\n", *what, mode)
			if *strict {
				return 1
			}
			return 0
		}
	}
	fmt.Fprintf(stderr, "app-emulator: %s: %v; %s\n", *what, err, mode)
	if *strict {
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
