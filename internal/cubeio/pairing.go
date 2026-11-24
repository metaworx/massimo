// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/roomorder"
	"go.metaworx.ch/massimo/internal/view"
)

// MaxPairingTime is the longest inclusion the API and the proxy ask for,
// the longest a Cube keeps (cube.MaxInclusion); the Cube may end the
// mode earlier on its own.
const MaxPairingTime = cube.MaxInclusion

// StartPairing puts the Cube into inclusion mode for timeout: it marks
// the mode in the client (cubelan.Client.StartInclusion), which then
// delivers the N lines, and writes n:<seconds> through the queue. It
// returns the watch of the N lines (cubelan.Client.WatchInclusion),
// registered before n: is written so that no line is missed, and the
// function that ends the watch. It fails with
// cubelan.ErrInclusionActive while pairing is already running.
func (w Writer) StartPairing(ctx context.Context, timeout time.Duration) (<-chan cube.Message, func(), error) {
	if timeout < time.Second || timeout > MaxPairingTime {
		return nil, nil, fmt.Errorf("pairing time must be 1 to %d s", int(MaxPairingTime.Seconds()))
	}
	if err := w.Client.StartInclusion(timeout); err != nil {
		return nil, nil, err
	}
	lines, stop := w.Client.WatchInclusion()
	res, withdrawn := w.run(ctx, engine.Command{Key: "pairing", Priority: engine.PriorityHigh, Lines: []cube.Message{cube.CmdInclusion(timeout)}})
	if res.Err != nil {
		stop()
		// Ended in the client only where the n: does not reach the Cube:
		// taken out of the queue, or failed. One that was sent by the time
		// the caller gave up keeps the inclusion, which its timer ends.
		if withdrawn || ctx.Err() == nil {
			w.Client.EndInclusion()
		}
		return nil, nil, res.Err
	}
	return lines, stop, nil
}

// stopWait is how long StopPairing waits for the Cube's own end of
// inclusion mode (an empty N line) before ending it in the client.
const stopWait = 2 * time.Second

// StopPairing ends inclusion mode with x:. It waits up to two seconds
// for the Cube's empty N line, so that watchers get it, then counts the
// mode as ended in any case.
func (w Writer) StopPairing(ctx context.Context) error {
	lines, stop := w.Client.WatchInclusion()
	defer stop()
	res := w.Do(ctx, engine.Command{Key: "pairing", Priority: engine.PriorityHigh, Lines: []cube.Message{cube.CmdCancelInclusion}})
	if res.Err == nil {
		wait := time.NewTimer(stopWait)
		defer wait.Stop()
	drain:
		for {
			select {
			case _, ok := <-lines:
				if !ok {
					break drain
				}
			case <-wait.C:
				break drain
			case <-ctx.Done():
				break drain
			}
		}
	}
	w.Client.EndInclusion()
	return res.Err
}

// AddDevice puts a paired device that is in no room yet into room with
// the placement's link half (linkIntoRoom), as the original software
// placed one in its capture of 2026-11-04: after a wakeup of the room,
// one 0x20 add-link-partner radiogram with the room flag naming the
// device, to the room's addressee, which every member of the room
// takes, one to each window contact among its partners, and one to the
// device for each partner, then 0x22 set group; then the metadata write
// that lists it with its name, and a re-read of its C line. A device
// whose C line names another room (paired long ago, never listed) is
// first unlinked from that room as in a move. members are the devices
// the metadata places in room; md is the current metadata; the devices
// in skip, the hidden ones, are no partners. The result is that of the
// first command that failed, else of the metadata write; reached says
// that a radiogram of the link half may have reached the room's members
// (Reached), whatever became of the rest: they may take the device's
// state then.
func (w Writer) AddDevice(ctx context.Context, md *cube.Metadata, c *cube.DeviceConfig, name string, room uint8, skip view.Hidden) (res engine.Result, reached bool, err error) {
	var members []cube.DeviceMeta
	for _, d := range md.Devices {
		if d.RoomID == room {
			members = append(members, d)
		}
	}
	out, in := MoveCommands(w.Client.Snapshot(), c.RFAddress, c.Type, c.RoomID, room, members, skip)
	cmds := []engine.Command{in}
	if c.RoomID != 0 {
		cmds = []engine.Command{out, in}
	}
	for i, cmd := range cmds {
		res := w.Do(ctx, cmd)
		if i == len(cmds)-1 { // the link half
			reached = Reached(cmd, res)
		}
		if res.Err != nil {
			return res, reached, nil
		}
	}
	cp := *md
	cp.Devices = append(append([]cube.DeviceMeta(nil), md.Devices...), cube.DeviceMeta{Type: c.Type, RFAddress: c.RFAddress, Serial: c.Serial, Name: name, RoomID: room})
	cmd, err := MetadataCommand(&cp)
	if err != nil {
		return engine.Result{}, reached, err
	}
	return w.Do(ctx, cmd, c.RFAddress), reached, nil
}

