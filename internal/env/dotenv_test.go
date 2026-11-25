// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package env

import (
	"bufio"
	"encoding/binary"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestLoadDotEnv(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(p, []byte("# comment\n\nMASSIMO_TEST_A=\"k0011\"\nexport MASSIMO_TEST_B = 'x y'\nMASSIMO_TEST_C=plain\n"), 0o600)
	t.Setenv("MASSIMO_TEST_C", "already")
	if _, err := LoadFile(p); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("MASSIMO_TEST_A") != "k0011" || os.Getenv("MASSIMO_TEST_B") != "x y" || os.Getenv("MASSIMO_TEST_C") != "already" {
		t.Fatalf("got %q %q %q", os.Getenv("MASSIMO_TEST_A"), os.Getenv("MASSIMO_TEST_B"), os.Getenv("MASSIMO_TEST_C"))
	}
	if _, err := LoadFile(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("missing file must not be an error: %v", err)
	}
	os.WriteFile(p, []byte("garbage\n"), 0o600)
	if _, err := LoadFile(p); err == nil {
		t.Fatal("want error for malformed line")
	}
}

// A line that does not parse, or one too long to be read, is named by
// its number, at the start as at a reload, and never quoted: a value
// wrapped onto a line of its own is half a secret, and a long line may
// be a whole one. Neither sets anything.
func TestParseErrorQuotesNoLine(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ what, text, want string }{
		{"a line without =", "MASSIMO_TEST_Q=placeholder-token-\nwrapped-secret-9\n", ":2: expected KEY=VALUE"},
		{"a line too long", "MASSIMO_TEST_Q=placeholder-token-1\nMASSIMO_TEST_Q2=" + strings.Repeat("s", bufio.MaxScanTokenSize) + "\n",
			":2: " + bufio.ErrTooLong.Error()},
	} {
		p := filepath.Join(t.TempDir(), ".env")
		if err := os.WriteFile(p, []byte(c.text), 0o600); err != nil {
			t.Fatal(err)
		}
		want := p + c.want
		changed, _, reloadErr := ReloadFile(p)
		_, loadErr := LoadFile(p)
		for _, f := range []struct {
			what string
			err  error
		}{{"LoadFile", loadErr}, {"ReloadFile", reloadErr}} {
			if f.err == nil || f.err.Error() != want {
				t.Errorf("%s, %s: %v, want %s", c.what, f.what, f.err, want)
			}
		}
		_, set1 := os.LookupEnv("MASSIMO_TEST_Q")
		_, set2 := os.LookupEnv("MASSIMO_TEST_Q2")
		if set1 || set2 || changed != nil {
			t.Errorf("%s: set %v %v, changed %v", c.what, set1, set2, changed)
		}
	}
}

// File is MASSIMO_ENV_FILE, else ./.env, or the default file
// SetDefaultFile named, which MASSIMO_ENV_FILE still overrides.
func TestFileDefault(t *testing.T) {
	t.Cleanup(func() { SetDefaultFile(".env") })
	t.Setenv(FileVar, "")
	if f := File(); f != ".env" {
		t.Fatalf("without MASSIMO_ENV_FILE: %q", f)
	}
	SetDefaultFile("/etc/massimo/env")
	if f := File(); f != "/etc/massimo/env" {
		t.Fatalf("with a default file: %q", f)
	}
	t.Setenv(FileVar, "given.env")
	if f := File(); f != "given.env" {
		t.Fatalf("MASSIMO_ENV_FILE over the default file: %q", f)
	}
}

// clean gives the test a record of sources of its own, and registers
// the names its files set, so that they are unset again after it.
func clean(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		t.Setenv(n, "")
		os.Unsetenv(n)
	}
	sourcesMu.Lock()
	saved := sources
	sources = map[string]string{}
	sourcesMu.Unlock()
	t.Cleanup(func() {
		sourcesMu.Lock()
		sources = saved
		sourcesMu.Unlock()
	})
}

