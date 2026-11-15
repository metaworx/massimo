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
// (structure.go) and while a command to it waits or runs in the queue;
// a disagreement not yet reported starts again after that.
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
// room command: Massimo's last setpoint to the room, where it is recent
// and a member still shows it, else the room's wall thermostat's state.
// It goes through the queue at low priority, never while the queue holds
// the radio for the duty cycle or the memory slots, and at most once a
// day per room, the day's re-sends seeded from the log at the first
// check. Agreeing again, an Info says so.
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
	"math"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
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
	// minutes off.
	driftGrace = 5 * time.Minute
	// driftPersist is how long a member must not follow its programme
	// before the Warn.
	driftPersist = 5 * time.Minute
	// driftWarnGap is the least time between two drift Warns of a
	// member.
	driftWarnGap = 24 * time.Hour
)

// The re-sent state's origins, the summary's from.
const (
	fromMassimo = "massimo"
	fromWall    = "wall_thermostat"
)

// driftState is a runtime's room checks: each room's disagreement and
// each member's drift as the checks follow them, the last re-send per
// room, the last drift Warn per member, and the last setpoint of
// Massimo's per room.
type driftState struct {
	mu      sync.Mutex
	rooms   map[uint8]*disagreement
	members map[cube.RFAddress]*driftEpisode
	resent  map[uint8]time.Time
	warned  map[cube.RFAddress]time.Time
	intent  map[uint8]roomIntent
	seeded  bool // the day's re-sends and Warns read from the log
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

// roomIntent is a setpoint of Massimo's to a room, and when it went.
type roomIntent struct {
	mode  cube.Mode
	temp  float64
	until cube.Until
	at    time.Time
}

// shownBy reports whether a member shows the setpoint: its mode, and its
// temperature but for auto on the programme, and its until in vacation.
func (in roomIntent) shownBy(m roomMember) bool {
	st := m.st
	if st.Mode != in.mode {
		return false
	}
	switch {
	case in.mode == cube.ModeAuto && in.temp == 0:
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
// itself, is none.
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
	wall    bool
	st      cube.DeviceState
	state   *view.State
	program *cube.WeekProgram
	judged  bool // in auto, with a programme and the Cube's time
	follows bool // judged, and its setpoint is its programme's
}

// roomScan is what a check finds in one room.
type roomScan struct {
	members []roomMember            // the members compared
	all     map[cube.RFAddress]bool // every thermostat of the room
	window  bool                    // a window contact of the room is open
}

// scanRoom reads one room's members from the snapshot, those in skip
// left out, judging the members in auto by the Cube's time where it is
// known.
func scanRoom(snap *cubelan.Snapshot, room uint8, skip view.Hidden, cubeNow time.Time, clock bool) roomScan {
	sc := roomScan{all: map[cube.RFAddress]bool{}}
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
		m := roomMember{rf: rf, wall: c.Type == cube.DeviceWallThermostat, st: st, state: view.StateOf(st)}
		switch {
		case c.Thermostat != nil:
			m.program = &c.Thermostat.WeekProgram
		case c.Wall != nil:
			m.program = &c.Wall.WeekProgram
		}
		if st.Mode == cube.ModeAuto && m.program != nil && clock {
			m.judged = true
			m.follows = followsAround(m.program, cubeNow, *st.Setpoint, driftGrace)
		}
		sc.members = append(sc.members, m)
	}
	return sc
}

// cubeClock is the Cube's clock at now: the date and time of its last
// greeting (the H line's, local time to the minute), moved on by this
// host's local wall clock since, so that a change of summer time here
// moves it as it moves the Cube's.
func cubeClock(snap *cubelan.Snapshot, now time.Time) (time.Time, bool) {
	h, at := snap.Hello(), snap.HelloAt()
	if h == nil || at.IsZero() || h.CubeDate == "" || h.CubeTime == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02 15:04", h.CubeDate+" "+h.CubeTime)
	if err != nil {
		return time.Time{}, false
	}
	return t.Add(wallClock(now).Sub(wallClock(at))), true
}

// wallClock is t's local date and time read as UTC, for the difference
// of two wall clock readings.
func wallClock(t time.Time) time.Time {
	l := t.Local()
	return time.Date(l.Year(), l.Month(), l.Day(), l.Hour(), l.Minute(), l.Second(), l.Nanosecond(), time.UTC)
}

// programDay is w's programme of t's weekday; the devices' week starts
// on Saturday.
func programDay(w *cube.WeekProgram, t time.Time) cube.DayProgram {
	return w[(int(t.Weekday())+1)%7]
}

// programTemp is w's temperature at t, a time of the Cube's clock.
func programTemp(w *cube.WeekProgram, t time.Time) (float64, bool) {
	m := t.Hour()*60 + t.Minute()
	for _, p := range programDay(w, t) {
		if m < p.Until {
			return p.Temp, true
		}
	}
	return 0, false
}

// switchedAt is when the switch point in force at t began, as "HH:MM".
func switchedAt(w *cube.WeekProgram, t time.Time) string {
	m, start := t.Hour()*60+t.Minute(), 0
	for _, p := range programDay(w, t) {
		if m < p.Until {
			break
		}
		start = p.Until
	}
	return fmt.Sprintf("%02d:%02d", start/60, start%60)
}

// followsAround reports whether setpoint is w's temperature at a minute
// within grace of t.
func followsAround(w *cube.WeekProgram, t time.Time, setpoint float64, grace time.Duration) bool {
	for d := -grace; d <= grace; d += time.Minute {
		if temp, ok := programTemp(w, t.Add(d)); ok && temp == setpoint {
			return true
		}
	}
	return false
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

// membersAgree reports whether two members agree: in mode, setpoint and
// until, or both in auto, each following its own programme.
func membersAgree(a, b roomMember) bool {
	return a.follows && b.follows || view.SameSetting(a.state, b.state)
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
	room  uint8
	mode  cube.Mode
	temp  float64 // 0 with auto: the programme
	until cube.Until
	from  string // fromMassimo or fromWall
}

// summary is the re-sent state as an event names it.
func (r resend) summary(outcome string) map[string]any {
	out := map[string]any{"room": r.room, "outcome": outcome, "from": r.from, "mode": r.mode.String()}
	if r.temp != 0 {
		out["setpoint"] = r.temp
	}
	if !r.until.IsZero() {
		out["until"] = r.until.String()
	}
	return out
}

// intended is the state to send a disagreeing room once more: Massimo's
// last setpoint to it, sent since the disagreement began or at most
// intentLead before, while a member shows it; else the room's wall
// thermostat's, on its programme where it follows it.
func (d *driftState) intended(room uint8, since time.Time, ms []roomMember) (resend, bool) {
	if in, ok := d.intent[room]; ok && !in.at.Before(since.Add(-intentLead)) {
		for _, m := range ms {
			if in.shownBy(m) {
				return resend{room: room, mode: in.mode, temp: in.temp, until: in.until, from: fromMassimo}, true
			}
		}
	}
	for _, m := range ms {
		if !m.wall {
			continue
		}
		r := resend{room: room, mode: m.st.Mode, temp: *m.st.Setpoint, from: fromWall}
		if m.st.Until != nil {
			r.until = *m.st.Until
		}
		if m.follows {
			r.temp = 0
		}
		return r, true
	}
	return resend{}, false
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
	cubeNow, clock := cubeClock(snap, now)
	skip := view.NewHidden(rt.config().Hidden, md, false)
	busy := len(rt.structure) > 0 // a structure change is being sent
	held := rt.queue.Holding()
	watched := rt.structureRooms(snap)
	type room struct {
		id    uint8
		scan  roomScan
		pause bool
	}
	var rooms []room
	for _, r := range md.Rooms {
		if r.ID == 0 || r.ID == cube.HiddenRoom {
			continue
		}
		sc := scanRoom(snap, r.ID, skip, cubeNow, clock)
		pause := busy || sc.window || watched[r.ID] || rt.queue.Pending(toRoom(r.ID, sc.all))
		rooms = append(rooms, room{r.ID, sc, pause})
	}

	d := &rt.drift
	d.mu.Lock()
	if d.rooms == nil {
		d.rooms, d.members, d.resent, d.warned = map[uint8]*disagreement{}, map[cube.RFAddress]*driftEpisode{}, map[uint8]time.Time{}, map[cube.RFAddress]time.Time{}
	}
	var evs []event
	var sends []resend
	seen := map[uint8]bool{}
	present := map[cube.RFAddress]bool{}
	for _, r := range rooms {
		seen[r.id] = true
		for rf := range r.scan.all {
			present[rf] = true
		}
		if r.pause {
			d.pause(r.id, r.scan)
			continue
		}
		e, send := d.checkRoom(snap, r.id, r.scan, now, cubeNow, held)
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
// not hold the radio (held).
func (d *driftState) checkRoom(snap *cubelan.Snapshot, id uint8, sc roomScan, now, cubeNow time.Time, held bool) ([]event, *resend) {
	var evs []event
	at := time.Now()
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

	var cmp []roomMember
	for _, m := range sc.members {
		if odd == nil || m.rf != odd.rf {
			cmp = append(cmp, m)
		}
	}
	ep := d.rooms[id]
	if !disagrees(cmp) {
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
	case !ep.resent && now.Sub(ep.reported) >= disagreeAfter && !held:
		ep.resent = true
		why := ""
		r, ok := d.intended(id, ep.since, cmp)
		switch last, sent := d.resent[id]; {
		case sent && now.Sub(last) < resendGap:
			why = "today"
		case !ok:
			why = "no_state"
		}
		if why != "" {
			evs = append(evs, event{Kind: "disagree", Source: "cube", Level: "info", Time: at,
				Summary: map[string]any{"room": id, "outcome": "not_resent", "why": why}})
			return evs, nil
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
	expected, _ := programTemp(m.program, cubeNow)
	summary := map[string]any{"room": room, "outcome": "not_following", "setpoint": *m.st.Setpoint, "expected": expected,
		"switch_at": switchedAt(m.program, cubeNow), "cube_time": cubeNow.Format("2006-01-02 15:04"), "others": others, "auto_dst": m.st.AutoDST}
	if followsAround(m.program, cubeNow.Add(-time.Hour), *m.st.Setpoint, driftGrace) || followsAround(m.program, cubeNow.Add(time.Hour), *m.st.Setpoint, driftGrace) {
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

// seedRoomChecks reads the day's re-sends and drift Warns from the
// store's log once, at the first check of a Cube with a row there, so
// that a restart does not lift their limits.
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
	for _, kind := range []string{"disagree", "drift"} {
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
			case kind == "disagree" && (outcome == "resent" || outcome == "resend_failed"):
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
	if d.resent == nil {
		d.rooms, d.members, d.resent, d.warned = map[uint8]*disagreement{}, map[cube.RFAddress]*driftEpisode{}, map[uint8]time.Time{}, map[cube.RFAddress]time.Time{}
	}
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
