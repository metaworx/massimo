// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The log's forward cursor, after=<id> (AP LogCursor, Block 1): a page of
// the first lines after an id, oldest first, on both log routes, from
// the store and from the ring, whose lines take their ids from one
// counter of the server (logRing).

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// The two log routes, of the fake Cube and of every Cube.
const (
	cubeLogPath = "/api/v1/cubes/KEQ0000100/log"
	allLogPath  = "/api/v1/log"
)

// silent gives the server a text log that goes nowhere, for a test that
// writes many lines to the log, each of which it would print.
func silent() startOpt {
	return withOptions(Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

// runtimeOf is the server's runtime of a Cube, by its serial.
func runtimeOf(t *testing.T, s *Server, serial string) *runtime {
	t.Helper()
	for _, rt := range s.runtimes() {
		if rt.cubeSerial() == serial {
			return rt
		}
	}
	t.Fatalf("no runtime of %s", serial)
	return nil
}

// newestOf is the id of the newest line of a log route at any level, 0
// for an empty log.
func newestOf(t *testing.T, ts *httptest.Server, path string) int64 {
	t.Helper()
	var page []event
	if code := getJSON(t, ts, "", path+"?level=debug&limit=1", &page); code != 200 {
		t.Fatalf("%s: %d", path, code)
	}
	if len(page) == 0 {
		return 0
	}
	return page[0].ID
}

// pageForward reads a log route forward from after, at any level, limit
// lines a page, until a page comes back empty, and checks each page:
// at most limit lines, oldest first, each above its cursor. It returns
// the lines read and the cursor after the last of them.
func pageForward(t *testing.T, ts *httptest.Server, path string, after int64, limit int) ([]event, int64) {
	t.Helper()
	read := []event{}
	for {
		q := fmt.Sprintf("%s?level=debug&limit=%d&after=%d", path, limit, after)
		var page []event
		if code := getJSON(t, ts, "", q, &page); code != 200 {
			t.Fatalf("%s: %d", q, code)
		}
		if len(page) == 0 {
			return read, after
		}
		for i, e := range page {
			if len(page) > limit || e.ID <= after || i > 0 && e.ID <= page[i-1].ID {
				t.Fatalf("%s: %v", q, ids(page))
			}
		}
		read = append(read, page...)
		after = page[len(page)-1].ID
	}
}

// burstLine is line n of a writer's burst, at info, named in its
// summary.
func burstLine(writer string, n int) event {
	return event{Kind: "change", Source: "config", Level: "info", Time: time.Now(), Summary: map[string]any{"burst": writer + strconv.Itoa(n)}}
}

// A burst many pages long, written while a reader pages the log forward
// from a mark, is read line by line exactly once, in id order: the
// fake Cube's log, the server's lines with it, and every Cube's; from
// the store, and from the rings. Two writers share the fake Cube's log,
// a third writes the second Cube's, a fourth the server's lines into
// both. Each writes on at a pace while the reader reads one page of
// each log at a time, up to a window of lines the reader has not read
// yet, which keeps the rings below their 500 lines and the store's
// writer below its buffer: the store's writer commits its batches of
// 100 while the pages are read, between the walks of a page too, which
// every walk's bound to the same newest row makes harmless.
func TestLogAfterReadsABurstOnce(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		opts []startOpt
	}{{"store", []startOpt{stored()}}, {"ring", nil}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := startServer(t, append(c.opts, alsoCube(secondCube), quiet(), silent())...)
			ts, s := r.ts, r.s
			second := runtimeOf(t, s, "KEQ0000200")
			oneAt, allAt := newestOf(t, ts, cubeLogPath), newestOf(t, ts, allLogPath)
			// n lines a writer: 3000 in the fake Cube's log and 4000 in
			// every Cube's, fifteen pages and more of each, and forty
			// batches of the store's writer.
			const n, window = 1000, 100
			writers := []struct {
				name  string
				write func(event)
				own   bool // in the fake Cube's log
			}{{"a", r.rt.remember, true}, {"b", r.rt.remember, true}, {"c", second.remember, false}, {"s", s.logServer, true}}
			// read is how many of each writer's lines the reader has read
			// in every log that holds them, under mu; a writer waits on
			// more while it is window lines ahead.
			var mu sync.Mutex
			more := sync.NewCond(&mu)
			read, stopped := make([]int, len(writers)), false
			t.Cleanup(func() { mu.Lock(); stopped = true; more.Broadcast(); mu.Unlock() }) // a reader that failed holds no writer up
			var wg sync.WaitGroup
			for k, w := range writers {
				wg.Go(func() {
					for i := range n {
						mu.Lock()
						for i-read[k] >= window && !stopped {
							more.Wait()
						}
						mu.Unlock()
						w.write(burstLine(w.name, i))
						if i%20 == 19 {
							// A pace, not a wait: the lines keep coming while
							// a page is read, rather than all at once between
							// two pages.
							time.Sleep(time.Millisecond)
						}
					}
				})
			}
			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()
			// one and all count each line read in the fake Cube's log and
			// in every Cube's; oneOf and allOf how many lines of each
			// writer they read, which a line missed holds back by one.
			one, all, oneOf, allOf := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
			// page reads one page of a route forward from its cursor,
			// counts the burst's lines on it, and reports whether it was
			// empty.
			page := func(path string, at *int64, seen, of map[string]int) bool {
				t.Helper()
				q := fmt.Sprintf("%s?level=debug&limit=200&after=%d", path, *at)
				var evs []event
				if code := getJSON(t, ts, "", q, &evs); code != 200 {
					t.Fatalf("%s: %d", q, code)
				}
				for i, e := range evs {
					if e.ID <= *at || i > 0 && e.ID <= evs[i-1].ID {
						t.Fatalf("%s: %v", q, ids(evs))
					}
					if b, ok := e.Summary["burst"].(string); ok {
						if seen[b]++; seen[b] == 1 {
							of[b[:1]]++
						}
					}
				}
				if len(evs) > 0 {
					*at = evs[len(evs)-1].ID
				}
				return len(evs) == 0
			}
			// round reads one page of each log, and reports whether the
			// burst is over and read to its end.
			round := func() bool {
				over := false
				select {
				case <-done:
					over = true // and the pages below come after it
				default:
				}
				// The writers go on from what the pages before read, while
				// the pages below are read.
				mu.Lock()
				for k, w := range writers {
					read[k] = allOf[w.name]
					if w.own {
						read[k] = min(read[k], oneOf[w.name])
					}
				}
				more.Broadcast()
				mu.Unlock()
				oneEmpty, allEmpty := page(cubeLogPath, &oneAt, one, oneOf), page(allLogPath, &allAt, all, allOf)
				return over && oneEmpty && allEmpty
			}
			// The reader goes on while its pages read new lines of the
			// burst, however long the burst takes on a slow runner, and
			// fails where none comes for a wait: a reader that stopped.
			// go test's -timeout bounds the whole.
			got := func() int { return len(one) + len(all) }
			for over := false; !over; {
				before := got()
				if !testkit.Within(testkit.Wait, func() bool { over = round(); return over || got() > before }, testkit.Every(time.Millisecond)) {
					t.Fatalf("no new line of the burst read for %v: %d of %d in the fake Cube's log, %d of %d in every Cube's, by writer %v and %v",
						testkit.Scale(testkit.Wait), len(one), 3*n, len(all), 4*n, oneOf, allOf)
				}
			}
			for _, w := range writers {
				want := 0
				if w.own {
					want = 1
				}
				for i := range n {
					if b := w.name + strconv.Itoa(i); one[b] != want || all[b] != 1 {
						t.Fatalf("line %s read %d times in the fake Cube's log, want %d, and %d times in every Cube's, want 1", b, one[b], want, all[b])
					}
				}
			}
			if len(one) != 3*n || len(all) != 4*n {
				t.Fatalf("%d lines of the burst in the fake Cube's log, want %d; %d in every Cube's, want %d", len(one), 3*n, len(all), 4*n)
			}
		})
	}
}

