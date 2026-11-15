// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package battery

import (
	"testing"
	"time"
)

// Thursday 2026-11-12 10:00 UTC; the decalcification is Saturday at
// 12:00, two days and two hours later.
var t0 = time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)

func valve(n int) *int { return &n }

func tracker() *Tracker {
	return &Tracker{Location: time.UTC, DecalcOf: func(string) *Decalc { return &Decalc{Day: time.Saturday, Hour: 12} }}
}

// want fails the test where the device "rad" is not at level l.
func want(t *testing.T, tr *Tracker, step string, l Level) {
	t.Helper()
	if got := tr.Level("rad"); got != l {
		t.Fatalf("%s: %s, want %s", step, got, l)
	}
}

// A flicker makes a radiator thermostat weak, not low; the flag held for
// 15 minutes makes it low; new batteries, the flag off for an hour, make
// it ok.
func TestFlickerAndHold(t *testing.T) {
	tr := tracker()
	if c := tr.Report("rad", t0, Report{Flag: true, Valve: valve(0)}); len(c) != 1 || c[0].From != OK || c[0].To != Weak {
		t.Fatalf("first flag: %+v", c)
	}
	tr.Report("rad", t0.Add(3*time.Minute), Report{Valve: valve(0)})
	tr.Tick(t0.Add(30 * time.Minute))
	want(t, tr, "a flicker of 3 minutes", Weak)

	at := t0.Add(time.Hour)
	tr.Report("rad", at, Report{Flag: true, Valve: valve(0)})
	tr.Tick(at.Add(14 * time.Minute))
	want(t, tr, "held 14 minutes", Weak)
	if c := tr.Tick(at.Add(HoldLow)); len(c) != 1 || c[0].From != Weak || c[0].To != Low {
		t.Fatalf("held 15 minutes: %+v", c)
	}
	tr.Report("rad", at.Add(2*time.Hour), Report{Valve: valve(0)})
	tr.Tick(at.Add(2*time.Hour + 59*time.Minute))
	want(t, tr, "off for 59 minutes", Low)
	if c := tr.Tick(at.Add(3 * time.Hour)); len(c) != 1 || c[0].To != OK {
		t.Fatalf("off for an hour: %+v", c)
	}
}

// A weak radiator thermostat is ok once its next decalcification has
// run without the flag; one that shows the flag in it stays weak until
// the next.
func TestDecalcification(t *testing.T) {
	tr := tracker()
	tr.Report("rad", t0, Report{Flag: true, Valve: valve(0)})
	tr.Report("rad", t0.Add(3*time.Minute), Report{Valve: valve(0)})
	sat := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	tr.Tick(sat.Add(14 * time.Minute))
	want(t, tr, "during the decalcification", Weak)
	if c := tr.Tick(sat.Add(DecalcRun)); len(c) != 1 || c[0].To != OK {
		t.Fatalf("after the decalcification: %+v", c)
	}

	tr.Report("rad", sat.Add(7*24*time.Hour+3*time.Minute), Report{Flag: true, Valve: valve(0)})
	tr.Report("rad", sat.Add(7*24*time.Hour+5*time.Minute), Report{Valve: valve(0)})
	tr.Tick(sat.Add(7*24*time.Hour + time.Hour))
	want(t, tr, "the flag in the decalcification", Weak)
	tr.Tick(sat.Add(14*24*time.Hour + DecalcRun))
	want(t, tr, "the next decalcification without it", OK)
}

// A valve move of 50 points without the flag for 10 minutes clears a
// weak radiator thermostat; a smaller one does not.
func TestValveMove(t *testing.T) {
	tr := tracker()
	tr.Report("rad", t0, Report{Flag: true, Valve: valve(10)})
	tr.Report("rad", t0.Add(2*time.Minute), Report{Valve: valve(10)})
	tr.Report("rad", t0.Add(time.Hour), Report{Valve: valve(40)})
	tr.Tick(t0.Add(2 * time.Hour))
	want(t, tr, "a move of 30 points", Weak)
	m := t0.Add(3 * time.Hour)
	tr.Report("rad", m, Report{Valve: valve(100)})
	tr.Tick(m.Add(9 * time.Minute))
	want(t, tr, "9 minutes after a long move", Weak)
	tr.Tick(m.Add(MoveQuiet))
	want(t, tr, "10 minutes after it", OK)
}

// A wall thermostat, without a valve, goes from clear to low through
// weak, and a weak one is ok after a day without the flag.
func TestWithoutValve(t *testing.T) {
	tr := tracker()
	if c := tr.Report("wall", t0, Report{Flag: true}); len(c) != 1 || c[0].To != Weak {
		t.Fatalf("first flag %+v", c)
	}
	tr.Report("wall", t0.Add(time.Minute), Report{})
	tr.Tick(t0.Add(23 * time.Hour))
	if tr.Level("wall") != Weak {
		t.Fatal("ok before a day")
	}
	if c := tr.Tick(t0.Add(ClearWall + time.Minute)); len(c) != 1 || c[0].Device != "wall" || c[0].To != OK {
		t.Fatalf("after a day %+v", c)
	}
	tr.Report("wall", t0.Add(48*time.Hour), Report{Flag: true})
	tr.Report("wall", t0.Add(48*time.Hour+3*time.Minute), Report{Flag: true})
	if c := tr.Tick(t0.Add(48*time.Hour + HoldLow)); len(c) != 1 || c[0].To != Low {
		t.Fatalf("held %+v", c)
	}
}

// An unknown device is ok; a forgotten one starts again.
func TestUnknownAndForgotten(t *testing.T) {
	tr := tracker()
	want(t, tr, "unknown", OK)
	tr.Report("rad", t0, Report{Flag: true})
	tr.Forget("rad")
	want(t, tr, "forgotten", OK)
	if c := tr.Tick(t0.Add(time.Hour)); len(c) != 0 {
		t.Fatalf("a forgotten device changed: %+v", c)
	}
}

// The next decalcification is after the time given, in its time zone.
func TestNextDecalc(t *testing.T) {
	sat := Decalc{Day: time.Saturday, Hour: 12}
	if got := nextDecalc(sat, t0, time.UTC); !got.Equal(time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("from Thursday %v", got)
	}
	at := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	if got := nextDecalc(sat, at, time.UTC); !got.Equal(at.AddDate(0, 0, 7)) {
		t.Fatalf("from the hour itself %v", got)
	}
}
