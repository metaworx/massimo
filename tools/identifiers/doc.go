// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package identifiers holds the check that keeps real identifiers out of
// the repository (the project contract, "Secrets and identities"): a Cube
// or device serial, a MAC address or a radio address that is not a
// placeholder fails `go test`, in the tracked files and in the decoded
// protocol fixtures, unless allowed.txt names it with its reason. It has
// no code of its own; identifiers_test.go is the check.
package identifiers
