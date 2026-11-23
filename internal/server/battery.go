// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"time"

	"go.metaworx.ch/massimo/internal/battery"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// The devices' battery levels (AP LegacyLogs v1.0, Block 11, D137-C):
// each runtime keeps a battery.Tracker, fed with every reported state
// of a device by the pump, evaluated every minute by batteryTicks, and
// rebuilt at the start from the log's state rows of the last
// batteryHistory. A change of a level is an event of the kind battery,
// in the log, on the stream and for the notification rules. New
// batteries marked through the API (battery_api.go, AP RefreshBatteries
// v1.0, Block 3) make a weak or low device ok at once; their event says
// so (causeReplaced), and the rebuild replays it.

// batteryHistory is how far back the log is read at the start: a weak
// level lasts until the next decalcification, a week at most.
const batteryHistory = 8 * 24 * time.Hour

// batteryTick is how often the levels' rules of time are evaluated.
const batteryTick = time.Minute

// decalcOf is a radiator thermostat's decalcification, from its C line;
// nil for a device without one.
func (rt *runtime) decalcOf(dev string) *battery.Decalc {
	rf, err := cube.ParseRFAddress(dev)
	if err != nil {
		return nil
	}
	c := rt.client.Snapshot().Config(rf)
	if c == nil || c.Thermostat == nil || c.Type == cube.DeviceWallThermostat {
		return nil
	}
	// The Cube counts the days from Saturday, the time package from
	// Sunday.
	return &battery.Decalc{Day: time.Weekday((int(c.Thermostat.DecalcDay) + 6) % 7), Hour: c.Thermostat.DecalcHour}
}

// batteryReport is what the tracker reads of a device's view: its flag,
// and a radiator thermostat's valve.
func batteryReport(d view.Device) (battery.Report, bool) {
	if !d.Reported || d.State == nil || d.Virtual {
		return battery.Report{}, false
	}
	r := battery.Report{Flag: d.State.BatteryLow}
	if d.Type.IsRadiatorThermostat() {
		r.Valve = d.State.ValvePct
	}
	return r, true
}

// setBattery gives a device's view its battery level.
func (rt *runtime) setBattery(d *view.Device) {
	if d.State == nil || d.Virtual || d.Type == cube.DeviceCube {
		return
	}
	d.Battery = string(rt.battery.Level(d.RFAddress))
}

// withBattery gives each device of a list its battery level.
func (rt *runtime) withBattery(devs []view.Device) []view.Device {
	for i := range devs {
		rt.setBattery(&devs[i])
	}
	return devs
}

// batteryEvent is the event of a change of a level: low at warn, the
// others at info, its summary the level and the one before, its state
// the device's view where the device has a state; false for a device
// that is no radio address.
func (s *Server) batteryEvent(rt *runtime, c battery.Change) (event, bool) {
	rf, err := cube.ParseRFAddress(c.Device)
	if err != nil {
		return event{}, false
	}
	snap := rt.client.Snapshot()
	level := "info"
	if c.To == battery.Low {
		level = "warn"
	}
	ev := event{Kind: "battery", Source: "device", Device: c.Device, Serial: snap.Serial(rf), Level: level, Time: c.At,
		Summary: map[string]any{"level": string(c.To), "from": string(c.From)}}
	if _, ok := snap.State(rf); ok {
		dv := view.DeviceOf(snap, rf, s.staleness())
		rt.setBattery(&dv)
		ev.State = dv
	}
	return ev, true
}

// emitBattery makes an event of each change of a level: in the log, on
// the stream and for the notification rules.
func (s *Server) emitBattery(rt *runtime, changes []battery.Change) {
	for _, c := range changes {
		if ev, ok := s.batteryEvent(rt, c); ok {
			rt.emit(ev)
			s.notifyEvent(rt, ev)
		}
	}
}

// batteryTicks evaluates the levels' rules of time every batteryTick.
func (s *Server) batteryTicks(ctx context.Context, rt *runtime) {
	t := time.NewTicker(batteryTick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.emitBattery(rt, rt.battery.Tick(now))
		}
	}
}

// rebuildBattery replays the log's state rows of the Cube's devices of
// the last batteryHistory into the tracker, and its battery rows of new
// batteries marked (causeReplaced), in the order of their times, the
// oldest first, so that a level outlives a restart; its changes make no
// events, as they are not news. A Cube without a row in the store has no
// log to replay.
func (s *Server) rebuildBattery(ctx context.Context, rt *runtime) {
	id := rt.storeID.Load()
	if rt.store == nil || id == 0 {
		return
	}
	now := time.Now()
	since := now.Add(-batteryHistory)
	states, err := eventRows(ctx, rt.store, store.EventQuery{CubeID: id, Kind: "state", Source: "device", Since: since})
	var marks []store.Event
	if err == nil {
		marks, err = eventRows(ctx, rt.store, store.EventQuery{CubeID: id, Kind: "battery", Source: "device", Since: since})
	}
	if err != nil {
		s.log.Warn("battery levels not rebuilt from the log", "cube", rt.cubeSerial(), "error", err)
		return
	}
	rows := append(states, marks...)
	slices.SortStableFunc(rows, func(a, b store.Event) int {
		if c := a.Time.Compare(b.Time); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	for _, row := range rows {
		if row.Device == "" {
			continue
		}
		var data struct {
			State   view.Device    `json:"state"`
			Summary map[string]any `json:"summary"`
		}
		if json.Unmarshal(row.Data, &data) != nil {
			continue
		}
		switch row.Kind {
		case "state":
			if r, ok := batteryReport(data.State); ok {
				rt.battery.Report(row.Device, row.Time, r)
			}
		case "battery":
			if data.Summary["cause"] == causeReplaced {
				rt.battery.Replaced(row.Device, row.Time)
			}
		}
	}
	rt.battery.Tick(now)
}

// eventRows reads every row of the log that q asks for, page by page,
// the newest first.
func eventRows(ctx context.Context, st *store.Store, q store.EventQuery) ([]store.Event, error) {
	q.Limit = store.MaxEventsPage
	var rows []store.Event
	for {
		page, err := st.Events(ctx, q)
		if err != nil {
			return nil, err
		}
		rows = append(rows, page...)
		if len(page) < store.MaxEventsPage {
			return rows, nil
		}
		q.BeforeID = page[len(page)-1].ID
	}
}
