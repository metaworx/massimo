// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package view

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

var t0 = time.Date(2026, 11, 13, 8, 0, 0, 0, time.UTC)

// thermo is a reported thermostat's view, its values received at at.
func thermo(rf string, typ cube.DeviceType, mode cube.Mode, setpoint, actual float64, at time.Time) Device {
	sp := setpoint
	st := &State{Mode: mode, Setpoint: &sp}
	rec := map[string]time.Time{"mode": at, "setpoint": at}
	if actual != 0 {
		a := actual
		st.ActualTemp = &a
		rec["actual_temp"] = at
	}
	if typ.IsRadiatorThermostat() {
		v := 0
		st.ValvePct = &v
		rec["valve_pct"] = at
	}
	seen := at
	return Device{RFAddress: rf, Type: typ, Reported: true, State: st, Received: rec, LastSeen: &seen}
}

func contact(rf string, open bool) Device {
	o := open
	return Device{RFAddress: rf, Type: cube.DeviceShutterContact, Reported: true, State: &State{WindowOpen: &o}, Received: map[string]time.Time{"window_open": t0}}
}

var room = cube.RoomMeta{ID: 5, Name: "Wohnen"}

// The rules of the virtual wall thermostat's state, room by room.
func TestRoomThermostatState(t *testing.T) {
	wall := thermo("0a0201", cube.DeviceWallThermostat, cube.ModeAuto, 21, 21.4, t0)
	r1 := thermo("0a0202", cube.DeviceRadiatorThermostat, cube.ModeAuto, 21, 20.5, t0.Add(time.Hour))
	r2 := thermo("0a0203", cube.DeviceRadiatorThermostat, cube.ModeAuto, 21, 22, t0.Add(-time.Hour))
	r3 := thermo("0a0204", cube.DeviceRadiatorThermostat, cube.ModeAuto, 21, 23.5, t0.Add(2*time.Hour))

	d := RoomThermostat(room, []Device{wall, r1, r2, contact("0a0205", true)}, RoomInfo{UID: "u5"}, 5)
	if d.RFAddress != "000005" || d.Serial != "VWT0000005" || !d.Virtual || d.Type != cube.DeviceWallThermostat || d.Name != "Wohnen" || d.RoomUID != "u5" || !d.Reported {
		t.Fatalf("identity %+v", d)
	}
	if *d.State.ActualTemp != 21.4 || strings.Join(d.Sources["actual_temp"], ",") != "0a0201" || d.Received["actual_temp"] != t0 {
		t.Errorf("with a wall thermostat: %v from %v at %v", *d.State.ActualTemp, d.Sources, d.Received["actual_temp"])
	}
	if !*d.State.WindowOpen || !*d.State.Consistent || d.Sources["mode"][0] != "0a0201" {
		t.Errorf("window, consistency, mode source: %+v %v", d.State, d.Sources)
	}

	// Radiator thermostats only: the median, or the mean; of two, the mean.
	d = RoomThermostat(room, []Device{r1, r2, r3}, RoomInfo{}, 5)
	if *d.State.ActualTemp != 22 || d.Received["actual_temp"] != t0.Add(-time.Hour) {
		t.Errorf("median %v at %v", *d.State.ActualTemp, d.Received["actual_temp"])
	}
	if d = RoomThermostat(room, []Device{r1, r2, r3}, RoomInfo{TempMethod: TempMean}, 5); *d.State.ActualTemp != 22 {
		t.Errorf("mean %v", *d.State.ActualTemp)
	}
	if d = RoomThermostat(room, []Device{r1, r3}, RoomInfo{}, 5); *d.State.ActualTemp != 22 {
		t.Errorf("median of two %v", *d.State.ActualTemp)
	}
	// The mode of the radiator thermostat that reported last.
	r3m := r3
	r3m.State = &State{Mode: cube.ModeManual, Setpoint: r3.State.Setpoint, ActualTemp: r3.State.ActualTemp}
	d = RoomThermostat(room, []Device{r1, r2, r3m}, RoomInfo{}, 5)
	if d.State.Mode != cube.ModeManual || d.Sources["mode"][0] != "0a0204" || *d.State.Consistent {
		t.Errorf("latest radiator's mode: %+v %v", d.State, d.Sources)
	}

	// Stale values are left out while a fresh one exists; all stale, all used.
	s2 := r2
	s2.Stale = true
	if d = RoomThermostat(room, []Device{r1, s2, r3}, RoomInfo{}, 5); *d.State.ActualTemp != 22 || d.Stale || len(d.Sources["actual_temp"]) != 2 {
		t.Errorf("one stale: %v from %v", *d.State.ActualTemp, d.Sources)
	}
	s1 := r1
	s1.Stale = true
	if d = RoomThermostat(room, []Device{s1, s2}, RoomInfo{}, 5); *d.State.ActualTemp != 21.3 || !d.Stale {
		t.Errorf("all stale: %v stale %v", *d.State.ActualTemp, d.Stale)
	}

	// A boost at one thermostat: the room boosts, and is not consistent.
	b := r2
	b.State = &State{Mode: cube.ModeBoost, Setpoint: r2.State.Setpoint, ActualTemp: r2.State.ActualTemp}
	d = RoomThermostat(room, []Device{wall, r1, b}, RoomInfo{}, 5)
	if d.State.Mode != cube.ModeBoost || *d.State.Consistent || strings.Join(d.Sources["mode"], ",") != "0a0203" || *d.State.Setpoint != 21 {
		t.Errorf("boost: %+v %v", d.State, d.Sources)
	}

	// The real wall thermostat's flags; the valve is the highest.
	w := wall
	w.State = &State{Mode: cube.ModeAuto, Setpoint: wall.State.Setpoint, ActualTemp: wall.State.ActualTemp, BatteryLow: true, PanelLocked: true}
	v := 40
	o := r1
	o.State = &State{Mode: cube.ModeAuto, Setpoint: r1.State.Setpoint, ValvePct: &v, BatteryLow: true}
	d = RoomThermostat(room, []Device{w, o}, RoomInfo{}, 5)
	if !d.State.BatteryLow || !d.State.PanelLocked || *d.State.ValvePct != 40 {
		t.Errorf("flags and valve: %+v", d.State)
	}
	if d = RoomThermostat(room, []Device{o}, RoomInfo{}, 5); d.State.BatteryLow {
		t.Errorf("a radiator thermostat's battery on the room: %+v", d.State)
	}

	// A contact alone: the window, nothing else, not reported.
	d = RoomThermostat(room, []Device{contact("0a0205", false)}, RoomInfo{}, 5)
	raw, _ := json.Marshal(d.State)
	if d.Reported || string(raw) != `{"window_open":false}` {
		t.Errorf("contact alone: reported %v, state %s", d.Reported, raw)
	}
	if d = RoomThermostat(room, nil, RoomInfo{}, 5); d.State != nil || d.Reported {
		t.Errorf("empty room %+v", d)
	}
	// A real device's state still marshals as before.
	raw, _ = json.Marshal(r1.State)
	if !strings.Contains(string(raw), `"mode":"auto"`) {
		t.Errorf("a real state %s", raw)
	}
}

