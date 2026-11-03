// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

import (
	"fmt"
	"strings"
	"time"
)

// Simple line commands.
var (
	CmdList            = Message{Type: 'l'} // request a fresh L message
	CmdQuit            = Message{Type: 'q'} // close the connection cleanly
	CmdGetNTP          = Message{Type: 'f'} // request the F message
	CmdCancelInclusion = Message{Type: 'x'} // stop pairing mode
)

// CmdInclusion starts pairing mode. A zero timeout means "no timeout".
func CmdInclusion(timeout time.Duration) Message {
	if timeout <= 0 {
		return Message{Type: 'n'}
	}
	return Message{Type: 'n', Data: fmt.Sprintf("%04x", int(timeout.Seconds()))}
}

// CmdSetNTP sets the Cube's NTP servers.
func CmdSetNTP(servers ...string) Message {
	return Message{Type: 'f', Data: strings.Join(servers, ",")}
}

// CmdSetPortalURL points the Cube's portal client at url, e.g.
// "http://192.168.1.10:8080". This changes persistent Cube configuration.
func CmdSetPortalURL(url string) Message { return Message{Type: 'u', Data: url} }

// CmdEncrypt asks the Cube to encrypt data with its portal key (AES-128-CBC).
func CmdEncrypt(data []byte) Message { return Message{Type: 'e', Data: b64.EncodeToString(data)} }

// CmdDecrypt asks the Cube to decrypt data with its portal key.
func CmdDecrypt(data []byte) Message { return Message{Type: 'd', Data: b64.EncodeToString(data)} }

// WakeupScope selects the target of a wakeup command.
type WakeupScope byte

const (
	WakeupAll    WakeupScope = 'A'
	WakeupRoom   WakeupScope = 'G'
	WakeupDevice WakeupScope = 'D'
)

// CmdWakeup keeps devices listening for the given duration, which makes
// subsequent commands faster. target is a room id (WakeupRoom) or a radio
// address (WakeupDevice) and is ignored for WakeupAll.
//
// The docs are ambiguous about the unit; the example "1e" is described as
// 30 seconds, so seconds are used.
func CmdWakeup(seconds uint8, scope WakeupScope, target string) Message {
	if scope == WakeupAll {
		return Message{Type: 'z', Data: fmt.Sprintf("%02x,A", seconds)}
	}
	return Message{Type: 'z', Data: fmt.Sprintf("%02x,%c,%s", seconds, scope, target)}
}

// CmdDeleteDevices removes devices from the Cube's memory.
func CmdDeleteDevices(force bool, addrs ...RFAddress) Message {
	var raw []byte
	for _, a := range addrs {
		b := a.Bytes()
		raw = append(raw, b[:]...)
	}
	f := 0
	if force {
		f = 1
	}
	return Message{Type: 't', Data: fmt.Sprintf("%02x,%d,%s", len(addrs), f, b64.EncodeToString(raw))}
}

// cubeEpoch is the reference point of the v command's clock value.
var cubeEpoch = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// CmdSetTime sets the Cube's clock and daylight-saving rules.
func CmdSetTime(winter, summer TimeZoneRule, now time.Time) (Message, error) {
	w, err := winter.Encode()
	if err != nil {
		return Message{}, err
	}
	s, err := summer.Encode()
	if err != nil {
		return Message{}, err
	}
	secs := now.UTC().Sub(cubeEpoch) / time.Second
	if secs < 0 || secs > 0xffffffff {
		return Message{}, fmt.Errorf("time %s out of range", now)
	}
	return Message{Type: 'v', Data: fmt.Sprintf("%s,%08x", b64.EncodeToString(append(w, s...)), uint32(secs))}, nil
}

// Radio command codes used in s messages.
const (
	RFSetTemperature  = 0x40
	RFSetWeekProgram  = 0x10
	RFSetTemperatures = 0x11
	RFSetValveConfig  = 0x12
	RFAddLinkPartner  = 0x20
	RFRemoveLink      = 0x21
	RFSetGroupAddress = 0x22
	RFRemoveGroup     = 0x23
	RFSetDisplayMode  = 0x82
)

// Target addresses a radio command to a single device or to a room.
type Target struct {
	RFAddress RFAddress // device address; ignored by devices for room commands
	RoomID    uint8
	Room      bool // address all devices of the room
}

func (t Target) header(cmd byte) []byte {
	flags := byte(0x00)
	if t.Room {
		flags = 0x04
	}
	a := t.RFAddress.Bytes()
	return []byte{0x00, flags, cmd, 0, 0, 0, a[0], a[1], a[2], t.RoomID}
}

func send(payload []byte) Message { return Message{Type: 's', Data: b64.EncodeToString(payload)} }

// CmdSetTemperature sets setpoint and mode. For ModeVacation, until must be
// set; for ModeAuto a zero temperature lets the device follow its week program.
func CmdSetTemperature(t Target, temp float64, mode Mode, until Until) (Message, error) {
	var v uint8
	if !(mode == ModeAuto && temp == 0) {
		var err error
		if v, err = EncodeTemp(temp); err != nil {
			return Message{}, err
		}
	}
	p := append(t.header(RFSetTemperature), v|byte(mode)<<6)
	if mode == ModeVacation {
		if until.IsZero() {
			return Message{}, fmt.Errorf("vacation mode needs an end time")
		}
		u, err := encodeUntil(until)
		if err != nil {
			return Message{}, err
		}
		p = append(p, u...)
	}
	return send(p), nil
}

