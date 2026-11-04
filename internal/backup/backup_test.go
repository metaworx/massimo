// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package backup

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
	f, err := Take(c.Snapshot(), []string{"ntp.homematic.com", "ntp.homematic.com"}, "test", time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
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
	if f.Cube.Serial != "KEQ0537249" || f.Cube.Firmware != "1.1.3" {
		t.Fatalf("cube info %+v", f.Cube)
	}
	if len(f.Rooms) != 6 || len(f.Devices) != 6 || f.CubeConfig == nil {
		t.Fatalf("rooms %d devices %d cube config %v", len(f.Rooms), len(f.Devices), f.CubeConfig != nil)
	}
	d, ok := f.Device(0x1514b6)
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
		if f.Devices[i].RFAddress == 0x1514b6 {
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
	res, err := p.Run(ctx, q, nil)
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
	want := []string{"f:pool.ntp.org", "v:", "m:00,", "z:1e,G,06", "s:AARAAAAAFRS2BmY=", "s:"}
	if len(got) != 7 {
		t.Fatalf("received %v", got)
	}
	for i, w := range []string{"f:pool.ntp.org", "v:", "m:00,", "z:1e,G,06", "s:", "s:", "s:"} {
		if !strings.HasPrefix(got[i], w) {
			t.Fatalf("received %v, want prefixes %v", got, want)
		}
	}
	// Temperatures (0x11): 00 00 11 000000 1514b6 06, then 28 21 3d 09 07 0a 03.
	if got[4] != "s:AAARAAAAFRS2BighPQkHGAM=" {
		t.Fatalf("temperature command %s", got[4])
	}
	// Valve (0x12): boost 10 min (2<<5) | 80 %/5 = 0x50, decalc sat 12:00 = 0x0c, max 0xff, offset 0.
	if got[5] != "s:AAASAAAAFRS2BlAM/wA=" {
		t.Fatalf("valve command %s", got[5])
	}
}
