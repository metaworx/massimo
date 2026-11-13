// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
)

// DeviceConfig is the decoded payload of a C message. The common header is
// always present; exactly one of the type-specific parts is set.
type DeviceConfig struct {
	RFAddress  RFAddress  `json:"rf_address"`
	Type       DeviceType `json:"type"`
	RoomID     uint8      `json:"room_id"`
	Firmware   uint8      `json:"firmware"`
	TestResult uint8      `json:"test_result"`
	Serial     string     `json:"serial"`

	Cube       *CubeConfig       `json:"cube,omitempty"`
	Thermostat *ThermostatConfig `json:"thermostat,omitempty"`
	Wall       *WallConfig       `json:"wall_thermostat,omitempty"`

	Raw []byte `json:"raw"` // complete decoded payload, kept for backups
}

// CubeConfig holds the Cube's own settings.
type CubeConfig struct {
	PortalEnabled  bool         `json:"portal_enabled"`
	PortalURL      string       `json:"portal_url"`
	PushButtonUp   uint8        `json:"push_button_up"`
	PushButtonDown uint8        `json:"push_button_down"`
	Winter         TimeZoneRule `json:"tz_winter"`
	Summer         TimeZoneRule `json:"tz_summer"`
}

// TimeZoneRule is one of the two daylight-saving rules stored in the Cube.
type TimeZoneRule struct {
	Name          string `json:"name"` // up to 5 characters, e.g. "CET"
	Month         uint8  `json:"month"`
	Weekday       uint8  `json:"weekday"`
	Hour          uint8  `json:"hour"`
	OffsetSeconds int32  `json:"offset_seconds"`
}

// TimeZoneRuleSize is the encoded size of a TimeZoneRule.
const TimeZoneRuleSize = 12

func decodeTimeZoneRule(b []byte) TimeZoneRule {
	return TimeZoneRule{
		Name:          strings.TrimRight(string(b[0:5]), "\x00"),
		Month:         b[5],
		Weekday:       b[6],
		Hour:          b[7],
		OffsetSeconds: int32(binary.BigEndian.Uint32(b[8:12])),
	}
}

// Encode returns the 12-byte representation used in C messages and the v command.
func (z TimeZoneRule) Encode() ([]byte, error) {
	if len(z.Name) > 5 {
		return nil, fmt.Errorf("time zone name %q longer than 5 characters", z.Name)
	}
	b := make([]byte, TimeZoneRuleSize)
	copy(b, z.Name)
	b[5], b[6], b[7] = z.Month, z.Weekday, z.Hour
	binary.BigEndian.PutUint32(b[8:], uint32(z.OffsetSeconds))
	return b, nil
}

// ThermostatConfig holds the settings of a radiator thermostat.
type ThermostatConfig struct {
	ComfortTemp    float64     `json:"comfort_temp"`
	EcoTemp        float64     `json:"eco_temp"`
	MaxSetpoint    float64     `json:"max_setpoint"`
	MinSetpoint    float64     `json:"min_setpoint"`
	TempOffset     float64     `json:"temp_offset"`
	WindowOpenTemp float64     `json:"window_open_temp"`
	WindowOpenMins int         `json:"window_open_minutes"`
	BoostMinutes   int         `json:"boost_minutes"`
	BoostValvePct  int         `json:"boost_valve_pct"`
	DecalcDay      Weekday     `json:"decalc_day"`
	DecalcHour     int         `json:"decalc_hour"`
	MaxValvePct    int         `json:"max_valve_pct"`
	ValveOffsetPct int         `json:"valve_offset_pct"`
	WeekProgram    WeekProgram `json:"week_program"`
}

// WallConfig holds the settings of a wall thermostat. The three bytes
// after the week program (offset, window-open temperature, boost) follow
// FHEM's reading and match a wall thermostat seen in 2026; the original
// software writes wall thermostats with the same 0x11 and 0x12 commands
// as radiator thermostats.
type WallConfig struct {
	ComfortTemp    float64 `json:"comfort_temp"`
	EcoTemp        float64 `json:"eco_temp"`
	MaxSetpoint    float64 `json:"max_setpoint"`
	MinSetpoint    float64 `json:"min_setpoint"`
	TempOffset     float64 `json:"temp_offset"`
	WindowOpenTemp float64 `json:"window_open_temp"`
	BoostMinutes   int     `json:"boost_minutes"`
	BoostValvePct  int     `json:"boost_valve_pct"`
	// DisplayActual: the display shows the measured temperature instead
	// of the setpoint. Not part of the C record; the device reports it in
	// its L entry and backups fill it in from there.
	DisplayActual *bool       `json:"display_actual,omitempty"`
	WeekProgram   WeekProgram `json:"week_program"`
	Extra         []byte      `json:"extra,omitempty"` // bytes beyond those, if any
}

