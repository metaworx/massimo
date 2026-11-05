// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package proxy

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

const fixture = "../protocol/cube/testdata/cube-fw0113-greeting.txt"

func setup(t *testing.T) (*fakecube.Server, *Proxy, *cubelan.Client, context.Context) {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
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
	p := New(Options{Client: c, Queue: q, ReplyTimeout: 5 * time.Second})
	t.Cleanup(func() { cancel(); fc.Close() })
	return fc, p, c, ctx
}

func readLine(t *testing.T, r *cube.Reader) cube.Message {
	t.Helper()
	m, err := r.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return m
}

func countType(fc *fakecube.Server, typ byte) int {
	n := 0
	for _, m := range fc.Received() {
		if m.Type == typ {
			n++
		}
	}
	return n
}

func TestProxyGreetingReadsAndPassthrough(t *testing.T) {
	fc, p, c, ctx := setup(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go p.ServeTCP(ctx, ln)

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	r := cube.NewReader(conn)

	// Greeting: H with our fields, M, the C lines, then L.
	h := readLine(t, r)
	if h.Type != 'H' {
		t.Fatalf("first line %s", h)
	}
	hello, err := cube.ParseHello(h.Data)
	if err != nil {
		t.Fatal(err)
	}
	real := c.Snapshot().Hello()
	if hello.Serial != real.Serial || hello.RFAddress != real.RFAddress || hello.Firmware != real.Firmware {
		t.Fatalf("hello %+v, want serial %s", hello, real.Serial)
	}
	if hello.ConnectionID == real.ConnectionID || len(hello.ConnectionID) != 8 {
		t.Fatalf("connection id %q not fresh", hello.ConnectionID)
	}
	if hello.CubeDate != time.Now().Format("2006-01-02") {
		t.Fatalf("cube date %s", hello.CubeDate)
	}
	var seen []byte
	var last cube.Message
	for last.Type != 'L' {
		last = readLine(t, r)
		seen = append(seen, last.Type)
	}
	if s := string(seen); !strings.HasPrefix(s, "M") || !strings.Contains(s, "C") || !strings.HasSuffix(s, "L") {
		t.Fatalf("greeting order %s", s)
	}
	greetingL := last

	// l: is answered locally.
	polls := countType(fc, 'l')
	conn.Write([]byte("l:\r\n"))
	if l := readLine(t, r); l.Type != 'L' || l.Data != greetingL.Data {
		t.Fatalf("l: answered %s", l)
	}
	if countType(fc, 'l') != polls {
		t.Fatal("l: reached the cube")
	}

	// c: for a known device is answered locally.
	conn.Write([]byte("c:1514b6\r\n"))
	if cl := readLine(t, r); cl.Type != 'C' || !strings.HasPrefix(cl.Data, "1514b6,") {
		t.Fatalf("c: answered %s", cl)
	}
	if countType(fc, 'c') != 0 {
		t.Fatal("c: reached the cube")
	}

	// s: goes through and the S reply comes back.
	conn.Write([]byte("s:AARAAAAAFRS2BgA=\r\n"))
	if s := readLine(t, r); s.Type != 'S' || s.Data != "19,0,31" {
		t.Fatalf("s: answered %s", s)
	}
	if countType(fc, 's') != 1 {
		t.Fatalf("cube received %d s commands", countType(fc, 's'))
	}

	// m: goes through, is answered A: and updates the snapshot.
	md := *c.Snapshot().Metadata()
	md.Rooms = append([]cube.RoomMeta(nil), md.Rooms...)
	md.Rooms[0].Name = "Renamed"
	cmds, err := cube.WriteMetadataCommands(&md)
	if err != nil || len(cmds) != 1 {
		t.Fatalf("metadata commands %v %v", cmds, err)
	}
	conn.Write([]byte(cmds[0].Wire()))
	if a := readLine(t, r); a.Type != 'A' {
		t.Fatalf("m: answered %s", a)
	}
	if countType(fc, 'm') != 1 {
		t.Fatal("m: did not reach the cube")
	}
	if got := c.Snapshot().Metadata().Rooms[0].Name; got != "Renamed" {
		t.Fatalf("snapshot room name %q after m:", got)
	}

	// q: closes.
	conn.Write([]byte("q:\r\n"))
	if _, err := r.Read(); err == nil {
		t.Fatal("connection still open after q:")
	}
}

func TestProxyDiscardsWhenBudgetExhausted(t *testing.T) {
	fc, p, _, ctx := setup(t)
	fc.Duty = 0x61 // 97 %
	// A command through the queue teaches the client the budget.
	p.opts.Queue.Enqueue(engine.Command{Passthrough: true, Lines: []cube.Message{{Type: 's', Data: "AARAAAAAFRS2BgA="}}}).Wait(ctx)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go p.ServeTCP(ctx, ln)
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	r := cube.NewReader(conn)
	for m := readLine(t, r); m.Type != 'L'; m = readLine(t, r) {
	}
	sent := countType(fc, 's')
	conn.Write([]byte("s:AARAAAAAFRS2BgA=\r\n"))
	if s := readLine(t, r); s.Type != 'S' || !strings.HasPrefix(s.Data, "61,1,") {
		t.Fatalf("answered %s", s)
	}
	if countType(fc, 's') != sent {
		t.Fatal("radio command reached the cube with the duty cycle exhausted")
	}
}

func TestProxyDiscovery(t *testing.T) {
	_, p, c, ctx := setup(t)
	p.opts.DiscoverySerial = "XEQ0537249"
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	resp := NewResponder(nil)
	if err := resp.Add(p, nil); err != nil {
		t.Fatal(err)
	}
	go resp.Serve(ctx, pc)
	client, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	req := append([]byte("eQ3Max*\x00**********"), 'I')
	if _, err := client.WriteTo(req, pc.LocalAddr()); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 256)
	client.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := buf[:n]
	if string(reply[:8]) != "eQ3MaxAp" || string(reply[8:18]) != "XEQ0537249" || reply[18] != '>' || reply[19] != 'I' || len(reply) != 26 {
		t.Fatalf("reply %q", reply)
	}
	h := c.Snapshot().Hello()
	if cube.RFAddress(binary.BigEndian.Uint32(reply[20:24])) != h.RFAddress || binary.BigEndian.Uint16(reply[24:26]) != 0x0113 {
		t.Fatalf("reply data % x", reply[20:])
	}
	// A request for another serial is ignored, one for the real serial answered.
	client.WriteTo(append([]byte("eQ3Max*\x00ZZZ0000000"), 'I'), pc.LocalAddr())
	client.WriteTo(append([]byte("eQ3Max*\x00"+h.Serial), 'I'), pc.LocalAddr())
	n, _, err = client.ReadFrom(buf)
	if err != nil || string(buf[8:18]) != "XEQ0537249" {
		t.Fatalf("second reply %q %v", buf[:n], err)
	}
}

