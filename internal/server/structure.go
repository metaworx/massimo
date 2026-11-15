// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Structure changes read back from the Cube (AP DeviceUnPairingFix v1.3,
// Block 3). The Cube accepts the radiograms of a removal, a placement or a
// move at once, but the members of the room take them only at their next
// wakeup, about ten minutes later (T134, docs/cube-protocol.md), and the
// Cube holds a memory slot for each until then. It reports its free slots
// only in S replies and in the H line of a new connection. So a structure
// change is pending until a budget the Cube reported after it shows the
// free slots back at their count before it: then it is done. The checks
// run in the follow-up loop (followup.go) at StructureChecks after the
// change; at the last, where nothing was read since the change, the server
// reconnects once for a fresh H line, where reconnects are allowed. Still
// short, the change is not confirmed, and the slots it still holds are
// named. Either way, a member of the rooms concerned that the change left
// with the error flag or a link error it did not have before is named.

import (
	"context"
	"errors"
	"math"
	"slices"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// DefaultStructureChecks are the times after a structure change at which
// its budget is read back: the follow-up's re-reads of the C lines take
// about six minutes, and the members of a room take their radiograms at
// their next wakeup, about ten minutes on.
var DefaultStructureChecks = []time.Duration{2 * time.Minute, 6 * time.Minute, 12 * time.Minute}

// memberFlags are a device's error flag (byte 5's 0x08) and link error
// (byte 6's 0x40).
type memberFlags struct{ err, link bool }

// structureWatch is one structure change read back.
type structureWatch struct {
	action string         // "remove", "add" or "move"
	device cube.RFAddress // the device removed, placed or moved
	rooms  []uint8        // the rooms concerned
	before int            // the free slots before the change; -1 unknown
	flags  map[cube.RFAddress]memberFlags
	end    time.Time // when the change's last line was answered
	step   int       // the index of the next check
	next   time.Time // when the next check is due
}

// beginStructure records what a structure change of device in rooms is
// read back against, before it is sent: the Cube's free slots and the
// flags of the rooms' members.
func (rt *runtime) beginStructure(action string, device cube.RFAddress, rooms ...uint8) *structureWatch {
	w := &structureWatch{action: action, device: device, before: -1, flags: map[cube.RFAddress]memberFlags{}}
	for _, r := range rooms {
		if r != 0 && r != cube.HiddenRoom && !slices.Contains(w.rooms, r) {
			w.rooms = append(w.rooms, r)
		}
	}
	if b := rt.client.Budget(); !b.UpdatedAt.IsZero() {
		w.before = b.FreeMemorySlots
	}
	snap := rt.client.Snapshot()
	for _, rf := range w.members(rt) {
		st, _ := snap.State(rf)
		w.flags[rf] = memberFlags{st.Error, st.LinkError}
	}
	rt.settleMismatches(device, w.rooms, rt.clock()) // until its C line follows (congruence.go)
	return w
}

// members are the device and the devices whose C line names one of the
// watch's rooms, the hidden ones left out.
func (w *structureWatch) members(rt *runtime) []cube.RFAddress {
	snap := rt.client.Snapshot()
	hidden := rt.hidden()
	var out []cube.RFAddress
	for _, rf := range snap.Devices() {
		c := snap.Config(rf)
		if c == nil || hidden[rf] || c.Type == cube.DeviceCube {
			continue
		}
		if rf == w.device || slices.Contains(w.rooms, c.RoomID) {
			out = append(out, rf)
		}
	}
	return out
}

// back reports whether b, a budget read after the change, has the free
// slots back at their count before it.
func (w *structureWatch) back(b cubelan.Budget) bool {
	return b.UpdatedAt.After(w.end) && b.FreeMemorySlots >= w.before
}

// watchStructure starts reading back a structure change the Cube took, w
// from beginStructure; false where it cannot, without the follow-up or
// a budget from before.
func (rt *runtime) watchStructure(w *structureWatch) bool {
	if w != nil {
		rt.settledMismatches(w.device, rt.clock()) // its lines answered (congruence.go)
	}
	f := rt.follow
	if f == nil || w == nil || w.before < 0 {
		return false
	}
	now := time.Now()
	w.end, w.next = now, now.Add(f.checks[0])
	f.mu.Lock()
	f.watches = append(f.watches, w)
	f.mu.Unlock()
	select {
	case f.wake <- struct{}{}:
	default:
	}
	return true
}

// structureStep reads back the structure changes that are due.
func (s *Server) structureStep(ctx context.Context, rt *runtime) {
	f := rt.follow
	now := time.Now()
	f.mu.Lock()
	var due []*structureWatch
	kept := f.watches[:0]
	for _, w := range f.watches {
		if w.next.After(now) {
			kept = append(kept, w)
		} else {
			due = append(due, w)
		}
	}
	f.watches = kept
	f.mu.Unlock()
	for _, w := range due {
		last := w.step+1 >= len(f.checks)
		b := rt.client.Budget()
		if last && !w.back(b) && !b.UpdatedAt.After(w.end) && f.reconnect {
			res := rt.queue.Enqueue(engine.Command{Key: "refresh", Priority: engine.PriorityLow, Reconnect: true}).Wait(ctx)
			if res.Err != nil && !errors.Is(res.Err, engine.ErrSuperseded) && ctx.Err() == nil {
				s.log.Warn("budget refresh failed", "cube", rt.cubeSerial(), "error", res.Err)
			}
			b = rt.client.Budget()
		}
		switch {
		case w.back(b):
			s.structureRead(rt, w, b, true)
		case last:
			s.structureRead(rt, w, b, false)
		default:
			w.step++
			w.next = w.end.Add(f.checks[w.step])
			f.mu.Lock()
			f.watches = append(f.watches, w)
			f.mu.Unlock()
		}
	}
}

// structureRead logs how a structure change was read back: done, or not
// confirmed with the slots still held (or no reading at all), and the
// members it left with the error flag or a link error.
func (s *Server) structureRead(rt *runtime, w *structureWatch, b cubelan.Budget, done bool) {
	snap := rt.client.Snapshot()
	var flagged, linkErr []string
	for _, rf := range w.members(rt) {
		st, ok := snap.State(rf)
		if !ok {
			continue
		}
		was := w.flags[rf]
		if st.Error && !was.err {
			flagged = append(flagged, rf.String())
		}
		if st.LinkError && !was.link {
			linkErr = append(linkErr, rf.String())
		}
	}
	summary := map[string]any{"action": w.action, "outcome": "done", "before": w.before, "free_slots": b.FreeMemorySlots,
		"minutes": int(math.Round(time.Since(w.end).Minutes()))}
	level := "info"
	if !done {
		summary["outcome"], level = "not_confirmed", "warn"
		if b.UpdatedAt.After(w.end) {
			summary["held"] = w.before - b.FreeMemorySlots
		} else {
			summary["no_reading"] = true
		}
	}
	if len(flagged) > 0 {
		summary["flagged"], level = flagged, "warn"
	}
	if len(linkErr) > 0 {
		summary["link_error"], level = linkErr, "warn"
	}
	rt.emit(event{Kind: "structure", Source: "cube", Level: level, Device: w.device.String(), Time: time.Now(), Summary: summary})
}
