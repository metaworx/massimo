// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// A Cursor is a place in a watch's items: the number of items fed before
// it. 0 is the watch's start; Mark returns the place after the last item
// fed so far.
type Cursor int

// A Watch keeps every item it is fed, in the order fed, for a test that
// waits for the next one of a kind: marked before the action, it misses
// nothing that came meanwhile, and a wait from the mark skips what does
// not match. On a timeout it says what it did see since the mark.
//
// A feeder calls Add from any goroutine, and End once its feed is over.
// The waits and checks (Await, AwaitN, AwaitAll, Never, Has, HasNot) are
// called from the test's own goroutine, as t.Fatal is; the rest from
// any. The zero Watch is ready to use and shows its items with fmt's
// %+v.
type Watch[T any] struct {
	show func(T) string

	mu    sync.Mutex
	items []T
	more  chan struct{} // closed at the next Add or End; nil while no wait waits
	ended bool
	err   error // why the feed ended, where its End said
}

// shown is how many items a failure shows: the last ones since the mark.
const shown = 20

// clip is how long one item's text may be in a failure, in bytes.
const clip = 240

// NewWatch returns an empty watch whose failures show each item as show
// writes it; a nil show is fmt's %+v, which is the item's String method
// where it has one.
func NewWatch[T any](show func(T) string) *Watch[T] {
	return &Watch[T]{show: show}
}

// Add feeds items to the watch and wakes its waits.
func (w *Watch[T]) Add(items ...T) {
	if len(items) == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.items = append(w.items, items...)
	w.wakeLocked()
}

// End says that the feed is over, err saying why where it is more than
// a plain end: a wait that has not found its item fails at once then,
// rather than at its deadline. Only the first End counts.
func (w *Watch[T]) End(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ended {
		return
	}
	w.ended, w.err = true, err
	w.wakeLocked()
}

// Ended reports whether the feed is over (End): a condition for
// Eventually where a test waits for a stream to end.
func (w *Watch[T]) Ended() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ended
}

// Err returns why the feed ended, as its End said: nil while the feed
// goes on, and where it ended without a reason. For a stream's watch it
// tells an end from a cut: nil where the server ended the stream, the
// read's error where the connection broke off. A test reads it once
// Ended holds.
func (w *Watch[T]) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

// wakeLocked wakes every wait.
func (w *Watch[T]) wakeLocked() {
	if w.more != nil {
		close(w.more)
		w.more = nil
	}
}

// Mark returns the cursor after the last item fed: "from now on", taken
// before the action whose items the test waits for.
func (w *Watch[T]) Mark() Cursor {
	w.mu.Lock()
	defer w.mu.Unlock()
	return Cursor(len(w.items))
}

// Since returns the items fed from the cursor on that match, in the
// order fed; a nil match takes every item. It is for a test that
// compares them as a whole, once it knows that they have all come.
func (w *Watch[T]) Since(from Cursor, match func(T) bool) []T {
	var out []T
	for _, it := range w.from(from) {
		if match == nil || match(it) {
			out = append(out, it)
		}
	}
	return out
}

// Count returns how many of the items fed from the cursor on match; a
// nil match counts every item.
func (w *Watch[T]) Count(from Cursor, match func(T) bool) int {
	n := 0
	for _, it := range w.from(from) {
		if match == nil || match(it) {
			n++
		}
	}
	return n
}

// Seen returns a condition that holds once an item that matches has
// come from the cursor on: the cond of an Eventually or a Within, or the
// end of a Never, as "the next poll came" ends "no second write came".
func (w *Watch[T]) Seen(from Cursor, match func(T) bool) func() bool {
	return func() bool { return w.Count(from, match) > 0 }
}

// from returns the items fed from the cursor on.
func (w *Watch[T]) from(c Cursor) []T {
	items, _ := w.state(c, false)
	return items
}

// state returns the items fed from the cursor on and the feed's state;
// for a wait, with a channel closed at the next Add or End.
func (w *Watch[T]) state(c Cursor, waits bool) ([]T, feed) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c = max(c, 0)
	var items []T
	if int(c) < len(w.items) {
		// Add only appends: the items up to len are not written again,
		// and are read outside the lock.
		items = w.items[c:len(w.items):len(w.items)]
	}
	if waits && w.more == nil && !w.ended {
		w.more = make(chan struct{})
	}
	return items, feed{more: w.more, ended: w.ended, err: w.err}
}

// feed is the state of a watch's feed as a wait read it.
type feed struct {
	more  <-chan struct{} // closed at the next Add or End; nil once ended
	ended bool
	err   error
}

// How a scan ended.
type scanEnd int

const (
	found     scanEnd = iota // look found its item
	endHeld                  // the end held first
	feedEnded                // the feed ended first
	timedOut                 // the wait was over first
)

