// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"go.metaworx.ch/massimo/internal/credits"
)

// The places the list is made from, relative to the repository's top.
const (
	thirdParty = "third_party"
	uiVendor   = "internal/server/ui/vendor"
)

// The licence of Massimo itself, as every file's SPDX line names it.
const ownLicence = "AGPL-3.0-or-later"

// target is a platform the binary is built for: GOOS, GOARCH and GOARM.
type target struct{ goos, goarch, goarm string }

// targets are the platforms the Makefile's cross builds, in its order
// (TestTargetsAreTheMakefiles).
var targets = []target{
	{"windows", "amd64", ""},
	{"linux", "amd64", ""},
	{"linux", "arm64", ""},
	{"linux", "arm", "7"},
	{"linux", "arm", "6"},
	{"darwin", "amd64", ""},
	{"darwin", "arm64", ""},
}

// module is a Go module the binary links: its path and version, the
// directory its files are in, and local, the directory of the
// repository that replaces it (go.mod's replace), "" for none.
type module struct {
	path, version, dir, local string
}

// source is what the list is made from: the repository at root, Go's
// own directory, and the modules the binary links.
type source struct {
	root    string
	goRoot  func() (string, error)
	modules func() ([]module, error)
}

// linkedModules are the modules go list -deps finds for ./cmd/massimo
// on every target, as the Makefile's cross builds them (no cgo), their
// union by path. A module of two versions is an error.
func linkedModules(root string) ([]module, error) {
	found := make([][]module, len(targets))
	errs := make([]error, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Go(func() { found[i], errs[i] = modulesFor(root, t) })
	}
	wg.Wait()
	byPath := map[string]module{}
	for i, list := range found {
		if errs[i] != nil {
			return nil, errs[i]
		}
		for _, m := range list {
			if have, ok := byPath[m.path]; ok && have.version != m.version {
				return nil, fmt.Errorf("credits: %s is %s on one platform and %s on another", m.path, have.version, m.version)
			}
			byPath[m.path] = m
		}
	}
	out := make([]module, 0, len(byPath))
	for _, m := range byPath {
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b module) int { return strings.Compare(a.path, b.path) })
	return out, nil
}

// modulesFor are the modules go list -deps finds for ./cmd/massimo on t.
func modulesFor(root string, t target) ([]module, error) {
	cmd := exec.Command("go", "list", "-deps", "-json=ImportPath,Module", "./cmd/massimo")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS="+t.goos, "GOARCH="+t.goarch, "GOARM="+t.goarm, "CGO_ENABLED=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("credits: go list for %s/%s%s: %w\n%s\n(go list reads the modules from the module cache and downloads one missing there where GOPROXY allows; with GOPROXY=off, go mod download fills the cache)",
			t.goos, t.goarch, t.goarm, err, bytes.TrimSpace(stderr.Bytes()))
	}
	type mod struct {
		Path, Version, Dir string
		Main               bool
		Replace            *struct{ Path, Version, Dir string }
	}
	seen := map[string]bool{}
	var list []module
	for d := json.NewDecoder(bytes.NewReader(out)); d.More(); {
		var p struct{ Module *mod }
		if err := d.Decode(&p); err != nil {
			return nil, fmt.Errorf("credits: go list for %s/%s%s: %w", t.goos, t.goarch, t.goarm, err)
		}
		m := p.Module
		if m == nil || m.Main || seen[m.Path] {
			continue // the standard library, Massimo's own packages
		}
		seen[m.Path] = true
		e := module{path: m.Path, version: m.Version, dir: m.Dir}
		if r := m.Replace; r != nil {
			e.dir = r.Dir
			if r.Version != "" {
				e.version = r.Version // another module's release in its place
			} else if rel, err := filepath.Rel(root, r.Dir); err == nil {
				e.local = filepath.ToSlash(rel)
			}
		}
		list = append(list, e)
	}
	return list, nil
}