// Reached reports whether a radiogram of cmd may have reached a device,
// by res, the queue's result of it: an s: line the Cube accepted, or one
// the queue began to send whose answer did not come (engine.Result.Sent),
// whose fate is unknown; so is every line's where the caller's context
// ended while it waited for the command (context.Canceled,
// DeadlineExceeded), which the queue may still be sending. A line the
// Cube discarded, and one never sent, is none.
func Reached(cmd engine.Command, res engine.Result) bool {
	if errors.Is(res.Err, context.Canceled) || errors.Is(res.Err, context.DeadlineExceeded) {
		return true // the wait ended, not the command: its fate is unknown
	}
	for i := 0; i < res.Sent && i < len(cmd.Lines); i++ {
		if cmd.Lines[i].Type != 's' {
			continue
		}
		if i >= len(res.Replies) {
			return true // sent, not answered
		}
		if sr, err := cube.ParseSendResult(res.Replies[i].Data); err != nil || !sr.Discarded {
			return true
		}
	}
	return false
}

// RoomState is the state a copy gives a thermostat put into a room: the
// room's state from before the change, which the server reads before it
// links the device (AP RoomMajority, D8-A): its mode, its setpoint, 0
// with auto for the week programme, and a holiday's end. The zero value
// is auto on the programme, the original software's 0x40.
type RoomState struct {
	Mode  cube.Mode
	Temp  float64
	Until cube.Until
}

// MarshalJSON writes the state as the API names one: its mode, its
// setpoint where it has one (none for auto on the programme) and its
// holiday's end.
func (s RoomState) MarshalJSON() ([]byte, error) {
	out := struct {
		Mode     cube.Mode   `json:"mode"`
		Setpoint *float64    `json:"setpoint,omitempty"`
		Until    *cube.Until `json:"until,omitempty"`
	}{Mode: s.Mode}
	if s.Temp != 0 {
		out.Setpoint = &s.Temp
	}
	if !s.Until.IsZero() {
		out.Until = &s.Until
	}
	return json.Marshal(out)
}

// Where the state a copy's 0x40 carried came from (Copied.StateFrom).
const (
	// StateOfRoom: the room's state from before the change, or Massimo's
	// setpoint to the room since.
	StateOfRoom = "room"
	// StateDefault: the room had none, nor another thermostat; the 0x40
	// carried auto on the programme, as the original software's does.
	StateDefault = "default"
)

// Why a copy sent no configuration (Copied.ConfigSkipped).
const (
	// SkipNoOther: the room has no other thermostat.
	SkipNoOther = "no_other_thermostat"
	// SkipNoState: the Cube holds no valid state for any other thermostat
	// of the room (RoomMajority.D9).
	SkipNoState = "no_valid_state"
)

// Copied says what a copy of a room's settings sent: State, the state
// the 0x40 carried, and StateFrom where it came from (StateOfRoom,
// StateDefault); the devices that got the room's temperatures (0x11), an
// empty list for none, whether the device got its valve settings (0x12),
// and how many days of the week programme (0x10); ConfigSkipped, why no
// configuration was sent (SkipNoOther, SkipNoState), empty where it was;
// ModeNotSet, that the Cube discarded the 0x40; RoomMode, that the room's
// state then went to the room as a room command, which the Cube took
// (RoomMajority.D16-A).
type Copied struct {
	State         RoomState        `json:"state"`
	StateFrom     string           `json:"state_from"`
	Temperatures  []cube.RFAddress `json:"temperatures"`
	Valve         bool             `json:"valve"`
	Days          int              `json:"days"`
	ConfigSkipped string           `json:"config_skipped,omitempty"`
	ModeNotSet    bool             `json:"mode_not_set,omitempty"`
	RoomMode      bool             `json:"room_mode,omitempty"`
}

func (c *Copied) add(o Copied) {
	c.Temperatures = append(c.Temperatures, o.Temperatures...)
	c.Valve = c.Valve || o.Valve
	c.Days += o.Days
}

