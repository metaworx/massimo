// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package view

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/roomorder"
)

// The virtual wall thermostat (docs/api-v1.md, "The virtual wall
// thermostat"): every visible room has one, a wall thermostat whose
// values the server computes from the room's real devices, so that every
// client shows a room by the same numbers. The original software allows
// one wall thermostat per room; the room's wall thermostat sends its
// measured temperature to the room's radiator thermostats, which
// regulate by it. So a room with a real wall thermostat takes that one's
// temperature, and one without takes the median or mean of its radiator
// thermostats'. It is computed here from the views of the room's
// devices, and never written to the Cube.

// Kind says which devices a list shows: the virtual wall thermostats,
// the real wall thermostats, or both (the API's ?devices=).
type Kind string

// The views of the device lists.
const (
	// KindDefault shows every real device, and the virtual wall
	// thermostat of each room without a real one: every room has one
	// wall thermostat.
	KindDefault Kind = "default"
	// KindReal shows the real devices only.
	KindReal Kind = "real"
	// KindVirtual shows the virtual wall thermostat of every room and
	// leaves the real wall thermostats out: every room has one wall
	// thermostat, always the same.
	KindVirtual Kind = "virtual"
	// KindAll shows every real device and every virtual wall thermostat.
	KindAll Kind = "all"
)

// ParseKind reads a view's name; "" is KindDefault.
func ParseKind(s string) (Kind, error) {
	switch k := Kind(s); k {
	case "":
		return KindDefault, nil
	case KindDefault, KindReal, KindVirtual, KindAll:
		return k, nil
	}
	return "", fmt.Errorf("devices=%q: default, real, virtual or all", s)
}

// The ways a room without a wall thermostat takes its temperature from
// its radiator thermostats.
const (
	TempMedian = "median"
	TempMean   = "mean"
)

// RoomInfo is what the server keeps for a room: its uid and its virtual
// wall thermostat's settings.
type RoomInfo struct {
	UID        string
	TempMethod string // TempMedian (also for "") or TempMean
	// RFAddress is the address the owner gave the virtual wall
	// thermostat; 0 for the room number.
	RFAddress cube.RFAddress
}

// VirtualSerial is the serial of a room's virtual wall thermostat.
func VirtualSerial(room uint8) string { return fmt.Sprintf("VWT%07d", room) }

// The addresses a virtual wall thermostat takes where its room number is
// a real device's.
const (
	virtualFirst cube.RFAddress = 0x000100
	virtualLast  cube.RFAddress = 0x0001ff
)

// VirtualAddresses gives each room's virtual wall thermostat its radio
// address: the one the owner set, else the room number, each unless a
// real device, the Cube or an earlier room has it; then the first free
// one from 000100 to 0001ff, in the rooms' order.
func VirtualAddresses(snap *cubelan.Snapshot, rooms []cube.RoomMeta, info map[uint8]RoomInfo) map[uint8]cube.RFAddress {
	taken := RealAddresses(snap)
	out := map[uint8]cube.RFAddress{}
	var later []uint8
	for _, r := range rooms {
		want := info[r.ID].RFAddress
		if want == 0 || taken[want] {
			want = cube.RFAddress(r.ID)
		}
		if taken[want] {
			later = append(later, r.ID)
			continue
		}
		taken[want] = true
		out[r.ID] = want
	}
	next := virtualFirst
	for _, id := range later {
		for next <= virtualLast && taken[next] {
			next++
		}
		if next > virtualLast {
			break
		}
		taken[next] = true
		out[id] = next
	}
	return out
}

// RealAddresses is every radio address a real device or the Cube has.
func RealAddresses(snap *cubelan.Snapshot) map[cube.RFAddress]bool {
	taken := map[cube.RFAddress]bool{}
	for _, rf := range snap.Devices() {
		taken[rf] = true
	}
	if md := snap.Metadata(); md != nil {
		for _, d := range md.Devices {
			taken[d.RFAddress] = true
		}
	}
	if h := snap.Hello(); h != nil {
		taken[h.RFAddress] = true
	}
	return taken
}

