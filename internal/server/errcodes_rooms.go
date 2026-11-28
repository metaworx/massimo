// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The codes of the refusals of rooms and devices, their virtual wall
// thermostats, the pairing, the batteries and the repairs (AP ErrorCodes,
// Block 2), declared as errcodes.go declares its own.

import (
	"errors"
	"net/http"
	"strconv"
)

// A room's and a device's names, the rooms they are in, and the Cube's
// structure, which one change at a time writes (lockStructure). A
// device's settings not copied carries the refusal that stopped the
// copy as reason (refusalOf).
var (
	codeStructureBusy       = errorCode("structure_change_running", "another change of the cube's rooms and devices is still running")
	codeNameEmpty           = errorCode("name_empty", "name must not be empty")
	codeNameTooLong         = errorCode("name_too_long", "name must be valid text of at most %d bytes", "max")
	codeRoomNotEmpty        = errorCode("room_not_empty", "room %d still has device %s; move it first", "room", "device")
	codeRoomWallTaken       = errorCode("room_wall_thermostat_taken", "room %d has a wall thermostat already (%s); a room takes one, as in the original software", "room", "device")
	codeDeviceRoomNeeded    = errorCode("device_room_needed", "device %s is in no room yet; give room_id to add it to one", "device")
	codeDeviceNotCopied     = errorCode("device_settings_not_copied", "device %s is in room %d, but copying the room's settings stopped: %s", "device", "room", "reason")
	codeSetpointModeInvalid = errorCode("setpoint_mode_invalid", "mode must be auto, manual, vacation or boost")
)

// A device's and a room's configuration, where a value is out of the
// range the original software offers (configRequest.validate).
var (
	codeConfigOffsetInvalid  = errorCode("config_temp_offset_invalid", "temp_offset: must be a multiple of 0.5 between -3.5 and 3.5")
	codeConfigWindowInvalid  = errorCode("config_window_minutes_invalid", "window_open_minutes: must be 0..60 in steps of 5")
	codeConfigBoostInvalid   = errorCode("config_boost_minutes_invalid", "boost_minutes: must be 0..30 in steps of 5, or 60")
	codeConfigValveInvalid   = errorCode("config_boost_valve_invalid", "boost_valve_pct: must be 0..100 in steps of 5")
	codeConfigDecalcInvalid  = errorCode("config_decalc_hour_invalid", "decalc_hour: must be 0..23")
	codeConfigPercentInvalid = errorCode("config_valve_pct_invalid", "%s: must be 0..100", "field")
)

// A week programme written with an If-Match of another tag than its
// current one (checkIfMatch), the current tag in etag beside the code.
var codeProgramChanged = errorCode("program_changed", "the week programme changed since it was read (or a write to it is still on its way): read it again, then write")

// A room's virtual wall thermostat (vwt.go): what of a device it is not,
// and its own settings. Its rename's English text names the room's path
// as well (refuseSaying).
var (
	codeVwtLockNoWall     = errorCode("vwt_lock_no_wall_thermostat", "room %d has no wall thermostat whose keys could be locked; lock its radiator thermostats one by one", "room")
	codeVwtRename         = errorCode("vwt_rename_with_room", "the virtual wall thermostat of room %d goes with its room: rename the room", "room")
	codeVwtRemove         = errorCode("vwt_remove_with_room", "the virtual wall thermostat of room %d goes with its room: it is removed with the room", "room")
	codeVwtMethodInvalid  = errorCode("vwt_temp_method_invalid", "temp_method: median or mean")
	codeVwtAddressInvalid = errorCode("vwt_address_invalid", "rf_address: six hex digits, or \"\" for the room number")
	codeVwtAddressReal    = errorCode("vwt_address_real", "rf_address %s is a real device's", "device")
	codeVwtAddressTaken   = errorCode("vwt_address_taken", "rf_address %s is the virtual wall thermostat's of room %d", "address", "room")
)

// The pairing (pairing_api.go).
var (
	codePairingSeconds = errorCode("pairing_seconds_invalid", "seconds must be 1..%d", "max")
	codePairingRunning = errorCode("pairing_running", "pairing is already running")
)

