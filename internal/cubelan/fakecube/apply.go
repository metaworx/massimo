// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"encoding/base64"
	"fmt"
	"math"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// The fake Cube applies the radio commands it acknowledges, when Apply is
// set: a setpoint, mode or boost, a day's programme, the preset
// temperatures, the valve settings, the key lock and a wall thermostat's
// display. Each changes the device's entry in the L line, or its C line,
// as the real Cube shows it once the device confirmed the command; a boost
// ends by itself after the device's boost time, or BoostFor. Once anything
// changed, a new connection's greeting carries the changed lines too.
//
// The fake applies a command to every thermostat it addresses, as the
// devices take it: a room command to each thermostat whose C line names
// the room, a direct one to the device alone.

// boost is a running boost of one device: what it returns to.
type boost struct {
	mode  cube.Mode
	valve byte
	timer *time.Timer
}

// Offsets of the L entry (with its length byte) the fake changes.
const (
	lFlags2   = 6  // battery, link error, lock, mode
	lValve    = 7  // valve of a radiator thermostat; display flag of a wall thermostat
	lSetpoint = 8  // setpoint in half degrees; bit 7 a wall thermostat's temperature's ninth bit
	lDate     = 9  // vacation end date, or a radiator thermostat's temperature
	lTime     = 11 // vacation end time
	lWallTemp = 12 // a wall thermostat's temperature, low byte

	weekProgramSize = cube.WeekProgramSize
	radiatorProgram = 0x1d // the week programme in a radiator thermostat's C payload
	wallProgram     = 0x16 // the week programme in a wall thermostat's C payload
)

// apply carries out a radio command the fake acknowledged, after
// ApplyAfter when it is set.
func (s *Server) apply(m cube.Message) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(m.Data))
	if err != nil || len(raw) < 11 {
		return
	}
	if s.ApplyAfter > 0 {
		time.AfterFunc(s.ApplyAfter, func() { s.applyNow(raw) })
		return
	}
	s.applyNow(raw)
}

// applyNow changes the addressed devices' lines for one radio command.
func (s *Server) applyNow(raw []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cmd, room := raw[2], raw[9]
	var targets []cube.RFAddress
	if raw[1]&0x04 != 0 {
		targets = s.roomThermostatsLocked(room)
	} else {
		targets = []cube.RFAddress{cube.RFAddressFromBytes(raw[6:9])}
	}
	for _, rf := range targets {
		switch cmd {
		case cube.RFSetTemperature:
			s.setTemperatureLocked(rf, raw[10:])
		case cube.RFSetWeekProgram:
			s.setDayLocked(rf, raw[10:])
		case cube.RFSetTemperatures:
			s.setTemperaturesLocked(rf, raw[10:])
		case cube.RFSetValveConfig:
			s.setValveLocked(rf, raw[10:])
		case cube.RFLockControls:
			s.editEntryLocked(rf, func(e []byte, _ cube.DeviceType) {
				e[lFlags2] &^= 0x20
				if raw[10]&0x01 != 0 {
					e[lFlags2] |= 0x20
				}
			})
		case cube.RFSetDisplayMode:
			s.editEntryLocked(rf, func(e []byte, t cube.DeviceType) {
				if t == cube.DeviceWallThermostat && len(e) > lValve {
					e[lValve] = 0x01
					if raw[10]&0x04 != 0 {
						e[lValve] = 0x04
					}
				}
			})
		}
	}
}