// scan calls look with each item from the cursor on, as the items come,
// until look reports that it found what it looks for, the feed ends or
// the wait is over. Where end is given, the scan checks it at each item
// and each tick, and ends once it holds, after a last look at the items
// that came; a feed that ended does not end such a scan, since its end
// may still come.
func (w *Watch[T]) scan(from Cursor, wt wait, look func(T, Cursor) bool, end func() bool) scanEnd {
	timer := time.NewTimer(wt.d)
	defer timer.Stop()
	var ticks <-chan time.Time
	if end != nil {
		tk := time.NewTicker(wt.tick)
		defer tk.Stop()
		ticks = tk.C
	}
	next := max(from, 0)
	looked := func() bool {
		items, _ := w.state(next, false)
		for i, it := range items {
			if look(it, next+Cursor(i)) {
				return true
			}
		}
		next += Cursor(len(items))
		return false
	}
	over := false // the wait is over: one more look at what came
	for {
		_, st := w.state(next, true)
		if looked() {
			return found
		}
		if end != nil && end() {
			if looked() {
				return found
			}
			return endHeld
		}
		if st.ended && end == nil {
			return feedEnded
		}
		if over {
			if st.ended {
				return feedEnded
			}
			return timedOut
		}
		select {
		case <-st.more:
		case <-ticks:
		case <-timer.C:
			over = true
		}
	}
}

// Await waits from the cursor for the first item that matches, skipping
// every other, and returns it with the cursor after it, from which a
// next wait goes on. It fails the test where none has come within Wait,
// scaled, or where the feed ended first, and the failure says what came
// since the mark. A nil match takes the next item. The options are
// Eventually's.
func (w *Watch[T]) Await(t testing.TB, from Cursor, what string, match func(T) bool, opts ...Option) (T, Cursor) {
	t.Helper()
	wt := newWait(opts)
	var got T
	var after Cursor
	end := w.scan(from, wt, func(it T, c Cursor) bool {
		if match == nil || match(it) {
			got, after = it, c+1
			return true
		}
		return false
	}, nil)
	if end != found {
		t.Fatalf("%s", w.failure(end, what, from, wt))
	}
	return got, after
}

// AwaitN waits from the cursor for n items that match, skipping every
// other, and returns them in the order fed, with the cursor after the
// last of them: "three failed retries". It waits and fails the test as
// Await does, and the failure says how many of the n came. An n of 0 or
// less returns nothing, and the cursor given, at once.
func (w *Watch[T]) AwaitN(t testing.TB, from Cursor, what string, n int, match func(T) bool, opts ...Option) ([]T, Cursor) {
	t.Helper()
	if n <= 0 {
		return nil, from
	}
	wt := newWait(opts)
	got := make([]T, 0, n)
	var after Cursor
	end := w.scan(from, wt, func(it T, c Cursor) bool {
		if match == nil || match(it) {
			got, after = append(got, it), c+1
		}
		return len(got) == n
	}, nil)
	if end != found {
		t.Fatalf("%s", w.failure(end, fmt.Sprintf("%s (%d of %d came)", what, len(got), n), from, wt))
	}
	return got, after
}

// AwaitAll waits from the cursor until each match has met an item, in
// any order, each item meeting one match at most, and returns the items
// in the order of the matches, with the cursor after the last of them.
// It waits Wait, scaled, and fails the test as Await does, saying which
// matches met no item.
func (w *Watch[T]) AwaitAll(t testing.TB, from Cursor, what string, matches ...func(T) bool) ([]T, Cursor) {
	t.Helper()
	wt := newWait(nil)
	var a assignment[T]
	a.matches = matches
	a.item = make([]int, len(matches))
	for m := range a.item {
		a.item[m] = -1
	}
	left := len(matches)
	if left > 0 {
		if end := w.scan(from, wt, func(it T, c Cursor) bool {
			if a.add(it, c) {
				left--
			}
			return left == 0
		}, nil); end != found {
			var missing []string
			for m, i := range a.item {
				if i < 0 {
					missing = append(missing, fmt.Sprint(m+1))
				}
			}
			t.Fatalf("%s", w.failure(end, fmt.Sprintf("%s (%d of %d met, not met: %s)", what, len(matches)-left, len(matches), strings.Join(missing, ", ")), from, wt))
			return nil, from
		}
	}
	out := make([]T, len(matches))
	after := from
	for m, i := range a.item {
		out[m] = a.seen[i]
		after = max(after, a.at[i]+1)
	}
	return out, after
}

// assignment gives AwaitAll's matches their items: each match one item,
// each item one match at most.
type assignment[T any] struct {
	matches []func(T) bool
	item    []int    // the index in seen of each match's item; -1: none yet
	seen    []T      // the items looked at
	at      []Cursor // where each of them is in the watch
}

// add looks at the next item and reports whether it gave one match more
// an item: it takes a match without one, or one whose item can move to
// another match it meets (an augmenting path), so that items meeting
// more than one match cannot make a wait fail that has an item for each.
func (a *assignment[T]) add(it T, c Cursor) bool {
	a.seen = append(a.seen, it)
	a.at = append(a.at, c)
	return a.place(len(a.seen)-1, make([]bool, len(a.matches)))
}