// generate makes the list: Massimo, Go, the modules by path, then what
// third_party/ and the UI's vendor/ hold, by path; and the number of
// entries.
func generate(s source) ([]byte, int, error) {
	mods, err := s.modules()
	if err != nil {
		return nil, 0, err
	}
	var l credits.List
	self, err := entryOf(credits.Entry{Name: "Massimo", Licence: ownLicence, Kind: credits.KindSelf}, s.root, "")
	if err != nil {
		return nil, 0, err
	}
	l.Entries = append(l.Entries, self)
	g, err := goEntry(s, mods)
	if err != nil {
		return nil, 0, err
	}
	l.Entries = append(l.Entries, g)
	local := map[string]bool{}
	for _, m := range mods {
		e, err := entryOf(credits.Entry{Name: m.path, Version: m.version, Kind: credits.KindModule, Path: m.local}, m.dir, "")
		if err != nil {
			return nil, 0, err
		}
		if m.local != "" {
			local[m.local] = true
		}
		l.Entries = append(l.Entries, e)
	}
	vendored, err := thirdPartyEntries(s.root, local)
	if err != nil {
		return nil, 0, err
	}
	l.Entries = append(l.Entries, vendored...)
	vendored, err = uiVendorEntries(s.root)
	if err != nil {
		return nil, 0, err
	}
	l.Entries = append(l.Entries, vendored...)
	names := map[string]string{}
	for _, e := range l.Entries {
		k := strings.ToLower(e.Name)
		if other, ok := names[k]; ok {
			return nil, 0, fmt.Errorf("credits: two entries are named %s (%s and %s)", e.Name, other, where(e))
		}
		names[k] = where(e)
	}
	b, err := credits.Encode(l)
	if err != nil {
		return nil, 0, err
	}
	return b, len(l.Entries), nil
}

// where names an entry's place for an error: its path, else its name.
func where(e credits.Entry) string {
	if e.Path != "" {
		return e.Path
	}
	return e.Name
}

// goEntry is Go's: the LICENSE and PATENTS of Go's own directory, and
// no version, which internal/credits sets to the running binary's. A Go
// that a distribution packaged may keep them elsewhere (Debian's in
// /usr/share/doc): then they are taken from golang.org/x/sys, a module
// of the Go project the binary links, whose LICENSE and PATENTS are
// Go's, byte for byte.
func goEntry(s source, mods []module) (credits.Entry, error) {
	dir, err := s.goRoot()
	if err != nil {
		return credits.Entry{}, err
	}
	e := credits.Entry{Name: "Go", Kind: credits.KindGo}
	if files, err := licenceFiles(dir, ""); err == nil && len(files) > 0 {
		return entryOf(e, dir, "")
	}
	for _, m := range mods {
		if m.path == "golang.org/x/sys" {
			return entryOf(e, m.dir, "")
		}
	}
	return credits.Entry{}, fmt.Errorf("credits: Go: no licence file in %s, and golang.org/x/sys is not linked", dir)
}

// thirdPartyEntries are the entries of third_party/: each directory but
// those replacing a module (local), which the module's entry covers.
// A directory's note (MASSIMO.md, else README.md) may name what it holds
// in a table with a Version column (noteRows): an entry a row; else the
// directory is one entry, named by it.
func thirdPartyEntries(root string, local map[string]bool) ([]credits.Entry, error) {
	dirs, err := os.ReadDir(filepath.Join(root, thirdParty))
	if err != nil {
		return nil, fmt.Errorf("credits: %w", err)
	}
	var out []credits.Entry
	for _, d := range dirs {
		rel := thirdParty + "/" + d.Name()
		if !d.IsDir() || local[rel] {
			continue
		}
		dir := filepath.Join(root, filepath.FromSlash(rel))
		rows, err := noteRows(dir, "MASSIMO.md", "README.md")
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			rows = []row{{path: ".", name: d.Name()}}
		}
		for _, r := range rows {
			p := path.Join(rel, r.path)
			if p != rel && !strings.HasPrefix(p, rel+"/") {
				return nil, fmt.Errorf("credits: %s's note names %s, outside the directory", rel, r.path)
			}
			if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil || !st.IsDir() {
				return nil, fmt.Errorf("credits: %s's note names %s: a row's path is a directory of %s that holds its own licence file", rel, r.path, rel)
			}
			e, err := entryOf(credits.Entry{Name: r.name, Version: r.version, Licence: r.licence, Kind: credits.KindVendored, Path: p, Includes: r.includes},
				filepath.Join(root, filepath.FromSlash(p)), "")
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
	}
	return out, nil
}

