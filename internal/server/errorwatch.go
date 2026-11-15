// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// A flag that keeps coming back, and one that lasts (AP RadioHealth v1.2,
// Block 6, D102-B). Block 4 resets a flagged device that reports
// (autoreset.go). The Cube sets a device's communication error, byte 5's
// 0x08, only when a radiogram of its own to the device failed, so a flag
// back soon after the reset means the Cube failed again. It is back when,
// within a day of the device's last automatic reset, an entry of the
// device read after the Cube answered the reset shows the flag at a
// report of the device (still there, or again), or shows it set anew
// after it was seen clear (later that day). An entry read before the
// answer is the reset's own aftermath, not a return: the snapshot keeps
// the flag until the next device list after the r:, and the pump may
// take an older line late, so an entry counts only where it changed after
// the answer (the snapshot's StateTimes.Changed). A flag back is an
// error_watch at warn, once a day per device, with the radiogram that
// failed, and Block 4's resets leave the device alone (autoResets.hold)
// until its flag has stayed clear for a day.
//
// A flag set for 24 hours without a break, the communication error or
// the link error (byte 6's 0x40, the device's own report), is an
// error_watch at error, once in each run of the flag: a timer keyed by the
// device and the flag, set when the flag shows and stopped when it
// clears, as the notify engine's delay and cancel work
// (internal/notify/rules.go), so that a device gone silent, whose entry
// keeps the flag, gets it as well, without a job queue or a scan. The
// clear of a flag that had a warning or an error, and of a held device's
// communication error, is an error_watch at info, which settles them
// among the log's alerts (logalerts.go).
//
// What it keeps of each device is seeded once, at the runtime's first
// state event once the Cube's row in the store is known (as Block 4's
// resets are), so that a restart sets nothing back: the start of each
// flag set now from the device's state lines (the line that set it, else
// the oldest of the unbroken run the log still holds; a device that has
// had no valid state since the Cube's restart has no flag in its lines,
// and its run starts with serve), and the last warning, the hold and the
// errors given from the error_watch lines. Until then nothing is watched.

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// The error watch's times.
const (
	// flagBackWithin is how long after an automatic reset a flag seen
	// again counts as back: that day.
	flagBackWithin = 24 * time.Hour
	// flagClearFor is how long a held device's communication error has
	// to stay clear before the automatic resets take the device again.
	flagClearFor = 24 * time.Hour
	// flagLasting is how long a flag set without a break takes to be an
	// error.
	flagLasting = 24 * time.Hour
	// flagWarnGap is the least time between two warnings of one device's
	// flag back.
	flagWarnGap = 24 * time.Hour
)

// The flags the watch follows, by their names in the API's state.
const (
	flagError = "error"      // the communication error, byte 5's 0x08
	flagLink  = "link_error" // the link error, byte 6's 0x40
)

// What an error_watch line says (its summary's "what").
const (
	watchBack    = "back"    // the communication error back after an automatic reset, at warn
	watchLasting = "lasting" // a flag set for flagLasting, at error
	watchClear   = "clear"   // a flag clear again, at info
)

// flagRun is one flag of a device as the watch follows it: a run is the
// time it stays set without a break.
type flagRun struct {
	on      bool         // set at the device's newest entry
	since   time.Time    // when it showed, while on
	timer   notify.Timer // due at since+flagLasting, while on and not lasting
	seq     uint64       // the timer's: one stopped too late finds another
	lasting bool         // the run's error was given
	open    bool         // a warning or an error of the flag that no clear followed yet
}

// deviceWatch is what the watch keeps of one device.
type deviceWatch struct {
	err, link flagRun
	// held is set while the device's communication error came back after
	// an automatic reset: the resets leave it alone until the flag has
	// been clear since clearSince for flagClearFor (zero while flagged).
	held       bool
	clearSince time.Time
	warned     time.Time // its last warning of the flag back
	backAfter  time.Time // the automatic reset its flag was last found back after
	reported   time.Time // the automatic reset it last reported after
}

// run is the device's run of flag.
func (d *deviceWatch) run(flag string) *flagRun {
	if flag == flagLink {
		return &d.link
	}
	return &d.err
}

