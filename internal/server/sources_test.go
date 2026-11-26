// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/roomorder"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
	"go.metaworx.ch/massimo/internal/view"
)

// The places where one thermostat stands for its room take the one
// order (AP RoomMajority v1.2, Block 4, D19-A): the room's configuration
// and programme, the room entry's thermostat and presets, and the
// virtual wall thermostat's mode, setpoint and until. The rooms fixture
// (startDrift): room 1 the wall thermostat 0a0201, the room's master, at
// comfort 21 and eco 17 with a programme of its own, and the radiator
// thermostats 0a0202 and 0a0203 at comfort 22 and eco 15; room 2 the
// radiator thermostats 0a0204, its master, to 0a0206, as 0a0202.

// The radiator thermostat's C line: the comfort temperature's byte, the
// highest and the lowest setpoint's, the week programme's start, the
// bytes of a day.
const (
	rawComfort = 0x12
	rawMax     = 0x14
	rawMin     = 0x15
	rawProgram = 0x1d
	rawDay     = 2 * cube.DaySlots
)

// withConfig changes rf's C line among lines.
func withConfig(t *testing.T, lines []cube.Message, rf cube.RFAddress, change func(raw []byte)) {
	t.Helper()
	raw := rawConfig(t, lines, rf)
	change(raw)
	for i, g := range lines {
		if g.Type == 'C' && strings.HasPrefix(g.Data, rf.String()+",") {
			lines[i] = configLine(raw, rf, raw[5], string(raw[8:18]))
		}
	}
}

// otherDay changes the first switch point's temperature of a day of a
// radiator thermostat's programme by half a degree.
func otherDay(d cube.Weekday) func(raw []byte) {
	return func(raw []byte) { raw[rawProgram+rawDay*int(d)] ^= 0x02 }
}

// kill makes the Cube hold no valid state for rf, as for a device out of
// date, and waits until the server's snapshot shows it.
func (r *rig) kill(rf cube.RFAddress) {
	r.t.Helper()
	off := false
	if err := r.fc.SetState(rf, fakecube.State{Reported: &off}); err != nil {
		r.t.Fatal(err)
	}
	testkit.Eventually(r.t, rf.String()+" dead", func() bool {
		st, ok := r.rt.client.Snapshot().State(rf)
		return ok && !st.Valid
	})
}

// roomView is GET …/rooms/{id}/config's answer.
func roomView(t *testing.T, r *driftRig, room int) roomConfigView {
	t.Helper()
	var v roomConfigView
	if code := getJSON(t, r.ts, "", "/api/v1/cubes/KEQ0000100/rooms/"+strconv.Itoa(room)+"/config", &v); code != 200 {
		t.Fatalf("room %d's configuration: %d", room, code)
	}
	return v
}

