// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// The virtual wall thermostats in the API (docs/api-v1.md, "The virtual
// wall thermostat"; view/vwt.go computes them). The runtime keeps each
// room's uid and settings, from the store where the Cube has a row there
// and in memory otherwise, in step with the Cube's metadata; the device
// lists and the event stream show the virtual wall thermostats as the
// request's view (?devices=) asks; and a change of a room's devices that
// changes its virtual wall thermostat is a state event of that one too.

// vwtState is a runtime's rooms and their virtual wall thermostats.
type vwtState struct {
	mu   sync.Mutex
	info map[uint8]view.RoomInfo // by room, through roomInfo
	// last is each room's virtual wall thermostat as its last event had
	// it, and sent that event's key; only the pump uses them.
	last map[uint8]view.Device
	sent map[uint8]string
}

// roomInfo is a copy of the rooms' uids and settings.
func (rt *runtime) roomInfo() map[uint8]view.RoomInfo {
	rt.vwt.mu.Lock()
	defer rt.vwt.mu.Unlock()
	out := make(map[uint8]view.RoomInfo, len(rt.vwt.info))
	for k, v := range rt.vwt.info {
		out[k] = v
	}
	return out
}

// syncRooms brings the rooms' uids and settings in step with the Cube's
// metadata: in the store where the Cube has a row there, else in
// memory, a new room getting a new uid and a room gone losing its own.
func (s *Server) syncRooms(ctx context.Context, rt *runtime) {
	md := rt.client.Snapshot().Metadata()
	if md == nil {
		return
	}
	var ids []uint8
	for _, r := range rt.hidden().VisibleRooms(md) {
		ids = append(ids, r.ID)
	}
	info := map[uint8]view.RoomInfo{}
	if st, id := rt.store, rt.storeID.Load(); st != nil && id != 0 {
		rows, err := st.SyncRooms(ctx, id, ids)
		if err != nil {
			s.log.Error("rooms not kept in step with the cube", "cube", rt.cubeSerial(), "error", err)
		} else {
			for _, r := range rows {
				ri := view.RoomInfo{UID: r.UUID, TempMethod: r.TempMethod}
				if r.RFAddress != "" {
					if a, err := cube.ParseRFAddress(r.RFAddress); err == nil {
						ri.RFAddress = a
					}
				}
				info[r.RoomID] = ri
			}
			rt.vwt.mu.Lock()
			rt.vwt.info = info
			rt.vwt.mu.Unlock()
			return
		}
	}
	rt.vwt.mu.Lock()
	defer rt.vwt.mu.Unlock()
	for _, id := range ids {
		ri, ok := rt.vwt.info[id]
		if !ok || ri.UID == "" {
			ri = view.RoomInfo{UID: uuid.NewString()}
		}
		info[id] = ri
	}
	rt.vwt.info = info
}

// kindOf is the request's view of the devices, ?devices=.
func kindOf(r *http.Request) (view.Kind, error) {
	k, err := view.ParseKind(r.URL.Query().Get("devices"))
	if err != nil {
		return "", fail(http.StatusBadRequest, "%v", err)
	}
	return k, nil
}

// roomsOf is the room list of a Cube as the API shows it in a view.
func (s *Server) roomsOf(rt *runtime, kind view.Kind) []view.Room {
	return view.RoomsOf(rt.client.Snapshot(), rt.hidden(), s.StaleAfter(), kind, rt.roomInfo())
}

// devicesOf is the device list of a Cube as the API shows it in a view.
func (s *Server) devicesOf(rt *runtime, kind view.Kind) []view.Device {
	return view.DevicesOf(rt.client.Snapshot(), rt.hidden(), s.StaleAfter(), kind, rt.roomInfo())
}

// roomAlias is a virtual wall thermostat named by its room, room-<id>.
var roomAlias = regexp.MustCompile(`^(?i)room-([0-9]{1,3})$`)

// virtualOf finds the virtual wall thermostat a request names, by its
// room (room-<id>), its serial or its radio address, and returns its
// room; false where the name is a real device's or none.
func virtualOf(rt *runtime, name string) (cube.RoomMeta, bool) {
	snap := rt.client.Snapshot()
	md := snap.Metadata()
	if md == nil {
		return cube.RoomMeta{}, false
	}
	rooms := rt.hidden().VisibleRooms(md)
	if m := roomAlias.FindStringSubmatch(name); m != nil {
		id, _ := strconv.Atoi(m[1])
		for _, r := range rooms {
			if int(r.ID) == id {
				return r, true
			}
		}
		return cube.RoomMeta{}, false
	}
	for _, r := range rooms {
		if strings.EqualFold(name, view.VirtualSerial(r.ID)) {
			return r, true
		}
	}
	rf, err := cube.ParseRFAddress(name)
	if err != nil || view.RealAddresses(snap)[rf] {
		return cube.RoomMeta{}, false
	}
	for id, a := range view.VirtualAddresses(snap, rooms, rt.roomInfo()) {
		if a == rf {
			for _, r := range rooms {
				if r.ID == id {
					return r, true
				}
			}
		}
	}
	return cube.RoomMeta{}, false
}

