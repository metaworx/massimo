// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package notify

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// scanEvery is how often the Sink reads its users again and runs the
// stale scan: a revoked session or a removed user stops being told
// within it, as does a preference set behind the API's back.
const scanEvery = 5 * time.Minute

// Clock is the time the rules run on; the tests drive one of their own.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

// Timer is a Clock's timer.
type Timer interface{ Stop() bool }

type realClock struct{}

func (realClock) Now() time.Time                            { return time.Now() }
func (realClock) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

// Options are what a Sink runs with.
type Options struct {
	// Users are the users with a push subscription, with their
	// preference; read at the start, on Refresh, and every five minutes.
	Users func(ctx context.Context) ([]User, error)
	// Deliver takes a notification for a user, on the Sink's goroutine:
	// it must not wait for the network.
	Deliver func(userID int64, n Notification)
	// Devices are every Cube's devices for the stale scan; nil for none.
	Devices func() []Device
	// Clock is the time; nil is the real one.
	Clock  Clock
	Logger *slog.Logger
}

// Sink runs the rules over the events it is given, for every user
// Options.Users names. Push never waits: the events queue, nothing is
// dropped, and Run takes them on a goroutine of its own, so that a slow
// deliverer never slows a Cube.
type Sink struct {
	o Options

	mu    sync.Mutex
	queue []item
	wake  chan struct{}
	seq   uint64

	// Run's goroutine alone:
	engines map[int64]*engine
	scanner Timer
}

// item is one thing for Run to take.
type item struct {
	ev      *Event
	fire    *fire
	refresh bool
	scan    bool
	done    chan struct{} // closed once the items before it are taken
}

// fire is a delay's timer, gone off.
type fire struct {
	user int64
	key  string
	seq  uint64
}

// New makes a Sink; Run starts it.
func New(o Options) *Sink {
	if o.Clock == nil {
		o.Clock = realClock{}
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Deliver == nil {
		o.Deliver = func(int64, Notification) {}
	}
	return &Sink{o: o, wake: make(chan struct{}, 1), engines: map[int64]*engine{}}
}

// Push hands the Sink an event; it never waits.
func (s *Sink) Push(ev Event) { s.enqueue(item{ev: &ev}) }

// Refresh asks the Sink to read its users and their preferences again:
// a subscription or a notify preference changed.
func (s *Sink) Refresh() { s.enqueue(item{refresh: true}) }

func (s *Sink) enqueue(it item) {
	s.mu.Lock()
	s.queue = append(s.queue, it)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Sink) nextSeq() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return s.seq
}

// sync waits until what was pushed before it is taken; for the tests.
func (s *Sink) sync() {
	done := make(chan struct{})
	s.enqueue(item{done: done})
	<-done
}

// Run takes the events and runs the rules until ctx ends, then stops
// every delay.
func (s *Sink) Run(ctx context.Context) {
	s.refresh(ctx)
	s.scheduleScan()
	defer func() {
		if s.scanner != nil {
			s.scanner.Stop()
		}
		for _, e := range s.engines {
			e.stop()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
			s.drain(ctx)
		}
	}
}

// drain takes every queued item.
func (s *Sink) drain(ctx context.Context) {
	for {
		s.mu.Lock()
		items := s.queue
		s.queue = nil
		s.mu.Unlock()
		if len(items) == 0 {
			return
		}
		for _, it := range items {
			s.take(ctx, it)
		}
	}
}

func (s *Sink) take(ctx context.Context, it item) {
	switch {
	case it.ev != nil:
		for _, e := range s.users() {
			e.event(*it.ev)
		}
	case it.fire != nil:
		if e := s.engines[it.fire.user]; e != nil {
			e.fired(it.fire.key, it.fire.seq)
		}
	case it.refresh:
		s.refresh(ctx)
	case it.scan:
		s.refresh(ctx)
		if s.o.Devices != nil {
			devices := s.o.Devices()
			for _, e := range s.users() {
				e.scan(devices)
			}
		}
		s.scheduleScan()
	case it.done != nil:
		close(it.done)
	}
}

// refresh reads the users again: a new one gets rules of its own, a
// known one its preference, one no longer named loses its rules.
func (s *Sink) refresh(ctx context.Context) {
	if s.o.Users == nil {
		return
	}
	users, err := s.o.Users(ctx)
	if err != nil {
		s.o.Logger.Warn("notify: the users with a push subscription not read; the rules run for those known", "error", err)
		return
	}
	seen := map[int64]bool{}
	for _, u := range users {
		seen[u.ID] = true
		if e := s.engines[u.ID]; e != nil {
			e.user.Prefs = u.Prefs
		} else {
			s.engines[u.ID] = newEngine(s, u)
		}
	}
	for id, e := range s.engines {
		if !seen[id] {
			e.stop()
			delete(s.engines, id)
		}
	}
}

// users are the rules of every user, in the order of their ids, so
// that the users are told in the same order every time.
func (s *Sink) users() []*engine {
	out := make([]*engine, 0, len(s.engines))
	for _, e := range s.engines {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].user.ID < out[j].user.ID })
	return out
}

func (s *Sink) scheduleScan() {
	s.scanner = s.o.Clock.AfterFunc(scanEvery, func() { s.enqueue(item{scan: true}) })
}