// The four views of the rooms fixture, and the addresses of the virtual
// wall thermostats.
func TestViews(t *testing.T) {
	lines, err := fakecube.RoomsFixture()
	if err != nil {
		t.Fatal(err)
	}
	snap, err := cubelan.SnapshotFromLines(lines)
	if err != nil {
		t.Fatal(err)
	}
	info := map[uint8]RoomInfo{1: {UID: "u1"}, 2: {UID: "u2", TempMethod: TempMean}, 3: {UID: "u3"}}
	count := func(kind Kind) (real, virtual, walls int) {
		for _, r := range RoomsOf(snap, nil, 0, kind, info) {
			for _, d := range r.Devices {
				switch {
				case d.Virtual:
					virtual++
				case d.Type == cube.DeviceWallThermostat:
					walls++
					real++
				default:
					real++
				}
			}
		}
		return
	}
	for kind, want := range map[Kind][3]int{KindDefault: {7, 2, 1}, KindReal: {7, 0, 1}, KindVirtual: {6, 3, 0}, KindAll: {7, 3, 1}} {
		if r, v, w := count(kind); [3]int{r, v, w} != want {
			t.Errorf("%s: real %d, virtual %d, walls %d; want %v", kind, r, v, w, want)
		}
		if n := len(DevicesOf(snap, nil, 0, kind, info)); n != want[0]+want[1] {
			t.Errorf("%s: %d devices", kind, n)
		}
	}
	rooms := RoomsOf(snap, nil, 0, KindAll, info)
	if rooms[0].UID != "u1" || !rooms[0].Devices[0].Virtual {
		t.Fatalf("room 1 %+v", rooms[0])
	}
	if d := rooms[0].Devices[0]; *d.State.ActualTemp != 21.4 || d.RFAddress != "000001" {
		t.Errorf("room 1's: %v at %s", *d.State.ActualTemp, d.RFAddress)
	}
	if d := rooms[1].Devices[0]; *d.State.ActualTemp != 20.7 {
		t.Errorf("room 2's mean: %v", *d.State.ActualTemp)
	}
	if d := rooms[2].Devices[0]; d.Reported || d.State == nil || *d.State.WindowOpen {
		t.Errorf("room 3's: %+v", d)
	}

	// An owner's address, a taken one, and the room number taken.
	taken := cube.RFAddress(0x0a0201)
	addrs := VirtualAddresses(snap, snap.Metadata().Rooms, map[uint8]RoomInfo{1: {RFAddress: 0x000150}, 2: {RFAddress: taken}})
	if addrs[1] != 0x000150 || addrs[2] != 0x000002 || addrs[3] != 0x000003 {
		t.Errorf("addresses %v", addrs)
	}
	lines2 := append([]cube.Message(nil), lines...)
	for i, m := range lines2 {
		if m.Type == 'H' {
			// The Cube's own address is room 3's number.
			lines2[i].Data = strings.Replace(m.Data, ",0a0107,", ",000003,", 1)
		}
	}
	snap2, err := cubelan.SnapshotFromLines(lines2)
	if err != nil {
		t.Fatal(err)
	}
	if a := VirtualAddresses(snap2, snap2.Metadata().Rooms, nil); a[3] != 0x000100 {
		t.Errorf("room 3 with its number taken: %v", a)
	}
	for in, want := range map[string]Kind{"": KindDefault, "all": KindAll, "virtual": KindVirtual} {
		if k, err := ParseKind(in); err != nil || k != want {
			t.Errorf("ParseKind(%q) = %v %v", in, k, err)
		}
	}
	if _, err := ParseKind("some"); err == nil {
		t.Error("ParseKind took some")
	}
}
