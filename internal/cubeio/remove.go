// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"context"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// RemoveCommands builds the commands that delete a device from the Cube
// as the original software deletes it (docs/portal-protocol.md,
// "Pairing a device, as the original software does it"): unlink, the
// out-part of MoveCommands for the room the device is in, the
// metadata's or else its C line's, which after a wakeup of that room
// unlinks it from every partner in both directions and removes its
// group; then del, t: with the force flag, which makes the Cube send the
// device a factory reset and drop it once it answered. ok is false for
// a device in no room, which has nothing to unlink.
func RemoveCommands(snap *cubelan.Snapshot, rf cube.RFAddress, typ cube.DeviceType) (unlink, del engine.Command, ok bool) {
	del = engine.Command{Key: "delete:" + rf.String(), Lines: []cube.Message{cube.CmdDeleteDevices(true, rf)}}
	room := uint8(0)
	if md := snap.Metadata(); md != nil {
		for _, d := range md.Devices {
			if d.RFAddress == rf {
				room = d.RoomID
			}
		}
	}
	if room == 0 {
		if c := snap.Config(rf); c != nil && c.Type != cube.DeviceCube {
			room = c.RoomID
		}
	}
	if room == 0 {
		return engine.Command{}, del, false
	}
	unlink, _ = MoveCommands(snap, rf, typ, room, 0, nil, nil)
	return unlink, del, true
}

// RemoveDevice deletes a device from the Cube (RemoveCommands): the
// unlink where there is one, t:, then, where md lists the device, the
// metadata without it; once the Cube took the t:, the device is dropped
// from the snapshot (cubelan.Client.ForgetDevice), also when the
// metadata write fails after it. The error is set only when the
// metadata cannot be encoded; the outcome of the writes is in the
// result, that of the last command run.
func (w Writer) RemoveDevice(ctx context.Context, md *cube.Metadata, c *cube.DeviceConfig) (engine.Result, error) {
	unlink, del, ok := RemoveCommands(w.Client.Snapshot(), c.RFAddress, c.Type)
	if ok {
		if res := w.Do(ctx, unlink); res.Err != nil {
			return res, nil
		}
	}
	res := w.Do(ctx, del)
	if res.Err != nil {
		return res, nil
	}
	defer w.Client.ForgetDevice(c.RFAddress)
	listed := false
	cp := *md
	cp.Devices = nil
	for _, d := range md.Devices {
		if d.RFAddress == c.RFAddress {
			listed = true
			continue
		}
		cp.Devices = append(cp.Devices, d)
	}
	if !listed {
		return res, nil
	}
	cmd, err := MetadataCommand(&cp)
	if err != nil {
		return res, err
	}
	return w.Do(ctx, cmd), nil
}
