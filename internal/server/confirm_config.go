// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Configuration writes compared with the C line (AP RoomConfirm v1.0,
// Block 2). The Cube changes a device's C line only once the device has
// acknowledged a configuration write: in T130 a written day was in it
// exactly as sent within 17 s. So the follow-up's re-reads of the C line
// (followup.go) compare it with what was written, field by field: the
// preset temperatures, the offset and the window-open settings of 0x11,
// the boost, decalcification and valve settings of 0x12, and the days of
// the week programme of 0x10. Each written device is a write of its own,
// confirmed once its C line shows every field, and not confirmed after the
// follow-up's last re-read, about six minutes on: the same unconfirmed
// event as a setpoint's names the fields, and the device's confirmation
// holds them until a later C line shows them, a line at info then, or a
// newer write sets them. A write whose radiograms do not decode, a room
// group's among them, keeps the follow-up's "changed at all".

import (
	"context"
	"maps"
	"slices"
	"strings"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// The fields of a C line that confirm a configuration write, as the API
// names the configuration's values; a day of the week programme is
// program.<day>, the second part of a long day program.<day>.2.
const (
	cfgComfort     = "comfort_temp"
	cfgEco         = "eco_temp"
	cfgMax         = "max_setpoint"
	cfgMin         = "min_setpoint"
	cfgOffset      = "temp_offset"
	cfgWindowTemp  = "window_open_temp"
	cfgWindowMins  = "window_open_minutes"
	cfgBoostMins   = "boost_minutes"
	cfgBoostValve  = "boost_valve_pct"
	cfgDecalcDay   = "decalc_day"
	cfgDecalcHour  = "decalc_hour"
	cfgMaxValve    = "max_valve_pct"
	cfgValveOffset = "valve_offset_pct"
	cfgProgram     = "program."
)

// wallFields are the fields a wall thermostat's C line holds, beside its
// week programme: no window-open time, decalcification or valve limits.
var wallFields = map[string]bool{cfgComfort: true, cfgEco: true, cfgMax: true, cfgMin: true, cfgOffset: true, cfgWindowTemp: true,
	cfgBoostMins: true, cfgBoostValve: true}

// programField is the field of a part of a day's programme.
func programField(p cube.DayProgramPart) string {
	f := cfgProgram + p.Day.String()
	if p.From > 0 {
		f += ".2"
	}
	return f
}

// configWant is what a configuration radiogram sets in a C line, by
// field, and the kind of write; false for one that does not decode or
// is no 0x10, 0x11 or 0x12.
func configWant(c cube.RadioCommand) (string, map[string]any, bool) {
	switch c.Command {
	case cube.RFSetTemperatures:
		t, err := c.Temperatures()
		if err != nil {
			return "", nil, false
		}
		return writeConfig, map[string]any{cfgComfort: t.Comfort, cfgEco: t.Eco, cfgMax: t.Max, cfgMin: t.Min, cfgOffset: t.Offset,
			cfgWindowTemp: t.WindowOpen, cfgWindowMins: t.WindowOpenMinutes}, true
	case cube.RFSetValveConfig:
		v, err := c.ValveSettings()
		if err != nil {
			return "", nil, false
		}
		return writeConfig, map[string]any{cfgBoostMins: v.BoostMinutes, cfgBoostValve: v.BoostValvePct, cfgDecalcDay: v.DecalcDay,
			cfgDecalcHour: v.DecalcHour, cfgMaxValve: v.MaxValvePct, cfgValveOffset: v.ValveOffsetPct}, true
	case cube.RFSetWeekProgram:
		p, err := c.DayProgram()
		if err != nil {
			return "", nil, false
		}
		return writeDays, map[string]any{programField(p): p}, true
	}
	return "", nil, false
}

// forDevice is want without the fields rf's C line does not hold.
func forDevice(snap *cubelan.Snapshot, rf cube.RFAddress, want map[string]any) map[string]any {
	c := snap.Config(rf)
	if c == nil || c.Wall == nil {
		return want
	}
	out := map[string]any{}
	for f, v := range want {
		if wallFields[f] || strings.HasPrefix(f, cfgProgram) {
			out[f] = v
		}
	}
	return out
}

// configValue is field's value in c, a device's configuration from its C
// line, a day's programme for a day's field; false where it holds none.
func configValue(c *cube.DeviceConfig, field string) (any, bool) {
	if c == nil {
		return nil, false
	}
	if name, ok := strings.CutPrefix(field, cfgProgram); ok {
		var day cube.Weekday
		if day.UnmarshalText([]byte(strings.TrimSuffix(name, ".2"))) != nil {
			return nil, false
		}
		switch {
		case c.Thermostat != nil:
			return c.Thermostat.WeekProgram[day], true
		case c.Wall != nil:
			return c.Wall.WeekProgram[day], true
		}
		return nil, false
	}
	if t := c.Thermostat; t != nil {
		v, ok := map[string]any{cfgComfort: t.ComfortTemp, cfgEco: t.EcoTemp, cfgMax: t.MaxSetpoint, cfgMin: t.MinSetpoint,
			cfgOffset: t.TempOffset, cfgWindowTemp: t.WindowOpenTemp, cfgWindowMins: t.WindowOpenMins, cfgBoostMins: t.BoostMinutes,
			cfgBoostValve: t.BoostValvePct, cfgDecalcDay: t.DecalcDay, cfgDecalcHour: t.DecalcHour, cfgMaxValve: t.MaxValvePct,
			cfgValveOffset: t.ValveOffsetPct}[field]
		return v, ok
	}
	if w := c.Wall; w != nil {
		v, ok := map[string]any{cfgComfort: w.ComfortTemp, cfgEco: w.EcoTemp, cfgMax: w.MaxSetpoint, cfgMin: w.MinSetpoint,
			cfgOffset: w.TempOffset, cfgWindowTemp: w.WindowOpenTemp, cfgBoostMins: w.BoostMinutes, cfgBoostValve: w.BoostValvePct}[field]
		return v, ok
	}
	return nil, false
}

// missingConfig are the fields of want that c, a C line's configuration,
// does not show, in order; every field without one.
func missingConfig(c *cube.DeviceConfig, want map[string]any) []string {
	var out []string
	for _, f := range slices.Sorted(maps.Keys(want)) {
		v, ok := configValue(c, f)
		switch p, part := want[f].(cube.DayProgramPart); {
		case !ok:
			out = append(out, f)
		case part:
			if day, _ := v.(cube.DayProgram); !p.Shown(day) {
				out = append(out, f)
			}
		case v != want[f]:
			out = append(out, f)
		}
	}
	return out
}

// configShown is what c, a C line's configuration, shows of fields, as
// the API names the values.
func configShown(c *cube.DeviceConfig, fields []string) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		v, ok := configValue(c, f)
		if !ok {
			continue
		}
		if d, isDay := v.(cube.Weekday); isDay {
			v = d.String()
		}
		out[f] = v
	}
	return out
}

