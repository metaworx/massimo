// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/credits"
)

// The tests' licences, shortened to what classify reads.
const (
	mitText = "MIT License\n\nCopyright (c) 2026 Someone\n\nPermission is hereby granted, free of charge, to any person obtaining a copy\n" +
		"of this software and associated documentation files (the \"Software\"), to deal\nin the Software without restriction ...\n\n" +
		"The above copyright notice and this permission notice shall be included in all\ncopies or substantial portions of the Software.\n"
	bsdBody = "Redistribution and use in source and binary forms, with or without\nmodification, are permitted provided that the following conditions are\nmet:\n\n" +
		"   * Redistributions of source code must retain the above copyright\nnotice, this list of conditions and the following disclaimer.\n" +
		"   * Redistributions in binary form must reproduce the above\ncopyright notice, this list of conditions and the following disclaimer.\n"
	bsdText = "Copyright 2009 The Go Authors.\n\n" + bsdBody +
		"   * Neither the name of Google LLC nor the names of its\ncontributors may be used to endorse or promote products derived from\nthis software without specific prior written permission.\n"
)

// TestTargetsAreTheMakefiles checks that the platforms whose modules
// are listed are those the Makefile's cross builds, in its order; a
// line with $(CROSS) in another form fails, so that no target is left
// out unseen.
func TestTargetsAreTheMakefiles(t *testing.T) {
	b, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	line := regexp.MustCompile(`^\s*GOOS=(\S+) GOARCH=(\S+)(?: GOARM=(\S+))? \$\(CROSS\)`)
	var got []target
	for sc := bufio.NewScanner(bytes.NewReader(b)); sc.Scan(); {
		if m := line.FindStringSubmatch(sc.Text()); m != nil {
			got = append(got, target{m[1], m[2], m[3]})
		} else if strings.Contains(sc.Text(), "$(CROSS)") {
			t.Errorf("a line of the Makefile builds with $(CROSS) in a form this test does not read: %q", sc.Text())
		}
	}
	if !slices.Equal(got, targets) {
		t.Errorf("the Makefile builds %v, the list is made for %v", got, targets)
	}
}

