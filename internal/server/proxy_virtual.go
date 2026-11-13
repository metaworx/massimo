// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net/http"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/proxy"
	"go.metaworx.ch/massimo/internal/view"
)

// The virtual wall thermostats for the LAN proxy's clients (docs/proxy.md,
// "Virtual wall thermostats"): with the Cube's option proxy_virtual on,
// the proxy shows each room's virtual wall thermostat in place of the
// real wall thermostats, as the API's view "virtual" does. The option is
// kept in the store's cube_options, off by default.

// loadOptions reads the Cube's options from the store.
func (rt *runtime) loadOptions(ctx context.Context) {
	id := rt.storeID.Load()
	if rt.store == nil || id == 0 {
		return
	}
	if o, err := rt.store.CubeOptions(ctx, id); err == nil {
		rt.proxyVirtual.Store(o.ProxyVirtual)
	}
}

// setProxyVirtual keeps the option proxy_virtual for a Cube's row. A
// client connected before sees the change at its next connection.
func (s *Server) setProxyVirtual(r *http.Request, rt *runtime, cubeID int64, on bool) error {
	o, err := rt.store.CubeOptions(r.Context(), cubeID)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	o.ProxyVirtual = on
	if err := rt.store.SetCubeOptions(r.Context(), cubeID, o, principalOf(r).userID()); err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	rt.proxyVirtual.Store(on)
	return nil
}

// proxyVirtuals is the virtual wall thermostats the proxy shows, none
// while the option is off.
func (s *Server) proxyVirtuals(rt *runtime) []proxy.Virtual {
	if !rt.proxyVirtual.Load() {
		return nil
	}
	snap := rt.client.Snapshot()
	md := snap.Metadata()
	if md == nil {
		return nil
	}
	vwts, _ := view.RoomThermostats(snap, rt.hidden(), s.StaleAfter(), rt.roomInfo())
	var out []proxy.Virtual
	for _, room := range rt.hidden().VisibleRooms(md) {
		d, ok := vwts[room.ID]
		if !ok {
			continue
		}
		rf, err := cube.ParseRFAddress(d.RFAddress)
		if err != nil {
			continue
		}
		radiators, walls := roomThermostats(rt, room.ID)
		v := proxy.Virtual{Room: room, RF: rf, Serial: d.Serial}
		if len(walls) > 0 {
			v.Wall = walls[0]
		}
		if members := append(radiators, walls...); len(members) > 0 {
			v.Member = members[0]
		}
		v.Entry = virtualEntry(rf, d, snap.Hello())
		v.Config = virtualConfig(rf, room.ID, d.Serial, s.virtualDetail(rt, room).Wall)
		out = append(out, v)
	}
	return out
}

// virtualEntry is a virtual wall thermostat's L entry, laid out as a
// wall thermostat's (docs/cube-protocol.md): the flags, the display,
// the setpoint and the measured temperature with its ninth bit.
func virtualEntry(rf cube.RFAddress, d view.Device, h *cube.Hello) []byte {
	a := rf.Bytes()
	e := []byte{0x0c, a[0], a[1], a[2], 0x00, 0x02, 0x10, 0x01, 0, 0, 0, 0, 0}
	st := d.State
	if !d.Reported || st == nil {
		return e
	}
	e[5] |= 0x10 // the Cube holds a state for it
	flag := func(on bool, bit byte) {
		if on {
			e[6] |= bit
		}
	}
	flag(st.BatteryLow, 0x80)
	flag(st.LinkError, 0x40)
	flag(st.PanelLocked, 0x20)
	e[6] |= byte(st.Mode) & 0x03
	if st.Setpoint != nil {
		if v, err := cube.EncodeTemp(*st.Setpoint); err == nil {
			e[8] = v
		}
	}
	if st.ActualTemp != nil {
		t := int(*st.ActualTemp*10 + 0.5)
		if t >= 0 && t < 512 {
			e[8] |= byte(t>>8) << 7
			e[12] = byte(t)
		}
	}
	if st.Until != nil && st.Mode == cube.ModeVacation {
		e[9], e[10], e[11] = untilBytes(*st.Until)
	}
	return e
}

// untilBytes is a vacation's end as an L entry holds it.
func untilBytes(u cube.Until) (byte, byte, byte) {
	return byte((u.Month&0x0e)<<4) | byte(u.Day), byte((u.Month&0x01)<<7) | byte(u.Year-2000), byte(u.Minutes / 30)
}

// virtualConfig is a virtual wall thermostat's C payload, laid out as a
// wall thermostat's: the header with its address, room and serial, the
// room's preset temperatures and week programme, then offset, window-open
// temperature and boost.
func virtualConfig(rf cube.RFAddress, room uint8, serial string, w *cube.WallConfig) []byte {
	const size = 0x16 + cube.WeekProgramSize + 3
	c := make([]byte, size)
	c[0] = size - 1
	a := rf.Bytes()
	copy(c[1:4], a[:])
	c[4], c[5], c[6] = byte(cube.DeviceWallThermostat), room, 0x10
	copy(c[8:18], serial)
	if w == nil {
		return c
	}
	for i, t := range []float64{w.ComfortTemp, w.EcoTemp, w.MaxSetpoint, w.MinSetpoint} {
		if v, err := cube.EncodeTemp(t); err == nil {
			c[0x12+i] = v
		}
	}
	if prog, err := cube.EncodeWeekProgram(w.WeekProgram); err == nil {
		copy(c[0x16:], prog)
	}
	rest := 0x16 + cube.WeekProgramSize
	c[rest] = byte((w.TempOffset + 3.5) * 2)
	if v, err := cube.EncodeTemp(w.WindowOpenTemp); err == nil {
		c[rest+1] = v
	}
	dur := w.BoostMinutes / 5
	if w.BoostMinutes == 60 {
		dur = 7
	}
	c[rest+2] = byte(dur<<5) | byte(w.BoostValvePct/5)
	return c
}
