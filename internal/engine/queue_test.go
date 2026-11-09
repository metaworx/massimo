// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package engine

import (
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

const fixture = "../protocol/cube/testdata/cube-fw0113-greeting.txt"

// withError returns the greeting with the error flag set on rf's L entry.
func withError(t *testing.T, greeting []cube.Message, rf cube.RFAddress) []cube.Message {
	t.Helper()
	out := append([]cube.Message(nil), greeting...)
	for i, m := range out {
		if m.Type != 'L' {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(m.Data)
		if err != nil {
			t.Fatal(err)
		}
		for off := 0; off < len(b); off += 1 + int(b[off]) {
			if cube.RFAddressFromBytes(b[off+1:off+4]) == rf {
				b[off+5] |= 0x08
			}
		}
		out[i].Data = base64.StdEncoding.EncodeToString(b)
	}
	return out
}

// waitFor polls until the fake cube received a command of type typ.
func waitFor(t *testing.T, srv *fakecube.Server, typ byte) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, m := range srv.Received() {
			if m.Type == typ {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("fake cube did not receive a %c command", typ)
}

func setup(t *testing.T, opts Options, patch ...func([]cube.Message) []cube.Message) (*fakecube.Server, *Queue, context.CancelFunc) {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range patch {
		greeting = p(greeting)
	}
	srv := &fakecube.Server{Greeting: greeting}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := cubelan.New(cubelan.Options{Addr: addr})
	go c.Run(ctx)
	select {
	case <-c.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("client not ready")
	}
	if opts.Gap == 0 {
		opts.Gap = time.Millisecond
	}
	q := NewQueue(c, opts)
	go q.Run(ctx)
	t.Cleanup(func() { cancel(); srv.Close() })
	return srv, q, cancel
}

func received(srv *fakecube.Server) []string {
	var out []string
	for _, m := range srv.Received() {
		if m.Type != 'l' {
			out = append(out, m.String())
		}
	}
	return out
}

func TestQueueResetsErrorStateAndSendsCommand(t *testing.T) {
	srv, q, _ := setup(t, Options{}, func(g []cube.Message) []cube.Message { return withError(t, g, 0x0a010d) })
	// The thermostat is in error state, so an r: must precede the s:.
	m, _ := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a010d, RoomID: 6, Room: true}, 19, cube.ModeManual, cube.Until{})
	res := q.Enqueue(Command{Target: 0x0a010d, Lines: []cube.Message{m}}).Wait(context.Background())
	if res.Err != nil || res.Discarded {
		t.Fatalf("result %+v", res)
	}
	got := received(srv)
	if len(got) != 2 || got[0] != "r:01,CgEN" || got[1] != m.String() {
		t.Fatalf("sent %v", got)
	}
	if res.Budget.FreeMemorySlots != 0x31 {
		t.Fatalf("budget not updated: %+v", res.Budget)
	}
}

func TestQueueWakeupAndOrdering(t *testing.T) {
	srv, q, _ := setup(t, Options{Gap: 20 * time.Millisecond}, func(g []cube.Message) []cube.Message { return withError(t, g, 0x0a0108) })
	prog, _ := cube.CmdSetDayProgram(cube.Target{RFAddress: 0x0a0108, RoomID: 3, Room: true}, cube.Monday, cube.DayProgram{{Temp: 17, Until: 360}, {Temp: 21, Until: 1440}})
	low := q.Enqueue(Command{Priority: PriorityLow, Target: 0x0a0108, RoomID: 3, Wakeup: true, Lines: prog})
	waitFor(t, srv, 'z') // low is in flight before high arrives
	high := q.Enqueue(Command{Priority: PriorityHigh, Lines: []cube.Message{cube.CmdGetNTP}})
	ctx := context.Background()
	if r := high.Wait(ctx); r.Err != nil {
		t.Fatal(r.Err)
	}
	if r := low.Wait(ctx); r.Err != nil {
		t.Fatal(r.Err)
	}
	got := received(srv)
	// The low-priority command is in flight when the high-priority one
	// arrives, so order is: wakeup, r:, s:, then f:.
	want := []string{"z:1e,G,03", "r:01,CgEI", prog[0].String(), "f:"}
	if len(got) != len(want) {
		t.Fatalf("sent %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sent %v, want %v", got, want)
		}
	}
}

