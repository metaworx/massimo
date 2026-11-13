// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package proxy

import (
	"context"
	"encoding/base64"
	"net"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// virtualsOfRooms is the virtual wall thermostats of the fake Cube's
// rooms fixture: room 1 with its real wall thermostat 0a0201, room 2
// with radiator thermostats only, room 3 with a contact alone.
func virtualsOfRooms() []Virtual {
	vwt := func(id uint8, name string, wall, member cube.RFAddress) Virtual {
		rf := cube.RFAddress(id)
		a := rf.Bytes()
		cfg := make([]byte, 0x16+cube.WeekProgramSize+3)
		cfg[0] = byte(len(cfg) - 1)
		copy(cfg[1:4], a[:])
		cfg[4], cfg[5] = byte(cube.DeviceWallThermostat), id
		copy(cfg[8:18], "VWT000000"+string('0'+rune(id)))
		return Virtual{Room: cube.RoomMeta{ID: id, Name: name}, RF: rf, Serial: "VWT000000" + string('0'+rune(id)), Wall: wall, Member: member,
			Entry: []byte{0x0c, a[0], a[1], a[2], 0, 0x12, 0x10, 0x01, 40, 0, 0, 0, 205}, Config: cfg}
	}
	return []Virtual{vwt(1, "Wohnen", 0x0a0201, 0x0a0202), vwt(2, "Schlafen", 0, 0x0a0204), vwt(3, "Flur", 0, 0)}
}

// setupVirtual serves the rooms fixture through a proxy that shows the
// virtual wall thermostats, and returns a client's connection past the
// greeting, with the greeting.
func setupVirtual(t *testing.T) (*fakecube.Server, *cube.Reader, net.Conn, []cube.Message) {
	t.Helper()
	greeting, err := fakecube.RoomsFixture()
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting, Duty: 0x19}
	addr, err := fc.Start("127.0.0.1:0")
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
	q := engine.NewQueue(c, engine.Options{Gap: time.Millisecond})
	go q.Run(ctx)
	p := New(Options{Client: c, Queue: q, ReplyTimeout: 5 * time.Second, Virtual: virtualsOfRooms})
	t.Cleanup(func() { cancel(); fc.Close() })
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go p.ServeTCP(ctx, ln)
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	r := cube.NewReader(conn)
	var lines []cube.Message
	for {
		m := readLine(t, r)
		lines = append(lines, m)
		if m.Type == 'L' {
			return fc, r, conn, lines
		}
	}
}

// send writes a line and reads the answer.
func sendLine(t *testing.T, conn net.Conn, r *cube.Reader, m cube.Message) cube.Message {
	t.Helper()
	if _, err := conn.Write([]byte(m.Wire())); err != nil {
		t.Fatal(err)
	}
	return readLine(t, r)
}

// The greeting shows the virtual wall thermostats in the metadata, their
// C lines and their entries, and leaves the real wall thermostat out.
func TestVirtualGreeting(t *testing.T) {
	_, r, conn, lines := setupVirtual(t)
	var parts []cube.MetadataPart
	configs := map[string]bool{}
	var list cube.Message
	for _, m := range lines {
		switch m.Type {
		case 'M':
			part, err := cube.ParseMetadataPart(m.Data)
			if err != nil {
				t.Fatal(err)
			}
			parts = append(parts, part)
		case 'C':
			addr, _, _ := strings.Cut(m.Data, ",")
			configs[addr] = true
		case 'L':
			list = m
		}
	}
	md, err := cube.ParseMetadata(parts)
	if err != nil {
		t.Fatal(err)
	}
	in := map[cube.RFAddress]uint8{}
	for _, d := range md.Devices {
		in[d.RFAddress] = d.RoomID
	}
	if in[0x000001] != 1 || in[0x000002] != 2 || in[0x000003] != 3 {
		t.Fatalf("virtual wall thermostats in the metadata: %v", in)
	}
	if _, ok := in[0x0a0201]; ok {
		t.Fatal("the real wall thermostat in the metadata")
	}
	if !configs["000001"] || !configs["000002"] || configs["0a0201"] {
		t.Fatalf("C lines %v", configs)
	}
	if !listed(t, list, 0x000002) || listed(t, list, 0x0a0201) {
		t.Fatal("L line: the virtual one missing or the real one there")
	}
	if c := sendLine(t, conn, r, cube.Message{Type: 'c', Data: "000002"}); c.Type != 'C' || !strings.HasPrefix(c.Data, "000002,") {
		t.Fatalf("c:000002 answered %s", c)
	}
	if l := sendLine(t, conn, r, cube.Message{Type: 'l'}); !listed(t, l, 0x000001) || listed(t, l, 0x0a0201) {
		t.Fatalf("l: answered %s", l)
	}
}

