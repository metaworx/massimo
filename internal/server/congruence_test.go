// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The Cube's C lines against its metadata (AP RadioHealth v1.2, Block 7):
// each finding warned of once, marked on its device and its rooms and
// cleared when the two agree again; a room master's warned again once
// read; the members of a room whose programme or temperatures differ;
// the hidden, the parked and the device of Massimo's own structure change
// left out; the log's findings taken at the start.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/view"
)

// fixtureLines is the fixture's greeting with its metadata and its C
// lines decoded, for a test to change before it greets with them.
type fixtureLines struct {
	t     *testing.T
	lines []cube.Message // the fixture's, in order
	md    *cube.Metadata
	raws  map[cube.RFAddress][]byte
	order []cube.RFAddress
}

func loadFixtureLines(t *testing.T) *fixtureLines {
	t.Helper()
	lines, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixtureLines{t: t, lines: lines, raws: map[cube.RFAddress][]byte{}}
	for _, g := range lines {
		switch g.Type {
		case 'M':
			p, err := cube.ParseMetadataPart(g.Data)
			if err != nil {
				t.Fatal(err)
			}
			if f.md, err = cube.ParseMetadata([]cube.MetadataPart{p}); err != nil {
				t.Fatal(err)
			}
		case 'C':
			rf, err := cube.ParseRFAddress(g.Data[:6])
			if err != nil {
				t.Fatal(err)
			}
			f.raws[rf] = rawConfig(t, lines, rf)
			f.order = append(f.order, rf)
		}
	}
	return f
}

// setRoom puts a device's C line into room.
func (f *fixtureLines) setRoom(rf cube.RFAddress, room uint8) { f.raws[rf][5] = room }

// addConfig adds a C line for rf, a copy of from's, in room.
func (f *fixtureLines) addConfig(rf, from cube.RFAddress, room uint8, serial string) {
	raw := append([]byte(nil), f.raws[from]...)
	b := rf.Bytes()
	copy(raw[1:4], b[:])
	raw[5] = room
	copy(raw[8:18], serial)
	f.raws[rf] = raw
	f.order = append(f.order, rf)
}

// dropConfig takes a device's C line out of the greeting; its L entry
// stays.
func (f *fixtureLines) dropConfig(rf cube.RFAddress) {
	f.order = slices.DeleteFunc(f.order, func(x cube.RFAddress) bool { return x == rf })
}

// list adds a device to the metadata.
func (f *fixtureLines) list(rf cube.RFAddress, typ cube.DeviceType, room uint8, serial, name string) {
	f.md.Devices = append(f.md.Devices, cube.DeviceMeta{Type: typ, RFAddress: rf, Serial: serial, Name: name, RoomID: room})
}

// listed is a device's metadata entry.
func (f *fixtureLines) listed(rf cube.RFAddress) *cube.DeviceMeta {
	for i := range f.md.Devices {
		if f.md.Devices[i].RFAddress == rf {
			return &f.md.Devices[i]
		}
	}
	f.t.Fatalf("%s not listed", rf)
	return nil
}

// master names a room's master in the metadata.
func (f *fixtureLines) master(room uint8, rf cube.RFAddress) {
	for i := range f.md.Rooms {
		if f.md.Rooms[i].ID == room {
			f.md.Rooms[i].RFAddress = rf
			return
		}
	}
	f.t.Fatalf("no room %d", room)
}

// greeting is the greeting with the changes.
func (f *fixtureLines) greeting() []cube.Message {
	var out []cube.Message
	cs := false
	for _, g := range f.lines {
		switch g.Type {
		case 'M':
			enc, err := f.md.Encode()
			if err != nil {
				f.t.Fatal(err)
			}
			out = append(out, cube.Message{Type: 'M', Data: "00,01," + base64.StdEncoding.EncodeToString(enc)})
		case 'C':
			if !cs {
				cs = true
				for _, rf := range f.order {
					out = append(out, cube.Message{Type: 'C', Data: rf.String() + "," + base64.StdEncoding.EncodeToString(f.raws[rf])})
				}
			}
		default:
			out = append(out, g)
		}
	}
	return out
}

