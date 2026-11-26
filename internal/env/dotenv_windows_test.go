// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package env

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// On Windows a name in two cases is one variable, and so one name to
// the settings file's read (NameKey): its first line wins, the later a
// duplicate's note, at a start as at a reload, and a reload of the same
// file changes nothing. A reload that finds the name in the other case,
// either way, keeps it the file's; one that finds its value changed, or
// the name dropped, names it in upper case, as a caller compares it.
func TestNameInTwoCases(t *testing.T) {
	const d = "MASSIMO_TEST_W"
	lower := strings.ToLower(d)
	start, again, changed := readThrice(t, settingsFile(t, d+"=\n"+lower+"=placeholder-secret-2\n"), d)
	note := Note{Lines: []int{1, 2}, Name: d, Text: d + " is given on lines 1 and 2; the first wins"}
	if start.values[d] != "" || start.sources[d] != "file, lines 1 and 2: the first wins" || !slices.EqualFunc(start.notes, []Note{note}, sameNote) {
		t.Errorf("the start took %q, its source %q, notes %v; want the first line's, empty, and %v", start.values[d], start.sources[d], start.notes, note)
	}
	agrees(t, "a name in two cases", start, again, changed)
	quotesNone(t, start.notes)

	for _, c := range []struct {
		what, start, reload, value, source string
		changed                            []string
	}{
		{"upper case after a start in lower", lower + "=placeholder-secret-3\n", d + "=placeholder-secret-3\n", "placeholder-secret-3", "file", nil},
		{"lower case after a start in upper", d + "=placeholder-secret-5\n", lower + "=placeholder-secret-5\n", "placeholder-secret-5", "file", nil},
		{"a new value in lower case", lower + "=placeholder-secret-6\n", lower + "=placeholder-secret-7\n", "placeholder-secret-7", "file", []string{d}},
		{"dropped after a start in lower", lower + "=placeholder-secret-4\n", "# " + d + " taken out\n", "", "environment", []string{d}},
	} {
		p := settingsFile(t, c.start)
		clean(t, d)
		if _, err := LoadFile(p); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(p, []byte(c.reload), 0o600)
		got, notes, err := ReloadFile(p)
		v, set := os.LookupEnv(d)
		if err != nil || !slices.Equal(got, c.changed) || set != (c.value != "") || v != c.value || Source(d) != c.source || Source(lower) != c.source || notes != nil {
			t.Errorf("%s: changed %v, set %v %q, from %q and %q, notes %v, %v; want changed %v, %q from %q",
				c.what, got, set, v, Source(d), Source(lower), notes, err, c.changed, c.value, c.source)
		}
	}
}
