// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Configuration endpoints: everything the original software's per-room
// Configuration menu does. Names and room membership are metadata
// writes (m:), the rest are the radio commands the restore planner uses
// (0x11 temperatures, 0x12 boost/valve, 0x10 week programme, 0x82
// display mode), sent after a wakeup of the room as the software does.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// maxNameBytes is the longest room or device name accepted; the
// original software stops at 32 characters.
const maxNameBytes = 32

// configRoutes registers the configuration endpoints (names, moves,
// device and room configuration, week programmes) on the API wrapper.
func (s *Server) configRoutes(api func(pattern, capability string, h func(http.ResponseWriter, *http.Request) error)) {
	api("PUT /api/v1/cubes/{sn}/rooms/{id}", "rename", s.renameRoom)
	api("PUT /api/v1/cubes/{sn}/devices/{rf}", "rename", s.updateDevice)
	api("GET /api/v1/cubes/{sn}/rooms/{id}/config", "read", s.getRoomConfig)
	api("PUT /api/v1/cubes/{sn}/rooms/{id}/config", "configure", s.setRoomConfig)
	api("PUT /api/v1/cubes/{sn}/devices/{rf}/config", "configure", s.setDeviceConfig)
	api("GET /api/v1/cubes/{sn}/rooms/{id}/program", "read", s.getRoomProgram)
	api("PUT /api/v1/cubes/{sn}/rooms/{id}/program", "configure", s.setRoomProgram)
	api("PUT /api/v1/cubes/{sn}/devices/{rf}/program", "configure", s.setDeviceProgram)
}

// Lookups.

func roomID(r *http.Request) (uint8, error) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 || id > 255 {
		return 0, fail(http.StatusBadRequest, "bad room id %q", r.PathValue("id"))
	}
	return uint8(id), nil
}

// visibleRoom returns the room and the metadata, or a 404/503 error.
func (s *Server) visibleRoom(rt *runtime, r *http.Request) (cube.RoomMeta, *cube.Metadata, error) {
	id, err := roomID(r)
	if err != nil {
		return cube.RoomMeta{}, nil, err
	}
	md := rt.client.Snapshot().Metadata()
	if md == nil {
		return cube.RoomMeta{}, nil, fail(http.StatusServiceUnavailable, "cube not connected yet")
	}
	if room, ok := rt.hidden().Room(md, id); ok {
		return room, md, nil
	}
	return cube.RoomMeta{}, nil, fail(http.StatusNotFound, "no room %d", id)
}

// visibleDevice returns a device's configuration, or a 404 error.
func (s *Server) visibleDevice(rt *runtime, r *http.Request) (cube.RFAddress, *cube.DeviceConfig, error) {
	rf, err := cube.ParseRFAddress(r.PathValue("rf"))
	if err != nil {
		return 0, nil, fail(http.StatusBadRequest, "%v", err)
	}
	c := rt.client.Snapshot().Config(rf)
	if c == nil || rt.hidden()[rf] {
		return 0, nil, fail(http.StatusNotFound, "no device %s", rf)
	}
	return rf, c, nil
}

// roomThermostats lists the thermostats of a room: radiator thermostats
// first, then wall thermostats, in the Cube's order.
func roomThermostats(rt *runtime, room uint8) (radiators, walls []cube.RFAddress) {
	snap := rt.client.Snapshot()
	hidden := rt.hidden()
	for _, rf := range snap.Devices() {
		c := snap.Config(rf)
		if c == nil || hidden[rf] || c.RoomID != room {
			continue
		}
		switch {
		case c.Thermostat != nil:
			radiators = append(radiators, rf)
		case c.Wall != nil:
			walls = append(walls, rf)
		}
	}
	return radiators, walls
}

// Sending.

// run sends one command through the queue and answers like the setpoint
// endpoint: ok with the budget, or an error with the discard explanation.
func (s *Server) run(w http.ResponseWriter, r *http.Request, rt *runtime, cmd engine.Command, roomCmd uint8) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	res := rt.enqueue(ctx, cmd).Wait(ctx)
	if err := s.commandError(rt, res, roomCmd); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "discarded": res.Discarded, "budget": res.Budget})
	return nil
}

