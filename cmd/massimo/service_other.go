// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !windows

package main

// serviceDataDir is the data directory of a Windows service started
// without -data; elsewhere the installers always pass -data.
func serviceDataDir() string { return "" }
