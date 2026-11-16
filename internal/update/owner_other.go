// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !unix

package update

import "os/exec"

// runAsOwner leaves cmd as it is: on Windows the service and the update
// helper both run as LocalSystem.
func runAsOwner(*exec.Cmd, string) error { return nil }