// Offsets inside the decoded C payload (including the leading length byte).
const (
	cfgHeaderLen      = 0x12
	cubePortalEnabled = 0x12
	cubePushUp        = 0x1e
	cubePushDown      = 0x3f
	cubePortalURL     = 0x55
	cubePortalURLEnd  = 0xd6
	// The docs' table says 0xd5/0xe1, but in the docs' own hexdump the
	// rules start at 0xd6 ("CET") and 0xe2 ("CEST"), ending exactly at the
	// payload length of 0xee bytes.
	cubeTZWinter = 0xd6
	cubeTZSummer = 0xe2

	thermoWeekProgram = 0x1d
	// The protocol docs list 0x1d for wall thermostats too, but also an
	// unknown 3-byte field at 0xcc. 0x16 + 182 = 0xcc is the only layout
	// consistent with both, and it matches FHEM, so the week program of a
	// wall thermostat is read from 0x16. Not yet verified on hardware.
	wallWeekProgram = 0x16
)

// ParseConfig parses the payload of a C message ("<rf>,<base64>").
func ParseConfig(data string) (*DeviceConfig, error) {
	addr, enc, ok := strings.Cut(data, ",")
	if !ok {
		return nil, fmt.Errorf("C: missing comma")
	}
	rf, err := ParseRFAddress(addr)
	if err != nil {
		return nil, fmt.Errorf("C: %w", err)
	}
	b, err := decodeB64("C", enc)
	if err != nil {
		return nil, err
	}
	c, err := DecodeConfig(b)
	if err != nil {
		return nil, err
	}
	if c.RFAddress != rf {
		return nil, fmt.Errorf("C: address %s in header does not match payload address %s", rf, c.RFAddress)
	}
	return c, nil
}

// DecodeConfig decodes a binary C payload.
func DecodeConfig(b []byte) (*DeviceConfig, error) {
	if len(b) < cfgHeaderLen {
		return nil, fmt.Errorf("C: payload too short (%d bytes)", len(b))
	}
	c := &DeviceConfig{
		RFAddress:  RFAddressFromBytes(b[1:4]),
		Type:       DeviceType(b[4]),
		RoomID:     b[5],
		Firmware:   b[6],
		TestResult: b[7],
		Serial:     string(b[8:18]),
		Raw:        append([]byte(nil), b...),
	}
	need := func(n int) error {
		if len(b) < n {
			return fmt.Errorf("C: %s payload too short (%d bytes, need %d)", c.Type, len(b), n)
		}
		return nil
	}
	switch {
	case c.Type == DeviceCube:
		if err := need(cubeTZSummer + TimeZoneRuleSize); err != nil {
			return nil, err
		}
		url := b[cubePortalURL:cubePortalURLEnd]
		if i := bytes.IndexByte(url, 0); i >= 0 {
			url = url[:i]
		}
		c.Cube = &CubeConfig{
			PortalEnabled:  b[cubePortalEnabled] != 0,
			PortalURL:      string(url),
			PushButtonUp:   b[cubePushUp],
			PushButtonDown: b[cubePushDown],
			Winter:         decodeTimeZoneRule(b[cubeTZWinter:]),
			Summer:         decodeTimeZoneRule(b[cubeTZSummer:]),
		}
	case c.Type.IsRadiatorThermostat():
		if err := need(thermoWeekProgram + WeekProgramSize); err != nil {
			return nil, err
		}
		boost := b[0x19]
		boostMins := int(boost>>5) * 5
		if boost>>5 == 7 {
			boostMins = 60
		}
		c.Thermostat = &ThermostatConfig{
			ComfortTemp:    DecodeTemp(b[0x12]),
			EcoTemp:        DecodeTemp(b[0x13]),
			MaxSetpoint:    DecodeTemp(b[0x14]),
			MinSetpoint:    DecodeTemp(b[0x15]),
			TempOffset:     DecodeTemp(b[0x16]) - 3.5,
			WindowOpenTemp: DecodeTemp(b[0x17]),
			WindowOpenMins: int(b[0x18]) * 5,
			BoostMinutes:   boostMins,
			BoostValvePct:  int(boost&0x1f) * 5,
			DecalcDay:      Weekday(b[0x1a] >> 5),
			DecalcHour:     int(b[0x1a] & 0x1f),
			MaxValvePct:    pctFrom255(b[0x1b]),
			ValveOffsetPct: pctFrom255(b[0x1c]),
			WeekProgram:    DecodeWeekProgram(b[thermoWeekProgram : thermoWeekProgram+WeekProgramSize]),
		}
	case c.Type == DeviceWallThermostat:
		if err := need(wallWeekProgram + WeekProgramSize); err != nil {
			return nil, err
		}
		w := &WallConfig{
			ComfortTemp: DecodeTemp(b[0x12]),
			EcoTemp:     DecodeTemp(b[0x13]),
			MaxSetpoint: DecodeTemp(b[0x14]),
			MinSetpoint: DecodeTemp(b[0x15]),
			WeekProgram: DecodeWeekProgram(b[wallWeekProgram : wallWeekProgram+WeekProgramSize]),
		}
		rest := b[wallWeekProgram+WeekProgramSize:]
		if len(rest) >= 3 {
			w.TempOffset = DecodeTemp(rest[0]) - 3.5
			w.WindowOpenTemp = DecodeTemp(rest[1])
			w.BoostMinutes = int(rest[2]>>5) * 5
			if rest[2]>>5 == 7 {
				w.BoostMinutes = 60
			}
			w.BoostValvePct = int(rest[2]&0x1f) * 5
			rest = rest[3:]
		}
		w.Extra = append([]byte(nil), rest...)
		c.Wall = w
	}
	return c, nil
}

