// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The automatic update at night (AP AutoUpdate v1.0, Block 3): the
// window, the settings, the night's one try while the radio is quiet,
// and what holds it back. The ticks are given their times; nothing
// sleeps.

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/update"
)

func TestParseWindow(t *testing.T) {
	t.Parallel()
	for s, want := range map[string][2]int{
		"03:00-05:00": {180, 300},
		"23:30-01:30": {1410, 90},
		"00:00-00:30": {0, 30},
		"23:45-00:15": {1425, 15},
		"05:00-03:00": {300, 180}, // 22 hours, past midnight
	} {
		if start, end, err := parseWindow(s); err != nil || start != want[0] || end != want[1] {
			t.Errorf("%s: %d, %d, %v", s, start, end, err)
		}
	}
	for _, s := range []string{"3:00-5:00", "03:00-03:29", "23:50-00:10", "03:00-03:00", "24:00-01:00", "03:60-05:00", "03:00", "", "03:00-05:00 "} {
		if _, _, err := parseWindow(s); err == nil {
			t.Errorf("%q taken", s)
		}
	}
}

// The window around a moment: the one it is in, its end on the next day
// past midnight, or the last one that opened, the next opening after it;
// its start belongs to it, its end does not.
func TestWindowAround(t *testing.T) {
	t.Parallel()
	at := func(d, h, m int) time.Time { return time.Date(2026, 11, d, h, m, 0, 0, time.UTC) }
	for _, c := range []struct {
		now        time.Time
		start, end int
		from, to   time.Time
		in         bool
		next       time.Time
	}{
		{at(25, 0, 30), 1410, 90, at(24, 23, 30), at(25, 1, 30), true, at(25, 0, 30)},
		{at(25, 23, 30), 1410, 90, at(25, 23, 30), at(26, 1, 30), true, at(25, 23, 30)},
		{at(25, 1, 30), 1410, 90, at(24, 23, 30), at(25, 1, 30), false, at(25, 23, 30)},
		{at(25, 12, 0), 1410, 90, at(24, 23, 30), at(25, 1, 30), false, at(25, 23, 30)},
		{at(25, 2, 59), 180, 300, at(24, 3, 0), at(24, 5, 0), false, at(25, 3, 0)},
		{at(25, 3, 0), 180, 300, at(25, 3, 0), at(25, 5, 0), true, at(25, 3, 0)},
		{at(25, 5, 0), 180, 300, at(25, 3, 0), at(25, 5, 0), false, at(26, 3, 0)},
	} {
		from, to := windowAround(c.now, c.start, c.end)
		n := night{from: from, to: to}
		if !from.Equal(c.from) || !to.Equal(c.to) || n.in(c.now) != c.in || !n.next(c.now).Equal(c.next) {
			t.Errorf("%v in %d-%d: %v to %v, in %v, next %v", c.now, c.start, c.end, from, to, n.in(c.now), n.next(c.now))
		}
	}
}

// The two settings through PUT /api/v1/settings: off and the default
// window unless set, night and a window past midnight taken, null back
// to the default; another mode or a window that is too short or not
// HH:MM-HH:MM refused.
func TestUpdateAutoSettings(t *testing.T) {
	t.Parallel()
	ts := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret"))).ts
	settings := func() map[string]settingView {
		t.Helper()
		var out map[string]settingView
		if code := getJSON(t, ts, "secret", "/api/v1/settings", &out); code != 200 {
			t.Fatalf("GET: %d", code)
		}
		return out
	}
	if got := settings(); got[store.UpdateAutoKey] != (settingView{Value: "off", Default: "off", InEffect: "off", Applies: "now"}) ||
		got[store.UpdateWindowKey] != (settingView{Value: "03:00-05:00", Default: "03:00-05:00", InEffect: "03:00-05:00", Applies: "now"}) {
		t.Fatalf("the defaults: %+v %+v", got[store.UpdateAutoKey], got[store.UpdateWindowKey])
	}
	if code, text := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", `{"update.auto": "night", "update.auto_window": "23:30-01:30"}`); code != 200 {
		t.Fatalf("PUT: %d %s", code, text)
	}
	if got := settings(); got[store.UpdateAutoKey].Value != "night" || !got[store.UpdateAutoKey].Set || got[store.UpdateWindowKey].InEffect != "23:30-01:30" {
		t.Fatalf("set: %+v %+v", got[store.UpdateAutoKey], got[store.UpdateWindowKey])
	}
	for _, bad := range []string{`{"update.auto": "always"}`, `{"update.auto": true}`, `{"update.auto_window": "03:00-03:15"}`, `{"update.auto_window": "3-5"}`, `{"update.auto_window": 3}`} {
		if code, text := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", bad); code != 400 {
			t.Errorf("%s: %d %s", bad, code, text)
		}
	}
	if code, text := callAPI(t, ts, "PUT", "/api/v1/settings", "secret", `{"update.auto": null, "update.auto_window": null}`); code != 200 {
		t.Fatalf("PUT null: %d %s", code, text)
	}
	if got := settings(); got[store.UpdateAutoKey].Set || got[store.UpdateWindowKey].Value != "03:00-05:00" {
		t.Fatalf("back to the defaults: %+v %+v", got[store.UpdateAutoKey], got[store.UpdateWindowKey])
	}
}