// CopyStep is one command of a copy of a room's settings and what it
// copies. Mode marks the device's 0x40, whose discard ends nothing;
// RoomMode the room command that runs only after such a discard.
type CopyStep struct {
	Command  engine.Command
	Copies   Copied
	Mode     bool
	RoomMode bool
}

// CopyCommands builds the commands that give a thermostat put into room
// the room's state and configuration, in the original software's order
// after pairing and after a move (docs/cube-protocol.md, "Pairing a
// device, as the original software does it", steps 4 to 7), with the
// room's own state where the original sends auto (AP RoomMajority,
// Block 2):
//  1. 0x40 to the device directly, as in the original software's log,
//     without a Target, since the Cube discarded such a 0x40 with one to
//     a device in a room on 2026-11-04 (the queue resets the device
//     first, and retries a discard, only where its error flag is set):
//     state's mode, setpoint and until, the room's state from before the
//     change; auto with temperature 0, the programme, where state is nil
//     or the room has no other thermostat;
//  2. after a wakeup of the room, 0x11 with the room's temperatures and
//     window-open minutes (RoomConfig) to the device and every thermostat
//     of the room the Cube holds a valid state for (Dead), each keeping
//     its own temperature offset: a dead one, whose configuration did not
//     count, is not given the others';
//  3. after a wakeup, 0x12 with the room's boost, descaling and valve
//     settings to the device;
//  4. after a wakeup, one 0x10 per day of the room's week programme that
//     differs from the device's own, then 0x11 to the device once more,
//     and no command at all when no day differs;
//  5. state once more, as a room command to the room's addressee
//     (RoomAddressee), which CopySettings runs only where the Cube
//     discarded the 0x40 of step 1 (RoomMajority.D16-A); none where state
//     is nil or the room has no other thermostat.
//
// The room's thermostats are those the metadata places in it, and the
// device, but not those in skip, the hidden ones. A room without another
// thermostat the Cube holds a valid state for has no configuration to
// copy (RoomConfig), and the device gets the 0x40 alone, the 0x40's step
// saying why (Copied.ConfigSkipped); a shutter contact or an eco switch
// gets nothing. Each command's key names the
// device, so that the copies of two devices put into one room do not
// supersede each other.
func CopyCommands(snap *cubelan.Snapshot, rf cube.RFAddress, typ cube.DeviceType, room uint8, skip view.Hidden, state *RoomState) ([]CopyStep, error) {
	if !typ.IsThermostat() {
		return nil, nil
	}
	members := roomMembers(snap, room, rf, skip)
	others := slices.ContainsFunc(members, func(d cube.DeviceMeta) bool { return d.RFAddress != rf })
	if !others {
		state = nil // no other thermostat: nothing speaks for the room
	}
	var st RoomState
	if state != nil {
		st = *state
	}
	mode, err := cube.CmdSetTemperature(cube.Target{RFAddress: rf}, st.Temp, st.Mode, st.Until)
	if err != nil {
		return nil, fmt.Errorf("the room's state for %s: %w", rf, err)
	}
	from := StateDefault
	if state != nil {
		from = StateOfRoom
	}
	steps := []CopyStep{{Command: engine.Command{Key: "copy-mode:" + rf.String(), Lines: []cube.Message{mode}}, Copies: Copied{State: st, StateFrom: from}, Mode: true}}
	switch ref, ok := RoomConfig(snap, RoomMaster(snap, room), members, rf, nil); {
	case ok:
		config, err := configSteps(snap, rf, room, members, ref)
		if err != nil {
			return nil, err
		}
		steps = append(steps, config...)
	case others:
		steps[0].Copies.ConfigSkipped = SkipNoState
	default:
		steps[0].Copies.ConfigSkipped = SkipNoOther
	}
	if state == nil {
		return steps, nil
	}
	if addr, ok := RoomAddressee(snap, room, skip); ok {
		m, err := cube.CmdSetTemperature(cube.Target{RFAddress: addr, RoomID: room, Room: true}, st.Temp, st.Mode, st.Until)
		if err != nil {
			return nil, fmt.Errorf("the room's state for room %d: %w", room, err)
		}
		steps = append(steps, CopyStep{Command: engine.Command{Key: "copy-room-mode:" + rf.String(), Lines: []cube.Message{m}}, RoomMode: true})
	}
	return steps, nil
}

