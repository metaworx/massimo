// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The log's cursor (AP LogCursor, Block 2, D8-A): a test that waits for
// the log's next line after an action marks the fake Cube's log before
// the action and waits from the mark, reading the log forward with
// after=<id> in its own goroutine, so that it reads each line once,
// misses none, and is held to no page's limit. docs/testing.md, "The
// log's cursor".

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	goruntime "runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// A logCursor is a place in the fake Cube's log as a caller reads it:
// the id of the last line before it, 0 for the log's start. A wait from
// it takes the lines after that one, in the order the log kept them.
type logCursor struct {
	ts    *httptest.Server
	token string // the caller's, "" for none
	at    int64
}

// logMark is the cursor after the newest line of the fake Cube's log,
// at any level: "from now on", taken before the action whose line the
// test waits for. Its read writes the store's pending lines first, so
// that a line kept before the mark is behind it.
func logMark(t testing.TB, ts *httptest.Server) logCursor {
	t.Helper()
	return logMarkAs(t, ts, "")
}

// logMarkAs is logMark for a caller with token, on a server that asks
// for one.
func logMarkAs(t testing.TB, ts *httptest.Server, token string) logCursor {
	t.Helper()
	c := logCursor{ts: ts, token: token}
	if page := c.page(t, cubeLogPath+"?level=debug&limit=1"); len(page) > 0 {
		c.at = page[0].ID
	}
	return c
}

// logStart is the cursor before the fake Cube's first line, for a wait
// that counts the lines of the whole log: "the log has n".
func logStart(ts *httptest.Server) logCursor { return logCursor{ts: ts} }

// logAfter waits for the first line after the cursor that match takes
// (nil takes every line), and returns it with the cursor after it, from
// which a next wait goes on. It fails the test after testkit.Wait,
// scaled, saying what came after the cursor, the last 20 lines; a
// cursor beyond the log's newest line (the API's 409) fails it at once.
func logAfter(t testing.TB, c logCursor, what string, match func(event) bool) (event, logCursor) {
	t.Helper()
	got, next := c.await(t, what, 1, match)
	return got[0], next
}

// logAfterN waits for the first n lines after the cursor that match,
// and returns them with the cursor after the last of them; the failure
// says how many came.
func logAfterN(t testing.TB, c logCursor, what string, n int, match func(event) bool) ([]event, logCursor) {
	t.Helper()
	return c.await(t, what, n, match)
}

// logLines are the lines after the cursor that match, as the log holds
// them now: for a check, once the action is over, of what came after a
// mark or of none more than a wait took. A nil match takes every line.
func logLines(t testing.TB, c logCursor, match func(event) bool) []event {
	t.Helper()
	var out []event
	for _, e := range c.read(t) {
		if match == nil || match(e) {
			out = append(out, e)
		}
	}
	return out
}

// logLine matches a line of kind and, where device is not "", of that
// device, at info and above: a line a page of the log shows without
// level=.
func logLine(kind, device string) func(event) bool {
	return func(e event) bool { return e.Kind == kind && (device == "" || e.Device == device) && atInfo(e) }
}

// atInfo reports whether a line is at info or above, the log's level
// where a page names none.
func atInfo(e event) bool {
	l, _ := store.ParseLevel(e.Level)
	return l >= store.LevelInfo
}

// await is logAfterN's wait: each check reads every line after the
// last one it read, keeps it for the failure, and takes it where it
// matches, until n are taken.
func (c logCursor) await(t testing.TB, what string, n int, match func(event) bool) ([]event, logCursor) {
	t.Helper()
	if n <= 0 {
		return nil, c
	}
	var seen, got []event
	read := c
	if testkit.Within(testkit.Wait, func() bool {
		for _, e := range read.read(t) {
			seen, read.at = append(seen, e), e.ID
			if len(got) < n && (match == nil || match(e)) {
				got = append(got, e)
			}
		}
		return len(got) == n
	}) {
		next := c
		next.at = got[n-1].ID
		return got, next
	}
	if n > 1 {
		what = fmt.Sprintf("%s (%d of %d came)", what, len(got), n)
	}
	t.Fatalf("timed out after %v waiting for %s; %s", testkit.Scale(testkit.Wait), what, logSeen(seen))
	return nil, c
}

