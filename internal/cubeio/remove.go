// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubeio

import (
	"context"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// RemoveCommands builds the commands that delete a device from the Cube
// as the original software removed one on 2026-11-15 (docs/cube-protocol.md,
// "Lessons from the hardware days"; AP DeviceUnPairingFix v1.2, Block 1):
// unlink, after a wakeup of the room the device is in, the metadata's or
// else its C line's: one r: for every other member of the room whose
// error flag is set (the queue resets the device itself, the command's
// target), then one 0x21 with the room flag naming the device
// (linkLines), which every member of the room takes; no 0x23 and no
// 0x21 to the device, which t: resets anyway. The devices in skip, the
// hidden ones, are left out. Then del, t: with the force flag, which
// makes the Cube send the device a factory reset and drop it once it
// answered. ok is false for a device in no room, which has nothing to
// unlink, and for one in room 255, whose partners there are all hidden.
func RemoveCommands(snap *cubelan.Snapshot, rf cube.RFAddress, typ cube.DeviceType, skip view.Hidden) (unlink, del engine.Command, ok bool) {
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
	if room == 0 || room == cube.HiddenRoom {
		return engine.Command{}, del, false
	}
	members := RoomMembers(snap, room, skip)
	lines := flaggedResets(snap, members, rf)
	lines = append(lines, linkLines(roomMaster(snap, room), rf, typ, room, members, false)...)
	unlink = engine.Command{Key: "unlink:" + rf.String(), Target: rf, RoomID: room, Wakeup: true, Lines: lines}
	return unlink, del, true
}

// RemoveDevice deletes a device from the Cube (RemoveCommands, skip the
// hidden devices): the unlink where there is one, t:, then, where md
// lists the device, the
// metadata without it; once the Cube took the t:, the device is dropped
// from the snapshot (cubelan.Client.ForgetDevice), also when the
// metadata write fails after it. The error is set only when the
// metadata cannot be encoded; the outcome of the writes is in the
// result, that of the last command run.
func (w Writer) RemoveDevice(ctx context.Context, md *cube.Metadata, c *cube.DeviceConfig, skip view.Hidden) (engine.Result, error) {
	unlink, del, ok := RemoveCommands(w.Client.Snapshot(), c.RFAddress, c.Type, skip)
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