func TestProxyHidesHiddenDevices(t *testing.T) {
	fc, p, c, ctx := setup(t)
	hidden := cube.RFAddress(0x1514b6) // in room 6 of the fixture
	p.opts.Hidden = []cube.RFAddress{hidden}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go p.ServeTCP(ctx, ln)
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	r := cube.NewReader(conn)
	var md *cube.Metadata
	seenC, lDevices := 0, 0
	for {
		m := readLine(t, r)
		switch m.Type {
		case 'M':
			_, rest, _ := strings.Cut(m.Data, ",")
			_, data, _ := strings.Cut(rest, ",")
			raw, _ := base64.StdEncoding.DecodeString(data)
			md, err = cube.DecodeMetadata(raw)
			if err != nil {
				t.Fatal(err)
			}
		case 'C':
			if strings.HasPrefix(m.Data, hidden.String()+",") {
				t.Fatal("C line of the hidden device sent")
			}
			seenC++
		case 'L':
			states, err := cube.ParseDeviceList(m.Data, nil)
			if err != nil {
				t.Fatal(err)
			}
			lDevices = len(states)
			for _, st := range states {
				if st.RFAddress == hidden {
					t.Fatal("hidden device in the L line")
				}
			}
		}
		if m.Type == 'L' {
			break
		}
	}
	for _, d := range md.Devices {
		if d.RFAddress == hidden {
			t.Fatal("hidden device in the metadata")
		}
	}
	if len(md.Rooms) != 6 || lDevices != 5 || seenC == 0 {
		t.Fatalf("rooms %d, devices in L %d, C lines %d", len(md.Rooms), lDevices, seenC)
	}
	// The client renames a room and writes the metadata it saw, without
	// the hidden device; the Cube receives it with the device put back.
	md.Rooms[0].Name = "Renamed"
	cmds, _ := cube.WriteMetadataCommands(md)
	conn.Write([]byte(cmds[0].Wire()))
	if a := readLine(t, r); a.Type != 'A' {
		t.Fatalf("m: answered %s", a)
	}
	var sent *cube.Metadata
	for _, m := range fc.Received() {
		if m.Type == 'm' {
			_, data, _ := strings.Cut(m.Data, ",")
			raw, _ := base64.StdEncoding.DecodeString(data)
			sent, _ = cube.DecodeMetadata(raw)
		}
	}
	found := false
	for _, d := range sent.Devices {
		if d.RFAddress == hidden && d.RoomID == 6 {
			found = true
		}
	}
	if !found || sent.Rooms[0].Name != "Renamed" {
		t.Fatalf("cube received metadata without the hidden device or the rename: %+v", sent)
	}
	if got := c.Snapshot().Metadata(); len(got.Devices) != 6 || got.Rooms[0].Name != "Renamed" {
		t.Fatalf("snapshot after m: %d devices, room %q", len(got.Devices), got.Rooms[0].Name)
	}

	// Parked in a room with a high id, the room disappears as well.
	cur := *c.Snapshot().Metadata()
	cur.Rooms = append(append([]cube.RoomMeta(nil), cur.Rooms...), cube.RoomMeta{ID: 255, Name: "parked"})
	cur.Devices = append([]cube.DeviceMeta(nil), cur.Devices...)
	for i := range cur.Devices {
		if cur.Devices[i].RFAddress == hidden {
			cur.Devices[i].RoomID = 255
		}
	}
	cmds, _ = cube.WriteMetadataCommands(&cur)
	_, data, _ := strings.Cut(cmds[0].Data, ",")
	c.ApplyLocal(cube.Message{Type: 'M', Data: "00,01," + data})
	conn2, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()
	r2 := cube.NewReader(conn2)
	for {
		m := readLine(t, r2)
		if m.Type == 'M' {
			_, rest, _ := strings.Cut(m.Data, ",")
			_, data, _ := strings.Cut(rest, ",")
			raw, _ := base64.StdEncoding.DecodeString(data)
			md, _ = cube.DecodeMetadata(raw)
		}
		if m.Type == 'L' {
			break
		}
	}
	for _, r := range md.Rooms {
		if r.ID == 255 {
			t.Fatal("parking room 255 visible")
		}
	}
	if len(md.Rooms) != 6 {
		t.Fatalf("%d rooms", len(md.Rooms))
	}
	// A write without it gets the room and the device back.
	md.Rooms[0].Name = "Again"
	cmds, _ = cube.WriteMetadataCommands(md)
	conn2.Write([]byte(cmds[0].Wire()))
	if a := readLine(t, r2); a.Type != 'A' {
		t.Fatalf("m: answered %s", a)
	}
	got := c.Snapshot().Metadata()
	room255, dev := false, false
	for _, r := range got.Rooms {
		room255 = room255 || r.ID == 255
	}
	for _, d := range got.Devices {
		dev = dev || (d.RFAddress == hidden && d.RoomID == 255)
	}
	if !room255 || !dev || got.Rooms[0].Name != "Again" {
		t.Fatalf("metadata after write: %+v", got)
	}
}

