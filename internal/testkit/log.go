// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
)

// A Record is a log record as a watch keeps it: read back from the line
// slog's text handler writes for it, or from a line a process wrote, so
// that a record logged in the test's process and one read from a child's
// output look the same.
type Record struct {
	Time  time.Time
	Level slog.Level
	Msg   string
	// Attrs are the record's attributes, a group's joined to its name by
	// a dot ("cube.serial"), each value as the text handler writes it,
	// unquoted: "1.5s" for a time.Duration, the text of an error.
	Attrs map[string]string
	// Line is the line the record was read from, without its newline.
	Line string
}

// String is the record's line without its time, as a failure shows it.
func (r Record) String() string {
	if r.Line == "" {
		var b strings.Builder
		fmt.Fprintf(&b, "level=%s msg=%q", r.Level, r.Msg)
		for _, k := range slices.Sorted(maps.Keys(r.Attrs)) {
			fmt.Fprintf(&b, " %s=%q", k, r.Attrs[k])
		}
		return b.String()
	}
	if rest, ok := strings.CutPrefix(r.Line, "time="); ok {
		if _, after, ok := strings.Cut(rest, " "); ok {
			return after
		}
	}
	return r.Line
}

// Msg returns a match for a record of msg whose attributes include each
// pair of attrs, a key and its value in turn: Msg("cube connected",
// "serial", "KEQ0000000").
func Msg(msg string, attrs ...string) func(Record) bool {
	return func(r Record) bool { return r.Msg == msg && hasAttrs(r, attrs) }
}

// MsgAt is Msg for a record at level only.
func MsgAt(level slog.Level, msg string, attrs ...string) func(Record) bool {
	return func(r Record) bool { return r.Level == level && r.Msg == msg && hasAttrs(r, attrs) }
}

// Contains returns a match for a record whose line, as String shows it
// (without its time), contains each of has: a part of the message, of
// an attribute's value, or "key=value" as the line has it. It is for a
// record known by a part of its text - a message that carries a number,
// a word anywhere in the line; a record known by its whole message is
// Msg's.
func Contains(has ...string) func(Record) bool {
	return func(r Record) bool {
		line := r.String()
		for _, h := range has {
			if !strings.Contains(line, h) {
				return false
			}
		}
		return true
	}
}

// ContainsAt is Contains for a record at level only.
func ContainsAt(level slog.Level, has ...string) func(Record) bool {
	contains := Contains(has...)
	return func(r Record) bool { return r.Level == level && contains(r) }
}

// hasAttrs reports whether r has each pair of attrs; a key without its
// value is a mistake of the test's, and matches nothing.
func hasAttrs(r Record, attrs []string) bool {
	if len(attrs)%2 != 0 {
		return false
	}
	for i := 0; i < len(attrs); i += 2 {
		if v, ok := r.Attrs[attrs[i]]; !ok || v != attrs[i+1] {
			return false
		}
	}
	return true
}

// LogWatch returns a logger and the watch of its records at level and
// above; a nil level is Info, as slog's handlers have it. The logger
// writes to each tee handler too, at the tee's own level, for a test
// that also shows the log.
func LogWatch(level slog.Leveler, tee ...slog.Handler) (*slog.Logger, *Watch[Record]) {
	w := NewWatch[Record](nil)
	// The text handler writes each record with one Write (its doc says
	// so), and handles WithAttrs and WithGroup as the product's loggers
	// do; the record is read back from that line.
	var h slog.Handler = slog.NewTextHandler(recordWriter{w}, &slog.HandlerOptions{Level: level})
	if len(tee) > 0 {
		h = slog.NewMultiHandler(append([]slog.Handler{h}, tee...)...)
	}
	return slog.New(h), w
}

// recordWriter feeds a watch with the record of each line written.
type recordWriter struct{ w *Watch[Record] }

// Write feeds the watch the record of each line in p.
func (rw recordWriter) Write(p []byte) (int, error) {
	for line := range bytes.Lines(p) {
		rw.w.Add(parseRecord(string(bytes.TrimRight(line, "\r\n"))))
	}
	return len(p), nil
}

