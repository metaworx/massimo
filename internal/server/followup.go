// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Follow-up after writes. The Cube acknowledges a radio write at once but
// changes a device's C line only when the device has acknowledged it,
// which took one to four minutes on hardware, and it reports its budget
// (free memory slots, so the pending radiograms) only in S replies and
// in the H line of a new connection. After a configuration write, its
// own or one a client of the LAN proxy sent (proxyWrote), the server
// therefore re-reads the device's C line a few times with growing
// pauses until it changes; after any radio write, once those re-reads
// are done, it reconnects once to get a fresh H line if radiograms were
// still pending, at most every FollowUpReconnectGap and only with the
// periodic refresh switched on. The changes reach the event stream as
// config and hello events.

import (
	"context"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Defaults of the follow-up.
var (
	// DefaultFollowUpDelays are the pauses before each re-read of a
	// written device's C line, about six minutes in all.
	DefaultFollowUpDelays = []time.Duration{15 * time.Second, 30 * time.Second, time.Minute, 2 * time.Minute, 2 * time.Minute}
	// DefaultFollowUpReconnectGap is the shortest time between two
	// reconnects for the budget.
	DefaultFollowUpReconnectGap = 15 * time.Minute
)

// budgetSettle is how long after the last radio write the budget
// reconnect waits at least, so that devices can acknowledge first.
const budgetSettle = 2 * time.Minute

type followUp struct {
	delays    []time.Duration
	gap       time.Duration
	settle    time.Duration
	reconnect bool // reconnects allowed (the periodic refresh is on)

	mu            sync.Mutex
	devices       map[cube.RFAddress]*followed
	budgetAfter   time.Time // a radio write wants a budget refresh after this
	lastReconnect time.Time
	wake          chan struct{}
}

type followed struct {
	baseline string // the C line right after the write
	step     int
	next     time.Time
}

func newFollowUp(delays []time.Duration, gap time.Duration, reconnect bool) *followUp {
	if len(delays) == 0 {
		delays = DefaultFollowUpDelays
	}
	if gap <= 0 {
		gap = DefaultFollowUpReconnectGap
	}
	settle := budgetSettle
	var total time.Duration
	for _, d := range delays {
		total += d
	}
	settle = min(settle, total)
	return &followUp{delays: delays, gap: gap, settle: settle, reconnect: reconnect,
		devices: map[cube.RFAddress]*followed{}, wake: make(chan struct{}, 1)}
}

// afterWrite records a radio write to the Cube and, for each device in
// configured, a configuration write whose C line is to be followed.
func (rt *runtime) afterWrite(configured ...cube.RFAddress) {
	f := rt.follow
	if f == nil {
		return
	}
	snap := rt.client.Snapshot()
	now := time.Now()
	f.mu.Lock()
	for _, rf := range configured {
		line, _ := snap.ConfigLine(rf)
		f.devices[rf] = &followed{baseline: line.Data, next: now.Add(f.delays[0])}
	}
	f.budgetAfter = now.Add(f.settle)
	f.mu.Unlock()
	select {
	case f.wake <- struct{}{}:
	default:
	}
}

// proxyWrote follows up a radio write that a client of the LAN proxy
// sent, as the API follows up its own: a configuration write (week
// programme, preset temperatures, valve settings, display mode, room
// group) has the C line of the device followed, or of every thermostat
// of the room when it was sent to the room, as the original software
// sends them; any other write (setpoint, key lock, links) only wants the
// budget refresh.
func (rt *runtime) proxyWrote(c cube.RadioCommand) {
	switch c.Command {
	case cube.RFSetWeekProgram, cube.RFSetTemperatures, cube.RFSetValveConfig, cube.RFSetDisplayMode, cube.RFSetGroupAddress, cube.RFRemoveGroup:
		if c.Target.Room {
			radiators, walls := roomThermostats(rt, c.Target.RoomID)
			rt.afterWrite(append(radiators, walls...)...)
			return
		}
		rt.afterWrite(c.Target.RFAddress)
	default:
		rt.afterWrite()
	}
}

// followUps runs the follow-up of one Cube until ctx ends.
func (s *Server) followUps(ctx context.Context, rt *runtime) {
	f := rt.follow
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		now := time.Now()
		due := f.nextDue(now, rt)
		timer.Reset(max(due.Sub(now), 0))
		select {
		case <-ctx.Done():
			return
		case <-f.wake:
			continue
		case <-timer.C:
		}
		s.followUpStep(ctx, rt)
	}
}

// nextDue is when the follow-up has to act next.
func (f *followUp) nextDue(now time.Time, rt *runtime) time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	due := now.Add(time.Hour)
	for _, d := range f.devices {
		if d.next.Before(due) {
			due = d.next
		}
	}
	if len(f.devices) == 0 && !f.budgetAfter.IsZero() {
		at := f.budgetAfter
		if next := f.lastReconnect.Add(f.gap); next.After(at) {
			at = next
		}
		if at.Before(due) {
			due = at
		}
	}
	return due
}

// followUpStep re-reads the C lines that are due, and reconnects for the
// budget when nothing else is left.
func (s *Server) followUpStep(ctx context.Context, rt *runtime) {
	f := rt.follow
	now := time.Now()
	f.mu.Lock()
	var due []cube.RFAddress
	for rf, d := range f.devices {
		if !d.next.After(now) {
			due = append(due, rf)
		}
	}
	f.mu.Unlock()
	w := rt.writer()
	for _, rf := range due {
		w.Reread(ctx, rf)
		line, _ := rt.client.Snapshot().ConfigLine(rf)
		f.mu.Lock()
		d := f.devices[rf]
		switch {
		case d == nil:
		case line.Data != d.baseline:
			delete(f.devices, rf)
			s.log.Info("device configuration taken over by the device", "cube", rt.serial, "device", rf)
		case d.step+1 >= len(f.delays):
			delete(f.devices, rf)
			s.log.Info("device configuration unchanged after the follow-up", "cube", rt.serial, "device", rf)
		default:
			d.step++
			d.next = time.Now().Add(f.delays[d.step])
		}
		f.mu.Unlock()
	}

	f.mu.Lock()
	ready := len(f.devices) == 0 && !f.budgetAfter.IsZero() && !now.Before(f.budgetAfter) && !now.Before(f.lastReconnect.Add(f.gap))
	if ready {
		f.budgetAfter = time.Time{}
	}
	f.mu.Unlock()
	if !ready || !f.reconnect {
		return
	}
	if rt.client.Budget().Pending() == 0 {
		return // nothing pending as far as the Cube last said
	}
	f.mu.Lock()
	f.lastReconnect = now
	f.mu.Unlock()
	res := rt.queue.Enqueue(engine.Command{Key: "refresh", Priority: engine.PriorityLow, Reconnect: true}).Wait(ctx)
	if res.Err != nil && ctx.Err() == nil {
		s.log.Warn("budget refresh failed", "cube", rt.serial, "error", res.Err)
		return
	}
	s.log.Info("budget refreshed after writes", "cube", rt.serial, "pending", rt.client.Budget().Pending())
}
