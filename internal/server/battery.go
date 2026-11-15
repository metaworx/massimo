// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
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
// in the log, on the stream and for the notification rules.

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

// emitBattery makes an event of each change of a level: in the log, on
// the stream and for the notification rules; low at warn, the others at
// info.
func (s *Server) emitBattery(rt *runtime, changes []battery.Change) {
	for _, c := range changes {
		rf, err := cube.ParseRFAddress(c.Device)
		if err != nil {
			continue
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
		rt.emit(ev)
		s.notifyEvent(rt, ev)
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
// the last batteryHistory into the tracker, oldest first, so that a
// level outlives a restart; its changes make no events, as they are not
// news. A Cube without a row in the store has no log to replay.
func (s *Server) rebuildBattery(ctx context.Context, rt *runtime) {
	id := rt.storeID.Load()
	if rt.store == nil || id == 0 {
		return
	}
	now := time.Now()
	var rows []store.Event
	q := store.EventQuery{CubeID: id, Kind: "state", Source: "device", Since: now.Add(-batteryHistory), Limit: store.MaxEventsPage}
	for {
		page, err := rt.store.Events(ctx, q)
		if err != nil {
			s.log.Warn("battery levels not rebuilt from the log", "cube", rt.cubeSerial(), "error", err)
			return
		}
		rows = append(rows, page...)
		if len(page) < store.MaxEventsPage {
			break
		}
		q.BeforeID = page[len(page)-1].ID
	}
	for i := len(rows) - 1; i >= 0; i-- {
		var row struct {
			State view.Device `json:"state"`
		}
		if json.Unmarshal(rows[i].Data, &row) != nil {
			continue
		}
		if r, ok := batteryReport(row.State); ok && rows[i].Device != "" {
			rt.battery.Report(rows[i].Device, rows[i].Time, r)
		}
	}
	rt.battery.Tick(now)
}