func pctFrom255(v byte) int { return (int(v)*100 + 127) / 255 }
func pctTo255(p int) byte   { return byte((p*255 + 50) / 100) }

// WeekProgramSize is the encoded size of a week program: 7 days × 13 slots × 2 bytes.
const WeekProgramSize = 7 * DaySlots * 2

// DaySlots is the number of switch points a device stores per day.
const DaySlots = 13

// WeekProgram holds one DayProgram per weekday, indexed by Weekday
// (Saturday first, as on the device).
type WeekProgram [7]DayProgram

// DayProgram is a list of switch points. Each point sets a temperature that
// applies until the given time; the last point always ends at 24:00.
type DayProgram []SwitchPoint

// SwitchPoint is "Temp °C until Until minutes after midnight".
type SwitchPoint struct {
	Temp  float64 `json:"temp"`
	Until int     `json:"until_min"` // minutes since midnight, multiple of 5, max 1440
}

// DecodeWeekProgram decodes 182 bytes. Unused slots after the 24:00 point are dropped.
func DecodeWeekProgram(b []byte) WeekProgram {
	var w WeekProgram
	for d := 0; d < 7; d++ {
		for s := 0; s < DaySlots; s++ {
			off := (d*DaySlots + s) * 2
			p := decodeSwitchPoint(binary.BigEndian.Uint16(b[off:]))
			w[d] = append(w[d], p)
			if p.Until >= 24*60 {
				break
			}
		}
	}
	return w
}

func decodeSwitchPoint(v uint16) SwitchPoint {
	return SwitchPoint{Temp: DecodeTemp(uint8(v >> 9)), Until: int(v&0x1ff) * 5}
}

func encodeSwitchPoint(p SwitchPoint) (uint16, error) {
	t, err := EncodeTemp(p.Temp)
	if err != nil {
		return 0, err
	}
	if p.Until <= 0 || p.Until > 24*60 || p.Until%5 != 0 {
		return 0, fmt.Errorf("switch point until %d min: must be a multiple of 5 between 5 and 1440", p.Until)
	}
	return uint16(t)<<9 | uint16(p.Until/5), nil
}

// Validate checks that the day program is well formed.
func (d DayProgram) Validate() error {
	if len(d) == 0 || len(d) > DaySlots {
		return fmt.Errorf("day program: want 1..%d switch points, got %d", DaySlots, len(d))
	}
	prev := 0
	for i, p := range d {
		if _, err := encodeSwitchPoint(p); err != nil {
			return fmt.Errorf("day program point %d: %w", i, err)
		}
		if p.Until <= prev {
			return fmt.Errorf("day program point %d: times must increase", i)
		}
		prev = p.Until
	}
	if prev != 24*60 {
		return fmt.Errorf("day program: last point must end at 24:00")
	}
	return nil
}