// combineAnswers is the answer of two writes as one: pending where
// either is, "" for neither.
func combineAnswers(a, b string) string {
	switch {
	case a == confirmPending || b == confirmPending:
		return confirmPending
	case a == "":
		return b
	}
	return a
}

// configConfirm starts confirming the configuration lines the Cube took
// for rf by its C line, one write of each kind: the presets and valve
// settings as config, the days as program. It returns the answer, ""
// where no line is one the C line confirms.
func (rt *runtime) configConfirm(ctx context.Context, rf cube.RFAddress, lines []cube.Message, target string, room uint8) string {
	snap := rt.client.Snapshot()
	byWhat := map[string]map[string]any{}
	for _, m := range lines {
		c, err := cube.ParseRadioCommand(m.Data)
		if err != nil || m.Type != 's' {
			continue
		}
		what, want, ok := configWant(c)
		if !ok {
			continue
		}
		if byWhat[what] == nil {
			byWhat[what] = map[string]any{}
		}
		maps.Copy(byWhat[what], forDevice(snap, rf, want))
	}
	answer := ""
	for _, what := range slices.Sorted(maps.Keys(byWhat)) {
		if want := byWhat[what]; len(want) > 0 {
			w := &confirmWrite{what: what, byConfig: true, target: target, room: room, source: "api", command: commandOf(ctx), want: want}
			answer = combineAnswers(answer, rt.beginConfirm(w, []cube.RFAddress{rf}))
		}
	}
	return answer
}

// proxyConfigConfirm starts confirming a LAN proxy client's configuration
// radiogram whose payload decodes, by the C line of each device it
// reaches: each visible thermostat of the room for a room-flagged one.
func (rt *runtime) proxyConfigConfirm(c cube.RadioCommand) {
	what, want, ok := configWant(c)
	if !ok {
		return
	}
	devices, target, room := []cube.RFAddress{c.Target.RFAddress}, "device:"+c.Target.RFAddress.String(), uint8(0)
	if c.Target.Room {
		radiators, walls := roomThermostats(rt, c.Target.RoomID)
		devices, target, room = append(radiators, walls...), roomTarget(c.Target.RoomID), c.Target.RoomID
	}
	snap := rt.client.Snapshot()
	for _, rf := range devices {
		if w := forDevice(snap, rf, want); len(w) > 0 {
			rt.beginConfirm(&confirmWrite{what: what, byConfig: true, target: target, room: room, source: "proxy", want: w}, []cube.RFAddress{rf})
		}
	}
}

// configCheck is how a device's C line compared with the configuration
// writes to it: expected, some were open and not yet judged; waiting,
// some of those are still not shown; failed, the last re-read judged some
// not confirmed. The events are the lines to emit.
type configCheck struct {
	expected, waiting, failed bool
	events                    []event
}

// checkConfig compares the open configuration writes to rf with its C
// line, at a re-read of the follow-up and at each config event: a write
// it shows is done, with a line at info where it was not confirmed
// before; at the follow-up's last re-read (end) the rest are not
// confirmed, a warning each.
func (s *Server) checkConfig(rt *runtime, rf cube.RFAddress, end bool) configCheck {
	snap := rt.client.Snapshot()
	c := &rt.confirms
	var out configCheck
	var late, due []confirmReport
	c.mu.Lock()
	c.writes = slices.DeleteFunc(c.writes, func(w *confirmWrite) bool {
		fields, open := w.open[rf]
		if !open || !w.byConfig {
			return false
		}
		out.expected = out.expected || !w.late
		if m := w.missingOf(snap, rf, fields); len(m) > 0 {
			w.open[rf] = m
			switch {
			case w.late:
			case end:
				w.late, out.failed = true, true
				due = append(due, report(w, rf))
			default:
				out.waiting = true
			}
			return false
		}
		if w.late {
			late = append(late, report(w, rf))
		}
		delete(w.open, rf)
		w.shown++
		return len(w.open) == 0
	})
	c.mu.Unlock()
	for _, r := range due {
		out.events = append(out.events, s.unconfirmed(rt, r))
	}
	for _, r := range late {
		out.events = append(out.events, s.confirmedLate(rt, r))
	}
	return out
}
