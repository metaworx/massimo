// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package env

import (
	"bufio"
	"bytes"
	"cmp"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// sources records where each variable came from: "file" when LoadFile
// or ReloadFile set it, with its lines where the file gives it more
// than once (pair.source), "environment, overrides <file>" when it was
// already set (the environment wins). sourcesMu guards it, since serve
// re-reads the file while it runs.
var (
	sourcesMu sync.Mutex
	sources   = map[string]string{}
)

// fromFile reports whether src, a source of sources, is the file's.
func fromFile(src string) bool { return strings.HasPrefix(src, "file") }

// Source says where a variable's value came from: "file", "file, lines
// 3 and 4: the first wins" for a name the file gives more than once,
// "environment, overrides <file>", or "environment" for one the file did
// not mention.
func Source(name string) string {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	if s := sources[name]; s != "" {
		return s
	}
	return "environment"
}

// defaultFile is the env file File returns without MASSIMO_ENV_FILE.
var defaultFile = ".env"

// SetDefaultFile makes path the env file File returns without
// MASSIMO_ENV_FILE, in place of ".env" in the current directory: the
// service's settings file for the service user (cmd/massimo). It is
// not MASSIMO_ENV_FILE itself, which config would show as set by the
// environment and child processes would inherit. Call it at the start,
// before the file is read.
func SetDefaultFile(path string) { defaultFile = path }

// File returns the env file to read: MASSIMO_ENV_FILE, else the default
// file, ".env" in the current directory unless SetDefaultFile named
// another.
func File() string {
	if f := os.Getenv(FileVar); f != "" {
		return f
	}
	return defaultFile
}

// A Note is a line of the settings file that a read set aside, which
// sets nothing: a later line of a name the file gives again, a line
// without a name, a MASSIMO_ENV_FILE line. Its Text names the lines by
// their numbers and never quotes a value, which may be a secret; it
// reads after "the settings file: ".
type Note struct {
	// Lines are the numbers of the lines it is about, in the file's
	// order: for a name given more than once every line of it, the
	// first the one that wins.
	Lines []int
	// Name is the variable the lines give, "" for a line without one
	// and for a name that is no variable's (varName), which may be part
	// of a secret wrapped onto a line of its own.
	Name string
	// Text says what the read did with them: "MASSIMO_X is given on
	// lines 3 and 4; the first wins", "lines 3 and 4 give the same name;
	// the first wins" for a name that is no variable's, "line 7 has no
	// name before its =, ignored", "line 2 sets MASSIMO_ENV_FILE,
	// ignored: name the file with -env".
	Text string
}

// pair is the line of a name that wins, the file's first of it, with
// the numbers of every line that gives the name.
type pair struct {
	k, v  string
	lines []int
}

// source is what Source says of a variable the pair set: "file", or
// "file, lines 3 and 4: the first wins".
func (p pair) source() string {
	if len(p.lines) < 2 {
		return "file"
	}
	return "file, lines " + lineList(p.lines) + ": the first wins"
}

// lineList names line numbers as a sentence does: "3", "3 and 4",
// "3, 4 and 7".
func lineList(lines []int) string {
	s := make([]string, len(lines))
	for i, n := range lines {
		s[i] = strconv.Itoa(n)
	}
	if len(s) < 2 {
		return strings.Join(s, "")
	}
	return strings.Join(s[:len(s)-1], ", ") + " and " + s[len(s)-1]
}

// varName reports whether k is a variable's name as settings files write
// one, [A-Z_][A-Z0-9_]*, which a note may quote: anything else before a
// line's = may be part of a secret, a value wrapped onto a line of its
// own (base64 and hex hold lower-case letters), and is named by its
// lines alone.
func varName(k string) bool {
	for i, c := range k {
		if c != '_' && (c < 'A' || c > 'Z') && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return k != ""
}

// headSize is how much of the file readFile looks at before its lines:
// bufio's default buffer.
const headSize = 4096

// utf8BOM is the UTF-8 byte order mark, which Windows' editors and
// PowerShell write at the start of a file.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// utf16Text reports whether head, the first bytes of a file, is UTF-16
// text throughout, as PowerShell 5.1's > writes it: a UTF-16 byte order
// mark of either order, or a NUL byte in the first line, which UTF-16
// gives every ASCII character. The line's end counts, as UTF-16LE's
// newline is "\n" and then its NUL. Lines that >> appends in UTF-16 to a
// UTF-8 file are refused by readFile, at the first of them.
func utf16Text(head []byte) bool {
	if bytes.HasPrefix(head, []byte{0xFF, 0xFE}) || bytes.HasPrefix(head, []byte{0xFE, 0xFF}) {
		return true
	}
	if i := bytes.IndexByte(head, '\n'); i >= 0 {
		head = head[:min(i+2, len(head))]
	}
	return bytes.IndexByte(head, 0) >= 0
}

// readFile parses path whole, for LoadFile and ReloadFile alike, so that
// a start and a reload take the same lines: the line of each name that
// wins, in the file's order, and the notes of the lines it set aside,
// in the order of their first lines (Note). A missing file is
// os.ErrNotExist. A UTF-8 byte order mark at the start is skipped;
// UTF-16 text is refused, a file of it (utf16Text) as a line holding a
// NUL byte, which is named by its number. A line that does not parse, or
// that the read stops at (one too long for the scanner, a read error),
// is named by the file and its number, never quoted: it may hold a
// secret, a value wrapped onto a line of its own, and the error is shown
// at the start and in serve's log.
//
// The first line of a name wins, an empty one too; its note quotes the
// name only where it is a variable's (varName). A line without a name
// is ignored. A MASSIMO_ENV_FILE line is ignored, in any case, as
// Windows takes the name: the file File names stays the one the start
// read, where a reload of another file would take its token as the
// owner's key.
func readFile(path string) ([]pair, []Note, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	head, err := r.Peek(headSize)
	if err != nil && err != io.EOF {
		return nil, nil, fmt.Errorf("%s:1: %w", path, err)
	}
	if utf16Text(head) {
		return nil, nil, fmt.Errorf("%s: UTF-16 text, save it as UTF-8", path)
	}
	if bytes.HasPrefix(head, utf8BOM) {
		r.Discard(len(utf8BOM))
	}
	var (
		pairs []pair
		notes []Note
		index = map[string]int{} // a name's pair
	)
	sc := bufio.NewScanner(r)
	n := 1 // the line read, or the one the read stopped at
	for ; sc.Scan(); n++ {
		if bytes.IndexByte(sc.Bytes(), 0) >= 0 {
			return nil, nil, fmt.Errorf("%s:%d: UTF-16 text, save it as UTF-8", path, n)
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, nil, fmt.Errorf("%s:%d: expected KEY=VALUE", path, n)
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
		i, again := index[k]
		switch {
		case k == "":
			notes = append(notes, Note{Lines: []int{n}, Text: fmt.Sprintf("line %d has no name before its =, ignored", n)})
		case strings.EqualFold(k, FileVar):
			notes = append(notes, Note{Lines: []int{n}, Name: k, Text: fmt.Sprintf("line %d sets %s, ignored: name the file with -env", n, FileVar)})
		case again:
			pairs[i].lines = append(pairs[i].lines, n)
		default:
			index[k] = len(pairs)
			pairs = append(pairs, pair{k, v, []int{n}})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, nil, fmt.Errorf("%s:%d: %w", path, n, err)
	}
	for _, p := range pairs {
		switch {
		case len(p.lines) < 2:
		case varName(p.k):
			notes = append(notes, Note{Lines: p.lines, Name: p.k, Text: fmt.Sprintf("%s is given on lines %s; the first wins", p.k, lineList(p.lines))})
		default: // what is before its = may be a secret's
			notes = append(notes, Note{Lines: p.lines, Text: fmt.Sprintf("lines %s give the same name; the first wins", lineList(p.lines))})
		}
	}
	slices.SortStableFunc(notes, func(a, b Note) int { return cmp.Compare(a.Lines[0], b.Lines[0]) })
	return pairs, notes, nil
}

// LoadFile reads KEY=VALUE lines from path and sets them in the process
// environment unless the variable is already set. It exists so that secrets
// such as the Cube key never have to be typed on a command line, where they
// would end up in the shell history. Blank lines and # comments are
// ignored; an optional "export " prefix and surrounding quotes are stripped.
// The first line of a name wins; the lines it sets aside, a later line
// of a name, a line without a name and a MASSIMO_ENV_FILE line, are
// returned as notes (readFile). A missing file is neither an error nor a
// note.
func LoadFile(path string) ([]Note, error) {
	pairs, notes, err := readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	for _, p := range pairs {
		if os.Getenv(p.k) == "" {
			os.Setenv(p.k, p.v)
			sources[p.k] = p.source()
		} else {
			sources[p.k] = "environment, overrides " + path
		}
	}
	return notes, nil
}

// ReloadFile reads path again while the program runs, as LoadFile
// would at a start: a variable the file set takes its new value, or is
// unset once the file drops it; one the file names now and the process
// environment had not set is set from it; one the environment set before
// the file stays as it is. The file is parsed whole first, so a parse
// error, or a missing file (an editor may be replacing it), changes
// nothing and is returned; a parse error is the one LoadFile returns,
// the line named by its number, never quoted (readFile). It takes the
// lines LoadFile takes, the first of a name, and returns the names whose
// value changed, sorted, and the notes LoadFile returns.
func ReloadFile(path string) (changed []string, notes []Note, err error) {
	pairs, notes, err := readFile(path)
	if err != nil {
		return nil, nil, err
	}
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	named := map[string]bool{}
	for _, p := range pairs {
		named[p.k] = true
		src := sources[p.k]
		if strings.HasPrefix(src, "environment") || src == "" && os.Getenv(p.k) != "" {
			sources[p.k] = "environment, overrides " + path
			continue
		}
		if os.Getenv(p.k) != p.v {
			os.Setenv(p.k, p.v)
			changed = append(changed, p.k)
		}
		sources[p.k] = p.source()
	}
	for k, src := range sources {
		if fromFile(src) && !named[k] {
			os.Unsetenv(k)
			delete(sources, k)
			changed = append(changed, k)
		}
	}
	sort.Strings(changed)
	return slices.Compact(changed), notes, nil
}
