// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"maps"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

// The log's watch keeps each record with its level, message and
// attributes, those of With and of groups too, joined by dots, each
// value as the text handler writes it.
func TestLogWatchRecords(t *testing.T) {
	log, w := LogWatch(nil)
	m := w.Mark()
	log.With("cube", "KEQ0000000").WithGroup("dev").With("rf", "0a0101").
		Info("state changed", "temp", 21.5, "after", 1500*time.Millisecond, slog.Group("why", "err", errors.New(`no "S" reply`)))
	log.Warn("two\nlines", "level", "mine")
	log.Debug("not kept at Info")

	r, next := w.Await(t, m, "the state", Msg("state changed"))
	want := map[string]string{"cube": "KEQ0000000", "dev.rf": "0a0101", "dev.temp": "21.5", "dev.after": "1.5s", "dev.why.err": `no "S" reply`}
	if r.Level != slog.LevelInfo || !maps.Equal(r.Attrs, want) || r.Time.IsZero() {
		t.Errorf("record %+v, want attrs %v", r, want)
	}
	if !strings.HasPrefix(r.Line, "time=") || strings.HasPrefix(r.String(), "time=") || !strings.HasPrefix(r.String(), `level=INFO msg="state changed" cube=KEQ0000000`) {
		t.Errorf("line %q, shown %q", r.Line, r)
	}
	// A key named as a built-in after the message is an attribute.
	r, _ = w.Await(t, next, "the warning", MsgAt(slog.LevelWarn, "two\nlines", "level", "mine"))
	if r.Level != slog.LevelWarn || r.Attrs["level"] != "mine" {
		t.Errorf("warning %+v", r)
	}
	if n := w.Count(m, nil); n != 2 {
		t.Errorf("%d records, want the two at Info and above", n)
	}
}

// The level given keeps the records below Info too; a tee handler gets
// every record it is enabled for, at its own level.
func TestLogWatchLevelAndTee(t *testing.T) {
	var tee bytes.Buffer
	log, w := LogWatch(slog.LevelDebug, slog.NewTextHandler(&tee, &slog.HandlerOptions{Level: slog.LevelInfo}))
	log.Debug("dial failed", "n", 3)
	log.Info("connected")
	if got := w.Since(0, MsgAt(slog.LevelDebug, "dial failed", "n", "3")); len(got) != 1 {
		t.Errorf("debug records %v", w.Since(0, nil))
	}
	if s := tee.String(); strings.Contains(s, "dial failed") || !strings.Contains(s, "msg=connected") {
		t.Errorf("the tee got %q", s)
	}
	if !w.Seen(0, Msg("connected"))() {
		t.Error("the watch lacks the Info record")
	}
}

// Msg and MsgAt match a record's message, its level, and the pairs of
// attributes given; a key without its value matches nothing.
func TestMsgMatches(t *testing.T) {
	r := Record{Level: slog.LevelWarn, Msg: "lost", Attrs: map[string]string{"serial": "KEQ0000000", "n": "2"}}
	for i, c := range []struct {
		match func(Record) bool
		want  bool
	}{
		{Msg("lost"), true},
		{Msg("lost", "serial", "KEQ0000000"), true},
		{Msg("lost", "serial", "KEQ0000000", "n", "2"), true},
		{Msg("lost", "serial", "KEQ0000001"), false},
		{Msg("lost", "missing", ""), false},
		{Msg("lost", "serial"), false},
		{Msg("lo"), false},
		{MsgAt(slog.LevelWarn, "lost"), true},
		{MsgAt(slog.LevelInfo, "lost"), false},
	} {
		if got := c.match(r); got != c.want {
			t.Errorf("match %d: %v, want %v", i+1, got, c.want)
		}
	}
}

// Contains matches a record whose line, without its time, holds each
// text given; ContainsAt one at its level too. A record without a line
// is matched as String writes it.
func TestContainsMatches(t *testing.T) {
	log, w := LogWatch(nil)
	log.Warn("tls: the certificate expires in 9 days", "cert", "/tmp/c.pem", "error", "dial: refused")
	r, _ := w.Await(t, 0, "the warning", nil)
	byHand := Record{Level: slog.LevelInfo, Msg: "cube served without a proxy", Attrs: map[string]string{"serial": "KEQ0000000"}}
	for i, c := range []struct {
		match func(Record) bool
		r     Record
		want  bool
	}{
		{Contains("expires in"), r, true},
		{Contains("expires in", "dial: refused", "cert=/tmp/c.pem"), r, true},
		{Contains("expires in", "renewed"), r, false},
		{Contains(), r, true},
		{Contains("time="), r, false},
		{Contains(r.Time.Format("2006")), r, false},
		{ContainsAt(slog.LevelWarn, "expires in"), r, true},
		{ContainsAt(slog.LevelInfo, "expires in"), r, false},
		{Contains("without a proxy", "KEQ0000000"), byHand, true},
		{ContainsAt(slog.LevelInfo, "serial="), byHand, true},
		{Contains("with a proxy"), byHand, false},
	} {
		if got := c.match(c.r); got != c.want {
			t.Errorf("match %d: %v, want %v", i+1, got, c.want)
		}
	}
}