// commandError maps a queue result to an HTTP error, naming the room
// members without state when a room command was discarded.
func (s *Server) commandError(rt *runtime, res engine.Result, roomCmd uint8) error {
	if res.Err == nil {
		return nil
	}
	status := http.StatusBadGateway
	if errors.Is(res.Err, engine.ErrSuperseded) {
		status = http.StatusConflict
	}
	if errors.Is(res.Err, engine.ErrDiscarded) && roomCmd != 0 {
		if silent := view.Silent(rt.client.Snapshot(), roomCmd, rt.hidden()); len(silent) > 0 {
			var names []string
			for _, rf := range silent {
				names = append(names, rf.String())
			}
			he := fail(status, "%v: the cube holds no state for room %d members %s (not reported since the cube's last restart); the cube refuses room commands until they report or are removed",
				res.Err, roomCmd, strings.Join(names, ", ")).(*httpError)
			he.noState = names
			return he
		}
	}
	return fail(status, "%v", res.Err)
}

// writeMetadata sends the metadata to the Cube and mirrors it into the
// snapshot, which the Cube does not echo.
func (s *Server) writeMetadata(ctx context.Context, rt *runtime, md *cube.Metadata) error {
	res, err := rt.writerOf(ctx).WriteMetadata(ctx, md)
	if err != nil {
		return fail(http.StatusBadRequest, "%v", err)
	}
	if res.Err != nil {
		return fail(http.StatusBadGateway, "%v", res.Err)
	}
	return nil
}

func checkName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fail(http.StatusBadRequest, "name must not be empty")
	}
	if !utf8.ValidString(name) || len(name) > maxNameBytes {
		return fail(http.StatusBadRequest, "name must be valid text of at most %d bytes", maxNameBytes)
	}
	return nil
}

// Names and rooms.