// dayOf is a programme's day as GET …/program gives it, as JSON.
func dayOf(t *testing.T, prog map[string]any, d cube.Weekday) string {
	t.Helper()
	raw, err := json.Marshal(prog[strings.ToLower(d.String())])
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// weekJSON is a device's own programme day as the API writes it.
func weekJSON(t *testing.T, r *driftRig, rf cube.RFAddress, d cube.Weekday) string {
	t.Helper()
	c := r.rt.client.Snapshot().Config(rf)
	var w cube.WeekProgram
	switch {
	case c.Thermostat != nil:
		w = c.Thermostat.WeekProgram
	case c.Wall != nil:
		w = c.Wall.WeekProgram
	}
	raw, err := json.Marshal(w[d])
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// The room's configuration and programme by the one order, each group
// and each day on its own: what more than half of its thermostats
// share, by most where the master differs (room 2, its master 0a0206 at
// comfort 22, 0a0204 and 0a0205 at 23, a Monday of their own; each of
// them alone with another Tuesday, Wednesday and Thursday: comfort 23,
// and a week that is none of theirs); on a tie, the master's, a radiator
// thermostat in room 2 (0a0205 dead: 0a0204 against 0a0206), the wall
// thermostat in room 1 (0a0203 dead: 0a0201 against 0a0202), a dead
// member passed over. source names the first of the order that has the
// room's temperatures, or its week, else the first of the order.
func TestRoomConfigByTheOneOrder(t *testing.T) {
	t.Parallel()
	r := startDrift(t, func(lines []cube.Message) []cube.Message {
		comfort23 := func(raw []byte) { raw[rawComfort] = 46 }
		withConfig(t, lines, 0x0a0204, comfort23)
		withConfig(t, lines, 0x0a0205, comfort23)
		for rf, days := range map[cube.RFAddress][]cube.Weekday{0x0a0204: {cube.Monday, cube.Tuesday}, 0x0a0205: {cube.Monday, cube.Wednesday},
			0x0a0206: {cube.Thursday}} {
			for _, d := range days {
				withConfig(t, lines, rf, otherDay(d))
			}
		}
		withRoomMaster(t, lines, 2, 0x0a0206)
		return lines
	})
	base := "/api/v1/cubes/KEQ0000100/rooms/"
	check := func(name string, room int, source string, comfort, eco float64, progSource string, days map[cube.Weekday]cube.RFAddress) {
		t.Helper()
		v := roomView(t, r, room)
		if v.Source != source || v.ComfortTemp != comfort || v.EcoTemp != eco {
			t.Errorf("%s: the configuration from %s at comfort %v eco %v, want %s %v %v", name, v.Source, v.ComfortTemp, v.EcoTemp, source, comfort, eco)
		}
		prog, _ := programWith(t, r.ts.URL+base+strconv.Itoa(room)+"/program")
		if prog["source"] != progSource {
			t.Errorf("%s: the programme from %v, want %s", name, prog["source"], progSource)
		}
		for d, rf := range days {
			if got, want := dayOf(t, prog, d), weekJSON(t, r, rf, d); got != want {
				t.Errorf("%s: %s is %s, want %s's %s", name, d, got, rf, want)
			}
		}
	}
	if weekJSON(t, r, 0x0a0204, cube.Monday) == weekJSON(t, r, 0x0a0206, cube.Monday) ||
		weekJSON(t, r, 0x0a0201, cube.Monday) == weekJSON(t, r, 0x0a0202, cube.Monday) {
		t.Fatal("the fixture's Mondays do not differ")
	}
	check("room 2 by most", 2, "0a0204", 23, 15, "0a0206", map[cube.Weekday]cube.RFAddress{cube.Monday: 0x0a0204, cube.Tuesday: 0x0a0205,
		cube.Wednesday: 0x0a0204, cube.Thursday: 0x0a0204, cube.Friday: 0x0a0206})
	check("room 1 by most", 1, "0a0202", 22, 15, "0a0202", map[cube.Weekday]cube.RFAddress{cube.Monday: 0x0a0202, cube.Saturday: 0x0a0203})

	r.kill(0x0a0205)
	check("room 2 on a tie, a dead member passed over", 2, "0a0206", 22, 15, "0a0206", map[cube.Weekday]cube.RFAddress{cube.Monday: 0x0a0206,
		cube.Tuesday: 0x0a0206, cube.Thursday: 0x0a0206})
	r.kill(0x0a0203)
	check("room 1 on a tie", 1, "0a0201", 21, 17, "0a0201", map[cube.Weekday]cube.RFAddress{cube.Monday: 0x0a0201, cube.Sunday: 0x0a0201})
}

// Where every thermostat of a room is dead, as after the Cube's restart
// until one reports, the room's configuration, programme and presets are
// taken by the same order over them all, as its addressee is, none
// refused: room 1, whose three thermostats share no temperatures and no
// Monday, the master's, its wall thermostat's.
func TestRoomConfigWithEveryThermostatDead(t *testing.T) {
	t.Parallel()
	r := startDrift(t, func(lines []cube.Message) []cube.Message {
		withConfig(t, lines, 0x0a0203, func(raw []byte) { raw[rawComfort] = 46 }) // comfort 23: no two radiators agree
		withConfig(t, lines, 0x0a0203, otherDay(cube.Monday))
		return lines
	})
	for _, rf := range []cube.RFAddress{0x0a0201, 0x0a0202, 0x0a0203} {
		r.kill(rf)
	}
	v := roomView(t, r, 1)
	if v.Source != "0a0201" || v.ComfortTemp != 21 || v.EcoTemp != 17 || len(v.Devices) != 3 {
		t.Fatalf("room 1's configuration, every one dead: %+v", v)
	}
	prog, _ := programWith(t, r.ts.URL+"/api/v1/cubes/KEQ0000100/rooms/1/program")
	if got, want := dayOf(t, prog, cube.Monday), weekJSON(t, r, 0x0a0201, cube.Monday); prog["source"] != "0a0201" || got != want {
		t.Fatalf("room 1's programme, every one dead: from %v, Monday %s, want the master's %s", prog["source"], got, want)
	}
	var rooms []view.Room
	getJSON(t, r.ts, "", "/api/v1/cubes/KEQ0000100/rooms", &rooms)
	if rooms[0].Thermostat != "0a0201" || rooms[0].ComfortTemp == nil || *rooms[0].ComfortTemp != 21 || *rooms[0].EcoTemp != 17 {
		t.Fatalf("room 1's entry, every one dead: %s %v %v", rooms[0].Thermostat, rooms[0].ComfortTemp, rooms[0].EcoTemp)
	}
}

// The programme's tag covers the members' programmes alone: a member's
// death that changes the room's programme, which no one changed, leaves
// it, and a write with it goes; that write changes it, so that a write
// with the tag from before is 412, carrying the new one, sending nothing;
// while the write is on its way (the fake Cube holds its apply), the
// room's programme shows the day written, pending, under the write's tag.
func TestRoomProgramTagOfTheMembers(t *testing.T) {
	t.Parallel()
	r := startServer(t, storedRooms())
	url := r.ts.URL + "/api/v1/cubes/KEQ0000100/rooms/1/program"
	prog, before := programWith(t, url)
	if prog["source"] != "0a0202" {
		t.Fatalf("room 1's programme from %v", prog["source"])
	}
	wall, _ := json.Marshal(roomorder.Program(r.rt.client.Snapshot().Config(0x0a0201))[cube.Monday])
	if dayOf(t, prog, cube.Monday) == string(wall) {
		t.Fatal("the fixture's wall thermostat has the radiator thermostats' Monday")
	}
	r.kill(0x0a0203) // a tie: the master's programme, the wall thermostat's
	prog, after := programWith(t, url)
	if prog["source"] != "0a0201" || dayOf(t, prog, cube.Monday) != string(wall) || after != before {
		t.Fatalf("after the death: from %v, Monday %s, tag %s, before %s", prog["source"], dayOf(t, prog, cube.Monday), after, before)
	}
	if _, member := programWith(t, r.ts.URL+"/api/v1/cubes/KEQ0000100/devices/0a0202/program"); member != after {
		t.Fatalf("a member's tag %s, the room's %s", member, after)
	}
	const monday = `[{"temp":17,"until":"06:00"},{"temp":21.5,"until":"22:00"},{"temp":17,"until":"24:00"}]`
	r.fc.HoldApply() // the room's thermostats asleep to the test's end: the write stays on its way
	code, body := putProgram(t, url, `"`+before+`"`, `{"monday":`+monday+`}`)
	if code != 200 || body["etag"] == before {
		t.Fatalf("the tag no one's change touched: %d %v", code, body)
	}
	written := body["etag"]
	received := cubewatch.Lines(r.fc)
	sent := received.Mark()
	if code, body := putProgram(t, url, `"`+before+`"`, `{"monday":`+monday+`}`); code != 412 || body["etag"] != written {
		t.Fatalf("the tag from before the write: %d %v", code, body)
	}
	if got := received.Since(sent, cubewatch.Type('s')); len(got) != 0 {
		t.Fatalf("a refused write sent %v", got)
	}
	prog, now := programWith(t, url)
	if now != written || prog["pending"] != true || dayOf(t, prog, cube.Monday) != monday || prog["source"] != "0a0201" {
		t.Fatalf("on its way: %s pending %v Monday %s from %v, the write's tag %v", now, prog["pending"], dayOf(t, prog, cube.Monday), prog["source"], written)
	}
}

// The room entry names the thermostat whose state is the room's, as the
// virtual wall thermostat takes it (roomorder.State): in room 2, its
// master 0a0204 at manual 18 and 0a0205 and 0a0206 at manual 21, the
// state most show, 0a0205's, which the tile shows (roomThermostat),
// though radiograms keep the master as the room's addressee.
func TestRoomEntryNamesTheRoomsState(t *testing.T) {
	t.Parallel()
	r := startDrift(t, nil)
	r.set(0x0a0204, "manual", 18)
	r.set(0x0a0205, "manual", 21)
	r.set(0x0a0206, "manual", 21)
	named, setpoint, vwt, from := roomEntryState(t, r, 2)
	if named != "0a0205" || setpoint != 21 || vwt != 21 || from != "0a0205,0a0206" {
		t.Fatalf("room 2: names %s at %v; its virtual wall thermostat at %v from %s", named, setpoint, vwt, from)
	}
	if a, _ := cubeio.RoomAddressee(r.rt.client.Snapshot(), 2, nil); a != 0x0a0204 {
		t.Fatalf("the room's addressee %s", a)
	}
}

// Where no state has more than half and no change has a time, the order
// decides, its master first though the Cube lists it last: room 2, its
// master 0a0206 (the metadata's), 0a0204 at manual 18, 0a0205 at 21 and
// 0a0206 at 19, the values there at the first connection; the room entry
// names 0a0206, the virtual wall thermostat shows 19 from it.
func TestRoomEntryNamesTheMasterByTheOrder(t *testing.T) {
	t.Parallel()
	manual := "manual"
	at := func(v float64) fakecube.State { return fakecube.State{Mode: &manual, Setpoint: &v} }
	r := startDriftWith(t, func(lines []cube.Message) []cube.Message {
		withRoomMaster(t, lines, 2, 0x0a0206)
		return lines
	}, map[cube.RFAddress]fakecube.State{0x0a0204: at(18), 0x0a0205: at(21), 0x0a0206: at(19)})
	named, setpoint, vwt, from := roomEntryState(t, r, 2)
	if named != "0a0206" || setpoint != 19 || vwt != 19 || from != "0a0206" {
		t.Fatalf("room 2: names %s at %v; its virtual wall thermostat at %v from %s", named, setpoint, vwt, from)
	}
}

// roomEntryState is a room's entry as GET …/rooms?devices=all gives it:
// the thermostat it names and that one's setpoint, which the tile shows,
// and its virtual wall thermostat's setpoint and sources.mode.
func roomEntryState(t *testing.T, r *driftRig, room uint8) (named string, setpoint, vwt float64, from string) {
	t.Helper()
	var rooms []view.Room
	if code := getJSON(t, r.ts, "", "/api/v1/cubes/KEQ0000100/rooms?devices=all", &rooms); code != 200 {
		t.Fatalf("rooms: %d", code)
	}
	for _, x := range rooms {
		if x.ID != room {
			continue
		}
		named = x.Thermostat
		for _, d := range x.Devices {
			switch {
			case d.State == nil || d.State.Setpoint == nil:
			case d.Virtual:
				vwt, from = *d.State.Setpoint, strings.Join(d.Sources["mode"], ",")
			case d.RFAddress == named:
				setpoint = *d.State.Setpoint
			}
		}
		return
	}
	t.Fatalf("no room %d", room)
	return
}

// While a thermostat of a room has not reported since the Cube's
// restart, the room's configuration is decided over them all, as the
// report waits for them all (RoomMajority.D21-A), so that it stays put
// while they report one by one: room 2, its master 0a0204 at comfort 23
// and the two others at 22, after a restart (every entry neither valid
// nor initialized), shows comfort 22 in its configuration, its entry,
// its virtual wall thermostat's configuration and the proxy's virtual C
// line, while 0a0204 reports alone, then 0a0205, and once all have.
func TestRoomConfigWhileTheThermostatsReportAgain(t *testing.T) {
	t.Parallel()
	r := startDrift(t, func(lines []cube.Message) []cube.Message {
		withConfig(t, lines, 0x0a0204, func(raw []byte) { raw[rawComfort] = 46 }) // comfort 23, the master's alone
		return restarted(t, lines)
	})
	if code, body := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_virtual":true}`); code != 200 || body["proxy_virtual"] != true {
		t.Fatalf("proxy_virtual: %d %v", code, body)
	}
	comforts := func() string {
		t.Helper()
		v := roomView(t, r, 2)
		var rooms []view.Room
		getJSON(t, r.ts, "", "/api/v1/cubes/KEQ0000100/rooms", &rooms)
		var entry any = "none"
		for _, x := range rooms {
			if x.ID == 2 && x.ComfortTemp != nil {
				entry = *x.ComfortTemp
			}
		}
		var d vwtDetail
		getJSON(t, r.ts, "", "/api/v1/cubes/KEQ0000100/devices/room-2", &d)
		var line any = "none"
		for _, p := range r.s.proxyVirtuals(r.rt) {
			if p.Room.ID != 2 {
				continue
			}
			if cfg, err := cube.ParseConfig(p.RF.String() + "," + base64.StdEncoding.EncodeToString(p.Config)); err == nil && cfg.Wall != nil {
				line = cfg.Wall.ComfortTemp
			}
		}
		var wall any = "none"
		if d.Wall != nil {
			wall = d.Wall.ComfortTemp
		}
		return fmt.Sprintf("config %v, entry %v, virtual %v, C line %v", v.ComfortTemp, entry, wall, line)
	}
	const want = "config 22, entry 22, virtual 22, C line 22"
	if got := comforts(); got != want {
		t.Fatalf("after the restart: %s, want %s", got, want)
	}
	on := true
	for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
		if err := r.fc.SetState(rf, fakecube.State{Reported: &on}); err != nil {
			t.Fatal(err)
		}
		testkit.Eventually(t, rf.String()+" reported", func() bool {
			st, ok := r.rt.client.Snapshot().State(rf)
			return ok && st.Valid
		})
		if got := comforts(); got != want {
			t.Fatalf("%s reported: %s, want %s", rf, got, want)
		}
	}
}

