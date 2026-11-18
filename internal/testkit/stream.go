// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

// An Event is what an event stream (text/event-stream) sent: an event,
// or a comment such as the server's keep-alive.
type Event struct {
	ID   string // the id field, "" where the event carried none
	Kind string // the event field; "" for a comment
	Data string // the data fields, joined by newlines
	// Comment is a comment line's text without its colon and the space
	// after it ("keep-alive"); the other fields are empty then.
	Comment string
}

// String is the event as a failure shows it: its kind, its id and its
// data, or the comment.
func (e Event) String() string {
	if e.Kind == "" && e.ID == "" && e.Data == "" {
		return ": " + e.Comment
	}
	s := e.Kind
	if e.ID != "" {
		s += " id=" + e.ID
	}
	return s + " " + e.Data
}

// Kind returns a match for an event of kind whose data contains each of
// has: Kind("state", `"device":"0a0101"`).
func Kind(kind string, has ...string) func(Event) bool {
	return func(e Event) bool {
		if e.Kind != kind {
			return false
		}
		for _, h := range has {
			if !strings.Contains(e.Data, h) {
				return false
			}
		}
		return true
	}
}

// StreamWatch returns the watch of an event stream's body, as an HTTP
// response the test opened and checked carries it: read until the
// stream ends, which ends the watch, or until the test ends, which
// closes the body. The watch's Err is nil where the server ended the
// stream, and the read's error where the stream was cut.
func StreamWatch(t testing.TB, body io.ReadCloser) *Watch[Event] {
	w := NewWatch[Event](nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.End(ReadEvents(w, body))
	}()
	t.Cleanup(func() {
		body.Close()
		<-done
	})
	return w
}

// ReadEvents feeds w with each event and comment read from r until r
// ends, and returns the error the read ended with, nil at the end of r.
// An event is sent at the blank line after its fields; a comment at
// once. It does not end w.
func ReadEvents(w *Watch[Event], r io.Reader) error {
	rd := bufio.NewReader(r)
	var ev Event
	var data []string
	pending := false
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			// A line without its newline is cut off: no event of its own.
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch {
		case line == "":
			if pending {
				ev.Data = strings.Join(data, "\n")
				w.Add(ev)
			}
			ev, data, pending = Event{}, nil, false
		case field == "":
			w.Add(Event{Comment: value})
		case field == "id":
			ev.ID, pending = value, true
		case field == "event":
			ev.Kind, pending = value, true
		case field == "data":
			data, pending = append(data, value), true
		}
	}
}