// resetAnswer is an automatic reset as the Cube answered it: its time, as
// autoResets keeps it, and when the device's entry had last changed then.
type resetAnswer struct{ at, entry time.Time }

// errorWatch is a Cube's watch of its devices' flags. The pump calls it
// at each state event, the timers when a run's day is up, Block 4's
// resets through hold and answered; its lock covers all of it.
type errorWatch struct {
	mu      sync.Mutex
	seeded  bool // the store's log was read, or there is none to read
	stopped bool // the pump has ended: no timer gives anything any more
	seq     uint64
	devices map[cube.RFAddress]*deviceWatch
	answers map[cube.RFAddress]resetAnswer
}

// hold is autoResets.hold: true while rf's communication error came back
// after an automatic reset and has not been clear for flagClearFor since,
// at at; a device clear that long is let go.
func (w *errorWatch) hold(rf cube.RFAddress, at time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	d := w.devices[rf]
	if d == nil || !d.held {
		return false
	}
	if !d.clearSince.IsZero() && at.Sub(d.clearSince) >= flagClearFor {
		d.held, d.clearSince = false, time.Time{}
		return false
	}
	return true
}

// answered keeps rf's automatic reset at at, which the Cube answered
// while the device's entry had last changed at entry.
func (w *errorWatch) answered(rf cube.RFAddress, at, entry time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.answers == nil {
		w.answers = map[cube.RFAddress]resetAnswer{}
	}
	w.answers[rf] = resetAnswer{at: at, entry: entry}
}

// forget drops a device deleted from the Cube.
func (w *errorWatch) forget(rf cube.RFAddress) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if d := w.devices[rf]; d != nil {
		stopTimer(&d.err)
		stopTimer(&d.link)
	}
	delete(w.devices, rf)
	delete(w.answers, rf)
}

// stop ends the watch with its runtime's pump: no timer gives anything.
func (w *errorWatch) stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopped = true
	for _, d := range w.devices {
		stopTimer(&d.err)
		stopTimer(&d.link)
	}
}

// stopTimer stops a run's timer.
func stopTimer(r *flagRun) {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

// entryFlags are a device's flags in its entry: the communication error,
// the Cube's own bookkeeping and so read in every entry, and the link
// error, the device's report, known only where the entry has values
// (valid, or out of date).
type entryFlags struct {
	err, link, linkKnown bool
	failed               *cube.RadiogramType
}

func flagsOf(st cube.DeviceState) entryFlags {
	return entryFlags{err: st.Error, link: st.LinkError, linkKnown: st.Valid || st.OutOfDate, failed: st.FailedRadiogram}
}

// realClock is the time of a server without Options.clock.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) AfterFunc(d time.Duration, f func()) notify.Timer { return time.AfterFunc(d, f) }

// clock is the time of the automatic resets and the error watch:
// Options.clock in the tests.
func (s *Server) clock() notify.Clock {
	if s.opts.clock != nil {
		return s.opts.clock
	}
	return realClock{}
}

// watchErrors follows rf's flags at its state event, whose changes are
// changes. Only the pump calls it, after autoReset.
func (s *Server) watchErrors(ctx context.Context, rt *runtime, rf cube.RFAddress, changes map[string]view.Change) {
	now := s.now()
	if !s.seedErrorWatch(ctx, rt, now) {
		return
	}
	snap := rt.client.Snapshot()
	st, ok := snap.State(rf)
	if !ok {
		return
	}
	var reset time.Time
	if s.seedResets(ctx, rt, now) {
		reset, _ = rt.resets.lastReset(rf)
	}
	times, _ := snap.Times(rf)
	report := reported(changes)
	f := flagsOf(st)

	w := &rt.errWatch
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return
	}
	d := w.devices[rf]
	if d == nil {
		// A device first seen since the seed, as a paired one: its flags
		// as they are, nothing to judge yet.
		d = &deviceWatch{}
		w.devices[rf] = d
		if f.err {
			s.startFlagRun(rt, w, rf, flagError, now, now)
		}
		if f.linkKnown && f.link {
			s.startFlagRun(rt, w, rf, flagLink, now, now)
		}
		return
	}

	wasOn := d.err.on
	switch {
	case f.err && !wasOn:
		if d.held && !d.clearSince.IsZero() && now.Sub(d.clearSince) >= flagClearFor {
			d.held = false
		}
		d.clearSince = time.Time{}
		s.startFlagRun(rt, w, rf, flagError, now, now)
	case !f.err && wasOn:
		s.endFlagRun(rt, w, rf, flagError, now)
		if d.held {
			d.clearSince = now
		}
	}
	if f.linkKnown {
		switch {
		case f.link && !d.link.on:
			s.startFlagRun(rt, w, rf, flagLink, now, now)
		case !f.link && d.link.on:
			s.endFlagRun(rt, w, rf, flagLink, now)
		}
	}

	// After an automatic reset: the entry read after the Cube answered
	// it, within the day.
	if reset.IsZero() || now.Sub(reset) >= flagBackWithin {
		return
	}
	if a, ok := w.answers[rf]; ok && a.at.Equal(reset) && !times.Changed.After(a.entry) {
		return // read before the answer: the reset's aftermath
	}
	next := report && !d.reported.Equal(reset)
	if report {
		d.reported = reset
	}
	if !f.err || !report && wasOn || d.backAfter.Equal(reset) {
		return
	}
	d.backAfter = reset
	d.held, d.clearSince = true, time.Time{}
	if !d.warned.IsZero() && now.Sub(d.warned) < flagWarnGap {
		return // once a day
	}
	d.warned, d.err.open = now, true
	summary := map[string]any{"what": watchBack, "flag": flagError, "reset": reset, "minutes": int(math.Round(now.Sub(reset).Minutes())),
		"next_report": next, "held": true}
	if f.failed != nil {
		summary["failed_radiogram"] = *f.failed
	}
	s.emitWatch(rt, rf, "warn", now, summary)
}

