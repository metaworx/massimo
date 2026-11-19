// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The server's tests run in parallel (AP TestSpeed, Block 7, D11-A):
// every test calls t.Parallel() before anything else, but those on
// serialTests, which Go runs one after the other before the parallel
// ones begin; a test that swaps a package variable says so through
// testkit.Swapping. docs/testing.md, "Parallel tests".

import (
	"testing"

	"go.metaworx.ch/massimo/internal/testkit"
)

// serialTests are the tests that do not run in parallel, and why: each
// changes something every test of the package shares, binds an address
// a test beside it could bind too, or checks a time that tests beside
// it would stretch.
var serialTests = map[string]string{
	// A package variable swapped while the test runs.
	"TestFullName":               "swaps lookupCNAME",
	"TestGetSystemInterfaces":    "swaps hostInterfaces",
	"TestProviderIssuerAtSaving": "swaps oidcClient (fakeDiscovery)",
	"TestProvidersAPI":           "swaps oidcClient (fakeDiscovery)",
	"TestRebootBlocked":          "swaps rebootStuck",
	"TestWallReset":              "sets wallSilence to 0",

	// A fixed port: a Cube added through the API as the only one gets
	// the proxy on the Cube's port, :62910, with the discovery port
	// :23272 where the server has no address of its own for it.
	"TestConcurrentAddCube":          "listens on :62910 and :23272",
	"TestDeleteAndAddCube":           "listens on :62910 and :23272",
	"TestRemovedCubeIsListedAsFound": "listens on :62910",

	// A time the test checks as the product's, which the load of the
	// tests beside it would stretch.
	"TestBackupAutomatic":                   "the backup at backups.auto_max within 5 s",
	"TestStepObserverDoesNotWaitOnTheStore": "the observer's three calls within 500 ms",
}

// Every test of the package calls t.Parallel() as its first statement,
// or is on serialTests with its reason and does not call it
// (testkit.ParallelOrListed).
func TestParallelOrListed(t *testing.T) {
	t.Parallel()
	testkit.ParallelOrListed(t, ".", serialTests)
}
