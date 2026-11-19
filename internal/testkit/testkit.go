// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package testkit is what the tests share for waiting: one scale for
// every wait, a wait until a condition holds, a check that something
// does not happen before a given end, and a watch of what comes - log
// records, a stream's events, a channel's values - that waits for the
// next one of a kind (Watch), a clock the test moves on in place of
// waiting for the real one (Clock), a port the test holds while it
// hands its listener on (Listen, ListenPacket), and the rule of parallel
// tests, which a package's guard holds its tests to (ParallelOrListed,
// Swapping). Its subpackage cubewatch watches the lines a fake Cube
// receives.
//
// Every wait is scaled by MASSIMO_TEST_SCALE, a positive number, 1 where
// it is not set: make race sets 2 and make test-armv6 4, so that a slow
// runner waits longer and a fast one no longer than it needs. A wait
// that holds returns at once, so the scale costs time only where a test
// would fail.
//
// Only test files import the package (TestOnlyTestsImportTheKit).
// docs/testing.md says how a test waits.
package testkit

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Wait is how long a wait lasts where it does not say otherwise, before
// the scale.
const Wait = 5 * time.Second

// Tick is how often a wait checks its condition where it does not say
// otherwise: often enough to go on soon after it holds, seldom enough
// that a condition asking the server's API does not load a busy runner.
const Tick = 10 * time.Millisecond

// ScaleVar is the environment variable that scales every wait.
const ScaleVar = "MASSIMO_TEST_SCALE"

// scale is ScaleVar's value, read once; a value that is not a positive
// number stops the tests at their first wait, rather than letting them
// run with waits nobody asked for.
var scale = sync.OnceValue(func() float64 {
	f, err := parseScale(os.Getenv(ScaleVar))
	if err != nil {
		panic("testkit: " + err.Error())
	}
	return f
})

// parseScale reads a value of ScaleVar: empty is 1.
func parseScale(v string) (float64, error) {
	if v == "" {
		return 1, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || !(f > 0) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("%s is a positive number, not %q", ScaleVar, v)
	}
	return f, nil
}

// Scale returns d times MASSIMO_TEST_SCALE: the time a test waits for
// something that takes d on an unhurried machine.
func Scale(d time.Duration) time.Duration {
	return time.Duration(float64(d) * scale())
}

// An Option changes how long a wait lasts or how often it checks.
type Option func(*wait)

// For makes a wait last d, before the scale, in place of Wait.
func For(d time.Duration) Option { return func(w *wait) { w.d = d } }

// Every makes a wait check its condition every d, in place of Tick; a d
// of 0 or less is Tick.
func Every(d time.Duration) Option { return func(w *wait) { w.tick = d } }

// wait is a wait's length, scaled, and its tick.
type wait struct{ d, tick time.Duration }

// newWait applies the options to Wait and Tick, and scales the length.
func newWait(opts []Option) wait {
	w := wait{d: Wait, tick: Tick}
	for _, o := range opts {
		o(&w)
	}
	if w.tick <= 0 {
		w.tick = Tick // a wait that never sleeps would spin
	}
	w.d = Scale(w.d)
	return w
}

// poll checks cond every tick until it holds or the wait is over; the
// last sleep ends with the wait, and cond is checked once more then.
func poll(w wait, cond func() bool) bool {
	deadline := time.Now().Add(w.d)
	for {
		if cond() {
			return true
		}
		if !sleep(deadline, w.tick) {
			return false
		}
	}
}

// sleep sleeps a tick, or until the deadline where that comes first, and
// reports whether there was time left to sleep.
func sleep(deadline time.Time, tick time.Duration) bool {
	left := time.Until(deadline)
	if left <= 0 {
		return false
	}
	time.Sleep(min(tick, left))
	return true
}

// Eventually checks cond until it holds, and fails the test where it has
// not held within Wait, scaled: "timed out after 5s waiting for <what>".
// Like t.Fatal, it is called from the test's own goroutine.
func Eventually(t testing.TB, what string, cond func() bool, opts ...Option) {
	t.Helper()
	w := newWait(opts)
	if !poll(w, cond) {
		t.Fatalf("timed out after %v waiting for %s", w.d, what)
	}
}

// Within reports whether cond holds within d, scaled, and fails nothing:
// for a test that says itself what it saw where the wait ran out
// (`if !testkit.Within(…) { t.Fatalf(…) }`), or goes on either way. The
// options are Eventually's; d stands for For(d).
func Within(d time.Duration, cond func() bool, opts ...Option) bool {
	return poll(newWait(append([]Option{For(d)}, opts...)), cond)
}

// Never is the check that something did not happen, what naming it ("a
// backup without a change"): it fails the test where cond holds before
// end does, or where end has not held within Wait, scaled. Once end
// holds, cond is checked once more. The end is what shows that the thing
// would have happened by then - a later line, an event, a count - never
// a fixed time.
func Never(t testing.TB, what string, cond, end func() bool, opts ...Option) {
	t.Helper()
	w := newWait(opts)
	deadline := time.Now().Add(w.d)
	for {
		if cond() {
			t.Fatalf("came, and was not to: %s", what)
			return
		}
		if end() {
			if cond() {
				t.Fatalf("came, and was not to: %s", what)
			}
			return
		}
		if !sleep(deadline, w.tick) {
			t.Fatalf("timed out after %v waiting for the end of the check for %s", w.d, what)
			return
		}
	}
}
