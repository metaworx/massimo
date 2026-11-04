// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package proxy

import (
	"context"
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
	go p.ServeUDP(ctx, pc)
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
