// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// What a line of the log says (AP Logging v1.1, Block 4): the
// differences of the Cube's room and device list (its M line) and of a
// device's configuration (its C line) against what the server held
// before, and the summaries of a greeting's lines and of a connection.
// The greeting brings every line again at each connection; its own lines
// go to the log at debug, and what differs follows the connected line at
// info.

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"time"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// listChange is one difference of the room and device list, as a
// metadata event's list_changes carry it.
type listChange struct {
	// What changed: room_added, room_renamed, room_deleted, device_added,
	// device_renamed, device_moved, device_left (out of the list).
	What   string `json:"what"`
	Room   uint8  `json:"room,omitempty"`   // the room's id
	Name   string `json:"name,omitempty"`   // the room's or the device's name, the new one of a rename
	From   string `json:"from,omitempty"`   // the name before a rename, the room before a move
	To     string `json:"to,omitempty"`     // the room after a move or an add
	Device string `json:"device,omitempty"` // a device's radio address
	Serial string `json:"serial,omitempty"`
}

// listChanges lists what differs between two room and device lists, the
// rooms first, then the devices, each in the order of the list; hidden
// devices, and the rooms that hold only them, are left out.
func listChanges(old, cur *cube.Metadata, hidden view.Hidden) []listChange {
	if old == nil || cur == nil {
		return nil
	}
	var out []listChange
	hiddenRooms := map[uint8]bool{}
	for id := range hidden.Rooms(cur) {
		hiddenRooms[id] = true
	}
	for id := range hidden.Rooms(old) {
		hiddenRooms[id] = true
	}
	roomName := func(md *cube.Metadata, id uint8) string {
		for _, r := range md.Rooms {
			if r.ID == id {
				return r.Name
			}
		}
		return ""
	}
	oldRooms := map[uint8]cube.RoomMeta{}
	for _, r := range old.Rooms {
		oldRooms[r.ID] = r
	}
	curRooms := map[uint8]bool{}
	for _, r := range cur.Rooms {
		curRooms[r.ID] = true
		if hiddenRooms[r.ID] {
			continue
		}
		was, had := oldRooms[r.ID]
		switch {
		case !had:
			out = append(out, listChange{What: "room_added", Room: r.ID, Name: r.Name})
		case was.Name != r.Name:
			out = append(out, listChange{What: "room_renamed", Room: r.ID, Name: r.Name, From: was.Name})
		}
	}
	for _, r := range old.Rooms {
		if !curRooms[r.ID] && !hiddenRooms[r.ID] {
			out = append(out, listChange{What: "room_deleted", Room: r.ID, Name: r.Name})
		}
	}
	oldDevices := map[cube.RFAddress]cube.DeviceMeta{}
	for _, d := range old.Devices {
		oldDevices[d.RFAddress] = d
	}
	curDevices := map[cube.RFAddress]bool{}
	for _, d := range cur.Devices {
		curDevices[d.RFAddress] = true
		if hidden[d.RFAddress] {
			continue
		}
		base := listChange{Device: d.RFAddress.String(), Serial: d.Serial, Name: d.Name}
		was, had := oldDevices[d.RFAddress]
		if !had {
			c := base
			c.What, c.Room, c.To = "device_added", d.RoomID, roomName(cur, d.RoomID)
			out = append(out, c)
			continue
		}
		if was.Name != d.Name {
			c := base
			c.What, c.From = "device_renamed", was.Name
			out = append(out, c)
		}
		if was.RoomID != d.RoomID {
			c := base
			c.What, c.Room, c.From, c.To = "device_moved", d.RoomID, roomName(old, was.RoomID), roomName(cur, d.RoomID)
			out = append(out, c)
		}
	}
	for _, d := range old.Devices {
		if !curDevices[d.RFAddress] && !hidden[d.RFAddress] {
			out = append(out, listChange{What: "device_left", Device: d.RFAddress.String(), Serial: d.Serial, Name: d.Name, From: roomName(old, d.RoomID)})
		}
	}
	return out
}

// configValues flattens a device's configuration into its values by
// their JSON names: the thermostat's, the wall thermostat's or the
// Cube's, its firmware, and one program_<day> per day of its week
// program.
func configValues(c *cube.DeviceConfig) map[string]any {
	m := map[string]any{"firmware": float64(c.Firmware)}
	var part any
	switch {
	case c.Thermostat != nil:
		part = c.Thermostat
	case c.Wall != nil:
		part = c.Wall
	case c.Cube != nil:
		part = c.Cube
	default:
		return m
	}
	b, _ := json.Marshal(part)
	var vals map[string]any
	json.Unmarshal(b, &vals)
	for k, v := range vals {
		switch k {
		case "extra":
		case "week_program":
			days, _ := v.(map[string]any) // keyed by the day's name (WeekProgram.MarshalJSON)
			for day, p := range days {
				m["program_"+day] = p
			}
		default:
			m[k] = v
		}
	}
	return m
}

