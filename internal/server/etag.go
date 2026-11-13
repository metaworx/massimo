// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// The week programme's tag (docs/api-v1.md, "Week programmes and their
// tag"): an editor that read a programme sends its tag back with its
// write, and a programme changed since, in another client or at a
// device, is refused with 412 instead of being overwritten unseen. The
// tag covers every thermostat of the room, since a room's programme is
// its members' and a write to one of them changes the room. A programme
// written and not yet in the devices' configurations, which the server
// re-reads a few minutes after a write, counts as written: the room
// shows it, marked pending, under its own tag.

// pendingTimeout is how long a written programme counts as written
// without the devices' configurations showing it; then the server shows
// what they show.
const pendingTimeout = 10 * time.Minute

// pendingPrograms is the programmes written and not yet confirmed, by
// device.
type pendingPrograms struct {
	mu  sync.Mutex
	set map[cube.RFAddress]pendingProgram
	// cfg is each thermostat's configuration as its last config event
	// had it, for what the next one changed.
	cfg map[cube.RFAddress]cube.DeviceConfig
}

type pendingProgram struct {
	prog  cube.WeekProgram
	until time.Time
}

// note records the programme a write leaves a device with: its current
// one with the days written replaced.
func (p *pendingPrograms) note(rf cube.RFAddress, prog cube.WeekProgram) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.set == nil {
		p.set = map[cube.RFAddress]pendingProgram{}
	}
	p.set[rf] = pendingProgram{prog: prog, until: time.Now().Add(pendingTimeout)}
}

// programOf is a device's week programme as the server shows it: the
// one written where it is still on its way, else its configuration's;
// pending says which. A configuration that shows the written programme,
// or a write older than pendingTimeout, ends the pending one.
func (rt *runtime) programOf(rf cube.RFAddress) (prog cube.WeekProgram, pending, ok bool) {
	c := rt.client.Snapshot().Config(rf)
	switch {
	case c == nil:
		return prog, false, false
	case c.Thermostat != nil:
		prog = c.Thermostat.WeekProgram
	case c.Wall != nil:
		prog = c.Wall.WeekProgram
	default:
		return prog, false, false
	}
	p := &rt.programs
	p.mu.Lock()
	defer p.mu.Unlock()
	if w, had := p.set[rf]; had {
		if samePrograms(w.prog, prog) || time.Now().After(w.until) {
			delete(p.set, rf)
		} else {
			return w.prog, true, true
		}
	}
	return prog, false, true
}

// samePrograms reports whether two week programmes hold the same points.
func samePrograms(a, b cube.WeekProgram) bool { return reflect.DeepEqual(a, b) }

// withDays is a week programme with the days of a write's body replaced.
func withDays(prog cube.WeekProgram, body []byte) cube.WeekProgram {
	var days cube.WeekProgram
	if json.Unmarshal(body, &days) != nil {
		return prog
	}
	var given map[string]json.RawMessage
	json.Unmarshal(body, &given)
	out := prog
	for d := cube.Saturday; d <= cube.Friday; d++ {
		if _, ok := given[strings.ToLower(d.String())]; ok {
			out[d] = days[d]
		}
	}
	return out
}

// roomProgram is a room's week programme as the API shows it: its first
// radiator thermostat's (else its wall thermostat's), with its source, the
// tag over every member's, whether one is still on its way, and whether
// the members' programmes are the same.
type roomProgram struct {
	members    []cube.RFAddress
	source     cube.RFAddress
	prog       cube.WeekProgram
	etag       string
	pending    bool
	consistent bool
}

// programTag is the tag of a list of programmes, in their order.
func programTag(progs []cube.WeekProgram) string {
	raw, _ := json.Marshal(progs)
	sum := sha256.Sum256(raw)
	return "p" + hex.EncodeToString(sum[:8])
}

// programOfRoom is a room's programme as the API shows it; false where
// the room has no thermostat.
func (rt *runtime) programOfRoom(room uint8) (roomProgram, bool) {
	radiators, walls := roomThermostats(rt, room)
	rp := roomProgram{members: append(radiators, walls...), consistent: true}
	var progs []cube.WeekProgram
	for _, rf := range rp.members {
		prog, pending, ok := rt.programOf(rf)
		if !ok {
			continue
		}
		if len(progs) == 0 {
			rp.source, rp.prog = rf, prog
		} else if !samePrograms(progs[0], prog) {
			rp.consistent = false
		}
		progs = append(progs, prog)
		rp.pending = rp.pending || pending
	}
	if len(progs) == 0 {
		return rp, false
	}
	rp.etag = programTag(progs)
	return rp, true
}