// nightBox is a box for the automatic update: over st, with the owner's
// key, v0.1.0-alpha.9 on alpha finding f's release, the helper started
// by the server itself as on Windows (starts counts its starts, start's
// error, where set, fails them), the Cube KEQ0000100 not connected; the
// update at night in the window 03:00-05:00.
type nightBox struct {
	s       *Server
	status  string
	request string
	starts  *atomic.Int32
	logs    *testkit.Watch[testkit.Record]
}

func newNightBox(t *testing.T, st *store.Store, f ReleaseFinder, startErr error) nightBox {
	t.Helper()
	dir := t.TempDir()
	b := nightBox{status: filepath.Join(dir, "status.json"), request: filepath.Join(dir, "update-request"), starts: &atomic.Int32{}}
	var logger *slog.Logger
	logger, b.logs = testkit.LogWatch(nil)
	b.s = New(Options{Store: st, Token: "secret", Logger: logger, Cubes: []CubeConfig{{Serial: "KEQ0000100", Addr: "127.0.0.1:9"}},
		Update: UpdateOptions{Version: "v0.1.0-alpha.9", Channel: "alpha", Request: b.request, Status: b.status, Finder: f,
			Start: func() error { b.starts.Add(1); return startErr }}})
	ctx := t.Context()
	if err := st.SetSetting(ctx, store.UpdateAutoKey, store.UpdateAutoNight); err != nil {
		t.Fatal(err)
	}
	return b
}

// night is 2026-11-25 at h:m in UTC, the zone the ticks are given in,
// and so the window's.
func nightAt(h, m int) time.Time { return time.Date(2026, 11, 25, h, m, 0, 0, time.UTC) }

// tick runs one tick of the update loop at now and reports the helper's
// starts so far.
func (b nightBox) tick(t *testing.T, now time.Time) int {
	t.Helper()
	b.s.updateTick(t.Context(), now)
	return int(b.starts.Load())
}

// At night the box installs the release itself inside the window, once:
// nothing before the window opens, a start at its opening that starts
// the helper as on Windows, asks for the channel, and is the history's
// automatic entry and the log's; nothing more that night, also after a
// restart of the server; the next night again.
func TestNightInstall(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	f := &fakeFinder{tag: "v0.1.0-alpha.10"}
	b := newNightBox(t, st, f, nil)
	if n := b.tick(t, nightAt(2, 59)); n != 0 {
		t.Fatalf("%d starts before the window", n)
	}
	if n := b.tick(t, nightAt(3, 0)); n != 1 {
		t.Fatalf("%d starts at the window's opening", n)
	}
	if c, ok, err := update.TakeRequest(b.request); err != nil || !ok || c != "alpha" {
		t.Fatalf("the request: %q %v %v", c, ok, err)
	}
	list := b.s.updateHistory(t.Context())
	if len(list) != 1 || !list[0].Auto || list[0].By != nil || list[0].To != "v0.1.0-alpha.10" || !list[0].waiting() {
		t.Fatalf("the history: %+v", list)
	}
	if got := b.logs.Since(0, testkit.Msg("update requested", "kind", "update", "auto", "true", "to", "v0.1.0-alpha.10")); len(got) != 1 {
		t.Fatalf("the log's line: %v", b.logs.Since(0, testkit.Msg("update requested")))
	}
	if n := b.tick(t, nightAt(3, 1)); n != 1 {
		t.Fatalf("%d starts in the same night", n)
	}
	// the update restarts the server; the new one tries nothing more
	again := newNightBox(t, st, f, nil)
	if n := again.tick(t, nightAt(3, 30)); n != 0 {
		t.Fatalf("%d starts after the restart", n)
	}
	if n := again.tick(t, nightAt(3, 0).AddDate(0, 0, 1)); n != 1 {
		t.Fatalf("%d starts the next night", n)
	}
}