// setTemperatureLocked applies 0x40: mode and setpoint, a vacation's end,
// or a boost.
func (s *Server) setTemperatureLocked(rf cube.RFAddress, p []byte) {
	mode, half := cube.Mode(p[0]>>6), p[0]&0x3f
	if mode == cube.ModeBoost {
		s.startBoostLocked(rf)
		return
	}
	s.endBoostLocked(rf, false)
	if mode == cube.ModeAuto && half == 0 {
		half = s.programHalfLocked(rf, time.Now())
	}
	s.editEntryLocked(rf, func(e []byte, t cube.DeviceType) {
		if len(e) <= lTime {
			return
		}
		was := cube.Mode(e[lFlags2] & 0x03)
		e[lFlags2] = e[lFlags2]&^0x03 | byte(mode)
		if half != 0 {
			e[lSetpoint] = e[lSetpoint]&0xc0 | half
		}
		switch {
		case mode == cube.ModeVacation && len(p) >= 4:
			copy(e[lDate:lTime+1], p[1:4])
		case was == cube.ModeVacation:
			// The bytes carried the end date; the measured temperature
			// comes with the device's next report.
			e[lDate], e[lDate+1], e[lTime] = 0, 0, 0
		}
	})
}

// startBoostLocked starts a boost: mode 3, a radiator thermostat's valve
// at its boost opening, the previous mode and valve back after the boost
// time.
func (s *Server) startBoostLocked(rf cube.RFAddress) {
	c := s.configLocked(rf)
	if c == nil || !c.Type.IsThermostat() {
		return
	}
	minutes, valvePct := 5, 80
	switch {
	case c.Thermostat != nil:
		minutes, valvePct = c.Thermostat.BoostMinutes, c.Thermostat.BoostValvePct
	case c.Wall != nil:
		minutes, valvePct = c.Wall.BoostMinutes, c.Wall.BoostValvePct
	}
	d := s.BoostFor
	if d == 0 {
		d = time.Duration(minutes) * time.Minute
	}
	if s.boosts == nil {
		s.boosts = map[cube.RFAddress]*boost{}
	}
	b := s.boosts[rf]
	if b == nil {
		b = &boost{}
		s.editEntryLocked(rf, func(e []byte, _ cube.DeviceType) {
			b.mode = cube.Mode(e[lFlags2] & 0x03)
			if len(e) > lValve {
				b.valve = e[lValve]
			}
		})
		s.boosts[rf] = b
	} else if b.timer != nil {
		b.timer.Stop()
	}
	s.editEntryLocked(rf, func(e []byte, t cube.DeviceType) {
		e[lFlags2] = e[lFlags2]&^0x03 | byte(cube.ModeBoost)
		if t.IsRadiatorThermostat() && len(e) > lValve {
			e[lValve] = byte(valvePct)
		}
	})
	b.timer = time.AfterFunc(d, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.boosts[rf] == b {
			s.endBoostLocked(rf, true)
		}
	})
}

// endBoostLocked ends a device's boost: with restore its mode and valve
// go back to what they were, as when the boost time is over; without,
// another command replaced the boost.
func (s *Server) endBoostLocked(rf cube.RFAddress, restore bool) {
	b := s.boosts[rf]
	if b == nil {
		return
	}
	delete(s.boosts, rf)
	if b.timer != nil {
		b.timer.Stop()
	}
	s.editEntryLocked(rf, func(e []byte, t cube.DeviceType) {
		if restore {
			e[lFlags2] = e[lFlags2]&^0x03 | byte(b.mode)
		}
		if t.IsRadiatorThermostat() && len(e) > lValve {
			e[lValve] = b.valve
		}
	})
}

// programHalfLocked is the temperature, in half degrees, that rf's week
// programme gives at now: what auto mode without a temperature sets.
func (s *Server) programHalfLocked(rf cube.RFAddress, now time.Time) byte {
	c := s.configLocked(rf)
	if c == nil {
		return 0
	}
	var w cube.WeekProgram
	switch {
	case c.Thermostat != nil:
		w = c.Thermostat.WeekProgram
	case c.Wall != nil:
		w = c.Wall.WeekProgram
	default:
		return 0
	}
	day := w[(int(now.Weekday())+1)%7] // the device's week starts on Saturday
	minute := now.Hour()*60 + now.Minute()
	for _, p := range day {
		if minute < p.Until {
			return byte(math.Round(p.Temp * 2))
		}
	}
	return 0
}