// The rings take their ids and their lines under the server's one lock:
// round after round, three writers into the fake Cube's ring, one into
// the second Cube's and one writing the server's lines into both leave
// no ring out of id order, and a reader that pages one Cube's log and
// every Cube's forward while they write reads each line once. A lock of
// each ring's own would let a line take its id and go into its ring
// after a line of a higher id, in that ring or in another.
func TestLogRingOneLock(t *testing.T) {
	t.Parallel()
	r := startServer(t, alsoCube(secondCube), quiet(), silent())
	s, second := r.s, runtimeOf(t, r.s, "KEQ0000200")
	_, newest := s.ringRead()
	views := []struct {
		name string
		rts  []*runtime
		at   int64
		own  string // the writers whose lines the view holds
	}{
		{"the fake Cube's", []*runtime{r.rt}, newest, "abcs"},
		{"every Cube's", []*runtime{r.rt, second}, newest, "abcds"},
	}
	for round := range 30 {
		writers := []struct {
			name  string
			write func(event)
		}{{"a", r.rt.remember}, {"b", r.rt.remember}, {"c", r.rt.remember}, {"d", second.remember}, {"s", s.logServer}}
		var wg sync.WaitGroup
		for _, w := range writers {
			wg.Go(func() {
				for i := range 60 {
					w.write(burstLine(fmt.Sprintf("%s%d.", w.name, round), i))
				}
			})
		}
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		seen := make([]map[string]int, len(views))
		for i := range views {
			seen[i] = map[string]int{}
		}
		testkit.Eventually(t, fmt.Sprintf("round %d's end", round), func() bool {
			over := false
			select {
			case <-done:
				over = true // and the reads below come after it
			default:
			}
			for i := range views {
				v := &views[i]
				for {
					page, err := s.ringLog(v.rts, logQuery{forward: true, after: v.at, limit: 50, minLevel: store.LevelDebug})
					if err != nil {
						t.Fatalf("round %d, %s log after %d: %v", round, v.name, v.at, err)
					}
					if len(page) == 0 {
						break
					}
					for k, e := range page {
						if e.ID <= v.at || k > 0 && e.ID <= page[k-1].ID {
							t.Fatalf("round %d, %s log after %d: %v", round, v.name, v.at, ids(page))
						}
						if b, ok := e.Summary["burst"].(string); ok {
							seen[i][b]++
						}
					}
					v.at = page[len(page)-1].ID
				}
			}
			return over
		}, testkit.Every(time.Millisecond))
		for i, v := range views {
			for _, w := range writers {
				want := 0
				if strings.Contains(v.own, w.name) {
					want = 1
				}
				for n := range 60 {
					if b := fmt.Sprintf("%s%d.%d", w.name, round, n); seen[i][b] != want {
						t.Fatalf("round %d, %s log: line %s read %d times, want %d", round, v.name, b, seen[i][b], want)
					}
				}
			}
		}
	}
}

