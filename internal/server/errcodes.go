// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Error codes (AP ErrorCodes): a refusal a page can meet carries a
// stable code and its parameters beside its English error, and the page
// says it in its own language from err.<code> of its tables. A code is
// declared once, by errorCode, in this file or in an errcodes_<area>.go
// of its own: its name, its English format and the names of its
// parameters in the format's order. A site refuses with a declared code
// (refuse, refuseSaying), so that it cannot give one that is not
// declared; TestErrorCodes holds the declarations to the page's six
// tables and to docs/api-v1.md ("Error codes").

import "fmt"

// errCode is a refusal's code as declared: the name a client branches
// on, never renamed once released; the English format of its error; and
// the names of the parameters the format takes, in its order.
type errCode struct {
	name   string
	format string
	params []string
}

// errorCodes lists every declared code, in the order of declaration.
var errorCodes []*errCode

// errorCode declares a code: its name, lower-case words joined by _, its
// English format and the names of its parameters in the format's order.
func errorCode(name, format string, params ...string) *errCode {
	c := &errCode{name: name, format: format, params: params}
	errorCodes = append(errorCodes, c)
	return c
}

// refusal is the error of a refusal with code c and status: args are
// its parameters in the order of c's names, and fill c's format, its
// English text. A parameter the site does not give is left out.
func (c *errCode) refusal(status int, args ...any) *httpError {
	he := &httpError{status: status, msg: fmt.Sprintf(c.format, args...), code: c}
	for i, name := range c.params {
		if i >= len(args) {
			break
		}
		if he.params == nil {
			he.params = map[string]any{}
		}
		he.params[name] = args[i]
	}
	return he
}

// refuse is the error of a refusal with a declared code and status:
// args are the code's parameters, in the order its declaration names
// them, and fill its English format.
func refuse(status int, c *errCode, args ...any) error {
	return c.refusal(status, args...)
}

// refuseSaying is refuse with the site's own English text in place of
// the code's format: for a refusal whose words are another error's, as
// a command's result, which the log, the audit and the scripts go on
// reading as they did.
func refuseSaying(status int, msg string, c *errCode, args ...any) error {
	he := c.refusal(status, args...)
	he.msg = msg
	return he
}

// The lookups every request to a Cube, a room or a device passes.
var (
	codeCubeUnknown         = errorCode("cube_unknown", "no cube %q", "cube")
	codeCubeNotConnectedYet = errorCode("cube_not_connected_yet", "cube not connected yet")
	codeRoomUnknown         = errorCode("room_unknown", "no room %d", "room")
	codeDeviceUnknown       = errorCode("device_unknown", "no device %s", "device")
)

// A room's thermostats, which its setpoint, its configuration and its
// programme go to (D4-A): none where the Cube's list has none in the
// room either; unplaced where the list has one in it, but the Cube has
// that thermostat in another room or in none, which the repair "Place
// in its listed room" mends (repair_api.go, buildPlace).
var (
	codeRoomNoThermostat       = errorCode("room_no_thermostat", "room %d has no thermostat", "room")
	codeRoomThermostatUnplaced = errorCode("room_thermostat_unplaced",
		"room %d has no thermostat on the cube: its list has thermostat %s in the room, but the cube has it in another room or in none; place it in its listed room", "room", "device")
)

// A command's result (commandError): the English text is the result's
// own error, as it was before the codes.
var (
	codeCommandSuperseded       = errorCode("command_superseded", "a newer command for the same target replaced the command before it was sent")
	codeCommandDiscarded        = errorCode("command_discarded", "the cube discarded the command")
	codeCommandDiscardedNoState = errorCode("command_discarded_no_state",
		"the cube discarded the command: it holds no state for room %d members %s since its last restart", "room", "devices")
	codeCommandUnwanted  = errorCode("command_unwanted", "the command was no longer wanted when its turn came")
	codeCubeNotConnected = errorCode("cube_not_connected", "the server is not connected to the cube")
	codeCubeBusy         = errorCode("cube_busy", "another command is waiting for the cube's reply")
	codeCommandTimedOut  = errorCode("command_timed_out", "the command's time ran out before the cube answered")
)

// The refusals of a call through an integration (onbehalf.go, auth.go;
// docs/api-v1.md, "Acting for an integration's users"), whose sites say
// which link refused in words of their own, and the repair's room name
// (repair_api.go): their answers are as before the codes, the room's
// top-level room beside its params.
var (
	codeBadHeader           = errorCode("bad_header", "the header says something it may not")
	codeNotIntegration      = errorCode("not_integration", "this key is no registered integration's; it acts for nobody")
	codeIntegrationDisabled = errorCode("integration_disabled", "the integration is disabled")
	codeInstancePending     = errorCode("instance_pending", "the call came from another installation than the one accepted")
	codeUserDisabled        = errorCode("user_disabled", "the integration's user, its linked user or one of their roles is disabled")
	codeForbidden           = errorCode("forbidden", "the integration may not do this")
	codeRoomNameNeeded      = errorCode("room_name_needed", "room %d is not in the Cube's room list; give room_name to list it there as a new room", "room")
)