func TestQueueCoalescesByKey(t *testing.T) {
	_, q, _ := setup(t, Options{Gap: 50 * time.Millisecond})
	ctx := context.Background()
	// Occupy the worker, then queue two commands with the same key.
	first := q.Enqueue(Command{Lines: []cube.Message{cube.CmdGetNTP}})
	a := q.Enqueue(Command{Key: "ntp", Lines: []cube.Message{cube.CmdSetNTP("a")}})
	b := q.Enqueue(Command{Key: "ntp", Lines: []cube.Message{cube.CmdSetNTP("b")}})
	if r := a.Wait(ctx); r.Err != ErrSuperseded {
		t.Fatalf("a: %v", r.Err)
	}
	if r := b.Wait(ctx); r.Err != nil {
		t.Fatalf("b: %v", r.Err)
	}
	first.Wait(ctx)
}

func TestQueueWaitsForDutyCycleReset(t *testing.T) {
	// Pretend the duty cycle is exhausted; the reset is 100 ms away.
	resetAt := func(now time.Time) time.Time { return now.Add(100 * time.Millisecond) }
	srv, q, _ := setup(t, Options{ResetAt: resetAt})
	srv.Duty = 0x64 // 100 %
	// First radio command learns the budget (100 %), second one must wait.
	m, _ := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0108, RoomID: 3, Room: true}, 18, cube.ModeManual, cube.Until{})
	q.Enqueue(Command{Lines: []cube.Message{m}}).Wait(context.Background())
	srv.Duty = 0
	start := time.Now()
	res := q.Enqueue(Command{Lines: []cube.Message{m}}).Wait(context.Background())
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if d := time.Since(start); d < 90*time.Millisecond {
		t.Fatalf("did not wait for the reset: %s", d)
	}
}

func TestQueueRetriesAfterDiscard(t *testing.T) {
	srv, q, _ := setup(t, Options{})
	srv.Discard = true
	m, _ := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0108, RoomID: 3, Room: true}, 18, cube.ModeManual, cube.Until{})
	res := q.Enqueue(Command{Target: 0x0a0108, Lines: []cube.Message{m}}).Wait(context.Background())
	if res.Err != ErrDiscarded || !res.Discarded {
		t.Fatalf("result %+v", res)
	}
	// s: discarded, then r: and a second s:, discarded again.
	want := []string{m.String(), "r:01,CgEI", m.String()}
	got := received(srv)
	if len(got) != len(want) {
		t.Fatalf("sent %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sent %v, want %v", got, want)
		}
	}
}

func TestQueueDrainsOnCancel(t *testing.T) {
	_, q, cancel := setup(t, Options{Gap: time.Second})
	first := q.Enqueue(Command{Lines: []cube.Message{cube.CmdGetNTP}})
	second := q.Enqueue(Command{Lines: []cube.Message{cube.CmdGetNTP}})
	first.Wait(context.Background())
	cancel()
	if r := second.Wait(context.Background()); r.Err != context.Canceled {
		t.Fatalf("second: %v", r.Err)
	}
}

// syncBuffer is a bytes.Buffer safe for the queue's goroutine.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestQueueLogsAcceptedCommands(t *testing.T) {
	var log syncBuffer
	_, q, _ := setup(t, Options{Logger: slog.New(slog.NewTextHandler(&log, nil))})
	m, _ := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0108, RoomID: 3, Room: true}, 21, cube.ModeManual, cube.Until{})
	if r := q.Enqueue(Command{Target: 0x0a0108, RoomID: 3, Wakeup: true, Lines: []cube.Message{m}}).Wait(context.Background()); r.Err != nil {
		t.Fatal(r.Err)
	}
	out := log.String()
	for _, want := range []string{
		`msg="room woken up" room=3 command=z:1e,G,03 reply=A:`,
		`msg="command accepted by the cube" target=0a0108 command="` + m.String() + `" duty_pct=0 free_slots=49`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q:\n%s", want, out)
		}
	}
}

// recorder is an Observer that keeps every call.
type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) note(what string, c Command, res *Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := what + ":" + c.ID
	if res != nil && res.Err != nil {
		s += ":" + res.Err.Error()
	}
	r.calls = append(r.calls, s)
}

func (r *recorder) Enqueued(c Command)             { r.note("enqueued", c, nil) }
func (r *recorder) Started(c Command)              { r.note("started", c, nil) }
func (r *recorder) Finished(c Command, res Result) { r.note("finished", c, &res) }

