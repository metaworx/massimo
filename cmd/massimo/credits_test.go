// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/credits"
)

// TestCreditsList checks massimo credits: a line for every entry of the
// embedded list, the one System info shows, with its version (Go's and
// each module's the binary's, builtVersions) and licence, "and others"
// where its files are under others too, "see the text" where its text is
// known by none, under a head; a module this binary does not link (on
// linux the two only the windows and darwin builds link) "-" and from
// another platform's build; then what an entry includes, whose texts are
// among its files: the OPAQUE client, MIT and others with its crates'
// texts.
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
		if e.Includes != "" && !strings.Contains(notes, e.Name+" includes "+e.Includes+" and the crates they are built with; their licence texts, collected for Massimo from the crates' archives, are among its files.") {
			t.Errorf("what %s includes is not said:\n%s", e.Name, notes)
		}
	}
	if g, _ := l.Find("Go"); g.Version != strings.TrimPrefix(runtime.Version(), "go") || strings.HasPrefix(g.Version, "go1") {
		t.Errorf("Go's version %q, want the binary's %q without its go", g.Version, runtime.Version())
	}
	built := builtVersions(t)
	printed, lineOf := map[string]string{}, map[string]string{}
	for _, line := range lines[1:] {
		if f := strings.Fields(line); len(f) > 1 {
			printed[f[0]], lineOf[f[0]] = f[1], line
		}
	}
	for _, e := range l.Entries {
		if e.Kind != credits.KindModule {
			continue
		}
		want, from := built[e.Name], "Go module"
		if want == "" {
			want, from = "-", "Go module, in another platform's build" // only another platform's build links it
		}
		if e.Path != "" {
			from = "Go module, its copy in " + e.Path
		}
		if printed[e.Name] != want {
			t.Errorf("%s: version %q printed, want the binary's %q", e.Name, printed[e.Name], want)
		}
		if !strings.HasSuffix(lineOf[e.Name], "  "+from) {
			t.Errorf("%s: %q, want it from %q", e.Name, lineOf[e.Name], from)
		}
	}
	if from := creditOrigin(credits.Entry{Name: "example.org/a", Kind: credits.KindModule}); from != "Go module, in another platform's build" {
		t.Errorf("a module without a version: from %q, want another platform's build", from)
	}
	if v := printed["golang.org/x/net"]; v == "" || v == "-" || v != built["golang.org/x/net"] {
		t.Errorf("golang.org/x/net: version %q printed, want the binary's %q", v, built["golang.org/x/net"])
	}
	for _, c := range []struct{ name, want string }{
		{"modernc.org/sqlite", "BSD-3-Clause and others"},
		{"gopkg.in/yaml.v3", "see the text"},
		{"@serenity-kit/opaque", "MIT and others"},
	} {
		if e, ok := l.Find(c.name); ok && entryLicence(e) != c.want {
			t.Errorf("%s's licence %q, want %q", c.name, entryLicence(e), c.want)
		}
	}
	if !strings.Contains(out.String(), "third_party/preact/preact") || !strings.Contains(out.String(), "Go module") {
		t.Errorf("where an entry comes from is not printed:\n%s", out.String())
	}
}

// builtVersions are the modules this binary was built with and their
// versions, as its build information names them: a module replaced by
// another at the replacement's version, one replaced by a directory
// (third_party/) at the version go.mod requires.
func builtVersions(t *testing.T) map[string]string {
	t.Helper()
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("the test binary has no build information")
	}
	out := map[string]string{}
	for _, d := range info.Deps {
		out[d.Path] = d.Version
		if r := d.Replace; r != nil && r.Version != "" && r.Version != "(devel)" {
			out[d.Path] = r.Version
		}
	}
	return out
}

// TestCreditsOne checks massimo credits <name>: what the entry
// includes, whose texts are among its files, then each file of its
// licence with the licence it is known by and its text, the OPAQUE
// client's crates' after its own, the name taken without regard to
// case;
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
	own := strings.Index(out.String(), "==> @serenity-kit/opaque: opaque.LICENSE (MIT) <==\n")
	crates := strings.Index(out.String(), "\n\n==> @serenity-kit/opaque: opaque.LICENSE-3RD-PARTY (see the text) <==\n")
	if own < 0 || crates < own || !strings.Contains(out.String()[crates:], "\n==> opaque-ke 4.0.1 (Apache-2.0 OR MIT) <==\n") {
		t.Errorf("the OPAQUE client's own licence, then its crates' texts, are not printed:\n%.300s", out.String())
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