// A room entry carries the room's comfort and eco temperatures, as its
// configuration has them, by most where its master differs (room 2's
// 0a0204 at comfort 23), in GET …/rooms and the Cube's room_list, each
// device keeping its own (the wall thermostat's 21 and 17), and a room
// without a thermostat none; it names the thermostat whose setpoint its
// virtual wall thermostat shows, every thermostat in auto at 20 the
// first in the Cube's order, and with that one dead (0a0201, 0a0204) the
// next one alive, the dead out of the room's state.
func TestRoomEntryCarriesItsOrder(t *testing.T) {
	t.Parallel()
	r := startDrift(t, func(lines []cube.Message) []cube.Message {
		withConfig(t, lines, 0x0a0204, func(raw []byte) { raw[rawComfort] = 46 }) // comfort 23, the master's alone
		return lines
	})
	entries := func() map[uint8]view.Room {
		var rooms []view.Room
		if code := getJSON(t, r.ts, "", "/api/v1/cubes/KEQ0000100/rooms?devices=all", &rooms); code != 200 {
			t.Fatalf("rooms: %d", code)
		}
		out := map[uint8]view.Room{}
		for _, x := range rooms {
			out[x.ID] = x
		}
		return out
	}
	temps := func(x view.Room) string {
		if x.ComfortTemp == nil || x.EcoTemp == nil {
			return "none"
		}
		raw, _ := json.Marshal([]float64{*x.ComfortTemp, *x.EcoTemp})
		return string(raw)
	}
	got := entries()
	for _, c := range []struct {
		room       uint8
		thermostat string
		temps      string
	}{{1, "0a0201", "[22,15]"}, {2, "0a0204", "[22,15]"}, {3, "", "none"}} {
		if x := got[c.room]; x.Thermostat != c.thermostat || temps(x) != c.temps {
			t.Errorf("room %d: %q %s, want %q %s", c.room, x.Thermostat, temps(x), c.thermostat, c.temps)
		}
	}
	for _, d := range got[1].Devices {
		if d.RFAddress == "0a0201" && (*d.ComfortTemp != 21 || *d.EcoTemp != 17) {
			t.Errorf("the wall thermostat keeps its own: %v %v", *d.ComfortTemp, *d.EcoTemp)
		}
	}
	var summary struct {
		RoomList []view.Room `json:"room_list"`
	}
	getJSON(t, r.ts, "", "/api/v1/cubes/KEQ0000100", &summary)
	if len(summary.RoomList) == 0 || summary.RoomList[0].Thermostat != "0a0201" || temps(summary.RoomList[0]) != "[22,15]" {
		t.Errorf("the Cube's room list: %+v", summary.RoomList)
	}

	r.kill(0x0a0201)
	r.kill(0x0a0204)
	got = entries()
	if x := got[1]; x.Thermostat != "0a0202" || temps(x) != "[22,15]" {
		t.Errorf("room 1, its wall thermostat dead: %q %s", x.Thermostat, temps(x))
	}
	if x := got[2]; x.Thermostat != "0a0205" || temps(x) != "[22,15]" {
		t.Errorf("room 2, its master dead: %q %s", x.Thermostat, temps(x))
	}
}