// uiVendorEntries are the entries of the UI's vendor/: each file but
// its README.md and the licence files beside the others (opaque.LICENSE
// is opaque.js'), named and versioned by README.md's table where it has
// a row, else by the file's name.
func uiVendorEntries(root string) ([]credits.Entry, error) {
	dir := filepath.Join(root, filepath.FromSlash(uiVendor))
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("credits: %w", err)
	}
	rows, err := noteRows(dir, "README.md")
	if err != nil {
		return nil, err
	}
	byFile := map[string]row{}
	for _, r := range rows {
		byFile[r.path] = r
	}
	var out []credits.Entry
	for _, f := range files {
		name := f.Name()
		if f.IsDir() || name == "README.md" || licenceName(name[strings.IndexByte(name, '.')+1:]) {
			continue
		}
		r, ok := byFile[name]
		if !ok {
			r = row{path: name, name: name}
		}
		delete(byFile, name)
		e, err := entryOf(credits.Entry{Name: r.name, Version: r.version, Licence: r.licence, Kind: credits.KindVendored, Path: uiVendor + "/" + name, Includes: r.includes},
			dir, strings.TrimSuffix(name, path.Ext(name)))
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	for name := range byFile {
		return nil, fmt.Errorf("credits: %s/README.md names %s, which is not there", uiVendor, name)
	}
	return out, nil
}

// entryOf is e with the licence's files of dir, those of the file stem
// where stem is given (opaque.LICENSE for opaque), each named by its
// text (classify) unless it is a notice. The entry's licence is the one
// e brings (its note's, Massimo's own), which must be the one its first
// licence file is known by where that one is known, and is that file's
// where it is not; else that file's.
// Mixed where the entry's licence is known and a licence file is under
// another or under none known. None
// found is an error.
func entryOf(e credits.Entry, dir, stem string) (credits.Entry, error) {
	files, err := licenceFiles(dir, stem)
	if err != nil {
		return credits.Entry{}, err
	}
	var first *credits.File
	for i := range files {
		if !files[i].Notice && first == nil {
			first = &files[i]
		}
	}
	if first == nil {
		what := "files"
		if stem != "" {
			what = stem + ".LICENSE and the like"
		}
		return credits.Entry{}, fmt.Errorf("credits: %s: no licence file (LICENSE, LICENCE, COPYING) among the %s of %s", e.Name, what, dir)
	}
	switch {
	case e.Licence == "":
		e.Licence = first.Licence
	case first.Licence == "":
		first.Licence = e.Licence // the note names what the text is
	case first.Licence != baseLicence(e.Licence):
		return credits.Entry{}, fmt.Errorf("credits: %s: its note says %s, but %s reads as %s", e.Name, e.Licence, first.Name, first.Licence)
	}
	for _, f := range files {
		if e.Licence != "" && !f.Notice && f.Licence != baseLicence(e.Licence) {
			e.Mixed = true
		}
	}
	e.Files = files
	return e, nil
}

// baseLicence is an SPDX identifier without the -or-later or -only that
// its text cannot tell (Massimo's AGPL-3.0-or-later is the AGPL-3.0's
// text).
func baseLicence(id string) string {
	return strings.TrimSuffix(strings.TrimSuffix(id, "-or-later"), "-only")
}

// maxLicence is the largest licence file taken, 256 KiB: three times the
// largest of today (modernc.org/sqlite's LICENSE-3RD-PARTY.md, 74 KB),
// while the whole list, which a phone fetches, stays small; a larger file
// is most likely no licence.
const maxLicence = 256 << 10

