// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// crate is a crate of the file's list: its name, its version and the
// SHA-256 of its archive on crates.io, in hex.
type crate struct {
	name, version, sum string
}

// licenceFile is a licence file taken from a crate's archive: its name
// there and its text, line ends as LF.
type licenceFile struct {
	name string
	text []byte
}

// texts are what a crate's archive gives the file: the licence its
// Cargo.toml declares and its licence files, in their order.
type texts struct {
	crate
	declared string
	files    []licenceFile
}

// The limits of what an archive may hold, far above the list's largest
// (web-sys 0.3.85: 645 KB, 10.7 MB unpacked); a licence file or a
// Cargo.toml above 1 MiB is most likely none.
const (
	maxArchive  = 64 << 20
	maxUnpacked = 512 << 20
	maxFile     = 1 << 20
)

var (
	// listLine is a crate of the head's list: name, version, SHA-256.
	listLine = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_-]*) ([0-9]+\.[0-9]+\.[0-9]+[0-9A-Za-z.+-]*) ([0-9a-f]{64})$`)
	// crateLine and fileLine are the lines the file writes for a crate
	// and for each of its files; no line of a text may read as one.
	crateLine = regexp.MustCompile(`^==> \S+ \S+ \(.*\) <==$`)
	fileLine  = regexp.MustCompile(`^--- \S+(: the same text as \S+ \S+ \S+ above)?$`)
	// aSum is a SHA-256 in hex, which a line of the head holds only as a
	// line of the list.
	aSum = regexp.MustCompile(`[0-9A-Fa-f]{64}`)
	// plainName is the name of a licence file the file takes: letters,
	// digits, '.', '_' and '-', a path's parts parted by '/'.
	plainName = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`)
)

// licencePrefixes open the names of the files taken from an archive's
// root, in any case.
var licencePrefixes = []string{"LICENSE", "LICENCE", "COPYING", "NOTICE", "UNLICENSE"}

// split is the file's head, which ends where its first crate's line
// (==> ) begins, with one newline at its end; and the crates its list
// names, in their order. A head without a list, a crate listed twice, a
// CR in the head, and a line that holds a SHA-256 but is no line of the
// list (which would drop its crate as prose) are errors.
func split(file []byte) (string, []crate, error) {
	head := string(file)
	if i := strings.Index(head, "\n==> "); i >= 0 {
		head = head[:i+1]
	} else if strings.HasPrefix(head, "==> ") {
		head = ""
	}
	head = strings.TrimRight(head, "\n") + "\n"
	if strings.Contains(head, "\r") {
		return "", nil, errors.New("opaquecrates: the file's head holds a CR: its lines end in LF alone")
	}
	var list []crate
	seen := map[string]bool{}
	for line := range strings.Lines(head) {
		line = strings.TrimSuffix(line, "\n")
		m := listLine.FindStringSubmatch(line)
		if m == nil {
			if aSum.MatchString(line) {
				return "", nil, fmt.Errorf(`opaquecrates: a line of the file's head holds a SHA-256 but is no line "<name> <version> <SHA-256>" of the list: %q`, line)
			}
			continue
		}
		if seen[m[1]+" "+m[2]] {
			return "", nil, fmt.Errorf("opaquecrates: the list names %s %s twice", m[1], m[2])
		}
		seen[m[1]+" "+m[2]] = true
		list = append(list, crate{name: m[1], version: m[2], sum: m[3]})
	}
	if len(list) == 0 {
		return "", nil, errors.New(`opaquecrates: the file's head lists no crate (a line "<name> <version> <SHA-256>")`)
	}
	return head, list, nil
}