// New batteries marked (battery_api.go), while the Cube cannot show yet
// that the device has them.
var (
	codeBatteryNoReport = errorCode("battery_no_report", "the Cube holds no current report of device %s: wait for its next report", "device")
	codeBatteryStillLow = errorCode("battery_still_low", "device %s still reports its batteries low: wait for its next report", "device")
)

// The repairs (repair_api.go): a run refused, and why a repair has
// nothing to send, which its preview says as nothing with the same code
// and params. A run changed since its preview names the two plans in its
// English text, and one that stopped names the repair (refuseSaying),
// the refusal that stopped it its reason; a room's disagreement lists
// the devices joined in its text.
var (
	codeRepairChanged       = errorCode("repair_changed", "the repair changed since its preview; preview it again")
	codeRepairDutyCycle     = errorCode("repair_duty_cycle", "the radio duty cycle at %d %% and the repair's about %.1f %% would reach the hold at %d %%, which would stop it halfway; wait for the duty cycle to fall", "duty", "cost", "max")
	codeRepairSlots         = errorCode("repair_memory_slots", "the Cube has %d memory slots free and the repair's room radiograms hold %d until the devices' next wakeup, which would leave fewer than %d and stop it halfway", "free", "held", "min")
	codeRepairStopped       = errorCode("repair_stopped", "stopped at step %d of %d: %s", "step", "steps", "reason")
	codeRepairNoneFlagged   = errorCode("repair_none_flagged", "no device of the cube has a communication error")
	codeRepairRoomUnflagged = errorCode("repair_room_none_flagged", "no device of room %d has a communication error", "room")
	codeRepairMasterValid   = errorCode("repair_master_valid", "room %d's master %s is a thermostat of the room already", "room", "device")
	codeRepairMasterNone    = errorCode("repair_master_none", "room %d has no thermostat to name as its master", "room")
	codeRepairRoomDisagrees = errorCode("repair_room_disagrees", "the Cube and its list disagree on the devices of room %d (%s): place or list them first", "room", "devices")
	codeRepairLinkTooFew    = errorCode("repair_link_too_few", "room %d has fewer than two devices to link", "room")
	codeRepairDeviceNoRoom  = errorCode("repair_device_no_room", "the Cube has device %s in no room", "device")
	codeRepairRoomsDiffer   = errorCode("repair_rooms_differ", "the Cube has device %s in room %d and its list in room %d: place or list it first", "device", "room", "list_room")
	codeRepairConfigSame    = errorCode("repair_config_same", "device %s has room %d's temperatures and week programme already (what most of the room's %d thermostats share, else what the first of the room's order has); nothing to re-send", "device", "room", "thermostats")
	codeRepairUnlisted      = errorCode("repair_device_unlisted", "the Cube's list does not list device %s: list it instead", "device")
	codeRepairListNoRoom    = errorCode("repair_list_no_room", "the Cube's list places device %s in no room", "device")
	codeRepairPlacedAlready = errorCode("repair_placed_already", "the Cube has device %s in room %d already, as its list", "device", "room")
	codeRepairHiddenRoom    = errorCode("repair_hidden_room", "the Cube has device %s in the hidden room %d", "device", "room")
	codeRepairListedAlready = errorCode("repair_listed_already", "the Cube's list has device %s in room %d already, as the Cube", "device", "room")
)

// refusalOf is err as an error answer's body: its text, and its code and
// params where it has them. A refusal that names another one that
// stopped it, as a copy's or a repair's, carries it so in its params
// (reason), and the page says it in its language as it says the first.
func refusalOf(err error) map[string]any {
	var he *httpError
	if errors.As(err, &he) {
		return he.body()
	}
	return map[string]any{"error": err.Error()}
}

// codeOf is the code and the params of a refusal, "" and nil for an
// error without a code: a repair's preview carries them beside its
// nothing or its refused (repairPlan).
func codeOf(err error) (string, map[string]any) {
	var he *httpError
	if !errors.As(err, &he) || he.code == nil {
		return "", nil
	}
	return he.code.name, he.params
}

// noAliasRoom refuses a device path's room alias, room-<id> (roomAlias),
// whose room has no virtual wall thermostat: as no room of the id, its
// English text the id as the path spells it.
func noAliasRoom(name string) error {
	id := roomAlias.FindStringSubmatch(name)[1]
	n, _ := strconv.Atoi(id)
	return refuseSaying(http.StatusNotFound, "no room "+id, codeRoomUnknown, n)
}
