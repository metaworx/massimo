// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package idioms holds the guard of the UI tests' old idioms (AP
// SteadyTests, Blocks 4 and 6): per test file of tools/ui-test, its fixed
// pauses (wait_for_timeout), its plain routes (page.route, not
// helpers.py's safe_route), its own definitions of a name helpers.py
// defines, and its page scripts' writes of the page's state
// (state.<field>), each counted against its list in this directory. A
// list may only fall: a count above its listed one fails `go test`, and
// so does a count below it, until the commit that lowered it lowers the
// list too (docs/ui-testing.md, section 1). It has no code of its own;
// idioms_test.go is the guard.
package idioms