// read takes a crate's texts from its archive, a .crate file of
// crates.io (a tar in gzip, its files under <name>-<version>/): an
// archive whose SHA-256 is not the list's is refused, and so is one
// whose Cargo.toml names another crate or version. The texts are the
// files at the archive's root whose names open with one of
// licencePrefixes, by name, then the one its Cargo.toml's license-file
// names where it is not among them; CRLF as LF.
func read(c crate, archive []byte) (texts, error) {
	where := c.name + " " + c.version
	if sum := sha256.Sum256(archive); hex.EncodeToString(sum[:]) != c.sum {
		return texts{}, fmt.Errorf("opaquecrates: %s: the archive's SHA-256 is %x, the list's %s", where, sum, c.sum)
	}
	root := c.name + "-" + c.version + "/"
	var manifest []byte
	if err := walk(archive, root, func(rel string, h *tar.Header, r io.Reader) error {
		if rel != "Cargo.toml" {
			return nil
		}
		var err error
		manifest, err = regular(h, r)
		return err
	}); err != nil {
		return texts{}, fmt.Errorf("opaquecrates: %s: %w", where, err)
	}
	if manifest == nil {
		return texts{}, fmt.Errorf("opaquecrates: %s: no %sCargo.toml in the archive", where, root)
	}
	pkg, err := packageKeys(manifest)
	if err != nil {
		return texts{}, fmt.Errorf("opaquecrates: %s: Cargo.toml: %w", where, err)
	}
	if pkg["name"] != c.name || pkg["version"] != c.version {
		return texts{}, fmt.Errorf("opaquecrates: %s: its Cargo.toml names %s %s", where, pkg["name"], pkg["version"])
	}
	out := texts{crate: c, declared: pkg["license"]}
	var named *licenceFile
	if err := walk(archive, root, func(rel string, h *tar.Header, r io.Reader) error {
		atRoot := !strings.Contains(rel, "/") && slices.ContainsFunc(licencePrefixes, func(p string) bool {
			return strings.HasPrefix(strings.ToUpper(rel), p)
		})
		if !atRoot && (rel == "" || rel != pkg["license-file"]) {
			return nil
		}
		if !plainName.MatchString(rel) {
			return fmt.Errorf("the licence file %q has a character in its name other than letters, digits, '.', '_', '-' and '/'", rel)
		}
		b, err := regular(h, r)
		if err != nil {
			return err
		}
		f := licenceFile{name: rel, text: bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))}
		if atRoot {
			out.files = append(out.files, f)
		} else {
			named = &f
		}
		return nil
	}); err != nil {
		return texts{}, fmt.Errorf("opaquecrates: %s: %w", where, err)
	}
	slices.SortFunc(out.files, func(a, b licenceFile) int { return strings.Compare(a.name, b.name) })
	if lf := pkg["license-file"]; lf != "" && !slices.ContainsFunc(out.files, func(f licenceFile) bool { return f.name == lf }) {
		if named == nil {
			return texts{}, fmt.Errorf("opaquecrates: %s: its Cargo.toml names the license-file %s, which the archive does not hold", where, lf)
		}
		out.files = append(out.files, *named)
	}
	for _, f := range out.files {
		if !utf8.Valid(f.text) {
			return texts{}, fmt.Errorf("opaquecrates: %s: %s is not UTF-8, which tools/credits takes alone", where, f.name)
		}
		if i := strings.IndexFunc(string(f.text), control); i >= 0 {
			r, _ := utf8.DecodeRuneInString(string(f.text[i:]))
			return texts{}, fmt.Errorf("opaquecrates: %s: %s holds the control character U+%04X, which a terminal would act on", where, f.name, r)
		}
		for line := range strings.Lines(string(f.text)) {
			if l := strings.TrimSuffix(line, "\n"); crateLine.MatchString(l) || fileLine.MatchString(l) {
				return texts{}, fmt.Errorf("opaquecrates: %s: a line of %s reads as one of the file's own: %q", where, f.name, l)
			}
		}
	}
	return out, nil
}

// walk calls fn with each entry of the archive under root, by its path
// below root; an entry outside root is an error, and so is an archive
// that unpacks to more than maxUnpacked.
func walk(archive []byte, root string, fn func(rel string, h *tar.Header, r io.Reader) error) error {
	z, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("the archive is no gzip: %w", err)
	}
	limited := &io.LimitedReader{R: z, N: maxUnpacked}
	tr := tar.NewReader(limited)
	for {
		h, err := tr.Next()
		switch {
		case limited.N <= 0:
			return fmt.Errorf("the archive unpacks to more than %d bytes", maxUnpacked)
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return fmt.Errorf("the archive is no tar: %w", err)
		case h.Typeflag == tar.TypeXGlobalHeader || h.Typeflag == tar.TypeDir:
			continue
		}
		rel, ok := strings.CutPrefix(h.Name, root)
		if !ok {
			return fmt.Errorf("the archive holds %s, outside %s", h.Name, root)
		}
		if err := fn(rel, h, tr); err != nil {
			return err
		}
	}
}

