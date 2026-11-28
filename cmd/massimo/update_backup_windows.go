// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// storeOfAnother is false on Windows: an administrator writes the
// service's store under %ProgramData% as the service does.
func storeOfAnother(string) bool { return false }
