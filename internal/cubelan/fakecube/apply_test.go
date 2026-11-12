// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// startRooms serves the rooms fixture with writes applied.
func startRooms(t *testing.T, s *Server) *Server {
	t.Helper()
	greeting, err := RoomsFixture()
	if err != nil {
		t.Fatal(err)
	}
	s.Greeting = greeting
	s.Apply = true
	if _, err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// states is the fake's device list as a client reads it, by address.
func (c *client) states() map[cube.RFAddress]cube.DeviceState {
	c.t.Helper()
	types := map[cube.RFAddress]cube.DeviceType{}
	for _, m := range c.greet {
		if m.Type == 'C' {
			if cfg, err := cube.ParseConfig(m.Data); err == nil {
				types[cfg.RFAddress] = cfg.Type
			}
		}
	}
	l := c.send(cube.Message{Type: 'l'})
	list, err := cube.ParseDeviceList(l.Data, types)
	if err != nil {
		c.t.Fatal(err)
	}
	out := map[cube.RFAddress]cube.DeviceState{}
	for _, s := range list {
		out[s.RFAddress] = s
	}
	return out
}

// config is a device's configuration as c: answers it.
func (c *client) config(rf cube.RFAddress) *cube.DeviceConfig {
	c.t.Helper()
	m := c.send(cube.Message{Type: 'c', Data: rf.String()})
	cfg, err := cube.ParseConfig(m.Data)
	if err != nil {
		c.t.Fatalf("c:%s: %v (%s)", rf, err, m)
	}
	return cfg
}

// must is a command the test built without an error.
func must(m cube.Message, err error) cube.Message {
	if err != nil {
		panic(err)
	}
	return m
}

// The rooms fixture holds the house the room thermostat's tests need.
func TestRoomsFixture(t *testing.T) {
	s := startRooms(t, &Server{})
	c := dial(t, s.ln.Addr().String())
	var md *cube.Metadata
	for _, m := range c.greet {
		if m.Type == 'M' {
			part, err := cube.ParseMetadataPart(m.Data)
			if err != nil {
				t.Fatal(err)
			}
			if md, err = cube.ParseMetadata([]cube.MetadataPart{part}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if md == nil || len(md.Rooms) != 3 || len(md.Devices) != 7 {
		t.Fatalf("metadata %+v", md)
	}
	st := c.states()
	want := map[cube.RFAddress]float64{0x0a0201: 21.4, 0x0a0202: 20.5, 0x0a0203: 22, 0x0a0204: 19, 0x0a0205: 19.5, 0x0a0206: 23.5}
	for rf, temp := range want {
		if got := st[rf]; got.ActualTemp == nil || *got.ActualTemp != temp || got.Mode != cube.ModeAuto || *got.Setpoint != 20 {
			t.Errorf("%s: %+v, want auto at 20 measuring %.1f", rf, got, temp)
		}
	}
	if w := st[0x0a0207].WindowOpen; w == nil || *w {
		t.Errorf("contact %+v", st[0x0a0207])
	}
	if cfg := c.config(0x0a0201); cfg.Type != cube.DeviceWallThermostat || cfg.RoomID != 1 || cfg.Serial != "NEQ0000201" {
		t.Errorf("wall thermostat config %+v", cfg)
	}
}

// A room setpoint changes every thermostat of the room and nothing else;
// a direct one only its device.
func TestApplySetpoint(t *testing.T) {
	s := startRooms(t, &Server{})
	c := dial(t, s.ln.Addr().String())
	c.send(must(cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0204, RoomID: 2, Room: true}, 22.5, cube.ModeManual, cube.Until{})))
	st := c.states()
	for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
		if st[rf].Mode != cube.ModeManual || *st[rf].Setpoint != 22.5 {
			t.Errorf("%s: %+v", rf, st[rf])
		}
	}
	if st[0x0a0202].Mode != cube.ModeAuto || *st[0x0a0202].Setpoint != 20 {
		t.Errorf("another room changed: %+v", st[0x0a0202])
	}
	if *st[0x0a0205].ActualTemp != 19.5 {
		t.Errorf("the measured temperature changed: %+v", st[0x0a0205])
	}
	c.send(must(cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0204}, 18, cube.ModeManual, cube.Until{})))
	st = c.states()
	if *st[0x0a0204].Setpoint != 18 || *st[0x0a0205].Setpoint != 22.5 {
		t.Errorf("direct setpoint: %+v %+v", st[0x0a0204], st[0x0a0205])
	}
}