// The fixture's rooms and devices: room 1 Wohnzimmer (0a010a), 2 Büro
// (0a0109), 3 Küche (0a0108), 4 Kind 2 (0a010b), 5 Kind 1 (0a010c),
// 6 Schlafzimmer (0a010d), each its radiator thermostat its master.

// mismatchCube is a server with a store and the owner's token over a
// fake Cube, its checks on a clock the test moves.
type mismatchCube struct {
	fc    *fakecube.Server
	ts    *httptest.Server
	s     *Server
	st    *store.Store
	clock *testClock
}

const mismatchToken = "secret"

func startMismatchCube(t *testing.T, greeting []cube.Message, cfg func(*CubeConfig), seed func(st *store.Store, cubeID int64)) *mismatchCube {
	t.Helper()
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	st := testStore(t, "")
	row, err := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv})
	if err != nil {
		t.Fatal(err)
	}
	if seed != nil {
		seed(st, row.ID)
	}
	cc, err := CubeFromRow(row)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		cfg(&cc)
	}
	clock := &testClock{t: time.Date(2026, 11, 15, 8, 0, 0, 0, time.UTC)}
	s := New(Options{Cubes: []CubeConfig{cc}, Store: st, Token: mismatchToken, Queue: engine.Options{Gap: time.Millisecond}, checkClock: clock.now})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	c := &mismatchCube{fc: fc, ts: ts, s: s, st: st, clock: clock}
	c.waitChecks(t, 1) // the first greeting's
	return c
}

func (c *mismatchCube) rt() *runtime { return c.s.runtimes()[0] }

// checks is how many checks the server ran.
func (c *mismatchCube) checks() int {
	cg := &c.rt().cong
	cg.mu.Lock()
	defer cg.mu.Unlock()
	return cg.checks
}

func (c *mismatchCube) waitChecks(t *testing.T, n int) {
	t.Helper()
	testkit.Eventually(t, fmt.Sprintf("check %d", n), func() bool { return c.checks() >= n })
}

// greet makes the server read greeting and waits for its check.
func (c *mismatchCube) greet(t *testing.T, greeting []cube.Message) {
	t.Helper()
	n := c.checks()
	c.fc.SetGreeting(greeting)
	if res := c.rt().queue.Enqueue(engine.Command{Key: "refresh", Reconnect: true}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
	c.waitChecks(t, n+1)
}

// events are the Cube's mismatch events, oldest first.
func (c *mismatchCube) events(t *testing.T) []event {
	t.Helper()
	var evs []event
	getJSON(t, c.ts, mismatchToken, "/api/v1/cubes/KEQ0000100/log?level=debug&limit=500&kind=mismatch", &evs)
	return evs
}

// event is the newest mismatch event of finding and device in state;
// nil for none.
func (c *mismatchCube) event(t *testing.T, finding, device, state string) *event {
	t.Helper()
	evs := c.events(t)
	for i := len(evs) - 1; i >= 0; i-- {
		if s := evs[i].Summary; s["finding"] == finding && evs[i].Device == device && s["state"] == state {
			return &evs[i]
		}
	}
	return nil
}

// device is a device's view in GET …/devices/{rf}, and the same in GET
// …/devices, where a device the metadata alone lists is.
func (c *mismatchCube) device(t *testing.T, rf string) view.Device {
	t.Helper()
	var devs []view.Device
	getJSON(t, c.ts, mismatchToken, "/api/v1/cubes/KEQ0000100/devices?devices=real", &devs)
	i := slices.IndexFunc(devs, func(d view.Device) bool { return d.RFAddress == rf })
	if i < 0 {
		t.Fatalf("no device %s in %+v", rf, devs)
	}
	var d view.Device
	if code := getJSON(t, c.ts, mismatchToken, "/api/v1/cubes/KEQ0000100/devices/"+rf, &d); code == 200 && !slices.Equal(keys(d.Mismatches), keys(devs[i].Mismatches)) {
		t.Fatalf("device %s: %+v, in the list %+v", rf, d.Mismatches, devs[i].Mismatches)
	}
	return devs[i]
}

// keys are the keys of a list of mismatches.
func keys(ms []view.Mismatch) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Key())
	}
	return out
}

