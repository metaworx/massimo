// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Writes confirmed by the devices (AP RoomConfirm v1.0, Block 1). The
// Cube answers a radio write once it has taken it, but a device's L entry
// takes a new setpoint, mode, key lock or display mode only when the
// device has acknowledged the radiogram (K1, docs/cube-protocol.md), and
// the members of a room take a room command each at its own wakeup (T134,
// T135). So a write is pending from the Cube's answer until every device
// it is for shows what it set: checked at each state of theirs (the pump)
// and at the deadline, confirmWithin after the answer. Then it is not
// confirmed: an unconfirmed event names the devices that do not show it,
// what they show, and their flags, and the devices and their room say so
// in the API until the devices show it, a line then saying so, or a newer
// write of the same kind to them replaces it. An entry the Cube marks out
// of date confirms nothing until the device is heard again (T131). It is
// all kept in memory, for the writes of the API and of the LAN proxy's
// clients whose payload decodes (proxyWrote). A configuration write is
// confirmed by the device's C line instead, at the follow-up's re-reads
// (Block 2, confirm_config.go).

import (
	"context"
	"maps"
	"math"
	"slices"
	"strconv"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// defaultConfirmWithin is how long after the Cube's answer the devices of
// a write have to show it (D177-A): on hardware, the members of a room
// showed a room command after 10 s to 122 s.
const defaultConfirmWithin = 5 * time.Minute

// maxConfirmWrites bounds the writes a Cube keeps open, the oldest
// dropped first: a client of the LAN proxy may send many.
const maxConfirmWrites = 256

// The values of an L entry that confirm a write.
const (
	fieldMode     = "mode"
	fieldSetpoint = "setpoint"
	fieldUntil    = "until"
	fieldLocked   = "locked"
	fieldDisplay  = "display_actual"
)

// The kinds of write confirmed, as the unconfirmed event names them; a
// newer write of a kind replaces an older one of the same kind to the
// same device, a configuration write the same fields of an older one.
const (
	writeSetpoint = "setpoint"
	writeLock     = "lock"
	writeDisplay  = "display"
	writeConfig   = "config"  // 0x11 and 0x12, the presets and the valve
	writeDays     = "program" // 0x10, days of the week programme
)

// The states of a confirmation, as the API shows them.
const (
	confirmPending     = "pending"
	confirmUnconfirmed = "unconfirmed"
	// confirmConfirmed is a write's answer alone: each of its devices
	// showed it already.
	confirmConfirmed = "confirmed"
)

// confirmWrite is one write whose devices are to show it.
type confirmWrite struct {
	what string // writeSetpoint, writeLock, writeDisplay, writeConfig or writeDays
	// byConfig marks a write the device's C line confirms, one device's,
	// at the follow-up's re-reads (confirm_config.go): it has no due.
	byConfig bool
	target   string         // room:<id> or device:<rf>
	room     uint8          // a room write's room; 0 for a device's
	source   string         // api, or proxy for a client of the LAN proxy
	command  string         // the API command's uuid; "" for none
	want     map[string]any // field → the value the devices are to show
	at       time.Time      // when the Cube answered
	due      time.Time      // the deadline
	// open are, per device, the fields it does not show yet.
	open  map[cube.RFAddress][]string
	shown int  // the devices that showed every field
	late  bool // past the deadline: the unconfirmed event is out
}

// confirmations are a Cube's writes not confirmed yet, in the order of
// their answers.
type confirmations struct {
	mu     sync.Mutex
	within time.Duration
	writes []*confirmWrite
}

// setpointWant is what a 0x40 write sets in the devices' L entries: the
// mode, the setpoint in manual and vacation, and a vacation's end. In
// auto the setpoint is the programme's, in boost the device keeps its
// own, so neither is the write's.
func setpointWant(st cube.SetTemperature) map[string]any {
	want := map[string]any{fieldMode: st.Mode}
	switch st.Mode {
	case cube.ModeManual:
		want[fieldSetpoint] = st.Setpoint
	case cube.ModeVacation:
		want[fieldSetpoint], want[fieldUntil] = st.Setpoint, st.Until
	}
	return want
}

// missing are the fields of want that st does not show, in want's
// order of fields; every field where the Cube holds no current state
// (ok false, or an entry out of date).
func missing(st cube.DeviceState, ok bool, want map[string]any) []string {
	var out []string
	for _, f := range slices.Sorted(maps.Keys(want)) {
		if !ok || !st.Valid || !shows(st, f, want[f]) {
			out = append(out, f)
		}
	}
	return out
}

// shows reports whether st shows field at want.
func shows(st cube.DeviceState, field string, want any) bool {
	switch field {
	case fieldMode:
		return st.Mode == want
	case fieldSetpoint:
		return st.Setpoint != nil && *st.Setpoint == want
	case fieldUntil:
		return st.Until != nil && *st.Until == want
	case fieldLocked:
		return st.PanelLocked == want
	case fieldDisplay:
		return st.DisplayActual != nil && *st.DisplayActual == want
	}
	return false
}

// shown is what st shows of fields, as the API names the values; a
// field st has no value for is left out.
func shown(st cube.DeviceState, fields []string) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		switch {
		case f == fieldMode:
			out[f] = st.Mode.String()
		case f == fieldSetpoint && st.Setpoint != nil:
			out[f] = *st.Setpoint
		case f == fieldUntil && st.Until != nil:
			out[f] = st.Until.String()
		case f == fieldLocked:
			out[f] = st.PanelLocked
		case f == fieldDisplay && st.DisplayActual != nil:
			out[f] = *st.DisplayActual
		}
	}
	return out
}

