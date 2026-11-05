// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"context"
	"fmt"
	"time"

	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// MaxPairingTime is the longest inclusion the API and the proxy ask for;
// the Cube may end the mode earlier on its own.
const MaxPairingTime = 10 * time.Minute

// StartPairing puts the Cube into inclusion mode for timeout: it marks
// the mode in the client (cubelan.Client.StartInclusion), which then
// delivers the N lines, and writes n:<seconds> through the queue. It
// fails with cubelan.ErrInclusionActive while pairing is already
// running.
func (w Writer) StartPairing(ctx context.Context, timeout time.Duration) error {
	if timeout < time.Second || timeout > MaxPairingTime {
		return fmt.Errorf("pairing time must be 1 s to %s", MaxPairingTime)
	}
	if err := w.Client.StartInclusion(timeout); err != nil {
		return err
	}
	res := w.Do(ctx, engine.Command{Key: "pairing", Priority: engine.PriorityHigh, Lines: []cube.Message{cube.CmdInclusion(timeout)}})
	if res.Err != nil {
		w.Client.EndInclusion()
		return res.Err
	}
	return nil
}

// StopPairing ends inclusion mode with x:. The client counts the mode as
// ended at once; an N line the Cube may still send is ignored.
func (w Writer) StopPairing(ctx context.Context) error {
	res := w.Do(ctx, engine.Command{Key: "pairing", Priority: engine.PriorityHigh, Lines: []cube.Message{cube.CmdCancelInclusion}})
	w.Client.EndInclusion()
	return res.Err
}

// AddDevice puts a paired device that is in no room yet into room as
// the original software does after pairing: after a wakeup of the room,
// an 0x20 add-link-partner radiogram for every pair of the new device and
// a room member it links with, in both directions, then 0x22 set group;
// then the metadata write that lists it with its name, and a re-read of
// its C line. A device whose C line names another room (paired long ago,
// never listed) is first unlinked from that room as in a move. members
// are the devices the metadata places in room; md is the current
// metadata.
func (w Writer) AddDevice(ctx context.Context, md *cube.Metadata, c *cube.DeviceConfig, name string, room uint8) (engine.Result, error) {
	var members []cube.DeviceMeta
	for _, d := range md.Devices {
		if d.RoomID == room {
			members = append(members, d)
		}
	}
	out, in := MoveCommands(w.Client.Snapshot(), c.RFAddress, c.Type, c.RoomID, room, members)
	cmds := []engine.Command{in}
	if c.RoomID != 0 {
		cmds = []engine.Command{out, in}
	}
	for _, cmd := range cmds {
		if res := w.Do(ctx, cmd); res.Err != nil {
			return res, nil
		}
	}
	cp := *md
	cp.Devices = append(append([]cube.DeviceMeta(nil), md.Devices...), cube.DeviceMeta{Type: c.Type, RFAddress: c.RFAddress, Serial: c.Serial, Name: name, RoomID: room})
	cmd, err := MetadataCommand(&cp)
	if err != nil {
		return engine.Result{}, err
	}
	return w.Do(ctx, cmd, c.RFAddress), nil
}
