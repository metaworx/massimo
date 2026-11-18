// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"slices"
	"testing"
	"time"
)

// The clock stands still but for Add, and runs the timers Add passes in
// the order of their times, a stopped one never, one a timer sets at the
// next Add.
func TestClock(t *testing.T) {
	at := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	c := NewClock(at)
	if !c.Now().Equal(at) {
		t.Fatalf("now %v, want %v", c.Now(), at)
	}
	var fired []string
	c.AfterFunc(2*time.Hour, func() { fired = append(fired, "two hours") })
	c.AfterFunc(time.Hour, func() {
		fired = append(fired, "an hour at "+c.Now().Format("15:04"))
		c.AfterFunc(0, func() { fired = append(fired, "set by the hour's") })
	})
	stopped := c.AfterFunc(30*time.Minute, func() { fired = append(fired, "stopped") })
	if !stopped.Stop() || stopped.Stop() {
		t.Fatal("Stop reported a timer stopped twice as pending")
	}
	if n := c.Pending(); n != 2 {
		t.Fatalf("%d timers pending, want 2", n)
	}
	c.Add(59 * time.Minute)
	if len(fired) != 0 {
		t.Fatalf("fired before their time: %v", fired)
	}
	c.Add(2 * time.Hour)
	if want := []string{"an hour at 14:59", "two hours"}; !slices.Equal(fired, want) {
		t.Fatalf("fired %v, want %v", fired, want)
	}
	if n := c.Pending(); n != 1 {
		t.Fatalf("%d timers pending, want the one set by a timer", n)
	}
	c.Add(0)
	if fired[len(fired)-1] != "set by the hour's" || c.Pending() != 0 {
		t.Fatalf("fired %v, %d pending", fired, c.Pending())
	}
	if !c.Now().Equal(at.Add(179 * time.Minute)) {
		t.Fatalf("now %v", c.Now())
	}
}