// RoomThermostat is a room's virtual wall thermostat, computed from the
// views of the room's visible real devices (members).
func RoomThermostat(room cube.RoomMeta, members []Device, info RoomInfo, addr cube.RFAddress) Device {
	d := Device{
		RFAddress: addr.String(), Type: cube.DeviceWallThermostat, Name: room.Name, RoomID: room.ID,
		Serial: VirtualSerial(room.ID), Virtual: true, RoomUID: info.UID,
	}
	var walls, radiators, thermostats, contacts []Device
	for _, m := range members {
		switch {
		case m.Type == cube.DeviceWallThermostat:
			walls = append(walls, m)
		case m.Type.IsRadiatorThermostat():
			radiators = append(radiators, m)
		case m.Type == cube.DeviceShutterContact:
			contacts = append(contacts, m)
		}
	}
	thermostats = append(append(thermostats, walls...), radiators...)
	received := map[string]time.Time{}
	newest := func(k string, t time.Time) {
		if t.After(received[k]) {
			received[k] = t
		}
	}
	var lastSeen time.Time
	for _, m := range members {
		if m.LastSeen != nil && m.LastSeen.After(lastSeen) {
			lastSeen = *m.LastSeen
		}
	}
	if !lastSeen.IsZero() {
		d.LastSeen = &lastSeen
	}

	st := &State{}
	// The window: any contact of the room open.
	for _, c := range contacts {
		if c.State == nil || c.State.WindowOpen == nil {
			continue
		}
		open := st.WindowOpen != nil && *st.WindowOpen || *c.State.WindowOpen
		st.WindowOpen = &open
		newest("window_open", c.Received["window_open"])
	}
	reported := reportedOnly(thermostats)
	if len(reported) == 0 {
		// No thermostat has reported, or the room has none: the window
		// alone, where the room has a contact.
		if st.WindowOpen != nil {
			st.windowOnly = true
			d.State, d.Received = st, nonEmpty(received)
		}
		return d
	}
	d.Reported = true

	// The temperature: the wall thermostat's, else the radiator
	// thermostats' median or mean; stale values left out while a fresh one
	// exists.
	source, method := withTemp(walls), TempMedian
	if len(source) == 0 {
		source, method = withTemp(radiators), info.TempMethod
	}
	if len(source) > 0 {
		used := fresh(source)
		if len(used) == 0 {
			used, d.Stale = source, true
		}
		var vals []float64
		var oldest time.Time
		var from []string
		for _, m := range used {
			vals = append(vals, *m.State.ActualTemp)
			from = append(from, m.RFAddress)
			if t := m.Received["actual_temp"]; oldest.IsZero() || t.Before(oldest) {
				oldest = t
			}
		}
		v := aggregate(vals, method)
		st.ActualTemp = &v
		if !oldest.IsZero() {
			received["actual_temp"] = oldest
		}
		d.Sources = map[string][]string{"actual_temp": from}
	}

	// The mode: boost while any thermostat boosts; else the room's state
	// by the one order (RoomMajority.D9, D19-A; roomState); setpoint and
	// until from the same, during a boost too, which sources.setpoint
	// names, that one first, and sources.mode outside a boost, the
	// boosting ones during one.
	b, setFrom := roomState(room.RFAddress, members, reported)
	modeFrom := setFrom
	st.Mode, st.Setpoint, st.Until = b.State.Mode, b.State.Setpoint, b.State.Until
	for _, k := range []string{"mode", "setpoint", "until"} {
		newest(k, b.Received[k])
	}
	var boosting []string
	for _, m := range reported {
		if m.State.Mode == cube.ModeBoost {
			boosting = append(boosting, m.RFAddress)
			newest("mode", m.Received["mode"])
		}
	}
	if len(boosting) > 0 {
		st.Mode, modeFrom = cube.ModeBoost, boosting
	}
	if d.Sources == nil {
		d.Sources = map[string][]string{}
	}
	d.Sources["mode"], d.Sources["setpoint"] = modeFrom, setFrom
	consistent := true
	for _, m := range reported[1:] {
		consistent = consistent && SameSetting(reported[0].State, m.State)
	}
	st.Consistent = &consistent

	// The valve: the highest of the radiator thermostats'.
	for _, m := range reportedOnly(radiators) {
		if v := m.State.ValvePct; v != nil && (st.ValvePct == nil || *v > *st.ValvePct) {
			st.ValvePct = v
			newest("valve_pct", m.Received["valve_pct"])
		}
	}

	// A real wall thermostat's own flags; the radiator thermostats show
	// theirs.
	if len(walls) > 0 && walls[0].State != nil {
		w := walls[0].State
		st.BatteryLow, st.Error, st.FailedRadiogram, st.LinkError, st.PanelLocked = w.BatteryLow, w.Error, w.FailedRadiogram, w.LinkError, w.PanelLocked
		for _, k := range []string{"battery_low", "error", "failed_radiogram", "link_error", "panel_locked"} {
			newest(k, walls[0].Received[k])
		}
	}
	d.State, d.Received = st, nonEmpty(received)
	return d
}