// wantView is want as the API names the values.
func wantView(want map[string]any) map[string]any {
	out := map[string]any{}
	for f, v := range want {
		switch v := v.(type) {
		case cube.Mode:
			out[f] = v.String()
		case cube.Until:
			out[f] = v.String()
		case cube.Weekday:
			out[f] = v.String()
		case cube.DayProgramPart:
			out[f] = v.Points
		default:
			out[f] = v
		}
	}
	return out
}

// missingOf are the fields of w's, or of fields where given, that rf does
// not show: in its state, or for a configuration write in its C line.
func (w *confirmWrite) missingOf(snap *cubelan.Snapshot, rf cube.RFAddress, fields []string) []string {
	want := w.want
	if fields != nil {
		want = map[string]any{}
		for _, f := range fields {
			want[f] = w.want[f]
		}
	}
	if w.byConfig {
		return missingConfig(snap.Config(rf), want)
	}
	st, ok := snap.State(rf)
	return missing(st, ok, want)
}

// shownOf is what rf shows of fields of w's; nil where the Cube holds
// no state, or no C line, for it.
func (w *confirmWrite) shownOf(snap *cubelan.Snapshot, rf cube.RFAddress, fields []string) map[string]any {
	if w.byConfig {
		if c := snap.Config(rf); c != nil {
			return configShown(c, fields)
		}
		return nil
	}
	if st, ok := snap.State(rf); ok {
		return shown(st, fields)
	}
	return nil
}

// beginConfirm starts confirming w, a write the Cube has just taken,
// against devices: a device that shows it already is done, and w
// replaces the open write of its kind to each of them. It returns the
// write's answer, pending, or confirmed where each device shows it
// already, and wakes the follow-up for the deadline.
func (rt *runtime) beginConfirm(w *confirmWrite, devices []cube.RFAddress) string {
	now := time.Now()
	c := &rt.confirms
	snap := rt.client.Snapshot()
	c.mu.Lock()
	w.at, w.due, w.open = now, now.Add(c.within), map[cube.RFAddress][]string{}
	for _, rf := range devices {
		if _, dup := w.open[rf]; dup {
			continue
		}
		c.replaceLocked(w, rf)
		if m := w.missingOf(snap, rf, nil); len(m) > 0 {
			w.open[rf] = m
		} else {
			w.shown++
		}
	}
	if len(w.open) == 0 {
		c.mu.Unlock()
		return confirmConfirmed
	}
	c.writes = append(c.writes, w)
	if n := len(c.writes) - maxConfirmWrites; n > 0 {
		c.writes = slices.Delete(c.writes, 0, n)
	}
	c.mu.Unlock()
	if f := rt.follow; f != nil {
		select {
		case f.wake <- struct{}{}:
		default:
		}
	}
	return confirmPending
}