// startFlagRun starts rf's run of flag at since, seen at now, and its timer.
// w.mu is held.
func (s *Server) startFlagRun(rt *runtime, w *errorWatch, rf cube.RFAddress, flag string, since, now time.Time) {
	r := w.devices[rf].run(flag)
	stopTimer(r)
	r.on, r.since, r.lasting = true, since, false
	s.armFlagRun(rt, w, rf, flag, r, runWait(r, now))
}

// runWait is how long after now a run's day is up: none where it is
// past.
func runWait(r *flagRun, now time.Time) time.Duration {
	return max(r.since.Add(flagLasting).Sub(now), 0)
}

// armFlagRun sets the timer of a run that is not lasting yet, due after
// wait. w.mu is held.
func (s *Server) armFlagRun(rt *runtime, w *errorWatch, rf cube.RFAddress, flag string, r *flagRun, wait time.Duration) {
	stopTimer(r)
	w.seq++
	seq := w.seq
	r.seq = seq
	r.timer = s.clock().AfterFunc(wait, func() { s.flagRunDue(rt, rf, flag, seq) })
}

// endFlagRun ends rf's run of flag, cleared at now: its timer stopped, and an
// info line where a warning or an error of it is open, or the device is
// held. w.mu is held.
func (s *Server) endFlagRun(rt *runtime, w *errorWatch, rf cube.RFAddress, flag string, now time.Time) {
	d := w.devices[rf]
	r := d.run(flag)
	stopTimer(r)
	held := flag == flagError && d.held
	if r.open || held {
		summary := map[string]any{"what": watchClear, "flag": flag}
		if !r.since.IsZero() {
			summary["since"], summary["hours"] = r.since, int(math.Round(now.Sub(r.since).Hours()))
		}
		if flag == flagError {
			summary["held"] = held
		}
		s.emitWatch(rt, rf, "info", now, summary)
	}
	*r = flagRun{}
}

// flagRetry is how soon a run's day that came up while the Cube was not
// connected is looked at again.
const flagRetry = time.Minute

