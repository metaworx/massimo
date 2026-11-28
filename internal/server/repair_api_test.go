// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
	"go.metaworx.ch/massimo/internal/view"
)

// The repairs' tests run over copyGreeting (config_api_test.go): room 1
// holds the radiator thermostat 0a010a, its master, and the wall
// thermostat copyWall; copyNew, copyNew2 and copyContact are in no room
// until a test puts them somewhere.

// withMetadata changes the metadata of the greeting's M line.
func withMetadata(t *testing.T, greeting []cube.Message, change func(md *cube.Metadata)) {
	t.Helper()
	for i, g := range greeting {
		if g.Type != 'M' {
			continue
		}
		p, err := cube.ParseMetadataPart(g.Data)
		if err != nil {
			t.Fatal(err)
		}
		md, err := cube.ParseMetadata([]cube.MetadataPart{p})
		if err != nil {
			t.Fatal(err)
		}
		change(md)
		enc, err := md.Encode()
		if err != nil {
			t.Fatal(err)
		}
		greeting[i] = cube.Message{Type: 'M', Data: "00,01," + base64.StdEncoding.EncodeToString(enc)}
		return
	}
	t.Fatal("no M line")
}

// listIn lists rf, of type typ, in room in the greeting's metadata.
func listIn(t *testing.T, greeting []cube.Message, rf cube.RFAddress, typ cube.DeviceType, serial string, room uint8) {
	t.Helper()
	withMetadata(t, greeting, func(md *cube.Metadata) {
		for i := range md.Devices {
			if md.Devices[i].RFAddress == rf {
				md.Devices[i].RoomID = room
				return
			}
		}
		md.Devices = append(md.Devices, cube.DeviceMeta{Type: typ, RFAddress: rf, Serial: serial, Name: "Dev " + rf.String(), RoomID: room})
	})
}

// withContactInRoom1 puts the shutter contact into room 1, by its C line
// and by the metadata.
func withContactInRoom1(t *testing.T, greeting []cube.Message) {
	t.Helper()
	withConfigRoom(t, greeting, copyContact, 1)
	listIn(t, greeting, copyContact, cube.DeviceShutterContact, "IEQ0000003", 1)
}

// repairing is the start of the repairs' tests: copyGreeting's Cube,
// the resets read back at once.
func repairing() startOpt {
	return func(c *startConfig) {
		withGreeting(copyGreeting(c.t))(c)
		editOptions(func(o *Options) { o.ResetSettle = time.Millisecond })(c)
	}
}

// previewOf GETs a repair's preview.
func previewOf(t *testing.T, ts *httptest.Server, path string) (int, repairPlan, string) {
	t.Helper()
	code, body := callAPI(t, ts, "GET", path, "", "")
	var p repairPlan
	if code == 200 {
		if err := json.Unmarshal([]byte(body), &p); err != nil {
			t.Fatal(err)
		}
	}
	return code, p, body
}

// runRepairOf previews a repair and runs it with the preview's plan.
func runRepairOf(t *testing.T, ts *httptest.Server, path, query string) (repairPlan, int, map[string]any) {
	t.Helper()
	code, p, body := previewOf(t, ts, path+query)
	if code != 200 {
		t.Fatalf("GET %s: %d %s", path+query, code, body)
	}
	sep := "?"
	if query != "" {
		sep = "&"
	}
	code, out := do(t, ts, "POST", path+query+sep+"plan="+p.Plan, "")
	return p, code, out
}

// linkPartner is a 0x20 (add) or 0x21 to target naming partner.
func linkPartner(target cube.Target, add bool, partner cube.RFAddress, typ cube.DeviceType) string {
	return cube.CmdLinkPartner(target, add, partner, typ).String()
}

// room1 is room 1's member rf addressed with the room flag.
func room1(rf cube.RFAddress) cube.Target { return cube.Target{RFAddress: rf, RoomID: 1, Room: true} }

// direct addresses rf alone.
func direct(rf cube.RFAddress) cube.Target { return cube.Target{RFAddress: rf} }

// relinkLines are the re-link of room 1 with 0a010a, the wall
// thermostat and the contact, for add (0x20 and 0x22) or not (0x21, no
// 0x22): each member named to the room's addressee, the master 0a010a
// but for itself, where the wall thermostat takes its place; the contact
// directly where a thermostat is named; the member told of each partner.
func relinkLines(add bool) []string {
	const rad = cube.DeviceRadiatorThermostat
	const wall = cube.DeviceWallThermostat
	const contact = cube.DeviceShutterContact
	var out []string
	part := func(lines ...string) { out = append(out, lines...) }
	group := func(rf cube.RFAddress) {
		if add {
			part(cube.CmdSetGroup(rf, 1, false).String())
		}
	}
	part("z:1e,G,01",
		linkPartner(room1(copyWall), add, 0x0a010a, rad),
		linkPartner(direct(copyContact), add, 0x0a010a, rad),
		linkPartner(direct(0x0a010a), add, copyWall, wall),
		linkPartner(direct(0x0a010a), add, copyContact, contact))
	group(0x0a010a)
	part("z:1e,G,01",
		linkPartner(room1(0x0a010a), add, copyWall, wall),
		linkPartner(direct(copyContact), add, copyWall, wall),
		linkPartner(direct(copyWall), add, 0x0a010a, rad),
		linkPartner(direct(copyWall), add, copyContact, contact))
	group(copyWall)
	part("z:1e,G,01",
		linkPartner(room1(0x0a010a), add, copyContact, contact),
		linkPartner(direct(copyContact), add, 0x0a010a, rad),
		linkPartner(direct(copyContact), add, copyWall, wall))
	group(copyContact)
	return out
}

