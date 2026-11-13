// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// What the log's lines say (AP Logging v1.1, Block 4): the differences
// of the room and device list and of a configuration, the summaries, a
// reconnect at info as one line with the greeting's own lines at debug,
// and what changed while the Cube was disconnected or the server down.

import (
	"context"
	"encoding/base64"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// The differences of two room and device lists, rooms first, each in the
// list's order; the hidden devices and their rooms left out.
func TestMetadataChanges(t *testing.T) {
	old := &cube.Metadata{
		Rooms: []cube.RoomMeta{{ID: 1, Name: "Bath"}, {ID: 2, Name: "Kitchen"}, {ID: 3, Name: "Attic"}, {ID: 9, Name: "Secret"}},
		Devices: []cube.DeviceMeta{
			{RFAddress: 0x0a0101, Serial: "MEQ0000101", Name: "Window", RoomID: 1},
			{RFAddress: 0x0a0102, Serial: "MEQ0000102", Name: "Door", RoomID: 2},
			{RFAddress: 0x0a0103, Serial: "MEQ0000103", Name: "Old", RoomID: 3},
			{RFAddress: 0x0a0109, Serial: "MEQ0000109", Name: "Hidden", RoomID: 9},
		},
	}
	cur := &cube.Metadata{
		Rooms: []cube.RoomMeta{{ID: 1, Name: "Bathroom"}, {ID: 2, Name: "Kitchen"}, {ID: 4, Name: "Hall"}, {ID: 9, Name: "Secret too"}},
		Devices: []cube.DeviceMeta{
			{RFAddress: 0x0a0101, Serial: "MEQ0000101", Name: "Window left", RoomID: 2},
			{RFAddress: 0x0a0102, Serial: "MEQ0000102", Name: "Door", RoomID: 2},
			{RFAddress: 0x0a0104, Serial: "MEQ0000104", Name: "New", RoomID: 4},
			{RFAddress: 0x0a0109, Serial: "MEQ0000109", Name: "Hidden renamed", RoomID: 9},
		},
	}
	got := listChanges(old, cur, view.Hidden{0x0a0109: true})
	want := []listChange{
		{What: "room_renamed", Room: 1, Name: "Bathroom", From: "Bath"},
		{What: "room_added", Room: 4, Name: "Hall"},
		{What: "room_deleted", Room: 3, Name: "Attic"},
		{What: "device_renamed", Device: "0a0101", Serial: "MEQ0000101", Name: "Window left", From: "Window"},
		{What: "device_moved", Device: "0a0101", Serial: "MEQ0000101", Name: "Window left", Room: 2, From: "Bathroom", To: "Kitchen"},
		{What: "device_added", Device: "0a0104", Serial: "MEQ0000104", Name: "New", Room: 4, To: "Hall"},
		{What: "device_left", Device: "0a0103", Serial: "MEQ0000103", Name: "Old", From: "Attic"},
	}
	// The move's From is the old list's name of the room it left.
	want[4].From = "Bath"
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n%+v\nwant\n%+v", got, want)
	}
	if c := listChanges(cur, cur, nil); c != nil {
		t.Fatalf("no difference: %+v", c)
	}
	if c := listChanges(nil, cur, nil); c != nil {
		t.Fatalf("nothing to compare with: %+v", c)
	}
}

// The differences of two configurations, by their JSON names; a day's
// program by its name alone.
func TestConfigChanges(t *testing.T) {
	prog := cube.WeekProgram{}
	old := &cube.DeviceConfig{Type: cube.DeviceRadiatorThermostat, Firmware: 0x10,
		Thermostat: &cube.ThermostatConfig{ComfortTemp: 21, EcoTemp: 17, TempOffset: 0, WeekProgram: prog}}
	changed := prog
	changed[2] = cube.DayProgram{{Temp: 20, Until: 1440}}
	cur := &cube.DeviceConfig{Type: cube.DeviceRadiatorThermostat, Firmware: 0x10,
		Thermostat: &cube.ThermostatConfig{ComfortTemp: 21.5, EcoTemp: 17, TempOffset: -0.5, WeekProgram: changed}}
	got := configChanges(old, cur)
	if keys := sortedKeys(got); strings.Join(keys, ",") != "comfort_temp,program_monday,temp_offset" {
		t.Fatalf("changed %v: %+v", keys, got)
	}
	if c := got["comfort_temp"]; c.From != 21.0 || c.To != 21.5 {
		t.Fatalf("comfort %+v", c)
	}
	if c := got["program_monday"]; c.From != nil || c.To != nil {
		t.Fatalf("a program's values: %+v", c)
	}
	if c := configChanges(cur, cur); c != nil {
		t.Fatalf("no difference: %+v", c)
	}
	if c := configChanges(nil, cur); c != nil {
		t.Fatalf("nothing to compare with: %+v", c)
	}
	cubeOld := &cube.DeviceConfig{Type: cube.DeviceCube, Cube: &cube.CubeConfig{PortalEnabled: true, Winter: cube.TimeZoneRule{Name: "CET"}}}
	cubeCur := &cube.DeviceConfig{Type: cube.DeviceCube, Cube: &cube.CubeConfig{PortalEnabled: false, Winter: cube.TimeZoneRule{Name: "CET"}}}
	if keys := sortedKeys(configChanges(cubeOld, cubeCur)); strings.Join(keys, ",") != "portal_enabled" {
		t.Fatalf("the Cube's own: %v", keys)
	}
}

