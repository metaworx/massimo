// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serve

// serve's tests run in parallel (AP TestSpeed, Block 8, D11-A): every
// test calls t.Parallel() before anything else, but those on
// serialTests, which Go runs one after the other before the parallel
// ones begin; a test that swaps a package variable says so through
// testkit.Swapping. docs/testing.md, "Parallel tests".

import (
	"testing"

	"go.metaworx.ch/massimo/internal/testkit"
)

// serialTests are the tests that do not run in parallel, and why. serve
// reads the settings file's Cubes from the process environment
// (FileCubes), so a test of what it takes from there sets the
// environment; making it parallel would take an environment of its own
// in Config, which is the product's change, not the tests'.
var serialTests = map[string]string{
	// Cube variables in the process environment (clearCubeEnv).
	"TestOpenRunsAgainstFakeCube":             "sets a Cube's address",
	"TestServeCubesConfiguredBySerial":        "sets Cube variables",
	"TestServeLowerCaseSerial":                "sets a Cube's address",
	"TestServePlainAddressOfTheNamedCube":     "sets Cube variables",
	"TestServePlainSeedCutShort":              "loads a settings file into the environment",
	"TestServeRemovedCubeStaysRemoved":        "sets Cube variables",
	"TestServeSeveralCubesNeedNamedListeners": "sets Cube variables",
	"TestServeSingleCubeWithoutSerial":        "loads a settings file into the environment",
	"TestServeStoreWins":                      "sets Cube variables",

	// A package variable swapped while the test runs.
	"TestRunAnnounces":          "swaps startAnnounce and browseHomeAssistant (fakeAnnounce)",
	"TestRunAnnouncesBySetting": "swaps startAnnounce and browseHomeAssistant (fakeAnnounce)",
	"TestRunUnannouncedWarns":   "swaps startAnnounce and browseHomeAssistant (fakeAnnounce)",
}

// Every test of the package calls t.Parallel() as its first statement,
// or is on serialTests with its reason and does not call it
// (testkit.ParallelOrListed).
func TestParallelOrListed(t *testing.T) {
	t.Parallel()
	testkit.ParallelOrListed(t, ".", serialTests)
}
