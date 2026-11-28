// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The retention of a target (D12-A), restic's forget policy: the last N
// backups, and the newest of each of the last D days, W weeks, M months
// and Y years that hold one; a backup one of them keeps is kept. Beyond
// restic, a pinned backup and a copy pulled by hand are kept whatever
// the policy, and so is a target's newest copy; a policy of zeros keeps
// everything, as restic refuses an empty one.

import (
	"fmt"
	"slices"
	"time"
)

// Policy is a target's retention, kept as JSON in its row.
type Policy struct {
	Last    int `json:"last,omitempty"`
	Daily   int `json:"daily,omitempty"`
	Weekly  int `json:"weekly,omitempty"`
	Monthly int `json:"monthly,omitempty"`
	Yearly  int `json:"yearly,omitempty"`
}

// DefaultPolicy is the retention of a target added without one: a week
// of days, a month of weeks, half a year of months.
var DefaultPolicy = Policy{Daily: 7, Weekly: 4, Monthly: 6}

// maxKeep bounds each count of a policy.
const maxKeep = 10000

// Check refuses a count below 0 or above 10000.
func (p Policy) Check() error {
	for _, c := range []struct {
		name string
		n    int
	}{{"last", p.Last}, {"daily", p.Daily}, {"weekly", p.Weekly}, {"monthly", p.Monthly}, {"yearly", p.Yearly}} {
		if c.n < 0 || c.n > maxKeep {
			return fmt.Errorf("serverbackup: retention %s %d: between 0 and %d", c.name, c.n, maxKeep)
		}
	}
	return nil
}

// Empty reports whether the policy keeps nothing by itself, and so
// prunes nothing.
func (p Policy) Empty() bool { return p == Policy{} }

// Candidate is a copy on a target as the retention weighs it.
type Candidate struct {
	// UUID is the backup's.
	UUID string
	// Time is when the backup was made, in the box's location, whose
	// days, weeks, months and years the policy counts.
	Time time.Time
	// PinnedUntil keeps it until then (a backup before an update or a
	// restore, one pinned by hand); Pulled keeps it until it is deleted.
	PinnedUntil time.Time
	Pulled      bool
}

// Keep returns the uuids of the backups that policy keeps on a target at
// now, of backups, its copies: those the policy keeps, as restic's
// forget does, those pinned at now or pulled, and the newest. The
// others are pruned. A policy of zeros keeps every one.
func Keep(policy Policy, backups []Candidate, now time.Time) map[string]bool {
	keep := map[string]bool{}
	list := slices.Clone(backups)
	// newest first; restic's order, a tie by uuid so that it is the same
	// on every target
	slices.SortStableFunc(list, func(a, b Candidate) int {
		if c := b.Time.Compare(a.Time); c != 0 {
			return c
		}
		if a.UUID < b.UUID {
			return 1
		}
		if a.UUID > b.UUID {
			return -1
		}
		return 0
	})
	if policy.Empty() {
		for _, b := range list {
			keep[b.UUID] = true
		}
		return keep
	}
	type bucket struct {
		count int
		of    func(time.Time) int
		last  int
		any   bool
	}
	buckets := []*bucket{
		{count: policy.Daily, of: dayOf},
		{count: policy.Weekly, of: weekOf},
		{count: policy.Monthly, of: monthOf},
		{count: policy.Yearly, of: yearOf},
	}
	for i, b := range list {
		if i == 0 || i < policy.Last || b.Pulled || (!b.PinnedUntil.IsZero() && now.Before(b.PinnedUntil)) {
			keep[b.UUID] = true
		}
		for _, k := range buckets {
			if k.count <= 0 {
				continue
			}
			if v := k.of(b.Time); !k.any || v != k.last {
				keep[b.UUID] = true
				k.last, k.any = v, true
				k.count--
			}
		}
	}
	return keep
}