// licenceFiles are the licence's files of dir, by name: those
// licenceName takes, or with stem those named stem.<a name it takes>;
// a NOTICE or PATENTS file a notice, any other named by its text. Their
// line ends are LF, as on every checkout.
func licenceFiles(dir, stem string) ([]credits.File, error) {
	list, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("credits: %w", err)
	}
	var out []credits.File
	for _, f := range list {
		name := f.Name()
		bare := name
		if stem != "" {
			var ok bool
			if bare, ok = strings.CutPrefix(name, stem+"."); !ok {
				continue
			}
		}
		if f.IsDir() || !licenceName(bare) {
			continue
		}
		path := filepath.Join(dir, name)
		if st, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("credits: %w", err)
		} else if st.Size() > maxLicence {
			return nil, fmt.Errorf("credits: %s has %d bytes, more than a licence file's %d", path, st.Size(), maxLicence)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("credits: %w", err)
		}
		if !utf8.Valid(b) {
			return nil, fmt.Errorf("credits: %s is not UTF-8", path)
		}
		text := strings.ReplaceAll(string(b), "\r\n", "\n")
		file := credits.File{Name: name, Text: text}
		if u := strings.ToUpper(bare); strings.HasPrefix(u, "NOTICE") || u == "PATENTS" {
			file.Notice = true
		} else {
			file.Licence = classify(text)
		}
		out = append(out, file)
	}
	return out, nil
}

// licenceName reports whether a file's name is a licence's: LICENSE,
// LICENCE, COPYING or NOTICE with anything after it (LICENSE-3RD-PARTY.md,
// COPYING.txt), or PATENTS, in any case.
func licenceName(name string) bool {
	n := strings.ToUpper(name)
	for _, p := range []string{"LICENSE", "LICENCE", "COPYING", "NOTICE"} {
		if strings.HasPrefix(n, p) {
			return true
		}
	}
	return n == "PATENTS"
}

// known are the licences classify names: each by a title or by a set
// of sentences only it has, all of them anywhere in the text, none of
// not among them; phrases in lower case with the spaces collapsed. A
// title stands on a line of its own: its first phrase opens the line,
// the others (the version) stand in it or in the line after it. A title
// alone names the GNU licences, the MPL and the CC's, whose texts name
// each other in their bodies. Apache-2.0's notice, which a file under it
// carries in place of the text, counts. X11's clause against
// advertising keeps the MIT text from being MIT.
var known = []struct {
	id        string
	title     [][]string
	sentences [][]string
	not       []string
}{
	{id: "AGPL-3.0", title: [][]string{{"gnu affero general public license", "version 3"}}},
	{id: "GPL-3.0", title: [][]string{{"gnu general public license", "version 3"}}},
	{id: "LGPL-3.0", title: [][]string{{"gnu lesser general public license", "version 3"}}},
	{id: "MPL-2.0", title: [][]string{{"mozilla public license", "version 2.0"}, {"mozilla public license, version 2.0"}}},
	{id: "Apache-2.0", title: [][]string{{"apache license", "version 2.0, january 2004"}},
		sentences: [][]string{
			{"terms and conditions for use, reproduction, and distribution", "grant of copyright license. subject to the terms and conditions of this license, each contributor hereby grants to you a perpetual, worldwide, non-exclusive, no-charge, royalty-free, irrevocable copyright license"},
			{"licensed under the apache license, version 2.0", "you may not use this file except in compliance with the license"}}},
	{id: "CC-BY-4.0", title: [][]string{{"attribution 4.0 international"}}},
	{id: "CC-BY-3.0-US", title: [][]string{{"attribution 3.0 united states"}, {"creative commons legal code", "attribution 3.0 united states"}}},
	{id: "MIT", sentences: [][]string{{
		"permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files",
		"the above copyright notice and this permission notice shall be included in all copies or substantial portions of the software"}},
		not: []string{"shall not be used in advertising or otherwise to promote"}},
	{id: "ISC", sentences: [][]string{{
		"permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted, provided that the above copyright notice and this permission notice appear in all copies"}}},
	{id: "BSD-3-Clause", sentences: [][]string{{
		"redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met",
		"redistributions of source code must retain the above copyright notice",
		"redistributions in binary form must reproduce the above copyright notice",
		"be used to endorse or promote products derived from this software"}},
		not: []string{"all advertising materials mentioning features or use of this software"}},
	{id: "BSD-2-Clause", sentences: [][]string{{
		"redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met",
		"redistributions of source code must retain the above copyright notice",
		"redistributions in binary form must reproduce the above copyright notice"}},
		not: []string{"all advertising materials mentioning features or use of this software", "be used to endorse or promote products derived from this software"}},
}

