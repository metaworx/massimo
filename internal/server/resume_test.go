// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/view"
)

// readStream opens a Cube's event stream, with the header or the
// parameter naming the last id where given, and returns the watch of
// what it sends, which ends with the stream.
func readStream(t *testing.T, ts *httptest.Server, header, param string) *testkit.Watch[testkit.Event] {
	t.Helper()
	url := ts.URL + "/api/v1/cubes/KEQ0000100/events"
	if param != "" {
		url += "?last_event_id=" + param
	}
	return readStreamFrom(t, url, header)
}

// readStreamURL opens the event stream at url.
func readStreamURL(t *testing.T, url string) *testkit.Watch[testkit.Event] {
	t.Helper()
	return readStreamFrom(t, url, "")
}

// readStreamFrom opens an event stream, with the header naming the last
// id where given.
func readStreamFrom(t *testing.T, url, header string) *testkit.Watch[testkit.Event] {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	if header != "" {
		req.Header.Set("Last-Event-ID", header)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		t.Fatalf("stream: %d", resp.StatusCode)
	}
	return testkit.StreamWatch(t, resp.Body)
}

// isEvent matches an event of a stream, its comments (the keep-alive)
// left out.
func isEvent(e testkit.Event) bool { return e.Kind != "" }

// burst reads a connection's state events from the cursor on and
// returns the id the last of them carries, with the cursor after it.
func burst(t *testing.T, evs *testkit.Watch[testkit.Event], from testkit.Cursor, devices int) (string, testkit.Cursor) {
	t.Helper()
	got, after := evs.AwaitN(t, from, "the state burst", devices, isEvent)
	id := ""
	for i, ev := range got {
		if ev.Kind != "state" {
			t.Fatalf("event %d: %q, want state", i, ev.Kind)
		}
		if i < devices-1 && ev.ID != "" {
			t.Fatalf("state %d carries id %q; only the last does", i, ev.ID)
		}
		id = ev.ID
	}
	if id == "" {
		t.Fatal("the last state event carries no id")
	}
	return id, after
}

// mark sends a test event on the Cube's stream.
func mark(rt *runtime, n int) {
	rt.send(event{Kind: "test", Summary: map[string]any{"n": n}, Time: time.Now()})
}

// seqOf is an id's number.
func seqOf(t *testing.T, id string) uint64 {
	t.Helper()
	_, num, _ := strings.Cut(id, "-")
	n, err := strconv.ParseUint(num, 10, 64)
	if err != nil {
		t.Fatalf("id %q: %v", id, err)
	}
	return n
}

// Every event of the stream carries an id that counts up; a client that
// reconnects with the last id it read, by the header or the parameter,
// gets what came since and no state burst.
func TestStreamResumes(t *testing.T) {
	r := startServer(t)
	ts, s := r.ts, r.s
	rt := s.cubes["keq0000100"]
	devices := len(s.devicesOf(rt, view.KindDefault))
	first := readStream(t, ts, "", "")
	head, c := burst(t, first, 0, devices)
	for i := 1; i <= 3; i++ {
		mark(rt, i)
	}
	var ids []string
	prev := seqOf(t, head)
	tests, _ := first.AwaitN(t, c, "the three test events", 3, testkit.Kind("test"))
	for _, ev := range tests {
		if n := seqOf(t, ev.ID); n <= prev {
			t.Fatalf("id %s after %d does not count up", ev.ID, prev)
		} else {
			prev = n
		}
		ids = append(ids, ev.ID)
	}
	mark(rt, 4)
	mark(rt, 5)
	for name, evs := range map[string]*testkit.Watch[testkit.Event]{
		"header":    readStream(t, ts, ids[2], ""),
		"parameter": readStream(t, ts, "", ids[2]),
		"both":      readStream(t, ts, ids[2], "nonsense-1"),
	} {
		var got []string
		for c := testkit.Cursor(0); len(got) < 2; {
			var ev testkit.Event
			ev, c = evs.Await(t, c, name+": the next event", isEvent)
			switch ev.Kind {
			case "state", "resync":
				t.Fatalf("%s: a resumed stream sent %s", name, ev.Kind)
			case "test":
				got = append(got, ev.Data)
			}
		}
		for i, want := range []int{4, 5} {
			if !strings.Contains(got[i], `"n":`+strconv.Itoa(want)) {
				t.Fatalf("%s: test events %v, want 4 and 5", name, got)
			}
		}
	}
}

// An id the ring no longer holds, one of another epoch or one not issued
// yet gets a resync event and the state, as a first connection does.
func TestStreamResyncs(t *testing.T) {
	r := startServer(t)
	ts, s := r.ts, r.s
	rt := s.cubes["keq0000100"]
	devices := len(s.devicesOf(rt, view.KindDefault))
	head, _ := burst(t, readStream(t, ts, "", ""), 0, devices)
	for i := 0; i < replaySize+2; i++ {
		mark(rt, i)
	}
	rt.subsMu.Lock()
	future := rt.streamID(rt.seq + 1)
	rt.subsMu.Unlock()
	for name, last := range map[string]string{
		"dropped":     head,
		"other epoch": "0-1",
		"future":      future,
		"malformed":   "garbage",
	} {
		evs := readStream(t, ts, last, "")
		ev, c := evs.Await(t, 0, name+": the first event", isEvent)
		if ev.Kind != "resync" {
			t.Fatalf("%s: first event %q, want resync", name, ev.Kind)
		}
		burst(t, evs, c, devices)
	}
}

// A user's notification is replayed to that user alone.
func TestStreamReplayKeepsUsersApart(t *testing.T) {
	s := startServer(t).s
	rt := s.cubes["keq0000100"]
	rt.subsMu.Lock()
	from := rt.streamID(rt.seq)
	rt.subsMu.Unlock()
	rt.sendUser(7, event{Kind: "notification", Time: time.Now()})
	mark(rt, 1)
	rt.subsMu.Lock()
	defer rt.subsMu.Unlock()
	for user, want := range map[int64]int{7: 2, 8: 1} {
		evs, ok := rt.since(from, &subscriber{user: user})
		if !ok || len(evs) != want {
			t.Fatalf("user %d: %d events (%v), want %d", user, len(evs), ok, want)
		}
	}
}

// A subscriber that cannot keep up is dropped and told to end, and its
// stream ends after writing what it still holds.
func TestStreamOfASlowClientEnds(t *testing.T) {
	r := startServer(t)
	ts, s := r.ts, r.s
	rt := s.cubes["keq0000100"]
	slow := make(chan event, 1)
	sub := &subscriber{gone: make(chan struct{})}
	rt.subsMu.Lock()
	rt.subs[slow] = sub
	rt.subsMu.Unlock()
	mark(rt, 1)
	mark(rt, 2)
	select {
	case <-sub.gone:
	default:
		t.Fatal("the full subscriber was not told to end")
	}
	rt.subsMu.Lock()
	_, still := rt.subs[slow]
	rt.subsMu.Unlock()
	if still {
		t.Fatal("the full subscriber is still subscribed")
	}

	evs := readStream(t, ts, "", "")
	burst(t, evs, 0, len(s.devicesOf(rt, view.KindDefault)))
	rt.subsMu.Lock()
	for ch, sub := range rt.subs {
		delete(rt.subs, ch)
		close(sub.gone)
	}
	rt.subsMu.Unlock()
	testkit.Eventually(t, "the stream's end", evs.Ended)
}