// A page after a cursor is the first lines after it, oldest first, the
// cursor's own line not again, from the store and from the ring, the
// ring of a server without a store and of a Cube without a row beside a
// store: with each filter, inside a window before an id and since a
// time, a Cube's log with the server's lines; 0 is the log's start. A
// bad cursor is 400, one beyond the newest line 409 with its error on
// both routes; the newest line's id answers an empty page.
func TestLogAfterAnswers(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		ring bool // the fake Cube's log is its ring
		opts func(t *testing.T) []startOpt
	}{
		{"store", false, func(*testing.T) []startOpt { return []startOpt{stored()} }},
		{"ring", true, func(*testing.T) []startOpt { return nil }},
		{"ring beside a store", true, func(t *testing.T) []startOpt {
			return []startOpt{editOptions(func(o *Options) { o.Store = testStore(t, "") })} // and no row of the Cube
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := startServer(t, append(c.opts(t), quiet(), silent())...)
			ts, s, rt := r.ts, r.s, r.rt
			if ring := rt.storeID.Load() == 0 || rt.store == nil; ring != c.ring {
				t.Fatalf("the fake Cube's log in its ring: %v, want %v", ring, c.ring)
			}
			mark := newestOf(t, ts, cubeLogPath)
			at := time.Now().UTC().Truncate(time.Millisecond)
			for i, ev := range []event{
				{Kind: "change", Source: "config", Level: "info"},
				{Kind: "state", Source: "device", Level: "info", Device: "0a0909", Serial: "MEQ0000909"},
				{Kind: "change", Source: "config", Level: "warn"},
				{Kind: "state", Source: "device", Level: "debug", Device: "0a0909", Serial: "MEQ0000909"},
				{Kind: "change", Source: "config", Level: "info"},
			} {
				ev.Time = at.Add(time.Duration(i) * time.Millisecond)
				rt.remember(ev)
			}
			server := systemEvent("settings_read", 0, nil)
			server.Time = at.Add(10 * time.Millisecond)
			s.logServer(server)

			// page reads a page of the fake Cube's log, and says it as
			// its lines' kinds and levels, and their ids.
			page := func(query string) (string, []int64) {
				t.Helper()
				code, evs := logOf(t, ts, query)
				if code != 200 {
					t.Fatalf("%s: %d", query, code)
				}
				var got string
				for _, e := range evs {
					got += " " + e.Kind + ":" + e.Level
				}
				return got, ids(evs)
			}
			lines, id := page("?level=debug&after=" + strconv.FormatInt(mark, 10))
			if lines != " change:info state:info change:warn state:debug change:info settings_read:info" || id[0] <= mark {
				t.Fatalf("the lines after %d: %s %v", mark, lines, id)
			}
			if c.ring && id[0] < s.started.UnixMicro() {
				t.Fatalf("a ring's id %d below the server's start, %d µs", id[0], s.started.UnixMicro())
			}
			after := func(i int) string { return strconv.FormatInt(id[i], 10) }
			m := strconv.FormatInt(mark, 10)
			for _, want := range []struct{ query, lines string }{
				{"?after=" + m, " change:info state:info change:warn change:info settings_read:info"},
				{"?level=debug&kind=state&after=" + m, " state:info state:debug"},
				{"?level=debug&device=0a0909&after=" + m, " state:info state:debug"},
				{"?source=config&after=" + m, " change:info change:warn change:info"},
				{"?level=warn&after=" + m, " change:warn"},
				{"?level=debug&after=" + m + "&before=" + after(3), " change:info state:info change:warn"},
				{"?level=debug&limit=2&after=" + after(0), " state:info change:warn"},
				{"?level=debug&after=" + after(0) + "&since=" + url.QueryEscape(at.Add(3*time.Millisecond).Format(time.RFC3339Nano)), " state:debug change:info settings_read:info"},
				{"?level=debug&after=" + after(4), " settings_read:info"},
			} {
				if got, _ := page(want.query); got != want.lines {
					t.Errorf("%s: %s, want %s", want.query, got, want.lines)
				}
			}
			// The newest line's id: an empty page.
			var raw json.RawMessage
			if code := getJSON(t, ts, "", cubeLogPath+"?after="+after(5), &raw); code != 200 || string(raw) != "[]" {
				t.Errorf("after the newest line: %d %q", code, raw)
			}
			// 0 is the start: the whole log, the oldest line first.
			_, whole := page("?level=debug&limit=1000")
			if _, start := page("?level=debug&limit=1000&after=0"); !slices.Equal(start, whole) {
				t.Errorf("after=0: %v, the log %v", start, whole)
			}
			if _, first := page("?level=debug&limit=1&after=0"); len(first) != 1 || first[0] != whole[0] {
				t.Errorf("after=0&limit=1: %v, the oldest %d", first, whole[0])
			}

			for _, bad := range []string{"-1", "x", "1.5", "9223372036854775808", "0x10"} {
				code, text := callAPI(t, ts, "GET", cubeLogPath+"?after="+url.QueryEscape(bad), "", "")
				var e map[string]string
				json.Unmarshal([]byte(text), &e)
				if want := fmt.Sprintf("bad after %q: an event's id, or 0 for the start", bad); code != 400 || e["error"] != want {
					t.Errorf("after=%s: %d %s", bad, code, text)
				}
			}
			beyond := strconv.FormatInt(id[5]+1, 10)
			for _, q := range []string{cubeLogPath + "?after=" + beyond, cubeLogPath + "?kind=state&after=" + beyond, allLogPath + "?after=" + beyond} {
				code, text := callAPI(t, ts, "GET", q, "", "")
				var e map[string]string
				json.Unmarshal([]byte(text), &e)
				if code != 409 || len(e) != 1 || e["error"] != "after "+beyond+" is beyond the log's newest line: reload the log" {
					t.Errorf("%s: %d %s", q, code, text)
				}
			}
		})
	}
}

// The ring's ids grow across a restart without a store: the second run's
// lines lie after the first run's newest, so that a cursor of the first
// run reads the second from its start, its server_started.
func TestLogRingIdsGrowAcrossARestart(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet(), silent())
	first, last := pageForward(t, r.ts, cubeLogPath, 0, 1000)
	if len(first) == 0 {
		t.Fatal("the first run's ring is empty")
	}
	r.stop()
	ts := startServer(t, again(r), quiet(), silent()).ts
	second, _ := pageForward(t, ts, cubeLogPath, last, 1000)
	whole, _ := pageForward(t, ts, cubeLogPath, 0, 1000)
	if len(second) == 0 || second[0].Kind != "server_started" || len(whole) < len(second) || !slices.Equal(ids(second), ids(whole)[:len(second)]) {
		t.Fatalf("after the first run's newest, %d: %v, the second run's ring %v", last, ids(second), ids(whole))
	}
}