// A room entry carries the room's lowest and highest setpoint, as its
// configuration has them (AP RoomsPage v1.0, Block 5), in GET …/rooms
// and the Cube's room_list: room 1 its thermostats' 4.5 and 30.5 °C, the
// recorded C lines' values; room 2 by most where its master differs
// (0a0205 and 0a0206 at 6 and 25 °C, 0a0204 at the recorded values);
// room 3, without a thermostat, neither.
func TestRoomEntryCarriesItsRange(t *testing.T) {
	t.Parallel()
	r := startDrift(t, func(lines []cube.Message) []cube.Message {
		for _, rf := range []cube.RFAddress{0x0a0205, 0x0a0206} {
			withConfig(t, lines, rf, func(raw []byte) { raw[rawMax], raw[rawMin] = 50, 12 }) // 25 and 6 °C
		}
		return lines
	})
	field := func(x map[string]any, k string) string {
		if v, ok := x[k]; ok {
			return fmt.Sprint(v)
		}
		return "absent"
	}
	check := func(what string, rooms []map[string]any) {
		t.Helper()
		got := map[string]string{}
		for _, x := range rooms {
			got[field(x, "id")] = field(x, "min_setpoint") + " " + field(x, "max_setpoint")
		}
		if want := map[string]string{"1": "4.5 30.5", "2": "6 25", "3": "absent absent"}; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: the rooms' ranges %v, want %v", what, got, want)
		}
	}
	var rooms []map[string]any
	if code := getJSON(t, r.ts, "", "/api/v1/cubes/KEQ0000100/rooms", &rooms); code != 200 {
		t.Fatalf("rooms: %d", code)
	}
	check("GET …/rooms", rooms)
	var summary struct {
		RoomList []map[string]any `json:"room_list"`
	}
	if code := getJSON(t, r.ts, "", "/api/v1/cubes/KEQ0000100", &summary); code != 200 {
		t.Fatalf("the Cube: %d", code)
	}
	check("the Cube's room_list", summary.RoomList)
}

