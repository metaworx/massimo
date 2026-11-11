// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package backup

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

const fixture = "../protocol/cube/testdata/cube-fw0113-greeting.txt"

func connect(t *testing.T) (*fakecube.Server, *cubelan.Client, context.Context) {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &fakecube.Server{Greeting: greeting}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := cubelan.New(cubelan.Options{Addr: addr})
	go c.Run(ctx)
	select {
	case <-c.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("client not ready")
	}
	t.Cleanup(func() { cancel(); srv.Close() })
	return srv, c, ctx
}

func take(t *testing.T) (*fakecube.Server, *cubelan.Client, context.Context, *File) {
	t.Helper()
	srv, c, ctx := connect(t)
	f, err := Take(c.Snapshot(), []string{"ntp.homematic.com", "ntp.homematic.com"}, "test", time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatal(err)
	}
	return srv, c, ctx, f
}

func TestTakeMatchesFixture(t *testing.T) {
	_, _, _, f := take(t)
	raw, _ := os.ReadFile(fixture)
	want, _ := cube.SplitLines(string(raw))
	if len(f.Lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(f.Lines), len(want))
	}
	for i := range want {
		if f.Lines[i] != want[i].String() {
			t.Fatalf("line %d:\n got %s\nwant %s", i, f.Lines[i], want[i].String())
		}
	}
	if f.Cube.Serial != "KEQ0000100" || f.Cube.Firmware != "1.1.3" {
		t.Fatalf("cube info %+v", f.Cube)
	}
	if len(f.Rooms) != 6 || len(f.Devices) != 6 || f.CubeConfig == nil {
		t.Fatalf("rooms %d devices %d cube config %v", len(f.Rooms), len(f.Devices), f.CubeConfig != nil)
	}
	d, ok := f.Device(0x0a010d)
	if !ok || d.Name != "Radiator Schlafzimmer" || d.RoomID != 6 || d.Thermostat == nil || d.Thermostat.ComfortTemp != 21.5 {
		t.Fatalf("device %+v", d)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	_, _, _, f := take(t)
	for _, name := range []string{"b.json", "b.yaml"} {
		path := filepath.Join(t.TempDir(), name)
		if err := f.Save(path); err != nil {
			t.Fatal(err)
		}
		g, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(f, g) {
			t.Fatalf("%s: round trip differs:\n%+v\n%+v", name, f, g)
		}
	}
}