// What the summaries say.
func TestGreetingSummaries(t *testing.T) {
	h := &cube.Hello{FirmwareVersion: "1.1.3", DutyCycle: 3, FreeMemorySlots: 180}
	if got := helloSummary(h); got["firmware"] != "1.1.3" || got["duty_cycle_pct"] != 3 || got["free_memory_slots"] != 180 {
		t.Fatalf("hello %v", got)
	}
	md := &cube.Metadata{Rooms: []cube.RoomMeta{{ID: 1}, {ID: 2}}, Devices: []cube.DeviceMeta{{RFAddress: 1, RoomID: 1}, {RFAddress: 2, RoomID: 2}, {RFAddress: 3, RoomID: 2}}}
	if got := listSummary(md, view.Hidden{1: true}); got["rooms"] != 1 || got["devices"] != 2 {
		t.Fatalf("list %v", got)
	}
	c := &cube.DeviceConfig{Type: cube.DeviceRadiatorThermostat, Thermostat: &cube.ThermostatConfig{ComfortTemp: 21, EcoTemp: 17, TempOffset: -0.5}}
	if got := configSummary(c); got["comfort_temp"] != 21.0 || got["eco_temp"] != 17.0 || got["temp_offset"] != -0.5 {
		t.Fatalf("config %v", got)
	}
}

// greetingWithRoomName returns a greeting with a room's name changed in its M
// line, as another app renames it while the server is not connected.
func greetingWithRoomName(t *testing.T, greeting []cube.Message, id uint8, name string) []cube.Message {
	t.Helper()
	out := append([]cube.Message(nil), greeting...)
	for i, m := range out {
		if m.Type != 'M' {
			continue
		}
		p, err := cube.ParseMetadataPart(m.Data)
		if err != nil {
			t.Fatal(err)
		}
		md, err := cube.ParseMetadata([]cube.MetadataPart{p})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for j := range md.Rooms {
			if md.Rooms[j].ID == id {
				md.Rooms[j].Name, found = name, true
			}
		}
		if !found {
			t.Fatalf("no room %d", id)
		}
		b, err := md.Encode()
		if err != nil {
			t.Fatal(err)
		}
		out[i] = cube.Message{Type: 'M', Data: "00,01," + base64.StdEncoding.EncodeToString(b)}
	}
	return out
}

// newLines reads the log at a level and returns the lines after the n
// it had.
func newLines(t *testing.T, s string, all []event, n int) []event {
	t.Helper()
	if len(all) < n {
		t.Fatalf("%s: %d lines, fewer than the %d before", s, len(all), n)
	}
	return all[n:]
}

// A reconnect after a loss is one connected line at info, its summary
// saying the devices, rooms, firmware, budget and cause; the greeting's
// own lines are at debug, each with its summary.
func TestReconnectAtInfoIsOneLine(t *testing.T) {
	fc, ts, _, _ := startStored(t, Options{})
	eventually(t, "the first connected", func() bool {
		_, evs := logOf(t, ts, "?kind=connected")
		return len(evs) == 1
	})
	_, before := logOf(t, ts, "?limit=1000")
	_, beforeDebug := logOf(t, ts, "?limit=1000&level=debug")
	fc.Drop()
	eventually(t, "the second connected", func() bool {
		_, evs := logOf(t, ts, "?kind=connected")
		return len(evs) == 2
	})
	_, all := logOf(t, ts, "?limit=1000")
	lines := newLines(t, "info", all, len(before))
	var kinds []string
	for _, e := range lines {
		kinds = append(kinds, e.Kind)
	}
	if strings.Join(kinds, ",") != "disconnected,connected" {
		t.Fatalf("the reconnect at info: %v", kinds)
	}
	c := lines[1]
	if c.Summary["cause"] != "loss" || c.Summary["devices"] == nil || c.Summary["rooms"] == nil || c.Summary["firmware"] == nil || c.Summary["duty_cycle_pct"] == nil {
		t.Fatalf("connected's summary: %v", c.Summary)
	}
	if lines[0].Level != "warn" || c.Level != "info" {
		t.Fatalf("levels %s and %s", lines[0].Level, c.Level)
	}
	_, allDebug := logOf(t, ts, "?limit=1000&level=debug")
	greeting := map[string]int{}
	for _, e := range newLines(t, "debug", allDebug, len(beforeDebug)) {
		if e.Level == "debug" {
			greeting[e.Kind]++
			if e.Summary == nil {
				t.Fatalf("a greeting line without its summary: %+v", e)
			}
		}
	}
	if greeting["hello"] != 1 || greeting["metadata"] != 1 || greeting["config"] < 2 {
		t.Fatalf("the greeting's own lines at debug: %v", greeting)
	}
}

