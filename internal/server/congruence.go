// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The Cube's C lines against its metadata (AP RadioHealth v1.2, Block 7,
// D165-A). A device's room for the Cube is the room byte of its C line,
// not the metadata's: the previous owner's unlisted devices, their C
// lines in two rooms, made the Cube discard those rooms' commands. So at
// every complete greeting, at every C line and at every change of the
// metadata the server compares the two, which costs no radio time
// (view.Mismatch names the findings): a device whose C line names another
// room than the metadata; a C line in a room for a device the metadata
// does not list; a device the metadata lists that the Cube has no C line
// for, whatever its device list says (D194-A); a type that differs; a room whose master in the metadata, the address of its
// M entry, is no device of the Cube, a device of another room or no
// thermostat, where the room has a thermostat at all. Then, within each
// room, the thermostats by their C lines (D162-A, third tier): a member
// with a day of its week programme, or a comfort, eco, maximum, minimum
// or window-open temperature, that differs from the room's, as the one
// rule of a room's configuration decides it (cubeio.RoomConfig,
// RoomMajority.D9), which the copy and the repair take too: per group
// and per day, what most of the members the Cube holds a valid state for
// share, else what the first of the room's order has. The window-open
// minutes are not compared, which a wall thermostat does not keep. While
// a thermostat of the room has not reported since the Cube's restart
// (its L entry neither valid nor initialized, 0x10 and 0x02 both clear),
// every finding of the room's members stays as it was, neither found nor
// cleared, so that a restart of the Cube clears nothing and warns of
// nothing again as the members report one by one. Then a member whose
// entry the Cube marks out of date (initialized, not valid) is passed
// over (RoomMajority.D9) and its finding stays as it was, as do those of
// a room with fewer than two members the Cube holds a valid state for,
// which has no reference; the others are judged.
//
// Left out are the hidden devices and those parked in room 255, and the
// rooms they alone make up; the device of Massimo's own structure change
// (structure.go) and the rooms it concerns, from the change's start until
// the device's C line follows, at most mismatchSettle after its end
// (mismatchBusy after its start, for one that never ends); for the third
// tier, a room with a member whose configuration write the follow-up
// still follows (followup.go) or whose programme is on its way (etag.go);
// and the whole Cube while a restore runs.
//
// The checks run at every complete greeting, at every C line, at every
// change of the metadata, and when the Cube gains or loses a device's
// state.
//
// Each finding is the Cube's log event mismatch at warn, once, and a mark
// in the device's and its rooms' mismatches in the API, until a check
// finds the two agree: then the event at info, which settles the warning.
// A room master's warning is given again while it holds once someone has
// marked the last one seen (logseen.go), as the owner asked: warned only
// while no unread warning for it exists. At the first check the server
// takes the findings its log holds unsettled, so that a restart does not
// warn again.