// Auto without a temperature takes the programme's; vacation keeps its
// end, and leaving it drops the bytes that carried it.
func TestApplyAutoAndVacation(t *testing.T) {
	s := startRooms(t, &Server{})
	c := dial(t, s.ln.Addr().String())
	room1 := cube.Target{RFAddress: 0x0a0201, RoomID: 1, Room: true}
	c.send(must(cube.CmdSetTemperature(room1, 15, cube.ModeVacation, cube.Until{Year: 2027, Month: 1, Day: 6, Minutes: 600})))
	st := c.states()[0x0a0202]
	if st.Mode != cube.ModeVacation || st.Until == nil || *st.Until != (cube.Until{Year: 2027, Month: 1, Day: 6, Minutes: 600}) || *st.Setpoint != 15 {
		t.Fatalf("vacation: %+v %+v", st, st.Until)
	}
	c.send(must(cube.CmdSetTemperature(room1, 0, cube.ModeAuto, cube.Until{})))
	s.mu.Lock()
	want := float64(s.programHalfLocked(0x0a0202, time.Now())) / 2
	s.mu.Unlock()
	st = c.states()[0x0a0202]
	if st.Mode != cube.ModeAuto || *st.Setpoint != want || want == 0 || st.ActualTemp != nil {
		t.Fatalf("auto: %+v, want %.1f and no temperature", st, want)
	}
}

// A boost opens a radiator thermostat's valve to its boost opening and
// ends by itself with the mode and valve it had.
func TestApplyBoost(t *testing.T) {
	s := startRooms(t, &Server{BoostFor: 200 * time.Millisecond})
	c := dial(t, s.ln.Addr().String())
	c.send(must(cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0203, RoomID: 1, Room: true}, 20, cube.ModeBoost, cube.Until{})))
	st := c.states()
	if st[0x0a0203].Mode != cube.ModeBoost || *st[0x0a0203].ValvePct != 80 || st[0x0a0201].Mode != cube.ModeBoost {
		t.Fatalf("boost: %+v %+v", st[0x0a0203], st[0x0a0201])
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		st = c.states()
		if st[0x0a0203].Mode == cube.ModeAuto {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the boost did not end: %+v", st[0x0a0203])
		}
		time.Sleep(50 * time.Millisecond)
	}
	if *st[0x0a0203].ValvePct != 15 || st[0x0a0201].Mode != cube.ModeAuto {
		t.Fatalf("after the boost: %+v %+v", st[0x0a0203], st[0x0a0201])
	}
	// A setpoint during a boost ends it and stays.
	c.send(must(cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0204, RoomID: 2, Room: true}, 20, cube.ModeBoost, cube.Until{})))
	c.send(must(cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0204, RoomID: 2, Room: true}, 21, cube.ModeManual, cube.Until{})))
	time.Sleep(400 * time.Millisecond)
	if st := c.states()[0x0a0204]; st.Mode != cube.ModeManual || *st.Setpoint != 21 || *st.ValvePct != 0 {
		t.Fatalf("a setpoint after a boost: %+v", st)
	}
}

// A day's programme, the preset temperatures and the lock reach the C
// line and the L entry; a new connection's greeting carries them.
func TestApplyProgramTemperaturesLock(t *testing.T) {
	s := startRooms(t, &Server{})
	c := dial(t, s.ln.Addr().String())
	room2 := cube.Target{RFAddress: 0x0a0204, RoomID: 2, Room: true}
	day := cube.DayProgram{{Temp: 17, Until: 360}, {Temp: 21.5, Until: 1320}, {Temp: 17, Until: 1440}}
	lines, err := cube.CmdSetDayProgram(room2, cube.Monday, day)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		c.send(l)
	}
	c.send(must(cube.CmdSetTemperatures(room2, cube.TemperatureSettings{Comfort: 22, Eco: 16, Max: 30.5, Min: 4.5, WindowOpen: 12, WindowOpenMinutes: 15})))
	c.send(cube.CmdLockControls(cube.Target{RFAddress: 0x0a0205}, true))
	for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
		th := c.config(rf).Thermostat
		if got := th.WeekProgram[cube.Monday]; len(got) != 3 || got[1] != day[1] {
			t.Errorf("%s monday %+v", rf, got)
		}
		if th.ComfortTemp != 22 || th.EcoTemp != 16 || th.WindowOpenMins != 15 {
			t.Errorf("%s temperatures %+v", rf, th)
		}
	}
	if !c.states()[0x0a0205].PanelLocked {
		t.Error("not locked")
	}
	if c.config(0x0a0202).Thermostat.EcoTemp == 16 {
		t.Error("another room's thermostat changed")
	}
	again := dial(t, s.ln.Addr().String())
	for _, m := range again.greet {
		if m.Type == 'C' && strings.HasPrefix(m.Data, "0a0205,") {
			cfg, _ := cube.ParseConfig(m.Data)
			if cfg.Thermostat.ComfortTemp != 22 {
				t.Errorf("greeting's C line %+v", cfg.Thermostat)
			}
		}
	}
	if !again.states()[0x0a0205].PanelLocked {
		t.Error("the greeting's L line lost the lock")
	}
}