// A re-link of room 1 adds every member's links and group again, one
// command per member after the room's wakeup, and unlinks nothing; the
// preview names those lines and their cost and sends nothing, and the
// run sends exactly them, then reads the slots back.
func TestRepairRelink(t *testing.T) {
	t.Parallel()
	rg := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { withContactInRoom1(t, fc.Greeting) }))
	fc, ts := rg.fc, rg.ts
	const path = "/api/v1/cubes/KEQ0000100/rooms/1/repair/relink"
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	code, p, body := previewOf(t, ts, path)
	if code != 200 || p.Action != "relink" || p.Plan == "" || len(p.Steps) != 3 || p.Refused != "" {
		t.Fatalf("preview: %d %s", code, body)
	}
	// 14 radiograms, three of them with the room flag to a room of three:
	// two more slots each, held until the members' next wakeup.
	if p.Radiograms != 14 || p.DutyCostPct != 14*radioCostPct || p.Slots != 20 || p.Held != 6 || p.Wakeups != 3 || p.Resets != 0 || p.Metadata {
		t.Fatalf("preview's cost: %s", body)
	}
	var shown []string
	for _, st := range p.Steps {
		shown = append(shown, "z:1e,G,01")
		shown = append(shown, st.Lines...)
	}
	want := relinkLines(true)
	if strings.Join(shown, "\n") != strings.Join(want, "\n") {
		t.Fatalf("preview's lines:\n%s\nwant\n%s", strings.Join(shown, "\n"), strings.Join(want, "\n"))
	}
	if got := received.Since(sent, cubewatch.Except('l')); len(got) != 0 {
		t.Fatalf("the preview sent %v", lineStrings(got))
	}
	code, out := do(t, ts, "POST", path+"?plan="+p.Plan, "")
	if code != 200 || out["ok"] != true || out["confirmation"] != "pending" {
		t.Fatalf("run: %d %v", code, out)
	}
	sameLines(t, received, sent, want)
	steps, _ := out["steps"].([]any)
	if len(steps) != 3 {
		t.Fatalf("steps %v", out["steps"])
	}
	for _, st := range steps {
		if m := st.(map[string]any); m["accepted"] != m["lines"] || m["error"] != nil {
			t.Fatalf("step %v", m)
		}
	}
}

// A preview counts the resets the queue's prelude sends, by the
// engine's list (AP RadioHealth v1.3, D204-A): with 0a010a's
// communication error set, the re-link's first step resets it as its
// Target, and each later step as the flagged member its room-flagged
// line reaches; the run sends exactly those r:, after each wakeup, the
// fake keeping the flag.
func TestRepairPreviewCountsTheQueuesResets(t *testing.T) {
	t.Parallel()
	r := startServer(t, repairing(), withFake(func(fc *fakecube.Server) {
		withContactInRoom1(t, fc.Greeting)
		withErrorFlag(t, fc.Greeting, 0x0a010a, 0x03)
	}))
	fc, ts := r.fc, r.ts
	const path = "/api/v1/cubes/KEQ0000100/rooms/1/repair/relink"
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	code, p, body := previewOf(t, ts, path)
	if code != 200 || len(p.Steps) != 3 || p.Resets != 3 || p.Radiograms != 14 {
		t.Fatalf("preview: %d %s", code, body)
	}
	for i, st := range p.Steps {
		if !st.Reset {
			t.Fatalf("step %d without its reset: %s", i, body)
		}
	}
	if code, out := do(t, ts, "POST", path+"?plan="+p.Plan, ""); code != 200 || out["ok"] != true {
		t.Fatalf("run: %d %v", code, out)
	}
	reset := cube.CmdResetError(0x0a010a).String()
	var want []string
	for _, l := range relinkLines(true) {
		want = append(want, l)
		if l == "z:1e,G,01" {
			want = append(want, reset)
		}
	}
	sameLines(t, received, sent, want)
}

// nothingOf asserts that a repair's preview has nothing to send, why
// naming want, and that its run is refused with 409, with the preview's
// code and params.
func nothingOf(t *testing.T, ts *httptest.Server, path, want string) repairPlan {
	t.Helper()
	code, p, body := previewOf(t, ts, path)
	if code != 200 || p.Plan != "" || len(p.Steps) != 0 || !strings.Contains(p.Nothing, want) {
		t.Fatalf("GET %s: %d %s, want nothing to send for %q", path, code, body, want)
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	code, out := do(t, ts, "POST", path+sep+"plan=0", "")
	var params any
	if p.Params != nil {
		params = p.Params
	}
	if code != http.StatusConflict || out["code"] != nil && out["code"] != p.Code || out["code"] == nil && p.Code != "" || !reflect.DeepEqual(out["params"], params) {
		t.Fatalf("POST %s: %d %v, want 409 with %q %v", path, code, out, p.Code, p.Params)
	}
	return p
}

// A room whose list and Cube disagree is not re-linked: the placement
// repairs it first.
func TestRepairRelinkRefusesADisagreeingRoom(t *testing.T) {
	t.Parallel()
	r := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyContact, 1) }))
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/rooms/1/repair/relink", "0d0e0f")
	if got := received.Since(sent, cubewatch.Except('l')); len(got) != 0 {
		t.Fatalf("sent %v", lineStrings(got))
	}
}

