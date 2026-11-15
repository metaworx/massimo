// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

import (
	"encoding/json"
	"fmt"
)

// RadiogramType is the type of a radiogram between the Cube and a
// device: the command byte of an s message's radio frame, and byte 4 of
// an L entry whose error flag is set, the type of the Cube's last
// radiogram to the device that failed. The codes and names are those of
// the original software's FrameType enum (MaxEssentialsBackend 1.4.6,
// docs/cube-protocol.md); a code it lacks is kept as a number.
type RadiogramType uint8

// The radiogram types of the original software's FrameType enum.
const (
	RadiogramSystemInformation             RadiogramType = 0x00 // pair ping
	RadiogramInclusion                     RadiogramType = 0x01 // pair pong
	RadiogramAnswer                        RadiogramType = 0x02
	RadiogramTimeInformation               RadiogramType = 0x03 // the Cube's own time push
	RadiogramConfigWeekTemperatureProfile  RadiogramType = 0x10
	RadiogramConfigTemperatures            RadiogramType = 0x11
	RadiogramConfigValveFunctions          RadiogramType = 0x12
	RadiogramAddLinkPartner                RadiogramType = 0x20
	RadiogramRemoveLinkPartner             RadiogramType = 0x21
	RadiogramSetGroupAddress               RadiogramType = 0x22
	RadiogramRemoveGroupAddress            RadiogramType = 0x23
	RadiogramShutterContactState           RadiogramType = 0x30
	RadiogramControlMode                   RadiogramType = 0x40 // set temperature
	RadiogramSetPointTemperature           RadiogramType = 0x41
	RadiogramSetPointAndCurrentTemperature RadiogramType = 0x42 // wall thermostat
	RadiogramComfortTemperature            RadiogramType = 0x43
	RadiogramEcoTemperature                RadiogramType = 0x44
	RadiogramCurrentTemperatureAndHumidity RadiogramType = 0x45
	RadiogramPushButtonState               RadiogramType = 0x50
	RadiogramHeatingThermostatState        RadiogramType = 0x60
	RadiogramLockManualControls            RadiogramType = 0x80
	RadiogramDaylightSavingTimeMode        RadiogramType = 0x81
	RadiogramSwitchDisplaySetPointActual   RadiogramType = 0x82
	RadiogramReset                         RadiogramType = 0xF0
	RadiogramWakeUp                        RadiogramType = 0xF1
	RadiogramTest                          RadiogramType = 0xFF
)

// radiogramNames are the FrameType names of the known codes.
var radiogramNames = map[RadiogramType]string{
	RadiogramSystemInformation:             "SystemInformation",
	RadiogramInclusion:                     "Inclusion",
	RadiogramAnswer:                        "Answer",
	RadiogramTimeInformation:               "TimeInformation",
	RadiogramConfigWeekTemperatureProfile:  "ConfigWeekTemperatureProfile",
	RadiogramConfigTemperatures:            "ConfigTemperatures",
	RadiogramConfigValveFunctions:          "ConfigValveFunctions",
	RadiogramAddLinkPartner:                "AddLinkPartner",
	RadiogramRemoveLinkPartner:             "RemoveLinkPartner",
	RadiogramSetGroupAddress:               "SetGroupAddress",
	RadiogramRemoveGroupAddress:            "RemoveGroupAddress",
	RadiogramShutterContactState:           "ShutterContactState",
	RadiogramControlMode:                   "ControlMode",
	RadiogramSetPointTemperature:           "SetPointTemperature",
	RadiogramSetPointAndCurrentTemperature: "SetPointAndCurrentTemperature",
	RadiogramComfortTemperature:            "ComfortTemperature",
	RadiogramEcoTemperature:                "EcoTemperature",
	RadiogramCurrentTemperatureAndHumidity: "CurrentTemperatureAndHumidity",
	RadiogramPushButtonState:               "PushButtonState",
	RadiogramHeatingThermostatState:        "HeatingThermostatState",
	RadiogramLockManualControls:            "LockManualControls",
	RadiogramDaylightSavingTimeMode:        "DaylightSavingTimeMode",
	RadiogramSwitchDisplaySetPointActual:   "SwitchDisplaySetPointActual",
	RadiogramReset:                         "Reset",
	RadiogramWakeUp:                        "WakeUp",
	RadiogramTest:                          "Test",
}

// Name is the type's FrameType name, "" for a code the enum lacks.
func (r RadiogramType) Name() string { return radiogramNames[r] }

// String is the type's name, or its code in hex for one the enum lacks
// ("0xf6").
func (r RadiogramType) String() string {
	if n := r.Name(); n != "" {
		return n
	}
	return fmt.Sprintf("0x%02x", uint8(r))
}

// radiogramJSON is a radiogram type as the API gives it.
type radiogramJSON struct {
	Code uint8  `json:"code"`
	Name string `json:"name,omitempty"`
}

// MarshalJSON gives the type as {"code": 3, "name": "TimeInformation"},
// without the name for a code the enum lacks.
func (r RadiogramType) MarshalJSON() ([]byte, error) {
	return json.Marshal(radiogramJSON{Code: uint8(r), Name: r.Name()})
}

// UnmarshalJSON reads the type back from its object, as a stored state
// holds it, or from a bare code; the name is not read.
func (r *RadiogramType) UnmarshalJSON(b []byte) error {
	var code uint8
	if err := json.Unmarshal(b, &code); err == nil {
		*r = RadiogramType(code)
		return nil
	}
	var v radiogramJSON
	if err := json.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("radiogram type: %w", err)
	}
	*r = RadiogramType(v.Code)
	return nil
}