// configSteps are steps 2 to 4 of CopyCommands: the room's
// configuration ref (RoomConfig) for rf, put into room, whose
// thermostats are members; the 0x11 goes to rf and to those of them the
// Cube holds a valid state for.
func configSteps(snap *cubelan.Snapshot, rf cube.RFAddress, room uint8, members []cube.DeviceMeta, ref RoomSettings) ([]CopyStep, error) {
	temps := func(dev cube.RFAddress) (cube.Message, error) {
		t := ref.Temps
		return cube.CmdSetTemperatures(cube.Target{RFAddress: dev}, cube.TemperatureSettings{Comfort: t.Comfort, Eco: t.Eco, Max: t.Max, Min: t.Min,
			Offset: ownOffset(snap.Config(dev)), WindowOpen: t.WindowOpen, WindowOpenMinutes: ref.WindowOpenMins})
	}
	var steps []CopyStep
	var lines []cube.Message
	var copied Copied
	var radiators, walls []cube.RFAddress // the original software's order: radiator thermostats first
	for _, d := range members {
		switch {
		case d.RFAddress != rf && roomorder.Dead(snap, d.RFAddress): // its own configuration did not count
		case d.Type == cube.DeviceWallThermostat:
			walls = append(walls, d.RFAddress)
		default:
			radiators = append(radiators, d.RFAddress)
		}
	}
	for _, dev := range append(radiators, walls...) {
		l, err := temps(dev)
		if err != nil {
			return nil, fmt.Errorf("the room's temperatures for %s: %w", dev, err)
		}
		lines = append(lines, l)
		copied.Temperatures = append(copied.Temperatures, dev)
	}
	if len(lines) > 0 {
		steps = append(steps, CopyStep{Command: engine.Command{Key: "copy-temps:" + rf.String(), RoomID: room, Wakeup: true, Lines: lines}, Copies: copied})
	}
	valve, err := cube.CmdSetValveConfig(cube.Target{RFAddress: rf}, ref.Valve)
	if err != nil {
		return nil, fmt.Errorf("the room's valve settings: %w", err)
	}
	steps = append(steps, CopyStep{Command: engine.Command{Key: "copy-valve:" + rf.String(), Target: rf, RoomID: room, Wakeup: true,
		Lines: []cube.Message{valve}}, Copies: Copied{Valve: true}})
	days := ref.Days(snap.Config(rf))
	if len(days) == 0 {
		return steps, nil
	}
	var prog []cube.Message
	for _, d := range days {
		ls, err := cube.CmdSetDayProgram(cube.Target{RFAddress: rf}, d, ref.Program[d])
		if err != nil {
			return nil, fmt.Errorf("the room's programme for %s: %w", d, err)
		}
		prog = append(prog, ls...)
	}
	l, err := temps(rf)
	if err != nil {
		return nil, fmt.Errorf("the room's temperatures for %s: %w", rf, err)
	}
	return append(steps, CopyStep{Command: engine.Command{Key: "copy-program:" + rf.String(), Target: rf, RoomID: room, Wakeup: true,
		Lines: append(prog, l)}, Copies: Copied{Days: len(days)}}), nil
}

// copyDayOrder is the order the original software sends a week
// programme's days in: Monday to Friday, then Saturday and Sunday.
var copyDayOrder = []cube.Weekday{cube.Monday, cube.Tuesday, cube.Wednesday, cube.Thursday, cube.Friday, cube.Saturday, cube.Sunday}

// roomMembers lists the thermostats the metadata places in room, and
// rf, that the Cube sent a configuration for, without those in skip but
// rf, in the Cube's order, each with the type of its configuration.
func roomMembers(snap *cubelan.Snapshot, room uint8, rf cube.RFAddress, skip view.Hidden) []cube.DeviceMeta {
	in := map[cube.RFAddress]bool{rf: true}
	if md := snap.Metadata(); md != nil {
		for _, d := range md.Devices {
			if d.RoomID == room {
				in[d.RFAddress] = true
			}
		}
	}
	var out []cube.DeviceMeta
	for _, dev := range snap.Devices() {
		c := snap.Config(dev)
		if !in[dev] || c == nil || skip[dev] && dev != rf || c.Thermostat == nil && c.Wall == nil {
			continue
		}
		out = append(out, cube.DeviceMeta{Type: c.Type, RFAddress: dev, RoomID: room})
	}
	return out
}