// A run needs the preview's plan, and one whose plan differs from the
// repair as it is now is refused: nothing is sent.
func TestRepairNeedsItsPlan(t *testing.T) {
	t.Parallel()
	r := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { withContactInRoom1(t, fc.Greeting) }))
	fc, ts := r.fc, r.ts
	const path = "/api/v1/cubes/KEQ0000100/rooms/1/repair/relink"
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	if code, out := do(t, ts, "POST", path, ""); code != http.StatusBadRequest {
		t.Fatalf("without a plan: %d %v", code, out)
	}
	if code, out := do(t, ts, "POST", path+"?plan=0000", ""); code != http.StatusConflict || !strings.Contains(out["error"].(string), "preview it again") {
		t.Fatalf("with another plan: %d %v", code, out)
	}
	if got := received.Since(sent, cubewatch.Except('l')); len(got) != 0 {
		t.Fatalf("sent %v", lineStrings(got))
	}
}

// A repair that would leave fewer free slots than the queue waits for
// is refused before it starts, in its preview and its run.
func TestRepairRefusedForTheSlots(t *testing.T) {
	t.Parallel()
	rg := startServer(t, repairing(), withFake(func(fc *fakecube.Server) {
		withContactInRoom1(t, fc.Greeting)
		fc.SetFreeSlots(8)
	}))
	fc, ts := rg.fc, rg.ts
	const path = "/api/v1/cubes/KEQ0000100/rooms/1/repair/relink"
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	code, p, body := previewOf(t, ts, path)
	if code != 200 || !strings.Contains(p.Refused, "memory slots") || p.Code != "repair_memory_slots" || p.Params["free"] != float64(8) || p.Params["min"] != float64(5) {
		t.Fatalf("preview with 8 slots free: %d %s", code, body)
	}
	if code, out := do(t, ts, "POST", path+"?plan="+p.Plan, ""); code != http.StatusConflict || out["code"] != "repair_memory_slots" || out["error"] != p.Refused {
		t.Fatalf("run: %d %v", code, out)
	}
	if got := received.Since(sent, cubewatch.Except('l')); len(got) != 0 {
		t.Fatalf("sent %v", lineStrings(got))
	}
}

// clearOnReset makes the fake Cube clear a device's error flag at its
// r:, as the Cube does (docs/cube-protocol.md, the r: findings).
func clearOnReset(t *testing.T, fc *fakecube.Server) {
	fc.DiscardIf = func(m cube.Message) bool {
		if m.Type != 'r' {
			return false
		}
		_, b64, _ := strings.Cut(m.Data, ",")
		raw, err := base64.StdEncoding.DecodeString(b64)
		for i := 0; err == nil && i+3 <= len(raw); i += 3 {
			off := false
			fc.SetState(cube.RFAddressFromBytes(raw[i:i+3]), fakecube.State{Error: &off})
		}
		return false
	}
}

// The resets of a room and of the Cube: one r: per flagged device, by
// their C lines, free; the device list read after them shows the flags
// gone, or names those still set.
func TestRepairReset(t *testing.T) {
	t.Parallel()
	flag := func(fc *fakecube.Server) {
		withErrorFlag(t, fc.Greeting, 0x0a010a, cube.RFRemoveLink)
		withErrorFlag(t, fc.Greeting, 0x0a010d, 0x03)
	}
	t.Run("room", func(t *testing.T) {
		r := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { flag(fc); clearOnReset(t, fc) }))
		fc, ts := r.fc, r.ts
		received := cubewatch.Lines(fc)
		sent := received.Mark()
		p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/rooms/1/repair/reset", "")
		if p.Radiograms != 0 || p.Resets != 1 || p.DutyCostPct != 0 || p.Wakeups != 0 || p.Refused != "" {
			t.Fatalf("preview %+v", p)
		}
		if code != 200 || out["ok"] != true || out["confirmation"] != nil {
			t.Fatalf("run: %d %v", code, out)
		}
		if still, _ := out["still_flagged"].([]any); len(still) != 0 {
			t.Fatalf("still flagged: %v", still)
		}
		sameLines(t, received, sent, []string{cube.CmdResetError(0x0a010a).String()})
	})
	t.Run("cube", func(t *testing.T) {
		r := startServer(t, repairing(), withFake(flag))
		fc, ts := r.fc, r.ts
		received := cubewatch.Lines(fc)
		sent := received.Mark()
		p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/repair/reset", "")
		if p.Resets != 2 || p.Radiograms != 0 {
			t.Fatalf("preview %+v", p)
		}
		still, _ := out["still_flagged"].([]any)
		if code != 200 || len(still) != 2 {
			t.Fatalf("run, the fake keeping the flags: %d %v", code, out)
		}
		sameLines(t, received, sent, []string{cube.CmdResetError(0x0a010d).String(), cube.CmdResetError(0x0a010a).String()})
	})
	t.Run("nothing flagged", func(t *testing.T) {
		ts := startServer(t, repairing()).ts
		nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/rooms/1/repair/reset", "no device of room 1 has a communication error")
		nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/repair/reset", "no device of the cube")
	})
}

