// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The room checks (AP RadioHealth v1.2, Block 9, D162-A's second and
// third tier): the members of a room that disagree, and a member that
// does not follow its week programme. A room's members are the
// thermostats whose C line names it, as the Cube goes by; hidden and
// parked devices, window contacts, members in boost and members whose
// entry is out of date are left out. A room is left out while a window
// of it is open (its thermostats then hold their own window-open
// temperature), while a structure change of Massimo's concerns it
// (structure.go), while a command to it waits or runs in the queue and
// while a write to it is open or not confirmed (confirm.go: Block 8's
// re-send, D199-A); a disagreement not yet reported starts again after
// that.
//
// Disagreement: the members differ in mode, setpoint or until, the rule
// of the virtual wall thermostat's Consistent (view.SameSetting), two
// members in auto that each follow their own programme counting as
// agreeing: a difference of their programmes is their configuration's,
// which their C lines show. No person can mean the members of a room to
// differ, since the original software and Massimo set a room's
// thermostats only together. Differing for disagreeAfter, a Warn names
// the room, the members and what differs; still differing a
// disagreeAfter later, the room's intended state is sent once more as a
// room command, taken in this order (D188, as AP RoomMajority's D4-A,
// D3-A, D7-C and D9 change it; referenceState): Massimo's last setpoint
// to the room, where it is recent and a member still shows it, auto
// again where it was auto, which takes the programme's value now; else
// the state held for the room before a structure change brought a
// newcomer into it (holdReference), in place of the state more than
// half of the members show; else the state of the member whose mode or
// setpoint changed last; else the state of the first member of the
// room's one order (roomorder.Of): its master, its wall thermostat, its
// radiator thermostats; else none, the Warn alone. The last three are
// roomorder.State, the rule the virtual wall thermostat shows a room by
// too (RoomMajority.D19-A).
// It goes through the queue at low priority, never while the queue holds
// the radio for the duty cycle or the memory slots, and at most once a
// day per room, a day shared with Block 8's re-send of an unconfirmed
// room setpoint (resend.go, D199-A), the day's re-sends of both seeded
// from the log at the first check. Agreeing again, an Info says so.
//
// Programme drift: in a room in auto, a member whose setpoint has not
// been its own programme's temperature at the Cube's time for
// driftPersist, while every other member follows its own: a Warn names
// its clock or its programme as suspect, with the radiogram that failed
// where it has the communication error, and says whether the setpoint
// is its programme's an hour off, its summer time (its state's auto_dst
// says whether the device switches summer time itself). Nothing is sent
// for it, and it is left out of the disagreement: a room command does
// not set a clock. A temperature the programme has within driftGrace of the
// Cube's time counts, so a switch point just past, or a clock a few
// minutes off, is no drift. Following again, an Info says so; the next
// Warn for the member comes a day later at the earliest.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/roomorder"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// The times of the room checks.
const (
	// roomCheckEvery is how often the checks run.
	roomCheckEvery = time.Minute
	// disagreeAfter is how long a room's members differ before the Warn,
	// and again before their room's state is sent once more.
	disagreeAfter = 10 * time.Minute
	// resendGap is the least time between two re-sends to a room.
	resendGap = 24 * time.Hour
	// intentLead is how long before a disagreement began Massimo's last
	// setpoint to the room may have been sent to count as its intended
	// state.
	intentLead = 15 * time.Minute
	// driftGrace is how far from the Cube's time a programme's
	// temperature still counts: a switch point just past, a clock a few
	// minutes off (roomorder.Grace, by which a member follows its
	// programme).
	driftGrace = roomorder.Grace
	// driftPersist is how long a member must not follow its programme
	// before the Warn.
	driftPersist = 5 * time.Minute
	// driftWarnGap is the least time between two drift Warns of a
	// member.
	driftWarnGap = 24 * time.Hour
)

// The re-sent state's origins, the summary's from, in the order they
// are taken (D188, RoomMajority.D9): Massimo's last setpoint, the
// members' majority (of those before a structure change, where one was
// held: before_change), the newest change, then the room's order; the
// last five are roomorder.State's rules.
const (
	fromMassimo  = "massimo"
	fromMajority = string(roomorder.ByMajority)
	fromNewest   = string(roomorder.ByNewest)
	fromMaster   = string(roomorder.ByMaster)
	fromWall     = string(roomorder.ByWall)
	fromRadiator = string(roomorder.ByRadiator)
)

// driftState is a runtime's room checks: each room's disagreement and
// each member's drift as the checks follow them, the last re-send per
// room, the last drift Warn per member, the last setpoint of Massimo's
// per room, and the state held per room before a structure change.
type driftState struct {
	mu      sync.Mutex
	rooms   map[uint8]*disagreement
	members map[cube.RFAddress]*driftEpisode
	resent  map[uint8]time.Time
	warned  map[cube.RFAddress]time.Time
	intent  map[uint8]roomIntent
	holds   map[uint8]*heldReference
	seeded  bool // the day's re-sends and Warns read from the log
}

// initLocked makes the maps of a runtime's first check, with d.mu held.
func (d *driftState) initLocked() {
	if d.rooms == nil {
		d.rooms, d.members, d.resent, d.warned = map[uint8]*disagreement{}, map[cube.RFAddress]*driftEpisode{}, map[uint8]time.Time{}, map[cube.RFAddress]time.Time{}
	}
}

