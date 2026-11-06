// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// MaxPairingTime is the longest inclusion the API and the proxy ask for;
// the Cube may end the mode earlier on its own.
const MaxPairingTime = 10 * time.Minute

// StartPairing puts the Cube into inclusion mode for timeout: it marks
// the mode in the client (cubelan.Client.StartInclusion), which then
// delivers the N lines, and writes n:<seconds> through the queue. It
// returns the watch of the N lines (cubelan.Client.WatchInclusion),
// registered before n: is written so that no line is missed, and the
// function that ends the watch. It fails with
// cubelan.ErrInclusionActive while pairing is already running.
func (w Writer) StartPairing(ctx context.Context, timeout time.Duration) (<-chan cube.Message, func(), error) {
	if timeout < time.Second || timeout > MaxPairingTime {
		return nil, nil, fmt.Errorf("pairing time must be 1 s to %s", MaxPairingTime)
	}
	if err := w.Client.StartInclusion(timeout); err != nil {
		return nil, nil, err
	}
	lines, stop := w.Client.WatchInclusion()
	res := w.Do(ctx, engine.Command{Key: "pairing", Priority: engine.PriorityHigh, Lines: []cube.Message{cube.CmdInclusion(timeout)}})
	if res.Err != nil {
		stop()
		w.Client.EndInclusion()
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

// AddDevice puts a paired device that is in no room yet into room as
// the original software does after pairing: after a wakeup of the room,
// an 0x20 add-link-partner radiogram for every pair of the new device and
// a room member it links with, in both directions, then 0x22 set group;
// then the metadata write that lists it with its name, and a re-read of
// its C line. A device whose C line names another room (paired long ago,
// never listed) is first unlinked from that room as in a move. members
// are the devices the metadata places in room; md is the current
// metadata; the devices in skip, the hidden ones, are no partners.
func (w Writer) AddDevice(ctx context.Context, md *cube.Metadata, c *cube.DeviceConfig, name string, room uint8, skip view.Hidden) (engine.Result, error) {
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
	for _, cmd := range cmds {
		if res := w.Do(ctx, cmd); res.Err != nil {
			return res, nil
		}
	}
	cp := *md
	cp.Devices = append(append([]cube.DeviceMeta(nil), md.Devices...), cube.DeviceMeta{Type: c.Type, RFAddress: c.RFAddress, Serial: c.Serial, Name: name, RoomID: room})
	cmd, err := MetadataCommand(&cp)
	if err != nil {
		return engine.Result{}, err
	}
	return w.Do(ctx, cmd, c.RFAddress), nil
}

// Copied says what a copy of a room's settings sent: the devices that
// got the room's temperatures (0x11), whether the device got its valve
// settings (0x12), and how many days of the week programme (0x10);
// ModeNotSet, that the Cube discarded the 0x40 that sets auto.
type Copied struct {
	Temperatures []cube.RFAddress `json:"temperatures"`
	Valve        bool             `json:"valve"`
	Days         int              `json:"days"`
	ModeNotSet   bool             `json:"mode_not_set,omitempty"`
}

// Empty reports whether nothing of the room's settings was sent.
func (c Copied) Empty() bool { return len(c.Temperatures) == 0 && !c.Valve && c.Days == 0 }

func (c *Copied) add(o Copied) {
	c.Temperatures = append(c.Temperatures, o.Temperatures...)
	c.Valve = c.Valve || o.Valve
	c.Days += o.Days
}

// CopyStep is one command of a copy of a room's settings and what it
// copies. Mode marks the 0x40, whose discard ends nothing.
type CopyStep struct {
	Command engine.Command
	Copies  Copied
	Mode    bool
}

// CopyCommands builds the commands that give a thermostat put into room
// the room's settings, in the original software's order after pairing
// and after a move (docs/portal-protocol.md, "Pairing a device, as the
// original software does it", steps 4 to 7): 0x40 auto with temperature
// 0 to the device, directly as in the original software's log, without
// the error reset and the retry, since the Cube discarded such a 0x40
// to a device in a room on 2026-11-04; after a wakeup of the room, 0x11
// with the room's temperatures to every thermostat of the room, the
// device included,
// each keeping its own temperature offset; after a wakeup, 0x12 with the
// room's boost, descaling and valve settings to the device; after a
// wakeup, one 0x10 per day of the room's week programme that differs
// from the device's own, then 0x11 to the device once more, and no
// command at all when no day differs. The room's thermostats are those
// the metadata places in it, and the device, but not those in skip, the
// hidden ones; its settings are those of its first other thermostat
// (roomReference). A room without another thermostat has nothing to
// copy, so the device gets 0x40 alone; a shutter contact or an eco
// switch gets nothing. Each command's key names the device, so that
// the copies of two devices put into one room do not supersede each
// other.
func CopyCommands(snap *cubelan.Snapshot, rf cube.RFAddress, typ cube.DeviceType, room uint8, skip view.Hidden) ([]CopyStep, error) {
	if !isThermostat(typ) {
		return nil, nil
	}
	mode, err := cube.CmdSetTemperature(cube.Target{RFAddress: rf}, 0, cube.ModeAuto, cube.Until{})
	if err != nil {
		return nil, err
	}
	steps := []CopyStep{{Command: engine.Command{Key: "copy-mode:" + rf.String(), Lines: []cube.Message{mode}}, Mode: true}}
	ref, ok := roomReference(snap, room, rf, skip)
	if !ok {
		return steps, nil
	}
	temps := func(dev cube.RFAddress) (cube.Message, error) {
		return cube.CmdSetTemperatures(cube.Target{RFAddress: dev}, cube.TemperatureSettings{Comfort: ref.ComfortTemp, Eco: ref.EcoTemp,
			Max: ref.MaxSetpoint, Min: ref.MinSetpoint, Offset: ownOffset(snap.Config(dev)), WindowOpen: ref.WindowOpenTemp,
			WindowOpenMinutes: ref.WindowOpenMins})
	}
	var lines []cube.Message
	var copied Copied
	radiators, walls := roomMembers(snap, room, rf, skip)
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
	valve, err := cube.CmdSetValveConfig(cube.Target{RFAddress: rf}, cube.ValveSettings{BoostMinutes: ref.BoostMinutes, BoostValvePct: ref.BoostValvePct,
		DecalcDay: ref.DecalcDay, DecalcHour: ref.DecalcHour, MaxValvePct: ref.MaxValvePct, ValveOffsetPct: ref.ValveOffsetPct})
	if err != nil {
		return nil, fmt.Errorf("the room's valve settings: %w", err)
	}
	steps = append(steps, CopyStep{Command: engine.Command{Key: "copy-valve:" + rf.String(), Target: rf, RoomID: room, Wakeup: true,
		Lines: []cube.Message{valve}}, Copies: Copied{Valve: true}})
	own := ownProgram(snap.Config(rf))
	var prog []cube.Message
	days := 0
	for _, d := range copyDayOrder {
		if len(ref.WeekProgram[d]) == 0 || sameDay(ref.WeekProgram[d], own[d]) {
			continue
		}
		ls, err := cube.CmdSetDayProgram(cube.Target{RFAddress: rf}, d, ref.WeekProgram[d])
		if err != nil {
			return nil, fmt.Errorf("the room's programme for %s: %w", d, err)
		}
		prog = append(prog, ls...)
		days++
	}
	if days > 0 {
		l, err := temps(rf)
		if err != nil {
			return nil, fmt.Errorf("the room's temperatures for %s: %w", rf, err)
		}
		steps = append(steps, CopyStep{Command: engine.Command{Key: "copy-program:" + rf.String(), Target: rf, RoomID: room, Wakeup: true,
			Lines: append(prog, l)}, Copies: Copied{Days: days}})
	}
	return steps, nil
}

// copyDayOrder is the order the original software sends a week
// programme's days in: Monday to Friday, then Saturday and Sunday.
var copyDayOrder = []cube.Weekday{cube.Monday, cube.Tuesday, cube.Wednesday, cube.Thursday, cube.Friday, cube.Saturday, cube.Sunday}

// roomMembers lists the thermostats the metadata places in room, and
// rf, that the Cube sent a configuration for, without those in skip:
// radiator thermostats first, then wall thermostats, in the Cube's
// order.
func roomMembers(snap *cubelan.Snapshot, room uint8, rf cube.RFAddress, skip view.Hidden) (radiators, walls []cube.RFAddress) {
	in := map[cube.RFAddress]bool{rf: true}
	if md := snap.Metadata(); md != nil {
		for _, d := range md.Devices {
			if d.RoomID == room {
				in[d.RFAddress] = true
			}
		}
	}
	for _, dev := range snap.Devices() {
		c := snap.Config(dev)
		switch {
		case !in[dev] || c == nil || skip[dev] && dev != rf:
		case c.Thermostat != nil:
			radiators = append(radiators, dev)
		case c.Wall != nil:
			walls = append(walls, dev)
		}
	}
	return radiators, walls
}

// roomReference returns the settings of the room's first thermostat
// other than rf (roomMembers' order), which the copy gives the device,
// and false when the room has no other thermostat. A wall thermostat
// does not store the window-open minutes, descaling and valve limits;
// they then come from RoomRadiator.
func roomReference(snap *cubelan.Snapshot, room uint8, rf cube.RFAddress, skip view.Hidden) (cube.ThermostatConfig, bool) {
	radiators, walls := roomMembers(snap, room, rf, skip)
	for _, dev := range append(radiators, walls...) {
		if dev == rf {
			continue
		}
		c := snap.Config(dev) // read again: a new greeting may have dropped it meanwhile
		switch {
		case c == nil || c.Thermostat == nil && c.Wall == nil:
			continue
		case c.Thermostat != nil:
			return *c.Thermostat, true
		}
		w := c.Wall
		t := RoomRadiator(snap, room, rf)
		t.ComfortTemp, t.EcoTemp, t.MaxSetpoint, t.MinSetpoint = w.ComfortTemp, w.EcoTemp, w.MaxSetpoint, w.MinSetpoint
		t.WindowOpenTemp, t.BoostMinutes, t.BoostValvePct = w.WindowOpenTemp, w.BoostMinutes, w.BoostValvePct
		t.WeekProgram = w.WeekProgram
		return t, true
	}
	return cube.ThermostatConfig{}, false
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

// CopySettings gives a device the settings of the room it was just put
// into (CopyCommands, skip the hidden devices), after the metadata that
// lists it there and the re-read of its C line. The commands run in
// order and the first that fails ends the copy, but for a discarded
// 0x40, which only sets ModeNotSet: Copied says what was sent until
// then, and the result is that of the last command run. The error is
// set only when the room's settings cannot be encoded.
func (w Writer) CopySettings(ctx context.Context, rf cube.RFAddress, typ cube.DeviceType, room uint8, skip view.Hidden) (Copied, engine.Result, error) {
	steps, err := CopyCommands(w.Client.Snapshot(), rf, typ, room, skip)
	if err != nil {
		return Copied{}, engine.Result{}, err
	}
	var done Copied
	var res engine.Result
	for _, st := range steps {
		res = w.Do(ctx, st.Command)
		if st.Mode && errors.Is(res.Err, engine.ErrDiscarded) {
			done.ModeNotSet = true
			res.Err = nil
			continue
		}
		if res.Err != nil {
			return done, res, nil
		}
		done.add(st.Copies)
	}
	return done, res, nil
}
