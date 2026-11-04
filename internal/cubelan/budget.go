// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Budget is what the Cube last reported about its radio and memory limits.
//
// The 868 MHz band allows 1 % transmit time per hour (36 s); the Cube
// reports the used share as a percentage that resets on the hour. Every
// unacknowledged radiogram occupies one of about 50 memory slots until the
// device acknowledges it. Observed on hardware: at 100 % the Cube still
// accepts commands and queues them, so a client must hold back itself.
type Budget struct {
	DutyCyclePct    int       `json:"duty_cycle_pct"`
	FreeMemorySlots int       `json:"free_memory_slots"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type budgetTracker struct {
	mu sync.Mutex
	b  Budget
}

func (t *budgetTracker) update(duty, slots int, now time.Time) {
	t.mu.Lock()
	t.b = Budget{DutyCyclePct: duty, FreeMemorySlots: slots, UpdatedAt: now}
	t.mu.Unlock()
}

func (t *budgetTracker) get() Budget {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.b
}

// Budget returns the latest radio budget, from the last H line or S reply.
func (c *Client) Budget() Budget { return c.budget.get() }

func (c *Client) noteBudget(h *cube.Hello) {
	if h != nil {
		c.budget.update(h.DutyCycle, h.FreeMemorySlots, time.Now())
	}
}

func (c *Client) noteSendResult(r *cube.SendResult) {
	if r != nil {
		c.budget.update(r.DutyCycle, r.FreeMemorySlots, time.Now())
	}
}