// renameRoom handles PUT /rooms/{id} with {"name": "…"}.
func (s *Server) renameRoom(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	room, md, err := s.visibleRoom(rt, r)
	if err != nil {
		return err
	}
	var req struct {
		Name *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == nil {
		return fail(http.StatusBadRequest, "body must be {\"name\": \"…\"}")
	}
	if err := checkName(*req.Name); err != nil {
		return err
	}
	cp := *md
	cp.Rooms = append([]cube.RoomMeta(nil), md.Rooms...)
	for i := range cp.Rooms {
		if cp.Rooms[i].ID == room.ID {
			cp.Rooms[i].Name = strings.TrimSpace(*req.Name)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if err := s.writeMetadata(ctx, rt, &cp); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// updateDevice handles PUT /devices/{rf} with {"name": "…"} and/or
// {"room_id": N}. A room change is done as the original software does
// it: unlink from the old room's partners, relink with the new room's,
// set the group, then the metadata. A device that is in no room yet,
// such as one just paired, needs a room_id and is added to the room the
// way the original software adds a new device (cubeio.Writer.AddDevice).
func (s *Server) updateDevice(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	rf, c, err := s.visibleDevice(rt, r)
	if err != nil {
		return err
	}
	var req struct {
		Name   *string `json:"name"`
		RoomID *int    `json:"room_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Name == nil && req.RoomID == nil) {
		return fail(http.StatusBadRequest, "body must have \"name\" and/or \"room_id\"")
	}
	// The route admits rename and structure; the body says which one is
	// needed: a name is rename, a move changes the house and is structure.
	if req.Name != nil {
		if err := s.require(r, "rename"); err != nil {
			return err
		}
	}
	if req.RoomID != nil {
		if err := s.require(r, "structure"); err != nil {
			return err
		}
	}
	if req.Name != nil {
		if err := checkName(*req.Name); err != nil {
			return err
		}
	}
	md := rt.client.Snapshot().Metadata()
	if md == nil {
		return fail(http.StatusServiceUnavailable, "cube not connected yet")
	}
	idx := -1
	for i, d := range md.Devices {
		if d.RFAddress == rf {
			idx = i
		}
	}
	if idx < 0 {
		return s.addDevice(w, r, rt, md, c, req.Name, req.RoomID)
	}
	cur := md.Devices[idx]
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	newRoom := cur.RoomID
	if req.RoomID != nil {
		if *req.RoomID < 1 || *req.RoomID > 255 {
			return fail(http.StatusBadRequest, "room_id must be 1..255")
		}
		newRoom = uint8(*req.RoomID)
		if _, ok := rt.hidden().Room(md, newRoom); !ok {
			return fail(http.StatusNotFound, "no room %d", newRoom)
		}
		if newRoom != cur.RoomID {
			var members []cube.DeviceMeta
			for _, d := range md.Devices {
				if d.RoomID == newRoom {
					members = append(members, d)
				}
			}
			out, in := cubeio.MoveCommands(rt.client.Snapshot(), rf, c.Type, cur.RoomID, newRoom, members)
			for _, cmd := range []engine.Command{out, in} {
				res := rt.enqueue(ctx, cmd).Wait(ctx)
				if err := s.commandError(rt, res, 0); err != nil {
					return err
				}
			}
		}
	}
	cp := *md
	cp.Devices = append([]cube.DeviceMeta(nil), md.Devices...)
	if req.Name != nil {
		cp.Devices[idx].Name = strings.TrimSpace(*req.Name)
	}
	cp.Devices[idx].RoomID = newRoom
	if err := s.writeMetadata(ctx, rt, &cp); err != nil {
		return err
	}
	if newRoom != cur.RoomID {
		rt.writer().Reread(ctx, rf)
		rt.afterWrite(rf) // the C line names the new room once the device acknowledged
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "budget": rt.client.Budget()})
	return nil
}

// addDevice puts a device that is not in the metadata, one just paired
// or paired long ago and never listed, into a room.
func (s *Server) addDevice(w http.ResponseWriter, r *http.Request, rt *runtime, md *cube.Metadata, c *cube.DeviceConfig, name *string, roomID *int) error {
	if roomID == nil {
		return fail(http.StatusConflict, "device %s is in no room yet; give room_id to add it to one", c.RFAddress)
	}
	if *roomID < 1 || *roomID > 255 {
		return fail(http.StatusBadRequest, "room_id must be 1..255")
	}
	room := uint8(*roomID)
	if _, ok := rt.hidden().Room(md, room); !ok {
		return fail(http.StatusNotFound, "no room %d", room)
	}
	if c.Type == cube.DeviceCube {
		return fail(http.StatusBadRequest, "the cube itself is not in a room")
	}
	n := ""
	if name != nil {
		n = strings.TrimSpace(*name)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	res, err := rt.writerOf(ctx).AddDevice(ctx, md, c, n, room)
	if err != nil {
		return fail(http.StatusBadRequest, "%v", err)
	}
	if err := s.commandError(rt, res, 0); err != nil {
		return err
	}
	rt.afterWrite(c.RFAddress) // the C line names the room once the device acknowledged
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "budget": rt.client.Budget()})
	return nil
}

// Device and room configuration.

// configRequest is the body of the config endpoints; every field is
// optional and merged into the device's current configuration.
type configRequest struct {
	ComfortTemp    *float64 `json:"comfort_temp"`
	EcoTemp        *float64 `json:"eco_temp"`
	MaxSetpoint    *float64 `json:"max_setpoint"`
	MinSetpoint    *float64 `json:"min_setpoint"`
	TempOffset     *float64 `json:"temp_offset"`
	WindowOpenTemp *float64 `json:"window_open_temp"`
	WindowOpenMins *int     `json:"window_open_minutes"`
	BoostMinutes   *int     `json:"boost_minutes"`
	BoostValvePct  *int     `json:"boost_valve_pct"`
	DecalcDay      *string  `json:"decalc_day"`
	DecalcHour     *int     `json:"decalc_hour"`
	MaxValvePct    *int     `json:"max_valve_pct"`
	ValveOffsetPct *int     `json:"valve_offset_pct"`
	DisplayActual  *bool    `json:"display_actual"`
}

// temperatures reports whether the request carries any field that goes
// into the 0x11 temperature radiogram.
func (q configRequest) temperatures() bool {
	return q.ComfortTemp != nil || q.EcoTemp != nil || q.MaxSetpoint != nil || q.MinSetpoint != nil ||
		q.TempOffset != nil || q.WindowOpenTemp != nil || q.WindowOpenMins != nil
}

// valve reports whether the request carries any field that goes into
// the 0x12 valve and boost radiogram.
func (q configRequest) valve() bool {
	return q.BoostMinutes != nil || q.BoostValvePct != nil || q.DecalcDay != nil || q.DecalcHour != nil ||
		q.MaxValvePct != nil || q.ValveOffsetPct != nil
}

// validate checks the ranges the original software offers; the builders
// check the encodings again.
func (q configRequest) validate() error {
	temp := func(name string, v *float64) error {
		if v == nil {
			return nil
		}
		if _, err := cube.EncodeTemp(*v); err != nil {
			return fail(http.StatusBadRequest, "%s: %v", name, err)
		}
		return nil
	}
	for name, v := range map[string]*float64{"comfort_temp": q.ComfortTemp, "eco_temp": q.EcoTemp, "max_setpoint": q.MaxSetpoint,
		"min_setpoint": q.MinSetpoint, "window_open_temp": q.WindowOpenTemp} {
		if err := temp(name, v); err != nil {
			return err
		}
	}
	if v := q.TempOffset; v != nil && (*v < -3.5 || *v > 3.5 || *v*2 != float64(int(*v*2))) {
		return fail(http.StatusBadRequest, "temp_offset: must be a multiple of 0.5 between -3.5 and 3.5")
	}
	if v := q.WindowOpenMins; v != nil && (*v < 0 || *v > 60 || *v%5 != 0) {
		return fail(http.StatusBadRequest, "window_open_minutes: must be 0..60 in steps of 5")
	}
	if v := q.BoostMinutes; v != nil && !((*v >= 0 && *v <= 30 && *v%5 == 0) || *v == 60) {
		return fail(http.StatusBadRequest, "boost_minutes: must be 0..30 in steps of 5, or 60")
	}
	if v := q.BoostValvePct; v != nil && (*v < 0 || *v > 100 || *v%5 != 0) {
		return fail(http.StatusBadRequest, "boost_valve_pct: must be 0..100 in steps of 5")
	}
	if v := q.DecalcDay; v != nil {
		var d cube.Weekday
		if err := d.UnmarshalText([]byte(*v)); err != nil {
			return fail(http.StatusBadRequest, "decalc_day: %v", err)
		}
	}
	if v := q.DecalcHour; v != nil && (*v < 0 || *v > 23) {
		return fail(http.StatusBadRequest, "decalc_hour: must be 0..23")
	}
	for name, v := range map[string]*int{"max_valve_pct": q.MaxValvePct, "valve_offset_pct": q.ValveOffsetPct} {
		if v != nil && (*v < 0 || *v > 100) {
			return fail(http.StatusBadRequest, "%s: must be 0..100", name)
		}
	}
	return nil
}

// merged returns the thermostat settings with the request applied. For a
// wall thermostat, the values it does not store come from ref, a
// radiator thermostat of the room or the software's defaults.
func (q configRequest) merged(c *cube.DeviceConfig, ref cube.ThermostatConfig) cube.ThermostatConfig {
	var t cube.ThermostatConfig
	switch {
	case c.Thermostat != nil:
		t = *c.Thermostat
	case c.Wall != nil:
		w := c.Wall
		t = ref
		t.ComfortTemp, t.EcoTemp, t.MaxSetpoint, t.MinSetpoint = w.ComfortTemp, w.EcoTemp, w.MaxSetpoint, w.MinSetpoint
		t.TempOffset, t.WindowOpenTemp, t.BoostMinutes, t.BoostValvePct = w.TempOffset, w.WindowOpenTemp, w.BoostMinutes, w.BoostValvePct
	}
	set := func(dst *float64, v *float64) {
		if v != nil {
			*dst = *v
		}
	}
	seti := func(dst *int, v *int) {
		if v != nil {
			*dst = *v
		}
	}
	set(&t.ComfortTemp, q.ComfortTemp)
	set(&t.EcoTemp, q.EcoTemp)
	set(&t.MaxSetpoint, q.MaxSetpoint)
	set(&t.MinSetpoint, q.MinSetpoint)
	set(&t.TempOffset, q.TempOffset)
	set(&t.WindowOpenTemp, q.WindowOpenTemp)
	seti(&t.WindowOpenMins, q.WindowOpenMins)
	seti(&t.BoostMinutes, q.BoostMinutes)
	seti(&t.BoostValvePct, q.BoostValvePct)
	if q.DecalcDay != nil {
		t.DecalcDay.UnmarshalText([]byte(*q.DecalcDay)) // validated
	}
	seti(&t.DecalcHour, q.DecalcHour)
	seti(&t.MaxValvePct, q.MaxValvePct)
	seti(&t.ValveOffsetPct, q.ValveOffsetPct)
	return t
}

// configLines builds the radio commands that apply q to one device.
func configLines(rt *runtime, rf cube.RFAddress, c *cube.DeviceConfig, q configRequest) ([]cube.Message, error) {
	if c.Thermostat == nil && c.Wall == nil {
		return nil, fail(http.StatusBadRequest, "device %s is a %s and has no settings to write", rf, c.Type)
	}
	t := cube.Target{RFAddress: rf}
	m := q.merged(c, cubeio.RoomRadiator(rt.client.Snapshot(), c.RoomID, rf))
	var lines []cube.Message
	if q.temperatures() {
		l, err := cube.CmdSetTemperatures(t, cube.TemperatureSettings{Comfort: m.ComfortTemp, Eco: m.EcoTemp, Max: m.MaxSetpoint, Min: m.MinSetpoint,
			Offset: m.TempOffset, WindowOpen: m.WindowOpenTemp, WindowOpenMinutes: m.WindowOpenMins})
		if err != nil {
			return nil, fail(http.StatusBadRequest, "%v", err)
		}
		lines = append(lines, l)
	}
	if q.valve() {
		l, err := cube.CmdSetValveConfig(t, cube.ValveSettings{BoostMinutes: m.BoostMinutes, BoostValvePct: m.BoostValvePct, DecalcDay: m.DecalcDay,
			DecalcHour: m.DecalcHour, MaxValvePct: m.MaxValvePct, ValveOffsetPct: m.ValveOffsetPct})
		if err != nil {
			return nil, fail(http.StatusBadRequest, "%v", err)
		}
		lines = append(lines, l)
	}
	if q.DisplayActual != nil {
		if c.Wall == nil {
			return nil, fail(http.StatusBadRequest, "display_actual applies to wall thermostats only")
		}
		lines = append(lines, cube.CmdSetDisplayActual(t, *q.DisplayActual))
	}
	if len(lines) == 0 {
		return nil, fail(http.StatusBadRequest, "nothing to change")
	}
	return lines, nil
}

// setDeviceConfig handles PUT /devices/{rf}/config.
func (s *Server) setDeviceConfig(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	rf, c, err := s.visibleDevice(rt, r)
	if err != nil {
		return err
	}
	var q configRequest
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	if err := q.validate(); err != nil {
		return err
	}
	lines, err := configLines(rt, rf, c, q)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	res := rt.writerOf(ctx).Do(ctx, engine.Command{Key: "config:" + rf.String(), Target: rf, RoomID: c.RoomID, Wakeup: c.RoomID != 0, Lines: lines}, rf)
	if err := s.commandError(rt, res, 0); err != nil {
		return err
	}
	rt.afterWrite(rf)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "discarded": res.Discarded, "budget": res.Budget})
	return nil
}

type deviceResult struct {
	RFAddress string `json:"rf_address"`
	Name      string `json:"name,omitempty"`
	OK        bool   `json:"ok"`
	Discarded bool   `json:"discarded"`
	Error     string `json:"error,omitempty"`
}

// setRoomConfig handles PUT /rooms/{id}/config: temperatures go to every
// thermostat of the room, boost, descaling and valve settings to the
// radiator thermostats, after one wakeup of the room.
func (s *Server) setRoomConfig(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	room, md, err := s.visibleRoom(rt, r)
	if err != nil {
		return err
	}
	var q configRequest
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	if q.TempOffset != nil || q.DisplayActual != nil {
		return fail(http.StatusBadRequest, "temp_offset and display_actual are per device: use PUT /devices/{rf}/config")
	}
	if err := q.validate(); err != nil {
		return err
	}
	if !q.temperatures() && !q.valve() {
		return fail(http.StatusBadRequest, "nothing to change")
	}
	radiators, walls := roomThermostats(rt, room.ID)
	if len(radiators)+len(walls) == 0 {
		return fail(http.StatusConflict, "room %d has no thermostat", room.ID)
	}
	snap := rt.client.Snapshot()
	names := map[cube.RFAddress]string{}
	for _, d := range md.Devices {
		names[d.RFAddress] = d.Name
	}
	var lines []cube.Message
	var per []cube.RFAddress // device of each line
	for _, rf := range append(append([]cube.RFAddress(nil), radiators...), walls...) {
		c := snap.Config(rf)
		dq := q
		if c.Wall != nil {
			// Wall thermostats take the temperatures; boost too, but not
			// descaling and valve limits, which they do not have.
			dq.DecalcDay, dq.DecalcHour, dq.MaxValvePct, dq.ValveOffsetPct = nil, nil, nil, nil
			if !dq.temperatures() && !dq.valve() {
				continue
			}
		}
		ls, err := configLines(rt, rf, c, dq)
		if err != nil {
			return err
		}
		for range ls {
			per = append(per, rf)
		}
		lines = append(lines, ls...)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	res := rt.enqueue(ctx, engine.Command{Key: fmt.Sprintf("config:room:%d", room.ID), Target: per[0], RoomID: room.ID, Wakeup: true, Lines: lines}).Wait(ctx)
	// One result per device, from the reply to each of its lines.
	results := map[cube.RFAddress]*deviceResult{}
	var order []cube.RFAddress
	for i, rf := range per {
		dr := results[rf]
		if dr == nil {
			dr = &deviceResult{RFAddress: rf.String(), Name: names[rf], OK: true}
			results[rf] = dr
			order = append(order, rf)
		}
		switch {
		case i < len(res.Replies):
			if sr, err := cube.ParseSendResult(res.Replies[i].Data); err == nil && sr.Discarded {
				dr.OK, dr.Discarded = false, true
			}
		case res.Err != nil:
			dr.OK, dr.Error = false, "not sent: "+res.Err.Error()
		}
	}
	okCount := 0
	var out []deviceResult
	var written []cube.RFAddress
	for _, rf := range order {
		if results[rf].OK {
			okCount++
			rt.writer().Reread(ctx, rf)
			written = append(written, rf)
		}
		out = append(out, *results[rf])
	}
	if okCount > 0 {
		rt.afterWrite(written...)
	}
	if okCount == 0 {
		if err := s.commandError(rt, res, 0); err != nil {
			return err
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": okCount == len(out), "devices": out, "budget": rt.client.Budget()})
	return nil
}

// roomConfigView is the answer of GET /rooms/{id}/config.
type roomConfigView struct {
	ID             uint8              `json:"id"`
	Name           string             `json:"name"`
	Source         string             `json:"source,omitempty"` // device the common fields come from
	ComfortTemp    float64            `json:"comfort_temp"`
	EcoTemp        float64            `json:"eco_temp"`
	MaxSetpoint    float64            `json:"max_setpoint"`
	MinSetpoint    float64            `json:"min_setpoint"`
	WindowOpenTemp float64            `json:"window_open_temp"`
	WindowOpenMins int                `json:"window_open_minutes"`
	BoostMinutes   int                `json:"boost_minutes"`
	BoostValvePct  int                `json:"boost_valve_pct"`
	DecalcDay      cube.Weekday       `json:"decalc_day"`
	DecalcHour     int                `json:"decalc_hour"`
	MaxValvePct    int                `json:"max_valve_pct"`
	ValveOffsetPct int                `json:"valve_offset_pct"`
	Devices        []roomConfigDevice `json:"devices"`
}

type roomConfigDevice struct {
	RFAddress     string          `json:"rf_address"`
	Type          cube.DeviceType `json:"type"`
	Name          string          `json:"name"`
	TempOffset    float64         `json:"temp_offset"`
	DisplayActual *bool           `json:"display_actual,omitempty"`
}

// getRoomConfig handles GET /rooms/{id}/config.
func (s *Server) getRoomConfig(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	room, md, err := s.visibleRoom(rt, r)
	if err != nil {
		return err
	}
	snap := rt.client.Snapshot()
	names := map[cube.RFAddress]string{}
	for _, d := range md.Devices {
		names[d.RFAddress] = d.Name
	}
	v := roomConfigView{ID: room.ID, Name: room.Name, Devices: []roomConfigDevice{}}
	radiators, walls := roomThermostats(rt, room.ID)
	for _, rf := range append(append([]cube.RFAddress(nil), radiators...), walls...) {
		c := snap.Config(rf)
		d := roomConfigDevice{RFAddress: rf.String(), Type: c.Type, Name: names[rf]}
		switch {
		case c.Thermostat != nil:
			t := c.Thermostat
			d.TempOffset = t.TempOffset
			if v.Source == "" {
				v.Source = rf.String()
				v.ComfortTemp, v.EcoTemp, v.MaxSetpoint, v.MinSetpoint = t.ComfortTemp, t.EcoTemp, t.MaxSetpoint, t.MinSetpoint
				v.WindowOpenTemp, v.WindowOpenMins = t.WindowOpenTemp, t.WindowOpenMins
				v.BoostMinutes, v.BoostValvePct, v.DecalcDay, v.DecalcHour = t.BoostMinutes, t.BoostValvePct, t.DecalcDay, t.DecalcHour
				v.MaxValvePct, v.ValveOffsetPct = t.MaxValvePct, t.ValveOffsetPct
			}
		case c.Wall != nil:
			wc := c.Wall
			d.TempOffset = wc.TempOffset
			if st, ok := snap.State(rf); ok && st.Valid {
				d.DisplayActual = st.DisplayActual
			}
			if v.Source == "" {
				ref := cubeio.RoomRadiator(snap, room.ID, rf)
				v.Source = rf.String()
				v.ComfortTemp, v.EcoTemp, v.MaxSetpoint, v.MinSetpoint = wc.ComfortTemp, wc.EcoTemp, wc.MaxSetpoint, wc.MinSetpoint
				v.WindowOpenTemp, v.WindowOpenMins = wc.WindowOpenTemp, ref.WindowOpenMins
				v.BoostMinutes, v.BoostValvePct, v.DecalcDay, v.DecalcHour = wc.BoostMinutes, wc.BoostValvePct, ref.DecalcDay, ref.DecalcHour
				v.MaxValvePct, v.ValveOffsetPct = ref.MaxValvePct, ref.ValveOffsetPct
			}
		}
		v.Devices = append(v.Devices, d)
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}

// Week programmes.

// programLines builds one 0x10 radiogram (two for long days) per day
// present in the body.
func programLines(t cube.Target, body []byte) ([]cube.Message, error) {
	var prog cube.WeekProgram
	if err := json.Unmarshal(body, &prog); err != nil {
		return nil, fail(http.StatusBadRequest, "bad body: %v", err)
	}
	var lines []cube.Message
	for d := cube.Saturday; d <= cube.Friday; d++ {
		if len(prog[d]) == 0 {
			continue
		}
		ls, err := cube.CmdSetDayProgram(t, d, prog[d])
		if err != nil {
			return nil, fail(http.StatusBadRequest, "%s: %v", d, err)
		}
		lines = append(lines, ls...)
	}
	if len(lines) == 0 {
		return nil, fail(http.StatusBadRequest, "no day given")
	}
	return lines, nil
}

func readBody(r *http.Request) ([]byte, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return nil, fail(http.StatusBadRequest, "bad body: %v", err)
	}
	return raw, nil
}

// setDeviceProgram handles PUT /devices/{rf}/program.
func (s *Server) setDeviceProgram(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	rf, c, err := s.visibleDevice(rt, r)
	if err != nil {
		return err
	}
	if c.Thermostat == nil && c.Wall == nil {
		return fail(http.StatusBadRequest, "device %s is a %s and has no week programme", rf, c.Type)
	}
	body, err := readBody(r)
	if err != nil {
		return err
	}
	lines, err := programLines(cube.Target{RFAddress: rf}, body)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	res := rt.writerOf(ctx).Do(ctx, engine.Command{Key: "program:" + rf.String(), Target: rf, RoomID: c.RoomID, Wakeup: c.RoomID != 0, Lines: lines}, rf)
	if err := s.commandError(rt, res, 0); err != nil {
		return err
	}
	rt.afterWrite(rf)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "discarded": res.Discarded, "budget": res.Budget})
	return nil
}

// setRoomProgram handles PUT /rooms/{id}/program: one group radiogram per
// day, which every thermostat of the room takes, as the original software
// sends it.
func (s *Server) setRoomProgram(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	room, _, err := s.visibleRoom(rt, r)
	if err != nil {
		return err
	}
	radiators, walls := roomThermostats(rt, room.ID)
	if len(radiators)+len(walls) == 0 {
		return fail(http.StatusConflict, "room %d has no thermostat", room.ID)
	}
	body, err := readBody(r)
	if err != nil {
		return err
	}
	addr := room.RFAddress
	if addr == 0 {
		addr = append(radiators, walls...)[0]
	}
	lines, err := programLines(cube.Target{RFAddress: addr, RoomID: room.ID, Room: true}, body)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	res := rt.writerOf(ctx).Do(ctx, engine.Command{Key: fmt.Sprintf("program:room:%d", room.ID), RoomID: room.ID, Wakeup: true, Lines: lines}, append(radiators, walls...)...)
	if err := s.commandError(rt, res, room.ID); err != nil {
		return err
	}
	rt.afterWrite(append(radiators, walls...)...)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "discarded": res.Discarded, "budget": res.Budget})
	return nil
}

// getRoomProgram handles GET /rooms/{id}/program: the programme of the
// room's first radiator thermostat, else its first wall thermostat.
func (s *Server) getRoomProgram(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	room, _, err := s.visibleRoom(rt, r)
	if err != nil {
		return err
	}
	radiators, walls := roomThermostats(rt, room.ID)
	all := append(radiators, walls...)
	if len(all) == 0 {
		return fail(http.StatusNotFound, "room %d has no thermostat", room.ID)
	}
	c := rt.client.Snapshot().Config(all[0])
	var prog cube.WeekProgram
	if c.Thermostat != nil {
		prog = c.Thermostat.WeekProgram
	} else {
		prog = c.Wall.WeekProgram
	}
	days, err := json.Marshal(prog)
	if err != nil {
		return fail(http.StatusInternalServerError, "week programme of %s: %v", all[0], err)
	}
	var out map[string]any
	json.Unmarshal(days, &out)
	out["source"] = all[0].String()
	writeJSON(w, http.StatusOK, out)
	return nil
}