// withRoom1Thermostats puts the two new radiator thermostats into room
// 1 beside 0a010a and the wall thermostat, by their C lines and the
// metadata: copyNew keeps its own settings (0a010d's, with room 1's
// Saturday and Sunday), copyNew2 gets 0a010a's where same is set.
func withRoom1Thermostats(t *testing.T, greeting []cube.Message, same bool) {
	t.Helper()
	ref := rawConfig(t, greeting, 0x0a010a)
	withConfigRoom(t, greeting, copyNew, 1)
	listIn(t, greeting, copyNew, cube.DeviceRadiatorThermostat, "TEQ0000001", 1)
	if !same {
		return
	}
	for i, g := range greeting {
		if g.Type == 'C' && strings.HasPrefix(g.Data, copyNew2.String()+",") {
			greeting[i] = configLine(ref, copyNew2, 1, "TEQ0000004")
		}
	}
	listIn(t, greeting, copyNew2, cube.DeviceRadiatorThermostat, "TEQ0000004", 1)
}

// A thermostat's configuration is re-sent from what most of its room's
// other thermostats share by their C lines: 0a010a and copyNew2 share
// temperatures that the wall thermostat does not, all three 0a010a's
// week programme; copyNew gets the five days of it that differ, then the
// shared temperatures with its own offset and window-open minutes, and
// its C line is read again.
func TestRepairResendConfig(t *testing.T) {
	t.Parallel()
	r := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { withRoom1Thermostats(t, fc.Greeting, true) }))
	fc, ts, s := r.fc, r.ts, r.s
	snap := s.cubes["keq0000100"].client.Snapshot()
	refProg := snap.Config(0x0a010a).Thermostat.WeekProgram
	own := snap.Config(copyNew).Thermostat
	var want []string
	want = append(want, "z:1e,G,01")
	for _, d := range []cube.Weekday{cube.Monday, cube.Tuesday, cube.Wednesday, cube.Thursday, cube.Friday} {
		ls, err := cube.CmdSetDayProgram(direct(copyNew), d, refProg[d])
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, lineStrings(ls)...)
	}
	temps, err := cube.CmdSetTemperatures(direct(copyNew), cube.TemperatureSettings{Comfort: 22, Eco: 15, Max: 30.5, Min: 4.5,
		Offset: own.TempOffset, WindowOpen: 5, WindowOpenMinutes: own.WindowOpenMins})
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, temps.String(), "c:"+copyNew.String())
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0a0b0c/repair/config", "")
	if p.Action != "config" || p.Details["others"] != float64(3) || p.Details["temperatures_by"] != float64(2) || p.Details["program_by"] != float64(3) {
		t.Fatalf("preview %+v", p)
	}
	if code != 200 || out["ok"] != true {
		t.Fatalf("run: %d %v", code, out)
	}
	sameLines(t, received, sent, want)
}

// On a tie the room's order decides (RoomMajority.D9), where the repair
// refused before: room 1's other thermostats, 0a010a, its master, and
// the wall thermostat, differ in their temperatures, and copyNew gets
// 0a010a's with its own offset and window-open minutes, after the five
// days of the programme both share that differ from its own.
func TestRepairResendOnATie(t *testing.T) {
	t.Parallel()
	r := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { withRoom1Thermostats(t, fc.Greeting, false) }))
	fc, ts, s := r.fc, r.ts, r.s
	snap := s.cubes["keq0000100"].client.Snapshot()
	refProg := snap.Config(0x0a010a).Thermostat.WeekProgram
	own := snap.Config(copyNew).Thermostat
	want := []string{"z:1e,G,01"}
	for _, d := range []cube.Weekday{cube.Monday, cube.Tuesday, cube.Wednesday, cube.Thursday, cube.Friday} {
		ls, err := cube.CmdSetDayProgram(direct(copyNew), d, refProg[d])
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, lineStrings(ls)...)
	}
	temps, err := cube.CmdSetTemperatures(direct(copyNew), cube.TemperatureSettings{Comfort: 22, Eco: 15, Max: 30.5, Min: 4.5,
		Offset: own.TempOffset, WindowOpen: 5, WindowOpenMinutes: own.WindowOpenMins})
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, temps.String(), "c:"+copyNew.String())
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0a0b0c/repair/config", "")
	if p.Action != "config" || p.Details["others"] != float64(2) || p.Details["temperatures_by"] != float64(1) || p.Details["program_by"] != float64(2) {
		t.Fatalf("preview %+v", p)
	}
	if code != 200 || out["ok"] != true {
		t.Fatalf("run: %d %v", code, out)
	}
	sameLines(t, received, sent, want)
}

