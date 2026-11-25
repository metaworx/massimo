// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package credits

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"testing"
)

// root is the repository's top, from this package's directory.
const root = "../.."

// TestEmbeddedList checks the list the binary embeds: it reads, its
// names differ without regard to case, Massimo comes first under its
// licence and Go second, and every entry has the texts of its licence.
func TestEmbeddedList(t *testing.T) {
	l, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Entries) < 2 {
		t.Fatalf("%d entries, want Massimo, Go and what the binary links", len(l.Entries))
	}
	if e := l.Entries[0]; e.Name != "Massimo" || e.Kind != KindSelf || e.Licence != "AGPL-3.0-or-later" {
		t.Errorf("first entry %s (%s, %s), want Massimo (self, AGPL-3.0-or-later)", e.Name, e.Kind, e.Licence)
	}
	if e := l.Entries[1]; e.Name != "Go" || e.Kind != KindGo {
		t.Errorf("second entry %s (%s), want Go", e.Name, e.Kind)
	}
	names := map[string]bool{}
	for _, e := range l.Entries {
		k := strings.ToLower(e.Name)
		if names[k] {
			t.Errorf("%s: a second entry of that name", e.Name)
		}
		names[k] = true
		for _, f := range e.Files {
			if strings.TrimSpace(f.Text) == "" {
				t.Errorf("%s: %s is empty", e.Name, f.Name)
			}
		}
		if !slices.Contains([]string{KindSelf, KindGo, KindModule, KindVendored}, e.Kind) {
			t.Errorf("%s: kind %q", e.Name, e.Kind)
		}
	}
	if e, ok := l.Find("PREACT"); !ok || e.Kind != KindVendored {
		t.Errorf("Find(PREACT) = %v, %v: want Preact's entry, found without regard to case", e.Name, ok)
	}
	if _, ok := l.Find("nope"); ok {
		t.Error("Find(nope) found an entry")
	}
}

// TestParseRefuses checks that an entry without a name or a licence's
// file, and a field the list does not have, are refused.
func TestParseRefuses(t *testing.T) {
	for _, in := range []string{
		`{"entries": [{"name": "", "licence": "MIT", "kind": "module", "files": [{"name": "LICENSE", "text": "x"}]}]}`,
		`{"entries": [{"name": "a", "licence": "MIT", "kind": "module", "files": []}]}`,
		`{"entries": [], "other": 1}`,
	} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%s) took it", in)
		}
	}
}

// crossLine is a target of the Makefile's cross builds: GOOS, GOARCH,
// GOARM and then $(CROSS).
var crossLine = regexp.MustCompile(`^\s*GOOS=(\S+) GOARCH=(\S+)(?: GOARM=(\S+))? \$\(CROSS\)`)

// crossTargets are the platforms the Makefile's cross builds, as
// GOOS, GOARCH and GOARM, read from the Makefile; a line with $(CROSS)
// in another form fails the test.
func crossTargets(t *testing.T) [][3]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	var out [][3]string
	for sc := bufio.NewScanner(bytes.NewReader(b)); sc.Scan(); {
		if m := crossLine.FindStringSubmatch(sc.Text()); m != nil {
			out = append(out, [3]string{m[1], m[2], m[3]})
		} else if strings.Contains(sc.Text(), "$(CROSS)") {
			t.Fatalf("a line of the Makefile builds with $(CROSS) in a form this test does not read: %q", sc.Text())
		}
	}
	if len(out) == 0 {
		t.Fatal("no cross target found in the Makefile")
	}
	return out
}

// linked are the modules ./cmd/massimo links on every platform of the
// Makefile's cross, by path, from go list -deps, which reads the modules
// from the module cache and downloads one missing there where GOPROXY
// allows; with GOPROXY=off, go mod download fills it.
func linked(t *testing.T) map[string]bool {
	t.Helper()
	targets := crossTargets(t)
	outs := make([][]byte, len(targets))
	errs := make([]error, len(targets))
	var wg sync.WaitGroup
	for i, tg := range targets {
		wg.Go(func() {
			cmd := exec.Command("go", "list", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}}{{end}}{{end}}", "./cmd/massimo")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GOOS="+tg[0], "GOARCH="+tg[1], "GOARM="+tg[2], "CGO_ENABLED=0")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			outs[i], errs[i] = cmd.Output()
			if errs[i] != nil {
				errs[i] = &listError{tg, errs[i], stderr.String()}
			}
		})
	}
	wg.Wait()
	mods := map[string]bool{}
	for i := range targets {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		for line := range strings.Lines(string(outs[i])) {
			if line = strings.TrimSpace(line); line != "" {
				mods[line] = true
			}
		}
	}
	return mods
}

// listError is go list's failure on a platform.
type listError struct {
	target [3]string
	err    error
	stderr string
}

