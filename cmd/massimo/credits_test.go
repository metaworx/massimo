// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/credits"
)

// TestCreditsList checks massimo credits: a line for every entry of the
// embedded list, the one System info shows, with its version (Go's the
// binary's) and licence, "and others" where its files are under others
// too, "see the text" where its text is known by none, under a head;
// then what an entry includes, whose texts are not reproduced.
func TestCreditsList(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := creditsCommand(nil, &out); err != nil {
		t.Fatal(err)
	}
	l, err := credits.Load()
	if err != nil {
		t.Fatal(err)
	}
	table, notes, _ := strings.Cut(out.String(), "\n\n")
	lines := strings.Split(strings.TrimSuffix(table, "\n"), "\n")
	if len(lines) != len(l.Entries)+1 || strings.Fields(lines[0])[0] != "NAME" {
		t.Fatalf("%d lines for %d entries:\n%s", len(lines), len(l.Entries), out.String())
	}
	for i, e := range l.Entries {
		f := strings.Fields(lines[i+1])
		version := e.Version
		if version == "" {
			version = "-"
		}
		if len(f) < 3 || f[0] != e.Name || f[1] != version || !strings.HasPrefix(strings.Join(f[2:], " "), entryLicence(e)) {
			t.Errorf("line %q, want %s %s %s", lines[i+1], e.Name, version, entryLicence(e))
		}
		if e.Includes != "" && !strings.Contains(notes, e.Name+" includes "+e.Includes+"; their own licence texts are not reproduced here.") {
			t.Errorf("what %s includes is not said:\n%s", e.Name, notes)
		}
	}
	if g, _ := l.Find("Go"); g.Version != credits.GoVersion() || strings.HasPrefix(g.Version, "go1") {
		t.Errorf("Go's version %q, want the binary's %q without its go", g.Version, runtime.Version())
	}
	for _, c := range []struct{ name, want string }{
		{"modernc.org/sqlite", "BSD-3-Clause and others"},
		{"gopkg.in/yaml.v3", "see the text"},
	} {
		if e, ok := l.Find(c.name); ok && entryLicence(e) != c.want {
			t.Errorf("%s's licence %q, want %q", c.name, entryLicence(e), c.want)
		}
	}
	if !strings.Contains(out.String(), "third_party/preact/preact") || !strings.Contains(out.String(), "Go module") {
		t.Errorf("where an entry comes from is not printed:\n%s", out.String())
	}
}

// TestCreditsOne checks massimo credits <name>: what the entry
// includes, whose texts are not reproduced, then each file of its
// licence with the licence it is known by and its text, the name taken
// without regard to case;
// a name the list does not hold is an error that names the list's
// command, and two names are one too many.
func TestCreditsOne(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := creditsCommand([]string{"GOLANG.ORG/X/SYS"}, &out); err != nil {
		t.Fatal(err)
	}
	l, _ := credits.Load()
	e, _ := l.Find("golang.org/x/sys")
	for _, f := range e.Files {
		if !strings.Contains(out.String(), "==> golang.org/x/sys: "+f.Name+" ("+fileLicence(f)+") <==\n"+f.Text) {
			t.Errorf("%s's text is not printed:\n%s", f.Name, out.String())
		}
	}
	if len(e.Files) != 2 || !strings.Contains(out.String(), "(BSD-3-Clause) <==") || !strings.Contains(out.String(), "PATENTS (notice) <==") {
		t.Errorf("golang.org/x/sys: %d files, want LICENSE and PATENTS with their licence and notice", len(e.Files))
	}
	out.Reset()
	if err := creditsCommand([]string{"@serenity-kit/opaque"}, &out); err != nil {
		t.Fatal(err)
	}
	if o, _ := l.Find("@serenity-kit/opaque"); o.Includes == "" || !strings.HasPrefix(out.String(), includesLine(o)+"\n\n==> ") {
		t.Errorf("what the OPAQUE client includes is not said first:\n%.300s", out.String())
	}
	err := creditsCommand([]string{"no-such-thing"}, &out)
	if err == nil || !strings.Contains(err.Error(), `"no-such-thing" is not in the list`) || !strings.Contains(err.Error(), "massimo credits lists the names") {
		t.Errorf("an unknown name: %v", err)
	}
	if err := creditsCommand([]string{"Go", "Massimo"}, &out); err == nil {
		t.Error("two names taken")
	}
}

// TestCreditsOnAFreshBox runs massimo credits as a binary of its own
// (the test binary, TestMain) in an empty directory with no settings
// file and an empty data directory: it lists, exits 0 and writes
// nothing; an unknown name exits 1, as a wrong argument to the other
// commands does.
func TestCreditsOnAFreshBox(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs the test binary")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir, data := t.TempDir(), t.TempDir()
	run := func(args ...string) (string, int) {
		cmd := exec.Command(self, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), runMainVar+"=1", "MASSIMO_ENV_FILE=/dev/null", "MASSIMO_DATA="+data)
		out, err := cmd.Output()
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return string(out), exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return string(out), 0
	}
	out, code := run("credits")
	if code != 0 || !strings.Contains(out, "Massimo") || !strings.Contains(out, "AGPL-3.0-or-later") {
		t.Fatalf("massimo credits: exit %d\n%s", code, out)
	}
	if _, code := run("credits", "no-such-thing"); code != 1 {
		t.Errorf("massimo credits no-such-thing: exit %d, want 1", code)
	}
	for _, d := range []string{dir, data} {
		if left, _ := os.ReadDir(d); len(left) != 0 {
			t.Errorf("%s holds %d files after massimo credits", d, len(left))
		}
	}
}
