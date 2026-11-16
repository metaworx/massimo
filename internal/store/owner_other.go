// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !unix

package store

// giveOwner does nothing where files have no Unix owner: on Windows the
// service and the update helper both run as LocalSystem.
func giveOwner(path, ref string) error { return nil }