// classify is the SPDX identifier of the licence a text is, where
// exactly one known licence matches it: by the title its first line
// that is not empty opens, by its sentences anywhere, or by a title that
// opens a line further down (a second licence appended); "" where none
// does or more than one (a file that holds several licences, a title of
// one that quotes another).
func classify(text string) string {
	var lines []string
	for line := range strings.Lines(text) {
		if f := strings.Fields(strings.ToLower(line)); len(f) > 0 {
			lines = append(lines, strings.Join(f, " "))
		}
	}
	t := strings.Join(lines, " ")
	all := func(in string, phrases []string) bool {
		for _, p := range phrases {
			if !strings.Contains(in, p) {
				return false
			}
		}
		return len(phrases) > 0
	}
	anyOf := func(in string, phrases []string) bool {
		for _, p := range phrases {
			if strings.Contains(in, p) {
				return true
			}
		}
		return false
	}
	// titled reports whether the line at i opens with title, its version
	// in it or in the next line.
	titled := func(i int, title []string) bool {
		if len(title) == 0 || !strings.HasPrefix(lines[i], title[0]) {
			return false
		}
		in := lines[i]
		if i+1 < len(lines) {
			in += " " + lines[i+1]
		}
		return all(in, title[1:]) || len(title) == 1
	}
	var found []string
	for _, k := range known {
		match := false
		for _, set := range k.sentences {
			match = match || (all(t, set) && !anyOf(t, k.not))
		}
		for _, title := range k.title {
			for i := range lines {
				match = match || titled(i, title)
			}
		}
		if match {
			found = append(found, k.id)
		}
	}
	if len(found) != 1 {
		return ""
	}
	return found[0]
}

// row is a row of a note's table: the path it names, relative to the
// note's directory ("." for the directory itself), the name, the
// version, the licence's SPDX identifier and what else it includes.
type row struct {
	path, name, version, licence, includes string
}

// noteRows are the rows of the first table of dir's note (the first of
// names there) whose head has a Version column: the first column the
// path, in backticks; Package the name, its first word in backticks,
// else the path's last element; Version the version; Licence (or
// License) the licence's SPDX identifier, before a comma; Includes what
// else it holds, names and versions alone. No note is an error; a note
// without such a table has none.
func noteRows(dir string, names ...string) ([]row, error) {
	var b []byte
	var err error
	for _, n := range names {
		if b, err = os.ReadFile(filepath.Join(dir, n)); err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("credits: %s has no note (%s)", dir, strings.Join(names, " or "))
	}
	var head []string
	var rows []row
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "|") {
			if rows != nil {
				break // the table has ended
			}
			head = nil
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		switch {
		case head == nil:
			head = cells
		case strings.Trim(strings.Join(cells, ""), "-: ") == "":
			if !slices.ContainsFunc(head, func(h string) bool { return strings.EqualFold(h, "Version") }) {
				head = []string{} // not a table of what the directory holds
			}
		case len(head) > 0:
			r := row{}
			for i, c := range cells {
				if i >= len(head) {
					break
				}
				switch strings.ToLower(head[i]) {
				case "version":
					r.version = strings.ReplaceAll(c, "`", "")
				case "package":
					r.name = ticked(c)
				case "licence", "license":
					id, _, _ := strings.Cut(c, ",")
					r.licence = strings.TrimSpace(strings.ReplaceAll(id, "`", ""))
				case "includes":
					r.includes = strings.ReplaceAll(c, "`", "")
				}
			}
			r.path = strings.TrimSuffix(ticked(cells[0]), "/")
			if r.path == "" {
				return nil, fmt.Errorf("credits: %s: a row of the note's table names no path in backticks: %s", dir, line)
			}
			if r.name == "" {
				r.name = path.Base(r.path)
				if r.path == "." {
					r.name = filepath.Base(dir)
				}
			}
			rows = append(rows, r)
		}
	}
	return rows, sc.Err()
}

// ticked is the first word of s in backticks, "" for none.
func ticked(s string) string {
	_, after, ok := strings.Cut(s, "`")
	if !ok {
		return ""
	}
	word, _, ok := strings.Cut(after, "`")
	if !ok {
		return ""
	}
	return word
}