// setDayLocked applies 0x10: one day's switch points into the C line, the
// second telegram of a long day from the eighth slot on.
func (s *Server) setDayLocked(rf cube.RFAddress, p []byte) {
	s.editConfigLocked(rf, func(c []byte, t cube.DeviceType) {
		base := radiatorProgram
		if t == cube.DeviceWallThermostat {
			base = wallProgram
		}
		day, slot := int(p[0]&0x07), 0
		if p[0]&0x08 != 0 {
			slot = 7
		}
		if day > 6 {
			return
		}
		words := p[1:]
		for i := 0; i+1 < len(words) && slot < cube.DaySlots; i, slot = i+2, slot+1 {
			off := base + (day*cube.DaySlots+slot)*2
			if off+2 <= len(c) {
				copy(c[off:off+2], words[i:i+2])
			}
		}
	})
}

// setTemperaturesLocked applies 0x11: comfort, eco, maximum, minimum,
// offset, window-open temperature and minutes, where the C line keeps
// them.
func (s *Server) setTemperaturesLocked(rf cube.RFAddress, p []byte) {
	if len(p) < 7 {
		return
	}
	s.editConfigLocked(rf, func(c []byte, t cube.DeviceType) {
		switch {
		case t.IsRadiatorThermostat() && len(c) >= 0x19:
			copy(c[0x12:0x19], p[:7])
		case t == cube.DeviceWallThermostat && len(c) >= 0x16:
			copy(c[0x12:0x16], p[:4])
			if rest := wallProgram + weekProgramSize; len(c) >= rest+2 {
				c[rest], c[rest+1] = p[4], p[5]
			}
		}
	})
}

// setValveLocked applies 0x12: boost, descaling and valve limits, where
// the C line keeps them.
func (s *Server) setValveLocked(rf cube.RFAddress, p []byte) {
	if len(p) < 4 {
		return
	}
	s.editConfigLocked(rf, func(c []byte, t cube.DeviceType) {
		switch {
		case t.IsRadiatorThermostat() && len(c) >= 0x1d:
			copy(c[0x19:0x1d], p[:4])
		case t == cube.DeviceWallThermostat:
			if rest := wallProgram + weekProgramSize; len(c) >= rest+3 {
				c[rest+2] = p[0]
			}
		}
	})
}

// roomThermostatsLocked lists the thermostats whose C line names room.
func (s *Server) roomThermostatsLocked(room uint8) []cube.RFAddress {
	var out []cube.RFAddress
	for _, rf := range s.addressesLocked() {
		if s.removed[rf] {
			continue
		}
		if c := s.configLocked(rf); c != nil && c.RoomID == room && c.Type.IsThermostat() {
			out = append(out, rf)
		}
	}
	return out
}

// addressesLocked lists the devices of the current L line, in its order.
func (s *Server) addressesLocked() []cube.RFAddress {
	var out []cube.RFAddress
	for _, e := range entries(s.listLocked()) {
		out = append(out, cube.RFAddressFromBytes(e[1:4]))
	}
	return out
}

// listLocked is the L line the fake answers l: with.
func (s *Server) listLocked() []byte {
	l := s.list
	if l == nil {
		for _, g := range s.Greeting {
			if g.Type == 'L' {
				g := g
				l = &g
			}
		}
	}
	if l == nil {
		return nil
	}
	raw, _ := base64.StdEncoding.DecodeString(l.Data)
	return raw
}

// entries splits an L payload into its entries, each with its length byte.
func entries(raw []byte) [][]byte {
	var out [][]byte
	for i := 0; i < len(raw); {
		end := i + 1 + int(raw[i])
		if end > len(raw) || int(raw[i]) < 6 {
			break
		}
		out = append(out, raw[i:end])
		i = end
	}
	return out
}

