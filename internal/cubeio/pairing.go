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
// returns the watch of the N lines (cubelan.Client.WatchInclusion),
// registered before n: is written so that no line is missed, and the
// function that ends the watch. It fails with
// cubelan.ErrInclusionActive while pairing is already running.
func (w Writer) StartPairing(ctx context.Context, timeout time.Duration) (<-chan cube.Message, func(), error) {
	if timeout < time.Second || timeout > MaxPairingTime {
		return nil, nil, fmt.Errorf("pairing time must be 1 s to %s", MaxPairingTime)
	}
	if err := w.Client.StartInclusion(timeout); err != nil {
		return nil, nil, err
	}
	lines, stop := w.Client.WatchInclusion()
	res := w.Do(ctx, engine.Command{Key: "pairing", Priority: engine.PriorityHigh, Lines: []cube.Message{cube.CmdInclusion(timeout)}})
	if res.Err != nil {
		stop()
		w.Client.EndInclusion()
		return nil, nil, res.Err
	}
	return lines, stop, nil
}

// stopWait is how long StopPairing waits for the Cube's own end of
// inclusion mode (an empty N line) before ending it in the client.
const stopWait = 2 * time.Second

// StopPairing ends inclusion mode with x:. It waits up to two seconds
// for the Cube's empty N line, so that watchers get it, then counts the
// mode as ended in any case.
func (w Writer) StopPairing(ctx context.Context) error {
	lines, stop := w.Client.WatchInclusion()
	defer stop()
	res := w.Do(ctx, engine.Command{Key: "pairing", Priority: engine.PriorityHigh, Lines: []cube.Message{cube.CmdCancelInclusion}})
	if res.Err == nil {
		wait := time.NewTimer(stopWait)
		defer wait.Stop()
	drain:
		for {
			select {
			case _, ok := <-lines:
				if !ok {
					break drain
				}
			case <-wait.C:
				break drain
			case <-ctx.Done():
				break drain
			}
		}
	}
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
