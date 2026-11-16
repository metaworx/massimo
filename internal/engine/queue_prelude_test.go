// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package engine

import (
	"context"
	"slices"
	"testing"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// The queue sends exactly the resets PreludeOf names, in its order,
// before the command's lines (AP RadioHealth v1.3, D204-A): the Target,
// a room's flagged members, the flagged devices a direct line
// addresses, each once; nothing for a LAN client's command. The
// fixture's 0a0109 is in room 3 beside 0a0108 by its C line (room3),
// 0a010d in room 6; all three have the communication error.
func TestQueueSendsThePreludesResets(t *testing.T) {
	flagged := map[cube.RFAddress][2]byte{0x0a0108: {0x08, 0}, 0x0a0109: {0x08, 0}, 0x0a010d: {0x08, 0}}
	m := roomSetpoint(t)
	other := directLines(t, 0x0a010d)[0]
	for _, c := range []struct {
		name string
		cmd  Command
		want []cube.RFAddress
	}{
		{"a room setpoint", Command{Lines: []cube.Message{m}}, []cube.RFAddress{0x0a0108, 0x0a0109}},
		{"its Target first", Command{Target: 0x0a0109, Lines: []cube.Message{m}}, []cube.RFAddress{0x0a0109, 0x0a0108}},
		{"a room's configuration write", roomConfigWrite(t), []cube.RFAddress{0x0a0108, 0x0a0109}},
		{"a member addressed directly as well", Command{Lines: []cube.Message{m, directLines(t, 0x0a0109)[0], other}},
			[]cube.RFAddress{0x0a0108, 0x0a0109, 0x0a010d}},
		{"its own r: lines", Command{Lines: []cube.Message{cube.CmdResetError(0x0a0108), m}}, []cube.RFAddress{0x0a0109}},
		{"a LAN client's", Command{Passthrough: true, Lines: []cube.Message{m}}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			srv, q, _ := setup(t, Options{}, room3(t, flagged))
			if got := PreludeOf(q.gw.Snapshot(), c.cmd).Resets(); !slices.Equal(got, c.want) {
				t.Fatalf("PreludeOf names %v, want %v", got, c.want)
			}
			if res := q.Enqueue(c.cmd).Wait(context.Background()); res.Err != nil {
				t.Fatal(res.Err)
			}
			var want []string
			if c.cmd.Wakeup {
				want = append(want, "z:1e,G,03")
			}
			for _, rf := range c.want {
				want = append(want, cube.CmdResetError(rf).String())
			}
			sameSent(t, received(srv), append(want, strs(c.cmd.Lines...)...))
		})
	}
}

// A command without radio lines has no prelude, nor does a device
// command whose Target has the link error alone (D111-A).
func TestPreludeOfNone(t *testing.T) {
	_, q, _ := setup(t, Options{}, room3(t, map[cube.RFAddress][2]byte{0x0a0108: {0x08, 0}, 0x0a010d: {0, 0x40}}))
	snap := q.gw.Snapshot()
	if got := PreludeOf(snap, Command{Target: 0x0a0108, Lines: []cube.Message{{Type: 'c', Data: "0a0108"}}}).Resets(); got != nil {
		t.Fatalf("no radio line: %v", got)
	}
	m, _ := cube.CmdSetTemperature(cube.SetpointTarget(0x0a010d, 6), 19, cube.ModeManual, cube.Until{})
	if got := PreludeOf(snap, Command{Target: 0x0a010d, Lines: []cube.Message{m}}).Resets(); got != nil {
		t.Fatalf("the link error alone: %v", got)
	}
}