// place finds the item i a match, moving the items of the matches
// visited where that frees one.
func (a *assignment[T]) place(i int, visited []bool) bool {
	for m, match := range a.matches {
		if visited[m] || (match != nil && !match(a.seen[i])) {
			continue
		}
		visited[m] = true
		if a.item[m] < 0 || a.place(a.item[m], visited) {
			a.item[m] = i
			return true
		}
	}
	return false
}

// Never is the check that no item that matches comes from the cursor on
// before end holds, what naming it, as testkit.Never is: it fails the
// test at the first that comes, or where end has not held within Wait,
// scaled; once end holds, the items that came meanwhile are looked at
// once more. The end is what shows that the item would have come by then
// - a later item, often of this watch (Seen) - never a fixed time. A nil
// match takes every item. The options are testkit.Never's.
func (w *Watch[T]) Never(t testing.TB, from Cursor, what string, match func(T) bool, end func() bool, opts ...Option) {
	t.Helper()
	wt := newWait(opts)
	var came T
	switch w.scan(from, wt, func(it T, _ Cursor) bool {
		if match == nil || match(it) {
			came = it
			return true
		}
		return false
	}, end) {
	case found:
		t.Fatalf("came, and was not to: %s: %s; %s", what, w.text(came), w.seen(from))
	case endHeld:
	default:
		t.Fatalf("%s", w.failure(timedOut, "the end of the check for "+what, from, wt))
	}
}

// Has is the check that an item that matches has come from the cursor
// on, made once the action that feeds it is over - the process ended,
// the call returned with its log written - so that nothing of it can
// still be on its way: it returns the first such item, with the cursor
// after it, and fails the test at once where none came, saying what
// did. Where the item may still be on its way, the test waits for it
// with Await. A nil match takes the first item.
func (w *Watch[T]) Has(t testing.TB, from Cursor, what string, match func(T) bool) (T, Cursor) {
	t.Helper()
	start := max(from, 0)
	for i, it := range w.from(from) {
		if match == nil || match(it) {
			return it, start + Cursor(i) + 1
		}
	}
	t.Fatalf("has not come: %s; %s", what, w.seen(from))
	var none T
	return none, from
}

// HasNot is the check that no item that matches has come from the
// cursor on, made once the action that feeds it is over, as Has is: it
// fails the test at once at the first that came, as Never does. Where
// the item could still come, the check is Never, with an end that shows
// it would have come by then. A nil match takes every item.
func (w *Watch[T]) HasNot(t testing.TB, from Cursor, what string, match func(T) bool) {
	t.Helper()
	for _, it := range w.from(from) {
		if match == nil || match(it) {
			t.Fatalf("came, and was not to: %s: %s; %s", what, w.text(it), w.seen(from))
			return
		}
	}
}

// failure is a wait's message where its scan did not find what it waited
// for.
func (w *Watch[T]) failure(end scanEnd, what string, from Cursor, wt wait) string {
	if end == feedEnded {
		_, st := w.state(from, false)
		msg := "the feed ended before " + what
		if st.err != nil {
			msg += ": " + st.err.Error()
		}
		return msg + "; " + w.seen(from)
	}
	return fmt.Sprintf("timed out after %v waiting for %s; %s", wt.d, what, w.seen(from))
}

// seen says what came from the cursor on, for a failure: how many items,
// and the last of them, numbered from the mark on.
func (w *Watch[T]) seen(from Cursor) string {
	items := w.from(from)
	if len(items) == 0 {
		return "nothing since the mark"
	}
	var b strings.Builder
	first := 0
	if len(items) > shown {
		first = len(items) - shown
		fmt.Fprintf(&b, "%d since the mark, the last %d:", len(items), shown)
	} else {
		fmt.Fprintf(&b, "%d since the mark:", len(items))
	}
	for i := first; i < len(items); i++ {
		fmt.Fprintf(&b, "\n\t%d %s", i+1, w.text(items[i]))
	}
	return b.String()
}

// text is an item as a failure shows it, clipped to clip bytes.
func (w *Watch[T]) text(it T) string {
	var s string
	if w.show != nil {
		s = w.show(it)
	} else {
		s = fmt.Sprintf("%+v", it)
	}
	if len(s) <= clip {
		return s
	}
	cut := clip
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// ChanWatch returns a watch fed from ch, a channel the test would read
// itself, such as a Cube client's events: it reads ch until ch is
// closed, which ends the watch, or until the test ends, and the test
// reads ch no more. show is NewWatch's.
func ChanWatch[T any](t testing.TB, ch <-chan T, show func(T) string) *Watch[T] {
	w := NewWatch(show)
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case v, ok := <-ch:
				if !ok {
					w.End(nil)
					return
				}
				w.Add(v)
			case <-stop:
				return
			}
		}
	}()
	t.Cleanup(func() {
		close(stop)
		<-done
	})
	return w
}
