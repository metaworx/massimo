// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// ReadEvents reads each event at its blank line, with its id, kind and
// data, several data lines joined; a comment at once; CRLF as LF; and
// nothing of an event the stream cut off.
func TestReadEvents(t *testing.T) {
	stream := strings.Join([]string{
		": keep-alive", "",
		"event: state", `data: {"device":"0a0101"}`, "",
		"id: 7", "event: test", "data: one", "data: two", "",
		"event:resync\r", "data:\r", "\r",
		"retry: 100", "",
		"event: cut", "data: off",
	}, "\n")
	w := NewWatch[Event](nil)
	if err := ReadEvents(w, strings.NewReader(stream)); err != nil {
		t.Fatal(err)
	}
	want := []Event{
		{Comment: "keep-alive"},
		{Kind: "state", Data: `{"device":"0a0101"}`},
		{ID: "7", Kind: "test", Data: "one\ntwo"},
		{Kind: "resync"},
	}
	got := w.Since(0, nil)
	if !slices.Equal(got, want) {
		t.Fatalf("events\n%+v\nwant\n%+v", got, want)
	}
	if s := got[2].String(); s != "test id=7 one\ntwo" {
		t.Errorf("shown %q", s)
	}
	if s := got[0].String(); s != ": keep-alive" {
		t.Errorf("comment shown %q", s)
	}
	if !Kind("state", `"device":"0a0101"`)(got[1]) || Kind("state", `"device":"0a0102"`)(got[1]) || Kind("test")(got[1]) {
		t.Error("Kind")
	}
}

// streamServer is an event stream whose events the test sends on the
// channel; closing it ends the stream.
func streamServer(t *testing.T) (string, chan<- string) {
	next := make(chan string)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		fl.Flush()
		for {
			select {
			case ev, ok := <-next:
				if !ok {
					return
				}
				fmt.Fprint(w, ev)
				fl.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}))
	t.Cleanup(ts.Close)
	return ts.URL, next
}

// openStream opens the event stream at url and watches it.
func openStream(t *testing.T, url string) *Watch[Event] {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("stream: %d", resp.StatusCode)
	}
	return StreamWatch(t, resp.Body)
}

// StreamWatch reads a response's event stream as it comes, and the end
// of the stream ends the watch.
func TestStreamWatch(t *testing.T) {
	url, next := streamServer(t)
	w := openStream(t, url)
	m := w.Mark()
	next <- "event: state\ndata: {\"device\":\"0a0101\"}\n\n"
	next <- "id: 3\nevent: test\ndata: {}\n\n"
	if ev, _ := w.Await(t, m, "the test event", Kind("test")); ev.ID != "3" {
		t.Fatalf("got %+v", ev)
	}
	close(next)
	Eventually(t, "the stream's end", w.Ended)
	if f := run(t, func(tb testing.TB) { w.Await(tb, m, "a resync", Kind("resync")) }); !strings.HasPrefix(f, "the feed ended before a resync; 2 since the mark:") {
		t.Errorf("after the end: %q", f)
	}
	if err := w.Err(); err != nil {
		t.Errorf("a stream the server ended: Err %v", err)
	}
}

// A stream the server cuts off, rather than ends, ends the watch with
// the read's error.
func TestStreamWatchCut(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: state\ndata: {}\n\n")
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler) // the connection is closed without the stream's end
	}))
	t.Cleanup(ts.Close)
	w := openStream(t, ts.URL)
	w.Await(t, 0, "the state", Kind("state"))
	Eventually(t, "the stream's end", w.Ended)
	if err := w.Err(); err == nil {
		t.Error("a cut stream: Err nil")
	}
}

// The end of the test closes a stream that goes on, and waits for its
// reader.
func TestStreamWatchClosedAtTheEnd(t *testing.T) {
	url, next := streamServer(t)
	var w *Watch[Event]
	t.Run("goes on", func(t *testing.T) {
		w = openStream(t, url)
		next <- ": keep-alive\n\n"
		w.Await(t, 0, "the keep-alive", func(e Event) bool { return e.Comment == "keep-alive" })
	})
	if !w.Ended() {
		t.Error("the stream's watch did not end with its test")
	}
}
