// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"os"
	"testing"
)

// testHost is the host's name the tests give the server, so that no test
// reads the machine's, whose first label could be a word of a fixture's
// password (password.Check).
const testHost = "kitchen.example.org"

func TestMain(m *testing.M) {
	hostName = func() (string, error) { return testHost, nil }
	os.Exit(m.Run())
}
