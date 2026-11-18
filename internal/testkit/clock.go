// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"slices"
	"sync"
	"time"
)

// Clock is a clock a test moves on, in place of the real time, so that
// a test of a day, a window or a timer takes no time at all: Now is the
// time it was given, moved on by every Add since, and a timer of
// AfterFunc runs once Add has passed its time. It never moves by
// itself. A product that takes a clock as an interface with its own
// timer type takes it through a small adapter of the test's package,
// whose AfterFunc returns this one's timer as that type.
//
// A Clock is safe for use by several goroutines.
type Clock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*ClockTimer
}

// ClockTimer is a timer of a Clock (Clock.AfterFunc).
type ClockTimer struct {
	c       *Clock
	at      time.Time
	f       func()
	stopped bool // stopped, or fired
}

// NewClock returns a clock that stands at at.
func NewClock(at time.Time) *Clock { return &Clock{now: at} }

// Now is the clock's time.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// AfterFunc returns a timer that calls f once the clock has been moved
// on by d or more, from the Add that moves it there.
func (c *Clock) AfterFunc(d time.Duration, f func()) *ClockTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &ClockTimer{c: c, at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

// Stop keeps the timer from firing, and reports whether it would still
// have: false where it fired or was stopped already, as time.Timer's.
func (t *ClockTimer) Stop() bool {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	was := !t.stopped
	t.stopped = true
	return was
}

// Add moves the clock on by d, then runs the timers that came due, in
// the order of their times, on the caller's goroutine, without the
// clock's lock: a timer may read the clock, and set another. One set by
// a timer that is due already runs at the next Add.
func (c *Clock) Add(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []*ClockTimer
	kept := c.timers[:0]
	for _, t := range c.timers {
		switch {
		case t.stopped:
		case !t.at.After(c.now):
			t.stopped = true
			due = append(due, t)
		default:
			kept = append(kept, t)
		}
	}
	clear(c.timers[len(kept):])
	c.timers = kept
	c.mu.Unlock()
	slices.SortStableFunc(due, func(a, b *ClockTimer) int { return a.at.Compare(b.at) })
	for _, t := range due {
		t.f()
	}
}

// Pending counts the timers that have neither fired nor been stopped.
func (c *Clock) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, t := range c.timers {
		if !t.stopped {
			n++
		}
	}
	return n
}