// Without another thermostat the Cube holds a valid state for, nothing
// is re-sent and the answer says why: a device alone in its room has
// nothing to take its configuration from, nor one whose room's other
// thermostats have not reported since the Cube's restart.
func TestRepairResendRefused(t *testing.T) {
	t.Parallel()
	r := startServer(t, repairing(), withFake(func(fc *fakecube.Server) {
		withRoom1Thermostats(t, fc.Greeting, false)
		fc.Greeting = withoutValues(t, withoutValues(t, fc.Greeting, 0x0a010a), copyWall)
	}))
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0a0b0c/repair/config",
		"room 1 has no other thermostat the cube holds a valid state for to take the configuration from: its 2 others")
	nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0a010d/repair/config", "room 6 has no other thermostat to take the configuration from")
	// each with its code
	checkNothing(t, ts, "/api/v1/cubes/KEQ0000100/devices/0a0b0c/repair/config", "repair_no_valid_thermostat", `{"room": 1, "dead": 2}`)
	checkNothing(t, ts, "/api/v1/cubes/KEQ0000100/devices/0a010d/repair/config", "repair_no_other_thermostat", `{"room": 6}`)
	if got := received.Since(sent, cubewatch.Except('l')); len(got) != 0 {
		t.Fatalf("sent %v", lineStrings(got))
	}
}

// A device the metadata lists in room 1 whose C line names no room is
// placed there with the placement's link half, the metadata left as it
// is; one whose C line names room 6 is moved from there. The link half
// wakes the device itself after room 1, whose group it is not in yet,
// which the preview counts.
func TestRepairPlace(t *testing.T) {
	t.Parallel()
	t.Run("from no room", func(t *testing.T) {
		r := startServer(t, repairing(), withFake(func(fc *fakecube.Server) {
			listIn(t, fc.Greeting, copyContact, cube.DeviceShutterContact, "IEQ0000003", 1)
		}))
		fc, ts := r.fc, r.ts
		received := cubewatch.Lines(fc)
		sent := received.Mark()
		p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0d0e0f/repair/place", "")
		if p.Action != "move" || len(p.Steps) != 1 || p.Radiograms != 4 || p.Wakeups != 2 { // room 1's and the contact's own
			t.Fatalf("preview %+v", p)
		}
		if code != 200 || out["confirmation"] != "pending" {
			t.Fatalf("run: %d %v", code, out)
		}
		sameLines(t, received, sent, []string{"z:1e,G,01", "z:1e,D,0d0e0f",
			linkPartner(room1(0x0a010a), true, copyContact, cube.DeviceShutterContact),
			linkPartner(direct(copyContact), true, 0x0a010a, cube.DeviceRadiatorThermostat),
			linkPartner(direct(copyContact), true, copyWall, cube.DeviceWallThermostat),
			cube.CmdSetGroup(copyContact, 1, false).String(),
			"c:" + copyContact.String()})
	})
	t.Run("from room 6", func(t *testing.T) {
		r := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyWall, 6) }))
		fc, ts := r.fc, r.ts
		received := cubewatch.Lines(fc)
		sent := received.Mark()
		_, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0c0d0e/repair/place", "")
		if code != 200 {
			t.Fatalf("run: %d %v", code, out)
		}
		sameLines(t, received, sent, []string{"z:1e,G,06",
			cube.CmdSetGroup(copyWall, 6, true).String(),
			linkPartner(cube.Target{RFAddress: 0x0a010d, RoomID: 6, Room: true}, false, copyWall, cube.DeviceWallThermostat),
			linkPartner(direct(copyWall), false, 0x0a010d, cube.DeviceRadiatorThermostat),
			"z:1e,G,01", "z:1e,D,0c0d0e",
			linkPartner(room1(0x0a010a), true, copyWall, cube.DeviceWallThermostat),
			linkPartner(direct(copyWall), true, 0x0a010a, cube.DeviceRadiatorThermostat),
			cube.CmdSetGroup(copyWall, 1, false).String(),
			"c:" + copyWall.String()})
	})
	t.Run("in its room", func(t *testing.T) {
		ts := startServer(t, repairing()).ts
		nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0a010a/repair/place", "in room 1 already")
	})
}

// listedRoom is the room the server's metadata lists rf in, false where
// it does not list it.
func listedRoom(s *Server, rf cube.RFAddress) (uint8, bool) {
	for _, d := range s.cubes["keq0000100"].client.Snapshot().Metadata().Devices {
		if d.RFAddress == rf {
			return d.RoomID, true
		}
	}
	return 0, false
}

