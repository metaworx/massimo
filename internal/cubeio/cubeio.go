// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package cubeio holds the writes to a Cube that take more than one
// step: a metadata write that is mirrored into the snapshot because the
// Cube does not echo it, a configuration write followed by a re-read of
// the device's C line, a move of a device to another room, and the
// merge that keeps hidden devices in a client's metadata write. The API,
// the LAN proxy and the restore planner use them, so the snapshot stays
// in step with the Cube the same way whichever of them wrote.
package cubeio

import (
	"context"
	"fmt"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// Writer sends commands to one Cube through its queue and keeps the
// client's snapshot in step with what was written.
type Writer struct {
	Client *cubelan.Client
	Queue  *engine.Queue
}

// MetadataCommand builds the command that writes md (rooms, names and
// room membership) to the Cube, at high priority and coalesced with any
// other waiting metadata write.
func MetadataCommand(md *cube.Metadata) (engine.Command, error) {
	lines, err := cube.WriteMetadataCommands(md)
	if err != nil {
		return engine.Command{}, fmt.Errorf("metadata: %w", err)
	}
	return engine.Command{Key: "metadata", Priority: engine.PriorityHigh, Lines: lines}, nil
}

// Do runs cmd through the queue. When it succeeded, a metadata write (m:
// lines) is mirrored into the snapshot, and the configuration of each
// device in reread is read back from the Cube.
func (w Writer) Do(ctx context.Context, cmd engine.Command, reread ...cube.RFAddress) engine.Result {
	res := w.Queue.Enqueue(cmd).Wait(ctx)
	if res.Err != nil {
		return res
	}
	if isMetadataWrite(cmd.Lines) {
		w.Client.ApplyMetadataWrite(cmd.Lines)
	}
	for _, rf := range reread {
		w.Reread(ctx, rf)
	}
	return res
}

// WriteMetadata writes md to the Cube and mirrors it into the snapshot.
// The error is set only when md cannot be encoded; the outcome of the
// write is in the result.
func (w Writer) WriteMetadata(ctx context.Context, md *cube.Metadata) (engine.Result, error) {
	cmd, err := MetadataCommand(md)
	if err != nil {
		return engine.Result{}, err
	}
	return w.Do(ctx, cmd), nil
}

// Reread asks the Cube for a device's C line, usually after a
// configuration write, and applies the answer to the snapshot. Failures
// are ignored: the write itself succeeded, and the copy catches up on
// the next connection.
func (w Writer) Reread(ctx context.Context, rf cube.RFAddress) {
	res := w.Queue.Enqueue(engine.Command{Lines: []cube.Message{{Type: 'c', Data: rf.String()}}}).Wait(ctx)
	if res.Err == nil && len(res.Replies) == 1 && res.Replies[0].Type == 'C' {
		w.Client.ApplyLocal(res.Replies[0])
	}
}

func isMetadataWrite(lines []cube.Message) bool {
	if len(lines) == 0 {
		return false
	}
	for _, l := range lines {
		if l.Type != 'm' {
			return false
		}
	}
	return true
}

// KeepHidden puts the hidden devices of cur, and the rooms they are in,
// back into write, a metadata write from a client that never saw them,
// so that the write cannot remove them from the Cube. It returns how
// many devices it added.
func KeepHidden(write, cur *cube.Metadata, hidden view.Hidden) int {
	have := map[cube.RFAddress]bool{}
	for _, d := range write.Devices {
		have[d.RFAddress] = true
	}
	rooms := map[uint8]bool{}
	for _, r := range write.Rooms {
		rooms[r.ID] = true
	}
	added := 0
	for _, d := range cur.Devices {
		if !hidden[d.RFAddress] || have[d.RFAddress] {
			continue
		}
		write.Devices = append(write.Devices, d)
		added++
		if d.RoomID != 0 && !rooms[d.RoomID] {
			for _, r := range cur.Rooms {
				if r.ID == d.RoomID {
					write.Rooms = append(write.Rooms, r)
					rooms[r.ID] = true
				}
			}
		}
	}
	return added
}
