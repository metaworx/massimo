// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
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

// Commands on a virtual wall thermostat. A setpoint, a programme and a
// room's settings go to its room, as a real wall thermostat's reach its
// room; the key lock, the offset and the display go to the room's real
// wall thermostat, which has them; its own settings go to the store. It
// is renamed with its room and never moved or deleted.

// Which command a device route is, for onVirtual.
const (
	vwtSetpoint = "setpoint"
	vwtProgram  = "program"
	vwtConfig   = "config"
	vwtLock     = "lock"
	vwtUpdate   = "update"
	vwtRemove   = "remove"
)

// onVirtual carries out a device route's command on a room's virtual
// wall thermostat.
func (s *Server) onVirtual(w http.ResponseWriter, r *http.Request, rt *runtime, room cube.RoomMeta, op string) error {
	id := strconv.Itoa(int(room.ID))
	switch op {
	case vwtSetpoint:
		r.SetPathValue("id", id)
		return s.setRoomSetpoint(w, r)
	case vwtProgram:
		r.SetPathValue("id", id)
		return s.setRoomProgram(w, r)
	case vwtLock:
		wall, ok := roomWall(rt, room.ID)
		if !ok {
			return fail(http.StatusConflict, "room %d has no wall thermostat whose keys could be locked; lock its radiator thermostats one by one", room.ID)
		}
		r.SetPathValue("rf", wall.String())
		return s.setDeviceLock(w, r)
	case vwtConfig:
		return s.virtualConfig(w, r, rt, room)
	case vwtUpdate:
		return fail(http.StatusConflict, "the virtual wall thermostat of room %d goes with its room: rename the room (PUT …/rooms/%d)", room.ID, room.ID)
	}
	return fail(http.StatusConflict, "the virtual wall thermostat of room %d goes with its room: it is removed with the room", room.ID)
}

// roomWall is a room's real wall thermostat, the first where another tool
// put two there.
func roomWall(rt *runtime, room uint8) (cube.RFAddress, bool) {
	_, walls := roomThermostats(rt, room)
	if len(walls) == 0 {
		return 0, false
	}
	return walls[0], true
}

// The fields of a configuration write that are a real wall thermostat's
// own, and those that are the virtual wall thermostat's settings.
var (
	wallOnlyFields = map[string]bool{"temp_offset": true, "display_actual": true}
	settingFields  = map[string]bool{"temp_method": true, "rf_address": true}
)

// virtualConfig writes a virtual wall thermostat's configuration: the
// radio fields to the room, temp_offset and display_actual to the real
// wall thermostat, the settings to the store. A body of one part answers
// as that part's route does; one of several answers each part by name.
func (s *Server) virtualConfig(w http.ResponseWriter, r *http.Request, rt *runtime, room cube.RoomMeta) error {
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) == 0 {
		return fail(http.StatusBadRequest, "body must be a JSON object of settings")
	}
	parts := map[string]map[string]json.RawMessage{}
	for k, v := range body {
		part := "room"
		switch {
		case wallOnlyFields[k]:
			part = "wall_thermostat"
		case settingFields[k]:
			part = "settings"
		}
		if parts[part] == nil {
			parts[part] = map[string]json.RawMessage{}
		}
		parts[part][k] = v
	}
	var wall cube.RFAddress
	if parts["wall_thermostat"] != nil {
		var ok bool
		if wall, ok = roomWall(rt, room.ID); !ok {
			return fail(http.StatusBadRequest, "room %d has no wall thermostat: temp_offset and display_actual are a real thermostat's", room.ID)
		}
	}
	var set *store.RoomSettings
	if p := parts["settings"]; p != nil {
		var err error
		if set, err = s.checkVirtualSettings(rt, room, p); err != nil {
			return err
		}
	}
	answers := map[string]any{}
	sub := func(part string, call func(http.ResponseWriter, *http.Request) error, pathKey, pathValue string) error {
		raw, _ := json.Marshal(parts[part])
		r2 := r.Clone(r.Context())
		r2.Body = io.NopCloser(bytes.NewReader(raw))
		r2.SetPathValue(pathKey, pathValue)
		rec := &recorder{header: http.Header{}}
		if err := call(rec, r2); err != nil {
			return err
		}
		answers[part] = json.RawMessage(rec.body.Bytes())
		return nil
	}
	if parts["room"] != nil {
		if err := sub("room", s.setRoomConfig, "id", strconv.Itoa(int(room.ID))); err != nil {
			return err
		}
	}
	if parts["wall_thermostat"] != nil {
		if err := sub("wall_thermostat", s.setDeviceConfig, "rf", wall.String()); err != nil {
			return err
		}
	}
	if set != nil {
		v, err := s.setVirtualSettings(r, rt, room, *set)
		if err != nil {
			return err
		}
		answers["settings"] = map[string]any{"ok": true, "settings": v}
	}
	if len(answers) == 1 {
		for _, a := range answers {
			writeJSON(w, http.StatusOK, a)
		}
		return nil
	}
	answers["ok"] = true
	writeJSON(w, http.StatusOK, answers)
	return nil
}