// A device is listed where its C line has it, metadata only: moved in
// the list from room 1 to room 6; one the list does not have, in a room
// it does not have, is listed there once the room has a name, the
// device its master.
func TestRepairList(t *testing.T) {
	t.Parallel()
	t.Run("another room", func(t *testing.T) {
		rg := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyWall, 6) }))
		fc, ts, s := rg.fc, rg.ts, rg.s
		received := cubewatch.Lines(fc)
		sent := received.Mark()
		p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/devices/0c0d0e/repair/list", "")
		if p.Radiograms != 0 || !p.Metadata || p.DutyCostPct != 0 {
			t.Fatalf("preview %+v", p)
		}
		if code != 200 || out["ok"] != true {
			t.Fatalf("run: %d %v", code, out)
		}
		for _, m := range received.Since(sent, cubewatch.Except('l')) {
			if m.Type != 'm' {
				t.Fatalf("sent %s", m)
			}
		}
		if room, ok := listedRoom(s, copyWall); !ok || room != 6 {
			t.Fatalf("listed in %d %v", room, ok)
		}
	})
	t.Run("a new room", func(t *testing.T) {
		rg := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyNew, 7) }))
		ts, s := rg.ts, rg.s
		const path = "/api/v1/cubes/KEQ0000100/devices/0a0b0c/repair/list"
		if p := nothingOf(t, ts, path, "give room_name"); p.Details["code"] != "room_name_needed" || p.Details["room"] != float64(7) {
			t.Fatalf("without the room's name: %+v", p)
		}
		if _, code, out := runRepairOf(t, ts, path, "?room_name="+url.QueryEscape("Attic")); code != 200 {
			t.Fatalf("run: %d %v", code, out)
		}
		if room, ok := listedRoom(s, copyNew); !ok || room != 7 {
			t.Fatalf("listed in %d %v", room, ok)
		}
		for _, r := range s.cubes["keq0000100"].client.Snapshot().Metadata().Rooms {
			if r.ID == 7 && (r.Name != "Attic" || r.RFAddress != copyNew) {
				t.Fatalf("room 7: %+v", r)
			}
		}
	})
}

// A device the list lacks is listed under the name the request gives,
// else "<room name> <type> <n>", n the lowest number the names of that
// form in the room leave free, whatever their case (D185); the run
// writes exactly the Cube's metadata with the device's entry added. The
// fixture's room 1 is "Wohnzimmer", its devices 0a010a and the wall
// thermostat.
func TestRepairListNamesTheDevice(t *testing.T) {
	t.Parallel()
	const path = "/api/v1/cubes/KEQ0000100/devices/0a0b0c/repair/list"
	for _, c := range []struct {
		name  string
		names map[cube.RFAddress]string // room 1's devices renamed
		query string
		want  string
	}{
		{"the default", nil, "", "Wohnzimmer radiator thermostat 1"},
		{"a taken n", map[cube.RFAddress]string{0x0a010a: "wohnzimmer Radiator Thermostat 1", copyWall: "Wohnzimmer radiator thermostat 3"}, "",
			"Wohnzimmer radiator thermostat 2"},
		{"another type's name takes no n", map[cube.RFAddress]string{copyWall: "Wohnzimmer wall thermostat 1"}, "", "Wohnzimmer radiator thermostat 1"},
		{"a given name", nil, "?name=" + url.QueryEscape(" Sofa side "), "Sofa side"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := startServer(t, repairing(), withFake(func(fc *fakecube.Server) {
				withConfigRoom(t, fc.Greeting, copyNew, 1)
				withMetadata(t, fc.Greeting, func(md *cube.Metadata) {
					for i := range md.Devices {
						if n, ok := c.names[md.Devices[i].RFAddress]; ok {
							md.Devices[i].Name = n
						}
					}
				})
			}))
			fc, ts, s := r.fc, r.ts, r.s
			before := *s.cubes["keq0000100"].client.Snapshot().Metadata()
			received := cubewatch.Lines(fc)
			sent := received.Mark()
			p, code, out := runRepairOf(t, ts, path, c.query)
			if p.Details["name"] != c.want || p.Details["room_name"] != "Wohnzimmer" || p.Details["type"] != "radiator_thermostat" || p.Details["listed"] != false {
				t.Fatalf("the preview's details %v", p.Details)
			}
			if code != 200 || out["ok"] != true {
				t.Fatalf("run: %d %v", code, out)
			}
			want := before
			want.Devices = append(append([]cube.DeviceMeta(nil), before.Devices...),
				cube.DeviceMeta{Type: cube.DeviceRadiatorThermostat, RFAddress: copyNew, Serial: "TEQ0000001", Name: c.want, RoomID: 1})
			lines, err := cube.WriteMetadataCommands(&want)
			if err != nil {
				t.Fatal(err)
			}
			var wantLines, got []string
			for _, l := range lines {
				wantLines = append(wantLines, l.String())
			}
			for _, m := range received.Since(sent, cubewatch.Except('l')) {
				got = append(got, m.String())
			}
			if strings.Join(got, "\n") != strings.Join(wantLines, "\n") {
				t.Fatalf("sent\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantLines, "\n"))
			}
		})
	}
	t.Run("a name too long", func(t *testing.T) {
		ts := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyNew, 1) })).ts
		if code, body := callAPI(t, ts, "GET", path+"?name="+strings.Repeat("x", 33), "", ""); code != http.StatusBadRequest {
			t.Fatalf("GET: %d %s", code, body)
		}
	})
	t.Run("a new room's name too long, dropped", func(t *testing.T) {
		ts := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { withConfigRoom(t, fc.Greeting, copyNew, 7) })).ts
		code, p, body := previewOf(t, ts, path+"?room_name="+url.QueryEscape("Wohnzimmer im Erdgeschoss"))
		if code != 200 || p.Details["name"] != "radiator thermostat 1" || p.Details["room_name"] != "Wohnzimmer im Erdgeschoss" {
			t.Fatalf("GET: %d %s", code, body)
		}
	})
}