// disagreement is a room whose members differ.
type disagreement struct {
	since    time.Time // the first check that found them differing
	reported time.Time // when the Warn went out; zero before
	resent   bool      // the second check is done
}

// driftEpisode is a member that does not follow its programme.
type driftEpisode struct {
	since    time.Time // the first check that found it so
	reported bool      // it lasted driftPersist
	warned   bool      // and its Warn went out
}

// heldReference is a room's state from before a structure change that
// brought a newcomer into it (RoomMajority, D8-A), and when it was held,
// a time of the room checks' clock.
type heldReference struct {
	state  resend
	at     time.Time
	judged bool // a check of the room has run since it was held
	// judgedAt is when the first check that judged it scanned the room,
	// a time of the snapshot's clock (cubelan.StateTimes).
	judgedAt time.Time
	resent   bool // its re-send went out, past the day's limit
}

// roomIntent is a setpoint of Massimo's to a room, and when it went.
type roomIntent struct {
	mode  cube.Mode
	temp  float64
	until cube.Until
	at    time.Time
}

// shownBy reports whether a member shows the setpoint: its mode, and its
// temperature but for auto on the programme, and its until in vacation.
// Auto with a temperature is shown also by a member on its programme,
// as after a switch point (D188).
func (in roomIntent) shownBy(m roomMember) bool {
	st := m.st
	if st.Mode != in.mode {
		return false
	}
	switch {
	case in.mode == cube.ModeAuto && (in.temp == 0 || m.follows):
		return true
	case in.mode == cube.ModeVacation:
		return *st.Setpoint == in.temp && st.Until != nil && *st.Until == in.until
	}
	return *st.Setpoint == in.temp
}

// roomNow is the room checks' clock: Options.driftClock in the tests.
func (s *Server) roomNow() time.Time {
	if s.opts.driftClock != nil {
		return s.opts.driftClock()
	}
	return time.Now()
}

// intend keeps a setpoint of Massimo's to a room that the Cube took, as
// the room's intended state for the room checks; a boost, which ends by
// itself, is none. It drops a state held for the room before it
// (holdReference), the newer wish, but for the held state itself sent
// as a room command.
func (rt *runtime) intend(room uint8, mode cube.Mode, temp float64, until cube.Until, at time.Time) {
	if room == 0 || mode == cube.ModeBoost {
		return
	}
	d := &rt.drift
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.intent == nil {
		d.intent = map[uint8]roomIntent{}
	}
	d.intent[room] = roomIntent{mode: mode, temp: temp, until: until, at: at}
	if h := d.holds[room]; h != nil && h.at.Before(at) && !h.state.is(mode, temp, until) {
		delete(d.holds, room)
	}
}

// intentAfter is Massimo's last setpoint to room where it was taken
// after at, a time of the room checks' clock; false for none.
func (rt *runtime) intentAfter(room uint8, at time.Time) (roomIntent, bool) {
	d := &rt.drift
	d.mu.Lock()
	defer d.mu.Unlock()
	in, ok := d.intent[room]
	return in, ok && at.Before(in.at)
}

// holdReference keeps state as room's reference for the room checks,
// held at at, a time of the room checks' clock (Server.roomNow): the
// state the room's members showed before a structure change that brings
// a newcomer into it (RoomMajority, D8-A), which the re-send takes in
// place of the members' majority (referenceState), marked
// before_change, once even where the room had a re-send that day. It
// is dropped at the room's first check that finds its members
// agreeing, two at least or every thermostat of the room, on any
// state; at a check that finds a member changed by hand to another
// state since the first check that judged it (wishedSince); by a newer
// setpoint of Massimo's to the room (intend); resendGap after at; once
// it is a holiday whose end has passed at the Cube's time; and by a
// later hold for the room, which replaces it. A state read before a
// setpoint of Massimo's to the room of another state, as one held only
// once a structure change's link half went through, is not held. The
// room checks leave the room out while the change is sent and read back
// (structureRooms), so that the first check after it is the first to
// judge the room. A state that is the room's hold itself, as
// roomReference gives it before a check judged the room (before_change),
// keeps the hold and its time.
func (rt *runtime) holdReference(room uint8, state resend, at time.Time) {
	if room == 0 {
		return
	}
	d := &rt.drift
	d.mu.Lock()
	defer d.mu.Unlock()
	if state.beforeChange && d.holds[room] != nil {
		return
	}
	if in, ok := d.intent[room]; ok && at.Before(in.at) && !state.is(in.mode, in.temp, in.until) {
		return // read before a newer setpoint of Massimo's to the room, the newer wish (intend)
	}
	state.room, state.beforeChange = room, false
	if d.holds == nil {
		d.holds = map[uint8]*heldReference{}
	}
	d.holds[room] = &heldReference{state: state, at: at}
}