// replaceLocked drops what w, a newer write, replaces of the open writes
// to rf: those of its kind, or for a configuration write the fields it
// sets; and a write left with no device.
func (c *confirmations) replaceLocked(w *confirmWrite, rf cube.RFAddress) {
	c.writes = slices.DeleteFunc(c.writes, func(o *confirmWrite) bool {
		switch {
		case o.byConfig && w.byConfig:
			if fields, ok := o.open[rf]; ok {
				fields = slices.DeleteFunc(fields, func(f string) bool { _, set := w.want[f]; return set })
				if len(fields) == 0 {
					delete(o.open, rf)
				} else {
					o.open[rf] = fields
				}
			}
		case !o.byConfig && !w.byConfig && o.what == w.what:
			delete(o.open, rf)
		}
		return len(o.open) == 0
	})
}

// forget drops a device the Cube deleted from the open writes.
func (c *confirmations) forget(rf cube.RFAddress) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes = slices.DeleteFunc(c.writes, func(w *confirmWrite) bool {
		delete(w.open, rf)
		return len(w.open) == 0
	})
}

// nextDue is the earliest deadline still to come; false for none.
func (c *confirmations) nextDue() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var due time.Time
	for _, w := range c.writes {
		if !w.late && !w.byConfig && (due.IsZero() || w.due.Before(due)) {
			due = w.due
		}
	}
	return due, !due.IsZero()
}

// confirmReport is what a line says of a write, copied under the lock.
type confirmReport struct {
	w       confirmWrite
	devices []cube.RFAddress // the devices the line names, in order
	fields  map[cube.RFAddress][]string
}

// report copies w for a line naming devices.
func report(w *confirmWrite, devices ...cube.RFAddress) confirmReport {
	r := confirmReport{w: *w, devices: devices, fields: map[cube.RFAddress][]string{}}
	for _, rf := range devices {
		r.fields[rf] = slices.Clone(w.open[rf])
	}
	r.w.open = nil
	return r
}