func TestProxyRefusesToDeleteHiddenDevices(t *testing.T) {
	fc, p, _, ctx := setup(t)
	hidden := cube.RFAddress(0x1514b6) // in room 6 of the fixture
	other := cube.RFAddress(0x14d08e)
	p.opts.Hidden = []cube.RFAddress{hidden}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go p.ServeTCP(ctx, ln)
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	r := cube.NewReader(conn)
	for readLine(t, r).Type != 'L' {
	}
	// Only the hidden device: answered locally, nothing reaches the Cube.
	conn.Write([]byte(cube.CmdDeleteDevices(true, hidden).Wire()))
	if a := readLine(t, r); a.Type != 'A' {
		t.Fatalf("t: answered %s", a)
	}
	if n := countType(fc, 't'); n != 0 {
		t.Fatalf("%d t: reached the cube", n)
	}
	// A visible and a hidden device: only the visible one is deleted.
	conn.Write([]byte(cube.CmdDeleteDevices(true, other, hidden).Wire()))
	if a := readLine(t, r); a.Type != 'A' {
		t.Fatalf("t: answered %s", a)
	}
	var got []cube.Message
	for _, m := range fc.Received() {
		if m.Type == 't' {
			got = append(got, m)
		}
	}
	if len(got) != 1 || got[0].String() != cube.CmdDeleteDevices(true, other).String() {
		t.Fatalf("cube received %v", got)
	}
	// With the hidden devices shown, the command goes through unchanged.
	p.opts.ShowHidden = true
	conn.Write([]byte(cube.CmdDeleteDevices(false, hidden).Wire()))
	if a := readLine(t, r); a.Type != 'A' {
		t.Fatalf("t: answered %s", a)
	}
	if n := countType(fc, 't'); n != 2 {
		t.Fatalf("%d t: reached the cube, want 2", n)
	}
}