// ReadLog reads a process's lines of slog's text and JSON handlers, and
// any other line as a record with its line alone; a last line without
// its newline counts.
func TestReadLog(t *testing.T) {
	out := strings.Join([]string{
		`time=2026-11-17T10:00:00.123+01:00 level=INFO msg="api listening" addr=127.0.0.1:8080`,
		`time=2026-11-17T10:00:01Z level=WARN+2 msg=quoted err="dial \"x\": refused"`,
		`{"time":"2026-11-17T10:00:02Z","level":"DEBUG","msg":"line","cube":{"serial":"KEQ0000000","n":3},"ok":true,"none":null,"list":[1,"a"]}`,
		`panic: runtime error`,
		`{not json`,
		"level=ERROR msg=last\r",
		`level=INFO msg=cut`,
	}, "\n")
	w := NewWatch[Record](nil)
	if err := ReadLog(w, strings.NewReader(out)); err != nil {
		t.Fatal(err)
	}
	got := w.Since(0, nil)
	if len(got) != 7 {
		t.Fatalf("%d records: %v", len(got), got)
	}
	if r := got[0]; r.Msg != "api listening" || r.Attrs["addr"] != "127.0.0.1:8080" || r.Level != slog.LevelInfo || r.Time.Nanosecond() != 123e6 {
		t.Errorf("text line %+v", r)
	}
	if r := got[1]; r.Level != slog.LevelWarn+2 || r.Attrs["err"] != `dial "x": refused` {
		t.Errorf("quoted %+v", r)
	}
	want := map[string]string{"cube.serial": "KEQ0000000", "cube.n": "3", "ok": "true", "none": "", "list": `[1,"a"]`}
	if r := got[2]; r.Msg != "line" || r.Level != slog.LevelDebug || !maps.Equal(r.Attrs, want) || r.Time.IsZero() {
		t.Errorf("JSON line %+v", r)
	}
	for i, line := range []string{"panic: runtime error", "{not json"} {
		if r := got[3+i]; r.Msg != "" || len(r.Attrs) != 0 || r.Line != line || r.String() != line {
			t.Errorf("other line %+v", r)
		}
	}
	if r := got[5]; r.Msg != "last" || r.Level != slog.LevelError || r.Line != "level=ERROR msg=last" {
		t.Errorf("CRLF line %+v", r)
	}
	if r := got[6]; r.Msg != "cut" {
		t.Errorf("the line without its newline %+v", r)
	}
	// The read's error is ReadLog's; the records before it are kept.
	w2 := NewWatch[Record](nil)
	if err := ReadLog(w2, io.MultiReader(strings.NewReader("msg=one\n"), iotest.ErrReader(errors.New("broken")))); err == nil || err.Error() != "broken" {
		t.Errorf("error %v", err)
	}
	if w2.Count(0, Msg("one")) != 1 || w2.Ended() {
		t.Errorf("before the error: %v, ended %v", w2.Since(0, nil), w2.Ended())
	}
}

// A record logged in the test's process and the same record read from a
// process's output are the same, but for the line's time.
func TestLogWatchAndReadLogAgree(t *testing.T) {
	var text bytes.Buffer
	log, w := LogWatch(nil, slog.NewTextHandler(&text, nil))
	log.WithGroup("g").Info("both", "d", time.Second, "s", "a b")
	read := NewWatch[Record](nil)
	if err := ReadLog(read, &text); err != nil {
		t.Fatal(err)
	}
	a, b := w.Since(0, nil), read.Since(0, nil)
	if len(a) != 1 || len(b) != 1 || a[0].Msg != b[0].Msg || !maps.Equal(a[0].Attrs, b[0].Attrs) || a[0].String() != b[0].String() {
		t.Errorf("in the process %+v, read %+v", a, b)
	}
}

// The handler takes records from many goroutines and from a context.
func TestLogWatchHandler(t *testing.T) {
	log, w := LogWatch(nil)
	if !log.Handler().Enabled(context.Background(), slog.LevelInfo) || log.Handler().Enabled(context.Background(), slog.LevelDebug) {
		t.Error("Enabled")
	}
	done := make(chan struct{})
	for i := range 4 {
		go func() {
			defer func() { done <- struct{}{} }()
			for range 50 {
				log.InfoContext(context.Background(), "tick", "g", i)
			}
		}()
	}
	for range 4 {
		<-done
	}
	if n := w.Count(0, Msg("tick")); n != 200 {
		t.Errorf("%d records, want 200", n)
	}
}
