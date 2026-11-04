// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package cube implements the text protocol spoken by the eQ-3 MAX! Cube LAN
// gateway on TCP port 62910. The same line format is tunnelled through the
// Cube's encrypted HTTP "portal" connection.
//
// The protocol was reverse-engineered by the community; the main reference is
// https://github.com/Bouni/max-cube-protocol. Where this package deviates from
// that documentation, the reason is noted next to the code.
package cube

import (
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

// RFAddress is the 24-bit radio address of a MAX! device or of the Cube.
type RFAddress uint32

// ParseRFAddress parses a 6-digit hexadecimal radio address such as "0fdaed".
func ParseRFAddress(s string) (RFAddress, error) {
	s = strings.TrimSpace(s)
	if len(s) != 6 {
		return 0, fmt.Errorf("rf address %q: want 6 hex digits", s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return 0, fmt.Errorf("rf address %q: %w", s, err)
	}
	return RFAddressFromBytes(b), nil
}

// RFAddressFromBytes reads a big-endian 3-byte radio address. b must hold at
// least 3 bytes.
func RFAddressFromBytes(b []byte) RFAddress {
	return RFAddress(uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2]))
}

// Bytes returns the big-endian 3-byte representation.
func (a RFAddress) Bytes() [3]byte {
	return [3]byte{byte(a >> 16), byte(a >> 8), byte(a)}
}

func (a RFAddress) String() string { return fmt.Sprintf("%06x", uint32(a)) }

// MarshalText implements encoding.TextMarshaler.
func (a RFAddress) MarshalText() ([]byte, error) { return []byte(a.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler.
func (a *RFAddress) UnmarshalText(b []byte) error {
	v, err := ParseRFAddress(string(b))
	if err != nil {
		return err
	}
	*a = v
	return nil
}

// DeviceType identifies the kind of MAX! device.
type DeviceType uint8

const (
	DeviceCube                   DeviceType = 0
	DeviceRadiatorThermostat     DeviceType = 1
	DeviceRadiatorThermostatPlus DeviceType = 2
	DeviceWallThermostat         DeviceType = 3
	DeviceShutterContact         DeviceType = 4
	DeviceEcoSwitch              DeviceType = 5 // called "push button" in the protocol docs
)

var deviceTypeNames = map[DeviceType]string{
	DeviceCube:                   "cube",
	DeviceRadiatorThermostat:     "radiator_thermostat",
	DeviceRadiatorThermostatPlus: "radiator_thermostat_plus",
	DeviceWallThermostat:         "wall_thermostat",
	DeviceShutterContact:         "shutter_contact",
	DeviceEcoSwitch:              "eco_switch",
}

func (t DeviceType) String() string {
	if n, ok := deviceTypeNames[t]; ok {
		return n
	}
	return fmt.Sprintf("unknown_%d", uint8(t))
}

// MarshalText implements encoding.TextMarshaler.
func (t DeviceType) MarshalText() ([]byte, error) { return []byte(t.String()), nil }

// IsRadiatorThermostat reports whether t is one of the valve-mounted thermostats.
func (t DeviceType) IsRadiatorThermostat() bool {
	return t == DeviceRadiatorThermostat || t == DeviceRadiatorThermostatPlus
}

// Mode is the operating mode of a thermostat.
type Mode uint8

const (
	ModeAuto     Mode = 0 // weekly program
	ModeManual   Mode = 1
	ModeVacation Mode = 2 // also: "temporary until"
	ModeBoost    Mode = 3
)

func (m Mode) String() string {
	switch m {
	case ModeAuto:
		return "auto"
	case ModeManual:
		return "manual"
	case ModeVacation:
		return "vacation"
	case ModeBoost:
		return "boost"
	}
	return fmt.Sprintf("unknown_%d", uint8(m))
}

// MarshalText implements encoding.TextMarshaler.
func (m Mode) MarshalText() ([]byte, error) { return []byte(m.String()), nil }

// ParseMode parses the names returned by Mode.String.
func ParseMode(s string) (Mode, error) {
	for m := ModeAuto; m <= ModeBoost; m++ {
		if strings.EqualFold(s, m.String()) {
			return m, nil
		}
	}
	return 0, fmt.Errorf("unknown mode %q", s)
}

// Temperature limits accepted by MAX! thermostats. 4.5 °C means "off" and
// 30.5 °C means "fully open".
const (
	TempOff = 4.5
	TempOn  = 30.5
)

// EncodeTemp converts a temperature in °C to the protocol's half-degree units.
// Only multiples of 0.5 between TempOff and TempOn are accepted.
func EncodeTemp(c float64) (uint8, error) {
	v := c * 2
	if v != math.Trunc(v) {
		return 0, fmt.Errorf("temperature %.2f: must be a multiple of 0.5 °C", c)
	}
	if c < TempOff || c > TempOn {
		return 0, fmt.Errorf("temperature %.1f: must be between %.1f and %.1f °C", c, TempOff, TempOn)
	}
	return uint8(v), nil
}

// DecodeTemp converts half-degree units to °C.
func DecodeTemp(v uint8) float64 { return float64(v) / 2 }

// Weekday as used by MAX! devices: the week starts on Saturday.
type Weekday uint8

const (
	Saturday Weekday = iota
	Sunday
	Monday
	Tuesday
	Wednesday
	Thursday
	Friday
)

var weekdayNames = [...]string{"saturday", "sunday", "monday", "tuesday", "wednesday", "thursday", "friday"}

func (d Weekday) String() string {
	if int(d) < len(weekdayNames) {
		return weekdayNames[d]
	}
	return fmt.Sprintf("unknown_%d", uint8(d))
}

// MarshalText implements encoding.TextMarshaler.
func (d Weekday) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler using the names from
// DeviceType.String.
func (t *DeviceType) UnmarshalText(b []byte) error {
	s := string(b)
	for k, n := range deviceTypeNames {
		if strings.EqualFold(s, n) {
			*t = k
			return nil
		}
	}
	var n uint8
	if _, err := fmt.Sscanf(s, "unknown_%d", &n); err == nil {
		*t = DeviceType(n)
		return nil
	}
	return fmt.Errorf("unknown device type %q", s)
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (m *Mode) UnmarshalText(b []byte) error {
	v, err := ParseMode(string(b))
	if err != nil {
		return err
	}
	*m = v
	return nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Weekday) UnmarshalText(b []byte) error {
	for i, n := range weekdayNames {
		if strings.EqualFold(string(b), n) {
			*d = Weekday(i)
			return nil
		}
	}
	return fmt.Errorf("unknown weekday %q", string(b))
}
