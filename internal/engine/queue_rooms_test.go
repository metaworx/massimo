// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package engine

import (
	"context"
	"encoding/base64"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// The room commands' resets of their flagged members (AP RadioHealth
// v1.2, Block 2): the fixture's 0a0109 is put into room 3 beside 0a0108
// by its C line, the room byte that counts for the Cube.

// withFlags returns the greeting with bits set in rf's L entry: f1 in
// byte 5 (0x08, the communication error), f2 in byte 6 (0x40, the link
// error).
func withFlags(t *testing.T, greeting []cube.Message, rf cube.RFAddress, f1, f2 byte) []cube.Message {
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
				b[off+5] |= f1
				b[off+6] |= f2
			}
		}
		out[i].Data = base64.StdEncoding.EncodeToString(b)
	}
	return out
}

// inRoom returns the greeting with rf's C line naming room.
func inRoom(t *testing.T, greeting []cube.Message, rf cube.RFAddress, room uint8) []cube.Message {
	t.Helper()
	out := append([]cube.Message(nil), greeting...)
	for i, m := range out {
		addr, enc, ok := strings.Cut(m.Data, ",")
		if m.Type != 'C' || !ok || addr != rf.String() {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			t.Fatal(err)
		}
		b[5] = room
		out[i].Data = addr + "," + base64.StdEncoding.EncodeToString(b)
		return out
	}
	t.Fatalf("no C line for %s", rf)
	return nil
}

// room3 is the fixture with 0a0109 in room 3, and the flags given set.
func room3(t *testing.T, flags map[cube.RFAddress][2]byte) func([]cube.Message) []cube.Message {
	return func(g []cube.Message) []cube.Message {
		g = inRoom(t, g, 0x0a0109, 3)
		for rf, f := range flags {
			g = withFlags(t, g, rf, f[0], f[1])
		}
		return g
	}
}

// roomSetpoint is room 3's setpoint as the server sends it: addressed to
// 0a0108, the room's master, and with no Target.
func roomSetpoint(t *testing.T) cube.Message {
	t.Helper()
	m, err := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0108, RoomID: 3, Room: true}, 19, cube.ModeManual, cube.Until{})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// sameSent fails the test where the fake Cube received other lines than
// want.
func sameSent(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("sent %v, want %v", got, want)
	}
}

// A room setpoint resets both flagged members of its room before it, and
// not a flagged device of another room; each reset is logged with the
// device and the room.
func TestQueueResetsFlaggedRoomMembers(t *testing.T) {
	var log syncBuffer
	srv, q, _ := setup(t, Options{Logger: slog.New(slog.NewTextHandler(&log, nil))},
		room3(t, map[cube.RFAddress][2]byte{0x0a0108: {0x08, 0}, 0x0a0109: {0x08, 0}, 0x0a010d: {0x08, 0}}))
	m := roomSetpoint(t)
	if res := q.Enqueue(Command{Lines: []cube.Message{m}}).Wait(context.Background()); res.Err != nil || res.Retried {
		t.Fatalf("result %+v", res)
	}
	sameSent(t, received(srv), []string{"r:01,CgEI", "r:01,CgEJ", m.String()})
	for _, want := range []string{
		`msg="communication error reset before a room command" device=0a0108 room=3`,
		`msg="communication error reset before a room command" device=0a0109 room=3`,
	} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, log.String())
		}
	}
}