// With ApplyAfter the device's entry changes some time after the S
// reply; a discarded command and a fake without Apply change nothing.
func TestApplyDelayDiscardAndOff(t *testing.T) {
	s := startRooms(t, &Server{ApplyAfter: 300 * time.Millisecond})
	c := dial(t, s.ln.Addr().String())
	room2 := cube.Target{RFAddress: 0x0a0204, RoomID: 2, Room: true}
	c.send(must(cube.CmdSetTemperature(room2, 25, cube.ModeManual, cube.Until{})))
	if st := c.states()[0x0a0204]; st.Mode != cube.ModeAuto {
		t.Fatalf("applied at once: %+v", st)
	}
	time.Sleep(600 * time.Millisecond)
	if st := c.states()[0x0a0204]; st.Mode != cube.ModeManual || *st.Setpoint != 25 {
		t.Fatalf("not applied after the delay: %+v", st)
	}

	d := startRooms(t, &Server{Discard: true})
	cd := dial(t, d.ln.Addr().String())
	cd.send(must(cube.CmdSetTemperature(room2, 25, cube.ModeManual, cube.Until{})))
	if st := cd.states()[0x0a0204]; st.Mode != cube.ModeAuto {
		t.Fatalf("a discarded command was applied: %+v", st)
	}

	greeting, _ := RoomsFixture()
	off := &Server{Greeting: greeting}
	if _, err := off.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { off.Close() })
	co := dial(t, off.ln.Addr().String())
	co.send(must(cube.CmdSetTemperature(room2, 25, cube.ModeManual, cube.Until{})))
	if st := co.states()[0x0a0204]; st.Mode != cube.ModeAuto {
		t.Fatalf("applied without Apply: %+v", st)
	}
}

// The control sets a device's report, takes it out of its room and puts
// it back; over HTTP as well.
func TestControl(t *testing.T) {
	s := startRooms(t, &Server{})
	temp, battery := 18.2, true
	if err := s.SetState(0x0a0201, State{ActualTemp: &temp, BatteryLow: &battery}); err != nil {
		t.Fatal(err)
	}
	c := dial(t, s.ln.Addr().String())
	if st := c.states()[0x0a0201]; *st.ActualTemp != 18.2 || !st.BatteryLow {
		t.Fatalf("set state: %+v", st)
	}
	open := true
	if err := s.SetState(0x0a0202, State{WindowOpen: &open}); err == nil {
		t.Fatal("a window on a thermostat")
	}

	s.Remove(0x0a0201)
	gone := dial(t, s.ln.Addr().String())
	if _, ok := gone.states()[0x0a0201]; ok {
		t.Fatal("removed device still listed")
	}
	for _, m := range gone.greet {
		switch m.Type {
		case 'C':
			if strings.HasPrefix(m.Data, "0a0201,") {
				t.Fatal("removed device's C line in the greeting")
			}
		case 'M':
			part, _ := cube.ParseMetadataPart(m.Data)
			md, err := cube.ParseMetadata([]cube.MetadataPart{part})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range md.Devices {
				if d.RFAddress == 0x0a0201 {
					t.Fatal("removed device in the metadata")
				}
			}
			if md.Rooms[0].RFAddress != 0x0a0202 {
				t.Fatalf("room 1's first device %s", md.Rooms[0].RFAddress)
			}
		}
	}
	// A room command no longer reaches it.
	gone.send(must(cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0202, RoomID: 1, Room: true}, 23, cube.ModeManual, cube.Until{})))
	s.Restore(0x0a0201)
	back := dial(t, s.ln.Addr().String())
	if st := back.states()[0x0a0201]; st.Mode != cube.ModeAuto || *st.ActualTemp != 18.2 {
		t.Fatalf("restored: %+v", st)
	}

	addr, err := s.Control("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&Server{}).Control("0.0.0.0:0"); err == nil {
		t.Fatal("control on a public address")
	}
	req, _ := http.NewRequest("PUT", "http://"+addr+"/devices/0a0204/state", bytes.NewBufferString(`{"actual_temp": 16.5, "mode": "manual"}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT state: %v %v", resp, err)
	}
	for path, code := range map[string]int{"/devices/0a0206/remove": 204, "/devices/0a0206/restore": 204, "/devices/zz/remove": 400, "/drop": 204} {
		resp, err := http.Post("http://"+addr+path, "", nil)
		if err != nil || resp.StatusCode != code {
			t.Fatalf("POST %s: %v %v", path, resp, err)
		}
	}
	req, _ = http.NewRequest("PUT", "http://"+addr+"/devices/0a0999/state", bytes.NewBufferString(`{"actual_temp": 16.5}`))
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("PUT state of no device: %v %v", resp, err)
	}
	resp, err = http.Get("http://" + addr + "/devices")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	var states []cube.DeviceState
	if err := json.Unmarshal(body, &states); err != nil {
		t.Fatal(err)
	}
	for _, st := range states {
		if st.RFAddress == 0x0a0204 && (st.Mode != cube.ModeManual || *st.ActualTemp != 16.5) {
			t.Fatalf("GET devices: %+v", st)
		}
	}
}
