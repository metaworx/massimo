// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build !unix && !windows

package serverbackup

import "errors"

// diskFree cannot tell the free space on this system; the check is
// skipped where it fails.
func diskFree(string) (uint64, error) { return 0, errors.ErrUnsupported }
