// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"testing"
	"time"
)

// Pending counts the slots in use: every one of them when none is free,
// as on a Cube whose radiograms wait for devices that never answer; none
// while the Cube has not reported its budget.
func TestBudgetPending(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name string
		b    Budget
		want int
	}{
		{"never reported", Budget{}, 0},
		{"idle", Budget{FreeMemorySlots: 50, MaxSlots: 50, UpdatedAt: now}, 0},
		{"three held", Budget{FreeMemorySlots: 47, MaxSlots: 50, UpdatedAt: now}, 3},
		{"one free", Budget{FreeMemorySlots: 1, MaxSlots: 50, UpdatedAt: now}, 49},
		{"none free", Budget{FreeMemorySlots: 0, MaxSlots: 50, UpdatedAt: now}, 50},
	} {
		if got := c.b.Pending(); got != c.want {
			t.Errorf("%s: %d pending, want %d", c.name, got, c.want)
		}
	}
}
