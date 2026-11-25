// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package credits holds the credits and licences of everything the
// massimo binary includes: Massimo itself, Go's runtime and standard
// library, the Go modules it links on every platform it is built for,
// and what the repository vendors under third_party/ and
// internal/server/ui/vendor/, each with the files of its licence.
// make credits (tools/credits) writes credits.json, which git keeps and
// the binary embeds; System info shows it (served at /ui/credits.json)
// and massimo credits prints it, with the versions of Go and of each
// module the binary was built with, which the file leaves out. go test
// fails while it is not current: a module added or removed, a licence's
// text changed, not a module's version.
package credits

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

// The kinds of an entry: what it is to the binary.
const (
	// KindSelf is Massimo itself.
	KindSelf = "self"
	// KindGo is Go's runtime and standard library.
	KindGo = "go"
	// KindModule is a Go module the binary links; Path is set where the
	// repository holds a copy of it that replaces the published one.
	KindModule = "module"
	// KindVendored is a file or a directory the repository holds, under
	// third_party/ or internal/server/ui/vendor/, at Path.
	KindVendored = "vendored"
)

// List is the credits: the entries in their order, Massimo first, then
// Go, the Go modules by path and what is vendored by its path.
type List struct {
	Entries []Entry `json:"entries"`
}

// Entry is one thing the binary includes, with its licence's files. It
// holds names and identifiers only: every sentence around them is the
// page's, in its language, or the command's.
type Entry struct {
	// Name is how the list names it, unique without regard to case: a
	// module's path, a package's name, Massimo, Go.
	Name string `json:"name"`
	// Version is its version, "" where none is known: a vendored entry's
	// its note's; Go's and a module's the running binary's (JSON), which
	// the committed file leaves out, none for a module it does not link.
	Version string `json:"version,omitempty"`
	// Licence is the SPDX identifier of its licence: the one its note
	// names, else the one its first licence file is known by; "" where
	// none is known, and the texts say it.
	Licence string `json:"licence,omitempty"`
	// Mixed says that its licence files are not all under Licence: the
	// files name each their own.
	Mixed bool `json:"mixed,omitempty"`
	// Kind is KindSelf, KindGo, KindModule or KindVendored.
	Kind string `json:"kind"`
	// Path is where the repository holds it, relative to its top, with
	// slashes; "" for a module taken as published, Go and Massimo.
	Path string `json:"path,omitempty"`
	// Includes names what else it holds, as its note names it (the Rust
	// crates inside the OPAQUE client's WebAssembly), whose licence texts
	// are among Files (opaque.LICENSE-3RD-PARTY).
	Includes string `json:"includes,omitempty"`
	// Files are its licence's files: every LICENSE, LICENCE, COPYING,
	// NOTICE and PATENTS file it has, by name.
	Files []File `json:"files"`
}

// File is one file of a licence: its name, the SPDX identifier of the
// licence its text is known by ("" where none is), whether it is a
// notice (NOTICE, PATENTS) rather than a licence, and its text.
type File struct {
	Name    string `json:"name"`
	Licence string `json:"licence,omitempty"`
	Notice  bool   `json:"notice,omitempty"`
	Text    string `json:"text"`
}

//go:embed credits.json
var embedded []byte

// served is the list as the binary serves and prints it: the embedded
// file with the versions the running binary was built with (built).
var served = sync.OnceValues(func() ([]byte, error) {
	l, err := Parse(embedded)
	if err != nil {
		return nil, err
	}
	info, _ := debug.ReadBuildInfo()
	return Encode(built(l, runtime.Version(), info))
})

// noVersion is the version the build information gives a directory
// that go.mod's replace puts in a module's place (third_party/).
const noVersion = "(devel)"

// built is l with the versions of a binary built with the Go of
// goVersion (runtime.Version()) and the modules of info
// (debug.ReadBuildInfo), which the committed file leaves out: Go's entry
// goVersion without its "go" (goVersionOf); a module's the version info
// gives its path, a module replaced by another's its replacement's, one
// replaced by a directory the version go.mod requires; none where info
// does not name the module (one that only another platform links, a
// test binary that does not link it) or there is no info. A vendored
// entry keeps its note's.
func built(l List, goVersion string, info *debug.BuildInfo) List {
	versions := map[string]string{}
	if info != nil {
		for _, d := range info.Deps {
			v := d.Version
			if r := d.Replace; r != nil && r.Version != "" && r.Version != noVersion {
				v = r.Version
			}
			versions[d.Path] = v
		}
	}
	for i, e := range l.Entries {
		switch e.Kind {
		case KindGo:
			l.Entries[i].Version = goVersionOf(goVersion)
		case KindModule:
			l.Entries[i].Version = versions[e.Name]
		}
	}
	return l
}

// goVersionOf is Go's version v (runtime.Version()) as Go's entry
// carries it: without a "go" in front of its digits (1.26.8), as the
// other entries' versions; a development toolchain's, which has no such
// prefix, as it is.
func goVersionOf(v string) string {
	if rest, ok := strings.CutPrefix(v, "go"); ok && rest != "" && rest[0] >= '0' && rest[0] <= '9' {
		return rest
	}
	return v
}

// JSON is the list as the server serves it at /ui/credits.json: the
// embedded credits.json with the versions of Go and of each module the
// running binary was built with. A file that does not read is served as
// it is (TestEmbeddedList fails on it).
func JSON() []byte {
	b, err := served()
	if err != nil {
		return embedded
	}
	return b
}

// Load reads the list as JSON serves it.
func Load() (List, error) {
	if _, err := served(); err != nil {
		return List{}, err
	}
	return Parse(JSON())
}

// Encode writes a list in the form of credits.json: indented by two
// spaces, < > & as they are, a newline at its end.
func Encode(l List) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(l); err != nil {
		return nil, fmt.Errorf("credits: %w", err)
	}
	return b.Bytes(), nil
}

// Parse reads a list in the form of credits.json; an entry without a
// name or a licence's file is an error.
func Parse(b []byte) (List, error) {
	var l List
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&l); err != nil {
		return List{}, fmt.Errorf("credits: %w", err)
	}
	for i, e := range l.Entries {
		if e.Name == "" {
			return List{}, fmt.Errorf("credits: entry %d has no name", i+1)
		}
		if len(e.Files) == 0 {
			return List{}, fmt.Errorf("credits: %s has no licence file", e.Name)
		}
	}
	return l, nil
}

// Find is the entry of a name, compared without regard to case.
func (l List) Find(name string) (Entry, bool) {
	for _, e := range l.Entries {
		if strings.EqualFold(e.Name, name) {
			return e, true
		}
	}
	return Entry{}, false
}
