// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package engine

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// The resets of the flagged devices a command's direct lines address
// (AP RadioHealth v1.2, D182-A): the fixture's 0a0109 is put into room 3
// beside 0a0108 by its C line (room3), as in the room commands' tests.

// directLines are the lines the room configuration write sends a
// thermostat, as the server builds them: 0x11 and 0x12 to the device
// directly, without the room flag.
func directLines(t *testing.T, rf cube.RFAddress) []cube.Message {
	t.Helper()
	temps, err := cube.CmdSetTemperatures(cube.Target{RFAddress: rf}, cube.TemperatureSettings{Comfort: 21, Eco: 17, Max: 30.5, Min: 4.5,
		WindowOpen: 12, WindowOpenMinutes: 15})
	if err != nil {
		t.Fatal(err)
	}
	valve, err := cube.CmdSetValveConfig(cube.Target{RFAddress: rf}, cube.ValveSettings{BoostMinutes: 5, BoostValvePct: 80,
		DecalcDay: cube.Saturday, DecalcHour: 12, MaxValvePct: 100})
	if err != nil {
		t.Fatal(err)
	}
	return []cube.Message{temps, valve}
}

// roomConfigWrite is room 3's configuration write as the server sends
// it: after a wakeup, the lines of each thermostat, the first its
// Target.
func roomConfigWrite(t *testing.T) Command {
	lines := append(directLines(t, 0x0a0108), directLines(t, 0x0a0109)...)
	return Command{Key: "config:room:3", Target: 0x0a0108, RoomID: 3, Wakeup: true, Lines: lines}
}

// strs is the wire form of lines.
func strs(lines ...cube.Message) []string {
	var out []string
	for _, l := range lines {
		out = append(out, l.String())
	}
	return out
}

// A room configuration write resets a flagged thermostat that is not its
// Target before its first radio line, once for its two lines, and logs
// the reset with the device; a flagged device the command does not
// address is left alone.
func TestQueueResetsAFlaggedAddressee(t *testing.T) {
	var log syncBuffer
	srv, q, _ := setup(t, Options{Logger: slog.New(slog.NewTextHandler(&log, nil))},
		room3(t, map[cube.RFAddress][2]byte{0x0a0109: {0x08, 0}, 0x0a010d: {0x08, 0}}))
	cmd := roomConfigWrite(t)
	if res := q.Enqueue(cmd).Wait(context.Background()); res.Err != nil || res.Retried {
		t.Fatalf("result %+v", res)
	}
	sameSent(t, received(srv), append([]string{"z:1e,G,03", "r:01,CgEJ"}, strs(cmd.Lines...)...))
	if want := `msg="communication error reset before a command" device=0a0109 room=3`; !strings.Contains(log.String(), want) {
		t.Errorf("log lacks %q:\n%s", want, log.String())
	}
}

// An addressee with the link error (0x40) alone is not reset (D111-A).
func TestQueueLeavesAnAddresseesLinkErrorAlone(t *testing.T) {
	srv, q, _ := setup(t, Options{}, room3(t, map[cube.RFAddress][2]byte{0x0a0109: {0, 0x40}}))
	cmd := roomConfigWrite(t)
	if res := q.Enqueue(cmd).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
	sameSent(t, received(srv), append([]string{"z:1e,G,03"}, strs(cmd.Lines...)...))
}

// A direct line the Cube discards is retried once, after its flagged
// addressee is reset again; the other line's device, unflagged, gets no
// reset. The command has no Target, as a copy's 0x11 to the room's
// thermostats.
func TestQueueRetriesADiscardedDirectLine(t *testing.T) {
	srv, q, _ := setup(t, Options{}, room3(t, map[cube.RFAddress][2]byte{0x0a0109: {0x08, 0}}))
	to08, to09 := directLines(t, 0x0a0108)[0], directLines(t, 0x0a0109)[0]
	var mu sync.Mutex
	discarded := false
	srv.DiscardIf = func(m cube.Message) bool {
		mu.Lock()
		defer mu.Unlock()
		if m == to09 && !discarded {
			discarded = true
			return true
		}
		return false
	}
	res := q.Enqueue(Command{Key: "copy-temps:0a0109", RoomID: 3, Wakeup: true, Lines: []cube.Message{to08, to09}}).Wait(context.Background())
	if res.Err != nil || res.Discarded || !res.Retried {
		t.Fatalf("result %+v", res)
	}
	sameSent(t, received(srv), []string{"z:1e,G,03", "r:01,CgEJ", to08.String(), to09.String(), "r:01,CgEJ", to09.String()})
}

// An addressee the command resets otherwise is not reset twice: the
// Target, by its prelude; a device of the command's own r: lines; a
// member reset for the room a room-flagged line of the command names.
func TestQueueResetsAnAddresseeOnce(t *testing.T) {
	flagged := map[cube.RFAddress][2]byte{0x0a0109: {0x08, 0}}
	lines := directLines(t, 0x0a0109)

	srv, q, _ := setup(t, Options{}, room3(t, flagged))
	if res := q.Enqueue(Command{Target: 0x0a0109, Lines: lines}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
	sameSent(t, received(srv), append([]string{"r:01,CgEJ"}, strs(lines...)...))

	srv, q, _ = setup(t, Options{}, room3(t, flagged))
	own := cube.CmdResetError(0x0a0109)
	if res := q.Enqueue(Command{Lines: append([]cube.Message{own}, lines...)}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
	sameSent(t, received(srv), append([]string{"r:01,CgEJ"}, strs(lines...)...))

	var log syncBuffer
	srv, q, _ = setup(t, Options{Logger: slog.New(slog.NewTextHandler(&log, nil))}, room3(t, flagged))
	m := roomSetpoint(t)
	if res := q.Enqueue(Command{Lines: []cube.Message{m, lines[0]}}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
	sameSent(t, received(srv), []string{"r:01,CgEJ", m.String(), lines[0].String()})
	if n := strings.Count(log.String(), "communication error reset before"); n != 1 {
		t.Fatalf("%d resets logged, want the room's one:\n%s", n, log.String())
	}
}
