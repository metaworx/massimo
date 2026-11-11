// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// sseEvent is an event as a stream sends it.
type sseEvent struct {
	id, kind, data string
}

// readStream opens a Cube's event stream, with the header or the
// parameter naming the last id where given, and returns a channel of the
// events it sends, closed when the stream ends.
func readStream(t *testing.T, ts *httptest.Server, header, param string) <-chan sseEvent {
	t.Helper()
	url := ts.URL + "/api/v1/cubes/KEQ0000100/events"
	if param != "" {
		url += "?last_event_id=" + param
	}
	req, _ := http.NewRequest("GET", url, nil)
	if header != "" {
		req.Header.Set("Last-Event-ID", header)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("stream: %d", resp.StatusCode)
	}
	out := make(chan sseEvent, 2*replaySize)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		rd := bufio.NewReader(resp.Body)
		var ev sseEvent
		for {
			l, err := rd.ReadString('\n')
			if err != nil {
				return
			}
			l = strings.TrimRight(l, "\n")
			switch {
			case strings.HasPrefix(l, "id: "):
				ev.id = strings.TrimPrefix(l, "id: ")
			case strings.HasPrefix(l, "event: "):
				ev.kind = strings.TrimPrefix(l, "event: ")
			case strings.HasPrefix(l, "data: "):
				ev.data = strings.TrimPrefix(l, "data: ")
			case l == "" && ev.kind != "":
				out <- ev
				ev = sseEvent{}
			}
		}
	}()
	t.Cleanup(func() { resp.Body.Close() })
	return out
}

// next returns the stream's next event, failing after a while.
func next(t *testing.T, evs <-chan sseEvent) sseEvent {
	t.Helper()
	select {
	case ev, ok := <-evs:
		if !ok {
			t.Fatal("the stream ended")
		}
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
	}
	return sseEvent{}
}

// nextTest returns the stream's next test event, skipping what the fake
// Cube's connection sends meanwhile.
func nextTest(t *testing.T, evs <-chan sseEvent) sseEvent {
	t.Helper()
	for {
		if ev := next(t, evs); ev.kind == "test" {
			return ev
		}
	}
}

// burst reads a first connection's state events and returns the id the
// last of them carries.
func burst(t *testing.T, evs <-chan sseEvent, devices int) string {
	t.Helper()
	id := ""
	for i := 0; i < devices; i++ {
		ev := next(t, evs)
		if ev.kind != "state" {
			t.Fatalf("event %d: %q, want state", i, ev.kind)
		}
		if i < devices-1 && ev.id != "" {
			t.Fatalf("state %d carries id %q; only the last does", i, ev.id)
		}
		id = ev.id
	}
	if id == "" {
		t.Fatal("the last state event carries no id")
	}
	return id
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
	_, ts, s := start(t, "")
	rt := s.cubes["keq0000100"]
	devices := len(s.devices(rt))
	first := readStream(t, ts, "", "")
	head := burst(t, first, devices)
	for i := 1; i <= 3; i++ {
		mark(rt, i)
	}
	var ids []string
	prev := seqOf(t, head)
	for i := 0; i < 3; i++ {
		ev := nextTest(t, first)
		if n := seqOf(t, ev.id); n <= prev {
			t.Fatalf("id %s after %d does not count up", ev.id, prev)
		} else {
			prev = n
		}
		ids = append(ids, ev.id)
	}
	mark(rt, 4)
	mark(rt, 5)
	for name, evs := range map[string]<-chan sseEvent{
		"header":    readStream(t, ts, ids[2], ""),
		"parameter": readStream(t, ts, "", ids[2]),
		"both":      readStream(t, ts, ids[2], "nonsense-1"),
	} {
		var got []string
		for len(got) < 2 {
			ev := next(t, evs)
			switch ev.kind {
			case "state", "resync":
				t.Fatalf("%s: a resumed stream sent %s", name, ev.kind)
			case "test":
				got = append(got, ev.data)
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
	_, ts, s := start(t, "")
	rt := s.cubes["keq0000100"]
	devices := len(s.devices(rt))
	head := burst(t, readStream(t, ts, "", ""), devices)
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
		if ev := next(t, evs); ev.kind != "resync" {
			t.Fatalf("%s: first event %q, want resync", name, ev.kind)
		}
		burst(t, evs, devices)
	}
}

// A user's notification is replayed to that user alone.
func TestStreamReplayKeepsUsersApart(t *testing.T) {
	_, _, s := start(t, "")
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
	_, ts, s := start(t, "")
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
	burst(t, evs, len(s.devices(rt)))
	rt.subsMu.Lock()
	for ch, sub := range rt.subs {
		delete(rt.subs, ch)
		close(sub.gone)
	}
	rt.subsMu.Unlock()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-evs:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("the stream did not end")
		}
	}
}