// holdFor is the state held for room that still counts at now, with
// d.mu held: younger than resendGap, and no holiday whose end has
// passed at cubeNow, the Cube's time, where clock says it is known; a
// hold that no longer counts is dropped. Nil for none.
func (d *driftState) holdFor(room uint8, now, cubeNow time.Time, clock bool) *heldReference {
	h := d.holds[room]
	if h == nil {
		return nil
	}
	st := h.state
	if now.Sub(h.at) >= resendGap || st.mode == cube.ModeVacation && clock && !st.until.IsZero() && !st.until.Time(cubeNow.Location()).After(cubeNow) {
		delete(d.holds, room)
		return nil
	}
	return h
}

// roomReference is the state that speaks for room at now, a time of
// the room checks' clock: referenceState over the room's thermostats
// with a state as the room checks find them (scanRoom, the hidden and
// parked devices and one drifting off its programme left out), Massimo's
// setpoint to the room of the last intentLead first (D4-A); a state
// held for the room before taking its members' majority only while no
// check has judged the room since it was held, as while the room is
// left out for the structure change that held it (before_change; its
// time stays as it was, holdReference). A structure change that brings
// a newcomer into the room reads it before it links the device, and
// holds it for the room checks once its link half reached a member
// (Server.roomBefore: an add or a move of a thermostat, which gives it
// to the device too where it copies, and the repair's placement;
// holdReference; RoomMajority, D8-A, D15-A). leave, the
// device the change brings in, is left out where its C line names the
// room already, as after a retry whose link half went through; 0 for
// none. False where no thermostat of the room has a state.
func (rt *runtime) roomReference(snap *cubelan.Snapshot, room uint8, leave cube.RFAddress, now time.Time) (resend, bool) {
	md := snap.Metadata()
	if md == nil {
		return resend{}, false
	}
	cubeNow, clock := snap.CubeClock(now)
	skip := view.Hidden{}
	maps.Copy(skip, view.NewHidden(rt.config().Hidden, md, false))
	if leave != 0 {
		skip[leave] = true
	}
	sc := scanRoom(snap, room, skip, cubeNow, clock)
	d := &rt.drift
	d.mu.Lock()
	defer d.mu.Unlock()
	held := d.holdFor(room, now, cubeNow, clock)
	if held != nil && held.judged {
		held = nil
	}
	return referenceState(room, sc.master, compared(sc.members), d.intentSince(room, now), held, now, cubeNow, clock)
}

// roomCheckLoop runs the room checks of one Cube every
// Options.driftEvery until ctx ends; a negative one runs none (the tests
// call roomChecks).
func (s *Server) roomCheckLoop(ctx context.Context, rt *runtime) {
	every := s.opts.driftEvery
	if every < 0 {
		return
	}
	if every == 0 {
		every = roomCheckEvery
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		s.roomChecks(ctx, rt)
	}
}

// roomMember is a member of a room as a check compares it.
type roomMember struct {
	rf      cube.RFAddress
	typ     cube.DeviceType
	st      cube.DeviceState
	program *cube.WeekProgram
	judged  bool // in auto, with a programme and the Cube's time (roomorder.Following)
	follows bool // judged, and its setpoint is its programme's
	// modeAt and setpointAt are when its mode and its setpoint changed
	// to their values (cubelan.StateTimes), zero where that is no one's
	// change: a value's first sighting (one there at the first
	// connection, whose age is unknown, a new device's, an entry's that
	// got its values late), or the end of a boost or of a holiday at its
	// until (StateTimes.Changes).
	modeAt, setpointAt time.Time
}

// member is m as the room's state is decided over it (roomorder.State).
func (m roomMember) member() roomorder.Member {
	return roomorder.Member{RF: m.rf, Type: m.typ, State: m.st, Follows: m.follows, ModeAt: m.modeAt, SetpointAt: m.setpointAt}
}

// members are ms as the room's state is decided over them, in their
// order.
func members(ms []roomMember) []roomorder.Member {
	out := make([]roomorder.Member, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.member())
	}
	return out
}

// roomScan is what a check finds in one room.
type roomScan struct {
	members []roomMember            // the members compared
	all     map[cube.RFAddress]bool // every thermostat of the room
	window  bool                    // a window contact of the room is open
	master  cube.RFAddress          // the metadata's master of the room
	at      time.Time               // when it was read, a time of the snapshot's clock
}

// scanRoom reads one room's members from the snapshot, those in skip
// left out, judging the members in auto by the Cube's time where it is
// known (roomorder.Following). A member the Cube holds no valid state
// for is dead (roomorder.Dead) and left out, as is one in boost.
func scanRoom(snap *cubelan.Snapshot, room uint8, skip view.Hidden, cubeNow time.Time, clock bool) roomScan {
	sc := roomScan{all: map[cube.RFAddress]bool{}, master: cubeio.RoomMaster(snap, room), at: time.Now()}
	for _, rf := range snap.Devices() {
		c := snap.Config(rf)
		if c == nil || c.RoomID != room || skip[rf] {
			continue
		}
		st, ok := snap.State(rf)
		if c.Type == cube.DeviceShutterContact {
			sc.window = sc.window || ok && (st.Valid || st.OutOfDate) && st.WindowOpen != nil && *st.WindowOpen
			continue
		}
		if !c.Type.IsThermostat() {
			continue
		}
		sc.all[rf] = true
		if !ok || !st.Valid || st.Mode == cube.ModeBoost || st.Setpoint == nil {
			continue
		}
		m := roomMember{rf: rf, typ: c.Type, st: st, program: roomorder.Program(c)}
		if times, ok := snap.Times(rf); ok {
			m.modeAt, m.setpointAt = roomorder.ChangedAt(times, "mode"), roomorder.ChangedAt(times, "setpoint")
		}
		m.judged, m.follows = roomorder.Following(st, m.program, cubeNow, clock)
		sc.members = append(sc.members, m)
	}
	return sc
}

