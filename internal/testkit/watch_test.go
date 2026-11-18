// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// is returns a match for the item s.
func is(s string) func(string) bool { return func(v string) bool { return v == s } }

// prefixed returns a match for an item that starts with p.
func prefixed(p string) func(string) bool {
	return func(v string) bool { return strings.HasPrefix(v, p) }
}

// later feeds w the items one by one from a goroutine of its own, a
// millisecond apart, so that a wait meets them as they come.
func later(w *Watch[string], items ...string) {
	go func() {
		for _, it := range items {
			time.Sleep(time.Millisecond)
			w.Add(it)
		}
	}()
}

// Await skips what does not match and returns the first item that does,
// with the cursor after it; what came before the mark does not count.
func TestAwaitSkipsTheRest(t *testing.T) {
	w := NewWatch[string](nil)
	w.Add("s:old")
	m := w.Mark()
	later(w, "l:", "f:", "s:one", "l:", "s:two")
	got, next := w.Await(t, m, "an s: line", prefixed("s:"))
	if got != "s:one" || next != m+3 {
		t.Fatalf("got %q at %d, want s:one at %d", got, next, m+3)
	}
	if got, _ := w.Await(t, next, "the next s: line", prefixed("s:")); got != "s:two" {
		t.Fatalf("from the cursor after the first: %q", got)
	}
	// Both have come: a wait from the mark finds the first again.
	if got, _ := w.Await(t, m, "an s: line", prefixed("s:")); got != "s:one" {
		t.Fatalf("again from the mark: %q", got)
	}
	if got, _ := w.Await(t, m, "the next item", nil); got != "l:" {
		t.Fatalf("a nil match: %q", got)
	}
}

// A wait is woken by the item it waits for: it does not wait for its
// deadline or a tick.
func TestAwaitIsWoken(t *testing.T) {
	w := NewWatch[string](nil)
	m := w.Mark()
	go func() {
		time.Sleep(20 * time.Millisecond)
		w.Add("the one")
	}()
	start := time.Now()
	w.Await(t, m, "the one", is("the one"), For(30*time.Second))
	if d := time.Since(start); d > Scale(10*time.Second) {
		t.Fatalf("returned after %v", d)
	}
}

// A wait that times out says what it waited for, how long, and what
// came since the mark; more than twenty items show as the last twenty,
// numbered from the mark on.
func TestAwaitFailureShowsWhatCame(t *testing.T) {
	w := NewWatch(func(s string) string { return "<" + s + ">" })
	w.Add("before")
	m := w.Mark()
	w.Add("l:", "f:")
	f := run(t, func(tb testing.TB) { w.Await(tb, m, "an s: line", prefixed("s:"), For(20*time.Millisecond)) })
	want := fmt.Sprintf("timed out after %v waiting for an s: line; 2 since the mark:\n\t1 <l:>\n\t2 <f:>", Scale(20*time.Millisecond))
	if f != want {
		t.Errorf("failure:\n%s\nwant:\n%s", f, want)
	}

	empty := NewWatch[string](nil)
	f = run(t, func(tb testing.TB) { empty.Await(tb, empty.Mark(), "anything", nil, For(time.Millisecond)) })
	if !strings.HasSuffix(f, "waiting for anything; nothing since the mark") {
		t.Errorf("nothing came: %q", f)
	}

	many := NewWatch[string](nil)
	for i := 1; i <= 25; i++ {
		many.Add(fmt.Sprintf("l:%d", i))
	}
	f = run(t, func(tb testing.TB) { many.Await(tb, 0, "an s: line", prefixed("s:"), For(time.Millisecond)) })
	if !strings.Contains(f, "; 25 since the mark, the last 20:\n\t6 l:6\n") || !strings.HasSuffix(f, "\n\t25 l:25") || strings.Contains(f, "\t5 l:5") {
		t.Errorf("25 items:\n%s", f)
	}
}

// An item's text is clipped in a failure, on a rune's start.
func TestFailureClipsLongItems(t *testing.T) {
	w := NewWatch[string](nil)
	w.Add(strings.Repeat("é", clip))
	f := run(t, func(tb testing.TB) { w.Await(tb, 0, "x", is("x"), For(time.Millisecond)) })
	_, item, _ := strings.Cut(f, "\t1 ")
	if !strings.HasSuffix(item, "…") || len(item) > clip+len("…") || !strings.HasPrefix(item, "éé") {
		t.Errorf("clipped item %q (%d bytes)", item, len(item))
	}
}

