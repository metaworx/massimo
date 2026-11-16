// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The re-send of an unconfirmed room command (AP RadioHealth v1.3,
// Block 8; D162-A's first tier, D199-A). A room setpoint of the API's
// that a member of the room does not show at its confirmation's
// deadline (confirm.go) is sent once more, the same room command,
// through the queue at low priority and behind its duty and slot holds;
// the queue resets the room's flagged members before it (Block 2), as a
// member whose communication error is set may have been held back. The
// re-send's own write is confirmed as the first was: shown by every
// member by its deadline, an Info says so; else a Warn names the members
// that still do not show it and what each shows. Nothing more is sent
// for it.
//
// It is not sent where a newer setpoint to the room or to one of its
// devices has been asked for since (superseded); where a member that
// showed it shows something else now, a person's change passed on in
// the room or its window open, which D203-A leaves alone (changed);
// while a command of Massimo's to the room waits or runs in the queue
// (busy); for a boost, which ends by itself; and where the room was sent
// a state once more within the day, by this or by the room checks
// (drift.go): one lost room command, one extra write (D199-A). The first
// two are asked again once the queue's turn has come
// (engine.Command.Wanted). A LAN proxy client's write is only reported,
// as its room-list writes are left to it (D193-B), and so is a device's.
// Each outcome is a line of the unconfirmed kind.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// The outcomes of a re-send, beside the unconfirmed kind's
// not_confirmed and confirmed_late (confirm.go).
const (
	outcomeResent     = "resent"               // sent once more, at info
	outcomeNotResent  = "not_resent"           // not sent, its why, at info
	outcomeResendFail = "resend_failed"        // the queue's error, at warn
	outcomeResendOK   = "resend_confirmed"     // every member shows it, at info
	outcomeResendNot  = "resend_not_confirmed" // members still do not, at warn
)

// Why a room setpoint is not sent once more, the not_resent line's why.
const (
	whySuperseded = "superseded"
	whyChanged    = "changed"
	whyBusy       = "busy"
	whyBoost      = "boost"
	whyToday      = "today" // as the room checks say it
)

// roomSetpoint is a room setpoint as the API sent it: the room command's
// target, the member its header names, and what it set.
type roomSetpoint struct {
	target cube.Target
	st     cube.SetTemperature
}

// resendKey is the queue's key of a room's re-send.
func resendKey(room uint8) string { return fmt.Sprintf("resend:room:%d", room) }

// setpointAsked notes a setpoint asked for to t's room, or to a device
// whose C line names a room, by the API or a client of the LAN proxy: a
// newer write than a room setpoint whose re-send is decided or waits.
func (rt *runtime) setpointAsked(t cube.Target) {
	room := t.RoomID
	if !t.Room {
		room = 0
		if c := rt.client.Snapshot().Config(t.RFAddress); c != nil {
			room = c.RoomID
		}
	}
	if room == 0 {
		return
	}
	c := &rt.confirms
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.setAt == nil {
		c.setAt = map[uint8]time.Time{}
	}
	c.setAt[room] = time.Now()
}

// askedSince reports whether a setpoint to room was asked for after at.
func (c *confirmations) askedSince(room uint8, at time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.setAt[room].After(at)
}

// changedSince reports whether a device of w's that showed it at its
// deadline, any but left, shows something else now, its entry valid.
func changedSince(snap *cubelan.Snapshot, w confirmWrite, left []cube.RFAddress) bool {
	for _, rf := range w.devices {
		if slices.Contains(left, rf) {
			continue
		}
		if st, ok := snap.State(rf); ok && st.Valid && len(missing(st, ok, w.want)) > 0 {
			return true
		}
	}
	return false
}

// roomBusy reports whether a command of Massimo's own to the room waits
// or runs in the queue: one toRoom matches (drift.go), or one with an r:
// line for a device of the room; a LAN proxy client's, a reconnect and
// the re-send's own (key) left out.
func roomBusy(rt *runtime, snap *cubelan.Snapshot, room uint8, key string) bool {
	devs := map[cube.RFAddress]bool{}
	for _, rf := range snap.Devices() {
		if c := snap.Config(rf); c != nil && c.RoomID == room {
			devs[rf] = true
		}
	}
	match := toRoom(room, devs)
	return rt.queue.Pending(func(c engine.Command) bool {
		if c.Passthrough || c.Key == key {
			return false
		}
		if match(c) {
			return true
		}
		for _, l := range c.Lines {
			if l.Type != 'r' {
				continue
			}
			addrs, _ := cube.ParseResetError(l.Data)
			for _, a := range addrs {
				if devs[a] {
					return true
				}
			}
		}
		return false
	})
}

