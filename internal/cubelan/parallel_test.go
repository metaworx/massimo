// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

// The Cube client's tests run in parallel (AP TestSpeed, Block 8,
// D11-A): every test calls t.Parallel() before anything else, but those
// on serialTests, which Go runs one after the other before the parallel
// ones begin; a test that swaps a package variable says so through
// testkit.Swapping. docs/testing.md, "Parallel tests".

import (
	"testing"

	"go.metaworx.ch/massimo/internal/testkit"
)

// serialTests are the tests that do not run in parallel, and why: each
// checks a time that the tests beside it would stretch. The network
// configuration's waits are an option of each call (NetWaits), not a
// package variable to swap.
var serialTests = map[string]string{
	"TestWaitingSendEndsWithItsContext": "the Send returns within 1 s of its context's 50 ms",
}

// Every test of the package calls t.Parallel() as its first statement,
// or is on serialTests with its reason and does not call it
// (testkit.ParallelOrListed).
func TestParallelOrListed(t *testing.T) {
	t.Parallel()
	testkit.ParallelOrListed(t, ".", serialTests)
}