// Over the recorded greeting, each room's range is its one radiator
// thermostat's, as its C line has it: 4.5 to 23 °C in four rooms, 4.5 to
// 30.5 °C in two.
func TestRoomEntryRangeOfTheRecordedGreeting(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet())
	var rooms []view.Room
	if code := getJSON(t, r.ts, "", "/api/v1/cubes/KEQ0000100/rooms", &rooms); code != 200 {
		t.Fatalf("rooms: %d", code)
	}
	show := func(p *float64) string {
		if p == nil {
			return "none"
		}
		return fmt.Sprint(*p)
	}
	snap := r.rt.client.Snapshot()
	ranges := map[string]int{}
	for _, x := range rooms {
		rf, err := cube.ParseRFAddress(x.Thermostat)
		if err != nil {
			t.Fatalf("room %d names no thermostat: %v", x.ID, err)
		}
		c := snap.Config(rf)
		if c == nil || c.Thermostat == nil {
			t.Fatalf("room %d: no C line of %s", x.ID, rf)
		}
		if x.MinSetpoint == nil || x.MaxSetpoint == nil || *x.MinSetpoint != c.Thermostat.MinSetpoint || *x.MaxSetpoint != c.Thermostat.MaxSetpoint {
			t.Errorf("room %d: range %s to %s, want %s's %v to %v", x.ID, show(x.MinSetpoint), show(x.MaxSetpoint), rf, c.Thermostat.MinSetpoint, c.Thermostat.MaxSetpoint)
			continue
		}
		ranges[show(x.MinSetpoint)+"-"+show(x.MaxSetpoint)]++
	}
	if want := map[string]int{"4.5-23": 4, "4.5-30.5": 2}; !reflect.DeepEqual(ranges, want) {
		t.Errorf("the rooms' ranges %v, want %v", ranges, want)
	}
}

