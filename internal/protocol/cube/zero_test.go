// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

import (
	"errors"
	"testing"
)

// A command to radio address 000000 as a device is refused; a room's
// radio command carrying 000000 and every command to another device
// pass.
func TestCheckZero(t *testing.T) {
	setTemp := func(tg Target) Message {
		m, err := CmdSetTemperature(tg, 21, ModeManual, Until{})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	cases := []struct {
		name    string
		m       Message
		refused bool
	}{
		{"setpoint to 000000", setTemp(Target{RFAddress: ZeroAddress}), true},
		{"setpoint to a room, 000000 with the room flag", setTemp(Target{RFAddress: ZeroAddress, RoomID: 1, Room: true}), false},
		{"setpoint to a device", setTemp(Target{RFAddress: 0x0a0101}), false},
		{"lock 000000", CmdLockControls(Target{RFAddress: ZeroAddress}, true), true},
		{"link 000000 as a partner", CmdLinkPartner(Target{RFAddress: 0x0a0101}, true, ZeroAddress, DeviceWallThermostat), true},
		{"unlink 000000 as a partner", CmdLinkPartner(Target{RFAddress: 0x0a0101}, false, ZeroAddress, DeviceWallThermostat), true},
		{"link a partner", CmdLinkPartner(Target{RFAddress: 0x0a0101}, true, 0x0a0102, DeviceWallThermostat), false},
		{"group of 000000", CmdSetGroup(ZeroAddress, 1, true), true},
		{"delete 000000", CmdDeleteDevices(true, ZeroAddress), true},
		{"delete 000000 among others", CmdDeleteDevices(false, 0x0a0101, ZeroAddress), true},
		{"delete a device", CmdDeleteDevices(true, 0x0a0101), false},
		{"reset 000000's error", CmdResetError(0x0a0101, ZeroAddress), true},
		{"reset a device's error", CmdResetError(0x0a0101), false},
		{"wake 000000", CmdWakeup(30, WakeupDevice, "000000"), true},
		{"wake all", CmdWakeup(30, WakeupAll, ""), false},
		{"wake a room", CmdWakeup(30, WakeupRoom, "01"), false},
		{"list", CmdList, false},
	}
	for _, c := range cases {
		err := CheckZero(c.m)
		if got := errors.Is(err, ErrZeroAddress); got != c.refused {
			t.Errorf("%s (%s): refused %v, want %v", c.name, c.m, got, c.refused)
		}
	}
}
