// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubewatch

import (
	"fmt"
	"net"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

const fixture = "../../protocol/cube/testdata/cube-fw0113-greeting.txt"

// client is a raw connection to a fake Cube that has read its greeting.
type client struct {
	t    *testing.T
	conn net.Conn
	r    *cube.Reader
}

// dial connects to the fake at addr and reads its greeting.
func dial(t *testing.T, addr string) *client {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetDeadline(time.Now().Add(testkit.Scale(testkit.Wait)))
	c := &client{t: t, conn: conn, r: cube.NewReader(conn)}
	for m := c.read(); m.Type != 'L'; m = c.read() {
	}
	return c
}

func (c *client) read() cube.Message {
	c.t.Helper()
	m, err := c.r.Read()
	if err != nil {
		c.t.Fatal(err)
	}
	return m
}

// send writes m and reads the fake's answer.
func (c *client) send(m cube.Message) cube.Message {
	c.t.Helper()
	if _, err := c.conn.Write([]byte(m.Wire())); err != nil {
		c.t.Fatal(err)
	}
	return c.read()
}

// startFake starts a fake Cube with the recorded greeting.
func startFake(t *testing.T) (*fakecube.Server, string) {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fc.Close() })
	return fc, addr
}

// strs is the lines as the Cube received them.
func strs(lines []cube.Message) []string {
	var out []string
	for _, m := range lines {
		out = append(out, m.String())
	}
	return out
}

// The watch holds the lines received before it was made and each after,
// as Received does, so that its cursor is an index into Received too;
// the filters pick the lines by type.
func TestLines(t *testing.T) {
	fc, addr := startFake(t)
	c := dial(t, addr)
	c.send(cube.CmdList)
	w := Lines(fc)
	m := w.Mark()
	if m != 1 {
		t.Fatalf("mark %d after one line", m)
	}
	c.send(cube.Message{Type: 'c', Data: "0a0101"})
	c.send(cube.CmdList)
	c.send(cube.Message{Type: 'f'})
	set, err := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0101, RoomID: 1, Room: true}, 21, cube.ModeManual, cube.Until{})
	if err != nil {
		t.Fatal(err)
	}
	c.send(set)
	if got, next := w.Await(t, m, "the s: line", Type('s')); got.String() != set.String() || next != 5 {
		t.Fatalf("got %s, next %d", got, next)
	}
	if got := strs(w.Since(0, nil)); !slices.Equal(got, strs(fc.Received())) {
		t.Fatalf("the watch %q, Received %q", got, strs(fc.Received()))
	}
	if got := strs(w.Since(m, Except('l'))); !slices.Equal(got, []string{"c:0a0101", "f:", set.String()}) {
		t.Errorf("without the polls: %q", got)
	}
	if got := strs(w.Since(m, Except('l', 'f'))); !slices.Equal(got, []string{"c:0a0101", set.String()}) {
		t.Errorf("without the polls and NTP reads: %q", got)
	}
	if n := w.Count(0, Type('l', 'f')); n != 3 {
		t.Errorf("%d polls and NTP reads, want 3", n)
	}
	// Two watches of one fake both get every line.
	w2 := Lines(fc)
	c.send(cube.CmdList)
	w.Await(t, 5, "the last poll", Type('l'))
	w2.Await(t, 5, "the last poll", Type('l'))
}

// A failure shows the lines as the Cube received them.
func TestLinesFailureShowsTheLines(t *testing.T) {
	fc, addr := startFake(t)
	w := Lines(fc)
	c := dial(t, addr)
	c.send(cube.CmdList)
	w.Await(t, 0, "the poll", Type('l'))
	// A test that records its failure instead of failing.
	failed := ""
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Await(recorder{TB: t, failed: &failed}, 0, "an s: line", Type('s'), testkit.For(time.Millisecond))
	}()
	<-done
	if !strings.HasSuffix(failed, "waiting for an s: line; 1 since the mark:\n\t1 l:") {
		t.Errorf("failure %q", failed)
	}
}

// recorder is a test that records its failure instead of failing; its
// Fatalf ends the goroutine it runs in, as the real one does.
type recorder struct {
	testing.TB
	failed *string
}

func (r recorder) Helper() {}

func (r recorder) Fatalf(format string, args ...any) {
	*r.failed = fmt.Sprintf(format, args...)
	runtime.Goexit()
}