import (
	"cmp"
	"context"
	"encoding/json"
	"maps"
	"slices"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/roomorder"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// The times of the checks.
const (
	// mismatchSettle is how long after Massimo's own structure change its
	// device and rooms are left out at most, while the device's C line
	// has not followed: the Cube changes it once the device acknowledged,
	// one to four minutes after a write.
	mismatchSettle = 5 * time.Minute
	// mismatchBusy bounds the leave-out of a structure change that
	// started and never ended, as one that failed.
	mismatchBusy = 15 * time.Minute
	// mismatchSeenEvery is how often a room master's warning that holds
	// is looked at for a seen mark.
	mismatchSeenEvery = time.Minute
	// mismatchScan is how many of the newest mismatch events the first
	// check reads.
	mismatchScan = 1000
)

// congruence is a Cube's findings, and the devices of Massimo's own
// structure changes left out.
type congruence struct {
	mu       sync.Mutex
	seeded   bool // the log's unsettled findings taken
	active   map[string]*activeMismatch
	settling map[cube.RFAddress]*settling
	checks   int // the checks run, which the tests wait for
}

// activeMismatch is a finding warned of and not cleared.
type activeMismatch struct {
	m view.Mismatch
	// seenAt is when its warning was last looked at for a seen mark (a
	// room master's).
	seenAt time.Time
}

// settling is the device of a structure change of Massimo's, left out
// with the rooms it concerns until its C line follows.
type settling struct {
	rooms []uint8
	room  int // the room of its C line at the change's start; -1 for none
	until time.Time
	ended bool
}

// settleMismatches leaves a device that a structure change of Massimo's
// is about to remove, place or move, and the rooms concerned, out of the
// checks (beginStructure).
func (rt *runtime) settleMismatches(device cube.RFAddress, rooms []uint8, now time.Time) {
	room := -1
	if c := rt.client.Snapshot().Config(device); c != nil {
		room = int(c.RoomID)
	}
	c := &rt.cong
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.settling == nil {
		c.settling = map[cube.RFAddress]*settling{}
	}
	c.settling[device] = &settling{rooms: slices.Clone(rooms), room: room, until: now.Add(mismatchBusy)}
}

// settledMismatches says that the structure change of a device ended,
// its lines answered (watchStructure): it stays left out until its C line
// follows, mismatchSettle at most.
func (rt *runtime) settledMismatches(device cube.RFAddress, now time.Time) {
	c := &rt.cong
	c.mu.Lock()
	defer c.mu.Unlock()
	if st := c.settling[device]; st != nil {
		st.until, st.ended = now.Add(mismatchSettle), true
	}
}

// lineFinding reports whether a finding is of a device's C line against
// its metadata entry, not of a room's master or members.
func lineFinding(finding string) bool {
	switch finding {
	case view.MismatchRoom, view.MismatchUnlisted, view.MismatchNoConfig, view.MismatchType:
		return true
	}
	return false
}

// leftOut reports whether a finding falls under a structure change still
// settling: its device's, or, for a room's master or members, its room's.
// c.mu is held.
func (c *congruence) leftOut(m view.Mismatch) bool {
	for rf, st := range c.settling {
		if m.Device == rf.String() || !lineFinding(m.Finding) && slices.ContainsFunc(st.rooms, m.Concerns) {
			return true
		}
	}
	return false
}

// followedRooms are the rooms with a member whose configuration write is
// still on its way: its C line followed after a write, or its programme
// pending (etag.go).
func (rt *runtime) followedRooms(snap *cubelan.Snapshot) map[uint8]bool {
	out := map[uint8]bool{}
	add := func(rf cube.RFAddress) {
		if c := snap.Config(rf); c != nil {
			out[c.RoomID] = true
		}
	}
	if f := rt.follow; f != nil {
		f.mu.Lock()
		for rf := range f.devices {
			add(rf)
		}
		f.mu.Unlock()
	}
	p := &rt.programs
	p.mu.Lock()
	for rf, w := range p.set {
		if time.Now().Before(w.until) {
			add(rf)
		}
	}
	p.mu.Unlock()
	return out
}

// findMismatches compares a snapshot's C lines with its metadata and the
// thermostats of each room with each other, the devices in hidden and
// room 255 left out, and the members of the rooms in busy. It reads each
// device's configuration once, so that a line applied meanwhile cannot
// take one away halfway. kept holds the keys of the members' findings
// that were not judged (memberMismatches), which stay as they are.
func findMismatches(snap *cubelan.Snapshot, md *cube.Metadata, hidden view.Hidden, busy map[uint8]bool) (out []view.Mismatch, kept map[string]bool) {
	names := map[uint8]string{}
	for _, r := range md.Rooms {
		names[r.ID] = r.Name
	}
	hiddenRooms := hidden.Rooms(md)
	listed := map[cube.RFAddress]cube.DeviceMeta{}
	for _, d := range md.Devices {
		listed[d.RFAddress] = d
	}
	devs := snap.Devices()
	cfgs := map[cube.RFAddress]*cube.DeviceConfig{}
	for _, rf := range devs {
		if c := snap.Config(rf); c != nil {
			cfgs[rf] = c
		}
	}
	at := func(r uint8) *uint8 { return &r }
	kept = map[string]bool{}
	members := map[uint8][]cube.RFAddress{} // each room's thermostats by their C lines
	for _, rf := range devs {
		c := cfgs[rf]
		d, isListed := listed[rf]
		switch {
		case hidden[rf], c != nil && (c.Type == cube.DeviceCube || c.RoomID == cube.HiddenRoom), isListed && d.RoomID == cube.HiddenRoom:
			continue
		case c == nil && isListed:
			out = append(out, view.Mismatch{Finding: view.MismatchNoConfig, Device: rf.String(), Name: d.Name, Room: d.RoomID, RoomName: names[d.RoomID],
				ListRoom: at(d.RoomID), ListRoomName: names[d.RoomID], ListType: d.Type.String()})
			continue
		case c == nil:
			continue
		case !isListed:
			if c.RoomID != 0 {
				out = append(out, view.Mismatch{Finding: view.MismatchUnlisted, Device: rf.String(), Room: c.RoomID, RoomName: names[c.RoomID],
					CubeRoom: at(c.RoomID), CubeRoomName: names[c.RoomID], CubeType: c.Type.String()})
			}
		default:
			if c.RoomID != d.RoomID {
				out = append(out, view.Mismatch{Finding: view.MismatchRoom, Device: rf.String(), Name: d.Name, Room: d.RoomID, RoomName: names[d.RoomID],
					CubeRoom: at(c.RoomID), CubeRoomName: names[c.RoomID], ListRoom: at(d.RoomID), ListRoomName: names[d.RoomID]})
			}
			if c.Type != d.Type {
				out = append(out, view.Mismatch{Finding: view.MismatchType, Device: rf.String(), Name: d.Name, Room: d.RoomID, RoomName: names[d.RoomID],
					CubeType: c.Type.String(), ListType: d.Type.String()})
			}
		}
		if c.Type.IsThermostat() && (c.Thermostat != nil || c.Wall != nil) {
			members[c.RoomID] = append(members[c.RoomID], rf)
		}
	}
	for _, r := range md.Rooms {
		if r.ID == cube.HiddenRoom || hiddenRooms[r.ID] || len(members[r.ID]) == 0 {
			continue
		}
		if m, ok := masterMismatch(cfgs, r, names, listed); ok {
			out = append(out, m)
		}
		if !busy[r.ID] {
			found, unjudged := memberMismatches(snap, cfgs, r, members[r.ID], names, listed)
			out = append(out, found...)
			for _, rf := range unjudged {
				for _, finding := range []string{view.MismatchProgramme, view.MismatchTemperatures} {
					kept[view.Mismatch{Finding: finding, Device: rf.String(), Room: r.ID}.Key()] = true
				}
			}
		}
	}
	return out, kept
}

// masterMismatch is the finding of a room's master in the metadata that
// is no device of the Cube, a device of another room, or no thermostat,
// by the configurations cfgs; false where it is a thermostat of the room.
func masterMismatch(cfgs map[cube.RFAddress]*cube.DeviceConfig, r cube.RoomMeta, names map[uint8]string, listed map[cube.RFAddress]cube.DeviceMeta) (view.Mismatch, bool) {
	m := view.Mismatch{Finding: view.MismatchMaster, Room: r.ID, RoomName: r.Name}
	if r.RFAddress != 0 {
		m.Device, m.Name = r.RFAddress.String(), listed[r.RFAddress].Name
	}
	c := cfgs[r.RFAddress]
	switch {
	case r.RFAddress == 0 || c == nil || c.Type == cube.DeviceCube:
		m.Why = "no_device"
	case c.RoomID != r.ID:
		room := c.RoomID
		m.Why, m.CubeRoom, m.CubeRoomName = "other_room", &room, names[room]
	case !c.Type.IsThermostat():
		m.Why, m.CubeType = "not_thermostat", c.Type.String()
	default:
		return view.Mismatch{}, false
	}
	return m, true
}

// memberMismatches are the thermostats rfs of a room with a day of their
// week programme or their temperatures, by the configurations cfgs, that
// differ from the room's: cubeio.RoomConfig's over them all, the one rule
// the copy and the repair take (RoomMajority.D9; the dead passed over by
// snap), so that a device fresh from its copy is none. unjudged lists
// those whose findings stay as they were: every member while one of
// them has not reported since the Cube's restart (unreported), else the
// dead (roomorder.Dead: out of date), and every member where fewer than
// two are alive, as the room then has no reference.
func memberMismatches(snap *cubelan.Snapshot, cfgs map[cube.RFAddress]*cube.DeviceConfig, r cube.RoomMeta, rfs []cube.RFAddress, names map[uint8]string, listed map[cube.RFAddress]cube.DeviceMeta) (out []view.Mismatch, unjudged []cube.RFAddress) {
	if len(rfs) < 2 {
		return nil, nil
	}
	if slices.ContainsFunc(rfs, func(rf cube.RFAddress) bool { return unreported(snap, rf) }) {
		return nil, rfs // the Cube's restart: nothing judged until every member reported
	}
	metas := make([]cube.DeviceMeta, 0, len(rfs))
	var alive []cube.RFAddress
	for _, rf := range rfs {
		metas = append(metas, cube.DeviceMeta{Type: cfgs[rf].Type, RFAddress: rf, RoomID: r.ID})
		if roomorder.Dead(snap, rf) {
			unjudged = append(unjudged, rf)
		} else {
			alive = append(alive, rf)
		}
	}
	if len(alive) < 2 {
		return nil, rfs // no reference: nothing of theirs is judged
	}
	ref, ok := cubeio.RoomConfig(snap, r.RFAddress, metas, 0, func(rf cube.RFAddress) *cube.DeviceConfig { return cfgs[rf] })
	if !ok {
		return nil, rfs
	}
	for _, k := range []struct {
		finding string
		differs func(*cube.DeviceConfig) bool
	}{
		{view.MismatchProgramme, func(c *cube.DeviceConfig) bool { return len(ref.Days(c)) > 0 }},
		{view.MismatchTemperatures, func(c *cube.DeviceConfig) bool { return !ref.SameTemperatures(c) }},
	} {
		for _, rf := range alive {
			if k.differs(cfgs[rf]) {
				out = append(out, view.Mismatch{Finding: k.finding, Device: rf.String(), Name: listed[rf].Name, Room: r.ID, RoomName: names[r.ID]})
			}
		}
	}
	return out, unjudged
}

// unreported reports whether the Cube holds nothing of rf since its
// restart: no L entry, or one neither valid nor initialized (0x10 and
// 0x02 of its first flags byte both clear), unlike one it marks out of
// date, initialized and not valid, of a device that fell silent.
func unreported(snap *cubelan.Snapshot, rf cube.RFAddress) bool {
	st, ok := snap.State(rf)
	return !ok || !st.Valid && !st.Initialized
}

// checkMismatches runs the checks on the Cube's snapshot and logs what
// changed: a new finding at warn, one that agrees again at info.
func (s *Server) checkMismatches(ctx context.Context, rt *runtime) {
	snap := rt.client.Snapshot()
	md := snap.Metadata()
	if md == nil || snap.Hello() == nil || rt.restoring() != nil {
		return
	}
	s.seedMismatches(ctx, rt)
	now := s.checkNow()
	found, kept := findMismatches(snap, md, view.NewHidden(rt.config().Hidden, md, false), rt.followedRooms(snap))
	byKey := map[string]view.Mismatch{}
	for _, m := range found {
		byKey[m.Key()] = m
	}
	c := &rt.cong
	c.mu.Lock()
	if c.active == nil {
		c.active = map[string]*activeMismatch{}
	}
	for rf, st := range c.settling {
		// the C line followed: it names another room than at the start,
		// or is gone, and the device has no finding of its own
		cur := -1
		if cfg := snap.Config(rf); cfg != nil {
			cur = int(cfg.RoomID)
		}
		own := slices.ContainsFunc(found, func(m view.Mismatch) bool { return m.Device == rf.String() && lineFinding(m.Finding) })
		if !now.Before(st.until) || st.ended && cur != st.room && !own {
			delete(c.settling, rf)
		}
	}
	var evs []event
	var seen []string // the room masters' warnings to look at for a seen mark
	for _, k := range slices.Sorted(maps.Keys(byKey)) {
		m := byKey[k]
		if a := c.active[k]; a != nil {
			m.Since = a.m.Since
			a.m = m // the names as they are now
			if m.Finding == view.MismatchMaster && now.Sub(a.seenAt) >= mismatchSeenEvery {
				a.seenAt = now
				seen = append(seen, k)
			}
			continue
		}
		if c.leftOut(m) {
			continue
		}
		m.Since = now
		c.active[k] = &activeMismatch{m: m, seenAt: now}
		evs = append(evs, mismatchEvent(snap, m, "found"))
	}
	for _, k := range slices.Sorted(maps.Keys(c.active)) {
		a := c.active[k]
		if _, still := byKey[k]; still || kept[k] || c.leftOut(a.m) { // kept: a member not judged now
			continue
		}
		delete(c.active, k)
		evs = append(evs, mismatchEvent(snap, a.m, "cleared"))
	}
	c.mu.Unlock()
	for _, k := range seen {
		if s.mismatchRead(ctx, rt, k) {
			c.mu.Lock()
			a := c.active[k]
			c.mu.Unlock()
			if a != nil {
				evs = append(evs, mismatchEvent(snap, a.m, "found"))
			}
		}
	}
	for _, ev := range evs {
		rt.emit(ev)
	}
	c.mu.Lock()
	c.checks++
	c.mu.Unlock()
}

// mismatchEvent is the log's mismatch event of a finding: found at warn,
// cleared at info; the summary is the finding, view.Mismatch's fields,
// and its state.
func mismatchEvent(snap *cubelan.Snapshot, m view.Mismatch, state string) event {
	summary := map[string]any{}
	raw, _ := json.Marshal(m)
	json.Unmarshal(raw, &summary)
	summary["state"] = state
	ev := event{Kind: "mismatch", Source: "cube", Level: "warn", Device: m.Device, Time: time.Now(), Summary: summary}
	if state != "found" {
		ev.Level = "info"
	}
	if rf, err := cube.ParseRFAddress(m.Device); err == nil && snap != nil {
		ev.Serial = snap.Serial(rf)
	}
	return ev
}

// mismatchOf is the finding a mismatch event's summary holds.
func mismatchOf(ev *event) view.Mismatch {
	var m view.Mismatch
	raw, _ := json.Marshal(ev.Summary)
	json.Unmarshal(raw, &m)
	return m
}

// sameMismatch: a later mismatch event that says the same finding
// agrees again.
func sameMismatch(alert, later *event) bool {
	return summaryText(later, "state") == "cleared" && mismatchOf(later).Key() == mismatchOf(alert).Key()
}

// seedMismatches takes, once, the findings the Cube's log holds
// unsettled: the newest event of each says found. A Cube without a row
// in the store has none to read.
func (s *Server) seedMismatches(ctx context.Context, rt *runtime) {
	c := &rt.cong
	c.mu.Lock()
	done := c.seeded
	c.mu.Unlock()
	if done {
		return
	}
	id := rt.storeID.Load()
	if rt.store == nil || id == 0 {
		c.mu.Lock()
		c.seeded = rt.store == nil
		c.mu.Unlock()
		return
	}
	evs, err := s.storedLog(ctx, rt.store, map[int64]string{id: rt.cubeSerial()}, logQuery{kind: "mismatch", minLevel: store.LevelDebug, limit: mismatchScan})
	if err != nil {
		s.log.Warn("mismatch: the log's findings not read", "cube", rt.cubeSerial(), "error", err)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seeded = true
	if c.active == nil {
		c.active = map[string]*activeMismatch{}
	}
	for i := range evs {
		m := mismatchOf(&evs[i])
		k := m.Key()
		switch summaryText(&evs[i], "state") {
		case "found":
			if since, err := time.Parse(time.RFC3339Nano, summaryText(&evs[i], "since")); err == nil {
				m.Since = since
			}
			c.active[k] = &activeMismatch{m: m}
		case "cleared":
			delete(c.active, k)
		}
	}
}

// mismatchRead reports whether someone marked the newest warning of a
// finding seen (logseen.go): any person's marks count. Without a store
// nobody's are known.
func (s *Server) mismatchRead(ctx context.Context, rt *runtime, key string) bool {
	st, id := s.opts.Store, rt.storeID.Load()
	if st == nil || id == 0 {
		return false
	}
	evs, err := s.storedLog(ctx, st, map[int64]string{id: rt.cubeSerial()}, logQuery{kind: "mismatch", minLevel: store.LevelWarn, limit: mismatchScan})
	if err != nil {
		return false
	}
	var line int64
	for i := len(evs) - 1; i >= 0 && line == 0; i-- {
		if mismatchOf(&evs[i]).Key() == key {
			line = evs[i].ID
		}
	}
	if line == 0 {
		return false
	}
	users, err := st.Users(ctx)
	if err != nil {
		return false
	}
	for _, u := range users {
		pr, ok, err := st.Preference(ctx, u.ID, seenPrefKey)
		if err != nil || !ok {
			continue
		}
		if m, err := parseSeen(pr.Value); err == nil && m.seen(line) {
			return true
		}
	}
	return false
}

// mismatchesOf are the findings warned of that match, oldest first.
func (rt *runtime) mismatchesOf(match func(view.Mismatch) bool) []view.Mismatch {
	c := &rt.cong
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []view.Mismatch
	for _, a := range c.active {
		if match(a.m) {
			out = append(out, a.m)
		}
	}
	slices.SortFunc(out, func(a, b view.Mismatch) int {
		if n := a.Since.Compare(b.Since); n != 0 {
			return n
		}
		return cmp.Compare(a.Key(), b.Key())
	})
	return out
}

// withMismatches gives each device of a list its findings.
func (rt *runtime) withMismatches(devs []view.Device) []view.Device {
	for i := range devs {
		rt.setMismatches(&devs[i])
	}
	return devs
}

// setMismatches gives a device's view its findings.
func (rt *runtime) setMismatches(d *view.Device) {
	if d.Virtual {
		return
	}
	d.Mismatches = rt.mismatchesOf(func(m view.Mismatch) bool { return m.Device == d.RFAddress })
}

// setRoomMismatches gives a room's view the findings that concern it,
// and its devices theirs.
func (rt *runtime) setRoomMismatches(r *view.Room) {
	r.Mismatches = rt.mismatchesOf(func(m view.Mismatch) bool { return m.Concerns(r.ID) })
	rt.withMismatches(r.Devices)
}