// regular is the content of a regular file of an archive; a link, or a
// file above maxFile, is an error.
func regular(h *tar.Header, r io.Reader) ([]byte, error) {
	if h.Typeflag != tar.TypeReg {
		return nil, fmt.Errorf("%s is no regular file in the archive", h.Name)
	}
	if h.Size > maxFile {
		return nil, fmt.Errorf("%s has %d bytes, more than %d", h.Name, h.Size, maxFile)
	}
	b, err := io.ReadAll(io.LimitReader(r, maxFile))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", h.Name, err)
	}
	return b, nil
}

// packageKeys are the keys of a Cargo.toml's [package] table the file
// needs, name, version, license and license-file, each a string on a
// line of its own, as cargo publish writes the manifest into an archive.
// The lines inside a string of several lines (a description) are passed
// over, whatever they hold; a key given twice is an error.
func packageKeys(manifest []byte) (map[string]string, error) {
	out := map[string]string{}
	table := ""
	inside := "" // the quotes that end the string of several lines the scan is in
	for line := range strings.Lines(string(manifest)) {
		if inside != "" {
			if strings.Contains(line, inside) {
				inside = ""
			}
			continue
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			table = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		for _, q := range []string{`"""`, "'''"} {
			if ok && strings.HasPrefix(value, q) && !strings.Contains(value[len(q):], q) {
				inside = q
			}
		}
		if !ok || table != "package" {
			continue
		}
		switch key {
		case "name", "version", "license", "license-file":
		default:
			continue
		}
		if _, twice := out[key]; twice {
			return nil, fmt.Errorf("%s is given twice in [package]", key)
		}
		s, err := tomlString(value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		out[key] = s
	}
	return out, nil
}

// control reports whether r is a control character other than a tab
// and a newline: C0, DEL or C1.
func control(r rune) bool {
	return (r < 0x20 && r != '\t' && r != '\n') || (r >= 0x7f && r < 0xa0)
}

// tomlString is the value of a TOML string on one line: a literal
// string ('…') as it stands, a basic one ("…") with its escapes, which
// are Go's.
func tomlString(v string) (string, error) {
	switch {
	case strings.HasPrefix(v, `"""`) || strings.HasPrefix(v, "'''"):
		return "", fmt.Errorf("a string of several lines: %s", v)
	case len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'':
		return v[1 : len(v)-1], nil
	case len(v) >= 2 && v[0] == '"':
		s, err := strconv.Unquote(v)
		if err != nil {
			return "", fmt.Errorf("not a string: %s", v)
		}
		return s, nil
	}
	return "", fmt.Errorf("not a string: %s", v)
}

// compose is the file: the head as it is, then each crate in the list's
// order, a line ==> <name> <version> (<the licence it declares>) <==,
// then each of its files, a line --- <file> and its text, or where the
// same text stands above, byte for byte, a line naming it there; a crate
// without a licence file says so, with the licence it declares.
func compose(head string, crates []texts) []byte {
	var b bytes.Buffer
	b.WriteString(head)
	first := map[[32]byte]string{}
	for _, c := range crates {
		if c.declared == "" {
			fmt.Fprintf(&b, "\n==> %s %s (no licence declared) <==\n", c.name, c.version)
		} else {
			fmt.Fprintf(&b, "\n==> %s %s (%s) <==\n", c.name, c.version, c.declared)
		}
		if len(c.files) == 0 {
			if c.declared == "" {
				b.WriteString("No licence file in its archive\n")
			} else {
				fmt.Fprintf(&b, "No licence file in its archive; declared %s\n", c.declared)
			}
		}
		for _, f := range c.files {
			sum := sha256.Sum256(f.text)
			if at, ok := first[sum]; ok {
				fmt.Fprintf(&b, "--- %s: the same text as %s above\n", f.name, at)
				continue
			}
			first[sum] = c.name + " " + c.version + " " + f.name
			fmt.Fprintf(&b, "--- %s\n", f.name)
			b.Write(f.text)
			if !bytes.HasSuffix(f.text, []byte("\n")) {
				b.WriteByte('\n')
			}
		}
	}
	return b.Bytes()
}

// generate is the file anew from the one there, whose head it keeps and
// whose list it reads, each crate's archive from fetch; and the number
// of crates.
func generate(file []byte, fetch func(crate) ([]byte, error)) ([]byte, int, error) {
	head, list, err := split(file)
	if err != nil {
		return nil, 0, err
	}
	crates := make([]texts, 0, len(list))
	for _, c := range list {
		archive, err := fetch(c)
		if err != nil {
			return nil, 0, err
		}
		t, err := read(c, archive)
		if err != nil {
			return nil, 0, err
		}
		crates = append(crates, t)
	}
	return compose(head, crates), len(crates), nil
}