// flagRunDue is a run's timer: rf's flag has been set for flagLasting, an
// error, unless the run ended or was replaced since. While the Cube is
// not connected, its flags are not known: it looks again in flagRetry,
// after the next greeting has said them.
func (s *Server) flagRunDue(rt *runtime, rf cube.RFAddress, flag string, seq uint64) {
	w := &rt.errWatch
	w.mu.Lock()
	defer w.mu.Unlock()
	d := w.devices[rf]
	if w.stopped || d == nil {
		return
	}
	r := d.run(flag)
	if r.seq != seq || !r.on || r.lasting {
		return
	}
	if !rt.client.Connected() {
		s.armFlagRun(rt, w, rf, flag, r, flagRetry)
		return
	}
	r.timer, r.lasting = nil, true
	if rt.hidden()[rf] {
		return
	}
	r.open = true
	summary := map[string]any{"what": watchLasting, "flag": flag, "since": r.since, "hours": int(flagLasting / time.Hour)}
	st, ok := rt.client.Snapshot().State(rf)
	if ok && !st.Valid {
		summary["silent"] = true
	}
	if flag == flagError {
		if ok && st.FailedRadiogram != nil {
			summary["failed_radiogram"] = *st.FailedRadiogram
		}
		summary["held"] = d.held
	}
	s.emitWatch(rt, rf, "error", s.now(), summary)
}

// emitWatch logs an error_watch line of rf.
func (s *Server) emitWatch(rt *runtime, rf cube.RFAddress, level string, at time.Time, summary map[string]any) {
	snap := rt.client.Snapshot()
	ev := event{Kind: "error_watch", Source: "device", Device: rf.String(), Serial: snap.Serial(rf), Level: level, Time: at, Summary: summary}
	if _, ok := snap.State(rf); ok {
		ev.State = view.DeviceOf(snap, rf, s.staleness())
	}
	rt.emit(ev)
}

// watchMark is an error_watch line as the seed reads it.
type watchMark struct {
	at      time.Time
	what    string
	flag    string
	held    bool
	reset   time.Time
	since   time.Time
	warning bool // at warn or above
}

