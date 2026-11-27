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
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/roomorder"
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
	api("DELETE /api/v1/cubes/{sn}/devices/{rf}", "structure", s.removeDevice)
	api("GET /api/v1/cubes/{sn}/rooms/{id}/config", "read", s.getRoomConfig)
	api("PUT /api/v1/cubes/{sn}/rooms/{id}/config", "configure", s.setRoomConfig)
	api("PUT /api/v1/cubes/{sn}/devices/{rf}/config", "configure", s.setDeviceConfig)
	api("GET /api/v1/cubes/{sn}/rooms/{id}/program", "read", s.getRoomProgram)
	api("PUT /api/v1/cubes/{sn}/rooms/{id}/program", "configure", s.setRoomProgram)
	api("PUT /api/v1/cubes/{sn}/devices/{rf}/program", "configure", s.setDeviceProgram)
	api("GET /api/v1/cubes/{sn}/devices/{rf}/program", "read", s.getDeviceProgram)
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
		return cube.RoomMeta{}, nil, refuse(http.StatusServiceUnavailable, codeCubeNotConnectedYet)
	}
	if room, ok := rt.hidden().Room(md, id); ok {
		return room, md, nil
	}
	return cube.RoomMeta{}, nil, refuse(http.StatusNotFound, codeRoomUnknown, id)
}

// visibleDevice returns a device's configuration, or a 404 error.
func (s *Server) visibleDevice(rt *runtime, r *http.Request) (cube.RFAddress, *cube.DeviceConfig, error) {
	rf, err := deviceOf(rt, r.PathValue("rf"))
	if err != nil {
		return 0, nil, err
	}
	c := rt.client.Snapshot().Config(rf)
	if c == nil || rt.hidden()[rf] {
		return 0, nil, refuse(http.StatusNotFound, codeDeviceUnknown, rf.String())
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

// commandError maps a queue result to an HTTP error with the code of
// its kind (errcodes.go), its English text the result's error as it
// is, naming the room members without state when a room command was
// discarded for that: by their addresses in no_state, and with the room
// in params. An error of no kind known is command_failed, its text the
// reason (cubeRefusal).
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
			msg := fmt.Sprintf("%v: the cube holds no state for room %d members %s (not reported since the cube's last restart); the cube refuses room commands until they report or are removed",
				res.Err, roomCmd, strings.Join(names, ", "))
			he := refuseSaying(status, msg, codeCommandDiscardedNoState, roomCmd, names).(*httpError)
			he.noState = names
			return he
		}
	}
	return cubeRefusal(status, res.Err)
}

// noThermostat refuses a request to room's thermostats, where the Cube
// has none in the room (cubeio.RoomAddressee, roomThermostats find none),
// with status (D4-A): room_thermostat_unplaced naming the first
// thermostat the Cube's list has in the room, which the repair "Place in
// its listed room" puts there; room_no_thermostat where the list has
// none in it either.
func noThermostat(rt *runtime, status int, room uint8) error {
	for _, d := range cubeio.RoomMembers(rt.client.Snapshot(), room, rt.hidden()) {
		if d.Type.IsThermostat() {
			return refuse(status, codeRoomThermostatUnplaced, room, d.RFAddress.String())
		}
	}
	return refuse(status, codeRoomNoThermostat, room)
}

// writeMetadata sends the metadata to the Cube and mirrors it into the
// snapshot, which the Cube does not echo.
func (s *Server) writeMetadata(ctx context.Context, rt *runtime, md *cube.Metadata) error {
	res, err := rt.writerOf(ctx).WriteMetadata(ctx, md)
	if err != nil {
		return passed(http.StatusBadRequest, err, codeMetadataNotWritten)
	}
	if res.Err != nil {
		return cubeRefusal(http.StatusBadGateway, res.Err)
	}
	return nil
}