// The virtual wall thermostat's mode, setpoint and until through the
// API (RoomMajority.D19-A): in room 2, by the newest change of mode or
// setpoint, not by the report that came last (0a0206's valve); a dead
// member passed over, its last report the newest change; by most once
// two agree. sources.mode names those the state came from.
func TestVirtualWallThermostatByTheOneOrder(t *testing.T) {
	t.Parallel()
	r := startDrift(t, nil)
	vwt := func(name string, mode cube.Mode, setpoint float64, from string) {
		t.Helper()
		testkit.Eventually(t, name, func() bool {
			d := devicesIn(t, r.ts, "all", 2)[0]
			return d.Virtual && d.State != nil && d.State.Mode == mode && d.State.Setpoint != nil && *d.State.Setpoint == setpoint &&
				strings.Join(d.Sources["mode"], ",") == from
		})
	}
	r.set(0x0a0204, "manual", 18)
	r.set(0x0a0205, "manual", 22)
	valve := 40
	if err := r.fc.SetState(0x0a0206, fakecube.State{ValvePct: &valve}); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "0a0206's valve", func() bool {
		st, ok := r.rt.client.Snapshot().State(0x0a0206)
		return ok && st.ValvePct != nil && *st.ValvePct == 40
	})
	vwt("by the newest change", cube.ModeManual, 22, "0a0205")
	r.kill(0x0a0205)
	vwt("0a0205 dead", cube.ModeManual, 18, "0a0204")
	r.set(0x0a0206, "manual", 18)
	vwt("by most", cube.ModeManual, 18, "0a0204,0a0206")
}