// checkVirtualSettings reads the settings of a configuration write: a
// temp_method of median or mean, an rf_address of six hex digits ("" for
// the room number) that no real device, the Cube or another room's
// virtual wall thermostat has.
func (s *Server) checkVirtualSettings(rt *runtime, room cube.RoomMeta, p map[string]json.RawMessage) (*store.RoomSettings, error) {
	var set store.RoomSettings
	if raw, ok := p["temp_method"]; ok {
		var m string
		if json.Unmarshal(raw, &m) != nil || m != view.TempMedian && m != view.TempMean {
			return nil, fail(http.StatusBadRequest, "temp_method: median or mean")
		}
		set.TempMethod = &m
	}
	if raw, ok := p["rf_address"]; ok {
		var a string
		if json.Unmarshal(raw, &a) != nil {
			return nil, fail(http.StatusBadRequest, "rf_address: six hex digits, or \"\" for the room number")
		}
		a = strings.ToLower(a)
		if a != "" {
			rf, err := cube.ParseRFAddress(a)
			if err != nil || len(a) != 6 {
				return nil, fail(http.StatusBadRequest, "rf_address: six hex digits, or \"\" for the room number")
			}
			snap := rt.client.Snapshot()
			if view.RealAddresses(snap)[rf] {
				return nil, fail(http.StatusConflict, "rf_address %s is a real device's", a)
			}
			if md := snap.Metadata(); md != nil {
				for id, other := range view.VirtualAddresses(snap, rt.hidden().VisibleRooms(md), rt.roomInfo()) {
					if other == rf && id != room.ID {
						return nil, fail(http.StatusConflict, "rf_address %s is the virtual wall thermostat's of room %d", a, id)
					}
				}
			}
		}
		set.RFAddress = &a
	}
	return &set, nil
}

// setVirtualSettings keeps a virtual wall thermostat's settings, in the
// store where the Cube has a row there, and announces them: a config
// event, and a state event where its state changes.
func (s *Server) setVirtualSettings(r *http.Request, rt *runtime, room cube.RoomMeta, set store.RoomSettings) (vwtSettings, error) {
	if st, id := rt.store, rt.storeID.Load(); st != nil && id != 0 {
		if _, err := st.SetRoomSettings(r.Context(), id, room.ID, set, principalOf(r).userID()); err != nil {
			if errors.Is(err, store.ErrNoRoom) {
				return vwtSettings{}, fail(http.StatusNotFound, "no room %d", room.ID)
			}
			return vwtSettings{}, err
		}
		s.syncRooms(r.Context(), rt)
	} else {
		rt.vwt.mu.Lock()
		ri := rt.vwt.info[room.ID]
		if set.TempMethod != nil {
			ri.TempMethod = *set.TempMethod
		}
		if set.RFAddress != nil {
			ri.RFAddress = 0
			if *set.RFAddress != "" {
				ri.RFAddress, _ = cube.ParseRFAddress(*set.RFAddress)
			}
		}
		rt.vwt.info[room.ID] = ri
		rt.vwt.mu.Unlock()
	}
	d := s.virtualDetail(rt, room)
	_, walls := roomThermostats(rt, room.ID)
	ev := event{Kind: "config", Device: d.RFAddress, Serial: d.Serial, Source: "device", Level: "info", Time: time.Now(),
		Summary: map[string]any{"changed": []string{"settings"}}, virtual: true, wallRoom: len(walls) > 0}
	rt.subsMu.Lock()
	rt.publishLocked(ev, 0)
	rt.subsMu.Unlock()
	s.publishVWTs(rt, 0)
	return d.Settings, nil
}

// recorder takes a route's answer for virtualConfig, which merges the
// answers of the routes it calls.
type recorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (rec *recorder) Header() http.Header         { return rec.header }
func (rec *recorder) Write(b []byte) (int, error) { return rec.body.Write(b) }
func (rec *recorder) WriteHeader(status int)      { rec.status = status }

// secondWall reports whether putting a wall thermostat rf into room would
// make it the room's second: the original software refuses that ("The
// maximum number of devices of this type has been reached for this
// room"), and so does Massimo.
func secondWall(md *cube.Metadata, rf cube.RFAddress, room uint8) (cube.RFAddress, bool) {
	for _, d := range md.Devices {
		if d.RFAddress != rf && d.RoomID == room && d.Type == cube.DeviceWallThermostat {
			return d.RFAddress, true
		}
	}
	return 0, false
}
