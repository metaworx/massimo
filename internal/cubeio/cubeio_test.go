// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"context"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

const fixture = "../protocol/cube/testdata/cube-fw0113-greeting.txt"

func connect(t *testing.T) (*fakecube.Server, Writer, context.Context) {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &fakecube.Server{Greeting: greeting}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := cubelan.New(cubelan.Options{Addr: addr})
	q := engine.NewQueue(c, engine.Options{Gap: time.Millisecond})
	go c.Run(ctx)
	go q.Run(ctx)
	select {
	case <-c.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("client not ready")
	}
	t.Cleanup(func() { cancel(); srv.Close() })
	return srv, Writer{Client: c, Queue: q}, ctx
}

func TestWriteMetadataIsMirrored(t *testing.T) {
	srv, w, ctx := connect(t)
	md := *w.Client.Snapshot().Metadata()
	md.Rooms = append([]cube.RoomMeta(nil), md.Rooms...)
	md.Rooms[0].Name = "Renamed"
	res, err := w.WriteMetadata(ctx, &md)
	if err != nil || res.Err != nil {
		t.Fatalf("write: %v %v", err, res.Err)
	}
	if got := w.Client.Snapshot().Metadata().Rooms[0].Name; got != "Renamed" {
		t.Fatalf("snapshot room name %q", got)
	}
	n := 0
	for _, m := range srv.Received() {
		if m.Type == 'm' {
			n++
		}
	}
	if n == 0 {
		t.Fatal("no m: sent")
	}
}

func TestDoRereadsConfig(t *testing.T) {
	srv, w, ctx := connect(t)
	const rf cube.RFAddress = 0x14d08e
	res := w.Do(ctx, engine.Command{Lines: []cube.Message{{Type: 'w', Data: "x"}}}, rf)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	got := srv.Received()
	if len(got) != 2 || got[1].Type != 'c' || got[1].Data != rf.String() {
		t.Fatalf("sent %v", got)
	}
}

func TestKeepHidden(t *testing.T) {
	cur := &cube.Metadata{
		Rooms:   []cube.RoomMeta{{ID: 1, Name: "a"}, {ID: cube.HiddenRoom, Name: "parked"}},
		Devices: []cube.DeviceMeta{{RFAddress: 0x000001, RoomID: 1}, {RFAddress: 0x000002, RoomID: cube.HiddenRoom}},
	}
	write := &cube.Metadata{Rooms: []cube.RoomMeta{{ID: 1, Name: "b"}}, Devices: []cube.DeviceMeta{{RFAddress: 0x000001, RoomID: 1}}}
	if n := KeepHidden(write, cur, view.NewHidden(nil, cur, false)); n != 1 {
		t.Fatalf("added %d", n)
	}
	if len(write.Devices) != 2 || len(write.Rooms) != 2 || write.Rooms[1].ID != cube.HiddenRoom || write.Rooms[0].Name != "b" {
		t.Fatalf("merged %+v", write)
	}
}

func TestMoveCommands(t *testing.T) {
	_, w, _ := connect(t)
	snap := w.Client.Snapshot()
	// 1514b6 (room 6) into room 1, whose only member is 14d08e.
	var members []cube.DeviceMeta
	for _, d := range snap.Metadata().Devices {
		if d.RoomID == 1 {
			members = append(members, d)
		}
	}
	c := snap.Config(0x1514b6)
	out, in := MoveCommands(snap, 0x1514b6, c.Type, 6, 1, members, nil)
	if out.RoomID != 6 || in.RoomID != 1 || !in.Wakeup || len(out.Lines) != 1 || len(in.Lines) != 1+2*len(members) {
		t.Fatalf("out %+v in %+v", out, in)
	}
}

// The copy of a room's settings: a thermostat alone in its room gets
// 0x40 only, a shutter contact nothing, and 1514b6 put into room 1
// takes 14d08e's settings, the 0x11 going to both thermostats.
func TestCopyCommands(t *testing.T) {
	_, w, _ := connect(t)
	snap := w.Client.Snapshot()
	steps, err := CopyCommands(snap, 0x1514b6, cube.DeviceRadiatorThermostat, 6, nil)
	if err != nil || len(steps) != 1 || steps[0].Command.Key != "copy-mode:1514b6" || !steps[0].Copies.Empty() {
		t.Fatalf("alone in its room: %+v %v", steps, err)
	}
	if steps, err := CopyCommands(snap, 0x1514b6, cube.DeviceShutterContact, 1, nil); err != nil || steps != nil {
		t.Fatalf("a contact: %+v %v", steps, err)
	}
	steps, err = CopyCommands(snap, 0x1514b6, cube.DeviceRadiatorThermostat, 1, nil)
	if err != nil || len(steps) != 4 {
		t.Fatalf("into room 1: %+v %v", steps, err)
	}
	temps, valve, prog := steps[1], steps[2], steps[3]
	if !temps.Command.Wakeup || temps.Command.RoomID != 1 || len(temps.Command.Lines) != 2 ||
		len(temps.Copies.Temperatures) != 2 || temps.Copies.Temperatures[0] != 0x1514b6 || temps.Copies.Temperatures[1] != 0x14d08e {
		t.Fatalf("temperatures %+v", temps)
	}
	ref := snap.Config(0x14d08e).Thermostat
	want, _ := cube.CmdSetTemperatures(cube.Target{RFAddress: 0x1514b6}, cube.TemperatureSettings{Comfort: ref.ComfortTemp, Eco: ref.EcoTemp,
		Max: ref.MaxSetpoint, Min: ref.MinSetpoint, WindowOpen: ref.WindowOpenTemp, WindowOpenMinutes: ref.WindowOpenMins})
	if temps.Command.Lines[0].String() != want.String() {
		t.Fatalf("0x11 %s, want %s", temps.Command.Lines[0], want)
	}
	if !valve.Copies.Valve || valve.Command.Target != 0x1514b6 || len(valve.Command.Lines) != 1 {
		t.Fatalf("valve %+v", valve)
	}
	if prog.Copies.Days != 7 || len(prog.Command.Lines) != 8 || prog.Command.Lines[7].String() != want.String() {
		t.Fatalf("programme %+v", prog)
	}
}

// RemoveCommands: 1514b6, alone in room 6, is unlinked from nobody but
// loses its group after a wakeup of room 6, then t: with the force flag;
// the Cube's own C line names no room, so it has nothing to unlink.
func TestRemoveCommands(t *testing.T) {
	_, w, _ := connect(t)
	snap := w.Client.Snapshot()
	unlink, del, ok := RemoveCommands(snap, 0x1514b6, cube.DeviceRadiatorThermostat)
	if !ok || !unlink.Wakeup || unlink.RoomID != 6 || len(unlink.Lines) != 1 || unlink.Lines[0].String() != cube.CmdSetGroup(0x1514b6, 6, true).String() {
		t.Fatalf("unlink %+v %v", unlink, ok)
	}
	if len(del.Lines) != 1 || del.Lines[0].String() != "t:01,1,FRS2" || del.Key != "delete:1514b6" {
		t.Fatalf("del %+v", del)
	}
	if _, _, ok := RemoveCommands(snap, 0x0b9573, cube.DeviceCube); ok {
		t.Fatal("the Cube's own C line taken for a room")
	}
}
