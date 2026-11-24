// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package view

import (
	"encoding/base64"
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
	// The mode most of the radiator thermostats show, not that of the one
	// that reported last (RoomMajority.D19-A).
	r3m := r3
	r3m.State = &State{Mode: cube.ModeManual, Setpoint: r3.State.Setpoint, ActualTemp: r3.State.ActualTemp}
	d = RoomThermostat(room, []Device{r1, r2, r3m}, RoomInfo{}, 5)
	if d.State.Mode != cube.ModeAuto || strings.Join(d.Sources["mode"], ",") != "0a0202,0a0203" || *d.State.Consistent {
		t.Errorf("most radiators' mode: %+v %v", d.State, d.Sources)
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
	timeInfo, link := cube.RadiogramTimeInformation, cube.RadiogramRemoveLinkPartner
	w.State = &State{Mode: cube.ModeAuto, Setpoint: wall.State.Setpoint, ActualTemp: wall.State.ActualTemp, BatteryLow: true, PanelLocked: true,
		Error: true, FailedRadiogram: &timeInfo}
	v := 40
	o := r1
	o.State = &State{Mode: cube.ModeAuto, Setpoint: r1.State.Setpoint, ValvePct: &v, BatteryLow: true, Error: true, FailedRadiogram: &link}
	d = RoomThermostat(room, []Device{w, o}, RoomInfo{}, 5)
	if !d.State.BatteryLow || !d.State.PanelLocked || *d.State.ValvePct != 40 || !d.State.Error || d.State.FailedRadiogram == nil ||
		*d.State.FailedRadiogram != timeInfo {
		t.Errorf("flags and valve: %+v", d.State)
	}
	if d = RoomThermostat(room, []Device{o}, RoomInfo{}, 5); d.State.BatteryLow || d.State.Error || d.State.FailedRadiogram != nil {
		t.Errorf("a radiator thermostat's flags on the room: %+v", d.State)
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

// The virtual wall thermostat's mode, setpoint and until by the one
// order (RoomMajority.D9, D19-A; roomorder.State), boost aside: what
// more than half of the room's thermostats with a state show, those on
// their own programmes as one; else the one whose mode or setpoint
// changed last; else the room's master, its wall thermostat, its first
// radiator thermostat; a dead thermostat (no state, its last report
// kept) passed over. sources.mode names those the state came from, the
// boosting ones during a boost, whose setpoint and until are those of
// the thermostats that do not boost.
func TestRoomThermostatByTheOneOrder(t *testing.T) {
	at := func(m int) time.Time { return t0.Add(time.Duration(m) * time.Minute) }
	until := cube.Until{Year: 2026, Month: 12, Day: 24, Minutes: 18 * 60}
	// th is a reported thermostat at mode and setpoint, its mode and its
	// setpoint changed by hand at changed minutes after t0 (0: not known),
	// in auto following its programme where follows says so.
	th := func(rf string, typ cube.DeviceType, mode cube.Mode, setpoint float64, changed int, follows bool) Device {
		d := thermo(rf, typ, mode, setpoint, 20, t0)
		if mode == cube.ModeVacation {
			d.State.Until = &until
		}
		if changed != 0 {
			d.modeAt, d.setpointAt = at(changed), at(changed)
		}
		d.follows = follows
		return d
	}
	const wallT, radA, radB, radC, radD = "0a0201", "0a0202", "0a0203", "0a0204", "0a0205"
	wallRoom := cube.RoomMeta{ID: 5, Name: "Wohnen", RFAddress: 0x0a0201}
	radRoom := cube.RoomMeta{ID: 5, Name: "Wohnen", RFAddress: 0x0a0203}
	dead := func(d Device) Device {
		d.LastKnown, d.State, d.Reported, d.OutOfDate = d.State, nil, false, true
		return d
	}
	for _, c := range []struct {
		name     string
		room     cube.RoomMeta
		members  []Device
		mode     cube.Mode
		setpoint float64
		until    bool
		from     string // sources.mode
		setFrom  string // sources.setpoint, from where it differs
	}{
		{"most, the wall thermostat and the master the other", wallRoom, []Device{th(wallT, cube.DeviceWallThermostat, cube.ModeManual, 23, 9, false),
			th(radA, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, 0, false), th(radB, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, 0, false)},
			cube.ModeManual, 18, false, radA + "," + radB, ""},
		{"those on their own programmes as one", radRoom, []Device{th(wallT, cube.DeviceWallThermostat, cube.ModeAuto, 21, 0, true),
			th(radA, cube.DeviceRadiatorThermostat, cube.ModeAuto, 19, 0, true), th(radB, cube.DeviceRadiatorThermostat, cube.ModeManual, 21, 0, false)},
			cube.ModeAuto, 21, false, wallT + "," + radA, ""},
		{"no majority: the newest change", wallRoom, []Device{th(wallT, cube.DeviceWallThermostat, cube.ModeManual, 23, 1, false),
			th(radA, cube.DeviceRadiatorThermostat, cube.ModeVacation, 15, 5, false), th(radB, cube.DeviceRadiatorThermostat, cube.ModeAuto, 20, 0, false)},
			cube.ModeVacation, 15, true, radA, ""},
		{"no time: the master, a radiator thermostat", radRoom, []Device{th(wallT, cube.DeviceWallThermostat, cube.ModeManual, 23, 0, false),
			th(radA, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, 0, false), th(radB, cube.DeviceRadiatorThermostat, cube.ModeManual, 19, 0, false)},
			cube.ModeManual, 19, false, radB, ""},
		{"no master: the wall thermostat", room, []Device{th(radA, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, 0, false),
			th(wallT, cube.DeviceWallThermostat, cube.ModeManual, 23, 0, false)}, cube.ModeManual, 23, false, wallT, ""},
		{"no wall thermostat: the first radiator thermostat", room, []Device{th(radB, cube.DeviceRadiatorThermostat, cube.ModeManual, 19, 0, false),
			th(radA, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, 0, false)}, cube.ModeManual, 19, false, radB, ""},
		{"a dead master passed over", radRoom, []Device{th(radA, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, 0, false),
			dead(th(radB, cube.DeviceRadiatorThermostat, cube.ModeManual, 5, 0, false)), th(radC, cube.DeviceRadiatorThermostat, cube.ModeManual, 22, 0, false)},
			cube.ModeManual, 18, false, radA, ""},
		{"a dead wall thermostat passed over, the newest change its own", wallRoom, []Device{dead(th(wallT, cube.DeviceWallThermostat, cube.ModeManual, 5, 9, false)),
			th(radA, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, 2, false), th(radB, cube.DeviceRadiatorThermostat, cube.ModeManual, 22, 1, false)},
			cube.ModeManual, 18, false, radA, ""},
		{"a boost: its setpoint that of those not boosting", wallRoom, []Device{th(wallT, cube.DeviceWallThermostat, cube.ModeManual, 23, 0, false),
			th(radA, cube.DeviceRadiatorThermostat, cube.ModeManual, 23, 0, false), th(radB, cube.DeviceRadiatorThermostat, cube.ModeBoost, 21, 0, false)},
			cube.ModeBoost, 23, false, radB, wallT + "," + radA},
		// Majority passes over B, whom fewer than half agree with, and
		// takes A, whom B and C agree with, C on its programme as A is:
		// A's state, A first.
		{"the one picked first among those its state came from", room, []Device{th(radB, cube.DeviceRadiatorThermostat, cube.ModeAuto, 21, 0, false),
			th(radA, cube.DeviceRadiatorThermostat, cube.ModeAuto, 21, 0, true), th(radC, cube.DeviceRadiatorThermostat, cube.ModeAuto, 19, 0, true),
			th(radD, cube.DeviceRadiatorThermostat, cube.ModeManual, 18, 0, false)},
			cube.ModeAuto, 21, false, radA + "," + radB + "," + radC, ""},
	} {
		if c.setFrom == "" {
			c.setFrom = c.from
		}
		d := RoomThermostat(c.room, c.members, RoomInfo{}, 5)
		if d.State == nil || d.State.Mode != c.mode || d.State.Setpoint == nil || *d.State.Setpoint != c.setpoint || (d.State.Until != nil) != c.until ||
			strings.Join(d.Sources["mode"], ",") != c.from || strings.Join(d.Sources["setpoint"], ",") != c.setFrom {
			t.Errorf("%s: %+v, its mode from %v, its setpoint from %v; want %s %v from %s and %s", c.name, d.State, d.Sources["mode"], d.Sources["setpoint"],
				c.mode, c.setpoint, c.from, c.setFrom)
		}
	}
}

// withEntries is the rooms fixture's greeting with its L line's entries
// changed by edit, each by its address: the flags (mode in the low bits
// of byte 6) and the setpoint (byte 8, in half degrees).
func withEntries(t *testing.T, lines []cube.Message, edit func(rf cube.RFAddress, e []byte)) cube.Message {
	t.Helper()
	for _, m := range lines {
		if m.Type != 'L' {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(m.Data)
		if err != nil {
			t.Fatal(err)
		}
		for j := 0; j < len(raw); j += int(raw[j]) + 1 {
			edit(cube.RFAddressFromBytes(raw[j+1:j+4]), raw[j:j+int(raw[j])+1])
		}
		return cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)}
	}
	t.Fatal("no L line")
	return cube.Message{}
}

// The facts of a thermostat that the room's state is decided by, as
// DeviceOf reads them from the snapshot, and the virtual wall thermostat
// of room 2 by them (RoomMajority.D19-A): at the Cube's 20:13, two
// minutes before the radiator thermostats' programme goes from 21 °C to
// 17 °C, 0a0204 in auto at 21 °C and 0a0205 at 17 °C each follow it, and
// count as one state against 0a0206's manual 21 °C; once 0a0205 is set to
// manual 19 °C, its change is the newest, though 0a0206's valve reported
// later, and the room entry names it; once 0a0206 boosts, the room
// boosts at 0a0205's setpoint, which the entry still names.
func TestRoomThermostatByTheSnapshot(t *testing.T) {
	lines, err := fakecube.RoomsFixture()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 3, 19, 51, 0, 0, time.Local) // the fixture's Cube's clock, Tuesday
	setting := func(e []byte, mode cube.Mode, setpoint float64) {
		e[6] = e[6]&^0x03 | byte(mode)
		e[8] = e[8]&0xc0 | byte(setpoint*2)
	}
	first := withEntries(t, lines, func(rf cube.RFAddress, e []byte) {
		switch rf {
		case 0x0a0204:
			setting(e, cube.ModeAuto, 21)
		case 0x0a0205:
			setting(e, cube.ModeAuto, 17)
		case 0x0a0206:
			setting(e, cube.ModeManual, 21)
		}
	})
	for i := range lines {
		if lines[i].Type == 'L' {
			lines[i] = first
		}
	}
	snap, err := cubelan.SnapshotOf(lines, at)
	if err != nil {
		t.Fatal(err)
	}
	was := now
	now = func() time.Time { return at.Add(22 * time.Minute) } // the Cube's 20:13
	t.Cleanup(func() { now = was })
	// room 2 as RoomsOf gives it, its virtual wall thermostat first, then
	// its devices by their addresses
	room2 := func() (Device, map[string]Device) {
		t.Helper()
		for _, r := range RoomsOf(snap, nil, Staleness{}, KindAll, nil) {
			if r.ID == 2 {
				byRF := map[string]Device{}
				for _, d := range r.Devices[1:] {
					byRF[d.RFAddress] = d
				}
				return r.Devices[0], byRF
			}
		}
		t.Fatal("no room 2")
		return Device{}, nil
	}
	vwt := func() Device { d, _ := room2(); return d }
	if _, ds := room2(); !ds["0a0204"].follows || !ds["0a0205"].follows || ds["0a0206"].follows ||
		!ds["0a0204"].modeAt.IsZero() || !ds["0a0205"].setpointAt.IsZero() {
		t.Fatalf("the facts at the first sighting: %+v", ds)
	}
	if d := vwt(); d.State.Mode != cube.ModeAuto || *d.State.Setpoint != 21 || strings.Join(d.Sources["mode"], ",") != "0a0204,0a0205" {
		t.Errorf("two on their programmes as one: %s %v from %v", d.State.Mode, *d.State.Setpoint, d.Sources["mode"])
	}

	later := withEntries(t, []cube.Message{first}, func(rf cube.RFAddress, e []byte) {
		if rf == 0x0a0205 {
			setting(e, cube.ModeManual, 19)
		}
	})
	if err := snap.Apply(later, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	valve := withEntries(t, []cube.Message{later}, func(rf cube.RFAddress, e []byte) {
		if rf == 0x0a0206 {
			e[7] = 40
		}
	})
	if err := snap.Apply(valve, at.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, ds := room2(); !ds["0a0205"].modeAt.Equal(at.Add(time.Minute)) || !ds["0a0205"].setpointAt.Equal(at.Add(time.Minute)) || ds["0a0205"].follows {
		t.Fatalf("0a0205 set by hand: %+v", ds["0a0205"])
	}
	if d := vwt(); d.State.Mode != cube.ModeManual || *d.State.Setpoint != 19 || strings.Join(d.Sources["mode"], ",") != "0a0205" {
		t.Errorf("the newest change: %s %v from %v", d.State.Mode, *d.State.Setpoint, d.Sources["mode"])
	}
	entry := func() string {
		for _, r := range RoomsOf(snap, nil, Staleness{}, KindAll, nil) {
			if r.ID == 2 {
				return r.Thermostat
			}
		}
		return ""
	}
	if got := entry(); got != "0a0205" {
		t.Errorf("the room entry names %q, want the newest change's 0a0205", got)
	}

	// 0a0206 boosts: the room boosts, at 0a0205's setpoint still, which
	// its entry names, sources.mode naming the boosting one.
	boost := withEntries(t, []cube.Message{valve}, func(rf cube.RFAddress, e []byte) {
		if rf == 0x0a0206 {
			setting(e, cube.ModeBoost, 21)
		}
	})
	if err := snap.Apply(boost, at.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if d := vwt(); d.State.Mode != cube.ModeBoost || *d.State.Setpoint != 19 || strings.Join(d.Sources["mode"], ",") != "0a0206" ||
		strings.Join(d.Sources["setpoint"], ",") != "0a0205" || entry() != "0a0205" {
		t.Errorf("a boost: %s %v, its mode from %v, its setpoint from %v, the entry %q", d.State.Mode, *d.State.Setpoint, d.Sources["mode"],
			d.Sources["setpoint"], entry())
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
		for _, r := range RoomsOf(snap, nil, Staleness{}, kind, info) {
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
		if n := len(DevicesOf(snap, nil, Staleness{}, kind, info)); n != want[0]+want[1] {
			t.Errorf("%s: %d devices", kind, n)
		}
	}
	rooms := RoomsOf(snap, nil, Staleness{}, KindAll, info)
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