// RoomSettings is a room's configuration as RoomConfig decides it, each
// group on its own (AP RoomMajority, D5-A, D9): Temps, the five
// temperatures of 0x11 that every thermostat stores; WindowOpenMins, the
// minutes of 0x11, and Valve, the boost, descaling and valve settings of
// 0x12, which only radiator thermostats store in full; Program, each day
// of the week programme (0x10), empty for a day none of them has. Others
// counts the thermostats it was decided over, TempsBy how many of them
// share Temps, and DaysBy, by weekday, how many share each day.
type RoomSettings struct {
	Others         int
	Temps          Temperatures
	TempsBy        int
	WindowOpenMins int
	Valve          cube.ValveSettings
	Program        cube.WeekProgram
	DaysBy         [7]int
}

// RoomConfig decides a room's configuration (AP RoomMajority, D9 and
// D5-A), the one rule the copy (CopyCommands), the repair
// (RoomAgreement) and the server's report of members that differ take.
// It is decided over the thermostats among members but leave that the
// Cube holds a valid state for (roomorder.Of, the dead passed over), by
// their configurations, config's (snap.Config where config is nil),
// each group on its own: the temperatures, the valve settings and each
// day of the week programme are what more than half of those that have
// the group share, else the group of the first of them in the room's
// order that has it: the room's master where it is one of them, its
// wall thermostat, its radiator thermostats in members' order. The
// window-open minutes and the valve settings are decided so among the
// radiator thermostats alone, which alone store them; without one, the
// valve settings are a wall thermostat's boost settings with the
// original software's descaling and valve limits, and the minutes the
// original software's (radiatorDefaults). Each device keeps its own
// temperature offset and display mode, which are not the room's. False
// where no thermostat among members but leave has a configuration and
// is not dead.
func RoomConfig(snap *cubelan.Snapshot, master cube.RFAddress, members []cube.DeviceMeta, leave cube.RFAddress, config func(cube.RFAddress) *cube.DeviceConfig) (RoomSettings, bool) {
	if config == nil {
		config = snap.Config
	}
	var all, radiators []*cube.DeviceConfig // in the room's order
	for _, rf := range roomorder.Of(snap, master, members, leave) {
		switch c := config(rf); {
		case c == nil:
		case c.Thermostat != nil:
			all, radiators = append(all, c), append(radiators, c)
		case c.Wall != nil:
			all = append(all, c)
		}
	}
	if len(all) == 0 {
		return RoomSettings{}, false
	}
	every := func(int) bool { return true }
	s := RoomSettings{Others: len(all)}
	i, n := decide(len(all), every, func(i, j int) bool { return temperatures(all[i]) == temperatures(all[j]) })
	s.Temps, s.TempsBy = temperatures(all[i]), n
	for d := range s.Program {
		day := func(i int) cube.DayProgram { return ownProgram(all[i])[d] }
		if i, n := decide(len(all), func(i int) bool { return len(day(i)) > 0 }, func(i, j int) bool { return sameDay(day(i), day(j)) }); i >= 0 {
			s.Program[d], s.DaysBy[d] = day(i), n
		}
	}
	if len(radiators) == 0 {
		// Only wall thermostats: their boost settings, the rest the
		// original software's.
		boost := func(i int) [2]int { return [2]int{all[i].Wall.BoostMinutes, all[i].Wall.BoostValvePct} }
		i, _ := decide(len(all), every, func(i, j int) bool { return boost(i) == boost(j) })
		d := radiatorDefaults
		s.WindowOpenMins = d.WindowOpenMins
		s.Valve = cube.ValveSettings{BoostMinutes: boost(i)[0], BoostValvePct: boost(i)[1], DecalcDay: d.DecalcDay, DecalcHour: d.DecalcHour,
			MaxValvePct: d.MaxValvePct, ValveOffsetPct: d.ValveOffsetPct}
		return s, true
	}
	valve := func(i int) cube.ValveSettings {
		t := radiators[i].Thermostat
		return cube.ValveSettings{BoostMinutes: t.BoostMinutes, BoostValvePct: t.BoostValvePct, DecalcDay: t.DecalcDay, DecalcHour: t.DecalcHour,
			MaxValvePct: t.MaxValvePct, ValveOffsetPct: t.ValveOffsetPct}
	}
	i, _ = decide(len(radiators), every, func(i, j int) bool { return valve(i) == valve(j) })
	s.Valve = valve(i)
	minutes := func(i int) int { return radiators[i].Thermostat.WindowOpenMins }
	i, _ = decide(len(radiators), every, func(i, j int) bool { return minutes(i) == minutes(j) })
	s.WindowOpenMins = minutes(i)
	return s, true
}