// read reads every line after the cursor, at any level, page by page
// with after=, until a page is not full.
func (c logCursor) read(t testing.TB) []event {
	t.Helper()
	var out []event
	for at := c.at; ; {
		page := c.page(t, fmt.Sprintf("%s?level=debug&limit=%d&after=%d", cubeLogPath, maxLogPage, at))
		out = append(out, page...)
		if len(page) < maxLogPage {
			return out
		}
		at = page[len(page)-1].ID
	}
}

// page reads a page of the log as the cursor's caller; a 409, a cursor
// beyond the log's newest line, fails the test, as any answer but a
// page does.
func (c logCursor) page(t testing.TB, path string) []event {
	t.Helper()
	req, err := http.NewRequest("GET", c.ts.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusConflict:
		t.Fatalf("GET %s: 409 %s: the cursor is beyond the log's newest line", path, body)
	default:
		t.Fatalf("GET %s: %d %s", path, resp.StatusCode, body)
	}
	var page []event
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return page
}

// logSeen says what came after the cursor, for a failure: how many
// lines, and the last 20 of them, numbered from the cursor on.
func logSeen(seen []event) string {
	if len(seen) == 0 {
		return "nothing since the mark"
	}
	var b strings.Builder
	first := 0
	if len(seen) > 20 {
		first = len(seen) - 20
		fmt.Fprintf(&b, "%d since the mark, the last 20:", len(seen))
	} else {
		fmt.Fprintf(&b, "%d since the mark:", len(seen))
	}
	for i := first; i < len(seen); i++ {
		fmt.Fprintf(&b, "\n\t%d %s", i+1, logText(seen[i]))
	}
	return b.String()
}

// logText is a line as a failure shows it: its id, level, kind and
// device, its changes and its summary, clipped to 240 bytes.
func logText(e event) string {
	s := fmt.Sprintf("%d %s %s", e.ID, e.Level, e.Kind)
	if e.Device != "" {
		s += " " + e.Device
	}
	if len(e.Changes) > 0 {
		s += fmt.Sprintf(" %v", e.Changes)
	}
	if len(e.Summary) > 0 {
		s += fmt.Sprintf(" %v", e.Summary)
	}
	if len(s) <= 240 {
		return s
	}
	cut := 240
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// failureOf runs f with a test that records its failure instead of
// failing, in a goroutine of its own, and returns the failure, "" for
// none: for a test of what a helper's failure says.
func failureOf(t *testing.T, f func(tb testing.TB)) string {
	t.Helper()
	r := &failRecorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		f(r)
	}()
	<-done
	return r.failed
}

// failRecorder is a test that records its failure instead of failing:
// its Fatalf ends the goroutine it runs in, as the real one does.
type failRecorder struct {
	testing.TB
	failed string
}

func (r *failRecorder) Helper() {}

func (r *failRecorder) Fatalf(format string, args ...any) {
	r.failed = fmt.Sprintf(format, args...)
	goruntime.Goexit()
}

func (r *failRecorder) Fatal(args ...any) {
	r.failed = fmt.Sprint(args...)
	goruntime.Goexit()
}