// JSON forms readable in backups: a week program is an object keyed by
// weekday name in calendar order, a switch point is {"temp": 17,
// "until": "07:00"}. Both also accept the device-native forms (a
// Saturday-first list, and "until_min" minutes) when read.

var jsonDayOrder = [...]Weekday{Monday, Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday}

// MarshalJSON implements json.Marshaler.
func (w WeekProgram) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, d := range jsonDayOrder {
		if i > 0 {
			buf.WriteByte(',')
		}
		day, err := json.Marshal(w[d])
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&buf, "%q:%s", d.String(), day)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (w *WeekProgram) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '[' {
		var days [7]DayProgram
		if err := json.Unmarshal(b, &days); err != nil {
			return err
		}
		*w = days
		return nil
	}
	var keyed map[string]DayProgram
	if err := json.Unmarshal(b, &keyed); err != nil {
		return err
	}
	var out WeekProgram
	for name, prog := range keyed {
		var d Weekday
		if err := d.UnmarshalText([]byte(name)); err != nil {
			return err
		}
		out[d] = prog
	}
	*w = out
	return nil
}

// MarshalJSON implements json.Marshaler.
func (p SwitchPoint) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Temp  float64 `json:"temp"`
		Until string  `json:"until"`
	}{p.Temp, fmt.Sprintf("%02d:%02d", p.Until/60, p.Until%60)})
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *SwitchPoint) UnmarshalJSON(b []byte) error {
	var raw struct {
		Temp     float64         `json:"temp"`
		Until    json.RawMessage `json:"until"`
		UntilMin *int            `json:"until_min"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	p.Temp = raw.Temp
	switch {
	case raw.UntilMin != nil:
		p.Until = *raw.UntilMin
	case len(raw.Until) > 0 && raw.Until[0] == '"':
		var s string
		if err := json.Unmarshal(raw.Until, &s); err != nil {
			return err
		}
		v, err := parseClock(s)
		if err != nil {
			return err
		}
		p.Until = v
	case len(raw.Until) > 0:
		// A number is HHMM (1905, 600); note that 0600 is octal in YAML.
		var n int
		if err := json.Unmarshal(raw.Until, &n); err != nil {
			return err
		}
		v, err := parseClock(fmt.Sprintf("%04d", n))
		if err != nil {
			return err
		}
		p.Until = v
	default:
		return fmt.Errorf("switch point: missing until")
	}
	return nil
}

// parseClock reads "HH:MM", "H:MM" or "HHMM" as minutes since midnight.
func parseClock(s string) (int, error) {
	var h, m int
	switch {
	case strings.Contains(s, ":"):
		if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
			return 0, fmt.Errorf("switch point until %q: want HH:MM", s)
		}
	case len(s) == 4:
		if _, err := fmt.Sscanf(s, "%2d%2d", &h, &m); err != nil {
			return 0, fmt.Errorf("switch point until %q: want HH:MM or HHMM", s)
		}
	default:
		return 0, fmt.Errorf("switch point until %q: want HH:MM or HHMM", s)
	}
	if h < 0 || h > 24 || m < 0 || m > 59 || (h == 24 && m != 0) {
		return 0, fmt.Errorf("switch point until %q: not a time of day", s)
	}
	return h*60 + m, nil
}

// EncodeWeekProgram encodes a week programme as a C line holds it: 182
// bytes, 13 switch points per day from Saturday on, a day of fewer
// points padded with its last one, as the devices store it.
func EncodeWeekProgram(w WeekProgram) ([]byte, error) {
	out := make([]byte, 0, WeekProgramSize)
	for d, day := range w {
		if len(day) == 0 || len(day) > DaySlots {
			return nil, fmt.Errorf("week programme: %s has %d switch points, want 1 to %d", Weekday(d), len(day), DaySlots)
		}
		for s := 0; s < DaySlots; s++ {
			p := day[min(s, len(day)-1)]
			v, err := encodeSwitchPoint(p)
			if err != nil {
				return nil, fmt.Errorf("week programme: %s: %w", Weekday(d), err)
			}
			out = binary.BigEndian.AppendUint16(out, v)
		}
	}
	return out, nil
}