// resendUnconfirmed decides, at the deadline of r's write, a room
// setpoint of the API's that members do not show, whether to send it
// once more, and logs why not where it does not; else it takes the
// room's re-send of the day and sends it in the background.
func (s *Server) resendUnconfirmed(ctx context.Context, rt *runtime, r confirmReport) {
	room, sent := r.w.room, *r.w.sent
	key := resendKey(room)
	snap := rt.client.Snapshot()
	now := s.roomNow()
	s.seedRoomChecks(ctx, rt, now)
	why := ""
	switch {
	case sent.st.Mode == cube.ModeBoost:
		why = whyBoost
	case rt.confirms.askedSince(room, r.w.at):
		why = whySuperseded
	case changedSince(snap, r.w, r.devices):
		why = whyChanged
	case roomBusy(rt, snap, room, key):
		why = whyBusy
	}
	d := &rt.drift
	var last time.Time
	var before bool
	if why == "" {
		d.mu.Lock()
		d.initLocked()
		last, before = d.resent[room]
		if before && now.Sub(last) < resendGap {
			why = whyToday
		} else {
			d.resent[room] = now
		}
		d.mu.Unlock()
	}
	if why != "" {
		ev := s.resendEvent(r.w, outcomeNotResent, "info")
		ev.Summary["why"] = why
		rt.emit(ev)
		return
	}
	st, program := s.resendState(rt, snap, r.w, sent.st)
	m, err := cube.CmdSetTemperature(sent.target, st.Setpoint, st.Mode, st.Until)
	if err != nil { // not for a write the Cube took once
		ev := s.resendEvent(r.w, outcomeResendFail, "warn")
		ev.Error = err.Error()
		rt.emit(ev)
		return
	}
	giveBack := func() { // nothing was sent: the room's day is free again
		d.mu.Lock()
		defer d.mu.Unlock()
		if !d.resent[room].Equal(now) {
			return
		}
		if before {
			d.resent[room] = last
		} else {
			delete(d.resent, room)
		}
	}
	go s.sendOnceMore(ctx, rt, r, m, st, program, giveBack)
}

// resendState is what a re-send of w sets: st as it went, but auto with
// a temperature, which lasts until the next switch point, as auto on the
// week programme where a switch point of a member's programme has passed
// since w, at the Cube's time (D188's rule for the room checks); program
// says so.
func (s *Server) resendState(rt *runtime, snap *cubelan.Snapshot, w confirmWrite, st cube.SetTemperature) (cube.SetTemperature, bool) {
	if st.Mode != cube.ModeAuto || st.Setpoint == 0 {
		return st, false
	}
	now := time.Now()
	cubeNow, ok := cubeClock(snap, now)
	if !ok {
		return st, false
	}
	sc := scanRoom(snap, w.room, rt.hidden(), cubeNow, true)
	if !switchedSince(sc.members, cubeNow.Add(w.at.Sub(now)), cubeNow) {
		return st, false
	}
	st.Setpoint = 0
	return st, true
}

// sendOnceMore sends m, r's write once more, through the queue at low
// priority and behind its holds, and logs what became of it: dropped at
// its turn (superseded or changed by then), giveBack frees the room's
// day; sent, its own confirmation follows, st what the members are to
// show.
func (s *Server) sendOnceMore(ctx context.Context, rt *runtime, r confirmReport, m cube.Message, st cube.SetTemperature, program bool, giveBack func()) {
	room := r.w.room
	why := ""
	wanted := func() bool {
		switch {
		case rt.confirms.askedSince(room, r.w.at):
			why = whySuperseded
		case changedSince(rt.client.Snapshot(), r.w, r.devices):
			why = whyChanged
		default:
			return true
		}
		return false
	}
	res := rt.queue.Enqueue(engine.Command{Key: resendKey(room), Priority: engine.PriorityLow, Lines: []cube.Message{m}, Wanted: wanted}).Wait(ctx)
	if ctx.Err() != nil {
		return
	}
	switch {
	case errors.Is(res.Err, engine.ErrUnwanted):
		giveBack()
		ev := s.resendEvent(r.w, outcomeNotResent, "info")
		ev.Summary["why"] = why
		rt.emit(ev)
		return
	case res.Err != nil:
		ev := s.resendEvent(r.w, outcomeResendFail, "warn")
		ev.Error = res.Err.Error()
		rt.emit(ev)
		return
	}
	rt.afterWrite()
	ev := s.resendEvent(r.w, outcomeResent, "info")
	ev.Summary["duty_pct"], ev.Summary["free_slots"] = res.Budget.DutyCyclePct, res.Budget.FreeMemorySlots
	if program {
		ev.Summary["program"] = true
	}
	rt.emit(ev)
	w := &confirmWrite{what: writeSetpoint, target: r.w.target, room: room, source: r.w.source, command: r.w.command, want: setpointWant(st), resend: true}
	radiators, walls := roomThermostats(rt, room)
	if rt.beginConfirm(w, append(radiators, walls...)) == confirmConfirmed {
		rt.emit(s.resendConfirmed(report(w)))
	}
}

// resendEvent is a line of w's re-send with its outcome, at level.
func (s *Server) resendEvent(w confirmWrite, outcome, level string) event {
	summary := summaryOf(w)
	summary["outcome"] = outcome
	return event{Kind: "unconfirmed", Source: "device", Level: level, Time: time.Now(), Summary: summary, Command: s.commandRow(w.command)}
}

// resendConfirmed is the line of a re-send every device of which shows
// it by its deadline: how many, and the seconds it took.
func (s *Server) resendConfirmed(r confirmReport) event {
	ev := s.resendEvent(r.w, outcomeResendOK, "info")
	ev.Summary["confirmed"], ev.Summary["seconds"] = r.w.shown, int(math.Round(time.Since(r.w.at).Seconds()))
	return ev
}