// A backup saved over an existing file replaces it whole and keeps its
// mode; nothing else is left beside it.
func TestSaveReplacesWhole(t *testing.T) {
	_, _, _, f := take(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "b.json")
	if err := os.WriteFile(path, []byte("an older, longer backup that is not JSON at all"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := f.Save(path); err != nil {
		t.Fatal(err)
	}
	if g, err := Load(path); err != nil || !reflect.DeepEqual(f, g) {
		t.Fatalf("after the save: %v", err)
	}
	if fi, _ := os.Stat(path); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o640 {
		t.Errorf("mode %v, want the kept 0640", fi.Mode().Perm())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("beside the backup: %v", entries)
	}
}

func TestPlanIsEmptyForIdenticalBackup(t *testing.T) {
	_, c, _, f := take(t)
	p, err := NewPlan(f, Current{Snapshot: c.Snapshot(), NTP: f.NTP}, PlanOptions{Portal: true})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Empty() || len(p.Notes) != 0 {
		t.Fatalf("plan not empty: %+v", p)
	}
}

func TestPlanAndRestore(t *testing.T) {
	srv, c, ctx, f := take(t)
	f.NTP = []string{"pool.ntp.org"}
	f.CubeConfig.Winter = cube.TimeZoneRule{Name: "CET", Month: 10, Weekday: 0, Hour: 3, OffsetSeconds: 3600}
	f.CubeConfig.Summer = cube.TimeZoneRule{Name: "CEST", Month: 3, Weekday: 0, Hour: 2, OffsetSeconds: 7200}
	f.CubeConfig.PortalURL = "http://example.invalid/cube"
	for i := range f.Devices {
		if f.Devices[i].RFAddress == 0x0a010d {
			f.Devices[i].Name = "Heizung Schlafzimmer"
			f.Devices[i].Thermostat.ComfortTemp = 20
			f.Devices[i].Thermostat.BoostMinutes = 10
			f.Devices[i].Thermostat.WeekProgram[cube.Monday] = cube.DayProgram{{Temp: 17, Until: 360}, {Temp: 21, Until: 1440}}
		}
	}
	f.Devices = append(f.Devices, Device{RFAddress: 0x123456, Type: cube.DeviceRadiatorThermostat, Serial: "MEQ0000000", Name: "Neu", RoomID: 1})

	p, err := NewPlan(f, Current{Snapshot: c.Snapshot(), NTP: []string{"ntp.homematic.com", "ntp.homematic.com"}}, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Notes) != 1 || !strings.Contains(p.Notes[0], "123456") {
		t.Fatalf("notes %v", p.Notes)
	}
	var whats []string
	for _, s := range p.Steps {
		whats = append(whats, s.What)
	}
	if len(p.Steps) != 4 {
		t.Fatalf("steps %v", whats)
	}
	if !strings.HasPrefix(whats[0], "set NTP") || !strings.HasPrefix(whats[1], "set time zone rules to CET/CEST (now GMT/BST)") ||
		!strings.Contains(whats[2], `"Radiator Schlafzimmer"→"Heizung Schlafzimmer"`) ||
		!strings.Contains(whats[3], "temperatures") || !strings.Contains(whats[3], "valve") || !strings.Contains(whats[3], "program monday") {
		t.Fatalf("steps %v", whats)
	}
	// The portal URL is only restored on request.
	if strings.Contains(strings.Join(whats, "\n"), "portal") {
		t.Fatalf("portal restored without -portal: %v", whats)
	}
	p2, _ := NewPlan(f, Current{Snapshot: c.Snapshot()}, PlanOptions{Portal: true, SkipClock: true, SkipNames: true, SkipPrograms: true, SkipSettings: true})
	if len(p2.Steps) != 1 || !strings.Contains(p2.Steps[0].What, "portal URL") {
		t.Fatalf("portal plan %+v", p2.Steps)
	}

	q := engine.NewQueue(c, engine.Options{Gap: time.Millisecond})
	go q.Run(ctx)
	res, err := p.Run(ctx, cubeio.Writer{Client: c, Queue: q}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 4 {
		t.Fatalf("results %+v", res)
	}
	var got []string
	for _, m := range srv.Received() {
		if m.Type != 'l' {
			got = append(got, m.String())
		}
	}
	want := []string{"f:pool.ntp.org", "v:", "m:00,", "z:1e,G,06", "s:AARAAAAACgENBmY=", "s:"}
	if len(got) != 7 {
		t.Fatalf("received %v", got)
	}
	for i, w := range []string{"f:pool.ntp.org", "v:", "m:00,", "z:1e,G,06", "s:", "s:", "s:"} {
		if !strings.HasPrefix(got[i], w) {
			t.Fatalf("received %v, want prefixes %v", got, want)
		}
	}
	// Temperatures (0x11): 00 00 11 000000 0a010d 06, then 28 21 3d 09 07 0a 03.
	if got[4] != "s:AAARAAAACgENBighPQkHGAM=" {
		t.Fatalf("temperature command %s", got[4])
	}
	// Valve (0x12): boost 10 min (2<<5) | 80 %/5 = 0x50, decalc sat 12:00 = 0x0c, max 0xff, offset 0.
	if got[5] != "s:AAASAAAACgENBlAM/wA=" {
		t.Fatalf("valve command %s", got[5])
	}
}

// withEmptyRoom adds a room without devices to the greeting's metadata.
func withEmptyRoom(t *testing.T, greeting []cube.Message) []cube.Message {
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
		md.Rooms = append(md.Rooms, cube.RoomMeta{ID: 9, Name: "Leer"})
		cmds, err := cube.WriteMetadataCommands(md)
		if err != nil || len(cmds) != 1 {
			t.Fatal(err)
		}
		_, data, _ := strings.Cut(cmds[0].Data, ",")
		out[i].Data = "00,01," + data
	}
	return out
}

func TestPlanKeepsEmptyRooms(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &fakecube.Server{Greeting: withEmptyRoom(t, greeting)}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); srv.Close() })
	c := cubelan.New(cubelan.Options{Addr: addr})
	go c.Run(ctx)
	<-c.Ready()
	f, err := Take(c.Snapshot(), nil, "test", time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.RoomName(9) != "Leer" {
		t.Fatalf("empty room not in backup: %+v", f.Rooms)
	}
	p, err := NewPlan(f, Current{Snapshot: c.Snapshot()}, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Empty() {
		t.Fatalf("plan not empty: %+v", p.Steps)
	}
}

// withUnlistedDevice adds a C line for a device that is not in the M list.
func withUnlistedDevice(greeting []cube.Message) []cube.Message {
	out := append([]cube.Message(nil), greeting...)
	for _, m := range greeting {
		if m.Type == 'C' && strings.HasPrefix(m.Data, "0a010d,") {
			// Same configuration, different address and serial.
			raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(m.Data, "0a010d,"))
			raw = append([]byte(nil), raw...)
			raw[1], raw[2], raw[3] = 0x0a, 0x01, 0x01
			copy(raw[8:18], "IEQ0000501")
			out = append(out, cube.Message{Type: 'C', Data: "0a0101," + base64.StdEncoding.EncodeToString(raw)})
		}
	}
	// Keep L last.
	for i, m := range out {
		if m.Type == 'L' && i != len(out)-1 {
			out = append(append(out[:i:i], out[i+1:]...), m)
			break
		}
	}
	return out
}