// A member with the link error (0x40) alone is not reset: r: clears the
// communication error only (D111-A).
func TestQueueLeavesARoomMembersLinkErrorAlone(t *testing.T) {
	srv, q, _ := setup(t, Options{}, room3(t, map[cube.RFAddress][2]byte{0x0a0109: {0, 0x40}}))
	m := roomSetpoint(t)
	if res := q.Enqueue(Command{Lines: []cube.Message{m}}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
	sameSent(t, received(srv), []string{m.String()})
}

// A device command to a device with the link error alone sends no r:.
func TestQueueLeavesATargetsLinkErrorAlone(t *testing.T) {
	srv, q, _ := setup(t, Options{}, func(g []cube.Message) []cube.Message { return withFlags(t, g, 0x0a010d, 0, 0x40) })
	m, _ := cube.CmdSetTemperature(cube.SetpointTarget(0x0a010d, 6), 19, cube.ModeManual, cube.Until{})
	if res := q.Enqueue(Command{Target: 0x0a010d, Lines: []cube.Message{m}}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
	sameSent(t, received(srv), []string{m.String()})
}

// The command's Target, which its prelude resets, and a member the
// command's own r: lines reset, as a removal's do, are not reset twice.
func TestQueueResetsEachMemberOnce(t *testing.T) {
	srv, q, _ := setup(t, Options{}, room3(t, map[cube.RFAddress][2]byte{0x0a0108: {0x08, 0}, 0x0a0109: {0x08, 0}}))
	m := roomSetpoint(t)
	if res := q.Enqueue(Command{Target: 0x0a0108, Lines: []cube.Message{m}}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
	sameSent(t, received(srv), []string{"r:01,CgEI", "r:01,CgEJ", m.String()})

	srv, q, _ = setup(t, Options{}, room3(t, map[cube.RFAddress][2]byte{0x0a0108: {0x08, 0}, 0x0a0109: {0x08, 0}}))
	own := cube.CmdResetError(0x0a0109)
	if res := q.Enqueue(Command{Lines: []cube.Message{own, m}}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
	sameSent(t, received(srv), []string{"r:01,CgEI", "r:01,CgEJ", m.String()})
}

// A room line the Cube discards is retried once, after the same members
// are reset again.
func TestQueueRetriesADiscardedRoomLine(t *testing.T) {
	srv, q, _ := setup(t, Options{}, room3(t, map[cube.RFAddress][2]byte{0x0a0108: {0x08, 0}, 0x0a0109: {0x08, 0}}))
	var mu sync.Mutex
	discarded := false
	srv.DiscardIf = func(m cube.Message) bool {
		mu.Lock()
		defer mu.Unlock()
		if m.Type == 's' && !discarded {
			discarded = true
			return true
		}
		return false
	}
	m := roomSetpoint(t)
	res := q.Enqueue(Command{Lines: []cube.Message{m}}).Wait(context.Background())
	if res.Err != nil || res.Discarded || !res.Retried {
		t.Fatalf("result %+v", res)
	}
	sameSent(t, received(srv), []string{"r:01,CgEI", "r:01,CgEJ", m.String(), "r:01,CgEI", "r:01,CgEJ", m.String()})
}

// A room line discarded with no flagged member is not retried, as
// before: nothing was reset that could change the Cube's answer.
func TestQueueDoesNotRetryAnUnflaggedRoom(t *testing.T) {
	srv, q, _ := setup(t, Options{}, room3(t, nil))
	srv.Discard = true
	m := roomSetpoint(t)
	res := q.Enqueue(Command{Lines: []cube.Message{m}}).Wait(context.Background())
	if res.Err != ErrDiscarded || res.Retried {
		t.Fatalf("result %+v", res)
	}
	sameSent(t, received(srv), []string{m.String()})
}

// A passthrough command, a LAN client's own, gets no reset.
func TestQueuePassthroughGetsNoMemberReset(t *testing.T) {
	srv, q, _ := setup(t, Options{}, room3(t, map[cube.RFAddress][2]byte{0x0a0108: {0x08, 0}, 0x0a0109: {0x08, 0}}))
	m := roomSetpoint(t)
	if res := q.Enqueue(Command{Passthrough: true, Lines: []cube.Message{m}}).Wait(context.Background()); res.Err != nil {
		t.Fatal(res.Err)
	}
	sameSent(t, received(srv), []string{m.String()})
}