// room is a room's view in GET …/rooms.
func (c *mismatchCube) room(t *testing.T, id uint8) view.Room {
	t.Helper()
	var rooms []view.Room
	getJSON(t, c.ts, mismatchToken, "/api/v1/cubes/KEQ0000100/rooms", &rooms)
	for _, r := range rooms {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no room %d in %+v", id, rooms)
	return view.Room{}
}

// has reports whether a list holds a finding of a device.
func has(ms []view.Mismatch, finding, device string) bool {
	return slices.ContainsFunc(ms, func(m view.Mismatch) bool { return m.Finding == finding && m.Device == device })
}

// alerted reports whether GET /api/v1/log/alerts has a mismatch alert of
// the device.
func (c *mismatchCube) alerted(t *testing.T, device string) bool {
	t.Helper()
	var v logAlertsView
	getJSON(t, c.ts, mismatchToken, "/api/v1/log/alerts", &v)
	return slices.ContainsFunc(v.Alerts, func(a logAlert) bool { return a.Kind == "mismatch" && a.Device == device })
}

// A device whose C line names room 2 and the metadata room 1: one
// warning naming both rooms, the device marked and both rooms; the same
// greeting again warns no more; once the C line names room 1 again, the
// finding clears at info, the marks go and the alert is settled.
func TestMismatchRoomAndClear(t *testing.T) {
	f := loadFixtureLines(t)
	f.setRoom(0x0a010a, 2)
	c := startMismatchCube(t, f.greeting(), nil, nil)
	ev := c.event(t, view.MismatchRoom, "0a010a", "found")
	if ev == nil || ev.Level != "warn" || ev.Source != "cube" || ev.Serial != "MEQ0000101" {
		t.Fatalf("warning %+v of %+v", ev, c.events(t))
	}
	s := ev.Summary
	if s["room"] != float64(1) || s["cube_room"] != float64(2) || s["list_room"] != float64(1) || s["cube_room_name"] != "Büro" ||
		s["list_room_name"] != "Wohnzimmer" || s["name"] != "Radiator Wohnzimmer" {
		t.Fatalf("summary %v", s)
	}
	if d := c.device(t, "0a010a"); !has(d.Mismatches, view.MismatchRoom, "0a010a") {
		t.Fatalf("device's mismatches %+v", d.Mismatches)
	}
	for _, id := range []uint8{1, 2} {
		if r := c.room(t, id); !has(r.Mismatches, view.MismatchRoom, "0a010a") {
			t.Fatalf("room %d's mismatches %+v", id, r.Mismatches)
		}
	}
	if !c.alerted(t, "0a010a") {
		t.Fatal("no alert")
	}
	c.greet(t, f.greeting())
	count := 0
	for _, e := range c.events(t) {
		if e.Summary["finding"] == view.MismatchRoom {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("%d room events after the same greeting again", count)
	}

	f.setRoom(0x0a010a, 1)
	c.greet(t, f.greeting())
	cleared := c.event(t, view.MismatchRoom, "0a010a", "cleared")
	if cleared == nil || cleared.Level != "info" || cleared.Summary["cube_room"] != float64(2) {
		t.Fatalf("cleared %+v", cleared)
	}
	if d := c.device(t, "0a010a"); len(d.Mismatches) != 0 {
		t.Fatalf("device's mismatches %+v after the clear", d.Mismatches)
	}
	if r := c.room(t, 2); len(r.Mismatches) != 0 {
		t.Fatalf("room 2's mismatches %+v after the clear", r.Mismatches)
	}
	if c.alerted(t, "0a010a") {
		t.Fatal("the alert is not settled")
	}
}

// The C line cases: a C line in room 4 for a device the metadata does not
// list; a device the metadata lists in room 3 without a C line, and one
// in room 6 without a C line though it has an L entry; a type that
// differs; each warned of with its rooms, and marked.
func TestMismatchLineCases(t *testing.T) {
	f := loadFixtureLines(t)
	f.addConfig(0x0a0a01, 0x0a010b, 4, "MEQ0000201")
	f.list(0x0a0a02, cube.DeviceRadiatorThermostat, 3, "MEQ0000202", "Listed only")
	f.listed(0x0a010c).Type = cube.DeviceWallThermostat
	f.dropConfig(0x0a010d) // listed, in the device list, without a C line (D194-A)
	c := startMismatchCube(t, f.greeting(), nil, nil)

	if nc := c.event(t, view.MismatchNoConfig, "0a010d", "found"); nc == nil || nc.Summary["list_room"] != float64(6) {
		t.Fatalf("no C line, an L entry %+v", nc)
	}
	un := c.event(t, view.MismatchUnlisted, "0a0a01", "found")
	if un == nil || un.Summary["room"] != float64(4) || un.Summary["cube_room"] != float64(4) || un.Summary["cube_room_name"] != "Kind 2" {
		t.Fatalf("unlisted %+v", un)
	}
	if r := c.room(t, 4); !has(r.Mismatches, view.MismatchUnlisted, "0a0a01") {
		t.Fatalf("room 4's mismatches %+v", r.Mismatches)
	}
	nc := c.event(t, view.MismatchNoConfig, "0a0a02", "found")
	if nc == nil || nc.Summary["list_room"] != float64(3) || nc.Summary["list_room_name"] != "Küche" || nc.Summary["name"] != "Listed only" {
		t.Fatalf("no C line %+v", nc)
	}
	if d := c.device(t, "0a0a02"); !has(d.Mismatches, view.MismatchNoConfig, "0a0a02") {
		t.Fatalf("device's mismatches %+v", d.Mismatches)
	}
	ty := c.event(t, view.MismatchType, "0a010c", "found")
	if ty == nil || ty.Summary["cube_type"] != "radiator_thermostat" || ty.Summary["list_type"] != "wall_thermostat" || ty.Summary["room"] != float64(5) {
		t.Fatalf("type %+v", ty)
	}
	if r := c.room(t, 5); !has(r.Mismatches, view.MismatchType, "0a010c") {
		t.Fatalf("room 5's mismatches %+v", r.Mismatches)
	}
	for _, e := range c.events(t) {
		if e.Summary["finding"] == view.MismatchProgramme || e.Summary["finding"] == view.MismatchTemperatures {
			t.Errorf("a member finding %v: room 4's two members agree", e.Summary)
		}
	}
}

// A room master in the metadata that is a device of another room, no
// device of the Cube, or no thermostat; a room without a thermostat is
// not looked at.
func TestMismatchRoomMaster(t *testing.T) {
	f := loadFixtureLines(t)
	contact := append([]byte{0x11, 0, 0, 0, byte(cube.DeviceShutterContact), 0, 0x10, 0xa0}, make([]byte, 10)...)
	f.raws[0x0a0a03] = contact
	f.addConfig(0x0a0a03, 0x0a0a03, 6, "MEQ0000203")
	f.list(0x0a0a03, cube.DeviceShutterContact, 6, "MEQ0000203", "Window")
	f.master(3, 0x0a0109) // room 2's radiator thermostat
	f.master(5, 0x0a0a04) // no device
	f.master(6, 0x0a0a03) // the window contact
	f.md.Rooms = append(f.md.Rooms, cube.RoomMeta{ID: 7, Name: "Empty", RFAddress: 0x0a0a05})
	c := startMismatchCube(t, f.greeting(), nil, nil)
	for _, want := range []struct {
		device, why string
		room        float64
	}{{"0a0109", "other_room", 3}, {"0a0a04", "no_device", 5}, {"0a0a03", "not_thermostat", 6}} {
		ev := c.event(t, view.MismatchMaster, want.device, "found")
		if ev == nil || ev.Level != "warn" || ev.Summary["why"] != want.why || ev.Summary["room"] != want.room {
			t.Fatalf("master %s: %+v", want.device, ev)
		}
	}
	if ev := c.event(t, view.MismatchMaster, "0a0109", "found"); ev.Summary["cube_room"] != float64(2) || ev.Summary["cube_room_name"] != "Büro" {
		t.Fatalf("the other room %v", ev.Summary)
	}
	if r := c.room(t, 3); !has(r.Mismatches, view.MismatchMaster, "0a0109") {
		t.Fatalf("room 3's mismatches %+v", r.Mismatches)
	}
	if c.event(t, view.MismatchMaster, "0a0a05", "found") != nil {
		t.Fatal("a room without a thermostat warned of")
	}
}

// A room master's warning is given again while it holds once someone
// marked it seen, and not while it is unread.
func TestMismatchMasterWarnedAgainOnceRead(t *testing.T) {
	f := loadFixtureLines(t)
	f.master(3, 0x0a0109)
	c := startMismatchCube(t, f.greeting(), nil, nil)
	first := c.event(t, view.MismatchMaster, "0a0109", "found")
	if first == nil {
		t.Fatal("no master warning")
	}
	warnings := func() int {
		n := 0
		for _, e := range c.events(t) {
			if e.Summary["finding"] == view.MismatchMaster && e.Summary["state"] == "found" {
				n++
			}
		}
		return n
	}
	c.clock.add(2 * time.Minute)
	c.greet(t, f.greeting())
	if n := warnings(); n != 1 {
		t.Fatalf("%d warnings while the first is unread", n)
	}
	ctx := context.Background()
	anna, err := c.st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "pw", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.st.SetPreference(ctx, anna.ID, seenPrefKey, json.RawMessage(fmt.Sprintf(`{"all": 0, "lines": [%d]}`, first.ID))); err != nil {
		t.Fatal(err)
	}
	c.clock.add(2 * time.Minute)
	c.greet(t, f.greeting())
	if n := warnings(); n != 2 {
		t.Fatalf("%d warnings once the first was read", n)
	}
	c.clock.add(2 * time.Minute)
	c.greet(t, f.greeting())
	if n := warnings(); n != 2 {
		t.Fatalf("%d warnings while the second is unread", n)
	}
}

// Within a room the members whose programme or temperatures differ from
// those most of them share; of two that differ, the one the room's master
// is not.
func TestMismatchMembers(t *testing.T) {
	f := loadFixtureLines(t)
	f.addConfig(0x0a0a01, 0x0a010a, 1, "MEQ0000201")
	f.addConfig(0x0a0a02, 0x0a010a, 1, "MEQ0000202")
	f.list(0x0a0a01, cube.DeviceRadiatorThermostat, 1, "MEQ0000201", "Second")
	f.list(0x0a0a02, cube.DeviceRadiatorThermostat, 1, "MEQ0000202", "Third")
	f.raws[0x0a0a01][0x1d] ^= 0x02 // the first switch point half a degree off
	f.raws[0x0a0a02][0x12] = 40    // comfort 20
	f.addConfig(0x0a0a04, 0x0a010b, 4, "MEQ0000204")
	f.list(0x0a0a04, cube.DeviceRadiatorThermostat, 4, "MEQ0000204", "Other")
	f.raws[0x0a0a04][0x13] = 34 // eco 17
	c := startMismatchCube(t, f.greeting(), nil, nil)
	for _, want := range []struct{ finding, device string }{
		{view.MismatchProgramme, "0a0a01"}, {view.MismatchTemperatures, "0a0a02"}, {view.MismatchTemperatures, "0a0a04"},
	} {
		if ev := c.event(t, want.finding, want.device, "found"); ev == nil || ev.Level != "warn" {
			t.Fatalf("%s of %s: %+v", want.finding, want.device, c.events(t))
		}
		if d := c.device(t, want.device); !has(d.Mismatches, want.finding, want.device) {
			t.Fatalf("%s's mismatches %+v", want.device, d.Mismatches)
		}
	}
	if n := len(c.events(t)); n != 3 {
		t.Fatalf("%d events, want 3: %+v", n, c.events(t))
	}
	if r := c.room(t, 1); !has(r.Mismatches, view.MismatchProgramme, "0a0a01") || !has(r.Mismatches, view.MismatchTemperatures, "0a0a02") {
		t.Fatalf("room 1's mismatches %+v", r.Mismatches)
	}
}

// Left out: a hidden device and one parked in room 255; the device of
// Massimo's own structure change and its rooms, until its C line follows
// or the time is up.
func TestMismatchLeftOut(t *testing.T) {
	f := loadFixtureLines(t)
	f.setRoom(0x0a010c, 2) // hidden by the list
	f.listed(0x0a010b).RoomID = cube.HiddenRoom
	c := startMismatchCube(t, f.greeting(), func(cc *CubeConfig) { cc.Hidden = []cube.RFAddress{0x0a010c} }, nil)
	if evs := c.events(t); len(evs) != 0 {
		t.Fatalf("events %+v for hidden and parked devices", evs)
	}

	// a move of 0a010d from room 6 to room 1: the metadata first
	rt := c.rt()
	rt.beginStructure("move", 0x0a010d, 6, 1)
	rt.settledMismatches(0x0a010d, c.clock.now())
	f.listed(0x0a010d).RoomID = 1
	c.greet(t, f.greeting())
	if evs := c.events(t); len(evs) != 0 {
		t.Fatalf("events %+v while the moved device's C line has not followed", evs)
	}
	c.clock.add(mismatchSettle + time.Second)
	c.greet(t, f.greeting())
	if ev := c.event(t, view.MismatchRoom, "0a010d", "found"); ev == nil {
		t.Fatalf("no warning after the time is up: %+v", c.events(t))
	}

	// another move, whose C line follows: then the device is checked again
	f.listed(0x0a010d).RoomID = 6
	c.greet(t, f.greeting())
	if c.event(t, view.MismatchRoom, "0a010d", "cleared") == nil {
		t.Fatalf("not cleared: %+v", c.events(t))
	}
	rt.beginStructure("move", 0x0a010d, 6, 1)
	rt.settledMismatches(0x0a010d, c.clock.now())
	f.listed(0x0a010d).RoomID = 1
	f.setRoom(0x0a010d, 1)
	c.greet(t, f.greeting())
	f.setRoom(0x0a010d, 6) // the C line back in room 6, the metadata in room 1
	c.greet(t, f.greeting())
	warned := 0
	for _, e := range c.events(t) {
		if e.Summary["finding"] == view.MismatchRoom && e.Summary["state"] == "found" {
			warned++
		}
	}
	if warned != 2 {
		t.Fatalf("%d warnings, want the second once the C line had followed: %+v", warned, c.events(t))
	}
}

// At the start the server takes the findings its log holds unsettled: one
// that still holds is not warned of again, one that no longer does is
// cleared.
func TestMismatchSeededFromTheLog(t *testing.T) {
	f := loadFixtureLines(t)
	f.setRoom(0x0a010a, 2)
	two, one := uint8(2), uint8(1)
	holds := view.Mismatch{Finding: view.MismatchRoom, Device: "0a010a", Room: 1, CubeRoom: &two, ListRoom: &one}
	gone := view.Mismatch{Finding: view.MismatchType, Device: "0a0109", Room: 2, CubeType: "radiator_thermostat", ListType: "wall_thermostat"}
	c := startMismatchCube(t, f.greeting(), nil, func(st *store.Store, cubeID int64) {
		for _, m := range []view.Mismatch{holds, gone} {
			st.AddEvent(storedEvent(cubeID, mismatchEvent(nil, m, "found")))
		}
	})
	testkit.Eventually(t, "the cleared finding", func() bool { return c.event(t, view.MismatchType, "0a0109", "cleared") != nil })
	n := 0
	for _, e := range c.events(t) {
		if e.Summary["finding"] == view.MismatchRoom {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d room events, want the stored one alone: %+v", n, c.events(t))
	}
	if d := c.device(t, "0a010a"); !has(d.Mismatches, view.MismatchRoom, "0a010a") {
		t.Fatalf("device's mismatches %+v", d.Mismatches)
	}
}

// The log's line of a finding names it.
func TestMismatchEventLine(t *testing.T) {
	m := view.Mismatch{Finding: view.MismatchMaster, Device: "0a0109", Room: 3, Why: "other_room"}
	if got := eventMessage(mismatchEvent(nil, m, "found")); !strings.Contains(got, "master (other_room)") || !strings.Contains(got, "differ") {
		t.Fatalf("line %q", got)
	}
	if got := eventMessage(mismatchEvent(nil, m, "cleared")); !strings.Contains(got, "agree again") {
		t.Fatalf("line %q", got)
	}
}