func TestPlanLeavesUnlistedDevicesAlone(t *testing.T) {
	_, c, _, f := take(t)
	_ = c
	greeting, _ := fakecube.LoadGreeting(fixture)
	srv := &fakecube.Server{Greeting: withUnlistedDevice(greeting)}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); srv.Close() })
	c2 := cubelan.New(cubelan.Options{Addr: addr})
	go c2.Run(ctx)
	<-c2.Ready()
	f, err = Take(c2.Snapshot(), nil, "test", time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := f.Device(0x0a0101)
	if !ok || !d.Unlisted || d.Name != "" || d.Serial != "IEQ0000501" {
		t.Fatalf("unlisted device %+v ok=%v", d, ok)
	}
	p, err := NewPlan(f, Current{Snapshot: c2.Snapshot()}, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Empty() {
		t.Fatalf("plan not empty: %+v", p.Steps)
	}
	// Naming it in the backup lists it.
	for i := range f.Devices {
		if f.Devices[i].RFAddress == 0x0a0101 {
			f.Devices[i].Name, f.Devices[i].RoomID = "Neu", 6
		}
	}
	p, _ = NewPlan(f, Current{Snapshot: c2.Snapshot()}, PlanOptions{})
	if len(p.Steps) != 1 || !strings.Contains(p.Steps[0].What, `0a0101: add to the list as "Neu" in room 6`) {
		t.Fatalf("steps %+v", p.Steps)
	}
}

// withLiveState gives the fixture's L entries a valid manual state.
func withLiveState(t *testing.T, greeting []cube.Message, rf cube.RFAddress, setpoint float64) []cube.Message {
	t.Helper()
	out := append([]cube.Message(nil), greeting...)
	for i, m := range out {
		if m.Type != 'L' {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(m.Data)
		if err != nil {
			t.Fatal(err)
		}
		for off := 0; off < len(b); off += 1 + int(b[off]) {
			if cube.RFAddressFromBytes(b[off+1:off+4]) == rf {
				b[off+5] = 0x12 // valid, initialized
				b[off+6] = 0x19 // manual, DST
				b[off+8] = byte(setpoint * 2)
			}
		}
		out[i].Data = base64.StdEncoding.EncodeToString(b)
	}
	return out
}

func TestBackupStateAndRestoreState(t *testing.T) {
	greeting, _ := fakecube.LoadGreeting(fixture)
	srv := &fakecube.Server{Greeting: withLiveState(t, greeting, 0x0a010d, 20)}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); srv.Close() })
	c := cubelan.New(cubelan.Options{Addr: addr})
	go c.Run(ctx)
	<-c.Ready()
	f, err := Take(c.Snapshot(), nil, "test", time.Date(2026, 11, 4, 14, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := f.Device(0x0a010d)
	if d.State == nil || d.State.Mode != cube.ModeManual || d.State.Setpoint == nil || *d.State.Setpoint != 20 {
		t.Fatalf("state %+v", d.State)
	}
	path := filepath.Join(t.TempDir(), "s.yaml")
	if err := f.Save(path); err != nil {
		t.Fatal(err)
	}
	if g, err := Load(path); err != nil || !reflect.DeepEqual(f, g) {
		t.Fatalf("round trip: %v", err)
	}
	// Same state → nothing; a different setpoint → one s: command.
	p, _ := NewPlan(f, Current{Snapshot: c.Snapshot()}, PlanOptions{State: true})
	if !p.Empty() {
		t.Fatalf("plan %+v", p.Steps)
	}
	for i := range f.Devices {
		if f.Devices[i].RFAddress == 0x0a010d {
			v := 21.5
			f.Devices[i].State.Setpoint = &v
		}
	}
	p, _ = NewPlan(f, Current{Snapshot: c.Snapshot()}, PlanOptions{State: true})
	if len(p.Steps) != 1 || !strings.Contains(p.Steps[0].What, "manual 21.5°C (now manual 20.0°C)") || p.Steps[0].Command.Wakeup {
		t.Fatalf("plan %+v", p.Steps)
	}
	// 00 04 40 000000 0a010d 06 6b: set temperature, manual (0x40) | 21.5 °C (43), addressed as room 6.
	if got := p.Steps[0].Command.Lines[0].String(); got != "s:AARAAAAACgENBms=" {
		t.Fatalf("command %s", got)
	}
	// Without -state the plan ignores the state section.
	p, _ = NewPlan(f, Current{Snapshot: c.Snapshot()}, PlanOptions{})
	if !p.Empty() {
		t.Fatalf("plan without State %+v", p.Steps)
	}
}

func TestPlanMovesDeviceBetweenRooms(t *testing.T) {
	_, c, _, f := take(t)
	// Move the Schlafzimmer thermostat (room 6, alone) into Küche (room 3,
	// with radiator thermostat 0a0108).
	for i := range f.Devices {
		if f.Devices[i].RFAddress == 0x0a010d {
			f.Devices[i].RoomID = 3
		}
	}
	p, err := NewPlan(f, Current{Snapshot: c.Snapshot()}, PlanOptions{SkipPrograms: true})
	if err != nil {
		t.Fatal(err)
	}
	var whats []string
	for _, s := range p.Steps {
		whats = append(whats, s.What)
	}
	if len(p.Steps) != 3 || !strings.Contains(whats[0], "room 6→3") || !strings.Contains(whats[1], "leave room 6 (0 partners)") || !strings.Contains(whats[2], "join room 3 (1 partners)") {
		t.Fatalf("steps %v", whats)
	}
	out := p.Steps[1].Command
	if out.RoomID != 6 || !out.Wakeup || len(out.Lines) != 1 || out.Lines[0].String() != "s:AAAjAAAACgENAAY=" {
		t.Fatalf("leave %+v", out)
	}
	in := p.Steps[2].Command
	if in.RoomID != 3 || !in.Wakeup || len(in.Lines) != 3 {
		t.Fatalf("join %+v", in)
	}
	// 00 04 20 000000 0a0108 03 0a010d 01: tell the room's radiator thermostat (room flag) about the new partner.
	// 00 00 20 000000 0a010d 00 0a0108 01: tell the moved device about it.
	// 00 00 22 000000 0a010d 00 03: set the group.
	want := []string{"s:AAQgAAAACgEIAwoBDQE=", "s:AAAgAAAACgENAAoBCAE=", "s:AAAiAAAACgENAAM="}
	for i, w := range want {
		if in.Lines[i].String() != w {
			t.Fatalf("join line %d: %s, want %s", i, in.Lines[i].String(), w)
		}
	}
}

// The configuration hash ignores the time, the tool, the NTP servers,
// the raw lines and the live state, and changes with a name or a
// setting.
func TestConfigHash(t *testing.T) {
	_, _, _, f := take(t)
	copyOf := func() *File {
		t.Helper()
		data, err := f.Marshal(false)
		if err != nil {
			t.Fatal(err)
		}
		g, err := Unmarshal(data, false)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	base := ConfigHash(f)
	g := copyOf()
	g.TakenAt, g.Tool, g.NTP, g.Lines = g.TakenAt.Add(time.Hour), "other", []string{"ntp.example.org"}, nil
	for i := range g.Devices {
		g.Devices[i].State = nil
	}
	if ConfigHash(g) != base {
		t.Fatal("the hash follows the time, the tool, the NTP servers, the lines or the state")
	}
	g = copyOf()
	g.Rooms[0].Name = "Renamed"
	if ConfigHash(g) == base {
		t.Fatal("a room's name does not change the hash")
	}
	g = copyOf()
	for i := range g.Devices {
		if g.Devices[i].Thermostat != nil {
			g.Devices[i].Thermostat.EcoTemp += 0.5
			break
		}
	}
	if ConfigHash(g) == base {
		t.Fatal("a setting does not change the hash")
	}
}
