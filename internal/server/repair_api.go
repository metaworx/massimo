// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The repairs of a Cube's rooms and devices (AP RadioHealth v1.2, Blocks
// 11 and 12, D163; docs/api-v1.md, "Repairs"). Each is a route under the
// capability repair. Its GET is the preview: what a run would send, line
// by line, and what that costs, the radiograms and their duty cycle, the
// memory slots they take, the resets (free) and the wakeups, sending
// nothing. Its POST with ?plan=<the preview's hash> runs it: the repair
// is built again from the Cube as it is then and refused where it
// differs, so that what is sent is what was shown, and refused where it
// would reach the queue's hold for the duty cycle or the memory slots
// and stop halfway. A run is judged by what the Cube shows: the lines it
// accepted; for radiograms, the structure read-back (structure.go), the
// Cube's free slots back at their count and no member left with a flag
// it did not have, in the log; for resets, the flags in the device list
// read after them.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// DefaultResetSettle is how long after a repair's resets the device list
// is read for their flags: the Cube showed a reset's flag cleared in the
// next device list, 2 to 13 s later (docs/cube-protocol.md).
const DefaultResetSettle = 2 * time.Second

// repairRunTimeout bounds a repair's run: a run that fits the budget
// sends a few dozen lines, each answered within seconds.
const repairRunTimeout = 10 * time.Minute

// repairRoutes registers the repairs, each a GET (the preview) and a
// POST (the run) under repair; "unlink all and re-link" needs structure
// as well, which its handler checks.
func (s *Server) repairRoutes(api func(pattern, capability string, h func(http.ResponseWriter, *http.Request) error)) {
	api("GET /api/v1/cubes/{sn}/repair/reset", "repair", s.repair("reset", s.buildCubeReset, false))
	api("POST /api/v1/cubes/{sn}/repair/reset", "repair", s.repair("reset", s.buildCubeReset, true))
	api("GET /api/v1/cubes/{sn}/rooms/{id}/repair/reset", "repair", s.repair("reset", s.buildRoomReset, false))
	api("POST /api/v1/cubes/{sn}/rooms/{id}/repair/reset", "repair", s.repair("reset", s.buildRoomReset, true))
	api("GET /api/v1/cubes/{sn}/rooms/{id}/repair/master", "repair", s.repair("master", s.buildMaster, false))
	api("POST /api/v1/cubes/{sn}/rooms/{id}/repair/master", "repair", s.repair("master", s.buildMaster, true))
	api("GET /api/v1/cubes/{sn}/rooms/{id}/repair/relink", "repair", s.repair("relink", s.buildRelink, false))
	api("POST /api/v1/cubes/{sn}/rooms/{id}/repair/relink", "repair", s.repair("relink", s.buildRelink, true))
	api("GET /api/v1/cubes/{sn}/rooms/{id}/repair/unlink-relink", "repair", s.repair("unlink_relink", s.buildUnlinkRelink, false))
	api("POST /api/v1/cubes/{sn}/rooms/{id}/repair/unlink-relink", "repair", s.repair("unlink_relink", s.buildUnlinkRelink, true))
	api("GET /api/v1/cubes/{sn}/devices/{rf}/repair/config", "repair", s.repair("config", s.buildResend, false))
	api("POST /api/v1/cubes/{sn}/devices/{rf}/repair/config", "repair", s.repair("config", s.buildResend, true))
	api("GET /api/v1/cubes/{sn}/devices/{rf}/repair/place", "repair", s.repair("move", s.buildPlace, false))
	api("POST /api/v1/cubes/{sn}/devices/{rf}/repair/place", "repair", s.repair("move", s.buildPlace, true))
	api("GET /api/v1/cubes/{sn}/devices/{rf}/repair/list", "repair", s.repair("list", s.buildList, false))
	api("POST /api/v1/cubes/{sn}/devices/{rf}/repair/list", "repair", s.repair("list", s.buildList, true))
}

// repairAction is one repair as it would run now: its commands in
// order, then a metadata write, and what its run is judged by.
type repairAction struct {
	name     string // as the log's structure line names it
	steps    []repairCmd
	metadata *cube.Metadata   // written after the steps; nil for none
	device   cube.RFAddress   // the device repaired, 0 for a room's or the Cube's
	rooms    []uint8          // the rooms concerned, for the read-back
	reread   []cube.RFAddress // the C lines followed up after a run
	resets   []cube.RFAddress // the devices whose flags the run resets
	notes    []string
	untested bool           // not yet run on hardware
	details  map[string]any // what the preview says of the repair's own
	// structure says the run holds the Cube's structure lock, as the
	// moves and removals do.
	structure bool
	// hold is the room a placement brings a thermostat into, whose state
	// the run reads before its steps and holds for the room checks once
	// its last step, the link half, reached a member (Server.roomBefore,
	// AP RoomMajority, D15-A); 0 for none.
	hold uint8
}

