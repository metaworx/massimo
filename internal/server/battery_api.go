// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"go.metaworx.ch/massimo/internal/battery"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// New batteries marked (POST /api/v1/cubes/{sn}/devices/{rf}/battery,
// AP RefreshBatteries v1.0, Block 3, D8-A): a device whose level is weak
// or low and whose flag is off, as the Cube shows it now, is ok at once,
// rather than when the tracker's rules say so (a low one an hour after
// its flag went). A device whose flag is still on has not reported its
// new batteries yet, and is refused. The change is a battery event as
// the tracker's are, its summary's cause causeReplaced and its user the
// one who asked, which the rebuild at a start replays (battery.go).
// Nothing goes to the Cube.

// causeReplaced is the cause in the summary of a battery event of new
// batteries marked.
const causeReplaced = "replaced"

// replaceBattery handles POST …/devices/{rf}/battery, whose body is
// {"replaced": true}: 404 for no such device, 400 for a virtual wall
// thermostat, the Cube itself or another body; 409 while the Cube holds
// no current report of the device, or the device's flag is on; else 200
// with the level, the one before and whether it changed, and a device
// that was weak or low is ok, its battery event in the log.
func (s *Server) replaceBattery(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	name := r.PathValue("rf")
	if room, ok := virtualOf(rt, name); ok {
		return fail(http.StatusBadRequest, "the virtual wall thermostat of room %d has no batteries", room.ID)
	} else if roomAlias.MatchString(name) {
		return noAliasRoom(name)
	}
	rf, err := deviceOf(rt, name)
	if err != nil {
		return err
	}
	snap := rt.client.Snapshot()
	st, ok := snap.State(rf)
	c := snap.Config(rf)
	if !ok && c == nil || rt.hidden()[rf] {
		return refuse(http.StatusNotFound, codeDeviceUnknown, rf.String())
	}
	if c != nil && c.Type == cube.DeviceCube || st.Type == cube.DeviceCube {
		return fail(http.StatusBadRequest, "device %s is the Cube, which has no batteries", rf)
	}
	if err := decodeReplaced(r); err != nil {
		return err
	}
	dv := view.DeviceOf(snap, rf, s.staleness())
	if dv.State == nil {
		// Since the Cube's restart, or for some minutes: the flag as the
		// Cube shows it now is not known.
		return refuse(http.StatusConflict, codeBatteryNoReport, rf.String())
	}
	still := refuse(http.StatusConflict, codeBatteryStillLow, rf.String())
	if dv.State.BatteryLow {
		return still
	}
	dev := rf.String()
	changes := rt.battery.Replaced(dev, s.now())
	if len(changes) == 0 {
		if rt.battery.Level(dev) != battery.OK {
			// The tracker has not read the report without the flag yet.
			return still
		}
		writeJSON(w, http.StatusOK, map[string]any{"battery": battery.OK, "from": battery.OK, "changed": false})
		return nil
	}
	ch := changes[0]
	if ev, ok := s.batteryEvent(rt, ch); ok {
		ev.Summary["cause"] = causeReplaced
		if p := principalOf(r); p.named() {
			ev.userID, ev.realUserID = p.user.ID, p.realUserID()
		}
		if a := auditOf(r); a != nil {
			ev.Command = a.commandID
		}
		s.emitBy(rt, ev)
		s.notifyEvent(rt, ev)
	}
	writeJSON(w, http.StatusOK, map[string]any{"battery": ch.To, "from": ch.From, "changed": true})
	return nil
}

// decodeReplaced reads the body of new batteries marked, which is
// {"replaced": true} and nothing else: its one key spelt so, true, and
// no more input after it.
func decodeReplaced(r *http.Request) error {
	var req map[string]any
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&req)
	if err == nil {
		if _, tail := dec.Token(); !errors.Is(tail, io.EOF) {
			err = errors.New("more after the object")
		}
	}
	if err != nil || len(req) != 1 || req["replaced"] != true {
		return fail(http.StatusBadRequest, `body must be {"replaced": true}`)
	}
	return nil
}
