// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

import (
	"fmt"
	"strconv"
	"strings"
)

// Hello is the H message the Cube sends first on every connection.
//
// Over the portal connection the Cube sends a shortened H line containing
// only serial, RF address and firmware; the remaining fields are then zero.
type Hello struct {
	Serial          string    `json:"serial"`
	RFAddress       RFAddress `json:"rf_address"`
	Firmware        string    `json:"firmware"`     // raw, e.g. "0113"
	FirmwareVersion string    `json:"firmware_ver"` // e.g. "1.1.3"
	ConnectionID    string    `json:"connection_id,omitempty"`
	DutyCycle       int       `json:"duty_cycle_pct"` // percentage of the hourly radio budget already used
	FreeMemorySlots int       `json:"free_memory_slots"`
	CubeDate        string    `json:"cube_date,omitempty"` // YYYY-MM-DD
	CubeTime        string    `json:"cube_time,omitempty"` // HH:MM
	Fields          []string  `json:"fields"`              // all raw fields, for diagnostics
}

// ParseHello parses the payload of an H message.
func ParseHello(data string) (*Hello, error) {
	f := strings.Split(data, ",")
	if len(f) < 3 {
		return nil, fmt.Errorf("H: want at least 3 fields, got %d", len(f))
	}
	rf, err := ParseRFAddress(f[1])
	if err != nil {
		return nil, fmt.Errorf("H: %w", err)
	}
	h := &Hello{Serial: f[0], RFAddress: rf, Firmware: f[2], FirmwareVersion: firmwareVersion(f[2]), Fields: f}
	if len(f) > 4 {
		h.ConnectionID = f[4]
	}
	if len(f) > 5 {
		if h.DutyCycle, err = parseHexInt("H: duty cycle", f[5]); err != nil {
			return nil, err
		}
	}
	if len(f) > 6 {
		if h.FreeMemorySlots, err = parseHexInt("H: free memory slots", f[6]); err != nil {
			return nil, err
		}
	}
	if len(f) > 7 && len(f[7]) == 6 {
		y, _ := strconv.ParseUint(f[7][0:2], 16, 8)
		m, _ := strconv.ParseUint(f[7][2:4], 16, 8)
		d, _ := strconv.ParseUint(f[7][4:6], 16, 8)
		h.CubeDate = fmt.Sprintf("%04d-%02d-%02d", 2000+y, m, d)
	}
	if len(f) > 8 && len(f[8]) == 4 {
		hh, _ := strconv.ParseUint(f[8][0:2], 16, 8)
		mm, _ := strconv.ParseUint(f[8][2:4], 16, 8)
		h.CubeTime = fmt.Sprintf("%02d:%02d", hh, mm)
	}
	return h, nil
}

func firmwareVersion(raw string) string {
	s := strings.TrimLeft(raw, "0")
	if s == "" {
		return raw
	}
	return strings.Join(strings.Split(s, ""), ".")
}

func parseHexInt(field, s string) (int, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 16, 32)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not hex", field, s)
	}
	return int(v), nil
}

// SendResult is the S message returned after an s command.
type SendResult struct {
	DutyCycle       int  `json:"duty_cycle_pct"`
	Discarded       bool `json:"discarded"` // true if the Cube rejected the command (usually: duty cycle exhausted)
	FreeMemorySlots int  `json:"free_memory_slots"`
}

// ParseSendResult parses the payload of an S message, e.g. "00,0,31".
func ParseSendResult(data string) (*SendResult, error) {
	f := strings.Split(data, ",")
	if len(f) != 3 {
		return nil, fmt.Errorf("S: want 3 fields, got %d", len(f))
	}
	dc, err := parseHexInt("S: duty cycle", f[0])
	if err != nil {
		return nil, err
	}
	slots, err := parseHexInt("S: free memory slots", f[2])
	if err != nil {
		return nil, err
	}
	return &SendResult{DutyCycle: dc, Discarded: f[1] != "0", FreeMemorySlots: slots}, nil
}

// NewDevice is the N message reported while the Cube is in pairing mode.
type NewDevice struct {
	Type      DeviceType `json:"type"`
	RFAddress RFAddress  `json:"rf_address"`
	Serial    string     `json:"serial"`
}

// ParseNewDevice parses the payload of an N message.
func ParseNewDevice(data string) (*NewDevice, error) {
	b, err := decodeB64("N", data)
	if err != nil {
		return nil, err
	}
	if len(b) < 14 {
		return nil, fmt.Errorf("N: payload too short (%d bytes)", len(b))
	}
	return &NewDevice{Type: DeviceType(b[0]), RFAddress: RFAddressFromBytes(b[1:4]), Serial: string(b[4:14])}, nil
}

// ParseNTPServers parses the payload of an F message.
func ParseNTPServers(data string) []string {
	var out []string
	for _, s := range strings.Split(data, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ParseCryptoReply decodes the base64 payload of an E (encrypted) or D
// (decrypted) message.
func ParseCryptoReply(data string) ([]byte, error) { return decodeB64("E/D", data) }