// The default name's rules (D185, D201): the lowest free n, a name of
// that form whatever its case; where the whole would pass 32 bytes,
// with a room's name of umlauts or a two-digit n, the room's name
// dropped, "<type> <n>", n the lowest free in that form.
func TestDefaultDeviceName(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		room  string
		typ   cube.DeviceType
		taken []string
		want  string
	}{
		{"Bath", cube.DeviceWallThermostat, nil, "Bath wall thermostat 1"},
		{"Bath", cube.DeviceShutterContact, []string{"Bath window contact 2", "BATH WINDOW CONTACT 1", "Bath window contact 04"}, "Bath window contact 3"},
		{"Bath", cube.DeviceEcoSwitch, []string{"Bath eco button", "Bath eco button x", "Bath eco button -1", "Bath eco button 0"}, "Bath eco button 1"},
		{"Bath", cube.DeviceRadiatorThermostatPlus, nil, "Bath radiator thermostat plus 1"},
		{"Wohnzimmer", cube.DeviceRadiatorThermostat, nil, "Wohnzimmer radiator thermostat 1"},
		{"Wohnzimmer Süd", cube.DeviceShutterContact, nil, "Wohnzimmer Süd window contact 1"}, // 32 bytes
		{"Wohnzimmer Süd", cube.DeviceRadiatorThermostat, nil, "radiator thermostat 1"},
		{"aÄÄÄÄÄÄ", cube.DeviceRadiatorThermostat, nil, "radiator thermostat 1"},
		{"Attic over the garage", cube.DeviceRadiatorThermostat, []string{"Attic over radiator thermostat 1", "radiator thermostat 1"},
			"radiator thermostat 2"},
		{"Wohnzimmer", cube.DeviceRadiatorThermostat, []string{"Wohnzimmer radiator thermostat 1", "Wohnzimmer radiator thermostat 2",
			"Wohnzimmer radiator thermostat 3", "Wohnzimmer radiator thermostat 4", "Wohnzimmer radiator thermostat 5",
			"Wohnzimmer radiator thermostat 6", "Wohnzimmer radiator thermostat 7", "Wohnzimmer radiator thermostat 8",
			"Wohnzimmer radiator thermostat 9"}, "radiator thermostat 1"},
		{"", cube.DeviceWallThermostat, []string{"wall thermostat 1"}, "wall thermostat 2"},
		{" ", cube.DeviceEcoSwitch, nil, "eco button 1"},
	} {
		got := defaultDeviceName(c.room, c.typ, c.taken)
		if got != c.want || len(got) > maxNameBytes || checkName(got) != nil {
			t.Errorf("%q %s %v: %q (%d bytes), want %q", c.room, c.typ, c.taken, got, len(got), c.want)
		}
	}
}

// A room whose metadata names a device of another room as its master
// gets its wall thermostat as master, metadata only; a valid master is
// left alone.
func TestRepairMaster(t *testing.T) {
	t.Parallel()
	rg := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { withRoomMaster(t, fc.Greeting, 1, 0x0a010d) }))
	fc, ts, s := rg.fc, rg.ts, rg.s
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	p, code, out := runRepairOf(t, ts, "/api/v1/cubes/KEQ0000100/rooms/1/repair/master", "")
	if p.Details["new_master"] != copyWall.String() || p.Radiograms != 0 || !p.Metadata {
		t.Fatalf("preview %+v", p)
	}
	if code != 200 {
		t.Fatalf("run: %d %v", code, out)
	}
	for _, m := range received.Since(sent, cubewatch.Except('l')) {
		if m.Type != 'm' {
			t.Fatalf("sent %s", m)
		}
	}
	for _, r := range s.cubes["keq0000100"].client.Snapshot().Metadata().Rooms {
		if r.ID == 1 && r.RFAddress != copyWall {
			t.Fatalf("room 1's master %s", r.RFAddress)
		}
	}
	nothingOf(t, ts, "/api/v1/cubes/KEQ0000100/rooms/1/repair/master", "a thermostat of the room already")
}

// Every preview sends nothing to the Cube, whichever repair.
func TestRepairPreviewsSendNothing(t *testing.T) {
	t.Parallel()
	r := startServer(t, repairing(), withFake(func(fc *fakecube.Server) {
		withContactInRoom1(t, fc.Greeting)
		withErrorFlag(t, fc.Greeting, 0x0a010a, 0x03)
		withRoomMaster(t, fc.Greeting, 1, 0x0a010d)
		withConfigRoom(t, fc.Greeting, copyNew, 1)
	}))
	fc, ts := r.fc, r.ts
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	const base = "/api/v1/cubes/KEQ0000100"
	for _, path := range []string{"/repair/reset", "/rooms/1/repair/reset", "/rooms/1/repair/master", "/rooms/1/repair/relink",
		"/rooms/1/repair/unlink-relink", "/devices/0a0b0c/repair/config", "/devices/0a0b0c/repair/place", "/devices/0a0b0c/repair/list"} {
		if code, _, body := previewOf(t, ts, base+path); code >= 500 {
			t.Fatalf("GET %s: %d %s", path, code, body)
		}
	}
	if got := received.Since(sent, cubewatch.Except('l')); len(got) != 0 {
		t.Fatalf("the previews sent %v", lineStrings(got))
	}
}