// editEntryLocked changes rf's entry in the L line the fake answers with.
func (s *Server) editEntryLocked(rf cube.RFAddress, edit func(e []byte, t cube.DeviceType)) {
	raw := s.listLocked()
	if raw == nil {
		return
	}
	var t cube.DeviceType
	if c := s.configLocked(rf); c != nil {
		t = c.Type
	}
	for _, e := range entries(raw) {
		if cube.RFAddressFromBytes(e[1:4]) == rf {
			if t == 0 {
				t = guessType(len(e) - 1)
			}
			edit(e, t)
		}
	}
	s.list = &cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)}
	s.live = true
}

// guessType is a device's type by the length of its L entry, for a
// device without a C line.
func guessType(n int) cube.DeviceType {
	switch {
	case n >= 12:
		return cube.DeviceWallThermostat
	case n >= 11:
		return cube.DeviceRadiatorThermostat
	default:
		return cube.DeviceShutterContact
	}
}

// configRawLocked is rf's C payload: the one set with SetConfig or
// changed, else the greeting's.
func (s *Server) configRawLocked(rf cube.RFAddress) []byte {
	line, ok := s.configs[rf.String()]
	if !ok {
		for _, g := range s.Greeting {
			if g.Type == 'C' && strings.HasPrefix(strings.ToLower(g.Data), rf.String()+",") {
				line, ok = g, true
			}
		}
	}
	if !ok {
		return nil
	}
	_, enc, _ := strings.Cut(line.Data, ",")
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil
	}
	return raw
}

// configLocked is rf's configuration, nil where the fake has none.
func (s *Server) configLocked(rf cube.RFAddress) *cube.DeviceConfig {
	raw := s.configRawLocked(rf)
	if raw == nil {
		return nil
	}
	c, err := cube.DecodeConfig(raw)
	if err != nil {
		return nil
	}
	return c
}

// editConfigLocked changes rf's C line, which c: and a new connection's
// greeting then answer with.
func (s *Server) editConfigLocked(rf cube.RFAddress, edit func(c []byte, t cube.DeviceType)) {
	raw := s.configRawLocked(rf)
	if len(raw) < 5 {
		return
	}
	edit(raw, cube.DeviceType(raw[4]))
	if s.configs == nil {
		s.configs = map[string]cube.Message{}
	}
	s.configs[rf.String()] = cube.Message{Type: 'C', Data: rf.String() + "," + base64.StdEncoding.EncodeToString(raw)}
	s.live = true
}

// liveGreetingLocked is the greeting with the lines the fake changed: the
// current L line and C lines, without removed devices.
func (s *Server) liveGreetingLocked() []cube.Message {
	out := append([]cube.Message(nil), s.Greeting...)
	if !s.live {
		return out
	}
	for i, g := range out {
		switch g.Type {
		case 'L':
			if s.list != nil {
				out[i] = *s.list
			}
		case 'C':
			addr, _, _ := strings.Cut(g.Data, ",")
			if c, ok := s.configs[strings.ToLower(addr)]; ok {
				out[i] = c
			}
		}
	}
	return out
}

// encodeActual writes a measured temperature into an L entry: a radiator
// thermostat's in bytes 9 and 10, a wall thermostat's in byte 12 with the
// ninth bit in byte 8.
func encodeActual(e []byte, t cube.DeviceType, c float64) error {
	v := int(math.Round(c * 10))
	if v < 0 || v > 511 {
		return fmt.Errorf("temperature %.1f out of range", c)
	}
	switch {
	case t == cube.DeviceWallThermostat && len(e) > lWallTemp:
		e[lSetpoint] = e[lSetpoint]&0x7f | byte(v>>8)<<7
		e[lWallTemp] = byte(v)
	case t.IsRadiatorThermostat() && len(e) > lDate+1:
		e[lDate] = e[lDate]&^0x01 | byte(v>>8)&0x01
		e[lDate+1] = byte(v)
	default:
		return fmt.Errorf("a %s reports no temperature", t)
	}
	return nil
}