// seedErrorWatch reads the store's log into the watch once and takes the
// snapshot's devices as they are now, and reports whether the watch is
// seeded: true once it is, also for a Cube without a store, which has
// nothing to read; false while the Cube's row in the store is not known
// yet (it may come with the connection, learnStoreID) or the log could
// not be read.
func (s *Server) seedErrorWatch(ctx context.Context, rt *runtime, now time.Time) bool {
	w := &rt.errWatch
	w.mu.Lock()
	seeded := w.seeded
	w.mu.Unlock()
	if seeded {
		return true
	}
	id := rt.storeID.Load()
	if rt.store != nil && id == 0 {
		return false
	}
	snap := rt.client.Snapshot()
	states := map[cube.RFAddress]cube.DeviceState{}
	for _, rf := range snap.Devices() {
		if st, ok := snap.State(rf); ok {
			states[rf] = st
		}
	}
	marks := map[cube.RFAddress][]watchMark{} // newest first
	starts := map[cube.RFAddress]map[string]time.Time{}
	if rt.store != nil {
		rows, err := rt.store.Events(ctx, store.EventQuery{CubeID: id, Kind: "error_watch", Source: "device", Limit: store.MaxEventsPage})
		if err != nil {
			s.log.Warn("error watch not read from the log", "cube", rt.cubeSerial(), "error", err)
			return false
		}
		for _, r := range rows {
			rf, err := cube.ParseRFAddress(r.Device)
			var row struct {
				Summary struct {
					What  string    `json:"what"`
					Flag  string    `json:"flag"`
					Held  bool      `json:"held"`
					Reset time.Time `json:"reset"`
					Since time.Time `json:"since"`
				} `json:"summary"`
			}
			if err != nil || json.Unmarshal(r.Data, &row) != nil {
				continue
			}
			marks[rf] = append(marks[rf], watchMark{at: r.Time, what: row.Summary.What, flag: row.Summary.Flag, held: row.Summary.Held,
				reset: row.Summary.Reset, since: row.Summary.Since, warning: r.Level >= store.LevelWarn})
		}
		for rf, st := range states {
			f := flagsOf(st)
			if !f.err && !(f.linkKnown && f.link) {
				continue
			}
			rows, err := rt.store.Events(ctx, store.EventQuery{CubeID: id, Kind: "state", Device: rf.String(), Limit: flagScan})
			if err != nil {
				s.log.Warn("error watch not read from the log", "cube", rt.cubeSerial(), "error", err)
				return false
			}
			lines := make([]event, 0, len(rows))
			for i := len(rows) - 1; i >= 0; i-- { // oldest first
				var ev event
				if json.Unmarshal(rows[i].Data, &ev) == nil {
					ev.Time = rows[i].Time
					lines = append(lines, ev)
				}
			}
			starts[rf] = map[string]time.Time{}
			for _, flag := range []string{flagError, flagLink} {
				if t, ok := runStart(lines, flag); ok {
					starts[rf][flag] = t
				}
			}
		}
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.seeded || w.stopped {
		return true
	}
	if w.devices == nil {
		w.devices = map[cube.RFAddress]*deviceWatch{}
	}
	for rf, st := range states {
		d := &deviceWatch{}
		w.devices[rf] = d
		f := flagsOf(st)
		newest := map[string]*watchMark{}
		var lastClear time.Time
		lasting := map[string]time.Time{}
		for i := range marks[rf] {
			m := &marks[rf][i]
			if m.what == watchBack && d.warned.IsZero() {
				d.warned, d.backAfter = m.at, m.reset
			}
			if newest[m.flag] == nil {
				newest[m.flag] = m
			}
			if m.what == watchLasting && lasting[m.flag].IsZero() {
				lasting[m.flag] = m.at
			}
		}
		if m := newest[flagError]; m != nil {
			d.held = m.held
			if m.what == watchClear {
				lastClear = m.at
			}
		}
		for _, flag := range []string{flagError, flagLink} {
			r := d.run(flag)
			on := f.err
			if flag == flagLink {
				on = f.linkKnown && f.link
			}
			m := newest[flag]
			switch {
			case on:
				// A warning or an error of the flag that no clear followed
				// is of this run: its clear would be in the log.
				open := m != nil && m.warning
				since, ok := starts[rf][flag]
				switch {
				case ok:
				case open && !m.since.IsZero():
					since = m.since
				case open:
					since = m.at
				default:
					since = now
				}
				r.on, r.since, r.open = true, since, open
				r.lasting = open && m.what == watchLasting || !lasting[flag].IsZero() && !lasting[flag].Before(since)
				if !r.lasting {
					s.armFlagRun(rt, w, rf, flag, r, runWait(r, now))
				}
			case m != nil && m.warning && (flag == flagError || f.linkKnown):
				// It cleared while serve was down: its clear settles it.
				r.open = true
				s.endFlagRun(rt, w, rf, flag, now)
			}
		}
		if d.held {
			switch {
			case d.err.on && !lastClear.IsZero() && d.err.since.Sub(lastClear) >= flagClearFor:
				d.held = false // clear a day before it came back
			case !d.err.on && !lastClear.IsZero():
				d.clearSince = lastClear
			case !d.err.on:
				d.clearSince = now // cleared while serve was down: from now
			}
		}
	}
	w.seeded = true
	return true
}

// runStart is when flag was set in the run that the newest of a device's
// state lines (oldest first) with a state ends: the line that set it,
// else the oldest line of the unbroken run, where the lines reach no
// further back; false where that newest line does not have the flag. A
// line without values says nothing of the flag.
func runStart(lines []event, flag string) (time.Time, bool) {
	var start time.Time
	found := false
	for i := len(lines) - 1; i >= 0; i-- {
		ev := &lines[i]
		on, known := lineFlag(ev, flag)
		if !known {
			continue
		}
		if !on {
			break
		}
		start, found = ev.Time, true
		if ch, ok := ev.Changes[flag]; ok && ch.To == true {
			break
		}
	}
	return start, found
}

// lineFlag is a stored state line's flag, and whether the line has
// values to tell it by: its state, else the last known values of a
// device out of date.
func lineFlag(ev *event, flag string) (on, known bool) {
	dv, _ := ev.State.(map[string]any)
	if dv == nil {
		return false, false
	}
	for _, key := range []string{"state", "last_known"} {
		if vals, _ := dv[key].(map[string]any); vals != nil {
			on, _ = vals[flag].(bool)
			return on, true
		}
	}
	return false, false
}