// A room renamed while the Cube was disconnected follows the connected
// line at info, as a metadata line that says so.
func TestChangedWhileDisconnected(t *testing.T) {
	fc, ts, _, _ := startStored(t, Options{})
	eventually(t, "the first connected", func() bool {
		_, evs := logOf(t, ts, "?kind=connected")
		return len(evs) == 1
	})
	greeting, _ := fakecube.LoadGreeting(fixture)
	fc.SetGreeting(greetingWithRoomName(t, greeting, 6, "Renamed elsewhere"))
	fc.Drop()
	var meta []event
	eventually(t, "the change after the reconnect", func() bool {
		_, meta = logOf(t, ts, "?kind=metadata")
		return len(meta) == 1
	})
	m := meta[0]
	if !m.Missed || m.Level != "info" || len(m.ListChanges) != 1 || m.ListChanges[0].What != "room_renamed" || m.ListChanges[0].Name != "Renamed elsewhere" || m.ListChanges[0].Room != 6 {
		t.Fatalf("the metadata line: %+v", m)
	}
	_, all := logOf(t, ts, "?limit=1000")
	if last := all[len(all)-1]; last.Kind != "metadata" || all[len(all)-2].Kind != "connected" {
		t.Fatalf("the change follows connected: %s after %s", last.Kind, all[len(all)-2].Kind)
	}
}

// After a restart the reference is the newest backup: a room renamed
// while the server was down shows after the first connected line.
func TestChangedWhileServerDown(t *testing.T) {
	fc, ts, s, st := startStored(t, Options{})
	rt := s.cubes["keq0000100"]
	eventually(t, "the first connected", func() bool {
		_, evs := logOf(t, ts, "?kind=connected")
		return len(evs) == 1
	})
	if _, _, err := s.takeBackup(context.Background(), rt, "manual", 0, ""); err != nil {
		t.Fatal(err)
	}
	// A server started again over the store, with a Cube renamed meanwhile.
	greeting, _ := fakecube.LoadGreeting(fixture)
	fc.SetGreeting(greetingWithRoomName(t, greeting, 6, "Renamed while down"))
	row, _, _ := st.Cube(context.Background(), "KEQ0000100")
	cfg, _ := CubeFromRow(row)
	s2 := New(Options{Cubes: []CubeConfig{cfg}, Store: st, Queue: s.opts.Queue})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s2.Run(ctx)
	rt2 := s2.cubes["keq0000100"]
	deadline := time.Now().Add(5 * time.Second)
	for {
		if evs, err := s2.storedLog(ctx, st, map[int64]string{row.ID: ""}, logQuery{kind: "metadata", limit: 10, minLevel: 0}); err == nil && len(evs) == 1 {
			m := evs[0]
			if !m.Missed || len(m.ListChanges) != 1 || m.ListChanges[0].Name != "Renamed while down" {
				t.Fatalf("the line after the restart: %+v", m)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no metadata line after the restart; connected %v", rt2.client.Connected())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A C line read again that changed nothing is at debug; one that changed
// says what at info.
func TestSameConfigAtDebug(t *testing.T) {
	fc, ts, s, _ := startStored(t, Options{})
	rt := s.cubes["keq0000100"]
	eventually(t, "the first connected", func() bool {
		_, evs := logOf(t, ts, "?kind=connected")
		return len(evs) == 1
	})
	ctx := context.Background()
	rt.writer().Reread(ctx, 0x0a010d)
	eventually(t, "the re-read at debug", func() bool {
		_, evs := logOf(t, ts, "?kind=config&device=0a010d&level=debug")
		return len(evs) >= 2 && evs[len(evs)-1].Level == "debug" && !evs[len(evs)-1].Missed
	})
	if _, evs := logOf(t, ts, "?kind=config&device=0a010d"); len(evs) != 0 {
		t.Fatalf("an unchanged C line at info: %+v", evs)
	}
	c := rt.client.Snapshot()
	line, ok := c.ConfigLine(0x0a010d)
	if !ok {
		t.Fatal("no C line")
	}
	rf, b64, _ := strings.Cut(line.Data, ",")
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	raw[0x12] = 44 // comfort 22.0
	fc.SetConfig(cube.Message{Type: 'C', Data: rf + "," + base64.StdEncoding.EncodeToString(raw)})
	rt.writer().Reread(ctx, 0x0a010d)
	eventually(t, "the change at info", func() bool {
		_, evs := logOf(t, ts, "?kind=config&device=0a010d")
		return len(evs) == 1 && evs[0].Changes["comfort_temp"].To == 22.0
	})
}
