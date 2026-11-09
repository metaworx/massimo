// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package notify

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClock is a clock the test moves: Advance runs the timers that
// come due, in their order.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	c       *fakeClock
	at      time.Time
	f       func()
	stopped bool
}

func (t *fakeTimer) Stop() bool {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	was := !t.stopped
	t.stopped = true
	return was
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{c: c, at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []*fakeTimer
	for _, t := range c.timers {
		if !t.stopped && !t.at.After(c.now) {
			t.stopped = true
			due = append(due, t)
		}
	}
	c.mu.Unlock()
	sort.SliceStable(due, func(i, j int) bool { return due[i].at.Before(due[j].at) })
	for _, t := range due {
		t.f()
	}
}

// harness runs a Sink on the fake clock and records what it delivers.
type harness struct {
	t       *testing.T
	clock   *fakeClock
	sink    *Sink
	mu      sync.Mutex
	users   []User
	devices []Device
	got     []delivered
}

type delivered struct {
	user int64
	n    Notification
}

func newHarness(t *testing.T, users ...User) *harness {
	t.Helper()
	h := &harness{t: t, clock: &fakeClock{now: time.Date(2026, 11, 9, 6, 0, 0, 0, time.UTC)}, users: users}
	h.sink = New(Options{
		Users: func(context.Context) ([]User, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			return append([]User(nil), h.users...), nil
		},
		Deliver: func(user int64, n Notification) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.got = append(h.got, delivered{user, n})
		},
		Devices: func() []Device {
			h.mu.Lock()
			defer h.mu.Unlock()
			return append([]Device(nil), h.devices...)
		},
		Clock: h.clock,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.sink.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	h.sink.sync()
	return h
}

func user(id int64, prefs string) User {
	p, err := ParsePrefs(json.RawMessage(prefs))
	if err != nil {
		panic(err)
	}
	return User{ID: id, Prefs: p}
}

// push hands the Sink an event at the clock's time and waits until the
// rules ran over it.
func (h *harness) push(ev Event) {
	if ev.Time.IsZero() {
		ev.Time = h.clock.Now()
	}
	h.sink.Push(ev)
	h.sink.sync()
}

func (h *harness) advance(d time.Duration) {
	h.clock.Advance(d)
	h.sink.sync()
}

// take returns what was delivered since the last take.
func (h *harness) take() []delivered {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := h.got
	h.got = nil
	return out
}

// kinds are the kinds of what was delivered since the last take.
func (h *harness) kinds() string {
	var out []string
	for _, d := range h.take() {
		out = append(out, d.n.Kind)
	}
	return strings.Join(out, ",")
}

func state(open bool) Event {
	return Event{Kind: "state", Cube: "KEQ0000100", Device: "0a0102", Serial: "MEQ0000001", DeviceName: "Window", Room: 3, RoomName: "Bath", State: &State{WindowOpen: &open}}
}

func temp(c float64) Event {
	return Event{Kind: "state", Cube: "KEQ0000100", Device: "0a0101", State: &State{ActualTemp: &c}}
}

func TestParsePrefs(t *testing.T) {
	p, err := ParsePrefs(nil)
	if err != nil || !p.on(KindWindowOpen) || p.after(KindWindowOpen) != 30*time.Minute || p.on(KindPaired) || *p[KindFrost].BelowC != 8 {
		t.Fatalf("defaults %+v %v", p, err)
	}
	p, err = ParsePrefs(json.RawMessage(`{"window_open": {"on": true, "after_min": 5}, "frost": {"on": false}, "paired": {"on": true}}`))
	if err != nil || p.after(KindWindowOpen) != 5*time.Minute || p.on(KindFrost) || *p[KindFrost].BelowC != 8 || !p.on(KindPaired) || !p.on(KindBatteryLow) {
		t.Fatalf("set %+v %v", p, err)
	}
	for _, bad := range []string{`[]`, `{"windows": {"on": true}}`, `{"battery_low": {"on": true, "after_min": 5}}`, `{"window_open": {"on": true, "below_c": 5}}`,
		`{"window_open": {"on": true, "after_min": 0}}`, `{"cube_offline": {"after_min": 1441}}`, `{"frost": {"below_c": 25}}`, `{"frost": {"on": "yes"}}`} {
		if _, err := ParsePrefs(json.RawMessage(bad)); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
	if got := Kinds(); len(got) != 10 || got[0] != KindBatteryLow {
		t.Errorf("kinds %v", got)
	}
}

// A window open is told once it stayed open for the delay, with when it
// opened; one that closes first is not.
func TestWindowOpen(t *testing.T) {
	h := newHarness(t, user(1, `{}`))
	opened := h.clock.Now()
	h.push(state(true))
	h.advance(29 * time.Minute)
	if k := h.kinds(); k != "" {
		t.Fatalf("before the delay: %s", k)
	}
	h.advance(time.Minute)
	got := h.take()
	if len(got) != 1 || got[0].user != 1 {
		t.Fatalf("after the delay: %+v", got)
	}
	n := got[0].n
	if n.Kind != KindWindowOpen || n.Cube != "KEQ0000100" || n.Device != "0a0102" || n.Serial != "MEQ0000001" || n.DeviceName != "Window" || n.Room != 3 || n.RoomName != "Bath" ||
		n.Since == nil || !n.Since.Equal(opened) || n.Values["minutes"] != 30 || !n.Time.Equal(opened.Add(30*time.Minute)) || n.URL != "/" {
		t.Fatalf("notification %+v", n)
	}

	h.push(state(false))
	h.advance(time.Hour) // past the coalescing
	h.push(state(true))
	h.advance(10 * time.Minute)
	h.push(state(false))
	h.advance(time.Hour)
	if k := h.kinds(); k != "" {
		t.Fatalf("closed within the delay: %s", k)
	}
}

// A Cube offline is told after the delay, and its return then; one back
// within the delay is neither.
func TestCubeOfflineAndOnline(t *testing.T) {
	h := newHarness(t, user(1, `{}`))
	cube := func(kind string) Event { return Event{Kind: kind, Cube: "KEQ0000100"} }
	h.push(cube("disconnected"))
	h.advance(9 * time.Minute)
	h.push(cube("connected"))
	h.advance(time.Hour)
	if k := h.kinds(); k != "" {
		t.Fatalf("back within the delay: %s", k)
	}
	h.push(cube("disconnected"))
	h.push(cube("disconnected")) // a retry's: the delay runs from the first
	h.advance(10 * time.Minute)
	got := h.take()
	if len(got) != 1 || got[0].n.Kind != KindCubeOffline || got[0].n.Values["minutes"] != 10 {
		t.Fatalf("offline: %+v", got)
	}
	h.advance(15 * time.Minute)
	h.push(cube("connected"))
	got = h.take()
	if len(got) != 1 || got[0].n.Kind != KindCubeOnline || got[0].n.Values["down_minutes"] != 25 {
		t.Fatalf("online: %+v", got)
	}
}

// A low battery and a device error are told when they begin, not while
// they last; frost when the temperature falls under the threshold, not
// again until it rose half a degree above it.
func TestDeviceConditions(t *testing.T) {
	h := newHarness(t, user(1, `{}`))
	ev := func(battery, errFlag, link bool) Event {
		return Event{Kind: "state", Cube: "KEQ0000100", Device: "0a0103", State: &State{BatteryLow: battery, Error: errFlag, LinkError: link}}
	}
	h.push(ev(false, false, false))
	h.push(ev(true, false, false))
	h.push(ev(true, false, false))
	h.push(ev(true, false, true))
	got := h.take()
	if len(got) != 2 || got[0].n.Kind != KindBatteryLow || got[1].n.Kind != KindDeviceError || got[1].n.Values["link_error"] != true || got[1].n.Values["error"] != false {
		t.Fatalf("battery and error: %+v", got)
	}

	h.push(temp(9))
	h.push(temp(7.5))
	got = h.take()
	if len(got) != 1 || got[0].n.Kind != KindFrost || got[0].n.Values["actual_c"] != 7.5 || got[0].n.Values["below_c"] != 8.0 {
		t.Fatalf("frost: %+v", got)
	}
	h.advance(time.Hour)
	h.push(temp(8.2))
	h.push(temp(7.9))
	if k := h.kinds(); k != "" {
		t.Fatalf("within the hysteresis: %s", k)
	}
	h.push(temp(8.6))
	h.push(temp(7.0))
	if k := h.kinds(); k != KindFrost {
		t.Fatalf("after rising above: %q", k)
	}
}

// One notification per kind and device per half hour.
func TestCoalescing(t *testing.T) {
	h := newHarness(t, user(1, `{}`))
	low := func(on bool) Event {
		return Event{Kind: "state", Cube: "KEQ0000100", Device: "0a0103", State: &State{BatteryLow: on}}
	}
	for range 3 {
		h.push(low(true))
		h.push(low(false))
		h.advance(5 * time.Minute)
	}
	if k := h.kinds(); k != KindBatteryLow {
		t.Fatalf("within the half hour: %q", k)
	}
	h.advance(30 * time.Minute)
	h.push(low(true))
	if k := h.kinds(); k != KindBatteryLow {
		t.Fatalf("after it: %q", k)
	}
}

// Each user's rules run with the user's own preference: a kind off is
// not told, and the delays are each user's.
func TestEachUsersPreference(t *testing.T) {
	h := newHarness(t, user(1, `{"window_open": {"on": true, "after_min": 5}}`), user(2, `{"window_open": {"on": false}, "battery_low": {"on": false}}`), user(3, `{}`))
	h.push(state(true))
	h.push(Event{Kind: "state", Cube: "KEQ0000100", Device: "0a0103", State: &State{BatteryLow: true}})
	h.advance(5 * time.Minute)
	h.advance(25 * time.Minute)
	var got []string
	for _, d := range h.take() {
		got = append(got, d.n.Kind+"@"+string(rune('0'+d.user)))
	}
	if strings.Join(got, " ") != "battery_low@1 battery_low@3 window_open@1 window_open@3" {
		t.Fatalf("delivered %v", got)
	}
}

// A preference that turns a kind off stops its delay from being told,
// and a user who no longer has a subscription is told nothing more.
func TestRefresh(t *testing.T) {
	h := newHarness(t, user(1, `{}`), user(2, `{}`))
	h.push(state(true))
	h.mu.Lock()
	h.users = []User{user(1, `{"window_open": {"on": false}}`)}
	h.mu.Unlock()
	h.sink.Refresh()
	h.advance(time.Hour)
	if k := h.kinds(); k != "" {
		t.Fatalf("after the refresh: %s", k)
	}
}

// The stale scan, pairing, a found Cube and a new release: each told
// once, where the user turned it on.
func TestOtherKinds(t *testing.T) {
	h := newHarness(t, user(1, `{"device_stale": {"on": true}, "paired": {"on": true}, "cube_found": {"on": true}, "update": {"on": true}}`), user(2, `{}`))
	h.mu.Lock()
	h.devices = []Device{{Cube: "KEQ0000100", Device: "0a0101", Stale: true}, {Cube: "KEQ0000100", Device: "0a0102"}}
	h.mu.Unlock()
	h.advance(scanEvery)
	h.advance(scanEvery) // still stale: not again
	h.push(Event{Kind: "paired", Cube: "KEQ0000100", Device: "0a0104", Values: map[string]any{"type": "radiator_thermostat"}})
	h.push(Event{Kind: "cube_found", Cube: "KEQ0000200", Values: map[string]any{"address": "192.0.2.20"}})
	h.push(Event{Kind: "update", Values: map[string]any{"version": "v0.13.0"}})
	h.push(Event{Kind: "update", Values: map[string]any{"version": "v0.13.0"}})
	got := h.take()
	var kinds []string
	for _, d := range got {
		if d.user != 1 {
			t.Fatalf("told user %d: %+v", d.user, d.n)
		}
		kinds = append(kinds, d.n.Kind)
	}
	if strings.Join(kinds, ",") != "device_stale,paired,cube_found,update" || got[1].n.Values["type"] != "radiator_thermostat" || got[3].n.Values["version"] != "v0.13.0" {
		t.Fatalf("delivered %+v", got)
	}
}
