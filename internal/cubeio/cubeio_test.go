// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
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
	ready(t, c)
	t.Cleanup(func() { cancel(); srv.Close() })
	return srv, Writer{Client: c, Queue: q}, ctx
}

// ready waits for the client's first complete greeting (Ready).
func ready(t *testing.T, c *cubelan.Client) {
	t.Helper()
	select {
	case <-c.Ready():
	case <-time.After(testkit.Scale(testkit.Wait)):
		t.Fatalf("client not ready within %v", testkit.Scale(testkit.Wait))
	}
}

func TestWriteMetadataIsMirrored(t *testing.T) {
	srv, w, ctx := connect(t)
	lines := cubewatch.Lines(srv)
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
	if lines.Count(0, cubewatch.Type('m')) == 0 {
		t.Fatal("no m: sent")
	}
}

func TestDoRereadsConfig(t *testing.T) {
	srv, w, ctx := connect(t)
	lines := cubewatch.Lines(srv)
	const rf cube.RFAddress = 0x0a010a
	res := w.Do(ctx, engine.Command{Lines: []cube.Message{{Type: 'w', Data: "x"}}}, rf)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	got := lines.Since(0, cubewatch.Except('l'))
	if len(got) != 2 || got[1].Type != 'c' || got[1].Data != rf.String() {
		t.Fatalf("sent %v", got)
	}
}

// One re-read is one config event of the device: the client's Send
// applies the C reply, and Reread does not apply it again (AP
// RereadOnce v1.0, Block 1).
func TestRereadIsOneConfigEvent(t *testing.T) {
	_, w, ctx := connect(t)
	evs := testkit.ChanWatch(t, w.Client.Events(), func(e cubelan.Event) string { return e.Kind.String() + " " + e.Device.String() })
	const rf cube.RFAddress = 0x0a010a
	_, before := w.Client.EventCounts()
	w.Reread(ctx, rf)
	if c := w.Client.Snapshot().Config(rf); c == nil {
		t.Fatalf("no configuration of %s after the re-read", rf)
	}
	// Reread has returned: every event of it is out, up to the client's
	// last, which the watch is to hold before it is counted.
	_, sent := w.Client.EventCounts()
	evs.Await(t, 0, "the client's last event", func(e cubelan.Event) bool { return e.Seq() >= sent })
	got := evs.Since(0, func(e cubelan.Event) bool {
		return e.Seq() > before && e.Kind == cubelan.EventConfig && e.Device == rf
	})
	if len(got) != 1 {
		t.Fatalf("one re-read gave %d config events of %s, want 1", len(got), rf)
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
	// 0a010d (room 6) into room 1, whose only member is 0a010a.
	var members []cube.DeviceMeta
	for _, d := range snap.Metadata().Devices {
		if d.RoomID == 1 {
			members = append(members, d)
		}
	}
	c := snap.Config(0x0a010d)
	out, in := MoveCommands(snap, 0x0a010d, c.Type, 6, 1, members, nil)
	if out.RoomID != 6 || in.RoomID != 1 || !out.Wakeup || !in.Wakeup || len(out.Lines) != 1 || len(in.Lines) != 3 {
		t.Fatalf("out %+v in %+v", out, in)
	}
	// Out: 0x23 alone, room 6 holding no partner. In, as the original's
	// capture of 2026-11-04: one 0x20 with the room flag to room 1's
	// master naming the device, one to the device naming its partner, 0x22.
	if out.Lines[0].String() != cube.CmdSetGroup(0x0a010d, 6, true).String() {
		t.Fatalf("out starts with %s, not 0x23", out.Lines[0])
	}
	want := []cube.Message{cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, true, 0x0a010d, c.Type),
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010d}, true, 0x0a010a, cube.DeviceRadiatorThermostat),
		cube.CmdSetGroup(0x0a010d, 1, false)}
	for i := range want {
		if in.Lines[i].String() != want[i].String() {
			t.Fatalf("in line %d: %s, want %s", i, in.Lines[i], want[i])
		}
	}
	// Out of room 1, whose master 0a010a stays: the 0x23, one 0x21 with
	// the room flag to 0a010a naming the device, one to the device naming
	// 0a010a.
	out, _ = MoveCommands(snap, 0x0a010d, c.Type, 1, 6, nil, nil)
	want = []cube.Message{cube.CmdSetGroup(0x0a010d, 1, true),
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}, false, 0x0a010d, c.Type),
		cube.CmdLinkPartner(cube.Target{RFAddress: 0x0a010d}, false, 0x0a010a, cube.DeviceRadiatorThermostat)}
	if len(out.Lines) != len(want) {
		t.Fatalf("out of room 1: %v", out.Lines)
	}
	for i := range want {
		if out.Lines[i].String() != want[i].String() {
			t.Fatalf("out line %d: %s, want %s", i, out.Lines[i], want[i])
		}
	}
}

// RemoveCommands: 0a010d, alone in room 6, has no partner to unlink: a
// wakeup of room 6 and no line, no 0x23 (a removal sends none), then t:
// with the force flag; the Cube's own C line names no room, so it has
// nothing to unlink.
func TestRemoveCommands(t *testing.T) {
	_, w, _ := connect(t)
	snap := w.Client.Snapshot()
	unlink, del, ok := RemoveCommands(snap, 0x0a010d, cube.DeviceRadiatorThermostat, nil)
	if !ok || !unlink.Wakeup || unlink.RoomID != 6 || unlink.Target != 0x0a010d || len(unlink.Lines) != 0 || unlink.Key != "unlink:0a010d" {
		t.Fatalf("unlink %+v %v", unlink, ok)
	}
	if len(del.Lines) != 1 || del.Lines[0].String() != "t:01,1,CgEN" || del.Key != "delete:0a010d" {
		t.Fatalf("del %+v", del)
	}
	if _, _, ok := RemoveCommands(snap, 0x0a0107, cube.DeviceCube, nil); ok {
		t.Fatal("the Cube's own C line taken for a room")
	}
}

// observed keeps the IDs of the commands the queue started.
type observed struct {
	mu  sync.Mutex
	ids []string
}

func (o *observed) Enqueued(engine.Command) {}
func (o *observed) Started(c engine.Command) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.ids = append(o.ids, c.ID+":"+string(c.Lines[0].Type))
}
func (o *observed) Finished(engine.Command, engine.Result) {}

// The re-read after a write is a step of the writer's request, as every
// command of the writer is, so that a failed re-read shows there.
func TestRereadIsAStepOfTheRequest(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	srv := &fakecube.Server{Greeting: greeting}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	obs := &observed{}
	c := cubelan.New(cubelan.Options{Addr: addr})
	q := engine.NewQueue(c, engine.Options{Gap: time.Millisecond, Observer: obs})
	go c.Run(ctx)
	go q.Run(ctx)
	ready(t, c)
	w := Writer{Client: c, Queue: q, ID: "request-1"}
	rf := cube.RFAddress(0x0a010d)
	if res := w.Do(ctx, engine.Command{Lines: []cube.Message{{Type: 's', Data: "AARAAAAACgENBms="}}}, rf); res.Err != nil {
		t.Fatal(res.Err)
	}
	obs.mu.Lock()
	defer obs.mu.Unlock()
	if len(obs.ids) != 2 || obs.ids[0] != "request-1:s" || obs.ids[1] != "request-1:c" {
		t.Fatalf("the request's steps %v, want the write and its re-read", obs.ids)
	}
}