// A file read again while the program runs changes what it set, sets
// what it names now, unsets what it dropped, and leaves what the
// environment set before it; a parse error or a missing file changes
// nothing.
func TestReloadFile(t *testing.T) {
	clean(t, "MASSIMO_TEST_R1", "MASSIMO_TEST_R2", "MASSIMO_TEST_R3", "MASSIMO_TEST_R4")
	t.Setenv("MASSIMO_TEST_RENV", "from-the-environment")
	p := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(p, []byte("MASSIMO_TEST_R1=one\nMASSIMO_TEST_R2=two\nMASSIMO_TEST_RENV=from-the-file\n"), 0o600)
	if _, err := LoadFile(p); err != nil {
		t.Fatal(err)
	}
	if changed, _, err := ReloadFile(p); err != nil || len(changed) != 0 {
		t.Fatalf("an unchanged file: %v %v", changed, err)
	}
	os.WriteFile(p, []byte("MASSIMO_TEST_R1=uno\nMASSIMO_TEST_R3=three\nMASSIMO_TEST_R3=ignored\nMASSIMO_TEST_RENV=again-the-file\n"), 0o600)
	changed, _, err := ReloadFile(p)
	if err != nil || strings.Join(changed, ",") != "MASSIMO_TEST_R1,MASSIMO_TEST_R2,MASSIMO_TEST_R3" {
		t.Fatalf("changed %v %v", changed, err)
	}
	if v, ok := os.LookupEnv("MASSIMO_TEST_R2"); ok {
		t.Errorf("a dropped variable is still set: %q", v)
	}
	if os.Getenv("MASSIMO_TEST_R1") != "uno" || os.Getenv("MASSIMO_TEST_R3") != "three" || os.Getenv("MASSIMO_TEST_RENV") != "from-the-environment" {
		t.Errorf("got %q %q %q", os.Getenv("MASSIMO_TEST_R1"), os.Getenv("MASSIMO_TEST_R3"), os.Getenv("MASSIMO_TEST_RENV"))
	}
	if Source("MASSIMO_TEST_R3") != "file, lines 2 and 3: the first wins" || !strings.HasPrefix(Source("MASSIMO_TEST_RENV"), "environment, overrides") {
		t.Errorf("sources %q %q", Source("MASSIMO_TEST_R3"), Source("MASSIMO_TEST_RENV"))
	}
	if changed, _, _ := ReloadFile(p); len(changed) != 0 {
		t.Errorf("a name given twice changes again: %v", changed)
	}
	os.WriteFile(p, []byte("MASSIMO_TEST_R4=four\ngarbage\n"), 0o600)
	if changed, _, err := ReloadFile(p); err == nil || changed != nil || os.Getenv("MASSIMO_TEST_R4") != "" || os.Getenv("MASSIMO_TEST_R1") != "uno" {
		t.Errorf("a parse error: %v %v", changed, err)
	}
	os.Remove(p)
	if changed, _, err := ReloadFile(p); !errors.Is(err, os.ErrNotExist) || changed != nil || os.Getenv("MASSIMO_TEST_R1") != "uno" {
		t.Errorf("a missing file: %v %v", changed, err)
	}
}

