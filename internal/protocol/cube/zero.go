// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

import (
	"errors"
	"strings"
)

// ErrZeroAddress refuses a command that addresses radio address 000000
// (ZeroAddress) as a device.
var ErrZeroAddress = errors.New("radio address 000000 is no device")

// CheckZero returns ErrZeroAddress for a command that addresses radio
// address 000000 as a device: a radio command (s:) to it without the
// room flag, or linking it as a partner; a deletion (t:), an error reset
// (r:) or a wakeup (z:) naming it. A room's radio command, which may
// carry 000000 with the room flag, passes; so does a line this cannot
// read, which is not a command to it. Removing a device makes the Cube
// send a factory reset to its address: to 000000 that could reach more
// than intended.
func CheckZero(m Message) error {
	switch m.Type {
	case 's':
		raw, err := b64.DecodeString(strings.TrimSpace(m.Data))
		if err != nil || len(raw) < 10 {
			return nil
		}
		room := raw[1]&0x04 != 0
		if !room && RFAddressFromBytes(raw[6:9]) == ZeroAddress {
			return ErrZeroAddress
		}
		if (raw[2] == RFAddLinkPartner || raw[2] == RFRemoveLink) && len(raw) >= 13 && RFAddressFromBytes(raw[10:13]) == ZeroAddress {
			return ErrZeroAddress
		}
	case 't':
		if _, addrs, err := ParseDeleteDevices(m.Data); err == nil && hasZero(addrs) {
			return ErrZeroAddress
		}
	case 'r':
		_, list, ok := strings.Cut(m.Data, ",")
		if !ok {
			return nil
		}
		raw, err := b64.DecodeString(strings.TrimSpace(list))
		if err != nil {
			return nil
		}
		for i := 0; i+3 <= len(raw); i += 3 {
			if RFAddressFromBytes(raw[i:i+3]) == ZeroAddress {
				return ErrZeroAddress
			}
		}
	case 'z':
		f := strings.Split(m.Data, ",")
		if len(f) == 3 && f[1] == string(WakeupDevice) && strings.TrimSpace(f[2]) == ZeroAddress.String() {
			return ErrZeroAddress
		}
	}
	return nil
}

func hasZero(addrs []RFAddress) bool {
	for _, a := range addrs {
		if a == ZeroAddress {
			return true
		}
	}
	return false
}
