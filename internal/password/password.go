// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package password holds the rule a new password is held to wherever
// Massimo sees it: massimo user (add, passwd, rename) and the plain
// password field of the users' API (AP PasswordRules, D4-A, D7-A,
// D11-A). A password has MinLength to MaxLength code points, counted
// as typed: not normalised, so that "é" typed as e and a combining
// accent is two, and no character classes are asked for. A sign-in
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
//   - "answer": "ok", "too_short" or "too_long";
//   - "why": what the case shows, for the reader.
package password

import (
	"fmt"
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
)

// Check reports whether pw may be set as a new password: nil, or
// ErrTooShort or ErrTooLong. A byte that is not UTF-8 counts as one
// character. The empty password is too short: where empty means none (a
// user with keys only, a password removed), the caller does not call
// Check. words are the words the password is checked against, such as
// the user's names; Check takes them from every caller and refuses
// nothing for them yet.
func Check(pw string, words ...string) error {
	switch n := utf8.RuneCountInString(pw); {
	case n < MinLength:
		return ErrTooShort
	case n > MaxLength:
		return ErrTooLong
	}
	return nil
}