// settingsFile writes text to a settings file of the test's own.
func settingsFile(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// read is what a read of a settings file left of the names a test asks
// for: their values and sources, the names it changed and its notes.
type read struct {
	values, sources map[string]string
	changed         []string
	notes           []Note
}

// readThrice reads the settings file p as a start does (LoadFile), as a
// reload does of the same file after that start (ReloadFile), and as a
// reload does that finds the file changed to it, in a process whose
// start found no file; the first and the last each in a process of its
// own (clean). A read that fails fails the test.
func readThrice(t *testing.T, p string, names ...string) (start, again, changed read) {
	t.Helper()
	look := func(r *read) {
		r.values, r.sources = map[string]string{}, map[string]string{}
		for _, n := range names {
			r.values[n], r.sources[n] = os.Getenv(n), Source(n)
		}
	}
	var err error
	clean(t, names...)
	if start.notes, err = LoadFile(p); err != nil {
		t.Fatal(err)
	}
	look(&start)
	if again.changed, again.notes, err = ReloadFile(p); err != nil {
		t.Fatal(err)
	}
	look(&again)
	clean(t, names...)
	if changed.changed, changed.notes, err = ReloadFile(p); err != nil {
		t.Fatal(err)
	}
	look(&changed)
	return start, again, changed
}

// agrees fails the test where a reload leaves values, sources or notes
// other than the start's, or a reload of the start's file changes
// anything.
func agrees(t *testing.T, what string, start, again, changed read) {
	t.Helper()
	if len(again.changed) != 0 {
		t.Errorf("%s: a reload of the start's file changed %v", what, again.changed)
	}
	for _, r := range []struct {
		what string
		read read
	}{{"a reload of the start's file", again}, {"a reload that finds the file", changed}} {
		if !maps.Equal(r.read.values, start.values) || !maps.Equal(r.read.sources, start.sources) || !slices.EqualFunc(r.read.notes, start.notes, sameNote) {
			t.Errorf("%s: %s left %q %q %v, the start %q %q %v", what, r.what,
				r.read.values, r.read.sources, r.read.notes, start.values, start.sources, start.notes)
		}
	}
}

// sameNote reports whether two notes name the same lines and say the same.
func sameNote(a, b Note) bool {
	return slices.Equal(a.Lines, b.Lines) && a.Name == b.Name && a.Text == b.Text
}

// quotesNone fails the test where a note quotes a value: each of the
// test's values holds "placeholder-secret".
func quotesNone(t *testing.T, notes []Note) {
	t.Helper()
	for _, n := range notes {
		if strings.Contains(n.Text, "placeholder-secret") {
			t.Errorf("a note quotes a value: %q", n.Text)
		}
	}
}

// A name given more than once takes its first line, an empty one too,
// at a start as at a reload; its later lines set nothing, and are a
// note with every line's number, which the source names too.
func TestFirstLineWins(t *testing.T) {
	const d = "MASSIMO_TEST_D"
	for _, c := range []struct {
		what, text, value, lines string
		numbers                  []int
	}{
		{"an empty first line", d + "=\n" + d + "=placeholder-secret-5\n", "", "1 and 2", []int{1, 2}},
		{"a value on the first line", d + "=placeholder-secret-3\n# a comment\n" + d + "=placeholder-secret-4\n",
			"placeholder-secret-3", "1 and 3", []int{1, 3}},
		{"three lines", d + "='placeholder-secret-a'\n" + d + "=placeholder-secret-b\n\nexport " + d + "=placeholder-secret-c\n",
			"placeholder-secret-a", "1, 2 and 4", []int{1, 2, 4}},
	} {
		start, again, changed := readThrice(t, settingsFile(t, c.text), d)
		source := "file, lines " + c.lines + ": the first wins"
		note := Note{Lines: c.numbers, Name: d, Text: d + " is given on lines " + c.lines + "; the first wins"}
		if start.values[d] != c.value || start.sources[d] != source || !slices.EqualFunc(start.notes, []Note{note}, sameNote) {
			t.Errorf("%s: the start took %q, its source %q, notes %v; want %q, %q, %v",
				c.what, start.values[d], start.sources[d], start.notes, c.value, source, note)
		}
		agrees(t, c.what, start, again, changed)
		quotesNone(t, start.notes)
	}

	// The start's later line no longer makes the variable the
	// environment's, which no reload would change.
	p := settingsFile(t, d+"=placeholder-secret-3\n"+d+"=placeholder-secret-4\n")
	clean(t, d)
	if _, err := LoadFile(p); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(p, []byte(d+"=placeholder-secret-5\n"), 0o600)
	changed, notes, err := ReloadFile(p)
	if err != nil || !slices.Equal(changed, []string{d}) || os.Getenv(d) != "placeholder-secret-5" || Source(d) != "file" || notes != nil {
		t.Errorf("the first line changed after a start: changed %v, %q from %q, notes %v, %v", changed, os.Getenv(d), Source(d), notes, err)
	}

	// A reload of a file that no longer names it unsets it.
	p = settingsFile(t, d+"=placeholder-secret-3\n"+d+"=placeholder-secret-4\n")
	clean(t, d)
	if _, err := LoadFile(p); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(p, []byte("# "+d+" taken out\n"), 0o600)
	changed, notes, err = ReloadFile(p)
	if _, set := os.LookupEnv(d); set || err != nil || !slices.Equal(changed, []string{d}) || Source(d) != "environment" || notes != nil {
		t.Errorf("dropped after a start: set %v, changed %v, its source %q, notes %v, %v", set, changed, Source(d), notes, err)
	}

	// The process environment still wins over every line, and says so.
	clean(t, d)
	t.Setenv(d, "from-the-environment")
	p = settingsFile(t, d+"=placeholder-secret-a\n"+d+"=placeholder-secret-b\n")
	atStart, err := LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	changed, notes, err = ReloadFile(p)
	want := "environment, overrides " + p
	if err != nil || os.Getenv(d) != "from-the-environment" || Source(d) != want || len(changed) != 0 ||
		len(atStart) != 1 || !slices.EqualFunc(notes, atStart, sameNote) {
		t.Errorf("the environment's value: %q from %q, changed %v, notes %v and %v, %v", os.Getenv(d), Source(d), changed, atStart, notes, err)
	}
}

// A UTF-8 byte order mark at the start of the file, which Windows'
// editors and PowerShell write, is skipped, before a setting as before
// a comment without "=", the template's first line.
func TestByteOrderMark(t *testing.T) {
	const b = "MASSIMO_TEST_BOM"
	for _, c := range []struct{ what, text string }{
		{"before a setting", "\xef\xbb\xbf" + b + "=1\n"},
		{"before a comment without =", "\xef\xbb\xbf# Settings of massimo serve, read by the service with -env (see README.md\n" + b + "=1\n"},
		{"before a comment in another script", "\xef\xbb\xbf# Réglages\r\n" + b + "=1\r\n"},
	} {
		start, again, changed := readThrice(t, settingsFile(t, c.text), b)
		if start.values[b] != "1" || start.sources[b] != "file" || start.notes != nil {
			t.Errorf("%s: %q from %q, notes %v", c.what, start.values[b], start.sources[b], start.notes)
		}
		agrees(t, c.what, start, again, changed)
		for _, kv := range os.Environ() {
			if strings.HasPrefix(kv, "\xef\xbb\xbf") {
				t.Errorf("%s: the byte order mark is part of a name: %q", c.what, strings.Split(kv, "=")[0])
				os.Unsetenv(strings.Split(kv, "=")[0])
			}
		}
	}
}

// UTF-16 text is refused at a start as at a reload, and sets nothing: a
// file of it, as PowerShell 5.1's > writes it, of either byte order and
// with or without its byte order mark, and lines of it after UTF-8 ones,
// as its >> appends them, named by the first of them.
func TestUTF16Refused(t *testing.T) {
	const u, a, ps = "MASSIMO_TEST_U16", "MASSIMO_TEST_MA", "MASSIMO_TEST_PS"
	clean(t, u, a, ps)
	for _, c := range []struct {
		what  string
		utf8  string // the UTF-8 lines before the UTF-16 text
		order binary.AppendByteOrder
		bom   bool
		text  string
		at    string // the line named
	}{
		{"UTF-16LE with its byte order mark", "", binary.LittleEndian, true, "# Settings\r\n" + u + "=1\r\n", ""},
		{"UTF-16LE", "", binary.LittleEndian, false, u + "=1\r\n", ""},
		{"UTF-16LE, its first line empty", "", binary.LittleEndian, false, "\n" + u + "=1\n", ""},
		{"UTF-16BE with its byte order mark", "", binary.BigEndian, true, u + "=1\n", ""},
		{"UTF-16BE", "", binary.BigEndian, false, "\n" + u + "=1\n", ""},
		// What PowerShell 5.1's >> wrote after the two UTF-8 lines.
		{"UTF-16LE appended to UTF-8", "# head\r\n" + a + "=1\r\n", binary.LittleEndian, false, ps + "=3\r\n", ":3"},
	} {
		units := utf16.Encode([]rune(c.text))
		if c.bom {
			units = append([]uint16{0xFEFF}, units...)
		}
		text := []byte(c.utf8)
		for _, w := range units {
			text = c.order.AppendUint16(text, w)
		}
		p := settingsFile(t, string(text))
		want := p + c.at + ": UTF-16 text, save it as UTF-8"
		changed, reloadNotes, reloadErr := ReloadFile(p)
		startNotes, loadErr := LoadFile(p)
		for _, f := range []struct {
			what string
			err  error
		}{{"LoadFile", loadErr}, {"ReloadFile", reloadErr}} {
			if f.err == nil || f.err.Error() != want {
				t.Errorf("%s, %s: %v, want %s", c.what, f.what, f.err, want)
			}
		}
		for _, n := range []string{u, a, ps} {
			if _, set := os.LookupEnv(n); set {
				t.Errorf("%s: %s set", c.what, n)
			}
		}
		if changed != nil || reloadNotes != nil || startNotes != nil {
			t.Errorf("%s: changed %v, notes %v %v", c.what, changed, startNotes, reloadNotes)
		}
	}
}

// A name given twice that is no variable's name, as a secret wrapped onto
// a line of its own may give one, is named by its lines alone: neither
// the note's text nor its Name quotes it, at a start as at a reload; a
// variable's name given twice is named (AP StartConfiguration, Block 2).
func TestDuplicateOfNoVariableName(t *testing.T) {
	const n, wrapped = "MASSIMO_TEST_W", "placeholder-secret+wrapped/1"
	p := settingsFile(t, n+"=1\n"+wrapped+"==\n"+n+"=2\n"+wrapped+"==\n")
	start, again, changed := readThrice(t, p, n, wrapped)
	want := []Note{
		{Lines: []int{1, 3}, Name: n, Text: n + " is given on lines 1 and 3; the first wins"},
		{Lines: []int{2, 4}, Text: "lines 2 and 4 give the same name; the first wins"},
	}
	if !slices.EqualFunc(start.notes, want, sameNote) {
		t.Errorf("the notes %v, want %v", start.notes, want)
	}
	agrees(t, "a name that is no variable's", start, again, changed)
	quotesNone(t, start.notes)
	for _, c := range []struct {
		name string
		ok   bool
	}{
		{"MASSIMO_X", true}, {"_X1", true}, {"X", true}, {"", false}, {"1X", false}, {"A-B", false},
		{"c2VjcmV0", false}, {"c2Vj+cmV0", false}, {"massimo_x", false}, {"a/b", false}, {"\u00e4", false}, {"X Y", false},
	} {
		if varName(c.name) != c.ok {
			t.Errorf("varName(%q) = %v, want %v", c.name, !c.ok, c.ok)
		}
	}
}

// A line without a name, "=x", sets nothing and is a note, which does
// not quote its value; a reload no longer reports an empty name
// changed. The notes come in the order of the lines they name first.
func TestLineWithoutName(t *testing.T) {
	const n = "MASSIMO_TEST_N"
	p := settingsFile(t, n+"=1\n=placeholder-secret-1\n"+n+"=2\n  export = 'placeholder-secret-2'\n")
	start, again, changed := readThrice(t, p, n)
	want := []Note{
		{Lines: []int{1, 3}, Name: n, Text: n + " is given on lines 1 and 3; the first wins"},
		{Lines: []int{2}, Text: "line 2 has no name before its =, ignored"},
		{Lines: []int{4}, Text: "line 4 has no name before its =, ignored"},
	}
	if start.values[n] != "1" || !slices.EqualFunc(start.notes, want, sameNote) {
		t.Errorf("the start: %q, notes %v, want %v", start.values[n], start.notes, want)
	}
	if !slices.Equal(changed.changed, []string{n}) {
		t.Errorf("a reload that finds the file changed %q, want %s alone", changed.changed, n)
	}
	agrees(t, "a line without a name", start, again, changed)
	quotesNone(t, start.notes)
}

// A MASSIMO_ENV_FILE line in the settings file, in any case, as Windows
// takes the name, sets nothing and is a note, so that the file File
// names stays the one the start read, at a reload too, which would
// otherwise read the other file and take its token as the owner's key.
// The variable the process environment sets, as -env does, still names
// the file.
func TestEnvFileLineIgnored(t *testing.T) {
	const f, lower = "MASSIMO_TEST_F", "massimo_env_file"
	clean(t, f, FileVar, lower)
	t.Cleanup(func() { SetDefaultFile(".env") })
	dir := t.TempDir()
	p, other := filepath.Join(dir, "first.env"), filepath.Join(dir, "other.env")
	os.WriteFile(other, []byte(f+"=the-other-file\n"), 0o600)
	os.WriteFile(p, []byte(f+"=the-first-file\nexport "+FileVar+"="+other+"\n"+lower+"="+other+"\n"), 0o600)
	SetDefaultFile(p)
	want := []Note{
		{Lines: []int{2}, Name: FileVar, Text: "line 2 sets MASSIMO_ENV_FILE, ignored: name the file with -env"},
		{Lines: []int{3}, Name: lower, Text: "line 3 sets MASSIMO_ENV_FILE, ignored: name the file with -env"},
	}
	check := func(what string, notes []Note, err error, fileVar string) {
		t.Helper()
		v, set := os.LookupEnv(FileVar)
		// On Windows the lower-case name is MASSIMO_ENV_FILE.
		lv, lset := os.LookupEnv(lower)
		if err != nil || File() != p || set != (fileVar != "") || v != fileVar || lset && lv != fileVar ||
			os.Getenv(f) != "the-first-file" || !slices.EqualFunc(notes, want, sameNote) {
			t.Errorf("%s: File() %q, %s %q (set %v), %s %q (set %v), %s %q, notes %v, %v",
				what, File(), FileVar, v, set, lower, lv, lset, f, os.Getenv(f), notes, err)
		}
	}
	notes, err := LoadFile(File())
	check("the start", notes, err, "")
	changed, notes, err := ReloadFile(File())
	check("a reload", notes, err, "")
	if len(changed) != 0 {
		t.Errorf("a reload changed %v", changed)
	}

	t.Setenv(FileVar, p) // as -env does
	notes, err = LoadFile(File())
	check("the start with -env", notes, err, p)
	_, notes, err = ReloadFile(File())
	check("a reload with -env", notes, err, p)
}