// The log's cursor, on the fake Cube's log from the store and from the
// ring: a line kept before the mark is behind it, the newest of them at
// any level too; a wait takes the first line after its cursor that
// matches, and goes on from the line it took, a wait for n the first n;
// logLine takes no debug line; a wait and a read go through more than a
// page of the store's log; a cursor beyond the log's newest line fails
// the wait at once, with the 409; a failure shows the last 20 lines
// since the mark.
func TestLogCursor(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		opts  []startOpt
		burst int // the lines of a burst: more than a page of the store's, less than the ring holds
	}{{"store", []startOpt{stored()}, maxLogPage + 200}, {"ring", nil, logSize - 50}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := startServer(t, append(c.opts, quiet(), silent())...)
			named := func(name string) func(event) bool { return func(e event) bool { return e.Summary["name"] == name } }
			names := func(evs []event) string {
				var out []string
				for _, e := range evs {
					out = append(out, fmt.Sprint(e.Summary["name"]))
				}
				return strings.Join(out, " ")
			}
			keepAt := func(level, name string) {
				r.rt.remember(event{Kind: "change", Source: "config", Level: level, Time: time.Now(), Summary: map[string]any{"name": name}})
			}
			keep := func(name string) { keepAt("info", name) }
			keep("before")
			keepAt("debug", "debug before") // the mark is the newest line at any level
			m := logMark(t, r.ts)
			if got := logLines(t, m, nil); len(got) != 0 {
				t.Fatalf("after the mark, before a line: %s", names(got))
			}
			keep("a")
			keep("b")
			keep("a")
			if e, _ := logAfter(t, m, "the first line after the mark", nil); e.Summary["name"] != "a" {
				t.Fatalf("the first line after the mark: %s", logText(e))
			}
			first, next := logAfter(t, m, "the first a", named("a"))
			second, last := logAfter(t, next, "the second a", named("a"))
			if got := names(logLines(t, next, nil)); first.ID >= second.ID || got != "b a" {
				t.Fatalf("after the first a (%d): %s, the second a %d", first.ID, got, second.ID)
			}
			if got := logLines(t, last, nil); len(got) != 0 {
				t.Fatalf("after the second a: %s", names(got))
			}
			if both, _ := logAfterN(t, m, "both a", 2, named("a")); len(both) != 2 || both[0].ID != first.ID || both[1].ID != second.ID {
				t.Fatalf("both a: %v, want %d and %d", ids(both), first.ID, second.ID)
			}
			// logLine takes what a page without level= shows: info and above.
			m = logMark(t, r.ts)
			keepAt("debug", "quiet")
			keep("loud")
			if e, _ := logAfter(t, m, "the change at info", logLine("change", "")); e.Summary["name"] != "loud" {
				t.Fatalf("the change at info: %s", logText(e))
			}

			m = logMark(t, r.ts)
			for i := range c.burst {
				keep(fmt.Sprintf("burst %d", i))
			}
			burst, end := logAfterN(t, m, "the burst", c.burst, nil)
			for i, e := range burst {
				if want := fmt.Sprintf("burst %d", i); e.Summary["name"] != want || i > 0 && e.ID <= burst[i-1].ID {
					t.Fatalf("line %d of the burst: %s, want %s", i, logText(e), want)
				}
			}
			if got := logLines(t, m, nil); len(got) != c.burst {
				t.Fatalf("logLines after the mark: %d lines, the burst %d", len(got), c.burst)
			}
			if end.at != burst[len(burst)-1].ID {
				t.Fatalf("the cursor after the burst at %d, its last line %d", end.at, burst[len(burst)-1].ID)
			}

			// A million lines on: no line the test does not write itself
			// can reach it.
			beyond := logCursor{ts: r.ts, at: end.at + 1_000_000}
			failed := failureOf(t, func(tb testing.TB) { logAfter(tb, beyond, "a line beyond the newest", nil) })
			if !strings.Contains(failed, ": 409 ") || !strings.Contains(failed, "is beyond the log's newest line") {
				t.Fatalf("a cursor beyond the newest line: %q", failed)
			}
		})
	}
	var lines []event
	for i := range 25 {
		lines = append(lines, event{ID: int64(i + 1), Level: "info", Kind: "change"})
	}
	if got := logSeen(nil); got != "nothing since the mark" {
		t.Errorf("nothing seen: %q", got)
	}
	if got := logSeen(lines); !strings.HasPrefix(got, "25 since the mark, the last 20:\n\t6 6 info change\n") || !strings.HasSuffix(got, "\n\t25 25 info change") {
		t.Errorf("25 seen: %q", got)
	}
}
