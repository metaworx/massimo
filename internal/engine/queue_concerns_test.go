// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package engine

import (
	"context"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// A command concerns a device by its Target, the device's room by its
// RoomID or a room-flagged line, the device by a line addressed to it or
// an r: that names it; a proxy client's command and a reconnect never do
// (AP RadioHealth v1.2, Block 4).
func TestConcerns(t *testing.T) {
	room6, _ := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a010d, RoomID: 6, Room: true}, 19, cube.ModeManual, cube.Until{})
	room3, _ := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0108, RoomID: 3, Room: true}, 19, cube.ModeManual, cube.Until{})
	direct := cube.CmdSetDisplayActual(cube.Target{RFAddress: 0x0a010d}, true)
	for _, c := range []struct {
		name string
		cmd  Command
		want bool
	}{
		{"its target", Command{Target: 0x0a010d, Lines: []cube.Message{room3}}, true},
		{"its room's wakeup", Command{RoomID: 6, Wakeup: true, Lines: []cube.Message{cube.CmdGetNTP}}, true},
		{"a line to its room", Command{Lines: []cube.Message{room6}}, true},
		{"a line to it", Command{Lines: []cube.Message{direct}}, true},
		{"an r: for it", Command{Lines: []cube.Message{cube.CmdResetError(0x0a0108, 0x0a010d)}}, true},
		{"another room", Command{Lines: []cube.Message{room3}}, false},
		{"a proxy client's", Command{Passthrough: true, Lines: []cube.Message{room6}}, false},
		{"a reconnect", Command{Reconnect: true}, false},
	} {
		if got := concerns(c.cmd, 0x0a010d, 6); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	// A device in no room is concerned by what names the device alone.
	if concerns(Command{Lines: []cube.Message{room6}}, 0x0a0101, 0) || concerns(Command{RoomID: 0, Lines: []cube.Message{cube.CmdGetNTP}}, 0x0a0101, 0) {
		t.Error("a device in no room concerned by a command of a room, or of none")
	}
}

// The queue's Concerns sees a command waiting, by the room of the
// device's C line, and none once it is done.
func TestQueueConcerns(t *testing.T) {
	_, q, _ := setup(t, Options{Gap: 200 * time.Millisecond})
	room6, _ := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a010d, RoomID: 6, Room: true}, 19, cube.ModeManual, cube.Until{})
	first := q.Enqueue(Command{Lines: []cube.Message{cube.CmdGetNTP}})
	second := q.Enqueue(Command{Lines: []cube.Message{room6}})
	if !q.Concerns(0x0a010d) {
		t.Fatal("a command to the device's room waits, and Concerns does not see it")
	}
	if q.Concerns(0x0a0108) {
		t.Fatal("a device of another room concerned")
	}
	ctx := context.Background()
	first.Wait(ctx)
	second.Wait(ctx)
	if !testkit.Within(5*time.Second, func() bool { return !q.Concerns(0x0a010d) }) {
		t.Fatal("a command done still concerns the device")
	}
}