// repairCmd is one command of a repair and what it does.
type repairCmd struct {
	what string
	cmd  engine.Command
}

// repairPlan is the answer of a repair's GET, its preview.
type repairPlan struct {
	Action string       `json:"action"`
	Plan   string       `json:"plan"`
	Steps  []repairStep `json:"steps"`
	// Radiograms counts the s: lines; DutyCostPct is about what they
	// cost of the hourly radio budget, radioCostPct each.
	Radiograms  int     `json:"radiograms"`
	DutyCostPct float64 `json:"duty_cost_pct"`
	// Slots is the memory slots the radiograms take: one each, a
	// radiogram with the room flag one per member of the room (T134);
	// Held, those that stay taken until the members' next wakeup, about
	// ten minutes: a room radiogram's but the addressee's.
	Slots int `json:"slots"`
	Held  int `json:"held_slots"`
	// Resets counts the r: lines, those the queue's prelude sends before
	// each command included (engine.PreludeOf), which cost no radio
	// time; Wakeups the room wakeups before the commands.
	Resets   int  `json:"resets"`
	Wakeups  int  `json:"wakeups"`
	Metadata bool `json:"metadata"`
	// Budget is what the Cube last reported; Refused, why a run would be
	// refused now: it would reach the queue's hold and stop halfway.
	Budget  cubelan.Budget `json:"budget"`
	Refused string         `json:"refused,omitempty"`
	// Nothing says why the repair has nothing to send now, without a
	// plan; a run of it is refused with 409.
	Nothing string   `json:"nothing,omitempty"`
	Notes   []string `json:"notes"`
	// Untested says that the repair has not yet run on hardware.
	Untested bool           `json:"untested,omitempty"`
	Details  map[string]any `json:"details,omitempty"`
}

// repairStep is one command of a preview: the device it addresses, the
// room woken up before it, whether the queue's prelude resets flags
// first (the device's, the flagged members its room lines reach, the
// flagged devices its direct lines address), and its lines as they go
// to the Cube.
type repairStep struct {
	What   string   `json:"what"`
	Device string   `json:"device,omitempty"`
	Room   uint8    `json:"room,omitempty"`
	Reset  bool     `json:"reset,omitempty"`
	Lines  []string `json:"lines"`
}

// repairStepResult is how one command of a run went.
type repairStepResult struct {
	What      string `json:"what"`
	Lines     int    `json:"lines"`
	Accepted  int    `json:"accepted"`
	Discarded bool   `json:"discarded,omitempty"`
	Error     string `json:"error,omitempty"`
}

// repair is the handler of a repair's route: the preview, or with run
// the run, of the repair build makes, which action names. A preview of a
// repair with nothing to do, which a run refuses with 409, answers what
// it is and why (nothing), so that a client shows it as it shows a
// preview.
func (s *Server) repair(action string, build func(*http.Request, *runtime) (*repairAction, error), run bool) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		rt, err := s.cube(r)
		if err != nil {
			return err
		}
		if run {
			return s.runRepair(w, r, rt, build)
		}
		a, err := build(r, rt)
		var need *errRoomNameNeeded
		var he *httpError
		switch {
		case errors.As(err, &need):
			writeJSON(w, http.StatusOK, repairNothing(rt, action, err, map[string]any{"code": codeRoomNameNeeded.name, "room": need.room}))
			return nil
		case errors.As(err, &he) && he.status == http.StatusConflict:
			writeJSON(w, http.StatusOK, repairNothing(rt, action, err, nil))
			return nil
		case err != nil:
			return err
		}
		writeJSON(w, http.StatusOK, s.repairView(rt, a))
		return nil
	}
}

// repairNothing is the preview of a repair with nothing to send now:
// no plan and no steps, nothing saying why, and in details what a client
// may give to change that (a listing's room name).
func repairNothing(rt *runtime, action string, why error, details map[string]any) repairPlan {
	return repairPlan{Action: action, Steps: []repairStep{}, Notes: []string{}, Budget: rt.client.Budget(), Nothing: why.Error(), Details: details}
}

// errRoomNameNeeded refuses a listing into a room the Cube's list does
// not have, without the name to list it under.
type errRoomNameNeeded struct{ room uint8 }

func (e *errRoomNameNeeded) Error() string {
	return fmt.Sprintf(codeRoomNameNeeded.format, e.room)
}