// configChanges lists what differs between two configurations of a
// device, by the values' JSON names, as a state event's changes do; a
// day's program that changed is named without its values, which the
// device view has.
func configChanges(old, cur *cube.DeviceConfig) map[string]view.Change {
	if old == nil || cur == nil {
		return nil
	}
	a, b := configValues(old), configValues(cur)
	out := map[string]view.Change{}
	for k, to := range b {
		from, had := a[k]
		if had && reflect.DeepEqual(from, to) {
			continue
		}
		if len(k) > 8 && k[:8] == "program_" {
			out[k] = view.Change{}
			continue
		}
		out[k] = view.Change{From: from, To: to}
	}
	for k, from := range a {
		if _, has := b[k]; !has {
			out[k] = view.Change{From: from}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// visibleCounts counts the rooms and devices of a list the UI shows: the
// hidden devices, and the rooms holding only them, left out.
func visibleCounts(md *cube.Metadata, hidden view.Hidden) (rooms, devices int) {
	if md == nil {
		return 0, 0
	}
	for _, d := range md.Devices {
		if !hidden[d.RFAddress] {
			devices++
		}
	}
	return len(hidden.VisibleRooms(md)), devices
}

// helloSummary is what an H line says: the firmware, the radio budget.
func helloSummary(h *cube.Hello) map[string]any {
	if h == nil {
		return nil
	}
	return map[string]any{"firmware": h.FirmwareVersion, "duty_cycle_pct": h.DutyCycle, "free_memory_slots": h.FreeMemorySlots}
}

// listSummary is what an M line holds: its rooms and devices.
func listSummary(md *cube.Metadata, hidden view.Hidden) map[string]any {
	rooms, devices := visibleCounts(md, hidden)
	return map[string]any{"rooms": rooms, "devices": devices}
}

// configSummary is what a C line holds, in short: a thermostat's comfort,
// eco and offset temperatures, the Cube's portal and time zones.
func configSummary(c *cube.DeviceConfig) map[string]any {
	switch {
	case c == nil:
		return nil
	case c.Thermostat != nil:
		return map[string]any{"comfort_temp": c.Thermostat.ComfortTemp, "eco_temp": c.Thermostat.EcoTemp, "temp_offset": c.Thermostat.TempOffset}
	case c.Wall != nil:
		return map[string]any{"comfort_temp": c.Wall.ComfortTemp, "eco_temp": c.Wall.EcoTemp, "temp_offset": c.Wall.TempOffset}
	case c.Cube != nil:
		return map[string]any{"portal_enabled": c.Cube.PortalEnabled, "tz_winter": c.Cube.Winter.Name, "tz_summer": c.Cube.Summer.Name}
	}
	return map[string]any{"firmware": c.Firmware}
}

// connectedSummary is what a connection brings: the devices and rooms,
// the firmware, the radio budget, why the connection began and, after a
// loss, how long the Cube was lost.
func connectedSummary(snap *cubelan.Snapshot, hidden view.Hidden, e cubelan.Event) map[string]any {
	m := map[string]any{"cause": e.Cause}
	if e.Cause == "" {
		m["cause"] = cubelan.CauseStart
	}
	for k, v := range listSummary(snap.Metadata(), hidden) {
		m[k] = v
	}
	for k, v := range helloSummary(snap.Hello()) {
		m[k] = v
	}
	if e.Cause == cubelan.CauseLoss && e.DownFor > 0 {
		m["down_for_s"] = int(e.DownFor.Round(time.Second) / time.Second)
	}
	if e.Held {
		m["held"] = true // it resumed on the held connection
	}
	return m
}

// logRef is what the log compares a Cube's lines with: the room and
// device list and the configurations as the server last held them, read
// from the newest backup at the first look after a start. Only the
// runtime's pump touches it.
type logRef struct {
	looked  bool
	meta    *cube.Metadata
	configs map[cube.RFAddress]*cube.DeviceConfig
	held    []event // the differences a greeting found, sent after its connected line
}

// logReference returns the runtime's reference, reading the newest
// backup at its first look: after a start, what changed while the server
// was down shows as it does after a loss.
func (s *Server) logReference(ctx context.Context, rt *runtime) *logRef {
	ref := &rt.logRef
	if ref.looked {
		return ref
	}
	ref.looked = true
	ref.configs = map[cube.RFAddress]*cube.DeviceConfig{}
	st, id := rt.store, rt.storeID.Load()
	if st == nil || id == 0 {
		return ref
	}
	b, ok, err := st.NewestBackup(ctx, id)
	if err != nil || !ok {
		return ref
	}
	body, ok, err := st.BackupBody(ctx, id, b.ID)
	if err != nil || !ok {
		return ref
	}
	f, err := backup.Unmarshal(body, b.Format == "yaml")
	if err != nil {
		s.log.Debug("log: the newest backup not read as the reference", "cube", rt.cubeSerial(), "error", err)
		return ref
	}
	md := &cube.Metadata{Rooms: append([]cube.RoomMeta(nil), f.Rooms...)}
	for _, d := range f.Devices {
		if !d.Unlisted {
			md.Devices = append(md.Devices, cube.DeviceMeta{Type: d.Type, RFAddress: d.RFAddress, Serial: d.Serial, Name: d.Name, RoomID: d.RoomID})
		}
		ref.configs[d.RFAddress] = &cube.DeviceConfig{RFAddress: d.RFAddress, Type: d.Type, RoomID: d.RoomID, Firmware: d.Firmware, Serial: d.Serial,
			Thermostat: d.Thermostat, Wall: d.Wall}
	}
	if f.CubeConfig != nil && f.Cube.RFAddress != 0 {
		ref.configs[f.Cube.RFAddress] = &cube.DeviceConfig{RFAddress: f.Cube.RFAddress, Type: cube.DeviceCube, Serial: f.Cube.Serial, Cube: f.CubeConfig}
	}
	ref.meta = md
	return ref
}

// sortedKeys lists a map's keys in order, for the tests.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