// GET /api/v1/update says the automatic update: off with the default
// window; at night with the window, and when the box will try where an
// update is due.
func TestNightInTheAnswer(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	f := &fakeFinder{tag: "v0.1.0-alpha.10"}
	b := newNightBox(t, st, f, nil)
	v, _ := b.s.updateRelease(t.Context(), false)
	if v.Auto == nil || v.Auto.Mode != "night" || v.Auto.Window != "03:00-05:00" || v.Auto.Next == nil {
		t.Fatalf("at night: %+v", v.Auto)
	}
	if next := v.Auto.Next.In(time.Local); next.Before(time.Now().Add(-time.Minute)) || next.After(time.Now().Add(24*time.Hour)) {
		t.Fatalf("the next try %v", next)
	}
	f.tag = "v0.1.0-alpha.9"
	if v, _ := b.s.updateRelease(t.Context(), true); v.Auto == nil || v.Auto.Next != nil {
		t.Fatalf("nothing due: %+v", v.Auto)
	}
	if err := st.SetSetting(t.Context(), store.UpdateAutoKey, store.UpdateAutoOff); err != nil {
		t.Fatal(err)
	}
	if v, _ := b.s.updateRelease(t.Context(), false); v.Auto == nil || v.Auto.Mode != "off" || v.Auto.Next != nil {
		t.Fatalf("off: %+v", v.Auto)
	}
}

// Each radio reason holds the night's update back: a command with a
// radio line waiting for the Cube, a write within its deadline, a
// restore, pairing; once the radio is quiet it runs. Where the radio
// stays busy until the window closes, the log says the update was
// postponed, once.
func TestNightWaitsForTheRadio(t *testing.T) {
	t.Parallel()
	reasons := map[string]func(rt *runtime) func(){
		"queue": func(rt *runtime) func() {
			// the queue is not running: the command waits in it
			tk := rt.queue.Enqueue(engine.Command{Key: "night-test", Lines: []cube.Message{{Type: 's', Data: "AAAAAAAAAAA="}}})
			return func() { rt.queue.Withdraw(tk, errors.New("the test is done with it")) }
		},
		"confirm": func(rt *runtime) func() {
			rt.confirms.mu.Lock()
			rt.confirms.writes = append(rt.confirms.writes, &confirmWrite{what: writeSetpoint, due: time.Now().Add(time.Hour)})
			rt.confirms.mu.Unlock()
			return func() {
				rt.confirms.mu.Lock()
				rt.confirms.writes = nil
				rt.confirms.mu.Unlock()
			}
		},
		"restore": func(rt *runtime) func() {
			rt.restoreMu.Lock()
			rt.restore = &restoreRun{State: "running"}
			rt.restoreMu.Unlock()
			return func() {
				rt.restoreMu.Lock()
				rt.restore.State = "done"
				rt.restoreMu.Unlock()
			}
		},
		"pairing": func(rt *runtime) func() {
			if err := rt.client.StartInclusion(time.Hour); err != nil {
				t.Fatal(err)
			}
			return func() { rt.client.EndInclusion() }
		},
	}
	for name, busy := range reasons {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newNightBox(t, testStore(t, ""), &fakeFinder{tag: "v0.1.0-alpha.10"}, nil)
			rt := b.s.runtimes()[0]
			quiet := busy(rt)
			if why := b.s.radioBusy(); why == "" {
				t.Fatal("the radio reads quiet")
			}
			if n := b.tick(t, nightAt(3, 0)); n != 0 {
				t.Fatalf("%d starts while the radio is busy", n)
			}
			if n := b.tick(t, nightAt(4, 59)); n != 0 {
				t.Fatalf("%d starts while the radio is busy", n)
			}
			postponed := testkit.Msg("automatic update postponed: the radio was busy until the window closed", "kind", "update_postponed", "version", "v0.1.0-alpha.10")
			if n := b.tick(t, nightAt(5, 0)); n != 0 || len(b.logs.Since(0, postponed)) != 1 {
				t.Fatalf("at the window's close: %d starts, %v", n, b.logs.Since(0, testkit.Msg("automatic update postponed: the radio was busy until the window closed")))
			}
			b.tick(t, nightAt(5, 1))
			if got := b.logs.Since(0, postponed); len(got) != 1 {
				t.Fatalf("postponed told %d times", len(got))
			}
			next := nightAt(3, 0).AddDate(0, 0, 1)
			if n := b.tick(t, next); n != 0 {
				t.Fatalf("%d starts the next night, still busy", n)
			}
			quiet()
			if why := b.s.radioBusy(); why != "" {
				t.Fatalf("the radio still busy: %s", why)
			}
			if n := b.tick(t, next.Add(time.Minute)); n != 1 {
				t.Fatalf("%d starts once the radio is quiet", n)
			}
		})
	}
}

