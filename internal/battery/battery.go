// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package battery keeps each device's battery level, ok, weak or low,
// from the history of its battery flag (byte 6's 0x80 of its L entry).
//
// On the owner's Cubes in November 2026 (docs/cube-protocol.md), a
// radiator thermostat's flag flickered for one to four minutes at a
// time, days before it stayed on: the device reports when its battery
// state changes, both ways, and it changes first under the load of the
// valve motor, as in the weekly decalcification. A wall thermostat, with
// no motor, went from clear to steady at once. So one flag makes a
// device weak, a flag held makes it low, and a weak device is ok again
// once a long valve move passes without the flag: its next
// decalcification, or a valve move of 50 points or more.
package battery

import (
	"sync"
	"time"
)

// Level is a device's battery level.
type Level string

// The levels.
const (
	OK   Level = "ok"   // no flag, or not since a long valve move
	Weak Level = "weak" // the flag seen, under load
	Low  Level = "low"  // the flag held
)

// The rules' times and the valve move that counts.
const (
	// HoldLow is how long the flag stays on before the device is low;
	// the longest flicker seen lasted 4 minutes.
	HoldLow = 15 * time.Minute
	// ClearLow is how long a low device's flag stays off before the
	// device is ok again: a held flag does not go by itself, new
	// batteries make it go.
	ClearLow = time.Hour
	// DecalcRun is how long after its hour a decalcification has run.
	DecalcRun = 15 * time.Minute
	// ClearWall is how long a weak device without a valve, a wall
	// thermostat or a contact, stays without the flag before it is ok.
	ClearWall = 24 * time.Hour
	// MoveMin is the valve move, in points, that counts as a long one,
	// and MoveQuiet how long after it the flag must stay away.
	MoveMin   = 50
	MoveQuiet = 10 * time.Minute
)

// Decalc is a radiator thermostat's weekly decalcification: the day, as
// the time package numbers it, and the hour, the device's local time.
type Decalc struct {
	Day  time.Weekday
	Hour int
}

// Report is what the tracker reads of one state of a device that the
// Cube reported (its valid bit set).
type Report struct {
	Flag  bool // the battery flag
	Valve *int // a radiator thermostat's valve position; nil for a device without a valve
}

// Change is a device's level that changed, at At.
type Change struct {
	Device   string
	From, To Level
	At       time.Time
}

// Tracker keeps the levels of one Cube's devices. It is safe for
// concurrent use.
type Tracker struct {
	// DecalcOf returns a device's decalcification, nil where it has
	// none or it is unknown; it is asked when the rules are evaluated,
	// so that it reads the configuration as it is then.
	DecalcOf func(device string) *Decalc
	// Location is the decalcification's time zone; nil is time.Local.
	Location *time.Location

	mu   sync.Mutex
	devs map[string]*device
}

// device is one device's history, as far as the rules need it.
type device struct {
	level    Level
	on       bool
	onSince  time.Time // the flag's last change to on
	offSince time.Time // its last change to off
	lastOn   time.Time // the last time it was seen on
	valved   bool      // the device has a valve
	valve    int
	moveAt   time.Time // the last long valve move
}

// Report takes one state of a device at the time at, and returns the
// changes of its level.
func (t *Tracker) Report(dev string, at time.Time, r Report) []Change {
	t.mu.Lock()
	defer t.mu.Unlock()
	d := t.device(dev)
	if r.Valve != nil {
		if d.valved && abs(*r.Valve-d.valve) >= MoveMin {
			d.moveAt = at
		}
		d.valved, d.valve = true, *r.Valve
	}
	var out []Change
	if r.Flag {
		if !d.on {
			d.on, d.onSince = true, at
		}
		d.lastOn = at
		if d.level == OK {
			out = append(out, d.set(dev, Weak, at))
		}
	} else if d.on {
		d.on, d.offSince = false, at
	}
	return append(out, t.evaluate(dev, d, at)...)
}

// Tick evaluates the rules that depend on time alone, for every device,
// at the time at, and returns the changes of their levels.
func (t *Tracker) Tick(at time.Time) []Change {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []Change
	for dev, d := range t.devs {
		out = append(out, t.evaluate(dev, d, at)...)
	}
	return out
}

// Level returns a device's level; OK for a device the tracker has not
// seen.
func (t *Tracker) Level(dev string) Level {
	t.mu.Lock()
	defer t.mu.Unlock()
	if d, ok := t.devs[dev]; ok {
		return d.level
	}
	return OK
}

// Forget drops a device, one removed from the Cube.
func (t *Tracker) Forget(dev string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.devs, dev)
}

func (t *Tracker) device(dev string) *device {
	if t.devs == nil {
		t.devs = map[string]*device{}
	}
	d, ok := t.devs[dev]
	if !ok {
		d = &device{level: OK}
		t.devs[dev] = d
	}
	return d
}

// evaluate applies the rules of time to a device: the flag held makes it
// low; a low one whose flag stayed off for ClearLow is ok; a weak one is
// ok once a long valve move has passed without the flag.
func (t *Tracker) evaluate(dev string, d *device, at time.Time) []Change {
	switch {
	case d.on && d.level != Low && at.Sub(d.onSince) >= HoldLow:
		return []Change{d.set(dev, Low, at)}
	case d.level == Low && !d.on && at.Sub(d.offSince) >= ClearLow:
		return []Change{d.set(dev, OK, at)}
	case d.level == Weak && !d.on && t.passed(dev, d, at):
		return []Change{d.set(dev, OK, at)}
	}
	return nil
}

// passed reports whether a long valve move has passed without the flag
// since the flag was last seen: the device's next decalcification, or a
// valve move of MoveMin points, with MoveQuiet after it; for a device
// without a valve, ClearWall without the flag.
func (t *Tracker) passed(dev string, d *device, at time.Time) bool {
	if !d.valved {
		return at.Sub(d.lastOn) >= ClearWall
	}
	if d.moveAt.After(d.lastOn) && at.Sub(d.moveAt) >= MoveQuiet {
		return true
	}
	if t.DecalcOf == nil {
		return false
	}
	dc := t.DecalcOf(dev)
	if dc == nil {
		return false
	}
	next := nextDecalc(*dc, d.lastOn, t.Location)
	return !at.Before(next.Add(DecalcRun))
}

// nextDecalc is the first decalcification after the time after.
func nextDecalc(dc Decalc, after time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	a := after.In(loc)
	for i := 0; i <= 7; i++ {
		day := a.AddDate(0, 0, i)
		c := time.Date(day.Year(), day.Month(), day.Day(), dc.Hour, 0, 0, 0, loc)
		if c.Weekday() == dc.Day && c.After(after) {
			return c
		}
	}
	return a.AddDate(0, 0, 8) // not reached for a valid day and hour
}

func (d *device) set(dev string, to Level, at time.Time) Change {
	c := Change{Device: dev, From: d.level, To: to, At: at}
	d.level = to
	return c
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