func TestProxyPairing(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	const newDevice cube.RFAddress = 0x0a0b0c
	pair, err := fakecube.PairingFrom(greeting, 0x1514b6, newDevice, "TEQ0000001", 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting, Pair: pair}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); fc.Close() })
	c := cubelan.New(cubelan.Options{Addr: addr})
	go c.Run(ctx)
	<-c.Ready()
	q := engine.NewQueue(c, engine.Options{Gap: time.Millisecond})
	go q.Run(ctx)
	p := New(Options{Client: c, Queue: q, ReplyTimeout: 5 * time.Second})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go p.ServeTCP(ctx, ln)
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	r := cube.NewReader(conn)
	for readLine(t, r).Type != 'L' {
	}

	// n: from the client starts pairing through the server; the N line
	// that comes later is relayed to the client.
	conn.Write([]byte("n:001e\r\n"))
	m := readLine(t, r)
	if want := fakecube.NewDeviceLine(pair.Device); m.String() != want.String() {
		t.Fatalf("relayed %s, want %s", m, want)
	}
	if st := c.Inclusion(); !st.Active || len(st.Paired) != 1 || st.Paired[0].RFAddress != newDevice {
		t.Fatalf("server's pairing state %+v", st)
	}
	// The client fetches the new device's configuration, as the
	// original software does; the server's copy learns it too.
	conn.Write([]byte("c:0a0b0c\r\n"))
	if m := readLine(t, r); m.Type != 'C' || !strings.HasPrefix(m.Data, "0a0b0c,") {
		t.Fatalf("c: answered %s", m)
	}
	if cfg := c.Snapshot().Config(newDevice); cfg == nil || cfg.Serial != "TEQ0000001" {
		t.Fatalf("server's copy %+v", cfg)
	}
	// x: ends it; the Cube's empty N line is relayed.
	conn.Write([]byte("x:\r\n"))
	if m := readLine(t, r); m.Type != 'N' || m.Data != "" {
		t.Fatalf("end of pairing relayed as %s", m)
	}
	if c.Inclusion().Active {
		t.Fatal("pairing still active")
	}
	var got []string
	for _, m := range fc.Received() {
		got = append(got, m.String())
	}
	if strings.Join(got, " ") != "n:001e c:0a0b0c x:" {
		t.Fatalf("cube received %v", got)
	}
}