// A command without a radio line in the queue leaves the radio quiet.
func TestRadioBusyPlainCommand(t *testing.T) {
	t.Parallel()
	b := newNightBox(t, testStore(t, ""), &fakeFinder{tag: "v0.1.0-alpha.10"}, nil)
	rt := b.s.runtimes()[0]
	tk := rt.queue.Enqueue(engine.Command{Key: "night-test", Lines: []cube.Message{{Type: 'l'}}})
	t.Cleanup(func() { rt.queue.Withdraw(tk, errors.New("the test is done with it")) })
	if why := b.s.radioBusy(); why != "" {
		t.Fatalf("a plain command makes the radio busy: %s", why)
	}
}

// The night tries nothing that POST would refuse or that is not due: a
// version held back, a box that cannot start updates (blocked), a
// helper at work, nothing newer; nor with the setting off.
func TestNightRespectsWhatHoldsBack(t *testing.T) {
	t.Parallel()
	for name, prep := range map[string]func(b nightBox){
		"held":    func(b nightBox) { b.s.holdVersion("v0.1.0-alpha.10", time.Now().Add(time.Hour)) },
		"blocked": func(b nightBox) { b.s.opts.Update.Blocked = "the service runs in a job" },
		"running": func(b nightBox) {
			update.WriteStatus(b.status, update.Status{State: "installing", From: "v0.1.0-alpha.9", Time: time.Now()})
		},
		"nothing newer": func(b nightBox) { b.s.opts.Update.Finder = &fakeFinder{tag: "v0.1.0-alpha.9"} },
		"off": func(b nightBox) {
			b.s.opts.Store.SetSetting(context.Background(), store.UpdateAutoKey, store.UpdateAutoOff)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newNightBox(t, testStore(t, ""), &fakeFinder{tag: "v0.1.0-alpha.10"}, nil)
			prep(b)
			for _, at := range []time.Time{nightAt(3, 0), nightAt(3, 30), nightAt(4, 59)} {
				if n := b.tick(t, at); n != 0 {
					t.Fatalf("%d starts at %v", n, at)
				}
			}
			if _, err := os.Stat(b.request); err == nil {
				t.Fatal("a request was written")
			}
		})
	}
}

// A start that fails ends the night: no second try that night, also
// after a restart, and a try the next night.
func TestNightFailedStart(t *testing.T) {
	t.Parallel()
	st := testStore(t, "")
	f := &fakeFinder{tag: "v0.1.0-alpha.10"}
	b := newNightBox(t, st, f, errors.New("access denied"))
	if n := b.tick(t, nightAt(3, 0)); n != 1 {
		t.Fatalf("%d starts", n)
	}
	if n := b.tick(t, nightAt(3, 31)); n != 1 {
		t.Fatalf("%d starts after the failed one", n)
	}
	if list := b.s.updateHistory(t.Context()); len(list) != 0 {
		t.Fatalf("a failed start in the history: %+v", list)
	}
	again := newNightBox(t, st, f, errors.New("access denied"))
	if n := again.tick(t, nightAt(4, 0)); n != 0 {
		t.Fatalf("%d starts after a restart the same night", n)
	}
	if n := again.tick(t, nightAt(3, 0).AddDate(0, 0, 1)); n != 1 {
		t.Fatalf("%d starts the next night", n)
	}
}

// Inside the window the release is checked afresh at the opening and
// every half hour while nothing was tried, so that a release built in
// the night is installed the same night.
func TestNightChecksAfresh(t *testing.T) {
	t.Parallel()
	f := &fakeFinder{tag: "v0.1.0-alpha.9"}
	b := newNightBox(t, testStore(t, ""), f, nil)
	b.tick(t, nightAt(2, 0)) // the loop's own first check
	checks := f.calls
	b.tick(t, nightAt(3, 0))
	if f.calls != checks+1 {
		t.Fatalf("%d checks at the opening", f.calls-checks)
	}
	b.tick(t, nightAt(3, 29))
	if f.calls != checks+1 {
		t.Fatalf("%d checks within half an hour", f.calls-checks)
	}
	f.tag = "v0.1.0-alpha.10" // the nightly is out
	if n := b.tick(t, nightAt(3, 30)); n != 1 || f.calls < checks+2 {
		t.Fatalf("half an hour on: %d checks, %d starts", f.calls-checks, n)
	}
}
