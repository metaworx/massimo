// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package view is the read model of a Cube shared by the API, the status
// command and backups: the live state of a device as the user sees it,
// the rooms and devices that are visible given the hidden set, the rule
// that says when a room accepts room commands, and the number of
// radiograms still pending on the Cube. It reads a cubelan.Snapshot and
// never writes to the Cube.
package view

import (
	"encoding/json"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Hidden is the set of hidden devices: devices that are paired but never
// report (they would block room commands if they were moved) are parked
// in a room of their own on the Cube and left out of every view and
// every write. The set is the configured list (MASSIMO_CUBE_<SN>_IGNORE)
// plus the devices of cube.HiddenRoom. See docs/portal-protocol.md.
type Hidden map[cube.RFAddress]bool

// NewHidden builds the hidden set: the listed devices plus every device
// the metadata places in cube.HiddenRoom. With show set nothing is hidden.
func NewHidden(list []cube.RFAddress, md *cube.Metadata, show bool) Hidden {
	if show {
		return nil
	}
	set := Set(list)
	if md == nil {
		return set
	}
	for _, d := range md.Devices {
		if d.RoomID == cube.HiddenRoom {
			if set == nil {
				set = Hidden{}
			}
			set[d.RFAddress] = true
		}
	}
	return set
}

// Set builds a hidden set from a list of addresses, nil for none.
func Set(list []cube.RFAddress) Hidden {
	if len(list) == 0 {
		return nil
	}
	set := Hidden{}
	for _, rf := range list {
		set[rf] = true
	}
	return set
}

// Rooms lists the rooms of md that hold only hidden devices (and at
// least one): they are hidden together with their devices.
func (h Hidden) Rooms(md *cube.Metadata) map[uint8]bool {
	hidden := map[uint8]bool{}
	if md == nil || len(h) == 0 {
		return hidden
	}
	count := map[uint8]int{}
	for _, d := range md.Devices {
		if h[d.RFAddress] {
			count[d.RoomID]++
		} else {
			count[d.RoomID] = -1 << 20
		}
	}
	for id, n := range count {
		if n > 0 && id != 0 {
			hidden[id] = true
		}
	}
	return hidden
}

// VisibleRooms returns the rooms of md that are not hidden, in the
// Cube's order.
func (h Hidden) VisibleRooms(md *cube.Metadata) []cube.RoomMeta {
	if md == nil {
		return nil
	}
	hidden := h.Rooms(md)
	var out []cube.RoomMeta
	for _, r := range md.Rooms {
		if !hidden[r.ID] {
			out = append(out, r)
		}
	}
	return out
}

// Room returns the visible room with the given id.
func (h Hidden) Room(md *cube.Metadata, id uint8) (cube.RoomMeta, bool) {
	for _, r := range h.VisibleRooms(md) {
		if r.ID == id {
			return r, true
		}
	}
	return cube.RoomMeta{}, false
}

// State is a device's live state as the API, status and backups show it.
type State struct {
	Mode       cube.Mode   `json:"mode"`
	Setpoint   *float64    `json:"setpoint,omitempty"`
	Until      *cube.Until `json:"until,omitempty"` // end of vacation mode
	ActualTemp *float64    `json:"actual_temp,omitempty"`
	ValvePct   *int        `json:"valve_pct,omitempty"`
	WindowOpen *bool       `json:"window_open,omitempty"`
	BatteryLow bool        `json:"battery_low"`
	Error      bool        `json:"error"`
	LinkError  bool        `json:"link_error"`
	// PanelLocked is the device's key lock ("LoC" on the display).
	PanelLocked bool `json:"panel_locked,omitempty"`
	// Consistent is a virtual wall thermostat's alone: false while the
	// room's thermostats disagree on mode, setpoint or until.
	Consistent *bool `json:"consistent,omitempty"`

	// windowOnly marks a virtual wall thermostat's state in a room
	// without a reported thermostat: it holds the window alone.
	windowOnly bool
}

// StateOf is the live state of a device, or nil while the Cube holds no
// state for it.
func StateOf(st cube.DeviceState) *State {
	if !st.Valid {
		return nil
	}
	return &State{Mode: st.Mode, Setpoint: st.Setpoint, Until: st.Until, ActualTemp: st.ActualTemp, ValvePct: st.ValvePct,
		WindowOpen: st.WindowOpen, BatteryLow: st.BatteryLow, Error: st.Error, LinkError: st.LinkError, PanelLocked: st.PanelLocked}
}

// Device is one device with its name, room and live state.
type Device struct {
	RFAddress string          `json:"rf_address"`
	Type      cube.DeviceType `json:"type"`
	Name      string          `json:"name"`
	RoomID    uint8           `json:"room_id"`
	Serial    string          `json:"serial,omitempty"`
	// Unlisted marks a device the Cube holds a configuration or state
	// for that is not in its room and device list.
	Unlisted bool `json:"unlisted,omitempty"`
	// Virtual marks a room's virtual wall thermostat (vwt.go), which the
	// server computes and which is no device of the Cube's.
	Virtual bool `json:"virtual,omitempty"`
	// RoomUID is a virtual wall thermostat's room's uid.
	RoomUID string `json:"room_uid,omitempty"`
	// Reported is true when the Cube holds state for the device, that
	// is, the device has transmitted since the Cube's last restart.
	Reported bool   `json:"reported"`
	State    *State `json:"state,omitempty"`
	// Received is, per value of State, when the server saw its current
	// value appear (see cubelan.StateTimes): the age of each value. A
	// time equal to the Cube's observed_since means the value was there
	// already and may be older.
	Received map[string]time.Time `json:"received,omitempty"`
	// LastSeen is when the device's entry in the Cube's device list last
	// changed, the closest the Cube lets a client get to the device's
	// last transmission; a device that reports unchanged values does not
	// move it.
	LastSeen *time.Time `json:"last_seen,omitempty"`
	// Stale marks a thermostat whose entry has not changed for
	// StaleAfter: it may be out of range or out of battery, and its
	// values may be old. Shutter contacts and eco buttons transmit only
	// on events and are never stale.
	Stale bool `json:"stale,omitempty"`
	// Sources is a virtual wall thermostat's alone: the radio addresses
	// its actual_temp and its mode came from.
	Sources map[string][]string `json:"sources,omitempty"`
}

// StaleAfter is how long a thermostat's entry may stay unchanged before
// the device counts as stale. Radiator thermostats report the measured
// temperature only occasionally, but their valve or temperature changes
// several times a day while heating; outside the heating season they
// may well be quiet for longer, so stale is a hint, not a fault.
// It is the default; the functions below take the threshold as a
// parameter (serve passes the store's stale_after), where 0 or less
// means this default.
const StaleAfter = 24 * time.Hour

// now is the clock of the stale rule, replaced in tests.
var now = time.Now

// DeviceOf builds the view of one device from the metadata, its
// configuration and its live state. The metadata names the device and
// its room; the configuration, when present, has the last word on the
// type. staleAfter is the stale threshold (0: StaleAfter).
func DeviceOf(snap *cubelan.Snapshot, rf cube.RFAddress, staleAfter time.Duration) Device {
	if staleAfter <= 0 {
		staleAfter = StaleAfter
	}
	st, _ := snap.State(rf)
	d := Device{RFAddress: rf.String(), Type: st.Type}
	if md := snap.Metadata(); md != nil {
		found := false
		for _, m := range md.Devices {
			if m.RFAddress == rf {
				d.Name, d.RoomID, d.Serial, d.Type, found = m.Name, m.RoomID, m.Serial, m.Type, true
			}
		}
		d.Unlisted = !found
	}
	if c := snap.Config(rf); c != nil {
		d.Type = c.Type
		if d.Serial == "" {
			d.Serial, d.RoomID = c.Serial, c.RoomID
		}
	}
	d.Reported = st.Valid
	d.State = StateOf(st)
	if t, ok := snap.Times(rf); ok && st.Valid {
		changed := t.Changed
		d.LastSeen = &changed
		if len(t.Fields) > 0 {
			d.Received = t.Fields
		}
		thermostat := d.Type.IsThermostat()
		d.Stale = thermostat && now().Sub(changed) > staleAfter
	}
	return d
}

// Devices lists every visible device, in the order the Cube reported
// them, without the Cube itself; staleAfter as for DeviceOf.
func Devices(snap *cubelan.Snapshot, h Hidden, staleAfter time.Duration) []Device {
	var out []Device
	hello := snap.Hello()
	for _, rf := range snap.Devices() {
		if hello != nil && rf == hello.RFAddress || h[rf] {
			continue
		}
		out = append(out, DeviceOf(snap, rf, staleAfter))
	}
	return out
}

// Room is a visible room with its devices.
type Room struct {
	ID   uint8  `json:"id"`
	Name string `json:"name"`
	// UID is the room's uid in the server's store, kept for the room's
	// life; empty where the server has none.
	UID string `json:"room_uid,omitempty"`
	// Ready is true when every member of the room has reported since the
	// Cube's last restart; the Cube discards room commands otherwise.
	Ready bool `json:"ready"`
	// Stale is set when a device of the room is stale (Device.Stale).
	Stale   bool     `json:"stale,omitempty"`
	Devices []Device `json:"devices"`
}

// Rooms lists the visible rooms in the Cube's order, each with the
// visible devices the metadata places in it; staleAfter as for
// DeviceOf.
func Rooms(snap *cubelan.Snapshot, h Hidden, staleAfter time.Duration) []Room {
	md := snap.Metadata()
	if md == nil {
		return nil
	}
	devs := Devices(snap, h, staleAfter)
	var out []Room
	for _, r := range h.VisibleRooms(md) {
		room := Room{ID: r.ID, Name: r.Name, Ready: len(Silent(snap, r.ID, h)) == 0, Devices: []Device{}}
		for _, d := range devs {
			if d.RoomID == r.ID && !d.Unlisted {
				room.Devices = append(room.Devices, d)
				room.Stale = room.Stale || d.Stale
			}
		}
		out = append(out, room)
	}
	return out
}

// Silent lists the visible members of a room the Cube holds no state
// for: devices that have not transmitted since the Cube's last restart.
// Observed on hardware: the Cube discards room commands while a room has
// such a member. Membership is the device's group as its configuration
// (C line) gives it, which is what the Cube itself goes by; a device the
// Cube sent no configuration for counts by the metadata's room.
func Silent(snap *cubelan.Snapshot, room uint8, h Hidden) []cube.RFAddress {
	listed := map[cube.RFAddress]uint8{}
	if md := snap.Metadata(); md != nil {
		for _, d := range md.Devices {
			listed[d.RFAddress] = d.RoomID
		}
	}
	var out []cube.RFAddress
	for _, rf := range snap.Devices() {
		if h[rf] {
			continue
		}
		member := false
		if c := snap.Config(rf); c != nil {
			member = c.Type != cube.DeviceCube && c.RoomID == room
		} else if id, ok := listed[rf]; ok {
			member = id == room
		}
		if !member {
			continue
		}
		if st, ok := snap.State(rf); !ok || !st.Valid {
			out = append(out, rf)
		}
	}
	return out
}

// Counts returns the number of visible rooms and of visible listed
// devices.
func Counts(snap *cubelan.Snapshot, h Hidden) (rooms, devices int) {
	md := snap.Metadata()
	if md == nil {
		return 0, 0
	}
	for _, d := range md.Devices {
		if !h[d.RFAddress] {
			devices++
		}
	}
	return len(h.VisibleRooms(md)), devices
}

// Pending is the number of radiograms the Cube has sent and no device
// has acknowledged yet (memory slots in use); their effect is not in the
// device states yet. It comes from the budget when one was observed,
// else from the greeting.
func Pending(snap *cubelan.Snapshot, b cubelan.Budget) int {
	if b.UpdatedAt.IsZero() {
		if h := snap.Hello(); h != nil {
			b = cubelan.Budget{FreeMemorySlots: h.FreeMemorySlots, MaxSlots: h.FreeMemorySlots}
		}
	}
	return b.Pending()
}

// Change is one value of a device's state before and after a report; a
// value the device did not have is null.
type Change struct {
	From any `json:"from"`
	To   any `json:"to"`
}

// Changes lists what differs between two states of a device, by the
// names of State's fields, plus "reported" when the Cube gained or lost
// the device's state. It is nil when nothing differs; old is nil for a
// device the Cube held no state for.
func Changes(old, cur *State) map[string]Change {
	a, b := stateFields(old), stateFields(cur)
	var out map[string]Change
	add := func(k string, from, to any) {
		if out == nil {
			out = map[string]Change{}
		}
		out[k] = Change{From: from, To: to}
	}
	if (old == nil) != (cur == nil) {
		add("reported", old != nil, cur != nil)
	}
	for k, v := range b {
		if w, ok := a[k]; !ok || !sameValue(v, w) {
			add(k, a[k], v)
		}
	}
	for k, v := range a {
		if _, ok := b[k]; !ok {
			add(k, v, nil)
		}
	}
	return out
}

// stateFields is a state as its JSON object, nil for none.
func stateFields(s *State) map[string]any {
	if s == nil {
		return nil
	}
	raw, _ := json.Marshal(s)
	var m map[string]any
	json.Unmarshal(raw, &m)
	return m
}

func sameValue(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