// A feed that ends fails a wait at once, with the reason it ended, which
// Err returns from then on.
func TestAwaitFailsWhenTheFeedEnds(t *testing.T) {
	w := NewWatch[string](nil)
	m := w.Mark()
	w.Add("state")
	if err := w.Err(); err != nil {
		t.Errorf("Err while the feed goes on: %v", err)
	}
	go func() {
		time.Sleep(5 * time.Millisecond)
		w.End(errors.New("the stream ended"))
	}()
	start := time.Now()
	f := run(t, func(tb testing.TB) { w.Await(tb, m, "the resync", is("resync"), For(30*time.Second)) })
	if want := "the feed ended before the resync: the stream ended; 1 since the mark:\n\t1 state"; f != want {
		t.Errorf("failure %q, want %q", f, want)
	}
	if d := time.Since(start); d > Scale(10*time.Second) {
		t.Errorf("failed after %v", d)
	}
	if !w.Ended() {
		t.Error("not ended")
	}
	if err := w.Err(); err == nil || err.Error() != "the stream ended" {
		t.Errorf("Err after the end: %v", err)
	}
	// What came before the end is still found.
	if got, _ := w.Await(t, m, "the state", is("state")); got != "state" {
		t.Errorf("after the end: %q", got)
	}
	// Only the first End counts.
	w.End(nil)
	if f := run(t, func(tb testing.TB) { w.Await(tb, m, "x", is("x")) }); !strings.HasPrefix(f, "the feed ended before x: the stream ended;") {
		t.Errorf("a second End: %q", f)
	}
	if err := w.Err(); err == nil || err.Error() != "the stream ended" {
		t.Errorf("Err after a second End: %v", err)
	}
	// A plain end has no reason.
	plain := NewWatch[string](nil)
	plain.End(nil)
	if !plain.Ended() || plain.Err() != nil {
		t.Errorf("a plain end: ended %v, Err %v", plain.Ended(), plain.Err())
	}
}

// AwaitN waits for n items that match, the rest skipped, and returns
// them in the order fed with the cursor after the last; what came before
// the mark does not count.
func TestAwaitN(t *testing.T) {
	w := NewWatch[string](nil)
	w.Add("s:old")
	m := w.Mark()
	later(w, "s:1", "l:", "s:2", "l:", "s:3", "s:4")
	got, next := w.AwaitN(t, m, "three s: lines", 3, prefixed("s:"))
	if strings.Join(got, ",") != "s:1,s:2,s:3" || next != m+5 {
		t.Fatalf("got %q, next %d; want s:1,s:2,s:3 and %d", got, next, m+5)
	}
	if got, _ := w.AwaitN(t, next, "the next s: line", 1, prefixed("s:")); strings.Join(got, ",") != "s:4" {
		t.Fatalf("from the cursor after the third: %q", got)
	}
	if got, next := w.AwaitN(t, m, "the next two", 2, nil); strings.Join(got, ",") != "s:1,l:" || next != m+2 {
		t.Fatalf("a nil match: %q, %d", got, next)
	}
	if got, next := w.AwaitN(t, m, "none", 0, prefixed("x:")); got != nil || next != m {
		t.Fatalf("n of 0: %q, %d", got, next)
	}
}

// AwaitN's failure says how many of the n came, and what came since the
// mark; a feed that ends fails it at once.
func TestAwaitNFailure(t *testing.T) {
	w := NewWatch[string](nil)
	w.Add("s:old")
	m := w.Mark()
	w.Add("s:1", "l:", "s:2")
	f := run(t, func(tb testing.TB) { w.AwaitN(tb, m, "three s: lines", 3, prefixed("s:"), For(20*time.Millisecond)) })
	want := fmt.Sprintf("timed out after %v waiting for three s: lines (2 of 3 came); 3 since the mark:\n\t1 s:1\n\t2 l:\n\t3 s:2", Scale(20*time.Millisecond))
	if f != want {
		t.Errorf("failure:\n%s\nwant:\n%s", f, want)
	}
	w.End(errors.New("the client stopped"))
	f = run(t, func(tb testing.TB) { w.AwaitN(tb, m, "three s: lines", 3, prefixed("s:"), For(30*time.Second)) })
	if want := "the feed ended before three s: lines (2 of 3 came): the client stopped; 3 since the mark:"; !strings.HasPrefix(f, want) {
		t.Errorf("after the end: %q, want %q…", f, want)
	}
}

