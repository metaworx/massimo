// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"errors"
	"net"
	"net/http"
	"slices"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// lines are messages as their protocol lines, for a comparison that
// names the lines that differ.
func lines(ms []cube.Message) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.String()
	}
	return out
}

// A reset over the control puts back the greeting the fake started
// with: what the commands, the control, an m:, an f:, a t: and a pairing
// changed is undone, a command held or due later is not applied after
// it, and a silence ends; the clients are dropped and read the greeting
// anew, a SingleClient fake's at once.
func TestReset(t *testing.T) {
	t.Parallel()
	greeting, err := RoomsFixture()
	if err != nil {
		t.Fatal(err)
	}
	const joins cube.RFAddress = 0x0a0b0c
	pair, err := PairingFrom(greeting, 0x0a0202, joins, "TEQ0000001", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Greeting: greeting, Pair: pair, SingleClient: true, ApplyAfter: 20 * time.Millisecond}
	s.Apply = true
	addr, err := s.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	control, err := s.Control("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	c := dial(t, addr)
	if !slices.Equal(lines(c.greet), lines(greeting)) {
		t.Fatalf("the first greeting:\n%v\nwant\n%v", lines(c.greet), lines(greeting))
	}

	// What a test changes: a setpoint applied, a state reported, a
	// device removed, one deaf and one deleted, the device that joins,
	// the rooms renamed, the NTP servers.
	room2 := cube.Target{RFAddress: 0x0a0204, RoomID: 2, Room: true}
	c.send(must(cube.CmdSetTemperature(room2, 25, cube.ModeManual, cube.Until{})))
	testkit.Eventually(t, "the setpoint applied after ApplyAfter", func() bool {
		return c.states()[0x0a0204].Mode == cube.ModeManual
	})
	temp, low := 16.5, true
	if err := s.SetState(0x0a0201, State{ActualTemp: &temp, BatteryLow: &low}); err != nil {
		t.Fatal(err)
	}
	s.Remove(0x0a0207)
	s.SetDeaf(0x0a0205, true)
	if a := c.send(cube.CmdDeleteDevices(true, 0x0a0206)); a.Type != 'A' {
		t.Fatalf("t: answered %s", a)
	}
	join(t, c, pair)
	md := metadataOf(t, greeting)
	md.Rooms[0].Name = "Renamed"
	cmds, err := cube.WriteMetadataCommands(md)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range cmds {
		if a := c.send(m); a.Type != 'A' {
			t.Fatalf("m: answered %s", a)
		}
	}
	if f := c.send(cube.CmdSetNTP("ntp.example.org")); f.Data != "ntp.example.org" {
		t.Fatalf("f: answered %s", f)
	}
	// Two commands acknowledged under a hold, each due after ApplyAfter,
	// before the reset or across it.
	release := s.HoldApply()
	c.send(cube.CmdLockControls(cube.Target{RFAddress: 0x0a0203}, true))
	c.send(must(cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0202}, 28, cube.ModeManual, cube.Until{})))
	s.SetSilent(true)
	changed := s.greeting()
	if slices.Equal(lines(changed), lines(greeting)) {
		t.Fatal("the test changed nothing")
	}

	resp, err := http.Post("http://"+control+"/reset", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /reset: %s", resp.Status)
	}
	// The client of before is dropped.
	c.conn.SetReadDeadline(time.Now().Add(testkit.Scale(testkit.Wait)))
	var timeout net.Error
	if m, err := c.r.Read(); err == nil || errors.As(err, &timeout) && timeout.Timeout() {
		t.Fatalf("the client of before the reset read %s, %v; want the end of its connection", m, err)
	}
	// A new client is greeted at once with the greeting of the start, the
	// silence over and the slot free.
	again := dial(t, addr)
	if !slices.Equal(lines(again.greet), lines(greeting)) {
		t.Fatalf("the greeting after the reset:\n%v\nwant\n%v", lines(again.greet), lines(greeting))
	}
	if l := again.send(cube.CmdList); l.String() != lastList(greeting).String() {
		t.Fatalf("l: after the reset: %s, want the greeting's", l)
	}
	for _, rf := range []cube.RFAddress{0x0a0206, 0x0a0207} {
		if cfg := again.config(rf); cfg.RFAddress != rf {
			t.Fatalf("c:%s after the reset: %+v", rf, cfg)
		}
	}
	if a := again.send(cube.Message{Type: 'c', Data: joins.String()}); a.Type != 'A' {
		t.Fatalf("c: of the device that joined before the reset: %s, want none", a)
	}
	if f := again.send(cube.Message{Type: 'f'}); f.String() != "F:ntp.homematic.com,ntp.homematic.com" {
		t.Fatalf("f: after the reset: %s", f)
	}
	// What was acknowledged before the reset is not applied at the
	// release, whether it fell due before the reset or after it.
	testkit.Eventually(t, "the two commands of before the reset due and held", func() bool {
		s.held.mu.Lock()
		defer s.held.mu.Unlock()
		return len(s.held.held) == 2
	})
	release()
	if l := again.send(cube.CmdList); l.String() != lastList(greeting).String() {
		t.Fatalf("l: after the release: %s, want the greeting's", l)
	}
	// The fake applies what comes after the reset, to the deaf device of
	// before too.
	again.send(must(cube.CmdSetTemperature(room2, 21, cube.ModeManual, cube.Until{})))
	testkit.Eventually(t, "the setpoint sent after the reset applied", func() bool {
		st := again.states()
		return st[0x0a0205].Setpoint != nil && *st[0x0a0205].Setpoint == 21
	})
}