// listed reports whether an L line has rf's entry.
func listed(t *testing.T, l cube.Message, rf cube.RFAddress) bool {
	t.Helper()
	raw, _ := base64.StdEncoding.DecodeString(l.Data)
	for i := 0; i+4 <= len(raw); i += 1 + int(raw[i]) {
		if cube.RFAddressFromBytes(raw[i+1:i+4]) == rf {
			return true
		}
	}
	return false
}

// A command to a virtual wall thermostat goes to its room, or to the
// room's real wall thermostat, or is answered by the proxy; a link with
// one is a link with the real wall thermostat.
func TestVirtualCommands(t *testing.T) {
	fc, r, conn, _ := setupVirtual(t)
	last := func() cube.RadioCommand {
		t.Helper()
		got := fc.Received()
		for i := len(got) - 1; i >= 0; i-- {
			if got[i].Type == 's' {
				rc, _ := cube.ParseRadioCommand(got[i].Data)
				return rc
			}
		}
		return cube.RadioCommand{}
	}
	sent := func() int { return countType(fc, 's') }

	set, _ := cube.CmdSetTemperature(cube.Target{RFAddress: 0x000002}, 21, cube.ModeManual, cube.Until{})
	if s := sendLine(t, conn, r, set); s.Type != 'S' {
		t.Fatalf("setpoint answered %s", s)
	}
	if rc := last(); rc.Command != cube.RFSetTemperature || !rc.Target.Room || rc.Target.RoomID != 2 || rc.Target.RFAddress != 0x0a0204 {
		t.Fatalf("setpoint sent %+v", rc)
	}
	sendLine(t, conn, r, cube.CmdLockControls(cube.Target{RFAddress: 0x000001}, true))
	if rc := last(); rc.Command != cube.RFLockControls || rc.Target.RFAddress != 0x0a0201 {
		t.Fatalf("lock sent %+v", rc)
	}
	before := sent()
	if s := sendLine(t, conn, r, cube.CmdLockControls(cube.Target{RFAddress: 0x000002}, true)); s.Type != 'S' || sent() != before {
		t.Fatalf("a lock without a real wall thermostat: %s, sent %d", s, sent()-before)
	}
	link := cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a0202}, true, 0x000001, cube.DeviceWallThermostat)
	sendLine(t, conn, r, link)
	got := fc.Received()
	raw, _ := base64.StdEncoding.DecodeString(got[len(got)-1].Data)
	if cube.RFAddressFromBytes(raw[10:13]) != 0x0a0201 {
		t.Fatalf("link partner %x", raw[10:13])
	}
	before = sent()
	link = cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a0204}, true, 0x000002, cube.DeviceWallThermostat)
	if s := sendLine(t, conn, r, link); s.Type != 'S' || sent() != before {
		t.Fatalf("a link with a virtual one without a real wall thermostat: %s", s)
	}
}

// A metadata write leaves the virtual wall thermostats out and puts the
// real wall thermostat back; one the proxy cannot read is not sent; a
// virtual wall thermostat is not deleted.
func TestVirtualMetadataAndDelete(t *testing.T) {
	fc, r, conn, lines := setupVirtual(t)
	var parts []cube.MetadataPart
	for _, m := range lines {
		if m.Type == 'M' {
			part, _ := cube.ParseMetadataPart(m.Data)
			parts = append(parts, part)
		}
	}
	md, _ := cube.ParseMetadata(parts)
	md.Rooms[1].Name = "Schlafen neu"
	cmds, err := cube.WriteMetadataCommands(md)
	if err != nil || len(cmds) != 1 {
		t.Fatalf("metadata %d parts %v", len(cmds), err)
	}
	if a := sendLine(t, conn, r, cmds[0]); a.Type != 'A' {
		t.Fatalf("m: answered %s", a)
	}
	var written *cube.Metadata
	for _, m := range fc.Received() {
		if m.Type == 'm' {
			_, data, _ := strings.Cut(m.Data, ",")
			raw, _ := base64.StdEncoding.DecodeString(data)
			written, _ = cube.DecodeMetadata(raw)
		}
	}
	if written == nil {
		t.Fatal("no metadata written")
	}
	in := map[cube.RFAddress]bool{}
	for _, d := range written.Devices {
		in[d.RFAddress] = true
	}
	if in[0x000001] || in[0x000002] || !in[0x0a0201] || written.Rooms[1].Name != "Schlafen neu" {
		t.Fatalf("written metadata %+v", written)
	}

	ms := countType(fc, 'm')
	if a := sendLine(t, conn, r, cube.Message{Type: 'm', Data: "01," + base64.StdEncoding.EncodeToString([]byte("rest"))}); a.Type != 'A' || countType(fc, 'm') != ms {
		t.Fatalf("a second part: %s, sent %d", a, countType(fc, 'm')-ms)
	}
	ts := countType(fc, 't')
	if a := sendLine(t, conn, r, cube.CmdDeleteDevices(true, 0x000001)); a.Type != 'A' || countType(fc, 't') != ts {
		t.Fatalf("delete of a virtual one: %s, sent %d", a, countType(fc, 't')-ts)
	}
}