// reportedOnly is the devices that have a state.
func reportedOnly(ds []Device) []Device {
	var out []Device
	for _, d := range ds {
		if d.State != nil {
			out = append(out, d)
		}
	}
	return out
}

// withTemp is the devices that have a measured temperature.
func withTemp(ds []Device) []Device {
	var out []Device
	for _, d := range reportedOnly(ds) {
		if d.State.ActualTemp != nil {
			out = append(out, d)
		}
	}
	return out
}

// fresh is the devices that are not stale.
func fresh(ds []Device) []Device {
	var out []Device
	for _, d := range ds {
		if !d.Stale {
			out = append(out, d)
		}
	}
	return out
}

// roomState is the thermostat whose mode, setpoint and until a virtual
// wall thermostat shows, and the radio addresses of those its state came
// from: the room's state by the one order (roomorder.State, the one rule
// of the room checks' re-send, without Massimo's own command and the
// state held before a structure change, which only the server knows)
// over members, the room's devices in the Cube's order, those with a
// state and a setpoint not in boost: what more than half of them show,
// the members in auto that each follow their own programme as one
// state; else the one whose mode or setpoint changed last; else the
// first of the room's order, its master (the metadata's, master), its
// wall thermostat, its radiator thermostats. Where every one boosts,
// the same over those in boost; where none has a setpoint, the first of
// reported, the room's thermostats with a state. The thermostat comes
// first among those its state came from, so that the first of them is
// the one whose setpoint the virtual wall thermostat shows
// (sources.setpoint, Room.Thermostat).
func roomState(master cube.RFAddress, members, reported []Device) (Device, []string) {
	for _, boost := range []bool{false, true} {
		var ds []Device
		var ms []roomorder.Member
		for _, d := range members {
			if m, ok := d.member(); ok && (m.State.Mode == cube.ModeBoost) == boost {
				ds, ms = append(ds, d), append(ms, m)
			}
		}
		if p, ok := roomorder.State(master, ms); ok {
			from := []string{ds[p.Index].RFAddress}
			for _, rf := range p.From {
				if rf != ms[p.Index].RF {
					from = append(from, rf.String())
				}
			}
			return ds[p.Index], from
		}
	}
	return reported[0], []string{reported[0].RFAddress}
}

// aggregate is the median (of two values, their mean) or the mean,
// rounded to 0.1 °C.
func aggregate(vals []float64, method string) float64 {
	var v float64
	if method == TempMean {
		for _, x := range vals {
			v += x
		}
		v /= float64(len(vals))
	} else {
		s := append([]float64(nil), vals...)
		sort.Float64s(s)
		n := len(s)
		v = s[n/2]
		if n%2 == 0 {
			v = (s[n/2-1] + s[n/2]) / 2
		}
	}
	return math.Round(v*10) / 10
}

// SameSetting reports whether two thermostats agree on mode, setpoint
// and until (roomorder.SameSetting): the rule of a virtual wall
// thermostat's Consistent, and of the server's check of a room's
// members.
func SameSetting(a, b *State) bool {
	return roomorder.SameSetting(cube.DeviceState{Mode: a.Mode, Setpoint: a.Setpoint, Until: a.Until},
		cube.DeviceState{Mode: b.Mode, Setpoint: b.Setpoint, Until: b.Until})
}