func (e *listError) Error() string {
	return "go list for " + strings.Join(e.target[:], "/") + ": " + e.err.Error() + "\n" + e.stderr +
		"(a module missing from the module cache with GOPROXY=off: go mod download)"
}

// moduleProblems are the modules the binary links without an entry,
// and the modules' entries the binary does not link, by path: the
// version is the binary's own (built), which the file leaves out.
func moduleProblems(l List, mods map[string]bool) []string {
	var out []string
	listed := map[string]bool{}
	for _, e := range l.Entries {
		if e.Kind != KindModule {
			continue
		}
		listed[e.Name] = true
		if !mods[e.Name] {
			out = append(out, "module "+e.Name+" has an entry, but the binary does not link it")
		}
	}
	for k := range mods {
		if !listed[k] {
			out = append(out, "module "+k+" is linked, but has no entry")
		}
	}
	slices.Sort(out)
	return out
}

// licenceExt is the end of a licence file's name beside a vendored
// file, after its stem's dot (opaque.LICENSE).
var licenceExt = regexp.MustCompile(`(?i)^(licen[cs]e|copying|notice|patents)`)

// vendoredProblems are the directories of third_party/ and the files of
// the UI's vendor/ under dir without an entry: a directory needs one at
// it or below it (a module's copy, Preact's three packages), a file of
// vendor/ but its README.md and the licence files beside the others one
// at it.
func vendoredProblems(l List, dir string) []string {
	var out []string
	has := func(p string, below bool) bool {
		return slices.ContainsFunc(l.Entries, func(e Entry) bool {
			return e.Path == p || (below && strings.HasPrefix(e.Path, p+"/"))
		})
	}
	dirs, _ := os.ReadDir(filepath.Join(dir, "third_party"))
	for _, d := range dirs {
		if p := "third_party/" + d.Name(); d.IsDir() && !has(p, true) {
			out = append(out, p+" has no entry")
		}
	}
	files, _ := os.ReadDir(filepath.Join(dir, "internal", "server", "ui", "vendor"))
	for _, f := range files {
		name := f.Name()
		_, ext, _ := strings.Cut(name, ".")
		licence := licenceExt.MatchString(ext)
		if p := "internal/server/ui/vendor/" + name; !f.IsDir() && name != "README.md" && !licence && !has(p, false) {
			out = append(out, p+" has no entry")
		}
	}
	return out
}

// TestEverythingIncludedHasAnEntry checks the list against what the
// binary includes: every module it links on every platform of the
// Makefile's cross, whatever its version, every directory of
// third_party/ and every file of the UI's vendor/; a module listed that
// it does not link fails too. tools/credits' TestGeneratedFileCurrent
// checks the texts.
func TestEverythingIncludedHasAnEntry(t *testing.T) {
	l, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	problems := append(moduleProblems(l, linked(t)), vendoredProblems(l, root)...)
	for _, p := range problems {
		t.Error(p)
	}
	if len(problems) > 0 {
		t.Log("internal/credits/credits.json has fallen behind: run make credits (go run ./tools/credits) from the repository's top")
	}
}

// TestFallenBehindFails checks that the checks above fail where the
// list has fallen behind: a module linked without an entry, one listed
// and not linked, and in a copy of the places, a new directory of
// third_party/ and a new file of vendor/; a module listed and linked
// passes, as the file names no version.
func TestFallenBehindFails(t *testing.T) {
	l := List{Entries: []Entry{
		{Name: "example.org/a", Kind: KindModule},
		{Name: "example.org/gone", Kind: KindModule},
		{Name: "kept", Kind: KindVendored, Path: "third_party/kept/inner"},
		{Name: "a.js", Kind: KindVendored, Path: "internal/server/ui/vendor/a.js"},
	}}
	got := moduleProblems(l, map[string]bool{"example.org/a": true, "example.org/new": true})
	want := []string{
		"module example.org/gone has an entry, but the binary does not link it",
		"module example.org/new is linked, but has no entry",
	}
	if !slices.Equal(got, want) {
		t.Errorf("module problems:\n%q\nwant\n%q", got, want)
	}
	dir := t.TempDir()
	for _, p := range []string{"third_party/kept/inner/LICENSE", "internal/server/ui/vendor/README.md", "internal/server/ui/vendor/a.js", "internal/server/ui/vendor/a.LICENSE"} {
		write(t, filepath.Join(dir, p), "x")
	}
	if got := vendoredProblems(l, dir); len(got) != 0 {
		t.Fatalf("problems before the new files: %q", got)
	}
	write(t, filepath.Join(dir, "third_party/made-up/LICENSE"), "x")
	write(t, filepath.Join(dir, "internal/server/ui/vendor/b.js"), "x")
	got = vendoredProblems(l, dir)
	want = []string{"third_party/made-up has no entry", "internal/server/ui/vendor/b.js has no entry"}
	if !slices.Equal(got, want) {
		t.Errorf("vendored problems:\n%q\nwant\n%q", got, want)
	}
}