// Has finds what came by now and fails at once where it did not, saying
// what did; HasNot fails at the first that came. Neither waits.
func TestHasAndHasNot(t *testing.T) {
	w := NewWatch(func(s string) string { return "<" + s + ">" })
	w.Add("s:old")
	m := w.Mark()
	w.Add("l:", "s:1", "s:2")
	if got, next := w.Has(t, m, "an s: line", prefixed("s:")); got != "s:1" || next != m+2 {
		t.Errorf("Has: %q, %d", got, next)
	}
	if got, next := w.Has(t, -1, "the first", nil); got != "s:old" || next != 1 {
		t.Errorf("Has with a nil match from -1: %q, %d", got, next)
	}
	w.HasNot(t, m, "an f: line", prefixed("f:"))
	w.HasNot(t, w.Mark(), "anything after the last", nil)

	start := time.Now()
	f := run(t, func(tb testing.TB) { w.Has(tb, m, "an f: line", prefixed("f:")) })
	if want := "has not come: an f: line; 3 since the mark:\n\t1 <l:>\n\t2 <s:1>\n\t3 <s:2>"; f != want {
		t.Errorf("Has's failure:\n%s\nwant:\n%s", f, want)
	}
	f = run(t, func(tb testing.TB) { w.HasNot(tb, m, "a second s: line", is("s:2")) })
	if want := "came, and was not to: a second s: line: <s:2>; 3 since the mark:\n\t1 <l:>\n\t2 <s:1>\n\t3 <s:2>"; f != want {
		t.Errorf("HasNot's failure:\n%s\nwant:\n%s", f, want)
	}
	f = run(t, func(tb testing.TB) { w.Has(tb, w.Mark(), "anything", nil) })
	if f != "has not come: anything; nothing since the mark" {
		t.Errorf("Has after the last: %q", f)
	}
	if d := time.Since(start); d > Scale(time.Second) {
		t.Errorf("the checks took %v: they do not wait", d)
	}
}

// AwaitAll takes the items in any order, each for one match only, and
// returns them in the order of the matches, with the cursor after the
// last of them.
func TestAwaitAll(t *testing.T) {
	w := NewWatch[string](nil)
	m := w.Mark()
	later(w, "l:", "b", "l:", "a", "l:")
	got, next := w.AwaitAll(t, m, "a and b", is("a"), is("b"))
	if strings.Join(got, ",") != "a,b" || next != m+4 {
		t.Fatalf("got %q, next %d", got, next)
	}
	// One item meets both matches: it is taken for one, and the next
	// item for the other.
	w2 := NewWatch[string](nil)
	later(w2, "a", "a")
	if got, _ := w2.AwaitAll(t, 0, "two a", is("a"), is("a")); len(got) != 2 {
		t.Fatalf("two a: %q", got)
	}
	// An item meeting two matches moves to the one the next item cannot
	// meet: "a" first takes "any", "b" can only take "any", so "a" moves
	// to "is a".
	w3 := NewWatch[string](nil)
	later(w3, "a", "b")
	got, _ = w3.AwaitAll(t, 0, "any and a", func(string) bool { return true }, is("a"))
	if strings.Join(got, ",") != "b,a" {
		t.Fatalf("any and a: %q", got)
	}
	// A nil match takes every item, as in every other call.
	if got, _ = w3.AwaitAll(t, 0, "any and a, nil", nil, is("a")); strings.Join(got, ",") != "b,a" {
		t.Fatalf("nil and a: %q", got)
	}
	if got, next := w3.AwaitAll(t, 1, "nothing"); len(got) != 0 || next != 1 {
		t.Fatalf("no match: %q, %d", got, next)
	}
}

// AwaitAll's failure says which matches met no item.
func TestAwaitAllFailure(t *testing.T) {
	w := NewWatch[string](nil)
	w.Add("b", "c")
	w.End(nil) // fails at once, rather than after Wait
	f := run(t, func(tb testing.TB) { w.AwaitAll(tb, 0, "a, b and c", is("a"), is("b"), is("d")) })
	if want := "the feed ended before a, b and c (1 of 3 met, not met: 1, 3); 2 since the mark:\n\t1 b\n\t2 c"; f != want {
		t.Errorf("failure:\n%s\nwant:\n%s", f, want)
	}
}

// Count and Since read the items from a cursor on, those that match.
func TestCountAndSince(t *testing.T) {
	var w Watch[string] // the zero Watch is ready to use
	w.Add("l:", "s:1")
	m := w.Mark()
	w.Add("l:", "s:2", "f:", "s:3")
	if n := w.Count(m, prefixed("s:")); n != 2 {
		t.Errorf("Count: %d", n)
	}
	if n := w.Count(0, nil); n != 6 {
		t.Errorf("Count of all: %d", n)
	}
	if got := strings.Join(w.Since(m, func(s string) bool { return s != "l:" }), ","); got != "s:2,f:,s:3" {
		t.Errorf("Since: %s", got)
	}
	if got := w.Since(w.Mark(), nil); got != nil {
		t.Errorf("Since the end: %q", got)
	}
	if got := w.Since(-1, nil); len(got) != 6 {
		t.Errorf("Since -1: %q", got)
	}
	if got, next := w.Await(t, -1, "the second s: line", is("s:2")); got != "s:2" || next != 4 {
		t.Errorf("Await from -1: %q, %d", got, next)
	}
	if !w.Seen(m, is("f:"))() || w.Seen(m, is("s:1"))() {
		t.Error("Seen")
	}
}