func (r *recorder) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

// startGatedRecorder holds the Started call of one command until
// released, so that what a test queues meanwhile is recorded first.
type startGatedRecorder struct {
	recorder
	gate    string
	release chan struct{}
}

func (r *startGatedRecorder) Started(c Command) {
	if c.ID == r.gate {
		<-r.release
	}
	r.recorder.Started(c)
}

// The observer sees every command with an ID join the queue, start and
// finish, a superseded and a drained one included, and none without.
func TestQueueObserver(t *testing.T) {
	rec := &startGatedRecorder{gate: "a", release: make(chan struct{})}
	_, q, cancel := setup(t, Options{Gap: 50 * time.Millisecond, Observer: rec})
	line := cube.Message{Type: 's', Data: "AARAAAAACgENBms="}
	// The first command's start is held while the next two, with one
	// key, are queued: the first of them is superseded. The queue may
	// take the first command at once, and on a slow machine it does.
	first := q.Enqueue(Command{ID: "a", Lines: []cube.Message{line}})
	q.Enqueue(Command{ID: "b", Key: "k", Lines: []cube.Message{line}})
	third := q.Enqueue(Command{ID: "c", Key: "k", Lines: []cube.Message{line}})
	q.Enqueue(Command{Lines: []cube.Message{line}}) // no ID: not observed
	close(rec.release)
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if res := first.Wait(ctx); res.Err != nil {
		t.Fatal(res.Err)
	}
	if res := third.Wait(ctx); res.Err != nil {
		t.Fatal(res.Err)
	}
	// A command still waiting when the queue stops is drained.
	q.Enqueue(Command{ID: "d", Lines: []cube.Message{line}})
	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(rec.list()) < 10 {
		time.Sleep(10 * time.Millisecond)
	}
	got := rec.list()
	want := []string{"enqueued:a", "enqueued:b", "finished:b:" + ErrSuperseded.Error(), "enqueued:c", "started:a", "finished:a", "started:c", "finished:c", "enqueued:d"}
	for i, w := range want {
		if i >= len(got) || got[i] != w {
			t.Fatalf("calls %v, want %v", got, want)
		}
	}
	if last := got[len(got)-1]; !strings.HasPrefix(last, "finished:d:") || len(got) != len(want)+1 {
		t.Fatalf("calls %v: the drained command", got)
	}
	for _, c := range got {
		if strings.HasSuffix(c, ":") {
			t.Fatalf("a command without an ID was observed: %v", got)
		}
	}
}

// gatedRecorder holds the Enqueued call of one command until released.
type gatedRecorder struct {
	recorder
	gate    string
	release chan struct{}
}

func (r *gatedRecorder) Enqueued(c Command) {
	if c.ID == r.gate {
		<-r.release
	}
	r.recorder.Enqueued(c)
}

// The observer hears of a command before the queue can start it: a
// command enqueued while another runs does not start while its Enqueued
// call is still going on, so that the step it records is there for
// Started and Finished.
func TestQueueObserverHearsEnqueuedFirst(t *testing.T) {
	rec := &gatedRecorder{gate: "b", release: make(chan struct{})}
	_, q, _ := setup(t, Options{Gap: 20 * time.Millisecond, Observer: rec})
	line := cube.Message{Type: 's', Data: "AARAAAAACgENBms="}
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	first := q.Enqueue(Command{ID: "a", Lines: []cube.Message{line}})
	second := make(chan *Ticket, 1)
	go func() { second <- q.Enqueue(Command{ID: "b", Lines: []cube.Message{line}}) }()
	// Long enough for a and its gap: a queue that can see b takes it now.
	// (The observer must not block; this one does, to show the order.)
	time.Sleep(300 * time.Millisecond)
	close(rec.release)
	if res := first.Wait(ctx); res.Err != nil {
		t.Fatal(res.Err)
	}
	if res := (<-second).Wait(ctx); res.Err != nil {
		t.Fatal(res.Err)
	}
	got := rec.list()
	at := func(s string) int {
		for i, c := range got {
			if c == s {
				return i
			}
		}
		return -1
	}
	if e, s := at("enqueued:b"), at("started:b"); e < 0 || s < 0 || s < e {
		t.Fatalf("calls %v: b started before its Enqueued returned", got)
	}
}