// vwtSettings are a virtual wall thermostat's settings as the API shows
// them.
type vwtSettings struct {
	TempMethod string `json:"temp_method"`
	RFAddress  string `json:"rf_address,omitempty"` // the owner's; absent for the room number
}

// vwtDetail is a virtual wall thermostat with its room's settings and
// week programme, in a wall thermostat's shape.
type vwtDetail struct {
	view.Device
	Wall     *cube.WallConfig `json:"wall_thermostat,omitempty"`
	Settings vwtSettings      `json:"settings"`
}

// virtualDetail is the answer of GET …/devices/{rf} for a room's virtual
// wall thermostat.
func (s *Server) virtualDetail(rt *runtime, room cube.RoomMeta) vwtDetail {
	snap := rt.client.Snapshot()
	info := rt.roomInfo()
	var d vwtDetail
	for _, r := range s.roomsOf(rt, view.KindAll) {
		if r.ID == room.ID && len(r.Devices) > 0 {
			d.Device = r.Devices[0]
		}
	}
	ri := info[room.ID]
	d.Settings.TempMethod = ri.TempMethod
	if d.Settings.TempMethod == "" {
		d.Settings.TempMethod = view.TempMedian
	}
	if ri.RFAddress != 0 {
		d.Settings.RFAddress = ri.RFAddress.String()
	}
	radiators, walls := roomThermostats(rt, room.ID)
	for _, rf := range append(walls, radiators...) {
		c := snap.Config(rf)
		switch {
		case c.Wall != nil:
			w := *c.Wall
			w.DisplayActual, w.Extra = nil, nil
			d.Wall = &w
		case c.Thermostat != nil:
			t := c.Thermostat
			d.Wall = &cube.WallConfig{ComfortTemp: t.ComfortTemp, EcoTemp: t.EcoTemp, MaxSetpoint: t.MaxSetpoint, MinSetpoint: t.MinSetpoint,
				TempOffset: t.TempOffset, WindowOpenTemp: t.WindowOpenTemp, BoostMinutes: t.BoostMinutes, BoostValvePct: t.BoostValvePct, WeekProgram: t.WeekProgram}
		}
		if d.Wall != nil {
			break
		}
	}
	return d
}

// publishVWTs publishes a state event for each virtual wall thermostat
// whose state changed, and device_forgotten for one whose room is gone;
// cmd names the command the change belongs to. They go to the stream
// alone: the log, the notifications and the proxy have the real
// devices' events.
func (s *Server) publishVWTs(rt *runtime, cmd int64) {
	vwts, walls := view.RoomThermostats(rt.client.Snapshot(), rt.hidden(), s.StaleAfter(), rt.roomInfo())
	if rt.vwt.last == nil {
		rt.vwt.last, rt.vwt.sent = map[uint8]view.Device{}, map[uint8]string{}
	}
	now := time.Now()
	var out []event
	for id, d := range vwts {
		key := vwtKey(d, walls[id])
		if rt.vwt.sent[id] == key {
			continue
		}
		ev := event{Kind: "state", Device: d.RFAddress, Serial: d.Serial, Source: "device", Level: "info", State: d, Time: now, Command: cmd,
			virtual: true, wallRoom: walls[id]}
		if prev, had := rt.vwt.last[id]; had && prev.RFAddress == d.RFAddress {
			ev.Changes = view.Changes(prev.State, d.State)
		}
		rt.vwt.last[id], rt.vwt.sent[id] = d, key
		out = append(out, ev)
	}
	for id, d := range rt.vwt.last {
		if _, ok := vwts[id]; ok {
			continue
		}
		delete(rt.vwt.last, id)
		delete(rt.vwt.sent, id)
		out = append(out, event{Kind: "device_forgotten", Device: d.RFAddress, Serial: d.Serial, Source: "device", Level: "info", State: d, Time: now, virtual: true})
	}
	if len(out) == 0 {
		return
	}
	rt.subsMu.Lock()
	for _, ev := range out {
		rt.publishLocked(ev, 0)
	}
	rt.subsMu.Unlock()
}

// vwtKey is what decides whether a virtual wall thermostat's state
// changed: its view, without its last_seen, which moves with every
// member's report, and whether its room has a real wall thermostat.
func vwtKey(d view.Device, wallRoom bool) string {
	d.LastSeen = nil
	raw, _ := json.Marshal(d)
	return strconv.FormatBool(wallRoom) + string(raw)
}

// realWall reports whether a device is a real wall thermostat in a room,
// which the view virtual leaves out.
func realWall(snap *cubelan.Snapshot, rf cube.RFAddress) bool {
	md := snap.Metadata()
	if md == nil {
		return false
	}
	for _, d := range md.Devices {
		if d.RFAddress == rf {
			return d.Type == cube.DeviceWallThermostat && d.RoomID != 0
		}
	}
	return false
}