// decide picks one of n values in the room's order (RoomMajority.D9):
// the one more than half of those that have one (has) share (same),
// else the first that has one. It returns its index and how many share
// it; -1 where none has one.
func decide(n int, has func(i int) bool, same func(i, j int) bool) (int, int) {
	shared := func(i int) int {
		c := 0
		for j := 0; j < n; j++ {
			if has(j) && same(i, j) {
				c++
			}
		}
		return c
	}
	first, total := -1, 0
	for i := 0; i < n; i++ {
		if has(i) {
			total++
			if first < 0 {
				first = i
			}
		}
	}
	if first < 0 {
		return -1, 0
	}
	for i := first; i < n; i++ {
		if has(i) {
			if c := shared(i); 2*c > total {
				return i, c
			}
		}
	}
	return first, shared(first)
}

// Days lists the days of c's week programme that differ from the
// room's, in the order the original software sends a programme
// (copyDayOrder); a day the room has none of is none.
func (s RoomSettings) Days(c *cube.DeviceConfig) []cube.Weekday {
	own := ownProgram(c)
	var out []cube.Weekday
	for _, d := range copyDayOrder {
		if len(s.Program[d]) > 0 && !sameDay(s.Program[d], own[d]) {
			out = append(out, d)
		}
	}
	return out
}

// SameTemperatures reports whether c, a thermostat's configuration, has
// the room's five temperatures (Temperatures).
func (s RoomSettings) SameTemperatures(c *cube.DeviceConfig) bool {
	t, ok := temperaturesOf(c)
	return ok && t == s.Temps
}

// ProgramBy is how many of the thermostats the room's week programme was
// decided over share it, the fewest of any day of it.
func (s RoomSettings) ProgramBy() int {
	n := 0
	for d, by := range s.DaysBy {
		if len(s.Program[d]) > 0 && (n == 0 || by < n) {
			n = by
		}
	}
	return n
}

// ownOffset is a thermostat's temperature offset, 0 when its
// configuration is unknown (a factory-fresh device's).
func ownOffset(c *cube.DeviceConfig) float64 {
	switch {
	case c == nil:
	case c.Thermostat != nil:
		return c.Thermostat.TempOffset
	case c.Wall != nil:
		return c.Wall.TempOffset
	}
	return 0
}

// ownProgram is a thermostat's week programme, empty when its
// configuration is unknown.
func ownProgram(c *cube.DeviceConfig) cube.WeekProgram {
	switch {
	case c == nil:
	case c.Thermostat != nil:
		return c.Thermostat.WeekProgram
	case c.Wall != nil:
		return c.Wall.WeekProgram
	}
	return cube.WeekProgram{}
}

// sameDay reports whether two days have the same switch points.
func sameDay(a, b cube.DayProgram) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// CopySettings gives a device the room's state from before it was put
// into the room (state, nil for none; RoomState) and the room's
// configuration (CopyCommands, skip the hidden devices), after the
// metadata that lists it there and the re-read of its C line. The
// commands run in order and the first that fails ends the copy, but for
// a discarded 0x40, which only sets ModeNotSet and lets the room command
// of state run after the rest (RoomMajority.D16-A), which runs only
// then, and whose own discard ends nothing either, RoomMode then unset:
// Copied says what was sent until then, and the result is that of the
// last command run. The error is set only when the room's settings
// cannot be encoded.
func (w Writer) CopySettings(ctx context.Context, rf cube.RFAddress, typ cube.DeviceType, room uint8, skip view.Hidden, state *RoomState) (Copied, engine.Result, error) {
	steps, err := CopyCommands(w.Client.Snapshot(), rf, typ, room, skip, state)
	if err != nil {
		return Copied{}, engine.Result{}, err
	}
	done := Copied{Temperatures: []cube.RFAddress{}}
	var res engine.Result
	for _, st := range steps {
		if st.RoomMode && !done.ModeNotSet {
			continue
		}
		if st.Mode {
			done.State, done.StateFrom, done.ConfigSkipped = st.Copies.State, st.Copies.StateFrom, st.Copies.ConfigSkipped
		}
		res = w.Do(ctx, st.Command)
		if (st.Mode || st.RoomMode) && errors.Is(res.Err, engine.ErrDiscarded) {
			done.ModeNotSet = true
			res.Err = nil
			continue
		}
		if res.Err != nil {
			return done, res, nil
		}
		done.add(st.Copies)
		done.RoomMode = done.RoomMode || st.RoomMode
	}
	return done, res, nil
}