// The virtual wall thermostat's configuration, as GET …/devices/{rf}
// gives it and the proxy's virtual C line carries it, is the room's by
// the one order (RoomMajority.D19-A), the line still a wall thermostat's
// that the original software reads: in room 1, whose wall thermostat
// 0a0201, its master, has comfort 21, eco 17, window-open 12 °C, a boost
// of 10 min at 90 %, an offset of +1.5 and a programme of its own, while
// its radiator thermostats have comfort 22, eco 15, window-open 5 °C, 5
// min at 80 % and another programme, 0a0202 an offset of -1.0: the
// radiator thermostats' temperatures, boost and programme, and the wall
// thermostat's own offset; on a tie (0a0203 dead) the master's
// temperatures and programme, the boost still the radiator thermostats',
// since the boost, descaling and valve settings are decided as one group
// among the radiator thermostats alone, a wall thermostat's counting only
// in a room of wall thermostats; with the wall thermostat dead too, 0a0202's
// temperatures and programme, and the offset still the wall
// thermostat's, which a write of it reaches.
func TestVirtualWallThermostatConfigByTheOneOrder(t *testing.T) {
	t.Parallel()
	const rest = 0x16 + cube.WeekProgramSize // a wall thermostat's offset, window-open temperature and boost
	r := startDrift(t, func(lines []cube.Message) []cube.Message {
		withConfig(t, lines, 0x0a0201, func(raw []byte) { raw[rest], raw[rest+2] = 10, 2<<5|18 }) // +1.5, 10 min at 90 %
		withConfig(t, lines, 0x0a0202, func(raw []byte) { raw[0x16] = 5 })                        // -1.0
		return lines
	})
	snap := r.rt.client.Snapshot()
	if w, rad := snap.Config(0x0a0201).Wall, snap.Config(0x0a0202).Thermostat; w.TempOffset != 1.5 || w.BoostMinutes != 10 || w.BoostValvePct != 90 ||
		w.ComfortTemp != 21 || w.WindowOpenTemp != 12 || rad.TempOffset != -1 || rad.ComfortTemp != 22 || rad.BoostMinutes != 5 {
		t.Fatalf("the fixture: %+v %+v", w, rad)
	}
	if code, body := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100", `{"proxy_virtual":true}`); code != 200 || body["proxy_virtual"] != true {
		t.Fatalf("proxy_virtual: %d %v", code, body)
	}
	own := func(rf cube.RFAddress) cube.WeekProgram { return *roomorder.Program(r.rt.client.Snapshot().Config(rf)) }
	check := func(name string, want cube.WallConfig) {
		t.Helper()
		var d vwtDetail
		if code := getJSON(t, r.ts, "", "/api/v1/cubes/KEQ0000100/devices/room-1", &d); code != 200 || d.Wall == nil {
			t.Fatalf("%s: room-1: %d %+v", name, code, d.Wall)
		}
		var line *cube.WallConfig
		for _, v := range r.s.proxyVirtuals(r.rt) {
			if v.Room.ID != 1 {
				continue
			}
			cfg, err := cube.ParseConfig(v.RF.String() + "," + base64.StdEncoding.EncodeToString(v.Config))
			if err != nil || cfg.Type != cube.DeviceWallThermostat || cfg.RoomID != 1 || cfg.Wall == nil || len(cfg.Wall.Extra) != 0 {
				t.Fatalf("%s: the virtual C line: %+v %v", name, cfg, err)
			}
			line = cfg.Wall
		}
		if line == nil {
			t.Fatalf("%s: no virtual C line for room 1", name)
		}
		line.Extra = nil
		for what, got := range map[string]cube.WallConfig{"the detail": *d.Wall, "the C line": *line} {
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: %s %+v, want %+v", name, what, got, want)
			}
		}
	}
	check("by most", cube.WallConfig{ComfortTemp: 22, EcoTemp: 15, MaxSetpoint: 30.5, MinSetpoint: 4.5, TempOffset: 1.5, WindowOpenTemp: 5,
		BoostMinutes: 5, BoostValvePct: 80, WeekProgram: own(0x0a0202)})
	if samePrograms(own(0x0a0201), own(0x0a0202)) {
		t.Fatal("the wall thermostat's programme is the radiator thermostats'")
	}
	r.kill(0x0a0203)
	check("on a tie, the master's", cube.WallConfig{ComfortTemp: 21, EcoTemp: 17, MaxSetpoint: 30.5, MinSetpoint: 4.5, TempOffset: 1.5, WindowOpenTemp: 12,
		BoostMinutes: 5, BoostValvePct: 80, WeekProgram: own(0x0a0201)})
	r.kill(0x0a0201)
	check("the wall thermostat dead", cube.WallConfig{ComfortTemp: 22, EcoTemp: 15, MaxSetpoint: 30.5, MinSetpoint: 4.5, TempOffset: 1.5, WindowOpenTemp: 5,
		BoostMinutes: 5, BoostValvePct: 80, WeekProgram: own(0x0a0202)})
}