// ReadLog feeds w with a record for each line read from r until r ends,
// and returns the error the read ended with, nil at the end of r: for a
// child process's log, a line of slog's text or JSON handler each. A
// line that is neither is a record with its Line alone. It does not end
// w, so that the caller can end it with what it knows of the end (the
// process's exit). It returns once r is read: the caller of
// exec.Cmd.Wait calls it first.
func ReadLog(w *Watch[Record], r io.Reader) error {
	rd := bufio.NewReader(r)
	for {
		line, err := rd.ReadString('\n')
		if line != "" {
			w.Add(parseRecord(strings.TrimRight(line, "\r\n")))
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// parseRecord reads a line of slog's JSON handler, or of its text
// handler.
func parseRecord(line string) Record {
	if strings.HasPrefix(line, "{") {
		if r, ok := parseJSONRecord(line); ok {
			return r
		}
	}
	return parseTextRecord(line)
}

// parseTextRecord reads a line of slog's text handler: key=value pairs,
// a value quoted where it needs to be. The built-in keys (time, level,
// msg) are taken as such up to msg; every key after it is an attribute.
// Where the line stops being pairs, the rest is left in Line alone.
func parseTextRecord(line string) Record {
	r := Record{Attrs: map[string]string{}, Line: line}
	msgSeen := false
	for s := line; ; {
		s = strings.TrimLeft(s, " ")
		if s == "" {
			return r
		}
		key, rest, ok := cutText(s, true)
		if !ok || !strings.HasPrefix(rest, "=") {
			return r
		}
		val, rest, ok := cutText(rest[1:], false)
		if !ok {
			return r
		}
		s = rest
		if !msgSeen {
			switch key {
			case slog.TimeKey:
				if t, err := time.Parse(time.RFC3339Nano, val); err == nil {
					r.Time = t
					continue
				}
			case slog.LevelKey:
				if r.Level.UnmarshalText([]byte(val)) == nil {
					continue
				}
			case slog.MessageKey:
				r.Msg, msgSeen = val, true
				continue
			}
		}
		r.Attrs[key] = val
	}
}

// cutText cuts a key (up to "=") or a value (up to a space) off s,
// unquoting it where it is quoted.
func cutText(s string, key bool) (string, string, bool) {
	if strings.HasPrefix(s, `"`) {
		q, err := strconv.QuotedPrefix(s)
		if err != nil {
			return "", "", false
		}
		v, err := strconv.Unquote(q)
		return v, s[len(q):], err == nil
	}
	if key {
		i := strings.IndexAny(s, "= ")
		if i <= 0 || s[i] != '=' {
			return "", "", false
		}
		return s[:i], s[i:], true
	}
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i], s[i:], true
	}
	return s, "", true
}

// parseJSONRecord reads a line of slog's JSON handler; a group is an
// object, its keys joined to the group's name by a dot.
func parseJSONRecord(line string) (Record, bool) {
	dec := json.NewDecoder(strings.NewReader(line))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return Record{}, false
	}
	r := Record{Attrs: map[string]string{}, Line: line}
	for k, v := range obj {
		s, isString := v.(string)
		switch {
		case k == slog.TimeKey && isString:
			if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
				r.Time = t
				continue
			}
		case k == slog.LevelKey && isString:
			if r.Level.UnmarshalText([]byte(s)) == nil {
				continue
			}
		case k == slog.MessageKey && isString:
			r.Msg = s
			continue
		}
		flatten(r.Attrs, k, v)
	}
	return r, true
}

// flatten writes a JSON value into attrs under key, an object's members
// under key and a dot.
func flatten(attrs map[string]string, key string, v any) {
	switch v := v.(type) {
	case map[string]any:
		for k, sub := range v {
			flatten(attrs, key+"."+k, sub)
		}
	case string:
		attrs[key] = v
	case json.Number:
		attrs[key] = v.String()
	case bool:
		attrs[key] = strconv.FormatBool(v)
	case nil:
		attrs[key] = ""
	default:
		b, _ := json.Marshal(v)
		attrs[key] = string(b)
	}
}
