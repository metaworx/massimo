// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package engine

// Pending reports whether a command that match accepts waits in the
// queue or runs now. The server's room checks ask it, so that an
// automatic correction never joins a command to the same room that is
// still on its way (AP RadioHealth v1.2, Block 9). match is called with
// the queue's lock held: it must return at once and not call the queue.
func (q *Queue) Pending(match func(Command) bool) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.inFlght != nil && match(q.inFlght.cmd) {
		return true
	}
	for _, e := range q.items {
		if match(e.cmd) {
			return true
		}
	}
	return false
}

// Holding reports whether a radio command would wait now, as
// waitForBudget holds it: the Cube's duty cycle at MaxDutyPct, or its
// free memory slots below MinFreeSlots. An automatic correction waits
// for a later check then, behind the hold (AP RadioHealth v1.2, Block 9).
func (q *Queue) Holding() bool {
	b := q.gw.Budget()
	return b.DutyCyclePct >= q.opts.MaxDutyPct || !b.UpdatedAt.IsZero() && b.FreeMemorySlots < q.opts.MinFreeSlots
}