// write writes text to path, making its directories.
func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestVersionsAreTheBinarys checks that Go's entry and the modules',
// which the committed file leaves without a version, carry the running
// binary's in what the server serves and the command prints: Go's
// without its "go" in front, a module's the one its build information
// names, none where it names none (this package's test binary links no
// module); and that nothing else of the file changes.
func TestVersionsAreTheBinarys(t *testing.T) {
	file, err := Parse(embedded)
	if err != nil {
		t.Fatal(err)
	}
	l, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Entries) != len(file.Entries) {
		t.Fatalf("%d entries served, %d in the file", len(l.Entries), len(file.Entries))
	}
	deps := map[string]string{}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, d := range info.Deps {
			deps[d.Path] = d.Version
		}
	}
	for i, e := range l.Entries {
		want := file.Entries[i]
		switch want.Kind {
		case KindGo:
			if want.Version != "" {
				t.Errorf("Go's version in the file: %q, want none", want.Version)
			}
			want.Version = goVersionOf(runtime.Version())
		case KindModule:
			if want.Version != "" {
				t.Errorf("%s's version in the file: %q, want none", want.Name, want.Version)
			}
			want.Version = deps[want.Name]
		}
		if e.Version != want.Version {
			t.Errorf("%s: version %q served, want %q", want.Name, e.Version, want.Version)
		} else if !reflect.DeepEqual(e, want) {
			t.Errorf("%s: served otherwise than the file holds it: %+v", want.Name, e)
		}
	}
	if g, _ := l.Find("Go"); g.Version != strings.TrimPrefix(runtime.Version(), "go") {
		t.Errorf("Go's version %q, want %q without its go", g.Version, runtime.Version())
	}
}

// TestBuilt checks the versions a binary fills in from its build
// information: Go's without its "go"; a module's its own, a module
// replaced by another's the replacement's, one replaced by a directory
// the version go.mod requires; none for a module the information does
// not name, even where the file had one, and none for any module
// without information; Massimo's and a vendored entry's as the file has
// them.
func TestBuilt(t *testing.T) {
	file := func() List {
		return List{Entries: []Entry{
			{Name: "Massimo", Kind: KindSelf},
			{Name: "Go", Kind: KindGo},
			{Name: "example.org/a", Kind: KindModule},
			{Name: "example.org/fork", Kind: KindModule},
			{Name: "example.org/copy", Kind: KindModule, Path: "third_party/copy"},
			{Name: "example.org/elsewhere", Kind: KindModule, Version: "v9.9.9"},
			{Name: "preact", Kind: KindVendored, Version: "10.29.0"},
		}}
	}
	info := &debug.BuildInfo{Deps: []*debug.Module{
		{Path: "example.org/a", Version: "v1.2.0"},
		{Path: "example.org/fork", Version: "v1.0.0", Replace: &debug.Module{Path: "example.org/other", Version: "v1.0.1-patched"}},
		{Path: "example.org/copy", Version: "v0.5.0", Replace: &debug.Module{Path: "./third_party/copy", Version: "(devel)"}},
		{Path: "example.org/unlisted", Version: "v3.0.0"},
	}}
	versions := func(l List) []string {
		var out []string
		for _, e := range l.Entries {
			out = append(out, e.Name+" "+e.Version)
		}
		return out
	}
	got := versions(built(file(), "go1.26.8", info))
	want := []string{"Massimo ", "Go 1.26.8", "example.org/a v1.2.0", "example.org/fork v1.0.1-patched", "example.org/copy v0.5.0",
		"example.org/elsewhere ", "preact 10.29.0"}
	if !slices.Equal(got, want) {
		t.Errorf("versions\n%q\nwant\n%q", got, want)
	}
	got = versions(built(file(), "go1.26.8", nil))
	want = []string{"Massimo ", "Go 1.26.8", "example.org/a ", "example.org/fork ", "example.org/copy ", "example.org/elsewhere ", "preact 10.29.0"}
	if !slices.Equal(got, want) {
		t.Errorf("versions without build information\n%q\nwant\n%q", got, want)
	}
}

// TestGoVersionOf checks Go's version without the "go" in front of its
// digits, a development toolchain's as it is.
func TestGoVersionOf(t *testing.T) {
	for in, want := range map[string]string{
		"go1.26.8":  "1.26.8",
		"go1.27rc1": "1.27rc1",
		"devel go1.27-0123456789ab Tue Nov 24 10:00:00 2026 +0000": "devel go1.27-0123456789ab Tue Nov 24 10:00:00 2026 +0000",
		"gopher": "gopher",
	} {
		if got := goVersionOf(in); got != want {
			t.Errorf("goVersionOf(%q) = %q, want %q", in, got, want)
		}
	}
}