// switchedAt is when the switch point in force at t began, as "HH:MM".
func switchedAt(w *cube.WeekProgram, t time.Time) string {
	m, start := t.Hour()*60+t.Minute(), 0
	for _, p := range roomorder.ProgramDay(w, t) {
		if m < p.Until {
			break
		}
		start = p.Until
	}
	return fmt.Sprintf("%02d:%02d", start/60, start%60)
}

// drifting is the member of a room in auto whose setpoint is not its
// programme's while every other member follows its own; nil for none,
// and in a room with a member not in auto or of one member.
func drifting(ms []roomMember) *roomMember {
	if len(ms) < 2 {
		return nil
	}
	var odd *roomMember
	for i := range ms {
		switch m := &ms[i]; {
		case !m.judged:
			return nil
		case !m.follows && odd != nil:
			return nil
		case !m.follows:
			odd = m
		}
	}
	return odd
}

// wishedSince reports whether one of ms changed its mode or setpoint
// after at, a time of the snapshot's clock, by someone's hand
// (roomMember.modeAt, setpointAt), to a state other than held: a
// newer wish than a state held from before a structure change. A
// member that shows the held state, as after that state's own room
// command, is none.
func wishedSince(ms []roomMember, held resend, at time.Time) bool {
	shown := roomIntent{mode: held.mode, temp: held.temp, until: held.until}
	for _, m := range ms {
		if (m.modeAt.After(at) || m.setpointAt.After(at)) && !shown.shownBy(m) {
			return true
		}
	}
	return false
}

// compared are the members a check compares: ms but the one drifting
// off its programme while the others follow theirs (drifting).
func compared(ms []roomMember) []roomMember {
	odd := drifting(ms)
	var out []roomMember
	for _, m := range ms {
		if odd == nil || m.rf != odd.rf {
			out = append(out, m)
		}
	}
	return out
}

// membersAgree reports whether two members agree: in mode, setpoint and
// until, or both in auto, each following its own programme
// (roomorder.Agree).
func membersAgree(a, b roomMember) bool {
	return roomorder.Agree(a.member(), b.member())
}

// disagrees reports whether two of the members do not agree.
func disagrees(ms []roomMember) bool {
	for i := range ms {
		for j := i + 1; j < len(ms); j++ {
			if !membersAgree(ms[i], ms[j]) {
				return true
			}
		}
	}
	return false
}

// differing names what differs among the members: mode, setpoint,
// until.
func differing(ms []roomMember) []string {
	modes, setpoints, untils := map[cube.Mode]bool{}, map[float64]bool{}, map[string]bool{}
	for _, m := range ms {
		modes[m.st.Mode] = true
		setpoints[*m.st.Setpoint] = true
		u := ""
		if m.st.Until != nil {
			u = m.st.Until.String()
		}
		untils[u] = true
	}
	var out []string
	if len(modes) > 1 {
		out = append(out, "mode")
	}
	if len(setpoints) > 1 {
		out = append(out, "setpoint")
	}
	if len(untils) > 1 {
		out = append(out, "until")
	}
	return out
}

// memberSummary is a member's setting as a disagreement's summary names
// it.
func memberSummary(m roomMember) map[string]any {
	out := map[string]any{"rf": m.rf.String(), "mode": m.st.Mode.String(), "setpoint": *m.st.Setpoint}
	if m.st.Until != nil {
		out["until"] = m.st.Until.String()
	}
	return out
}

// resend is a room's state to send once more.
type resend struct {
	room         uint8
	mode         cube.Mode
	temp         float64 // 0 with auto: the programme
	until        cube.Until
	from         string // one of the from constants: the rule that took it
	beforeChange bool   // a state held before a structure change (holdReference)
}

// is reports whether r is the state of a setpoint of mode, temp and
// until.
func (r resend) is(mode cube.Mode, temp float64, until cube.Until) bool {
	return r.mode == mode && r.temp == temp && r.until == until
}

// summary is the re-sent state as an event names it: before_change
// marks a state held before a structure change.
func (r resend) summary(outcome string) map[string]any {
	out := map[string]any{"room": r.room, "outcome": outcome, "from": r.from, "mode": r.mode.String()}
	if r.temp != 0 {
		out["setpoint"] = r.temp
	}
	if !r.until.IsZero() {
		out["until"] = r.until.String()
	}
	if r.beforeChange {
		out["before_change"] = true
	}
	return out
}

// intended is the state to send a disagreeing room once more
// (referenceState): Massimo's last setpoint to it counts where it was
// sent since the disagreement began, or at most intentLead before;
// a state held for the room while it still counts (holdFor). now is
// the room checks' clock, cubeNow the Cube's at now.
func (d *driftState) intended(room uint8, master cube.RFAddress, since time.Time, ms []roomMember, now, cubeNow time.Time, clock bool) (resend, bool) {
	return referenceState(room, master, ms, d.intentSince(room, since), d.holdFor(room, now, cubeNow, clock), now, cubeNow, clock)
}

