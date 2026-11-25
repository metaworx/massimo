// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// put writes body into the file path under dir, its directories made.
func put(t *testing.T, dir, path, body string) {
	t.Helper()
	p := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// junit is a results file as Gradle's connected run writes it, with the
// cases given.
func junit(cases ...string) string {
	return `<?xml version='1.0' encoding='UTF-8' ?>
<testsuite name="ch.example.ATest" tests="3" failures="1" errors="0" skipped="1" time="12.5" timestamp="2026-11-24T02:40:43" hostname="localhost">
  <properties>
    <property name="device" value="massimo-test(AVD) - 14" />
    <property name="flavor" value="foss" />
    <property name="project" value=":app" />
  </properties>
` + strings.Join(cases, "\n") + `
</testsuite>
`
}

// sources writes two test classes and a helper under dir: ATest's two
// tests, one with an annotation between, BTest's one on its @Test's line,
// a KDoc that names @Test and a helper with no test.
func sources(t *testing.T, dir string) {
	t.Helper()
	put(t, dir, "ch/example/ATest.kt", `package ch.example

/**
 * Not a test:
 * @Test in a comment
 */
@RunWith(AndroidJUnit4::class)
class ATest : Base() {
    @Test
    fun first() {
        val x = helper(1)
    }

    @Test
    @SdkSuppress(minSdkVersion = 30)
    fun second() {}

    private fun helper(n: Int): Int = n
}
`)
	put(t, dir, "ch/example/BTest.kt", "package ch.example\n\nclass BTest {\n    @Test fun only() {}\n}\n")
	put(t, dir, "ch/example/Base.kt", "package ch.example\n\nabstract class Base {\n    @TestOnly\n    fun notATest() {}\n}\n")
	put(t, dir, "ch/example/notes.txt", "@Test\nfun no()\n")
}

// TestDeclaredReadsTheTestsOfEachClass: every @Test that opens a line
// declares the next fun, in its class and package; a comment, another
// annotation and a file that is not Kotlin declare none.
func TestDeclaredReadsTheTestsOfEachClass(t *testing.T) {
	dir := t.TempDir()
	sources(t, dir)
	got, err := declared(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ch.example.ATest#first", "ch.example.ATest#second", "ch.example.BTest#only"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("declared %q, want %q", got, want)
	}
}

// TestDeclaredReadsTheSuite: the suite's own sources as tally reads them,
// each test class of it with a test, the enrollment among them.
func TestDeclaredReadsTheSuite(t *testing.T) {
	src := filepath.Join("..", "..", "..", "app", "android", "app", "src", "androidTest", "kotlin")
	got, err := declared(src)
	if err != nil {
		t.Fatal(err)
	}
	classes := map[string]bool{}
	for _, k := range got {
		class, _, _ := strings.Cut(k, "#")
		classes[class] = true
	}
	files, err := filepath.Glob(filepath.Join(src, "ch", "metaworx", "massimo", "*Test.kt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "@RunWith") {
			continue // the abstract EmulatorTest
		}
		class := "ch.metaworx.massimo." + strings.TrimSuffix(filepath.Base(f), ".kt")
		if !classes[class] {
			t.Errorf("no test declared in %s: %q", class, got)
		}
	}
	if len(narrow(got, "ch.metaworx.massimo.EnrollTest")) != 1 {
		t.Errorf("the enrollment is not one test: %q", got)
	}
}

// TestReadResults: each testcase of every TEST-*.xml under the
// directory, at any depth, with its outcome and a failure's first lines;
// other files are not read.
func TestReadResults(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "fossDebug/TEST-massimo-test(AVD) - 14-_app-.xml", junit(
		`  <testcase name="first" classname="ch.example.ATest" time="1.0" />`,
		`  <testcase name="second" classname="ch.example.ATest" time="0.0">
    <skipped />
  </testcase>`,
		`  <testcase name="only" classname="ch.example.BTest" time="2.0">
    <failure>
java.lang.AssertionError: the prompt stayed

	at ch.example.BTest.only(BTest.kt:4)
</failure>
  </testcase>`,
	))
	put(t, dir, "fossDebug/test-result.pb", "not xml")
	got, err := readResults(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []result{
		{"ch.example.ATest#first", passed, ""},
		{"ch.example.ATest#second", skipped, ""},
		{"ch.example.BTest#only", failed, "java.lang.AssertionError: the prompt stayed\nat ch.example.BTest.only(BTest.kt:4)"},
	}
	if len(got) != len(want) {
		t.Fatalf("read %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("case %d: %v, want %v", i, got[i], want[i])
		}
	}
	if _, err := readResults(t.TempDir()); err == nil || !strings.Contains(err.Error(), "no TEST-*.xml") {
		t.Errorf("no results: %v", err)
	}
	put(t, dir, "fossDebug/TEST-broken.xml", "<testsuite><testcase name=")
	if _, err := readResults(dir); err == nil {
		t.Error("a broken file read as results")
	}
}

// TestNarrow: the class filter keeps a class's tests, a test by its
// method, a list of either; empty, it keeps all.
func TestNarrow(t *testing.T) {
	all := []string{"p.A#x", "p.A#y", "p.AB#z", "p.B#w"}
	for filter, want := range map[string]string{
		"":             "p.A#x p.A#y p.AB#z p.B#w",
		"p.A":          "p.A#x p.A#y",
		"p.A#y":        "p.A#y",
		"p.A#y, p.B":   "p.A#y p.B#w",
		"p.C":          "",
		"p.A#nothing,": "",
	} {
		if got := strings.Join(narrow(all, filter), " "); got != want {
			t.Errorf("filter %q: %q, want %q", filter, got, want)
		}
	}
}

// TestRun: the line and each test's, a failure's first lines under it,
// Gradle's record of an instrumentation that ended early named as the
// run's, and the exit code: 0 for a run of every declared test passing;
// with -strict 1 for a test skipped, failed or not run, none run or no
// results, and 0 without -strict all the same.
func TestRun(t *testing.T) {
	src := t.TempDir()
	sources(t, src)
	pass := func(name, class string) string {
		return `  <testcase name="` + name + `" classname="` + class + `" time="1.0" />`
	}
	all := []string{pass("first", "ch.example.ATest"), pass("second", "ch.example.ATest"), pass("only", "ch.example.BTest")}
	// seven lines, of which the report prints the first six
	failure := `<testcase name="only" classname="ch.example.BTest"><failure>java.lang.AssertionError: the prompt stayed
	at line 1
	at line 2
	at line 3
	at line 4
	at line 5
	at line 6</failure></testcase>`
	// the orchestrator's record of an instrumentation killed before its
	// test, its seventh line past what the report prints
	crash := `<testcase name="testRunFailed" classname="android-test" time="0.000"><failure>Test run failed to complete. Expected 3 tests, received 2.

Time: 12.5
There was 1 failure:
1) only(ch.example.BTest)
Test instrumentation process crashed. Check ch.example.BTest#only.txt for details

FAILURES!!!
Tests found: 3, Tests run: 2,  Failures: 1</failure></testcase>`
	for _, tc := range []struct {
		name   string
		cases  []string
		class  string
		strict int
		says   []string
		never  []string
	}{
		{"every test", all, "", 0, []string{"app-emulator: the suite: 3 tests, 0 skipped, 0 failed; 0 of the 3 the sources declare not run", "passed   ATest.first", "passed   BTest.only"}, nil},
		{"one class", all[2:], "ch.example.BTest", 0, []string{"1 tests, 0 skipped, 0 failed; 0 of the 1 the sources declare not run"}, nil},
		{"a skip", []string{all[0], `<testcase name="second" classname="ch.example.ATest"><skipped/></testcase>`, all[2]}, "", 1, []string{"3 tests, 1 skipped", "skipped  ATest.second", "not every test the sources declare ran and passed"}, nil},
		{"one left out", all[:2], "", 1, []string{"2 tests, 0 skipped, 0 failed; 1 of the 3", "not run  BTest.only"}, nil},
		{"one more", append(append([]string{}, all...), pass("new", "ch.example.CTest")), "", 0, []string{"4 tests", "CTest.new (not among the tests the sources declare)"}, nil},
		{"a failure", []string{all[0], all[1], failure}, "", 1, []string{
			"3 tests, 0 skipped, 1 failed; 0 of the 3",
			"app-emulator:   failed   BTest.only\n",
			"app-emulator:            java.lang.AssertionError: the prompt stayed\n",
			"app-emulator:            at line 5\n",
		}, []string{"at line 6"}},
		{"a crash", []string{all[0], all[1], crash}, "", 1, []string{
			"3 tests, 0 skipped, 1 failed; 1 of the 3 the sources declare not run",
			"failed   android-test.testRunFailed (the instrumentation ended before the run did: a crash or a kill, the device's lines below)\n",
			"app-emulator:            Test instrumentation process crashed. Check ch.example.BTest#only.txt for details\n",
			"not run  BTest.only",
		}, []string{"testRunFailed (not among", "Tests found"}},
		{"none", nil, "", 1, []string{"0 tests", "not run  ATest.first"}, nil},
		{"no results", []string{"-"}, "", 1, []string{"no TEST-*.xml"}, nil},
	} {
		results := t.TempDir()
		if len(tc.cases) != 1 || tc.cases[0] != "-" {
			put(t, results, "fossDebug/TEST-x.xml", junit(tc.cases...))
		}
		for _, strict := range []bool{false, true} {
			args := []string{"-what", "the suite", "-tests", src}
			if tc.class != "" {
				args = append(args, "-class", tc.class)
			}
			want := 0
			if strict {
				args = append(args, "-strict")
				want = tc.strict
			}
			var out bytes.Buffer
			code := run(append(args, results), &out, &out)
			if code != want {
				t.Errorf("%s, strict %v: exit %d, want %d:\n%s", tc.name, strict, code, want, out.String())
			}
			for _, s := range tc.says {
				if !strings.Contains(out.String(), s) {
					t.Errorf("%s, strict %v: the output does not say %q:\n%s", tc.name, strict, s, out.String())
				}
			}
			for _, s := range tc.never {
				if strings.Contains(out.String(), s) {
					t.Errorf("%s, strict %v: the output says %q:\n%s", tc.name, strict, s, out.String())
				}
			}
		}
	}
	var out bytes.Buffer
	if code := run([]string{"-what", "x"}, &out, &out); code != 2 {
		t.Errorf("a wrong call: exit %d:\n%s", code, out.String())
	}
}