// TestGeneratedFileCurrent checks that internal/credits/credits.json is
// what make credits writes for this checkout, texts included, that two
// runs write the same bytes, and that it names no module's version: a
// module of another version, as an update that keeps its licence's
// texts, writes the same file, a module more another.
func TestGeneratedFileCurrent(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	s := repository(root)
	mods, err := s.modules()
	if err != nil {
		t.Fatal(err)
	}
	s.modules = func() ([]module, error) { return mods, nil }
	want, _, err := generate(s)
	if err != nil {
		t.Fatal(err)
	}
	again, _, err := generate(s)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, again) {
		t.Fatal("two runs over the same checkout differ")
	}
	if bytes.Contains(want, []byte(root)) {
		t.Error("the list holds the checkout's absolute path")
	}
	have, err := os.ReadFile(filepath.Join(root, outPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(have, want) {
		t.Fatalf("%s is not current: run make credits (go run ./tools/credits) from the repository's top", outPath)
	}
	l, err := credits.Parse(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range l.Entries {
		if (e.Kind == credits.KindModule || e.Kind == credits.KindGo) && e.Version != "" {
			t.Errorf("%s: version %s in the file, which the binary fills in", e.Name, e.Version)
		}
	}
	if len(mods) == 0 {
		t.Fatal("no module linked")
	}
	bumped := slices.Clone(mods)
	bumped[0].version += "-bumped"
	s.modules = func() ([]module, error) { return bumped, nil }
	if got, _, err := generate(s); err != nil || !bytes.Equal(got, have) {
		t.Errorf("%s at %s: another file (%v), want the same", bumped[0].path, bumped[0].version, err)
	}
	added := append(slices.Clone(mods), module{path: "example.org/added", version: "v1.0.0", dir: mods[0].dir})
	s.modules = func() ([]module, error) { return added, nil }
	if got, _, err := generate(s); err != nil || bytes.Equal(got, have) {
		t.Errorf("a module more: the same file (%v), want another", err)
	}
}

// tree is a made-up repository in a temporary directory: Massimo's
// LICENSE and go.mod, Go's directory, a module, one directory of
// third_party/ and a file of the UI's vendor/, each with its licence.
type tree struct {
	root string
	mods []module
}

// newTree writes the made-up repository.
func newTree(t *testing.T) *tree {
	t.Helper()
	dir := t.TempDir()
	tr := &tree{root: filepath.Join(dir, "repo")}
	tr.write(t, "go.mod", "module example.org/m\n\ngo 1.26.0\n")
	if err := os.MkdirAll(filepath.Join(tr.root, "internal", "credits"), 0o755); err != nil {
		t.Fatal(err)
	}
	tr.write(t, "LICENSE", "                    GNU AFFERO GENERAL PUBLIC LICENSE\n                       Version 3, 19 November 2007\n")
	tr.write(t, "../goroot/LICENSE", bsdText)
	tr.write(t, "../goroot/PATENTS", "Additional IP Rights Grant (Patents)\n")
	tr.write(t, "../cache/example.org/a@v1.0.0/LICENSE", mitText)
	tr.write(t, "../cache/example.org/a@v1.0.0/LICENSE-3RD-PARTY.md", "# Others\r\n")
	tr.write(t, "third_party/kept/LICENSE", mitText)
	tr.write(t, "third_party/kept/MASSIMO.md", "# Kept\n\nA note, no table.\n")
	tr.write(t, "internal/server/ui/vendor/README.md", "| File | Package | Version | Licence |\n|---|---|---|---|\n| `x.js` | `@x/x` | 2.0.0 | MIT, `x.LICENSE` |\n")
	tr.write(t, "internal/server/ui/vendor/x.js", "x")
	tr.write(t, "internal/server/ui/vendor/x.LICENSE", mitText)
	tr.mods = []module{{path: "example.org/a", version: "v1.0.0", dir: filepath.Join(dir, "cache", "example.org", "a@v1.0.0")}}
	return tr
}

// write writes text to the tree's file at path, making its directories.
func (tr *tree) write(t *testing.T, path, text string) {
	t.Helper()
	p := filepath.Join(tr.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// source is the tree as the list is made from it.
func (tr *tree) source() source {
	return source{
		root:    tr.root,
		goRoot:  func() (string, error) { return filepath.Join(tr.root, "..", "goroot"), nil },
		modules: func() ([]module, error) { return slices.Clone(tr.mods), nil },
	}
}

// list is the tree's written list.
func (tr *tree) list(t *testing.T) credits.List {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tr.root, outPath))
	if err != nil {
		t.Fatal(err)
	}
	l, err := credits.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// TestMadeUpTree checks the list of a made-up repository: the entries in
// their order, named, versioned, with their licences, Go's and the
// module's without a version, which the binary fills in, a module with a
// file under no licence known mixed, the licences' files each with its
// own, line ends as LF; then -check passes.
func TestMadeUpTree(t *testing.T) {
	tr := newTree(t)
	if err := run(tr.root, false, tr.source(), io.Discard); err != nil {
		t.Fatal(err)
	}
	l := tr.list(t)
	var got []string
	for _, e := range l.Entries {
		var files []string
		for _, f := range e.Files {
			if f.Notice {
				files = append(files, f.Name+":notice")
			} else {
				files = append(files, f.Name+":"+f.Licence)
			}
		}
		got = append(got, strings.Join([]string{e.Name, e.Version, e.Licence, strconv.FormatBool(e.Mixed), e.Kind, e.Path, strings.Join(files, ",")}, "|"))
	}
	want := []string{
		"Massimo||AGPL-3.0-or-later|false|self||LICENSE:AGPL-3.0",
		"Go||BSD-3-Clause|false|go||LICENSE:BSD-3-Clause,PATENTS:notice",
		"example.org/a||MIT|true|module||LICENSE:MIT,LICENSE-3RD-PARTY.md:",
		"kept||MIT|false|vendored|third_party/kept|LICENSE:MIT",
		"@x/x|2.0.0|MIT|false|vendored|internal/server/ui/vendor/x.js|x.LICENSE:MIT",
	}
	if !slices.Equal(got, want) {
		t.Errorf("entries:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if e, _ := l.Find("example.org/a"); len(e.Files) == 2 && e.Files[1].Text != "# Others\n" {
		t.Errorf("a CRLF text is %q, want its lines ended by LF", e.Files[1].Text)
	}
	if err := run(tr.root, true, tr.source(), io.Discard); err != nil {
		t.Errorf("-check after the write: %v", err)
	}
}

// TestNewDirectoryTakenUp checks that a directory added to third_party/,
// as a later block adds a list, is taken up with no change to the code:
// -check fails until make credits has run, and its entry is there after.
func TestNewDirectoryTakenUp(t *testing.T) {
	tr := newTree(t)
	if err := run(tr.root, false, tr.source(), io.Discard); err != nil {
		t.Fatal(err)
	}
	tr.write(t, "third_party/made-up-list/LICENSE", "Attribution 4.0 International\n\nCreative Commons Corporation ...\n")
	tr.write(t, "third_party/made-up-list/MASSIMO.md", "# A made-up list\n")
	err := run(tr.root, true, tr.source(), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "not current: run make credits") {
		t.Fatalf("-check with a new directory: %v, want not current", err)
	}
	if err := run(tr.root, false, tr.source(), io.Discard); err != nil {
		t.Fatal(err)
	}
	e, ok := tr.list(t).Find("made-up-list")
	if !ok || e.Path != "third_party/made-up-list" || e.Kind != credits.KindVendored || e.Licence != "CC-BY-4.0" {
		t.Errorf("the new directory's entry: %+v, %v", e, ok)
	}
	if err := run(tr.root, true, tr.source(), io.Discard); err != nil {
		t.Errorf("-check after make credits: %v", err)
	}
}

// TestChangedModulesFailCheck checks that -check fails where the modules
// the binary links have changed, one more or one less, and passes where
// one has another version whose licence's texts are the same: the
// binary fills in its version. Another text fails it.
func TestChangedModulesFailCheck(t *testing.T) {
	tr := newTree(t)
	if err := run(tr.root, false, tr.source(), io.Discard); err != nil {
		t.Fatal(err)
	}
	base := tr.mods
	tr.mods = []module{{path: base[0].path, version: "v1.0.1", dir: base[0].dir}}
	if err := run(tr.root, true, tr.source(), io.Discard); err != nil {
		t.Errorf("another version: -check gave %v, want current", err)
	}
	b := module{path: "example.org/b", version: "v0.1.0", dir: base[0].dir}
	for name, mods := range map[string][]module{
		"one more": {base[0], b},
		"one less": {},
	} {
		tr.mods = mods
		if err := run(tr.root, true, tr.source(), io.Discard); err == nil || !strings.Contains(err.Error(), "not current") {
			t.Errorf("%s: -check gave %v, want not current", name, err)
		}
	}
	tr.mods = base
	tr.write(t, "../cache/example.org/a@v1.0.0/LICENSE", mitText+"\nA sentence more.\n")
	if err := run(tr.root, true, tr.source(), io.Discard); err == nil || !strings.Contains(err.Error(), "not current") {
		t.Errorf("another licence text: -check gave %v, want not current", err)
	}
}

// TestNoLicenceStops checks that an entry without a licence file stops
// the generator, naming it and where it looked, and leaves the file as
// it was; so does a directory of third_party/ without a note.
func TestNoLicenceStops(t *testing.T) {
	tr := newTree(t)
	if err := run(tr.root, false, tr.source(), io.Discard); err != nil {
		t.Fatal(err)
	}
	before := tr.list(t)
	tr.write(t, "third_party/bare/MASSIMO.md", "# Bare\n")
	tr.write(t, "third_party/bare/data.txt", "x")
	err := run(tr.root, false, tr.source(), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "bare: no licence file") {
		t.Errorf("a directory without a licence: %v", err)
	}
	if err := os.Remove(filepath.Join(tr.root, "third_party/bare/MASSIMO.md")); err != nil {
		t.Fatal(err)
	}
	tr.write(t, "third_party/bare/LICENSE", mitText)
	if err := run(tr.root, false, tr.source(), io.Discard); err == nil || !strings.Contains(err.Error(), "has no note") {
		t.Errorf("a directory without a note: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(tr.root, "third_party/bare")); err != nil {
		t.Fatal(err)
	}
	tr.write(t, "internal/server/ui/vendor/y.js", "y")
	if err := run(tr.root, false, tr.source(), io.Discard); err == nil || !strings.Contains(err.Error(), "y.js: no licence file") {
		t.Errorf("a vendored file without a licence: %v", err)
	}
	if err := os.Remove(filepath.Join(tr.root, "internal/server/ui/vendor/y.js")); err != nil {
		t.Fatal(err)
	}
	tr.mods = append(tr.mods, module{path: "example.org/bare", version: "v1.0.0", dir: filepath.Join(tr.root, "third_party")})
	if err := run(tr.root, false, tr.source(), io.Discard); err == nil || !strings.Contains(err.Error(), "example.org/bare: no licence file") {
		t.Errorf("a module without a licence: %v", err)
	}
	if after := tr.list(t); len(after.Entries) != len(before.Entries) {
		t.Errorf("the file changed: %d entries, %d before", len(after.Entries), len(before.Entries))
	}
}

// TestGoLicenceFromXSys checks that a Go whose directory has no LICENSE
// (Debian's) takes Go's licence from golang.org/x/sys, and that without
// that module either the generator stops.
func TestGoLicenceFromXSys(t *testing.T) {
	tr := newTree(t)
	for _, f := range []string{"LICENSE", "PATENTS"} {
		if err := os.Remove(filepath.Join(tr.root, "..", "goroot", f)); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := generate(tr.source()); err == nil || !strings.Contains(err.Error(), "Go: no licence file") {
		t.Errorf("no Go licence and no x/sys: %v", err)
	}
	sys := filepath.Join(tr.root, "..", "cache", "golang.org", "x", "sys@v0.50.0")
	tr.write(t, "../cache/golang.org/x/sys@v0.50.0/LICENSE", bsdText)
	tr.write(t, "../cache/golang.org/x/sys@v0.50.0/PATENTS", "Additional IP Rights Grant (Patents)\n")
	tr.mods = append(tr.mods, module{path: "golang.org/x/sys", version: "v0.50.0", dir: sys})
	if err := run(tr.root, false, tr.source(), io.Discard); err != nil {
		t.Fatal(err)
	}
	if e, _ := tr.list(t).Find("Go"); len(e.Files) != 2 || e.Files[0].Text != bsdText || e.Licence != "BSD-3-Clause" || e.Version != "" {
		t.Errorf("Go's entry: %+v", e)
	}
}

// TestModuleReplacedByACopy checks that a module go.mod replaces with a
// directory of third_party/ is one entry, the module's, at that path.
func TestModuleReplacedByACopy(t *testing.T) {
	tr := newTree(t)
	tr.write(t, "third_party/patched/LICENSE", mitText)
	tr.write(t, "third_party/patched/MASSIMO.md", "# Patched\n")
	tr.mods = append(tr.mods, module{path: "example.org/patched", version: "v0.5.0", dir: filepath.Join(tr.root, "third_party", "patched"), local: "third_party/patched"})
	if err := run(tr.root, false, tr.source(), io.Discard); err != nil {
		t.Fatal(err)
	}
	l := tr.list(t)
	if _, ok := l.Find("patched"); ok {
		t.Error("the copy has an entry of its own")
	}
	if e, ok := l.Find("example.org/patched"); !ok || e.Path != "third_party/patched" || e.Kind != credits.KindModule {
		t.Errorf("the module's entry: %+v, %v", e, ok)
	}
}

// licencesToday are the licences the files of the list are known by
// today, by entry and file; an entry not here is not checked. yaml.v3's
// LICENSE holds both the MIT licence and Apache's notice, and the OPAQUE
// client's opaque.LICENSE-3RD-PARTY the texts of many: each is named by
// none.
var licencesToday = map[string]map[string]string{
	"Massimo":                          {"LICENSE": "AGPL-3.0"},
	"Go":                               {"LICENSE": "BSD-3-Clause"},
	"filippo.io/edwards25519":          {"LICENSE": "BSD-3-Clause"},
	"filippo.io/nistec":                {"LICENSE": "BSD-3-Clause"},
	"github.com/bytemare/ecc":          {"LICENSE": "MIT"},
	"github.com/bytemare/hash":         {"LICENSE": "MIT"},
	"github.com/bytemare/hash2curve":   {"LICENSE": "MIT"},
	"github.com/bytemare/ksf":          {"LICENSE": "MIT"},
	"github.com/bytemare/opaque":       {"LICENSE": "MIT"},
	"github.com/bytemare/secp256k1":    {"LICENSE": "MIT"},
	"github.com/dustin/go-humanize":    {"LICENSE": "MIT"},
	"github.com/google/uuid":           {"LICENSE": "BSD-3-Clause"},
	"github.com/gtank/ristretto255":    {"LICENSE": "BSD-3-Clause"},
	"github.com/mattn/go-isatty":       {"LICENSE": "MIT"},
	"github.com/ncruces/go-strftime":   {"LICENSE": "MIT"},
	"github.com/remyoudompheng/bigfft": {"LICENSE": "BSD-3-Clause"},
	"golang.org/x/crypto":              {"LICENSE": "BSD-3-Clause"},
	"golang.org/x/net":                 {"LICENSE": "BSD-3-Clause"},
	"golang.org/x/sys":                 {"LICENSE": "BSD-3-Clause"},
	"golang.org/x/term":                {"LICENSE": "BSD-3-Clause"},
	"golang.org/x/text":                {"LICENSE": "BSD-3-Clause"},
	"gopkg.in/yaml.v3":                 {"LICENSE": ""},
	"modernc.org/libc":                 {"LICENSE": "BSD-3-Clause", "LICENSE-3RD-PARTY.md": ""},
	"modernc.org/mathutil":             {"LICENSE": "BSD-3-Clause"},
	"modernc.org/memory":               {"LICENSE": "BSD-3-Clause", "LICENSE-GO": "BSD-3-Clause", "LICENSE-LOGO": "", "LICENSE-MMAP-GO": "BSD-3-Clause"},
	"modernc.org/sqlite":               {"LICENSE": "BSD-3-Clause", "LICENSE-3RD-PARTY.md": "", "LICENSE-SQLITE": "", "LICENSE-SQLITE_VEC": "MIT"},
	"preact":                           {"LICENSE": "MIT"},
	"@preact/signals":                  {"LICENSE": "MIT"},
	"@preact/signals-core":             {"LICENSE": "MIT"},
	"@tabler/icons":                    {"LICENSE": "MIT"},
	"@serenity-kit/opaque":             {"opaque.LICENSE": "MIT", "opaque.LICENSE-3RD-PARTY": ""},
}

// TestClassifyToday checks classify on the full texts the list holds:
// every licence file of today's entries gets the name it has, each file
// licencesToday names is among its entry's, and the repository's own
// LICENSE is the AGPL-3.0's.
func TestClassifyToday(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", outPath))
	if err != nil {
		t.Fatal(err)
	}
	l, err := credits.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range l.Entries {
		want, ok := licencesToday[e.Name]
		if !ok {
			continue
		}
		seen++
		for _, f := range e.Files {
			w, ok := want[f.Name]
			if f.Notice {
				continue
			}
			if !ok {
				t.Errorf("%s: %s is a licence file the test does not know", e.Name, f.Name)
			}
			if got := classify(f.Text); got != w {
				t.Errorf("%s: classify(%s) = %q, want %q", e.Name, f.Name, got, w)
			}
		}
		for name := range want {
			if !slices.ContainsFunc(e.Files, func(f credits.File) bool { return f.Name == name }) {
				t.Errorf("%s: %s is not among its licence files", e.Name, name)
			}
		}
	}
	if seen < len(licencesToday)/2 {
		t.Errorf("only %d of the %d entries of today are in the list", seen, len(licencesToday))
	}
	own, err := os.ReadFile("../../LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	if got := classify(string(own)); got != "AGPL-3.0" {
		t.Errorf("the repository's LICENSE is %q, want AGPL-3.0", got)
	}
}

// TestClassifyCommonLicenses checks classify on the licences a Debian
// system holds in /usr/share/common-licenses, where it has them: those
// known by their names, the others by none (GPL-2 is no GPL-3, MPL-1.1
// no MPL-2.0, 0BSD-like and public-domain texts nothing).
func TestClassifyCommonLicenses(t *testing.T) {
	const dir = "/usr/share/common-licenses"
	if _, err := os.Stat(dir); err != nil {
		t.Skip("no " + dir)
	}
	for name, want := range map[string]string{
		"Apache-2.0": "Apache-2.0", "BSD": "BSD-3-Clause", "GPL-3": "GPL-3.0", "LGPL-3": "LGPL-3.0", "MPL-2.0": "MPL-2.0",
		"GPL-1": "", "GPL-2": "", "LGPL-2": "", "LGPL-2.1": "", "MPL-1.1": "", "Artistic": "", "CC0-1.0": "", "GFDL-1.2": "", "GFDL-1.3": "",
	} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue // not on this system
		}
		if got := classify(string(b)); got != want {
			t.Errorf("classify(%s) = %q, want %q", name, got, want)
		}
	}
	// two full texts in one file, in both orders: named by none
	gpl, err1 := os.ReadFile(filepath.Join(dir, "GPL-3"))
	lgpl, err2 := os.ReadFile(filepath.Join(dir, "LGPL-3"))
	if err1 == nil && err2 == nil {
		for what, text := range map[string]string{"LGPL-3 then GPL-3": string(lgpl) + string(gpl), "GPL-3 then LGPL-3": string(gpl) + string(lgpl)} {
			if got := classify(text); got != "" {
				t.Errorf("classify(%s) = %q, want none", what, got)
			}
		}
	}
}

// The heads of the GNU licences version 3, as their texts open.
const (
	gpl3  = "                    GNU GENERAL PUBLIC LICENSE\n                       Version 3, 29 June 2007\n\n Copyright (C) 2007 Free Software Foundation, Inc.\n"
	lgpl3 = "                   GNU LESSER GENERAL PUBLIC LICENSE\n                       Version 3, 29 June 2007\n\n Copyright (C) 2007 Free Software Foundation, Inc.\n"
)

// TestClassifyMisleading checks the texts that name or quote another
// licence than their own: a licence is named by a title that opens its
// line or a sentence only it has, and only where exactly one matches; a
// second licence further down the file names it by none.
func TestClassifyMisleading(t *testing.T) {
	apache := "                                 Apache License\n                           Version 2.0, January 2004\n                        http://www.apache.org/licenses/\n\n" +
		"   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION\n"
	for _, c := range []struct{ what, text, want string }{
		{"MIT", mitText, "MIT"},
		{"BSD-3-Clause", bsdText, "BSD-3-Clause"},
		{"BSD-2-Clause", "Copyright (c) 2026 Someone\n\n" + bsdBody, "BSD-2-Clause"},
		{"BSD-3 with the name of the author", "Copyright (c) 2026 Someone\n\n" + bsdBody +
			"3. The name of the author may not be used to endorse or promote products\nderived from this software without specific prior written permission.\n", "BSD-3-Clause"},
		{"BSD-4", bsdText + "3. All advertising materials mentioning features or use of this software\nmust display the following acknowledgement\n", ""},
		{"Apache-2.0", apache, "Apache-2.0"},
		{"Apache's notice", "Copyright 2026 Someone\n\nLicensed under the Apache License, Version 2.0 (the \"License\");\nyou may not use this file except in compliance with the License.\n", "Apache-2.0"},
		{"MIT that mentions Apache", mitText + "\nThe Apache License, Version 2.0, does not apply to this software.\n", "MIT"},
		{"MIT and Apache's notice in one file", mitText + "\nLicensed under the Apache License, Version 2.0 (the \"License\");\nyou may not use this file except in compliance with the License.\n", ""},
		{"Apache with an MIT text appended", apache + "\n" + mitText, ""},
		{"GPL-3 naming the Affero licence", "                    GNU GENERAL PUBLIC LICENSE\n                       Version 3, 29 June 2007\n\n" +
			"  13. Use with the GNU Affero General Public License.\n", "GPL-3.0"},
		{"LGPL-3 naming the GPL", "                   GNU LESSER GENERAL PUBLIC LICENSE\n                       Version 3, 29 June 2007\n\n" +
			"  This version of the GNU Lesser General Public License incorporates\nthe terms and conditions of version 3 of the GNU General Public\nLicense\n", "LGPL-3.0"},
		{"MPL-2.0 naming the AGPL", "Mozilla Public License Version 2.0\n==================================\n\n" +
			"1.12. \"Secondary License\"\n    means either the GNU General Public License, Version 2.0, the GNU Lesser\n    General Public License, Version 2.1, the GNU Affero General Public\n    License, Version 3.0, or any later versions of those licenses.\n", "MPL-2.0"},
		{"GPL-2", "                    GNU GENERAL PUBLIC LICENSE\n                       Version 2, June 1991\n\n ... version 3 ...\n", ""},
		{"0BSD", "Copyright (C) 2026 Someone\n\nPermission to use, copy, modify, and/or distribute this software for any\npurpose with or without fee is hereby granted.\n", ""},
		{"ISC", "Copyright (C) 2026 Someone\n\nPermission to use, copy, modify, and/or distribute this software for any\npurpose with or without fee is hereby granted, provided that the above\ncopyright notice and this permission notice appear in all copies.\n", "ISC"},
		{"Boost", "Boost Software License - Version 1.0 - August 17th, 2003\n\nPermission is hereby granted, free of charge, to any person or organization\n" +
			"obtaining a copy of the software and accompanying documentation covered by\nthis license (the \"Software\") to use, reproduce, display, distribute ...\n\n" +
			"The copyright notices in the Software and this entire statement, including\nthe above license grant, this restriction and the following disclaimer,\nmust be included in all copies of the Software\n", ""},
		{"CC BY 4.0", "Attribution 4.0 International\n\n=======================================================================\n\nCreative Commons Corporation (\"Creative Commons\") is not a law firm\n", "CC-BY-4.0"},
		{"public domain", "The author disclaims copyright to this source code.\n", ""},
		{"a title that does not open its line", "ODC Open Database License\nsee Attribution 4.0 International for the contents\n", ""},
		{"X11", mitText + "\nExcept as contained in this notice, the name of the X Consortium shall not be used in advertising or\notherwise to promote the sale, use or other dealings in this Software.\n", ""},
		{"LGPL-3 followed by GPL-3", lgpl3 + "\n" + gpl3, ""},
		{"GPL-3 followed by LGPL-3", gpl3 + "\n" + lgpl3, ""},
		{"GPL-3 followed by an MIT text", gpl3 + "\n" + mitText, ""},
	} {
		if got := classify(c.text); got != c.want {
			t.Errorf("%s: classify = %q, want %q", c.what, got, c.want)
		}
	}
}

// TestNoteLicence checks a vendored entry's licence from its note's
// Licence column: taken, for the entry and its file, where the text is
// known by none; where the text is known by another, make credits stops
// naming both.
func TestNoteLicence(t *testing.T) {
	tr := newTree(t)
	tr.write(t, "third_party/kept/MASSIMO.md", "| Directory | Package | Version | Licence |\n|---|---|---|---|\n| `.` | `kept` | 1.0 | Unlicense, `LICENSE` |\n")
	tr.write(t, "third_party/kept/LICENSE", "This is free and unencumbered software released into the public domain.\n")
	if err := run(tr.root, false, tr.source(), io.Discard); err != nil {
		t.Fatal(err)
	}
	if e, _ := tr.list(t).Find("kept"); e.Licence != "Unlicense" || e.Mixed || e.Files[0].Licence != "Unlicense" {
		t.Errorf("the note's licence for a text known by none: %+v", e)
	}
	tr.write(t, "third_party/kept/LICENSE", mitText)
	tr.write(t, "third_party/kept/MASSIMO.md", "| Directory | Package | Version | Licence |\n|---|---|---|---|\n| `.` | `kept` | 1.0 | Apache-2.0, `LICENSE` |\n")
	err := run(tr.root, false, tr.source(), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "kept: its note says Apache-2.0, but LICENSE reads as MIT") {
		t.Errorf("a note at odds with its text: %v", err)
	}
}

// TestIncludesNeedTheirTexts checks that a vendored entry whose note
// names what it includes stops make credits, naming it, while no
// licence file beside its first holds their texts, which the page and
// massimo credits say one does; with such a file beside it, the entry
// is taken, mixed where that file is known by no licence.
func TestIncludesNeedTheirTexts(t *testing.T) {
	tr := newTree(t)
	tr.write(t, "internal/server/ui/vendor/README.md", "| File | Package | Version | Licence | Includes |\n|---|---|---|---|---|\n| `x.js` | `@x/x` | 2.0.0 | MIT, `x.LICENSE` | `y` 1.0 |\n")
	err := run(tr.root, false, tr.source(), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "@x/x: its note says it includes y 1.0, but no licence file beside x.LICENSE holds their texts") {
		t.Errorf("what it includes without their texts: %v", err)
	}
	tr.write(t, "internal/server/ui/vendor/x.LICENSE-3RD-PARTY", "==> y 1.0 (MIT OR Apache-2.0) <==\n--- LICENSE-MIT\n"+mitText+
		"--- LICENSE-APACHE\n                              Apache License\n                        Version 2.0, January 2004\n")
	if err := run(tr.root, false, tr.source(), io.Discard); err != nil {
		t.Fatal(err)
	}
	if e, _ := tr.list(t).Find("@x/x"); e.Includes != "y 1.0" || len(e.Files) != 2 || e.Files[1].Name != "x.LICENSE-3RD-PARTY" || e.Files[1].Licence != "" || !e.Mixed {
		t.Errorf("the entry with its texts beside it: %+v", e)
	}
}

// TestRowNamesAFile checks that a row of a note naming a file, not a
// directory, stops make credits with what a row's path must be.
func TestRowNamesAFile(t *testing.T) {
	tr := newTree(t)
	tr.write(t, "third_party/kept/MASSIMO.md", "| Directory | Package | Version | Licence |\n|---|---|---|---|\n| `list.txt` | `kept` | 1.0 | MIT |\n")
	tr.write(t, "third_party/kept/list.txt", "x")
	err := run(tr.root, false, tr.source(), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "a row's path is a directory of third_party/kept that holds its own licence file") {
		t.Errorf("a row naming a file: %v", err)
	}
}

// TestLicenceTooLarge checks that a licence file above maxLicence stops
// make credits, naming it.
func TestLicenceTooLarge(t *testing.T) {
	tr := newTree(t)
	tr.write(t, "third_party/kept/LICENSE", mitText+strings.Repeat("x", maxLicence))
	err := run(tr.root, false, tr.source(), io.Discard)
	if err == nil || !strings.Contains(err.Error(), filepath.Join("kept", "LICENSE")+" has ") {
		t.Errorf("a licence file too large: %v", err)
	}
}

// TestNoteRows checks the table of a note: the first with a Version
// column, its paths, names, versions, licences and includes.
func TestNoteRows(t *testing.T) {
	dir := t.TempDir()
	note := "# Note\n\n| A | B |\n|---|---|\n| `no` | version |\n\n" +
		"| Directory | Package | Version | Licence | Includes |\n|-----------|---------|---------|---------|---|\n" +
		"| `one/` | `@x/one`, with `@x/one/hooks` | 1.2.3 | MIT, `one/LICENSE` | |\n| `two/` | | `2.0` | `Apache-2.0` | `y` 1.0, `z` 2.1 |\n| `.` | | 3 | | |\n\n" +
		"| Directory | Version |\n|---|---|\n| `later/` | 9 |\n"
	if err := os.WriteFile(filepath.Join(dir, "MASSIMO.md"), []byte(note), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := noteRows(dir, "MASSIMO.md", "README.md")
	if err != nil {
		t.Fatal(err)
	}
	want := []row{
		{path: "one", name: "@x/one", version: "1.2.3", licence: "MIT"},
		{path: "two", name: "two", version: "2.0", licence: "Apache-2.0", includes: "y 1.0, z 2.1"},
		{path: ".", name: filepath.Base(dir), version: "3"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("rows %+v\nwant %+v", got, want)
	}
}