// Never passes where nothing matching came before the end, and fails
// where it came first, where it came as the end did, or where the end
// did not come.
func TestWatchNever(t *testing.T) {
	w := NewWatch[string](nil)
	m := w.Mark()
	later(w, "l:", "s:1", "l:")
	w.Never(t, m, "an f: line", prefixed("f:"), w.Seen(m, is("l:")), Every(time.Millisecond))

	w2 := NewWatch[string](nil)
	later(w2, "l:", "s:1", "s:2", "end")
	f := run(t, func(tb testing.TB) {
		w2.Never(tb, 0, "a second s: line", func(s string) bool { return s == "s:2" }, w2.Seen(0, is("end")), Every(time.Millisecond))
	})
	if !strings.HasPrefix(f, "came, and was not to: a second s: line: s:2; ") {
		t.Errorf("the item before the end: %q", f)
	}

	// The item and the end come together: the look after the end finds it.
	w3 := NewWatch[string](nil)
	ended := false
	f = run(t, func(tb testing.TB) {
		w3.Never(tb, 0, "an s: line", prefixed("s:"), func() bool {
			if !ended {
				ended = true
				w3.Add("s:1") // fed as the end comes
			}
			return true
		})
	})
	if !strings.HasPrefix(f, "came, and was not to: an s: line: s:1;") {
		t.Errorf("the item as the end comes: %q", f)
	}

	w4 := NewWatch[string](nil)
	w4.Add("l:")
	f = run(t, func(tb testing.TB) {
		w4.Never(tb, 0, "an s: line", prefixed("s:"), func() bool { return false }, For(20*time.Millisecond), Every(time.Millisecond))
	})
	if want := fmt.Sprintf("timed out after %v waiting for the end of the check for an s: line; 1 since the mark:\n\t1 l:", Scale(20*time.Millisecond)); f != want {
		t.Errorf("no end: %q, want %q", f, want)
	}

	// A feed that ended does not end the check: its end still has to come.
	w5 := NewWatch[string](nil)
	w5.End(nil)
	var checks int
	w5.Never(t, 0, "anything", nil, func() bool { checks++; return checks > 3 }, Every(time.Millisecond))
}

// Items fed from many goroutines at once are all kept, and each wait
// finds its own.
func TestWatchConcurrentFeeding(t *testing.T) {
	const feeders, each = 8, 200
	w := NewWatch[string](nil)
	m := w.Mark()
	var wg sync.WaitGroup
	for f := range feeders {
		wg.Go(func() {
			for i := range each {
				w.Add(fmt.Sprintf("%d:%d", f, i))
			}
		})
	}
	var waits sync.WaitGroup
	errs := make(chan string, feeders)
	for f := range feeders {
		waits.Go(func() {
			// Waits from many goroutines: each from a recorder of its own.
			if fail := run(t, func(tb testing.TB) { w.Await(tb, m, "the last", is(fmt.Sprintf("%d:%d", f, each-1))) }); fail != "" {
				errs <- fail
			}
		})
	}
	wg.Wait()
	waits.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	if n := w.Count(m, nil); n != feeders*each {
		t.Fatalf("%d items, want %d", n, feeders*each)
	}
	for f := range feeders {
		if n := w.Count(m, prefixed(fmt.Sprintf("%d:", f))); n != each {
			t.Errorf("feeder %d: %d items", f, n)
		}
	}
}

// ChanWatch reads a channel until it is closed, which ends the watch.
func TestChanWatch(t *testing.T) {
	ch := make(chan int)
	w := ChanWatch(t, ch, func(i int) string { return fmt.Sprintf("#%d", i) })
	go func() {
		for i := 1; i <= 3; i++ {
			ch <- i
		}
		close(ch)
	}()
	if got, _ := w.Await(t, 0, "the third", func(i int) bool { return i == 3 }); got != 3 {
		t.Fatalf("got %d", got)
	}
	Eventually(t, "the end", w.Ended)
	f := run(t, func(tb testing.TB) { w.Await(tb, 0, "a fourth", func(i int) bool { return i == 4 }) })
	if want := "the feed ended before a fourth; 3 since the mark:\n\t1 #1\n\t2 #2\n\t3 #3"; f != want {
		t.Errorf("after the close: %q, want %q", f, want)
	}

	// A channel that stays open is read until the test ends.
	t.Run("open", func(t *testing.T) {
		open := make(chan int, 1)
		w := ChanWatch(t, open, nil)
		open <- 7
		w.Await(t, 0, "the 7", func(i int) bool { return i == 7 })
	})
}
