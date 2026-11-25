// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package credits holds the credits and licences of everything the
// massimo binary includes: Massimo itself, Go's runtime and standard
// library, the Go modules it links on every platform it is built for,
// and what the repository vendors under third_party/ and
// internal/server/ui/vendor/, each with the files of its licence.
// make credits (tools/credits) writes credits.json, which git keeps and
// the binary embeds; System info shows it (served at /ui/credits.json).
// go test fails while it is not current.
package credits

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"runtime"
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
	// Version is its version, "" where none is known; Go's is the
	// running binary's (JSON), which the committed file leaves out.
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
	// crates inside the OPAQUE client's WebAssembly), whose own licence
	// texts are not in Files.
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
// file with Go's version set to the running binary's.
var served = sync.OnceValues(func() ([]byte, error) {
	l, err := Parse(embedded)
	if err != nil {
		return nil, err
	}
	for i := range l.Entries {
		if l.Entries[i].Kind == KindGo {
			l.Entries[i].Version = GoVersion()
		}
	}
	return Encode(l)
})

// GoVersion is the version of the Go the running binary was built with,
// as Go's entry carries it: runtime.Version() without its "go" in front
// (1.26.8), as the other entries' versions; a development toolchain's,
// which has no such prefix, as it is.
func GoVersion() string {
	return goVersionOf(runtime.Version())
}

// goVersionOf is v without a "go" in front of its digits.
func goVersionOf(v string) string {
	if rest, ok := strings.CutPrefix(v, "go"); ok && rest != "" && rest[0] >= '0' && rest[0] <= '9' {
		return rest
	}
	return v
}

// JSON is the list as the server serves it at /ui/credits.json: the
// embedded credits.json, Go's version the running binary's (GoVersion).
// A file that does not read is served as it is (TestEmbeddedList fails
// on it).
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
