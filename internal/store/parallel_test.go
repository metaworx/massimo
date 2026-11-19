// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The store's tests run in parallel (AP TestSpeed, Block 8, D11-A):
// every test calls t.Parallel() before anything else, but those on
// serialTests, which Go runs one after the other before the parallel
// ones begin; a test that swaps a package variable says so through
// testkit.Swapping. docs/testing.md, "Parallel tests".

import (
	"testing"

	"go.metaworx.ch/massimo/internal/testkit"
)

// serialTests are the tests that do not run in parallel, and why: each
// changes a package variable that every store of the package reads, or
// measures what the whole process allocates.
var serialTests = map[string]string{
	// A package variable swapped while the test runs.
	"TestEventsDropped":         "sets eventBuffer to 8",
	"TestEventsPruneStops":      "sets pruneChunk and pruneStep",
	"TestEventsPruneYields":     "sets pruneChunk and pruneStep",
	"TestEventsPruned":          "sets pruneChunk to 7",
	"TestNoHashForAKnownAnswer": "swaps argonKey",
	"TestPasswordChecksBounded": "swaps argonKey",

	// The process's allocations (runtime.MemStats.TotalAlloc), which
	// the tests beside it would add to.
	"TestPasswordMemory":     "a hash's allocations, about 19 MiB",
	"TestPasswordParameters": "a refused hash allocates at most 1 MiB",
}

// Every test of the package calls t.Parallel() as its first statement,
// or is on serialTests with its reason and does not call it
// (testkit.ParallelOrListed).
func TestParallelOrListed(t *testing.T) {
	t.Parallel()
	testkit.ParallelOrListed(t, ".", serialTests)
}