// CmdSetDayProgram writes the switch points of one weekday. Programs with
// more than 7 points need two telegrams; the first one is padded with its
// last point, as the original software does.
func CmdSetDayProgram(t Target, day Weekday, prog DayProgram) ([]Message, error) {
	if day > Friday {
		return nil, fmt.Errorf("invalid weekday %d", day)
	}
	if err := prog.Validate(); err != nil {
		return nil, err
	}
	words := make([]uint16, len(prog))
	for i, p := range prog {
		words[i], _ = encodeSwitchPoint(p) // validated above
	}
	build := func(dayByte byte, w []uint16) Message {
		p := append(t.header(RFSetWeekProgram), dayByte)
		for _, v := range w {
			p = append(p, byte(v>>8), byte(v))
		}
		return send(p)
	}
	first := words[:min(7, len(words))]
	for len(first) < 7 {
		first = append(first, first[len(first)-1])
	}
	out := []Message{build(byte(day), first)}
	if len(words) > 7 {
		out = append(out, build(byte(day)|0x08, words[7:]))
	}
	return out, nil
}

// TemperatureSettings are the preset temperatures of a thermostat.
type TemperatureSettings struct {
	Comfort, Eco, Max, Min float64
	Offset                 float64 // -3.5 .. +3.5 °C
	WindowOpen             float64
	WindowOpenMinutes      int // multiple of 5
}

// CmdSetTemperatures writes the preset temperatures (radio command 0x11).
func CmdSetTemperatures(t Target, s TemperatureSettings) (Message, error) {
	p := t.header(RFSetTemperatures)
	for _, c := range []float64{s.Comfort, s.Eco, s.Max, s.Min} {
		v, err := EncodeTemp(c)
		if err != nil {
			return Message{}, err
		}
		p = append(p, v)
	}
	if s.Offset < -3.5 || s.Offset > 3.5 || s.Offset*2 != float64(int(s.Offset*2)) {
		return Message{}, fmt.Errorf("offset %.1f: must be a multiple of 0.5 between -3.5 and 3.5", s.Offset)
	}
	p = append(p, uint8((s.Offset+3.5)*2))
	w, err := EncodeTemp(s.WindowOpen)
	if err != nil {
		return Message{}, err
	}
	if s.WindowOpenMinutes < 0 || s.WindowOpenMinutes > 255*5 || s.WindowOpenMinutes%5 != 0 {
		return Message{}, fmt.Errorf("window open duration %d min: must be a multiple of 5", s.WindowOpenMinutes)
	}
	return send(append(p, w, byte(s.WindowOpenMinutes/5))), nil
}

// ValveSettings configures boost, decalcification and valve limits.
type ValveSettings struct {
	BoostMinutes   int // 0, 5, 10 … 30, or 60
	BoostValvePct  int // multiple of 5
	DecalcDay      Weekday
	DecalcHour     int
	MaxValvePct    int
	ValveOffsetPct int
}

// CmdSetValveConfig writes valve settings (radio command 0x12).
func CmdSetValveConfig(t Target, s ValveSettings) (Message, error) {
	var dur int
	switch {
	case s.BoostMinutes == 60:
		dur = 7
	case s.BoostMinutes >= 0 && s.BoostMinutes <= 30 && s.BoostMinutes%5 == 0:
		dur = s.BoostMinutes / 5
	default:
		return Message{}, fmt.Errorf("boost duration %d min: must be 0..30 in steps of 5, or 60", s.BoostMinutes)
	}
	if s.BoostValvePct < 0 || s.BoostValvePct > 100 || s.BoostValvePct%5 != 0 {
		return Message{}, fmt.Errorf("boost valve %d%%: must be 0..100 in steps of 5", s.BoostValvePct)
	}
	if s.DecalcDay > Friday || s.DecalcHour < 0 || s.DecalcHour > 23 {
		return Message{}, fmt.Errorf("invalid decalcification time")
	}
	for _, v := range []int{s.MaxValvePct, s.ValveOffsetPct} {
		if v < 0 || v > 100 {
			return Message{}, fmt.Errorf("valve percentage %d out of range", v)
		}
	}
	p := append(t.header(RFSetValveConfig),
		byte(dur<<5|s.BoostValvePct/5),
		byte(s.DecalcDay)<<5|byte(s.DecalcHour),
		pctTo255(s.MaxValvePct),
		pctTo255(s.ValveOffsetPct))
	return send(p), nil
}

// CmdLinkPartner links (add=true) or unlinks a partner device, e.g. a
// shutter contact or wall thermostat to a radiator thermostat.
func CmdLinkPartner(t Target, add bool, partner RFAddress, partnerType DeviceType) Message {
	cmd := byte(RFRemoveLink)
	if add {
		cmd = RFAddLinkPartner
	}
	a := partner.Bytes()
	return send(append(t.header(cmd), a[0], a[1], a[2], byte(partnerType)))
}

// CmdSetGroup assigns (or removes) a device's room/group address.
func CmdSetGroup(dev RFAddress, room uint8, remove bool) Message {
	cmd := byte(RFSetGroupAddress)
	if remove {
		cmd = RFRemoveGroup
	}
	return send(append(Target{RFAddress: dev}.header(cmd), room))
}

// CmdSetDisplayActual makes a wall thermostat show the measured (true) or
// the set temperature (false).
func CmdSetDisplayActual(t Target, actual bool) Message {
	v := byte(0)
	if actual {
		v = 0x04
	}
	return send(append(t.header(RFSetDisplayMode), v))
}
