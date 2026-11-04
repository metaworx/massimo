// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package backup

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Step is one command of a restore plan with a description for humans.
type Step struct {
	What    string
	Command engine.Command
}

// Plan is what a restore would do.
type Plan struct {
	Steps []Step
	// Notes list differences that cannot be restored automatically, for
	// example devices that still have to be paired.
	Notes []string
}

// Empty reports whether the plan has nothing to send.
func (p *Plan) Empty() bool { return len(p.Steps) == 0 }

// PlanOptions select what a restore writes.
type PlanOptions struct {
	// Portal also restores the portal URL and whether the Cube polls it.
	// Off by default: a backup from the vendor's portal would otherwise
	// send the Cube back there.
	Portal bool
	// Names, Settings, Programs, Clock can switch off the corresponding
	// group. All are restored when the options are zero.
	SkipNames    bool // rooms and device names
	SkipSettings bool // preset temperatures and valve settings
	SkipPrograms bool // week programs
	SkipClock    bool // NTP servers and time zone rules
}

// Current is the live state the plan compares the backup with.
type Current struct {
	Snapshot *cubelan.Snapshot
	NTP      []string // nil when not requested
}

// NewPlan compares the backup with the Cube's current state and lists the
// commands that bring the Cube back to the backup. Devices in the backup
// that are not paired with the Cube are reported in the notes; the Cube's
// pairing cannot be restored over the LAN.
func NewPlan(f *File, cur Current, opts PlanOptions) (*Plan, error) {
	p := &Plan{}
	snap := cur.Snapshot
	h := snap.Hello()
	if h == nil {
		return nil, fmt.Errorf("restore: no greeting from the cube yet")
	}
	if f.Cube.Serial != "" && h.Serial != f.Cube.Serial {
		p.Notes = append(p.Notes, fmt.Sprintf("the backup is from cube %s, this is cube %s", f.Cube.Serial, h.Serial))
	}

	// Which backup devices exist on this Cube?
	known := map[cube.RFAddress]cube.DeviceMeta{}
	if md := snap.Metadata(); md != nil {
		for _, d := range md.Devices {
			known[d.RFAddress] = d
		}
	}
	for _, rf := range snap.Devices() {
		if _, ok := known[rf]; ok || rf == h.RFAddress {
			continue
		}
		if c := snap.Config(rf); c != nil {
			known[rf] = cube.DeviceMeta{Type: c.Type, RFAddress: rf, Serial: c.Serial, RoomID: c.RoomID}
		}
	}
	var present, missing []Device
	for _, d := range f.Devices {
		if _, ok := known[d.RFAddress]; ok {
			present = append(present, d)
		} else {
			missing = append(missing, d)
		}
	}
	for _, d := range missing {
		p.Notes = append(p.Notes, fmt.Sprintf("device %s %q (%s, %s) is not paired with this cube; pair it and restore again", d.RFAddress, d.Name, d.Type, d.Serial))
	}
	for _, rf := range snap.Devices() {
		d, ok := known[rf]
		if !ok {
			continue
		}
		if _, inBackup := f.Device(rf); !inBackup {
			p.Notes = append(p.Notes, fmt.Sprintf("device %s %q (%s) is paired but not in the backup; it is left as it is", rf, d.Name, d.Type))
		}
	}

	if !opts.SkipClock {
		p.planClock(f, cur, h)
	}
	if opts.Portal {
		p.planPortal(f, snap, h)
	}
	if !opts.SkipNames {
		if err := p.planNames(f, snap, present); err != nil {
			return nil, err
		}
	}
	for _, d := range present {
		if err := p.planDevice(f, snap, known[d.RFAddress], d, opts); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (p *Plan) add(what string, cmd engine.Command) {
	if cmd.Priority == 0 {
		cmd.Priority = engine.PriorityLow
	}
	p.Steps = append(p.Steps, Step{What: what, Command: cmd})
}

func (p *Plan) planClock(f *File, cur Current, h *cube.Hello) {
	if len(f.NTP) > 0 && cur.NTP != nil && !slices.Equal(f.NTP, cur.NTP) {
		p.add(fmt.Sprintf("set NTP servers to %s (now %s)", strings.Join(f.NTP, ","), strings.Join(cur.NTP, ",")),
			engine.Command{Key: "restore:ntp", Lines: []cube.Message{cube.CmdSetNTP(f.NTP...)}})
	}
	if f.CubeConfig == nil {
		return
	}
	c := cur.Snapshot.Config(h.RFAddress)
	if c == nil || c.Cube == nil {
		p.Notes = append(p.Notes, "the cube's own configuration (C line) was not received; time zone not compared")
		return
	}
	if f.CubeConfig.Winter != c.Cube.Winter || f.CubeConfig.Summer != c.Cube.Summer {
		m, err := cube.CmdSetTimeZone(f.CubeConfig.Winter, f.CubeConfig.Summer)
		if err != nil {
			p.Notes = append(p.Notes, "time zone rules in the backup are invalid: "+err.Error())
			return
		}
		p.add(fmt.Sprintf("set time zone rules to %s/%s (now %s/%s)", f.CubeConfig.Winter.Name, f.CubeConfig.Summer.Name, c.Cube.Winter.Name, c.Cube.Summer.Name),
			engine.Command{Key: "restore:tz", Lines: []cube.Message{m}})
	}
}

func (p *Plan) planPortal(f *File, snap *cubelan.Snapshot, h *cube.Hello) {
	if f.CubeConfig == nil {
		return
	}
	c := snap.Config(h.RFAddress)
	if c == nil || c.Cube == nil {
		return
	}
	var lines []cube.Message
	var what []string
	if f.CubeConfig.PortalURL != c.Cube.PortalURL {
		lines = append(lines, cube.CmdSetPortalURL(f.CubeConfig.PortalURL))
		what = append(what, fmt.Sprintf("portal URL %q (now %q)", f.CubeConfig.PortalURL, c.Cube.PortalURL))
	}
	if f.CubeConfig.PortalEnabled != c.Cube.PortalEnabled {
		iv := int32(-1)
		if f.CubeConfig.PortalEnabled {
			iv = 0
		}
		lines = append(lines, cube.CmdSetPortalInterval(iv))
		what = append(what, fmt.Sprintf("portal polling %v", f.CubeConfig.PortalEnabled))
	}
	if len(lines) > 0 {
		p.add("set "+strings.Join(what, ", "), engine.Command{Key: "restore:portal", Lines: lines})
	}
}

// planNames rebuilds the metadata: the backup's rooms and names for the
// devices that exist, plus whatever the Cube knows about devices that are
// not in the backup.
func (p *Plan) planNames(f *File, snap *cubelan.Snapshot, present []Device) error {
	cur := snap.Metadata()
	if cur == nil {
		p.Notes = append(p.Notes, "no room/device list (M lines) received from the cube; names not compared")
		return nil
	}
	// Keep the Cube's order of rooms and devices so that an unchanged
	// list encodes to the same bytes.
	backup := map[cube.RFAddress]Device{}
	for _, d := range present {
		backup[d.RFAddress] = d
	}
	want := &cube.Metadata{Header: cur.Header, Trailing: cur.Trailing}
	for _, r := range cur.Rooms {
		if n := f.RoomName(r.ID); n != "" {
			r.Name = n
		}
		want.Rooms = append(want.Rooms, r)
	}
	for _, r := range f.Rooms {
		if !hasRoom(want.Rooms, r.ID) {
			want.Rooms = append(want.Rooms, r)
		}
	}
	seen := map[cube.RFAddress]bool{}
	for _, d := range cur.Devices {
		if b, ok := backup[d.RFAddress]; ok {
			d.Name, d.RoomID = b.Name, b.RoomID
		}
		want.Devices = append(want.Devices, d)
		seen[d.RFAddress] = true
	}
	for _, d := range present {
		if !seen[d.RFAddress] {
			want.Devices = append(want.Devices, cube.DeviceMeta{Type: d.Type, RFAddress: d.RFAddress, Serial: d.Serial, Name: d.Name, RoomID: d.RoomID})
		}
	}
	// Rooms with no device left are dropped, as the original software does.
	used := map[uint8]bool{}
	for _, d := range want.Devices {
		used[d.RoomID] = true
	}
	want.Rooms = slices.DeleteFunc(want.Rooms, func(r cube.RoomMeta) bool { return !used[r.ID] })

	a, err := want.Encode()
	if err != nil {
		return fmt.Errorf("restore: names: %w", err)
	}
	b, err := cur.Encode()
	if err != nil {
		return fmt.Errorf("restore: current names: %w", err)
	}
	if bytes.Equal(a, b) {
		return nil
	}
	cmds, err := cube.WriteMetadataCommands(want)
	if err != nil {
		return err
	}
	var changes []string
	for _, d := range want.Devices {
		for _, c := range cur.Devices {
			if c.RFAddress != d.RFAddress {
				continue
			}
			if c.Name != d.Name {
				changes = append(changes, fmt.Sprintf("%s: %q→%q", d.RFAddress, c.Name, d.Name))
			}
			if c.RoomID != d.RoomID {
				changes = append(changes, fmt.Sprintf("%s: room %d→%d", d.RFAddress, c.RoomID, d.RoomID))
			}
		}
	}
	for _, r := range want.Rooms {
		found := false
		for _, c := range cur.Rooms {
			if c.ID == r.ID {
				found = true
				if c.Name != r.Name {
					changes = append(changes, fmt.Sprintf("room %d: %q→%q", r.ID, c.Name, r.Name))
				}
			}
		}
		if !found {
			changes = append(changes, fmt.Sprintf("room %d: new %q", r.ID, r.Name))
		}
	}
	what := "write room and device names"
	if len(changes) > 0 {
		what += " (" + strings.Join(changes, "; ") + ")"
	}
	p.add(what, engine.Command{Key: "restore:names", Lines: cmds})
	return nil
}

func hasRoom(rooms []cube.RoomMeta, id uint8) bool {
	for _, r := range rooms {
		if r.ID == id {
			return true
		}
	}
	return false
}

// planDevice compares one device's configuration and queues the radio
// commands that differ, after a wakeup of its room.
func (p *Plan) planDevice(f *File, snap *cubelan.Snapshot, cur cube.DeviceMeta, want Device, opts PlanOptions) error {
	if want.Thermostat == nil && want.Wall == nil {
		return nil // nothing configurable (shutter contact, eco switch)
	}
	name := want.Name
	if name == "" {
		name = cur.Name
	}
	c := snap.Config(want.RFAddress)
	if c == nil {
		p.Notes = append(p.Notes, fmt.Sprintf("device %s %q: no configuration (C line) received yet; settings not compared", want.RFAddress, name))
		return nil
	}
	room := c.RoomID
	t := cube.Target{RFAddress: want.RFAddress, RoomID: room}
	var lines []cube.Message
	var what []string

	if !opts.SkipSettings && want.RoomID != room {
		lines = append(lines, cube.CmdSetGroup(want.RFAddress, want.RoomID, false))
		what = append(what, fmt.Sprintf("room %d→%d", room, want.RoomID))
	}

	switch {
	case want.Thermostat != nil && c.Thermostat != nil:
		w, h := want.Thermostat, c.Thermostat
		if !opts.SkipSettings {
			if w.ComfortTemp != h.ComfortTemp || w.EcoTemp != h.EcoTemp || w.MaxSetpoint != h.MaxSetpoint || w.MinSetpoint != h.MinSetpoint ||
				w.TempOffset != h.TempOffset || w.WindowOpenTemp != h.WindowOpenTemp || w.WindowOpenMins != h.WindowOpenMins {
				m, err := cube.CmdSetTemperatures(t, cube.TemperatureSettings{Comfort: w.ComfortTemp, Eco: w.EcoTemp, Max: w.MaxSetpoint, Min: w.MinSetpoint,
					Offset: w.TempOffset, WindowOpen: w.WindowOpenTemp, WindowOpenMinutes: w.WindowOpenMins})
				if err != nil {
					return fmt.Errorf("device %s: temperatures: %w", want.RFAddress, err)
				}
				lines = append(lines, m)
				what = append(what, fmt.Sprintf("temperatures comfort %.1f eco %.1f max %.1f min %.1f offset %+.1f window %.1f/%dmin",
					w.ComfortTemp, w.EcoTemp, w.MaxSetpoint, w.MinSetpoint, w.TempOffset, w.WindowOpenTemp, w.WindowOpenMins))
			}
			if w.BoostMinutes != h.BoostMinutes || w.BoostValvePct != h.BoostValvePct || w.DecalcDay != h.DecalcDay || w.DecalcHour != h.DecalcHour ||
				w.MaxValvePct != h.MaxValvePct || w.ValveOffsetPct != h.ValveOffsetPct {
				m, err := cube.CmdSetValveConfig(t, cube.ValveSettings{BoostMinutes: w.BoostMinutes, BoostValvePct: w.BoostValvePct, DecalcDay: w.DecalcDay,
					DecalcHour: w.DecalcHour, MaxValvePct: w.MaxValvePct, ValveOffsetPct: w.ValveOffsetPct})
				if err != nil {
					return fmt.Errorf("device %s: valve settings: %w", want.RFAddress, err)
				}
				lines = append(lines, m)
				what = append(what, fmt.Sprintf("valve boost %dmin/%d%% decalc %s %02d:00 max %d%% offset %d%%",
					w.BoostMinutes, w.BoostValvePct, w.DecalcDay, w.DecalcHour, w.MaxValvePct, w.ValveOffsetPct))
			}
		}
		if !opts.SkipPrograms {
			days, err := programDiff(t, w.WeekProgram, h.WeekProgram)
			if err != nil {
				return fmt.Errorf("device %s: %w", want.RFAddress, err)
			}
			for _, d := range days {
				lines = append(lines, d.lines...)
				what = append(what, "program "+d.day.String())
			}
		}
	case want.Wall != nil && c.Wall != nil:
		w, h := want.Wall, c.Wall
		if !opts.SkipSettings && (w.ComfortTemp != h.ComfortTemp || w.EcoTemp != h.EcoTemp || w.MaxSetpoint != h.MaxSetpoint || w.MinSetpoint != h.MinSetpoint) {
			p.Notes = append(p.Notes, fmt.Sprintf("device %s %q: preset temperatures of wall thermostats are not restored yet (encoding unverified)", want.RFAddress, name))
		}
		if !opts.SkipPrograms {
			days, err := programDiff(t, w.WeekProgram, h.WeekProgram)
			if err != nil {
				return fmt.Errorf("device %s: %w", want.RFAddress, err)
			}
			for _, d := range days {
				lines = append(lines, d.lines...)
				what = append(what, "program "+d.day.String())
			}
		}
	default:
		p.Notes = append(p.Notes, fmt.Sprintf("device %s %q: backup type %s does not match the cube's %s; skipped", want.RFAddress, name, want.Type, c.Type))
		return nil
	}
	if len(lines) == 0 {
		return nil
	}
	p.add(fmt.Sprintf("%s %q: %s", want.RFAddress, name, strings.Join(what, ", ")),
		engine.Command{Key: "restore:" + want.RFAddress.String(), Target: want.RFAddress, RoomID: room, Wakeup: true, Lines: lines})
	return nil
}

type dayChange struct {
	day   cube.Weekday
	lines []cube.Message
}

func programDiff(t cube.Target, want, have cube.WeekProgram) ([]dayChange, error) {
	var out []dayChange
	for d := cube.Saturday; d <= cube.Friday; d++ {
		if len(want[d]) == 0 || slices.Equal(want[d], have[d]) {
			continue
		}
		lines, err := cube.CmdSetDayProgram(t, d, want[d])
		if err != nil {
			return nil, fmt.Errorf("program %s: %w", d, err)
		}
		out = append(out, dayChange{day: d, lines: lines})
	}
	return out, nil
}

// Run sends the plan's steps through the queue one after the other and
// returns one result per step. It stops at the first failure; done, if
// not nil, is called after each step.
func (p *Plan) Run(ctx context.Context, q *engine.Queue, done func(i int, s Step, r engine.Result)) ([]engine.Result, error) {
	var out []engine.Result
	for i, s := range p.Steps {
		r := q.Enqueue(s.Command).Wait(ctx)
		out = append(out, r)
		if done != nil {
			done(i, s, r)
		}
		if r.Err != nil {
			return out, fmt.Errorf("step %d (%s): %w", i+1, s.What, r.Err)
		}
	}
	return out, nil
}