// A role without repair is refused every repair, its preview included,
// and sends nothing; one with repair alone is let through, but for
// "unlink all and re-link", which needs structure as well.
func TestRepairNeedsRepair(t *testing.T) {
	t.Parallel()
	rg := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	fc, ts, s := rg.fc, rg.ts, rg.s
	st := s.opts.Store
	st.AddRole(t.Context(), store.Role{Key: "builder", Capabilities: []string{"read", "configure", "structure", "pair"}}, 0)
	st.AddRole(t.Context(), store.Role{Key: "fixer", Capabilities: []string{"read", "repair"}}, 0)
	builder := userToken(t, st, "builder", "builder")
	fixer := userToken(t, st, "fixer", "fixer")
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	for _, r := range registeredRoutes(t) {
		if r.capability != "repair" {
			continue
		}
		code, text := callAPI(t, ts, r.method, fillRoute(r.path)+"?plan=0", builder, "")
		if code != http.StatusForbidden || errorOf(text) != `capability "repair" required` {
			t.Errorf("builder: %s %s: %d %s", r.method, r.path, code, text)
		}
		code, text = callAPI(t, ts, r.method, fillRoute(r.path)+"?plan=0", fixer, "")
		needsStructure := strings.HasSuffix(r.path, "/unlink-relink")
		if refused := code == http.StatusForbidden; refused != needsStructure ||
			needsStructure && errorOf(text) != `capability "structure" required` {
			t.Errorf("fixer: %s %s: %d %s", r.method, r.path, code, text)
		}
	}
	if got := received.Since(sent, cubewatch.Except('l')); len(got) != 0 {
		t.Fatalf("sent %v", lineStrings(got))
	}
}

// "Unlink all and re-link" removes every link of room 1, one command per
// member with the 0x21s of a move's unlink half and no 0x23, then sends
// the re-link of every member; its preview says it has not yet run on
// hardware, and its run sends exactly the lines it showed.
func TestRepairUnlinkRelink(t *testing.T) {
	t.Parallel()
	r := startServer(t, repairing(), withFake(func(fc *fakecube.Server) { withContactInRoom1(t, fc.Greeting) }))
	fc, ts := r.fc, r.ts
	const path = "/api/v1/cubes/KEQ0000100/rooms/1/repair/unlink-relink"
	received := cubewatch.Lines(fc)
	sent := received.Mark()
	code, p, body := previewOf(t, ts, path)
	if code != 200 || p.Action != "unlink_relink" || !p.Untested || len(p.Notes) != 1 || len(p.Steps) != 6 {
		t.Fatalf("preview: %d %s", code, body)
	}
	// 11 radiograms out, 14 in; six with the room flag to a room of three.
	if p.Radiograms != 25 || p.Held != 12 || p.Wakeups != 6 || p.Refused != "" {
		t.Fatalf("preview's cost: %s", body)
	}
	if got := received.Since(sent, cubewatch.Except('l')); len(got) != 0 {
		t.Fatalf("the preview sent %v", lineStrings(got))
	}
	code, out := do(t, ts, "POST", path+"?plan="+p.Plan, "")
	if code != 200 || out["ok"] != true || out["confirmation"] != "pending" {
		t.Fatalf("run: %d %v", code, out)
	}
	sameLines(t, received, sent, append(relinkLines(false), relinkLines(true)...))
}

// The repair re-sends what the report of members that differ names
// (RoomMajority.D9): room 1's 0a010a, its master, and the wall
// thermostat differ in their temperatures, a tie the master's decides,
// so the report finds the wall thermostat's; its repair sends it
// 0a010a's, and 0a010a's repair has nothing to send, the room's being
// its own.
func TestRepairResendNamesWhatTheReportNames(t *testing.T) {
	t.Parallel()
	r := startServer(t, repairing())
	snap := r.rt.client.Snapshot()
	var found []string
	mismatches, _ := findMismatches(snap, snap.Metadata(), view.NewHidden(nil, snap.Metadata(), false), nil)
	for _, m := range mismatches {
		if m.Room == 1 && (m.Finding == view.MismatchTemperatures || m.Finding == view.MismatchProgramme) {
			found = append(found, m.Finding+" "+m.Device)
		}
	}
	if !slices.Equal(found, []string{"temperatures 0c0d0e"}) {
		t.Fatalf("the report of room 1: %v", found)
	}
	nothingOf(t, r.ts, "/api/v1/cubes/KEQ0000100/devices/0a010a/repair/config", "device 0a010a has room 1's temperatures and week programme already")
	code, p, body := previewOf(t, r.ts, "/api/v1/cubes/KEQ0000100/devices/0c0d0e/repair/config")
	w := snap.Config(copyWall).Wall
	temps, err := cube.CmdSetTemperatures(direct(copyWall), cube.TemperatureSettings{Comfort: 22, Eco: 15, Max: 30.5, Min: 4.5,
		Offset: w.TempOffset, WindowOpen: 5, WindowOpenMinutes: cubeio.RoomRadiator(snap, 1, copyWall).WindowOpenMins})
	if err != nil {
		t.Fatal(err)
	}
	if code != 200 || len(p.Steps) != 1 || !slices.Equal(p.Steps[0].Lines, []string{temps.String()}) ||
		p.Details["others"] != float64(1) || p.Details["thermostats"] != float64(2) || p.Details["temperatures_by"] != float64(1) {
		t.Fatalf("the wall thermostat's repair: %d %s", code, body)
	}
}