// nonEmpty is nil for an empty map.
func nonEmpty(m map[string]time.Time) map[string]time.Time {
	for k, v := range m {
		if v.IsZero() {
			delete(m, k)
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// MarshalJSON writes a state; a virtual wall thermostat's in a room
// without a reported thermostat holds its window alone.
func (s State) MarshalJSON() ([]byte, error) {
	if s.windowOnly {
		return json.Marshal(struct {
			WindowOpen *bool `json:"window_open,omitempty"`
		}{s.WindowOpen})
	}
	type plain State
	return json.Marshal(plain(s))
}

// RoomsOf lists the visible rooms as the view kind shows them, each with
// its uid and the thermostat whose state is the room's, the one its
// virtual wall thermostat takes its setpoint and until from
// (Room.Thermostat); info holds the rooms' uids and settings.
func RoomsOf(snap *cubelan.Snapshot, h Hidden, stale Staleness, kind Kind, info map[uint8]RoomInfo) []Room {
	md := snap.Metadata()
	if md == nil {
		return nil
	}
	rooms := Rooms(snap, h, stale)
	visible := h.VisibleRooms(md)
	addrs := VirtualAddresses(snap, visible, info)
	cubeNow, clock := snap.CubeClock(now())
	for i := range rooms {
		r := &rooms[i]
		r.UID = info[r.ID].UID
		meta, _ := h.Room(md, r.ID)
		for j := range r.Devices {
			r.Devices[j].setFollows(snap, cubeNow, clock)
		}
		vwt := RoomThermostat(meta, r.Devices, info[r.ID], addrs[r.ID])
		if from := vwt.Sources["setpoint"]; len(from) > 0 {
			r.Thermostat = from[0] // the one whose setpoint the virtual wall thermostat shows
		}
		r.Devices = pick(kind, r.Devices, vwt)
	}
	return rooms
}

// DevicesOf lists the visible devices as the view kind shows them: the
// real ones in the Cube's order, then the virtual wall thermostats in the
// rooms' order.
func DevicesOf(snap *cubelan.Snapshot, h Hidden, stale Staleness, kind Kind, info map[uint8]RoomInfo) []Device {
	devs := Devices(snap, h, stale)
	if kind == KindReal {
		return devs
	}
	var vwts []Device
	for _, r := range RoomsOf(snap, h, stale, kind, info) {
		for _, d := range r.Devices {
			if d.Virtual {
				vwts = append(vwts, d)
			}
		}
	}
	walls := map[string]bool{}
	if kind == KindVirtual {
		for _, d := range devs {
			if d.Type == cube.DeviceWallThermostat && !d.Unlisted {
				walls[d.RFAddress] = true
			}
		}
	}
	var out []Device
	for _, d := range devs {
		if !walls[d.RFAddress] {
			out = append(out, d)
		}
	}
	return append(out, vwts...)
}

// pick is a room's devices as the view kind shows them, the virtual wall
// thermostat first where it is shown.
func pick(kind Kind, real []Device, vwt Device) []Device {
	hasWall := false
	var noWalls []Device
	for _, d := range real {
		if d.Type == cube.DeviceWallThermostat {
			hasWall = true
		} else {
			noWalls = append(noWalls, d)
		}
	}
	switch kind {
	case KindReal:
		return real
	case KindAll:
		return append([]Device{vwt}, real...)
	case KindVirtual:
		return append([]Device{vwt}, noWalls...)
	}
	if hasWall {
		return real
	}
	return append([]Device{vwt}, real...)
}

// Shown reports whether a view kind shows a room's virtual wall
// thermostat, the room having a real wall thermostat or not.
func Shown(kind Kind, roomHasWall bool) bool {
	switch kind {
	case KindReal:
		return false
	case KindDefault:
		return !roomHasWall
	}
	return true
}

// RoomThermostats computes every visible room's virtual wall thermostat,
// by room, with whether the room has a real wall thermostat.
func RoomThermostats(snap *cubelan.Snapshot, h Hidden, stale Staleness, info map[uint8]RoomInfo) (map[uint8]Device, map[uint8]bool) {
	out, walls := map[uint8]Device{}, map[uint8]bool{}
	for _, r := range RoomsOf(snap, h, stale, KindAll, info) {
		for _, d := range r.Devices {
			switch {
			case d.Virtual:
				out[r.ID] = d
			case d.Type == cube.DeviceWallThermostat:
				walls[r.ID] = true
			}
		}
	}
	return out, walls
}