// programOfDevice is a device's programme as the API shows it: a room
// member's tag is its room's, since a write to the room or to another
// member changes it too; a device in no room has its own.
func (rt *runtime) programOfDevice(rf cube.RFAddress, c *cube.DeviceConfig) (roomProgram, bool) {
	if c.RoomID != 0 {
		if rp, ok := rt.programOfRoom(c.RoomID); ok {
			prog, pending, _ := rt.programOf(rf)
			rp.source, rp.prog, rp.pending = rf, prog, pending
			return rp, true
		}
	}
	prog, pending, ok := rt.programOf(rf)
	if !ok {
		return roomProgram{}, false
	}
	return roomProgram{members: []cube.RFAddress{rf}, source: rf, prog: prog, etag: programTag([]cube.WeekProgram{prog}), pending: pending, consistent: true}, true
}

// writeProgram answers a GET of a programme: its days, the source, the
// tag in the body and in the ETag header, pending and
// program_consistent.
func writeProgram(w http.ResponseWriter, rp roomProgram) error {
	days, err := json.Marshal(rp.prog)
	if err != nil {
		return fail(http.StatusInternalServerError, "week programme of %s: %v", rp.source, err)
	}
	var out map[string]any
	json.Unmarshal(days, &out)
	out["source"] = rp.source.String()
	out["etag"] = rp.etag
	out["pending"] = rp.pending
	out["program_consistent"] = rp.consistent
	w.Header().Set("ETag", quoteTag(rp.etag))
	writeJSON(w, http.StatusOK, out)
	return nil
}

// checkIfMatch refuses a write whose If-Match names another tag than the
// current one with 412, carrying the current tag; a write without it is
// unconditional, "*" matches any.
func checkIfMatch(r *http.Request, current string) error {
	h := strings.TrimSpace(r.Header.Get("If-Match"))
	if h == "" || h == "*" {
		return nil
	}
	for _, t := range strings.Split(h, ",") {
		t = strings.TrimSpace(t)
		t = strings.TrimPrefix(t, "W/")
		if strings.Trim(t, `"`) == current {
			return nil
		}
	}
	return &httpError{status: http.StatusPreconditionFailed, etag: current,
		msg: "the week programme changed since it was read (or a write to it is still on its way): read it again, then write"}
}

// quoteTag is a tag as the ETag header carries it.
func quoteTag(t string) string { return `"` + t + `"` }

// configChanged says what of a thermostat's configuration its new C line
// changed against the one the runtime saw before: program and temps
// (the presets, boost and valve settings), with the programme's tag
// where it changed; nothing for a device's first one.
func (rt *runtime) configChanged(rf cube.RFAddress) ([]string, string) {
	c := rt.client.Snapshot().Config(rf)
	if c == nil || c.Thermostat == nil && c.Wall == nil {
		return nil, ""
	}
	p := &rt.programs
	p.mu.Lock()
	if p.cfg == nil {
		p.cfg = map[cube.RFAddress]cube.DeviceConfig{}
	}
	old, had := p.cfg[rf]
	p.cfg[rf] = *c
	p.mu.Unlock()
	if !had {
		return nil, ""
	}
	progOf := func(c cube.DeviceConfig) (cube.WeekProgram, any) {
		if c.Thermostat != nil {
			t := *c.Thermostat
			prog := t.WeekProgram
			t.WeekProgram = cube.WeekProgram{}
			return prog, t
		}
		w := *c.Wall
		prog := w.WeekProgram
		w.WeekProgram, w.DisplayActual, w.Extra = cube.WeekProgram{}, nil, nil
		return prog, w
	}
	op, ot := progOf(old)
	np, nt := progOf(*c)
	var changed []string
	etag := ""
	if !samePrograms(op, np) {
		changed = append(changed, "program")
		if rp, ok := rt.programOfDevice(rf, c); ok {
			etag = rp.etag
		}
	}
	if !reflect.DeepEqual(ot, nt) {
		changed = append(changed, "temps")
	}
	return changed, etag
}

// publishVirtualConfig announces a member's changed configuration as a
// config event of its room's virtual wall thermostat too, for a client
// that follows the room by it.
func (s *Server) publishVirtualConfig(rt *runtime, rf cube.RFAddress, changed []string, etag string) {
	c := rt.client.Snapshot().Config(rf)
	if c == nil || c.RoomID == 0 {
		return
	}
	md := rt.client.Snapshot().Metadata()
	if md == nil {
		return
	}
	room, ok := rt.hidden().Room(md, c.RoomID)
	if !ok {
		return
	}
	d := s.virtualDetail(rt, room)
	_, walls := roomThermostats(rt, room.ID)
	ev := event{Kind: "config", Device: d.RFAddress, Serial: d.Serial, Source: "device", Level: "info", Time: time.Now(),
		Changed: changed, Etag: etag, virtual: true, wallRoom: len(walls) > 0}
	rt.subsMu.Lock()
	rt.publishLocked(ev, 0)
	rt.subsMu.Unlock()
}