// intentSince is Massimo's last setpoint to room where it was sent at
// most intentLead before since, with d.mu held; nil for none.
func (d *driftState) intentSince(room uint8, since time.Time) *roomIntent {
	if in, ok := d.intent[room]; ok && !in.at.Before(since.Add(-intentLead)) {
		return &in
	}
	return nil
}

// referenceState is the state that speaks for a room, the first of
// these (RadioHealth.D188 as AP RoomMajority changes it: D4-A, D3-A,
// D7-C, D9) over ms, its thermostats with a state (scanRoom: the dead
// and those in boost left out), master the metadata's master of the
// room:
//  1. Massimo's last setpoint to it, in, while a member shows it
//     (D4-A); where it was auto, auto again on the programme, which
//     takes the programme's value now, and auto with its temperature
//     only while no switch point has passed since (switchedSince), the
//     Cube's clock known;
//  2. held, the state its members showed before a structure change
//     brought a newcomer into the room (holdReference), marked
//     before_change; where it is auto with a temperature, auto on the
//     programme once a switch point has passed since it was held; else
//     the room's state by the one order over ms (roomorder.State,
//     RoomMajority.D19-A, which the virtual wall thermostat shows too):
//     the state more than half of ms show (roomorder.Majority);
//  3. else the state of the member whose mode or setpoint changed last
//     (roomorder.Newest);
//  4. else the state of the first of the room's order among ms
//     (roomorder.ByOrder, D9): the master's where it is a thermostat of
//     the room, the wall thermostat's, the first radiator thermostat's;
//  5. else none, where ms is empty.
//
// A member that follows its programme gives auto on the programme
// (memberResend). now is the room checks' clock, cubeNow the Cube's at
// now.
func referenceState(room uint8, master cube.RFAddress, ms []roomMember, in *roomIntent, held *heldReference, now, cubeNow time.Time, clock bool) (resend, bool) {
	if in != nil {
		for _, m := range ms {
			if !in.shownBy(m) {
				continue
			}
			r := resend{room: room, mode: in.mode, temp: in.temp, until: in.until, from: fromMassimo}
			if in.mode == cube.ModeAuto && clock && switchedSince(ms, cubeNow.Add(in.at.Sub(now)), cubeNow) {
				r.temp = 0
			}
			return r, true
		}
	}
	if held != nil {
		r := held.state
		r.room, r.beforeChange = room, true
		if r.mode == cube.ModeAuto && clock && switchedSince(ms, cubeNow.Add(held.at.Sub(now)), cubeNow) {
			r.temp = 0
		}
		return r, true
	}
	if p, ok := roomorder.State(master, members(ms)); ok {
		return pickResend(room, ms, p), true
	}
	return resend{}, false
}

// pickResend is the state roomorder decided over ms, p, as a re-send:
// the member's it names, from the rule that took it.
func pickResend(room uint8, ms []roomMember, p roomorder.Pick) resend {
	return memberResend(room, ms[p.Index], string(p.Rule))
}

// memberResend is a member's state as a re-send: its mode, setpoint and
// until, auto on the programme where it follows it.
func memberResend(room uint8, m roomMember, from string) resend {
	r := resend{room: room, mode: m.st.Mode, temp: *m.st.Setpoint, from: from}
	if m.st.Until != nil {
		r.until = *m.st.Until
	}
	if m.follows {
		r.temp = 0
	}
	return r
}

// switchedSince reports whether a switch point of a member's week
// programme lies after then and up to now, both times of the Cube's
// clock: the programme's period in force differs, or the day does.
func switchedSince(ms []roomMember, then, now time.Time) bool {
	if !then.Before(now) {
		return false
	}
	for _, m := range ms {
		if m.program == nil {
			continue
		}
		if then.YearDay() != now.YearDay() || then.Year() != now.Year() || switchedAt(m.program, then) != switchedAt(m.program, now) {
			return true
		}
	}
	return false
}

// leftOut is why a check of the room checks leaves a room out, a bit
// for each reason, zero where it checks the room. roomChecks reads it
// from scanRooms alone, and so does the tests' drift rig, which waits
// until none holds before it runs a check: a reason the checks come to
// know is one the rig waits for.
type leftOut uint8

// The reasons a check leaves a room out.
const (
	leftOutStructure  leftOut = 1 << iota // a structure change is being sent (lockStructure), for every room
	leftOutWindow                         // a window contact of the room is open
	leftOutWatched                        // a structure change of Massimo's that concerns it is read back (structureRooms)
	leftOutQueued                         // a command to it waits or runs in the queue (toRoom)
	leftOutConfirming                     // a write to it is open or not confirmed (roomConfirmation)
)

// leftOutNames name the reasons, in the order of their bits.
var leftOutNames = [...]string{"structure", "window", "watched", "queued", "confirming"}

