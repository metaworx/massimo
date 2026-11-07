// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

import "fmt"

// DeviceState is one entry of the L message: the live status of a device.
type DeviceState struct {
	RFAddress RFAddress  `json:"rf_address"`
	Type      DeviceType `json:"type"` // taken from metadata; guessed from the entry length if unknown

	Valid        bool `json:"valid"`
	Error        bool `json:"error"`
	IsAnswer     bool `json:"is_answer"` // the entry answers a command (protocol bit is inverted)
	Initialized  bool `json:"initialized"`
	BatteryLow   bool `json:"battery_low"`
	LinkError    bool `json:"link_error"`
	PanelLocked  bool `json:"panel_locked"`
	GatewayKnown bool `json:"gateway_known"`
	DST          bool `json:"dst"`
	Mode         Mode `json:"mode"`

	// Thermostats only.
	ValvePct   *int     `json:"valve_pct,omitempty"`
	Setpoint   *float64 `json:"setpoint,omitempty"`
	ActualTemp *float64 `json:"actual_temp,omitempty"` // nil when the device did not report one
	Until      *Until   `json:"until,omitempty"`       // vacation mode end

	// Wall thermostats only: the display shows the measured temperature
	// (true) or the setpoint (false). Set with radio command 0x82.
	DisplayActual *bool `json:"display_actual,omitempty"`

	// Shutter contacts only.
	WindowOpen *bool `json:"window_open,omitempty"`

	Raw []byte `json:"raw"`
}

// ParseDeviceList parses the payload of an L message. types maps radio
// addresses to device types (from the metadata); it may be nil.
func ParseDeviceList(data string, types map[RFAddress]DeviceType) ([]DeviceState, error) {
	b, err := decodeB64("L", data)
	if err != nil {
		return nil, err
	}
	var out []DeviceState
	for off := 0; off < len(b); {
		n := int(b[off])
		if n < 6 || off+1+n > len(b) {
			return out, fmt.Errorf("L: bad entry length %d at offset %d", n, off)
		}
		e := b[off : off+1+n]
		out = append(out, decodeDeviceState(e, types))
		off += 1 + n
	}
	return out, nil
}

// decodeDeviceState decodes one entry including its length byte. Offsets
// follow the protocol docs: 1-3 address, 4 unknown, 5-6 flags, then
// 7 valve, 8 setpoint, 9-10 date or temperature, 11 time, 12 wall temperature.
func decodeDeviceState(e []byte, types map[RFAddress]DeviceType) DeviceState {
	s := DeviceState{RFAddress: RFAddressFromBytes(e[1:4]), Raw: append([]byte(nil), e...)}
	if t, ok := types[s.RFAddress]; ok {
		s.Type = t
	} else {
		s.Type = guessType(len(e) - 1)
	}
	f1, f2 := e[5], e[6]
	s.Valid = f1&0x10 != 0
	s.Error = f1&0x08 != 0
	s.IsAnswer = f1&0x04 == 0
	s.Initialized = f1&0x02 != 0
	s.BatteryLow = f2&0x80 != 0
	s.LinkError = f2&0x40 != 0
	s.PanelLocked = f2&0x20 != 0
	s.GatewayKnown = f2&0x10 != 0
	s.DST = f2&0x08 != 0
	s.Mode = Mode(f2 & 0x03)

	switch {
	case s.Type == DeviceShutterContact:
		open := f2&0x02 != 0
		s.WindowOpen = &open
	case !s.Valid:
		// The Cube has no data for the device yet (e.g. after a reboot);
		// the value bytes are all zero and must not be read as 0 °C.
	case s.Type.IsThermostat() && len(e) >= 12:
		sp := DecodeTemp(e[8] & 0x3f)
		s.Setpoint = &sp
		if s.Type.IsRadiatorThermostat() {
			valve := int(e[7])
			s.ValvePct = &valve
		}
		if s.Mode == ModeVacation {
			y, m, d := decodeUntilDate(e[9], e[10])
			s.Until = &Until{Year: y, Month: m, Day: d, Minutes: int(e[11]&0x3f) * 30}
		} else if s.Type.IsRadiatorThermostat() {
			// Thermostats report the measured temperature only occasionally
			// (e.g. after the valve moved); 0 means "not reported".
			if raw := int(e[9]&0x01)<<8 | int(e[10]); raw != 0 {
				t := float64(raw) / 10
				s.ActualTemp = &t
			}
		}
		if s.Type == DeviceWallThermostat && len(e) >= 13 {
			t := float64(int(e[8]&0x80)<<1|int(e[12])) / 10
			s.ActualTemp = &t
			// Observed: byte 7 is 0x01 with the setpoint on the display
			// and 0x04 after the "show measured temperature" command.
			d := e[7]&0x04 != 0
			s.DisplayActual = &d
		}
	}
	return s
}

func guessType(n int) DeviceType {
	switch {
	case n >= 12:
		return DeviceWallThermostat
	case n >= 11:
		return DeviceRadiatorThermostat
	default:
		return DeviceShutterContact
	}
}
