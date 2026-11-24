// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Automatic updates at night (AP AutoUpdate D1, D2, D6, D7): with the
// setting update.auto at night, the server installs an available release
// itself inside the window update.auto_window, on the server's clock,
// while no command waits for the radio on any Cube, once a night. It
// checks afresh at the window's start and every half hour inside it
// while it has tried nothing, so that a nightly built in the night is
// taken the same night. A window that closes while the radio held a due
// update back says so in the log (update_postponed). The try is
// beginUpdate's, as POST /api/v1/update's, but nobody's and marked auto
// in the history and the log; on Windows it starts the helper as POST
// does. Nothing runs where POST would be refused: an open server, no
// helper, a package manager without one, a version held back.

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"time"

	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
)

// nightCheckEvery is how often the release is checked afresh inside the
// window while nothing was tried.
const nightCheckEvery = 30 * time.Minute

// minWindowMinutes is the shortest window of an automatic update.
const minWindowMinutes = 30

// autoTriedKey keeps when the server last tried an automatic update, so
// that a night has one try across the restart an update makes, and
// across any other; written before the try, a failed one counts.
const autoTriedKey = "update.auto_tried"

// windowRe is a window as the setting holds it.
var windowRe = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])-([01][0-9]|2[0-3]):([0-5][0-9])$`)

// parseWindow reads a window of an automatic update, HH:MM-HH:MM, as
// minutes of the day: at least minWindowMinutes long, its end on the
// next day where it is not after its start (23:30-01:30).
func parseWindow(s string) (start, end int, err error) {
	m := windowRe.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, fmt.Errorf("%q: want a window HH:MM-HH:MM in the server's time, such as %s", s, store.DefaultUpdateWindow)
	}
	n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
	start, end = n(1)*60+n(2), n(3)*60+n(4)
	if (end-start+24*60)%(24*60) < minWindowMinutes {
		return 0, 0, fmt.Errorf("%q: the window is shorter than %d minutes", s, minWindowMinutes)
	}
	return start, end, nil
}

// night is the automatic update at a moment: on or off, its window as
// the setting names it, and the window around the moment (windowAround).
type night struct {
	on       bool
	window   string
	from, to time.Time
}

// in reports whether t is inside the window.
func (n night) in(t time.Time) bool { return !t.Before(n.from) && t.Before(n.to) }

// next is when the window opens next after t, or t inside it.
func (n night) next(t time.Time) time.Time {
	if n.in(t) {
		return t
	}
	y, m, d := n.from.Date()
	return time.Date(y, m, d+1, n.from.Hour(), n.from.Minute(), 0, 0, n.from.Location())
}

// windowAround is the window that holds now, or else the last one that
// opened before it: its start and end on the clock of now's zone, the
// server's, its end on the next day where it is past midnight.
func windowAround(now time.Time, start, end int) (from, to time.Time) {
	y, m, d := now.Date()
	loc := now.Location()
	from = time.Date(y, m, d, start/60, start%60, 0, 0, loc)
	if from.After(now) {
		from = time.Date(y, m, d-1, start/60, start%60, 0, 0, loc)
	}
	fy, fm, fd := from.Date()
	if end <= start {
		fd++
	}
	return from, time.Date(fy, fm, fd, end/60, end%60, 0, 0, loc)
}

// nightAt reads the settings of the automatic update at now; a window
// that cannot be read is the default.
func (s *Server) nightAt(ctx context.Context, now time.Time) night {
	n := night{window: store.DefaultUpdateWindow}
	st := s.opts.Store
	if st == nil {
		return n
	}
	mode, err := st.SettingString(ctx, store.UpdateAutoKey)
	if err != nil {
		s.log.Warn("automatic update: the setting not read", "error", err)
	}
	n.on = mode == store.UpdateAutoNight
	if w, err := st.SettingString(ctx, store.UpdateWindowKey); err == nil && w != "" {
		n.window = w
	}
	start, end, err := parseWindow(n.window)
	if err != nil {
		n.window = store.DefaultUpdateWindow
		start, end, _ = parseWindow(n.window)
	}
	n.from, n.to = windowAround(now, start, end)
	return n
}

// triedSince reports whether the server tried an automatic update at or
// after from (autoTriedKey).
func (s *Server) triedSince(ctx context.Context, from time.Time) bool {
	v, err := s.opts.Store.SettingString(ctx, autoTriedKey)
	if err != nil || v == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339Nano, v)
	return err == nil && !t.Before(from)
}

// skippedVersion reports whether an answer of the owner's holds version
// back from the night (AP AutoUpdate D3, Skip this version); none does
// yet.
func (s *Server) skippedVersion(context.Context, string) bool { return false }

// radioBusy says why the radio is not quiet, "" where it is (D6-A): on a
// Cube a command with a radio line waits in the queue or runs (the
// proxy's clients' among them), a write of Massimo's waits for its
// devices within its deadline (its re-send would be lost to the
// restart), a restore runs, or pairing is on. The Cube's own slots in
// use are left out: it keeps them across the server's restart, and a
// device out of reach can hold one for hours. A page open in a browser
// holds nothing back.
func (s *Server) radioBusy() string {
	radio := func(c engine.Command) bool {
		return slices.ContainsFunc(c.Lines, func(l cube.Message) bool { return l.Type == 's' || l.Type == 'r' })
	}
	for _, rt := range s.runtimes() {
		name := rt.cubeSerial()
		if name == "" {
			name = "a Cube"
		}
		switch _, writing := rt.confirms.nextDue(); {
		case rt.queue.Pending(radio):
			return name + ": a radio command waits"
		case writing:
			return name + ": a write waits for its devices to show it"
		case rt.restoring() != nil:
			return name + ": a restore runs"
		case rt.client.Inclusion().Active:
			return name + ": pairing is on"
		}
	}
	return ""
}

// autoState is the night's state in memory, the update loop's alone.
type autoState struct {
	window  time.Time // the start of the window it is for
	checked time.Time // the last fresh check inside that window
	// busy is why the radio held a due update back the last time, and
	// version the update; told is set once update_postponed said so.
	busy, version string
	told          bool
}

// nightTick runs the automatic update at a tick of the update loop
// (updateTick), after its own check: a fresh check at the window's
// start and every nightCheckEvery inside it, then the night's one try
// where an update is due and the radio is quiet. A window that closed
// while the radio held a due update back writes update_postponed.
func (s *Server) nightTick(ctx context.Context, now time.Time, checked bool) {
	st := s.opts.Store
	if st == nil || st.ReadOnly() {
		return
	}
	n := s.nightAt(ctx, now)
	a := &s.auto
	if !a.window.Equal(n.from) {
		s.postponed(n.window) // a window left without a tick at its end
		*a = autoState{window: n.from}
	}
	if !n.on {
		return
	}
	if !n.in(now) {
		s.postponed(n.window)
		return
	}
	if s.triedSince(ctx, n.from) {
		return
	}
	switch {
	case checked:
		a.checked = now
	case a.checked.IsZero() || now.Sub(a.checked) >= nightCheckEvery:
		s.updateRelease(ctx, true)
		a.checked = now
	}
	v, _ := s.updateRelease(ctx, false)
	if !v.CanUpdate || s.skippedVersion(ctx, v.Latest) {
		return
	}
	if why := s.radioBusy(); why != "" {
		if a.busy == "" {
			s.log.Info("automatic update waits for the radio", "version", v.Latest, "why", why)
		}
		a.busy, a.version = why, v.Latest
		return
	}
	if err := st.SetSetting(ctx, autoTriedKey, now.UTC().Format(time.RFC3339Nano)); err != nil {
		s.log.Warn("automatic update: its try not kept, so none is made", "error", err)
		return
	}
	a.busy = ""
	if _, err := s.beginUpdate(ctx, 0, true); err != nil {
		s.log.Warn("automatic update not started; the next night tries again", "version", v.Latest, "error", err)
	}
}

// postponed writes update_postponed, once, where the radio held the
// window's due update back and it closed without a try.
func (s *Server) postponed(window string) {
	a := &s.auto
	if a.busy == "" || a.told {
		return
	}
	a.told = true
	s.logServer(systemEvent("update_postponed", 0, map[string]any{"version": a.version, "why": a.busy, "window": window}))
}

// autoView is the automatic update in GET /api/v1/update.
type autoView struct {
	Mode   string `json:"mode"`   // off or night
	Window string `json:"window"` // HH:MM-HH:MM, the server's time
	// Next is when the box will try: inside the window now, else at its
	// next opening; absent while no update is due.
	Next *time.Time `json:"next,omitempty"`
}

// autoFor is the automatic update as v, the update's state, makes it;
// nil without a store.
func (s *Server) autoFor(ctx context.Context, v updateView) *autoView {
	if s.opts.Store == nil {
		return nil
	}
	now := time.Now()
	n := s.nightAt(ctx, now)
	out := &autoView{Mode: store.UpdateAutoOff, Window: n.window}
	if !n.on {
		return out
	}
	out.Mode = store.UpdateAutoNight
	if !v.CanUpdate || s.opts.Store.ReadOnly() || s.skippedVersion(ctx, v.Latest) {
		return out
	}
	next := n.next(now)
	if n.in(now) && s.triedSince(ctx, n.from) {
		y, m, d := n.from.Date()
		next = time.Date(y, m, d+1, n.from.Hour(), n.from.Minute(), 0, 0, n.from.Location())
	}
	out.Next = &next
	return out
}