// confirmState checks the open writes to rf against its state, at each
// state event, and returns the lines of those it now shows after their
// deadline.
func (s *Server) confirmState(rt *runtime, rf cube.RFAddress) []event {
	snap := rt.client.Snapshot()
	if st, ok := snap.State(rf); !ok || !st.Valid {
		return nil
	}
	c := &rt.confirms
	var late []confirmReport
	c.mu.Lock()
	c.writes = slices.DeleteFunc(c.writes, func(w *confirmWrite) bool {
		fields, open := w.open[rf]
		if !open || w.byConfig {
			return false
		}
		if m := w.missingOf(snap, rf, fields); len(m) > 0 {
			w.open[rf] = m
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
	var out []event
	for _, r := range late {
		out = append(out, s.confirmedLate(rt, r))
	}
	return out
}

// confirmStep looks, in the follow-up loop, at the writes whose deadline
// has come: what their devices show now, and an unconfirmed event for
// each write that a device does not show yet.
func (s *Server) confirmStep(rt *runtime) {
	now := time.Now()
	snap := rt.client.Snapshot()
	c := &rt.confirms
	var due []confirmReport
	c.mu.Lock()
	c.writes = slices.DeleteFunc(c.writes, func(w *confirmWrite) bool {
		if w.late || w.byConfig || w.due.After(now) {
			return false
		}
		var left []cube.RFAddress
		for _, rf := range slices.Sorted(maps.Keys(w.open)) {
			if m := w.missingOf(snap, rf, w.open[rf]); len(m) > 0 {
				w.open[rf] = m
				left = append(left, rf)
				continue
			}
			delete(w.open, rf)
			w.shown++
		}
		if len(left) == 0 {
			return true
		}
		w.late = true
		due = append(due, report(w, left...))
		return false
	})
	c.mu.Unlock()
	for _, r := range due {
		rt.emit(s.unconfirmed(rt, r))
	}
}

// summaryOf is what every line of a write says: its kind, target,
// source and what it set.
func summaryOf(w confirmWrite) map[string]any {
	summary := map[string]any{"write": w.what, "target": w.target, "source": w.source, "expected": wantView(w.want)}
	if w.room != 0 {
		summary["room"] = w.room
	}
	return summary
}

// unconfirmed is the warning of a write that devices do not show at its
// deadline: each of them with what it does not show, what it shows, and
// its error flag, link error and out-of-date mark.
func (s *Server) unconfirmed(rt *runtime, r confirmReport) event {
	snap := rt.client.Snapshot()
	summary := summaryOf(r.w)
	summary["outcome"], summary["confirmed"] = "not_confirmed", r.w.shown
	summary["minutes"] = int(math.Round(time.Since(r.w.at).Minutes()))
	var devices []map[string]any
	for _, rf := range r.devices {
		d := map[string]any{"rf_address": rf.String(), "fields": r.fields[rf]}
		if sn := snap.Serial(rf); sn != "" {
			d["serial"] = sn
		}
		if sh := r.w.shownOf(snap, rf, r.fields[rf]); sh != nil {
			d["shows"] = sh
		}
		if st, ok := snap.State(rf); ok {
			d["error"], d["link_error"], d["out_of_date"] = st.Error, st.LinkError, st.OutOfDate
		} else {
			d["reported"] = false
		}
		devices = append(devices, d)
	}
	summary["devices"] = devices
	ev := event{Kind: "unconfirmed", Source: "device", Level: "warn", Time: time.Now(), Summary: summary, Command: s.commandRow(r.w.command)}
	if len(r.devices) == 1 && (r.w.room == 0 || r.w.byConfig) {
		ev.Device, ev.Serial = r.devices[0].String(), snap.Serial(r.devices[0])
	}
	return ev
}

// confirmedLate is the line of a device that shows a write after its
// deadline.
func (s *Server) confirmedLate(rt *runtime, r confirmReport) event {
	rf := r.devices[0]
	summary := summaryOf(r.w)
	summary["outcome"], summary["seconds"] = "confirmed_late", int(math.Round(time.Since(r.w.at).Seconds()))
	return event{Kind: "unconfirmed", Source: "device", Level: "info", Time: time.Now(), Summary: summary, Command: s.commandRow(r.w.command),
		Device: rf.String(), Serial: rt.client.Snapshot().Serial(rf)}
}

// commandRow is the store's row of a command's uuid; 0 for none.
func (s *Server) commandRow(uuid string) int64 {
	if uuid == "" || s.opts.Store == nil {
		return 0
	}
	row, ok, err := s.opts.Store.CommandByUUID(context.Background(), uuid)
	if err != nil || !ok {
		return 0
	}
	return row.ID
}

// mergeConfirmation adds the fields a write's device has not shown to
// out, which it starts where nil.
func mergeConfirmation(out *view.Confirmation, w *confirmWrite, fields []string) *view.Confirmation {
	if out == nil {
		out = &view.Confirmation{State: confirmPending, Since: w.at}
	}
	if w.late {
		out.State = confirmUnconfirmed
	}
	if w.at.Before(out.Since) {
		out.Since = w.at
	}
	for _, f := range fields {
		if !slices.Contains(out.Fields, f) {
			out.Fields = append(out.Fields, f)
		}
	}
	slices.Sort(out.Fields)
	return out
}

// device is what rf has not shown of the writes to it; nil for none.
func (c *confirmations) device(rf cube.RFAddress) *view.Confirmation {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out *view.Confirmation
	for _, w := range c.writes {
		if fields, ok := w.open[rf]; ok {
			out = mergeConfirmation(out, w, fields)
		}
	}
	return out
}

// room is what members have not shown of the writes to them, with the
// members that have not; nil for none.
func (c *confirmations) room(members []cube.RFAddress) *view.Confirmation {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out *view.Confirmation
	for _, w := range c.writes {
		for _, rf := range members {
			if fields, ok := w.open[rf]; ok {
				out = mergeConfirmation(out, w, fields)
				if !slices.Contains(out.Devices, rf.String()) {
					out.Devices = append(out.Devices, rf.String())
				}
			}
		}
	}
	return out
}

// roomConfirmation is what the visible thermostats of a room have not
// shown of the writes to them.
func (rt *runtime) roomConfirmation(room uint8) *view.Confirmation {
	radiators, walls := roomThermostats(rt, room)
	return rt.confirms.room(append(radiators, walls...))
}

// withConfirmations sets each device's confirmation, a virtual wall
// thermostat's its room's, and returns devs.
func (rt *runtime) withConfirmations(devs []view.Device) []view.Device {
	for i := range devs {
		rt.setConfirmation(&devs[i])
	}
	return devs
}

// setConfirmation sets d's confirmation, a virtual wall thermostat's its
// room's.
func (rt *runtime) setConfirmation(d *view.Device) {
	if d.Virtual {
		d.Confirmation = rt.roomConfirmation(d.RoomID)
		return
	}
	if rf, err := cube.ParseRFAddress(d.RFAddress); err == nil {
		d.Confirmation = rt.confirms.device(rf)
	}
}

// setpointConfirm starts confirming a setpoint write the Cube took: a
// room's to each of its visible thermostats, a device's to the device.
func (rt *runtime) setpointConfirm(ctx context.Context, t cube.Target, device cube.RFAddress, st cube.SetTemperature, source string) string {
	w := &confirmWrite{what: writeSetpoint, source: source, command: commandOf(ctx), want: setpointWant(st)}
	devices := []cube.RFAddress{device}
	if device == 0 {
		radiators, walls := roomThermostats(rt, t.RoomID)
		devices = append(radiators, walls...)
		w.target, w.room = roomTarget(t.RoomID), t.RoomID
	} else {
		w.target = "device:" + device.String()
	}
	return rt.beginConfirm(w, devices)
}

// roomTarget is a room write's target, as the commands name it.
func roomTarget(room uint8) string { return "room:" + strconv.Itoa(int(room)) }

// proxyConfirm starts confirming a write of a LAN proxy client whose
// payload decodes: a setpoint, a key lock or a display mode, a
// room-flagged one to each visible thermostat of the room (the wall
// thermostats alone for the display), a direct one to the device.
func (rt *runtime) proxyConfirm(c cube.RadioCommand) {
	w := &confirmWrite{source: "proxy"}
	switch c.Command {
	case cube.RFSetTemperature:
		st, err := c.SetTemperature()
		if err != nil {
			return
		}
		w.what, w.want = writeSetpoint, setpointWant(st)
	case cube.RFLockControls:
		locked, err := c.Locked()
		if err != nil {
			return
		}
		w.what, w.want = writeLock, map[string]any{fieldLocked: locked}
	case cube.RFSetDisplayMode:
		actual, err := c.DisplayActual()
		if err != nil {
			return
		}
		w.what, w.want = writeDisplay, map[string]any{fieldDisplay: actual}
	default:
		return
	}
	devices := []cube.RFAddress{c.Target.RFAddress}
	w.target = "device:" + c.Target.RFAddress.String()
	if c.Target.Room {
		radiators, walls := roomThermostats(rt, c.Target.RoomID)
		devices = append(radiators, walls...)
		if c.Command == cube.RFSetDisplayMode {
			devices = walls
		}
		w.target, w.room = roomTarget(c.Target.RoomID), c.Target.RoomID
	}
	if len(devices) > 0 {
		rt.beginConfirm(w, devices)
	}
}
