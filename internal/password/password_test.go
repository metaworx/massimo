// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package password

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

// testCase is one case of testdata/cases.json (the package comment).
type testCase struct {
	Password string   `json:"password"`
	Context  []string `json:"context"`
	Answer   string   `json:"answer"`
	Why      string   `json:"why"`
}

// answers are the answers cases.json names, by their error.
var answers = map[string]error{"ok": nil, "too_short": ErrTooShort, "too_long": ErrTooLong}

func readCases(t *testing.T) []testCase {
	t.Helper()
	b, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Cases []testCase `json:"cases"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		t.Fatalf("cases.json: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("cases.json has no case")
	}
	return f.Cases
}

// TestCases checks every case of cases.json: the answer it names, for
// the reason it gives.
func TestCases(t *testing.T) {
	for _, c := range readCases(t) {
		want, ok := answers[c.Answer]
		if !ok {
			t.Fatalf("%q: unknown answer %q", c.Why, c.Answer)
		}
		if c.Why == "" || c.Context == nil {
			t.Fatalf("%q: a case has a why and a context, if empty", c.Password)
		}
		if err := Check(c.Password, c.Context...); !errors.Is(err, want) || (want == nil) != (err == nil) {
			t.Errorf("%s: Check(%q) = %v, want %v", c.Why, c.Password, err, want)
		}
	}
}

// TestCasesCountCodePoints holds cases.json to what it is for: at each
// edge of the rule (7, 8, 64 and 65 code points) a password in ASCII,
// one of letters of two bytes, whose bytes answer otherwise, and one
// beyond the basic plane, whose UTF-16 units answer otherwise, so that a
// check counting either fails.
func TestCasesCountCodePoints(t *testing.T) {
	have := map[string]bool{}
	for _, c := range readCases(t) {
		n := utf8.RuneCountInString(c.Password)
		kind := "ascii"
		switch {
		case len(utf16.Encode([]rune(c.Password))) != n:
			kind = "utf16"
		case len(c.Password) != n:
			kind = "bytes"
		}
		have[fmt.Sprintf("%d %s %s", n, kind, c.Answer)] = true
	}
	for _, e := range []struct {
		n      int
		answer string
	}{{MinLength - 1, "too_short"}, {MinLength, "ok"}, {MaxLength, "ok"}, {MaxLength + 1, "too_long"}} {
		for _, kind := range []string{"ascii", "bytes", "utf16"} {
			if k := fmt.Sprintf("%d %s %s", e.n, kind, e.answer); !have[k] {
				t.Errorf("cases.json has no case %q", k)
			}
		}
	}
}

// TestErrorTexts holds the errors to the rule's numbers: they are what
// a client of the API and the command line read.
func TestErrorTexts(t *testing.T) {
	if ErrTooShort.Error() != "password: at least 8 characters" || ErrTooLong.Error() != "password: at most 64 characters" {
		t.Fatalf("the texts: %q, %q", ErrTooShort, ErrTooLong)
	}
}

// TestNotUTF8 checks that bytes that are not UTF-8 count one each and
// do not panic.
func TestNotUTF8(t *testing.T) {
	if err := Check("\xff\xfe\xfd\xfc\xfb\xfa\xf9"); !errors.Is(err, ErrTooShort) {
		t.Fatalf("seven bad bytes: %v", err)
	}
	if err := Check("\xff\xfe\xfd\xfc\xfb\xfa\xf9\xf8"); err != nil {
		t.Fatalf("eight bad bytes: %v", err)
	}
}