func checkName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return refuse(http.StatusBadRequest, codeNameEmpty)
	}
	if !utf8.ValidString(name) || len(name) > maxNameBytes {
		return refuse(http.StatusBadRequest, codeNameTooLong, maxNameBytes)
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
	var req struct {
		Name *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == nil {
		return fail(http.StatusBadRequest, "body must be {\"name\": \"…\"}")
	}
	if err := checkName(*req.Name); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	unlock, err := rt.lockStructure(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	room, md, err := s.visibleRoom(rt, r)
	if err != nil {
		return err
	}
	cp := *md
	cp.Rooms = append([]cube.RoomMeta(nil), md.Rooms...)
	for i := range cp.Rooms {
		if cp.Rooms[i].ID == room.ID {
			cp.Rooms[i].Name = strings.TrimSpace(*req.Name)
		}
	}
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
// way the original software adds a new device (cubeio.Writer.AddDevice),
// without a name under the default "<room name> <type> <n>" (D200-A).
// Either way a thermostat then gets the room's state and configuration
// (cubeio.Writer.CopySettings), which needs the capability configure as
// well, unless the body says "copy": false. The links and the metadata
// are written under the Cube's structure lock (lockStructure), which
// leaves every room out of the room checks, the copy after it; the
// room's state is read under the lock before the links, so that the new
// device's own cannot reach the room's members first, and held for the
// room checks once a radiogram of the link half reached them
// (roomBefore; AP RoomMajority, D8-A).
func (s *Server) updateDevice(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	if room, ok := virtualOf(rt, r.PathValue("rf")); ok {
		return s.onVirtual(w, r, rt, room, vwtUpdate)
	}
	rf, c, err := s.visibleDevice(rt, r)
	if err != nil {
		return err
	}
	var req struct {
		Name   *string `json:"name"`
		RoomID *int    `json:"room_id"`
		Copy   *bool   `json:"copy"`
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
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	unlock, err := rt.lockStructure(ctx)
	if err != nil {
		return err
	}
	locked := true
	defer func() {
		if locked {
			unlock()
		}
	}()
	md := rt.client.Snapshot().Metadata()
	if md == nil {
		return refuse(http.StatusServiceUnavailable, codeCubeNotConnectedYet)
	}
	idx := -1
	for i, d := range md.Devices {
		if d.RFAddress == rf {
			idx = i
		}
	}
	withCopy := (req.Copy == nil || *req.Copy) && c.Type.IsThermostat()
	var at placement
	if idx < 0 {
		at, err = s.addDevice(ctx, r, rt, md, c, req.Name, req.RoomID, withCopy)
	} else {
		at, err = s.moveDevice(ctx, r, rt, md, idx, c, req.Name, req.RoomID, withCopy)
	}
	if err != nil {
		return err
	}
	unlock()
	locked = false
	answer := map[string]any{"ok": true}
	if at.room != 0 && withCopy {
		copied, err := s.copySettings(ctx, rt, rf, c.Type, at.room, rt.copyState(at))
		if err != nil {
			return err
		}
		answer["copied"] = copied // the state its 0x40 carried, whatever else went
	}
	answer["budget"] = rt.client.Budget()
	if at.pending {
		answer["confirmation"] = "pending"
	}
	writeJSON(w, http.StatusOK, answer)
	return nil
}

// placement is what an add or a move did: the room it put the device
// into, 0 when the room stayed; whether the change is read back
// (structure.go); and the room's state from before it, which the copy
// gives the device (roomBefore), nil for none, and when it was read, a
// time of the room checks' clock.
type placement struct {
	room    uint8
	pending bool
	state   *cubeio.RoomState
	at      time.Time
}

// copyState is the state the copy gives a device put into a room: the
// room's from before the change (at.state), but where Massimo set the
// room another state since it was read, as by a room setpoint taken
// while the change's link half ran, that setpoint, Massimo's own command
// coming first (RoomMajority.D8-A, D4-A).
func (rt *runtime) copyState(at placement) *cubeio.RoomState {
	in, ok := rt.intentAfter(at.room, at.at)
	if !ok {
		return at.state
	}
	if st := at.state; st != nil && st.Mode == in.mode && st.Temp == in.temp && st.Until == in.until {
		return at.state
	}
	return &cubeio.RoomState{Mode: in.mode, Temp: in.temp, Until: in.until}
}

// before is a room's state before a structure change brings a thermostat
// into it (Server.roomBefore), held for the room checks only once the
// change's link half reached a member (hold).
type before struct {
	room uint8
	ref  resend
	at   time.Time
	ok   bool // the room has a state
}

// roomBefore reads room's state before a structure change brings the
// thermostat rf into it (roomReference, by the room checks' clock, rf
// left out where its C line names the room already): what the room's
// other members show before rf is linked, so that its own state, which
// may reach them first, does not count (AP RoomMajority, D8-A, D15-A).
// The caller holds the structure lock, which leaves every room out of
// the room checks (roomChecks: busy) until the change is through, and
// holds the state (before.hold) once the link half reached a member.
func (s *Server) roomBefore(rt *runtime, room uint8, rf cube.RFAddress) before {
	now := s.roomNow()
	ref, ok := rt.roomReference(rt.client.Snapshot(), room, rf, now)
	return before{room: room, ref: ref, at: now, ok: ok}
}

// state is the room's state for the copy, nil where it has none.
func (b before) state() *cubeio.RoomState {
	if !b.ok {
		return nil
	}
	return &cubeio.RoomState{Mode: b.ref.mode, Temp: b.ref.temp, Until: b.ref.until}
}

// hold holds the room's state for the room checks (holdReference) where
// reached says a radiogram of the link half may have reached a member
// (cubeio.Reached), which may then take the newcomer's state: where none
// did, nothing is held, and a hold of an earlier change stays as it is.
func (b before) hold(rt *runtime, reached bool) {
	if b.ok && reached {
		rt.holdReference(b.room, b.ref, b.at)
	}
}

// moveDevice renames a device the metadata lists, md.Devices[idx],
// and moves it to another room as the original software does; withCopy
// says that the copy follows, which needs configure. A thermostat's new
// room's state is read before the links and held once they reached a
// member of the room (roomBefore). It returns the placement, its room 0
// when the room stayed.
func (s *Server) moveDevice(ctx context.Context, r *http.Request, rt *runtime, md *cube.Metadata, idx int, c *cube.DeviceConfig, name *string, roomID *int, withCopy bool) (placement, error) {
	rf, cur := c.RFAddress, md.Devices[idx]
	var at placement
	newRoom := cur.RoomID
	if roomID != nil {
		if *roomID < 1 || *roomID > 255 {
			return placement{}, fail(http.StatusBadRequest, "room_id must be 1..255")
		}
		newRoom = uint8(*roomID)
		if _, ok := rt.hidden().Room(md, newRoom); !ok {
			return placement{}, refuse(http.StatusNotFound, codeRoomUnknown, newRoom)
		}
		if other, ok := secondWall(md, rf, newRoom); ok && c.Type == cube.DeviceWallThermostat && newRoom != cur.RoomID {
			return placement{}, refuse(http.StatusConflict, codeRoomWallTaken, newRoom, other.String())
		}
		if newRoom != cur.RoomID {
			if withCopy {
				if err := s.require(r, "configure"); err != nil {
					return placement{}, err
				}
			}
			var members []cube.DeviceMeta
			for _, d := range md.Devices {
				if d.RoomID == newRoom {
					members = append(members, d)
				}
			}
			var was before
			if c.Type.IsThermostat() {
				was = s.roomBefore(rt, newRoom, rf) // before the links
				at.state, at.at = was.state(), was.at
			}
			watch := rt.beginStructure("move", rf, cur.RoomID, newRoom)
			out, in := cubeio.MoveCommands(rt.client.Snapshot(), rf, c.Type, cur.RoomID, newRoom, members, rt.hidden())
			for _, cmd := range []engine.Command{out, in} {
				res := rt.enqueue(ctx, cmd).Wait(ctx)
				if cmd.Key == in.Key {
					was.hold(rt, cubeio.Reached(cmd, res)) // the link half reached the room
				}
				if err := s.commandError(rt, res, 0); err != nil {
					return placement{}, err
				}
			}
			at.pending = rt.watchStructure(watch)
		}
	}
	cp := *md
	cp.Devices = append([]cube.DeviceMeta(nil), md.Devices...)
	if name != nil {
		cp.Devices[idx].Name = strings.TrimSpace(*name)
	}
	cp.Devices[idx].RoomID = newRoom
	if err := s.writeMetadata(ctx, rt, &cp); err != nil {
		return placement{}, err
	}
	if newRoom == cur.RoomID {
		return placement{}, nil
	}
	rt.writer().Reread(ctx, rf)
	rt.afterWrite(rf) // the C line names the new room once the device acknowledged
	at.room = newRoom
	return at, nil
}

// copySettings gives a device just put into room the room's state from
// before (state, roomBefore's, nil for none) and its configuration
// (cubeio.Writer.CopySettings, the hidden devices left out), follows up
// the C lines it wrote and logs what was sent. A room command of the
// state, which follows a discarded 0x40 (RoomMajority.D16-A), counts as
// Massimo's own setpoint to the room (intend), which keeps the room's
// hold of the same state. Its error says that the device is in the room
// and why the copy stopped; what was copied until then is returned with
// it.
func (s *Server) copySettings(ctx context.Context, rt *runtime, rf cube.RFAddress, typ cube.DeviceType, room uint8, state *cubeio.RoomState) (cubeio.Copied, error) {
	copied, res, err := rt.writerOf(ctx).CopySettings(ctx, rf, typ, room, rt.hidden(), state)
	if err != nil {
		msg := fmt.Sprintf("device %s is in room %d, but the room's settings cannot be copied: %v", rf, room, err)
		return copied, refuseSaying(http.StatusBadGateway, msg, codeDeviceNotCopied, rf.String(), room, refusalOf(passed(http.StatusBadGateway, err, codeConfigNotEncodable)))
	}
	if st := copied.State; copied.RoomMode {
		rt.intend(room, st.Mode, st.Temp, st.Until, s.roomNow()) // the room's intended state (drift.go)
	}
	rt.afterWrite(copied.Temperatures...) // with none, the budget's follow-up of the 0x40
	attrs := []any{"cube", rt.cubeSerial(), "device", rf, "room", room, "mode", copied.State.Mode.String()}
	if copied.State.Temp != 0 {
		attrs = append(attrs, "setpoint", copied.State.Temp)
	}
	if !copied.State.Until.IsZero() {
		attrs = append(attrs, "until", copied.State.Until.String())
	}
	if copied.ConfigSkipped != "" {
		attrs = append(attrs, "config_skipped", copied.ConfigSkipped)
	}
	s.log.Info("settings copied", append(attrs, "temperatures", len(copied.Temperatures), "valve", copied.Valve, "days", copied.Days,
		"mode_not_set", copied.ModeNotSet, "room_mode", copied.RoomMode)...)
	if err := s.commandError(rt, res, 0); err != nil {
		// The result's refusal, with its code, is the reason the copy
		// stopped, with its status.
		status := http.StatusBadGateway
		var he *httpError
		if errors.As(err, &he) {
			status = he.status
		}
		msg := fmt.Sprintf("device %s is in room %d, but copying the room's settings stopped: %s", rf, room, err)
		return copied, refuseSaying(status, msg, codeDeviceNotCopied, rf.String(), room, refusalOf(err))
	}
	return copied, nil
}

// addDevice puts a device that is not in the metadata, one just paired
// or paired long ago and never listed, into a room, under name, else
// under defaultDeviceName's default; withCopy says that the copy
// follows, which needs configure. A thermostat's room's state is read
// before the links and held once they reached a member of the room
// (roomBefore). It returns the placement.
func (s *Server) addDevice(ctx context.Context, r *http.Request, rt *runtime, md *cube.Metadata, c *cube.DeviceConfig, name *string, roomID *int, withCopy bool) (placement, error) {
	if roomID == nil {
		return placement{}, refuse(http.StatusConflict, codeDeviceRoomNeeded, c.RFAddress.String())
	}
	if *roomID < 1 || *roomID > 255 {
		return placement{}, fail(http.StatusBadRequest, "room_id must be 1..255")
	}
	room := uint8(*roomID)
	target, ok := rt.hidden().Room(md, room)
	if !ok {
		return placement{}, refuse(http.StatusNotFound, codeRoomUnknown, room)
	}
	if c.Type == cube.DeviceCube {
		return placement{}, fail(http.StatusBadRequest, "the cube itself is not in a room")
	}
	if other, ok := secondWall(md, c.RFAddress, room); ok && c.Type == cube.DeviceWallThermostat {
		return placement{}, refuse(http.StatusConflict, codeRoomWallTaken, room, other.String())
	}
	if withCopy {
		if err := s.require(r, "configure"); err != nil {
			return placement{}, err
		}
	}
	// Without a name the device is listed under the default a repair's
	// listing gives (D200-A): "<room name> <type> <n>".
	n := defaultDeviceName(target.Name, c.Type, roomDeviceNames(md, room))
	if name != nil {
		n = strings.TrimSpace(*name)
	}
	at := placement{room: room}
	var was before
	if c.Type.IsThermostat() {
		was = s.roomBefore(rt, room, c.RFAddress) // before the links
		at.state, at.at = was.state(), was.at
	}
	watch := rt.beginStructure("add", c.RFAddress, room, c.RoomID)
	res, reached, err := rt.writerOf(ctx).AddDevice(ctx, md, c, n, room, rt.hidden())
	was.hold(rt, reached) // the link half reached the room
	if err != nil {
		return placement{}, passed(http.StatusBadRequest, err, codeMetadataNotWritten)
	}
	if err := s.commandError(rt, res, 0); err != nil {
		return placement{}, err
	}
	rt.afterWrite(c.RFAddress) // the C line names the room once the device acknowledged
	at.pending = rt.watchStructure(watch)
	return at, nil
}

// removeDevice handles DELETE /devices/{rf}: the device is deleted from
// the Cube as the original software deletes it
// (cubeio.Writer.RemoveDevice, the hidden devices left out), unlinked
// from its room's partners, t:, and the metadata without it; a device
// only the metadata lists, without a C line, loses its entry alone. The
// metadata write's event, or the device_forgotten one of a device in no
// room, tells the clients. It runs under the Cube's structure lock.
func (s *Server) removeDevice(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	if room, ok := virtualOf(rt, r.PathValue("rf")); ok {
		return s.onVirtual(w, r, rt, room, vwtRemove)
	}
	rf, err := deviceOf(rt, r.PathValue("rf"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	unlock, err := rt.lockStructure(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	snap := rt.client.Snapshot()
	md := snap.Metadata()
	hidden := rt.hidden()
	c := snap.Config(rf)
	var listed *cube.DeviceMeta
	if md != nil {
		for i := range md.Devices {
			if md.Devices[i].RFAddress == rf {
				listed = &md.Devices[i]
			}
		}
	}
	switch {
	case hidden[rf] || c == nil && listed == nil:
		return refuse(http.StatusNotFound, codeDeviceUnknown, rf.String())
	case c != nil && c.Type == cube.DeviceCube:
		return fail(http.StatusBadRequest, "%s is the cube itself", rf)
	case md == nil || !rt.client.Connected():
		return refuseSaying(http.StatusServiceUnavailable, "cube not connected", codeCubeNotConnected)
	}
	if c == nil {
		// A leftover of the metadata: the Cube holds no device to delete.
		cp := *md
		cp.Devices = nil
		for _, d := range md.Devices {
			if d.RFAddress != rf {
				cp.Devices = append(cp.Devices, d)
			}
		}
		if err := s.writeMetadata(ctx, rt, &cp); err != nil {
			return err
		}
		rt.client.ForgetDevice(rf)
		s.log.Info("device removed", "cube", rt.cubeSerial(), "device", rf, "type", listed.Type, "from", "metadata", "remote", s.clientIP(r))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "budget": rt.client.Budget()})
		return nil
	}
	room := c.RoomID
	if listed != nil && listed.RoomID != 0 {
		room = listed.RoomID
	}
	watch := rt.beginStructure("remove", rf, room)
	res, err := rt.writerOf(ctx).RemoveDevice(ctx, md, c, hidden)
	if err != nil {
		return passed(http.StatusBadRequest, err, codeMetadataNotWritten)
	}
	if err := s.commandError(rt, res, 0); err != nil {
		return err
	}
	rt.afterWrite() // the unlinks were radio writes
	s.log.Info("device removed", "cube", rt.cubeSerial(), "device", rf, "type", c.Type, "remote", s.clientIP(r))
	answer := map[string]any{"ok": true, "budget": rt.client.Budget()}
	if rt.watchStructure(watch) {
		answer["confirmation"] = "pending"
	}
	writeJSON(w, http.StatusOK, answer)
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
			return refuseSaying(http.StatusBadRequest, name+": "+err.Error(), codeTempInvalid, name)
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
		return refuse(http.StatusBadRequest, codeConfigOffsetInvalid)
	}
	if v := q.WindowOpenMins; v != nil && (*v < 0 || *v > 60 || *v%5 != 0) {
		return refuse(http.StatusBadRequest, codeConfigWindowInvalid)
	}
	if v := q.BoostMinutes; v != nil && !((*v >= 0 && *v <= 30 && *v%5 == 0) || *v == 60) {
		return refuse(http.StatusBadRequest, codeConfigBoostInvalid)
	}
	if v := q.BoostValvePct; v != nil && (*v < 0 || *v > 100 || *v%5 != 0) {
		return refuse(http.StatusBadRequest, codeConfigValveInvalid)
	}
	if v := q.DecalcDay; v != nil {
		var d cube.Weekday
		if err := d.UnmarshalText([]byte(*v)); err != nil {
			return refuseSaying(http.StatusBadRequest, "decalc_day: "+err.Error(), codeDecalcDayInvalid)
		}
	}
	if v := q.DecalcHour; v != nil && (*v < 0 || *v > 23) {
		return refuse(http.StatusBadRequest, codeConfigDecalcInvalid)
	}
	for name, v := range map[string]*int{"max_valve_pct": q.MaxValvePct, "valve_offset_pct": q.ValveOffsetPct} {
		if v != nil && (*v < 0 || *v > 100) {
			return refuse(http.StatusBadRequest, codeConfigPercentInvalid, name)
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
			return nil, passed(http.StatusBadRequest, err, codeConfigNotEncodable)
		}
		lines = append(lines, l)
	}
	if q.valve() {
		l, err := cube.CmdSetValveConfig(t, cube.ValveSettings{BoostMinutes: m.BoostMinutes, BoostValvePct: m.BoostValvePct, DecalcDay: m.DecalcDay,
			DecalcHour: m.DecalcHour, MaxValvePct: m.MaxValvePct, ValveOffsetPct: m.ValveOffsetPct})
		if err != nil {
			return nil, passed(http.StatusBadRequest, err, codeConfigNotEncodable)
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
	if room, ok := virtualOf(rt, r.PathValue("rf")); ok {
		return s.onVirtual(w, r, rt, room, vwtConfig)
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
	answer := map[string]any{"ok": true, "discarded": res.Discarded, "budget": res.Budget}
	if q.temperatures() || q.valve() {
		rt.afterWrite(rf)
	} else {
		rt.afterWrite() // the display mode is no part of the C line
	}
	// The C line shows the presets and the valve settings, the L entry the
	// display mode, once the device took them (confirm_config.go, confirm.go).
	confirmation := rt.configConfirm(ctx, rf, lines, "device:"+rf.String(), 0)
	if q.DisplayActual != nil {
		confirmation = combineAnswers(confirmation, rt.beginConfirm(&confirmWrite{what: writeDisplay, target: "device:" + rf.String(), source: "api",
			command: commandOf(ctx), want: map[string]any{fieldDisplay: *q.DisplayActual}}, []cube.RFAddress{rf}))
	}
	answer["confirmation"] = confirmation
	writeJSON(w, http.StatusOK, answer)
	return nil
}

// deviceResult is how a room's configuration went for one device: its
// error, where it was not sent, with the code of the command's error
// (failure).
type deviceResult struct {
	RFAddress string `json:"rf_address"`
	Name      string `json:"name,omitempty"`
	OK        bool   `json:"ok"`
	Discarded bool   `json:"discarded"`
	failure
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
		return noThermostat(rt, http.StatusConflict, room.ID)
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
			dr.OK, dr.failure = false, asFailure(saying("not sent: ", cubeRefusal(http.StatusBadGateway, res.Err)))
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
	answer := map[string]any{"ok": okCount == len(out), "devices": out, "budget": rt.client.Budget()}
	// Each device written is confirmed by its C line (confirm_config.go).
	confirmation := ""
	for _, rf := range written {
		var own []cube.Message
		for i, m := range lines {
			if per[i] == rf {
				own = append(own, m)
			}
		}
		confirmation = combineAnswers(confirmation, rt.configConfirm(ctx, rf, own, roomTarget(room.ID), room.ID))
	}
	if confirmation != "" {
		answer["confirmation"] = confirmation
	}
	writeJSON(w, http.StatusOK, answer)
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

// roomConfig is a room's configuration as the API shows it
// (RoomMajority.D19-A): cubeio.RoomConfig over the thermostats whose C
// line names the room, the hidden left out, by their configurations,
// config's (the snapshot's where config is nil), each group on its own,
// the temperatures, the valve settings and each day of the week
// programme what more than half of them share, else the first of the
// room's order's (its master, its wall thermostat, its radiator
// thermostats), one the Cube holds no valid state for, being out of
// date, passed over: the rule of the copy, the repair and the report of
// members that differ. While a thermostat of the room has not reported
// since the Cube's restart (unreported), as the report waits for them
// all (RoomMajority.D21-A), and where every thermostat is out of date,
// the same over them all, as the room's addressee is taken then
// (cubeio.Addressee), so that the room's configuration stays as it was
// while its thermostats report one by one: a configuration shown, unlike
// one copied or re-sent, is no reference that the dead could spread.
// order is the room's thermostats in that order. False where no
// thermostat of the room has a configuration.
func (rt *runtime) roomConfig(room uint8, config func(cube.RFAddress) *cube.DeviceConfig) (s cubeio.RoomSettings, order []cube.RFAddress, ok bool) {
	snap := rt.client.Snapshot()
	if config == nil {
		config = snap.Config
	}
	master, members := cubeio.RoomMaster(snap, room), cubeio.ConfigMembers(snap, room, rt.hidden())
	settled := !slices.ContainsFunc(members, func(d cube.DeviceMeta) bool { return d.Type.IsThermostat() && unreported(snap, d.RFAddress) })
	if settled {
		if s, ok = cubeio.RoomConfig(snap, master, members, 0, config); ok {
			return s, roomorder.Of(snap, master, members, 0), true
		}
	}
	s, ok = cubeio.RoomConfig(nil, master, members, 0, config)
	return s, roomorder.Of(nil, master, members, 0), ok
}

// getRoomConfig handles GET /rooms/{id}/config: the room's configuration
// (roomConfig), from source, the first thermostat of the room's order
// that has the room's temperatures, else the first of the order, and
// each thermostat's own offset and display. A room without a thermostat
// has no source, and no devices.
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
			d.TempOffset = c.Thermostat.TempOffset
		case c.Wall != nil:
			d.TempOffset = c.Wall.TempOffset
			if st, ok := snap.State(rf); ok && st.Valid {
				d.DisplayActual = st.DisplayActual
			}
		}
		v.Devices = append(v.Devices, d)
	}
	if len(v.Devices) == 0 {
		writeJSON(w, http.StatusOK, v)
		return nil
	}
	rs, order, ok := rt.roomConfig(room.ID, nil)
	if !ok {
		writeJSON(w, http.StatusOK, v)
		return nil
	}
	source := order[0]
	for _, rf := range order {
		if rs.SameTemperatures(snap.Config(rf)) {
			source = rf
			break
		}
	}
	t, vs := rs.Temps, rs.Valve
	v.Source = source.String()
	v.ComfortTemp, v.EcoTemp, v.MaxSetpoint, v.MinSetpoint, v.WindowOpenTemp = t.Comfort, t.Eco, t.Max, t.Min, t.WindowOpen
	v.WindowOpenMins = rs.WindowOpenMins
	v.BoostMinutes, v.BoostValvePct, v.DecalcDay, v.DecalcHour = vs.BoostMinutes, vs.BoostValvePct, vs.DecalcDay, vs.DecalcHour
	v.MaxValvePct, v.ValveOffsetPct = vs.MaxValvePct, vs.ValveOffsetPct
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
			return nil, refuseSaying(http.StatusBadRequest, fmt.Sprintf("%s: %v", d, err), codeProgramDayInvalid, d.String(), err.Error())
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
	if room, ok := virtualOf(rt, r.PathValue("rf")); ok {
		return s.onVirtual(w, r, rt, room, vwtProgram)
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
	if before, ok := rt.programOfDevice(rf, c); ok {
		if err := checkIfMatch(r, before.etag); err != nil {
			return err
		}
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
	if prog, _, ok := rt.programOf(rf); ok && !res.Discarded {
		rt.programs.note(rf, withDays(prog, body))
	}
	confirmation := rt.configConfirm(ctx, rf, lines, "device:"+rf.String(), 0) // confirm_config.go
	after, _ := rt.programOfDevice(rf, c)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "discarded": res.Discarded, "budget": res.Budget, "etag": after.etag, "confirmation": confirmation})
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
	addr, ok := cubeio.RoomAddressee(rt.client.Snapshot(), room.ID, rt.hidden())
	if !ok {
		return noThermostat(rt, http.StatusConflict, room.ID)
	}
	radiators, walls := roomThermostats(rt, room.ID)
	body, err := readBody(r)
	if err != nil {
		return err
	}
	before, _ := rt.programOfRoom(room.ID)
	if err := checkIfMatch(r, before.etag); err != nil {
		return err
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
	if !res.Discarded {
		for _, rf := range append(radiators, walls...) {
			if prog, _, ok := rt.programOf(rf); ok {
				rt.programs.note(rf, withDays(prog, body))
			}
		}
	}
	// Every thermostat of the room takes the days, confirmed by its own C
	// line (confirm_config.go).
	confirmation := ""
	for _, rf := range append(radiators, walls...) {
		confirmation = combineAnswers(confirmation, rt.configConfirm(ctx, rf, lines, roomTarget(room.ID), room.ID))
	}
	after, _ := rt.programOfRoom(room.ID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "discarded": res.Discarded, "budget": res.Budget, "etag": after.etag, "confirmation": confirmation})
	return nil
}

// getRoomProgram handles GET /rooms/{id}/program: the room's week
// programme, each day decided over its thermostats by the one order
// (programOfRoom).
func (s *Server) getRoomProgram(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	room, _, err := s.visibleRoom(rt, r)
	if err != nil {
		return err
	}
	return writeRoomProgram(w, rt, room.ID)
}

// writeRoomProgram answers a GET of a room's programme (programOfRoom),
// 404 where the room has no thermostat.
func writeRoomProgram(w http.ResponseWriter, rt *runtime, room uint8) error {
	rp, ok := rt.programOfRoom(room)
	if !ok {
		return noThermostat(rt, http.StatusNotFound, room)
	}
	return writeProgram(w, rp)
}

// getDeviceProgram handles GET /devices/{rf}/program: a thermostat's
// week programme with its tag, a room member's its room's; a virtual
// wall thermostat's is its room's.
func (s *Server) getDeviceProgram(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	if room, ok := virtualOf(rt, r.PathValue("rf")); ok {
		return writeRoomProgram(w, rt, room.ID)
	}
	rf, c, err := s.visibleDevice(rt, r)
	if err != nil {
		return err
	}
	rp, ok := rt.programOfDevice(rf, c)
	if !ok {
		return fail(http.StatusBadRequest, "device %s is a %s and has no week programme", rf, c.Type)
	}
	return writeProgram(w, rp)
}