// String names the reasons, "queued+confirming", or "none": for a
// test's failure.
func (l leftOut) String() string {
	var names []string
	for i, n := range leftOutNames {
		if l&(1<<i) != 0 {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, "+")
}

// checkedRoom is a room as a check of the room checks reads it.
type checkedRoom struct {
	id   uint8
	scan roomScan
	why  leftOut // why the check leaves it out, zero where it checks it
}

// scanRooms reads every room of the Cube for a check of the room checks
// from snap and its metadata md (scanRoom), the Cube's time cubeNow
// known where clock says so, and why the check leaves each out
// (leftOut).
func (rt *runtime) scanRooms(snap *cubelan.Snapshot, md *cube.Metadata, cubeNow time.Time, clock bool) []checkedRoom {
	skip := view.NewHidden(rt.config().Hidden, md, false)
	busy := len(rt.structure) > 0 // a structure change is being sent
	watched := rt.structureRooms(snap)
	var rooms []checkedRoom
	for _, r := range md.Rooms {
		if r.ID == 0 || r.ID == cube.HiddenRoom {
			continue
		}
		sc := scanRoom(snap, r.ID, skip, cubeNow, clock)
		rooms = append(rooms, checkedRoom{id: r.ID, scan: sc, why: rt.roomLeftOut(r.ID, sc, busy, watched)})
	}
	return rooms
}

// roomLeftOut is why a check leaves room, read as sc, out: busy says
// that a structure change is being sent, watched are the rooms whose
// structure change is read back, both read once for the check.
func (rt *runtime) roomLeftOut(room uint8, sc roomScan, busy bool, watched map[uint8]bool) leftOut {
	var why leftOut
	if busy {
		why |= leftOutStructure
	}
	if sc.window {
		why |= leftOutWindow
	}
	if watched[room] {
		why |= leftOutWatched
	}
	if rt.queue.Pending(toRoom(room, sc.all)) {
		why |= leftOutQueued
	}
	if rt.roomConfirmation(room) != nil {
		why |= leftOutConfirming
	}
	return why
}

// roomChecks runs the room checks of every room of the Cube once.
func (s *Server) roomChecks(ctx context.Context, rt *runtime) {
	if !rt.client.Connected() {
		return
	}
	snap := rt.client.Snapshot()
	md := snap.Metadata()
	if md == nil {
		return
	}
	now := s.roomNow()
	s.seedRoomChecks(ctx, rt, now)
	cubeNow, clock := snap.CubeClock(now)
	radioHeld := rt.queue.Holding()
	rooms := rt.scanRooms(snap, md, cubeNow, clock)

	d := &rt.drift
	d.mu.Lock()
	d.initLocked()
	var evs []event
	var sends []resend
	seen := map[uint8]bool{}
	present := map[cube.RFAddress]bool{}
	for _, r := range rooms {
		seen[r.id] = true
		for rf := range r.scan.all {
			present[rf] = true
		}
		if r.why != 0 {
			d.pause(r.id, r.scan)
			continue
		}
		e, send := d.checkRoom(snap, r.id, r.scan, now, cubeNow, clock, radioHeld)
		evs = append(evs, e...)
		if send != nil {
			sends = append(sends, *send)
		}
	}
	for id := range d.rooms {
		if !seen[id] {
			delete(d.rooms, id)
		}
	}
	for id, h := range d.holds {
		if now.Sub(h.at) >= resendGap {
			delete(d.holds, id)
		}
	}
	for rf := range d.members {
		if !present[rf] {
			delete(d.members, rf)
		}
	}
	d.mu.Unlock()

	for _, ev := range evs {
		rt.emit(ev)
	}
	for _, r := range sends {
		s.resendRoom(ctx, rt, r)
	}
}

// pause leaves a room out of this check: a disagreement or a drift not
// yet reported starts again, one reported stays as it is.
func (d *driftState) pause(room uint8, sc roomScan) {
	if ep, ok := d.rooms[room]; ok && ep.reported.IsZero() {
		delete(d.rooms, room)
	}
	for rf := range sc.all {
		if ep, ok := d.members[rf]; ok && !ep.reported {
			delete(d.members, rf)
		}
	}
}

// checkRoom checks one room, with d.mu held: the drift of its members,
// then their disagreement. It returns the events to emit and the state
// to send once more, where the second check has come and the queue does
// not hold the radio (radioHeld); clock says whether cubeNow, the
// Cube's time, is known. A state held for the room before a structure
// change (holdReference) older than the check is judged by it: dropped
// where a compared member changed by someone's hand to another state
// since the first check that judged it (wishedSince), and where two
// members or more agree, or every thermostat of the room is compared
// and they agree, one included; its re-send goes out once even where
// the room had one that day.
func (d *driftState) checkRoom(snap *cubelan.Snapshot, id uint8, sc roomScan, now, cubeNow time.Time, clock, radioHeld bool) ([]event, *resend) {
	var evs []event
	at := time.Now()
	hold := d.holds[id]
	if hold != nil && !hold.at.Before(now) {
		hold = nil // held after this check's scan: the next one judges it
	}
	judgedBefore := hold != nil && hold.judged
	if hold != nil && !hold.judged {
		hold.judged, hold.judgedAt = true, sc.at
	}
	odd := drifting(sc.members)
	for _, m := range sc.members {
		ep := d.members[m.rf]
		switch {
		case odd != nil && m.rf == odd.rf:
			if ep == nil {
				d.members[m.rf] = &driftEpisode{since: now}
				continue
			}
			if ep.reported || now.Sub(ep.since) < driftPersist {
				continue
			}
			ep.reported = true
			if last, ok := d.warned[m.rf]; ok && now.Sub(last) < driftWarnGap {
				continue
			}
			ep.warned, d.warned[m.rf] = true, now
			evs = append(evs, driftWarn(snap, id, m, sc.members, cubeNow, at))
		case ep == nil:
		case m.follows:
			if ep.warned {
				evs = append(evs, event{Kind: "drift", Source: "device", Level: "info", Device: m.rf.String(), Serial: snap.Serial(m.rf), Time: at,
					Summary: map[string]any{"room": id, "outcome": "follows", "minutes": wholeMinutes(now.Sub(ep.since))}})
			}
			delete(d.members, m.rf)
		case m.st.Mode != cube.ModeAuto:
			if ep.warned {
				evs = append(evs, event{Kind: "drift", Source: "device", Level: "info", Device: m.rf.String(), Serial: snap.Serial(m.rf), Time: at,
					Summary: map[string]any{"room": id, "outcome": "left_auto", "mode": m.st.Mode.String()}})
			}
			delete(d.members, m.rf)
		case !ep.reported:
			delete(d.members, m.rf) // not following, but not the room's one odd member now
		}
	}

	cmp := compared(sc.members)
	if judgedBefore && wishedSince(cmp, hold.state, hold.judgedAt) {
		delete(d.holds, id) // a newer wish than the state from before the change
		hold = nil
	}
	ep := d.rooms[id]
	if !disagrees(cmp) {
		if hold != nil && (len(cmp) > 1 || len(cmp) == len(sc.all)) {
			delete(d.holds, id) // they agree: the state from before a change is done
		}
		if ep != nil && !ep.reported.IsZero() {
			evs = append(evs, event{Kind: "disagree", Source: "cube", Level: "info", Time: at,
				Summary: map[string]any{"room": id, "outcome": "agree", "minutes": wholeMinutes(now.Sub(ep.since))}})
		}
		delete(d.rooms, id)
		return evs, nil
	}
	if ep == nil {
		d.rooms[id] = &disagreement{since: now}
		return evs, nil
	}
	switch {
	case ep.reported.IsZero():
		if now.Sub(ep.since) < disagreeAfter {
			return evs, nil
		}
		ep.reported = now
		var members []map[string]any
		for _, m := range cmp {
			members = append(members, memberSummary(m))
		}
		evs = append(evs, event{Kind: "disagree", Source: "cube", Level: "warn", Time: at,
			Summary: map[string]any{"room": id, "outcome": "differ", "minutes": wholeMinutes(now.Sub(ep.since)), "members": members, "differs": differing(cmp)}})
	case !ep.resent && now.Sub(ep.reported) >= disagreeAfter && !radioHeld:
		ep.resent = true
		why := ""
		r, ok := d.intended(id, sc.master, ep.since, cmp, now, cubeNow, clock)
		// A held state's one re-send is the radiogram its structure change
		// costs: the day's limit does not stop it.
		first := ok && r.beforeChange && d.holds[id] != nil && !d.holds[id].resent
		switch last, sent := d.resent[id]; {
		case sent && now.Sub(last) < resendGap && !first:
			why = "today"
		case !ok:
			why = "no_state"
		}
		if why != "" {
			evs = append(evs, event{Kind: "disagree", Source: "cube", Level: "info", Time: at,
				Summary: map[string]any{"room": id, "outcome": "not_resent", "why": why}})
			return evs, nil
		}
		if first {
			d.holds[id].resent = true
		}
		d.resent[id] = now
		return evs, &r
	}
	return evs, nil
}

// driftWarn is the Warn of a member that does not follow its programme.
func driftWarn(snap *cubelan.Snapshot, room uint8, m roomMember, ms []roomMember, cubeNow, at time.Time) event {
	var others []string
	for _, o := range ms {
		if o.rf != m.rf {
			others = append(others, o.rf.String())
		}
	}
	expected, _ := roomorder.ProgramTemp(m.program, cubeNow)
	summary := map[string]any{"room": room, "outcome": "not_following", "setpoint": *m.st.Setpoint, "expected": expected,
		"switch_at": switchedAt(m.program, cubeNow), "cube_time": cubeNow.Format("2006-01-02 15:04"), "others": others, "auto_dst": m.st.AutoDST}
	if roomorder.FollowsAround(m.program, cubeNow.Add(-time.Hour), *m.st.Setpoint, driftGrace) || roomorder.FollowsAround(m.program, cubeNow.Add(time.Hour), *m.st.Setpoint, driftGrace) {
		summary["hour_off"] = true
	}
	if m.st.Error && m.st.FailedRadiogram != nil {
		summary["failed_radiogram"] = m.st.FailedRadiogram
	}
	return event{Kind: "drift", Source: "device", Level: "warn", Device: m.rf.String(), Serial: snap.Serial(m.rf), Time: at, Summary: summary}
}

// wholeMinutes is a duration in whole minutes.
func wholeMinutes(d time.Duration) int { return int(math.Round(d.Minutes())) }

// toRoom matches a command to a room: by its room, its target, or one
// of its s: lines, room-flagged to the room or to one of its members;
// a reconnect is none.
func toRoom(room uint8, members map[cube.RFAddress]bool) func(engine.Command) bool {
	return func(c engine.Command) bool {
		if c.Reconnect {
			return false
		}
		if c.RoomID == room || members[c.Target] {
			return true
		}
		for _, l := range c.Lines {
			if l.Type != 's' {
				continue
			}
			rc, err := cube.ParseRadioCommand(l.Data)
			if err == nil && (rc.Target.Room && rc.Target.RoomID == room || members[rc.Target.RFAddress]) {
				return true
			}
		}
		return false
	}
}

// structureRooms are the rooms a structure change of Massimo's that is
// read back concerns (structure.go): its rooms, and its device's.
func (rt *runtime) structureRooms(snap *cubelan.Snapshot) map[uint8]bool {
	out := map[uint8]bool{}
	f := rt.follow
	if f == nil {
		return out
	}
	var devices []cube.RFAddress
	f.mu.Lock()
	for _, w := range f.watches {
		for _, r := range w.rooms {
			out[r] = true
		}
		devices = append(devices, w.device)
	}
	f.mu.Unlock()
	for _, rf := range devices {
		if c := snap.Config(rf); c != nil {
			out[c.RoomID] = true
		}
	}
	return out
}

// resendRoom sends a disagreeing room its intended state once more, as
// a room command at low priority, and logs what became of it.
func (s *Server) resendRoom(ctx context.Context, rt *runtime, r resend) {
	snap := rt.client.Snapshot()
	addr, ok := cubeio.RoomAddressee(snap, r.room, rt.hidden())
	var m cube.Message
	var err error
	if !ok {
		err = errors.New("the room has no thermostat to address")
	} else {
		m, err = cube.CmdSetTemperature(cube.Target{RFAddress: addr, RoomID: r.room, Room: true}, r.temp, r.mode, r.until)
	}
	var res engine.Result
	if err == nil {
		res = rt.queue.Enqueue(engine.Command{Key: fmt.Sprintf("roomcheck:room:%d", r.room), Priority: engine.PriorityLow, Lines: []cube.Message{m}}).Wait(ctx)
		err = res.Err
	}
	if ctx.Err() != nil {
		return
	}
	ev := event{Kind: "disagree", Source: "cube", Level: "info", Time: time.Now(), Summary: r.summary("resent")}
	if err != nil {
		ev.Level, ev.Summary["outcome"], ev.Error = "warn", "resend_failed", err.Error()
	} else {
		ev.Summary["duty_pct"], ev.Summary["free_slots"] = res.Budget.DutyCyclePct, res.Budget.FreeMemorySlots
		rt.afterWrite()
	}
	rt.emit(ev)
}

// seedRoomChecks reads the day's re-sends, the room checks' and Block
// 8's of an unconfirmed room setpoint (resend.go), and the drift Warns
// from the store's log once, at the first check of a Cube with a row
// there or at Block 8's first decision, so that a restart does not lift
// their limits.
func (s *Server) seedRoomChecks(ctx context.Context, rt *runtime, now time.Time) {
	d := &rt.drift
	d.mu.Lock()
	done := d.seeded
	d.mu.Unlock()
	if done {
		return
	}
	id := rt.storeID.Load()
	if rt.store == nil || id == 0 {
		return // no log to read, or not yet: the next check tries again
	}
	resent, warned := map[uint8]time.Time{}, map[cube.RFAddress]time.Time{}
	for _, kind := range []string{"disagree", "drift", "unconfirmed"} {
		q := store.EventQuery{CubeID: id, Kind: kind, Since: now.Add(-resendGap), Limit: store.MaxEventsPage}
		rows, err := rt.store.Events(ctx, q)
		if err != nil {
			s.log.Warn("room checks: the day's re-sends not read from the log", "cube", rt.cubeSerial(), "error", err)
			return
		}
		for _, row := range rows {
			var ev struct {
				Summary map[string]any `json:"summary"`
			}
			if json.Unmarshal(row.Data, &ev) != nil {
				continue
			}
			switch outcome, _ := ev.Summary["outcome"].(string); {
			case kind != "drift" && (outcome == "resent" || outcome == "resend_failed"): // the room checks' and Block 8's (D199-A)
				if room, ok := ev.Summary["room"].(float64); ok && row.Time.After(resent[uint8(room)]) {
					resent[uint8(room)] = row.Time
				}
			case kind == "drift" && outcome == "not_following":
				if rf, err := cube.ParseRFAddress(row.Device); err == nil && row.Time.After(warned[rf]) {
					warned[rf] = row.Time
				}
			}
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.initLocked()
	for room, t := range resent {
		if t.After(d.resent[room]) {
			d.resent[room] = t
		}
	}
	for rf, t := range warned {
		if t.After(d.warned[rf]) {
			d.warned[rf] = t
		}
	}
	d.seeded = true
}

// roomCheckSettles settles a room check's Warn: a later disagree of the
// same room whose members agree, a later drift of the same device that
// follows its programme or left auto.
func roomCheckSettles(alert, later *event) bool {
	outcome := summaryText(later, "outcome")
	switch alert.Kind {
	case "disagree":
		return outcome == "agree" && summaryText(later, "room") == summaryText(alert, "room")
	case "drift":
		return later.Device == alert.Device && (outcome == "follows" || outcome == "left_auto")
	}
	return false
}