// repairRefusal writes a refusal of a repair: an httpError as it is,
// errRoomNameNeeded with its code and room, top-level as before and in
// its params, so that a client can ask for the name.
func repairRefusal(w http.ResponseWriter, err error) error {
	var need *errRoomNameNeeded
	if errors.As(err, &need) {
		he := refuse(http.StatusConflict, codeRoomNameNeeded, need.room).(*httpError)
		body := he.body()
		body["room"] = need.room
		writeJSON(w, he.status, body)
		return nil
	}
	return err
}

// repairView is a repair's preview, its plan the hash over what it
// sends.
func (s *Server) repairView(rt *runtime, a *repairAction) repairPlan {
	snap := rt.client.Snapshot()
	v := repairPlan{Action: a.name, Steps: []repairStep{}, Metadata: a.metadata != nil, Budget: rt.client.Budget(), Notes: a.notes,
		Untested: a.untested, Details: a.details}
	if v.Notes == nil {
		v.Notes = []string{}
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\n", a.name)
	for _, st := range a.steps {
		step := repairStep{What: st.what, Lines: wireLines(st.cmd.Lines)}
		for _, l := range st.cmd.Lines {
			switch l.Type {
			case 's':
				v.Radiograms++
				v.Slots++
				if rc, err := cube.ParseRadioCommand(l.Data); err == nil && rc.Target.Room {
					if n := len(cubeio.ConfigMembers(snap, rc.Target.RoomID, rt.hidden())); n > 1 {
						v.Slots += n - 1
						v.Held += n - 1
					}
				}
			case 'r':
				v.Resets++
			}
		}
		if st.cmd.Target != 0 {
			step.Device = st.cmd.Target.String()
		}
		// The resets the queue's prelude sends before the step, by the
		// engine's own list (AP RadioHealth v1.3, D204-A): the Target's,
		// the flagged members its room lines reach, the flagged devices
		// its direct lines address.
		if n := len(engine.PreludeOf(snap, st.cmd).Resets()); n > 0 {
			step.Reset = true
			v.Resets += n
		}
		if st.cmd.Wakeup {
			step.Room = st.cmd.RoomID
			v.Wakeups++
		}
		fmt.Fprintf(h, "%d %v %s\n", st.cmd.RoomID, st.cmd.Wakeup, strings.Join(step.Lines, " "))
		v.Steps = append(v.Steps, step)
	}
	if a.metadata != nil {
		if lines, err := cube.WriteMetadataCommands(a.metadata); err == nil {
			step := repairStep{What: "write the room and device list", Lines: wireLines(lines)}
			fmt.Fprintf(h, "m %s\n", strings.Join(step.Lines, " "))
			v.Steps = append(v.Steps, step)
		}
	}
	v.Plan = hex.EncodeToString(h.Sum(nil))
	v.DutyCostPct = float64(v.Radiograms) * radioCostPct
	v.Refused = s.repairRefused(v)
	return v
}

// wireLines writes protocol lines as they go to the Cube.
func wireLines(lines []cube.Message) []string {
	out := []string{}
	for _, l := range lines {
		out = append(out, l.String())
	}
	return out
}

// repairRefused says why a run of v would be refused now, "" where it
// fits the budget the Cube last reported: with its cost the duty cycle
// would reach the queue's hold, or the slots its room radiograms hold
// would leave fewer free than the queue waits for; either would stop it
// halfway (engine.Options.MaxDutyPct, MinFreeSlots).
func (s *Server) repairRefused(v repairPlan) string {
	b := v.Budget
	if v.Radiograms == 0 || b.UpdatedAt.IsZero() {
		return ""
	}
	maxDuty, minFree := s.opts.Queue.MaxDutyPct, s.opts.Queue.MinFreeSlots
	if maxDuty == 0 {
		maxDuty = 95
	}
	if minFree == 0 {
		minFree = 5
	}
	if float64(b.DutyCyclePct)+v.DutyCostPct >= float64(maxDuty) {
		return fmt.Sprintf("the radio duty cycle at %d %% and the repair's about %.1f %% would reach the hold at %d %%, which would stop it halfway; wait for the duty cycle to fall",
			b.DutyCyclePct, v.DutyCostPct, maxDuty)
	}
	if b.FreeMemorySlots-v.Held < minFree {
		return fmt.Sprintf("the Cube has %d memory slots free and the repair's room radiograms hold %d until the devices' next wakeup, which would leave fewer than %d and stop it halfway",
			b.FreeMemorySlots, v.Held, minFree)
	}
	return ""
}

// runRepair runs a repair whose preview's hash the request names: built
// again, compared, sent command by command until one fails, then its
// metadata; it answers how each step went, the budget, and for
// radiograms that the read-back is pending, for resets the devices
// still flagged.
func (s *Server) runRepair(w http.ResponseWriter, r *http.Request, rt *runtime, build func(*http.Request, *runtime) (*repairAction, error)) error {
	want := r.URL.Query().Get("plan")
	if want == "" {
		return fail(http.StatusBadRequest, "plan required: the hash of the repair's preview (GET %s)", r.URL.Path)
	}
	ctx, cancel := context.WithTimeout(r.Context(), repairRunTimeout)
	defer cancel()
	a, err := build(r, rt)
	if err != nil {
		return repairRefusal(w, err)
	}
	if a.structure {
		unlock, err := rt.lockStructure(ctx)
		if err != nil {
			return err
		}
		defer unlock()
		// Built again under the lock: another change may have run while
		// this one waited for it.
		if a, err = build(r, rt); err != nil {
			return repairRefusal(w, err)
		}
	}
	v := s.repairView(rt, a)
	if v.Plan != want {
		return fail(http.StatusConflict, "the repair changed since its preview (plan %s, now %s); preview it again", want, v.Plan)
	}
	if v.Refused != "" {
		return fail(http.StatusConflict, "%s", v.Refused)
	}
	if !rt.client.Connected() {
		return fail(http.StatusServiceUnavailable, "cube not connected")
	}
	var was before
	if a.hold != 0 {
		was = s.roomBefore(rt, a.hold, a.device) // before the link half
	}
	var watch *structureWatch
	if v.Radiograms > 0 {
		watch = rt.beginStructure(a.name, a.device, a.rooms...)
	}
	s.log.Info("repair started", append(a.logAttrs(rt), "radiograms", v.Radiograms, "resets", v.Resets, "remote", s.clientIP(r))...)
	answer := map[string]any{"action": a.name}
	var results []repairStepResult
	var failed error
	writer := rt.writerOf(ctx)
	for i, st := range a.steps {
		res := writer.Do(ctx, st.cmd)
		if a.hold != 0 && i == len(a.steps)-1 {
			was.hold(rt, cubeio.Reached(st.cmd, res)) // the link half reached the room
		}
		sr := repairStepResult{What: st.what, Lines: len(st.cmd.Lines), Discarded: res.Discarded}
		for _, rep := range res.Replies {
			if rep.Type != 'S' {
				sr.Accepted++
			} else if p, err := cube.ParseSendResult(rep.Data); err == nil && !p.Discarded {
				sr.Accepted++
			}
		}
		if res.Err != nil {
			sr.Error = res.Err.Error()
			failed = s.commandError(rt, res, 0)
		}
		results = append(results, sr)
		if failed != nil {
			break
		}
	}
	if failed == nil && a.metadata != nil {
		failed = s.writeMetadata(ctx, rt, a.metadata)
		sr := repairStepResult{What: "write the room and device list", Lines: 1}
		if failed == nil {
			sr.Accepted = 1
		} else {
			sr.Error = failed.Error()
		}
		results = append(results, sr)
	}
	if v.Radiograms > 0 {
		for _, rf := range a.reread {
			rt.writer().Reread(ctx, rf)
		}
		rt.afterWrite(a.reread...)
		if rt.watchStructure(watch) {
			answer["confirmation"] = "pending"
		}
	}
	answer["steps"], answer["budget"] = results, rt.client.Budget()
	if failed != nil {
		var he *httpError
		status := http.StatusBadGateway
		if errors.As(failed, &he) {
			status = he.status
		}
		s.log.Warn("repair stopped", append(a.logAttrs(rt), "steps_done", len(results)-1, "steps", len(a.steps), "error", failed)...)
		answer["error"] = fmt.Sprintf("%s: stopped at step %d of %d: %v", a.name, len(results), len(v.Steps), failed)
		writeJSON(w, status, answer)
		return nil
	}
	if len(a.resets) > 0 {
		answer["still_flagged"] = s.stillFlagged(ctx, rt, a.resets)
	}
	answer["ok"] = true
	s.log.Info("repair sent", append(a.logAttrs(rt), "steps", len(results))...)
	writeJSON(w, http.StatusOK, answer)
	return nil
}

// logAttrs are the key-value pairs that name a repair in the log: the
// Cube, the action, the device where it has one, the rooms as numbers.
func (a *repairAction) logAttrs(rt *runtime) []any {
	attrs := []any{"cube", rt.cubeSerial(), "action", a.name}
	if a.device != 0 {
		attrs = append(attrs, "device", a.device)
	}
	var rooms []int
	for _, r := range a.rooms {
		if r != 0 {
			rooms = append(rooms, int(r))
		}
	}
	if len(rooms) > 0 {
		attrs = append(attrs, "rooms", rooms)
	}
	return attrs
}

// stillFlagged reads the device list once the resets settled and lists
// those of devices still flagged.
func (s *Server) stillFlagged(ctx context.Context, rt *runtime, devices []cube.RFAddress) []string {
	settle := s.opts.ResetSettle
	if settle <= 0 {
		settle = DefaultResetSettle
	}
	select {
	case <-ctx.Done():
	case <-time.After(settle):
	}
	rt.writer().Do(ctx, engine.Command{Lines: []cube.Message{cube.CmdList}})
	snap := rt.client.Snapshot()
	out := []string{}
	for _, rf := range devices {
		if st, ok := snap.State(rf); ok && st.Error {
			out = append(out, rf.String())
		}
	}
	return out
}

// The repairs.

// buildCubeReset is the reset of every flagged device of the Cube: one
// r: each, at no radio cost (item 1).
func (s *Server) buildCubeReset(r *http.Request, rt *runtime) (*repairAction, error) {
	if rt.client.Snapshot().Metadata() == nil {
		return nil, fail(http.StatusServiceUnavailable, "cube not connected yet")
	}
	flagged := cubeio.Flagged(rt.client.Snapshot(), 0, rt.hidden())
	if len(flagged) == 0 {
		return nil, fail(http.StatusConflict, "no device of the cube has a communication error")
	}
	return resetAction(rt, "reset", flagged, "repair-reset"), nil
}

// buildRoomReset is the reset of the flagged devices of a room, by their
// C lines (item 1).
func (s *Server) buildRoomReset(r *http.Request, rt *runtime) (*repairAction, error) {
	room, _, err := s.visibleRoom(rt, r)
	if err != nil {
		return nil, err
	}
	flagged := cubeio.Flagged(rt.client.Snapshot(), room.ID, rt.hidden())
	if len(flagged) == 0 {
		return nil, fail(http.StatusConflict, "no device of room %d has a communication error", room.ID)
	}
	a := resetAction(rt, "reset", flagged, fmt.Sprintf("repair-reset:room:%d", room.ID))
	a.rooms = []uint8{room.ID}
	return a, nil
}

// resetAction is the reset of devices, one command.
func resetAction(rt *runtime, name string, devices []cube.RFAddress, key string) *repairAction {
	var names []string
	for _, rf := range devices {
		names = append(names, rf.String())
	}
	return &repairAction{name: name, resets: devices,
		steps:   []repairCmd{{what: "reset the communication error of " + strings.Join(names, ", "), cmd: cubeio.ResetCommand(key, devices)}},
		details: map[string]any{"devices": names}}
}

// buildMaster names a thermostat of the room its master in the metadata,
// its wall thermostat, else its first radiator thermostat, a dead one
// passed over (cubeio.MasterCandidate, RoomMajority.D9, in place of
// D167), where the metadata's master is no thermostat of the room
// (item 3).
func (s *Server) buildMaster(r *http.Request, rt *runtime) (*repairAction, error) {
	room, md, err := s.visibleRoom(rt, r)
	if err != nil {
		return nil, err
	}
	snap, hidden := rt.client.Snapshot(), rt.hidden()
	if cubeio.MasterValid(snap, room.ID, hidden) {
		return nil, fail(http.StatusConflict, "room %d's master %s is a thermostat of the room already", room.ID, room.RFAddress)
	}
	next, ok := cubeio.MasterCandidate(snap, room.ID, hidden)
	if !ok {
		return nil, fail(http.StatusConflict, "room %d has no thermostat to name as its master", room.ID)
	}
	cp := *md
	cp.Rooms = append([]cube.RoomMeta(nil), md.Rooms...)
	for i := range cp.Rooms {
		if cp.Rooms[i].ID == room.ID {
			cp.Rooms[i].RFAddress = next
		}
	}
	return &repairAction{name: "master", metadata: &cp, rooms: []uint8{room.ID}, structure: true,
		details: map[string]any{"room": room.ID, "master": room.RFAddress.String(), "new_master": next.String()}}, nil
}

// roomToLink is a room a re-link may run on: visible, its list and the
// Cube agreeing on its members.
func (s *Server) roomToLink(r *http.Request, rt *runtime) (cube.RoomMeta, error) {
	room, _, err := s.visibleRoom(rt, r)
	if err != nil {
		return room, err
	}
	if off := cubeio.RoomDisagreement(rt.client.Snapshot(), room.ID, rt.hidden()); len(off) > 0 {
		var names []string
		for _, rf := range off {
			names = append(names, rf.String())
		}
		return room, fail(http.StatusConflict, "the Cube and its list disagree on the devices of room %d (%s): place or list them first",
			room.ID, strings.Join(names, ", "))
	}
	return room, nil
}

// buildRelink is the re-link of a room, adding only (item 4).
func (s *Server) buildRelink(r *http.Request, rt *runtime) (*repairAction, error) {
	room, err := s.roomToLink(r, rt)
	if err != nil {
		return nil, err
	}
	cmds := cubeio.RelinkCommands(rt.client.Snapshot(), room.ID, rt.hidden())
	if len(cmds) < 2 {
		return nil, fail(http.StatusConflict, "room %d has fewer than two devices to link", room.ID)
	}
	a := &repairAction{name: "relink", rooms: []uint8{room.ID}, structure: true}
	for _, c := range cmds {
		a.steps = append(a.steps, repairCmd{what: "link " + c.Target.String() + " with its partners in room " + fmt.Sprint(room.ID), cmd: c})
	}
	return a, nil
}

// buildUnlinkRelink is "unlink all and re-link" of a room (Block 12,
// D163-B): every link removed, then built again. It needs structure as
// well as repair, its preview included.
func (s *Server) buildUnlinkRelink(r *http.Request, rt *runtime) (*repairAction, error) {
	if err := s.require(r, "structure"); err != nil {
		return nil, err
	}
	room, err := s.roomToLink(r, rt)
	if err != nil {
		return nil, err
	}
	cmds := cubeio.UnlinkRelinkCommands(rt.client.Snapshot(), room.ID, rt.hidden())
	links := 0
	for _, c := range cmds {
		if strings.HasPrefix(c.Key, "repair-relink:") {
			links++
		}
	}
	if links < 2 {
		return nil, fail(http.StatusConflict, "room %d has fewer than two devices to link", room.ID)
	}
	a := &repairAction{name: "unlink_relink", rooms: []uint8{room.ID}, structure: true, untested: true,
		notes: []string{"not yet run on hardware: every link of the room is removed before it is built again, and a run that stops halfway leaves the room's devices without their partners until it is run again"}}
	for _, c := range cmds {
		what := fmt.Sprintf("link %s with its partners in room %d", c.Target, room.ID)
		if strings.HasPrefix(c.Key, "repair-unlink:") {
			what = fmt.Sprintf("unlink %s from its partners in room %d", c.Target, room.ID)
		}
		a.steps = append(a.steps, repairCmd{what: what, cmd: c})
	}
	return a, nil
}

// repairDevice is the device a device's repair names: visible, no
// virtual wall thermostat, with its C line and its metadata entry, nil
// where it has none.
func (s *Server) repairDevice(r *http.Request, rt *runtime) (*cube.DeviceConfig, *cube.DeviceMeta, *cube.Metadata, error) {
	if _, ok := virtualOf(rt, r.PathValue("rf")); ok {
		return nil, nil, nil, fail(http.StatusBadRequest, "a virtual wall thermostat is no device of the cube; repair its room")
	}
	_, c, err := s.visibleDevice(rt, r)
	if err != nil {
		return nil, nil, nil, err
	}
	if c.Type == cube.DeviceCube {
		return nil, nil, nil, fail(http.StatusBadRequest, "%s is the cube itself", c.RFAddress)
	}
	md := rt.client.Snapshot().Metadata()
	if md == nil {
		return nil, nil, nil, fail(http.StatusServiceUnavailable, "cube not connected yet")
	}
	for i := range md.Devices {
		if md.Devices[i].RFAddress == c.RFAddress {
			return c, &md.Devices[i], md, nil
		}
	}
	return c, nil, md, nil
}

// buildResend re-sends a thermostat's configuration, its temperatures
// and the days of its week programme that differ, from its room's by
// their C lines (item 2; cubeio.RoomAgreement, the one rule of AP
// RoomMajority's D9 over the room's thermostats, the device's included,
// as the report of members that differ decides it: what most of them
// share, else the first of the room's order's, those the Cube holds no
// valid state for passed over).
func (s *Server) buildResend(r *http.Request, rt *runtime) (*repairAction, error) {
	c, listed, _, err := s.repairDevice(r, rt)
	if err != nil {
		return nil, err
	}
	if !c.Type.IsThermostat() {
		return nil, fail(http.StatusBadRequest, "device %s is a %s and has no configuration to re-send", c.RFAddress, c.Type)
	}
	room := c.RoomID
	if room == 0 {
		return nil, fail(http.StatusConflict, "the Cube has device %s in no room", c.RFAddress)
	}
	if listed != nil && listed.RoomID != room {
		return nil, fail(http.StatusConflict, "the Cube has device %s in room %d and its list in room %d: place or list it first", c.RFAddress, room, listed.RoomID)
	}
	cmds, sent, agree, others, err := cubeio.ResendCommands(rt.client.Snapshot(), c.RFAddress, room, rt.hidden())
	var none *cubeio.NoMajorityError
	switch {
	case errors.As(err, &none):
		return nil, fail(http.StatusConflict, "%v", err)
	case err != nil:
		return nil, fail(http.StatusBadRequest, "%v", err)
	case len(cmds) == 0:
		return nil, fail(http.StatusConflict, "device %s has room %d's temperatures and week programme already (what most of the room's %d thermostats share, else what the first of the room's order has); nothing to re-send",
			c.RFAddress, room, agree.Others)
	}
	days := []string{}
	for _, d := range sent.Days {
		days = append(days, d.String())
	}
	a := &repairAction{name: "config", device: c.RFAddress, rooms: []uint8{room}, reread: []cube.RFAddress{c.RFAddress},
		details: map[string]any{"room": room, "others": others, "thermostats": agree.Others, "temperatures_by": agree.TempsBy, "program_by": agree.ProgramBy(),
			"temperatures": sent.Temperatures, "days": days, "agreed_temperatures": agree.Temps}}
	for _, cmd := range cmds {
		what := "temperatures of " + c.RFAddress.String()
		if len(days) > 0 {
			what = "week programme (" + strings.Join(days, ", ") + ") and temperatures of " + c.RFAddress.String()
		}
		a.steps = append(a.steps, repairCmd{what: what, cmd: cmd})
	}
	return a, nil
}

// buildPlace places a device in the room the metadata lists it in, where
// its C line names another room or none: a move from its C line's room,
// cubeio.MoveCommands, or the placement's link half alone from none
// (item 3). The metadata stays as it is; the room's settings are not
// copied (item 2 re-sends them). A thermostat's run holds the room's
// state before its steps, as an add does (hold; AP RoomMajority,
// D15-A): a placed device brings its state as an added one.
func (s *Server) buildPlace(r *http.Request, rt *runtime) (*repairAction, error) {
	c, listed, md, err := s.repairDevice(r, rt)
	if err != nil {
		return nil, err
	}
	switch {
	case listed == nil:
		return nil, fail(http.StatusConflict, "the Cube's list does not list device %s: list it instead", c.RFAddress)
	case listed.RoomID == 0:
		return nil, fail(http.StatusConflict, "the Cube's list places device %s in no room", c.RFAddress)
	case listed.RoomID == c.RoomID:
		return nil, fail(http.StatusConflict, "the Cube has device %s in room %d already, as its list", c.RFAddress, c.RoomID)
	case c.RoomID == cube.HiddenRoom:
		return nil, fail(http.StatusConflict, "the Cube has device %s in the hidden room %d", c.RFAddress, cube.HiddenRoom)
	}
	to := listed.RoomID
	if _, ok := rt.hidden().Room(md, to); !ok {
		return nil, fail(http.StatusNotFound, "no room %d", to)
	}
	if other, ok := secondWall(md, c.RFAddress, to); ok && c.Type == cube.DeviceWallThermostat {
		return nil, fail(http.StatusConflict, "room %d has a wall thermostat already (%s); a room takes one, as in the original software", to, other)
	}
	var members []cube.DeviceMeta
	for _, d := range md.Devices {
		if d.RoomID == to {
			members = append(members, d)
		}
	}
	out, in := cubeio.MoveCommands(rt.client.Snapshot(), c.RFAddress, c.Type, c.RoomID, to, members, rt.hidden())
	a := &repairAction{name: "move", device: c.RFAddress, rooms: []uint8{c.RoomID, to}, reread: []cube.RFAddress{c.RFAddress}, structure: true,
		details: map[string]any{"from": c.RoomID, "to": to}}
	if c.Type.IsThermostat() {
		a.hold = to
	}
	if c.RoomID != 0 {
		a.steps = append(a.steps, repairCmd{what: fmt.Sprintf("unlink %s from room %d", c.RFAddress, c.RoomID), cmd: out})
	}
	a.steps = append(a.steps, repairCmd{what: fmt.Sprintf("link %s into room %d", c.RFAddress, to), cmd: in})
	return a, nil
}

// buildList lists a device in the room its C line names, metadata only
// (item 3): its entry moved there, or one made for a device the list
// does not have, named as the request's name says, else by
// defaultDeviceName (D185). A room the list does not have is made with
// room_name, its master the room's wall thermostat, else its first
// radiator thermostat, a dead one passed over (cubeio.MasterCandidate);
// without the name the request is refused.
func (s *Server) buildList(r *http.Request, rt *runtime) (*repairAction, error) {
	c, listed, md, err := s.repairDevice(r, rt)
	if err != nil {
		return nil, err
	}
	to := c.RoomID
	switch {
	case to == 0:
		return nil, fail(http.StatusConflict, "the Cube has device %s in no room", c.RFAddress)
	case to == cube.HiddenRoom:
		return nil, fail(http.StatusConflict, "the Cube has device %s in the hidden room %d", c.RFAddress, cube.HiddenRoom)
	case listed != nil && listed.RoomID == to:
		return nil, fail(http.StatusConflict, "the Cube's list has device %s in room %d already, as the Cube", c.RFAddress, to)
	}
	if other, ok := secondWall(md, c.RFAddress, to); ok && c.Type == cube.DeviceWallThermostat {
		return nil, fail(http.StatusConflict, "room %d has a wall thermostat already (%s); a room takes one, as in the original software", to, other)
	}
	cp := *md
	cp.Rooms = append([]cube.RoomMeta(nil), md.Rooms...)
	cp.Devices = append([]cube.DeviceMeta(nil), md.Devices...)
	newRoom := !slices.ContainsFunc(md.Rooms, func(m cube.RoomMeta) bool { return m.ID == to })
	var roomName string
	if newRoom {
		roomName = strings.TrimSpace(r.URL.Query().Get("room_name"))
		if roomName == "" {
			return nil, &errRoomNameNeeded{room: to}
		}
		if err := checkName(roomName); err != nil {
			return nil, err
		}
		master, _ := cubeio.MasterCandidate(rt.client.Snapshot(), to, rt.hidden())
		cp.Rooms = append(cp.Rooms, cube.RoomMeta{ID: to, Name: roomName, RFAddress: master})
	} else if room, ok := rt.hidden().Room(md, to); ok {
		roomName = room.Name
	} else {
		return nil, fail(http.StatusNotFound, "no room %d", to)
	}
	details := map[string]any{"to": to, "listed": listed != nil, "new_room": newRoom}
	from := uint8(0)
	if listed != nil {
		from = listed.RoomID
		for i := range cp.Devices {
			if cp.Devices[i].RFAddress == c.RFAddress {
				cp.Devices[i].RoomID = to
			}
		}
	} else {
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name != "" {
			if err := checkName(name); err != nil {
				return nil, err
			}
		} else {
			name = defaultDeviceName(roomName, c.Type, roomDeviceNames(md, to))
		}
		cp.Devices = append(cp.Devices, cube.DeviceMeta{Type: c.Type, RFAddress: c.RFAddress, Serial: c.Serial, Name: name, RoomID: to})
		details["name"], details["room_name"], details["type"] = name, roomName, c.Type.String()
	}
	details["from"] = from
	return &repairAction{name: "list", device: c.RFAddress, metadata: &cp, rooms: []uint8{from, to}, structure: true, details: details}, nil
}

// deviceTypeWords are the device types' words in a default name, as the
// web UI's English table has them (type.<type>).
var deviceTypeWords = map[cube.DeviceType]string{
	cube.DeviceRadiatorThermostat:     "radiator thermostat",
	cube.DeviceRadiatorThermostatPlus: "radiator thermostat plus",
	cube.DeviceWallThermostat:         "wall thermostat",
	cube.DeviceShutterContact:         "window contact",
	cube.DeviceEcoSwitch:              "eco button",
}

// roomDeviceNames are the names of the devices the metadata lists in a
// room.
func roomDeviceNames(md *cube.Metadata, room uint8) []string {
	var out []string
	for _, d := range md.Devices {
		if d.RoomID == room {
			out = append(out, d.Name)
		}
	}
	return out
}

// defaultDeviceName is the name a device listed into a room (D185), or
// put into one (D200-A), gets where the request names none: "<room
// name> <type> <n>", the type in English (deviceTypeWords), n the
// lowest number from 1 that no name of the room's (taken) has in that
// form, which a name matches whatever its case. Where the whole would
// pass maxNameBytes, the room's name is dropped, "<type> <n>", n the
// lowest free in that form: nobody is there to shorten it, as the web
// UI asks its user to (D201).
func defaultDeviceName(room string, typ cube.DeviceType, taken []string) string {
	word, ok := deviceTypeWords[typ]
	if !ok {
		word = strings.ReplaceAll(typ.String(), "_", " ")
	}
	if room = strings.TrimSpace(room); room != "" {
		prefix := room + " " + word
		if name := prefix + " " + strconv.Itoa(freeNumber(prefix, taken)); len(name) <= maxNameBytes {
			return name
		}
	}
	return word + " " + strconv.Itoa(freeNumber(word, taken))
}

// freeNumber is the lowest number from 1 that no name of taken has in
// the form "<prefix> <n>", whatever its case.
func freeNumber(prefix string, taken []string) int {
	used := map[int]bool{}
	for _, name := range taken {
		name = strings.TrimSpace(name)
		if len(name) <= len(prefix)+1 || !strings.EqualFold(name[:len(prefix)], prefix) || name[len(prefix)] != ' ' {
			continue
		}
		rest := name[len(prefix)+1:]
		if strings.Trim(rest, "0123456789") != "" {
			continue
		}
		if n, err := strconv.Atoi(rest); err == nil && n > 0 {
			used[n] = true
		}
	}
	n := 1
	for used[n] {
		n++
	}
	return n
}
