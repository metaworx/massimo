// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package password holds the rule a new password is held to wherever
// Massimo sees it: massimo user (add, passwd, rename) and the plain
// password field of the users' API (AP PasswordRules, D4-A, D7-A,
// D11-A). A password has MinLength to MaxLength code points, counted
// as typed: not normalised, so that "é" typed as e and a combining
// accent is two, and no character classes are asked for. It contains
// no word of its context, whatever the case of either (D50-A): the
// user's names and the server's, which the caller gives, and Massimo's
// and the Cube's, which are always there (Always); no list of common
// passwords is held (AP PasswordRules, the NOTE of v1.0 on it). A sign-in
// takes any password that opens a user's record or hash, of any length,
// and so does the upgrade of a version-1 user at its sign-in; a record
// registered by OPAQUE is never seen, so never checked.
//
// The cases the rule is tested with are testdata/cases.json, which the
// page's own check is held to as well: an object whose "cases" is a list
// of objects, each with
//
//   - "password": the password, a JSON string;
//   - "context": the words it is checked with (Check's words), a list,
//     possibly empty;
//   - "answer": "ok", "too_short", "too_long" or "context";
//   - "why": what the case shows, for the reader;
//
// and whose "hosts" is a list of the server's names as a host gives
// them, for HostWord, each with
//
//   - "host": the host, as os.Hostname or a request's Host gives it;
//   - "word": HostWord's answer, "" for none;
//   - "why": what the case shows.
package password

import (
	"fmt"
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MinLength and MaxLength are the fewest and the most code points a new
// password has.
const (
	MinLength = 8
	MaxLength = 64
)

var (
	// ErrTooShort is a password of fewer than MinLength code points; its
	// text is what a client of the API reads in the answer.
	ErrTooShort = fmt.Errorf("password: at least %d characters", MinLength)
	// ErrTooLong is a password of more than MaxLength code points.
	ErrTooLong = fmt.Errorf("password: at most %d characters", MaxLength)
	// ErrContext is a password that contains a word of its context. Its
	// text names no word, so that the API's answer tells a caller
	// neither the server's host name nor another user's names.
	ErrContext = fmt.Errorf("password: contains the user's or the server's name")
)

// MinWord is the fewest letters a word of the context has: a shorter one
// is no word, so that a name such as "op7" refuses nothing.
const MinWord = 4

// Always are the names every new password is checked against beside the
// caller's: Massimo's and the Cube's.
var Always = []string{"massimo", "maxcube"}

// Check reports whether pw may be set as a new password: nil, or
// ErrTooShort or ErrTooLong, then ErrContext. A byte that is not UTF-8
// counts as one character. The empty password is too short: where empty
// means none (a user with keys only, a password removed), the caller
// does not call Check.
//
// names are the names the password is checked against beside Always:
// the user's login name and display name, the server's name. Each gives
// its words: the name itself, without what is neither a letter nor a
// digit at either end, and each run of letters in it (letters with
// their combining marks: "Anna Berg" gives "Anna Berg", "Anna" and
// "Berg", "anna2026" gives "anna2026" and "anna"). A word is taken where
// it has MinWord letters or more, so that an empty or a short name, or
// one of digits, is none; pw is refused where it contains a word, each
// lower-cased a code point at a time by Unicode's simple mapping
// (unicode.ToLower), which the page's check does alike: "İ" is "i", and
// "Σ" is "σ" wherever it stands.
func Check(pw string, names ...string) error {
	switch n := utf8.RuneCountInString(pw); {
	case n < MinLength:
		return ErrTooShort
	case n > MaxLength:
		return ErrTooLong
	}
	folded := fold(pw)
	for _, w := range words(names) {
		if strings.Contains(folded, w) {
			return ErrContext
		}
	}
	return nil
}

// words are the words of Always and of names, as Check takes them,
// lower-cased.
func words(names []string) []string {
	var out []string
	add := func(w string) {
		n := 0
		for _, r := range w {
			if unicode.IsLetter(r) {
				n++
			}
		}
		if n >= MinWord {
			out = append(out, fold(w))
		}
	}
	for _, name := range append(append([]string{}, Always...), names...) {
		add(strings.TrimFunc(name, func(r rune) bool { return !inWord(r) && !unicode.IsDigit(r) }))
		for _, run := range strings.FieldsFunc(name, func(r rune) bool { return !inWord(r) }) {
			add(run)
		}
	}
	return out
}

// inWord reports whether r belongs to a run of letters: a letter or a
// combining mark.
func inWord(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r)
}

// fold lower-cases s a code point at a time.
func fold(s string) string {
	return strings.Map(unicode.ToLower, s)
}

// HostWord is the server's name a host gives, for Check: the first
// label of a host name, without what follows a single colon, its port,
// as os.Hostname or a request's Host gives it ("kitchen" of
// "kitchen.example.org:8443"); "" for anything in brackets or with more
// than one colon (an IPv6 address), for an IPv4 address, and for
// localhost, lower-cased as Check lower-cases, which name no server. The
// page's hostWord answers alike.
func HostWord(host string) string {
	h := strings.TrimSpace(host)
	if strings.HasPrefix(h, "[") || strings.Count(h, ":") > 1 {
		return ""
	}
	h, _, _ = strings.Cut(h, ":")
	if a, err := netip.ParseAddr(h); err == nil && a.Is4() {
		return ""
	}
	label, _, _ := strings.Cut(h, ".")
	if fold(label) == "localhost" {
		return ""
	}
	return label
}
