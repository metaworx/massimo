// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The codes of the errors passed through and of those kept with a
// result (AP ErrorCodes, Block 5): a Go error of another package that a
// site passes on, mapped to its code in one place per kind, its English
// text the error's own (refuseSaying, passed); and the error a failed
// step or run keeps, in the log, a command's row, a restore's run and
// the views of the reboot, the restart and the update, as an object
// with its error, code and params (failure), which the page says in its
// language as it says a refusal. An error of no kind a page meets keeps
// none: a body that does not parse, input no page sends, our own
// failures.

import (
	"context"
	"errors"
	"net/http"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/password"
	"go.metaworx.ch/massimo/internal/store"
)

// A command to the Cube that did not end well, beyond the kinds
// errcodes.go declares: the Cube left it unanswered, or it failed for a
// reason of the link, whose text is its reason; a command the server's
// start dropped (store.DropUnfinished).
var (
	codeCubeNoReply     = errorCode("cube_no_reply", "the cube did not answer the command in time")
	codeCommandFailed   = errorCode("command_failed", "the command failed: %s", "reason")
	codeServerRestarted = errorCode("server_restarted", "server restarted")
)

// The rooms' and devices' writes: a radio address no device has, the
// Cube's list that cannot be written, a value the protocol does not
// take, a setpoint's end and a day of the week programme.
var (
	codeDeviceZeroAddress  = errorCode("device_zero_address", "radio address 000000 is no device")
	codeMetadataNotWritten = errorCode("metadata_not_written", "the cube's room and device list cannot be written: %s", "reason")
	codeTempInvalid        = errorCode("temp_invalid", "%s: temperatures are 4.5 to 30.5 °C in steps of 0.5", "field")
	codeDecalcDayInvalid   = errorCode("config_decalc_day_invalid", "decalc_day: want a day of the week, monday to sunday")
	codeConfigNotEncodable = errorCode("config_not_encodable", "the settings cannot be written: %s", "reason")
	codeUntilInvalid       = errorCode("setpoint_until_invalid", "until: want a date and a time YYYY-MM-DD HH:MM on the hour or the half hour")
	codeProgramDayInvalid  = errorCode("program_day_invalid", "the week programme's %s cannot be written: %s", "day", "reason")
)

// A repair's configuration re-sent from its room (cubeio.ResendCommands,
// cubeio.NoMajorityError): no other thermostat of the room to take it
// from, or none the Cube holds a valid state for; or none that can be
// built.
var (
	codeRepairNoOther       = errorCode("repair_no_other_thermostat", "room %d has no other thermostat to take the configuration from", "room")
	codeRepairNoneValid     = errorCode("repair_no_valid_thermostat", "room %d has no other thermostat the cube holds a valid state for to take the configuration from: its %d others have not reported since the cube's last restart or are out of date", "room", "dead")
	codeRepairNotResendable = errorCode("repair_resend_impossible", "the configuration of device %s cannot be re-sent: %s", "device", "reason")
)

// A Cube's row, its hidden devices, its refresh and reboot, the
// discovery, a backup and a restore's file, an update's check and its
// night's window, the integrations found on the LAN.
var (
	codeCubeHiddenBad        = errorCode("cube_hidden_bad", "hidden: %q is no radio address of six hex digits", "address")
	codeCubeRowNotFound      = errorCode("cube_row_not_found", "no such cube")
	codeCubeRowExists        = errorCode("cube_row_exists", "cube exists")
	codeCubeRefreshFailed    = errorCode("cube_refresh_failed", "refresh of %s: %s", "cube", "reason")
	codeCubeRebootFailed     = errorCode("cube_reboot_failed", "reboot of %s: %s", "cube", "reason")
	codeDiscoveryPortTaken   = errorCode("discovery_port_taken", "discovery: the discovery port is in use by another program")
	codeRestoreBackupUnsaved = errorCode("restore_backup_not_stored", "the backup before the restore was not stored: %s", "reason")
	codeRestoreFileBad       = errorCode("restore_file_bad", "bad backup file: %s", "reason")
	codeUpdateCheckFailed    = errorCode("update_check_failed", "release check failed: %s", "reason")
	codeUpdateSourceFailed   = errorCode("update_source_failed", "reading what %s has failed: %s", "source", "reason")
	codeUpdateWindowBad      = errorCode("update_window_bad", "%q: want a window HH:MM-HH:MM in the server's time, such as %s", "window", "example")
	codeUpdateWindowShort    = errorCode("update_window_short", "%q: the window is shorter than %d minutes", "window", "min")
	codeIntegrationsNotAsked = errorCode("integrations_lan_not_asked", "the LAN not asked: %s", "reason")
)

// The computer's reboot that failed, as GET /api/v1/reboot says it last
// did: the helper refused it, or the computer is still up after it.
var (
	codeRebootFailed  = errorCode("reboot_failed", "the reboot failed: %s", "reason")
	codeRebootNotDone = errorCode("reboot_not_done", "the computer has not rebooted within %s; the reboot helper's log says why", "after")
)

// The store's users, roles, tokens, providers, identities and
// integrations (storeCodes), a new password's rule (passwordRefusal),
// a provider's role and issuer, and the signed-in user's preferences
// and stream tickets. The store's sentinels name no value: a code of
// one names none either, and its English text is the store's.
var (
	codeUserNotFound           = errorCode("user_not_found", "no such user")
	codeRoleNotFound           = errorCode("role_not_found", "no such role")
	codeTokenNotFound          = errorCode("token_not_found", "no such token")
	codeUserExists             = errorCode("user_exists", "user exists")
	codeRoleExists             = errorCode("role_exists", "role exists")
	codeUserLastOwner          = errorCode("user_last_owner", "the last enabled owner without an end date cannot be disabled, removed, demoted or given one")
	codeRoleInUse              = errorCode("role_in_use", "the role still has users")
	codeRoleKeyBad             = errorCode("role_key_bad", "bad role key")
	codeRoleImmutable          = errorCode("role_immutable", "the role cannot be changed")
	codeUserIntegrationAlone   = errorCode("user_integration_alone", "an integration's user acts through its integration alone")
	codeOpaqueNoKeys           = errorCode("opaque_no_keys", "no OPAQUE keys: opaque.key is not readable in a store opened read-only")
	codeUserExpiresAtBad       = errorCode("user_expires_at_bad", "expires_at: want a time such as 2026-12-24T18:00:00Z, or null")
	codePasswordTooShort       = errorCode("password_too_short", "password: at least %d characters", "min")
	codePasswordTooLong        = errorCode("password_too_long", "password: at most %d characters", "max")
	codePasswordHasName        = errorCode("password_has_name", "password: contains the user's or the server's name")
	codeProviderNotFound       = errorCode("provider_not_found", "no such provider")
	codeProviderExists         = errorCode("provider_exists", "provider exists")
	codeProviderBad            = errorCode("provider_bad", "bad provider: %s", "reason")
	codeProviderAutoRoleOwner  = errorCode("provider_auto_role_owner", "the role %s may not be given to users a provider creates", "role")
	codeProviderAutoRoleAdmin  = errorCode("provider_auto_role_admin", "the role %s holds %s, which users a provider creates may not have", "role", "capability")
	codeProviderUnreached      = errorCode("provider_unreached", "provider %s: %s", "provider", "reason")
	codeProviderIssuerUnreach  = errorCode("provider_issuer_unreached", "the provider was not reached at %s: %s", "issuer", "reason")
	codeProviderIssuerOther    = errorCode("provider_issuer_other", "the provider calls itself %q, not %q", "named", "issuer")
	codeIdentityNotFound       = errorCode("identity_not_found", "no such identity")
	codeIdentityExists         = errorCode("identity_exists", "identity exists")
	codeIdentityUsed           = errorCode("identity_used", "the identity was used for a sign-in")
	codeEmailBad               = errorCode("email_bad", "bad e-mail")
	codeIntegrationNotFound    = errorCode("integration_not_found", "no such integration")
	codeIntegrationNotPending  = errorCode("integration_not_pending", "the integration is not a pending request")
	codeIntegrationRequestGone = errorCode("integration_request_gone", "the request was refused, expired, or its key collected and used")
	codeIntegrationsWaiting    = errorCode("integration_requests_waiting", "too many registration requests waiting")
	codeIntegrationLinkBad     = errorCode("integration_link_bad", "an integration's user links to a Massimo user of its own")
	codePreferenceValueBad     = errorCode("preference_value_bad", "the preference %s is not one the server reads: %s", "preference", "reason")
	codePreferenceTooLarge     = errorCode("preference_too_large", "a preference holds at most %d bytes", "max")
	codeStreamTicketsTooMany   = errorCode("stream_tickets_too_many", "too many stream tickets outstanding")
)

// passed is err, a Go error a site passes on with status, as the
// refusal a page meets: a refusal as it is; else c with args and err's
// text as c's last parameter, its reason, the English text err's own.
// TestErrorCodes holds a call to as many args as c names before its
// reason.
func passed(status int, err error, c *errCode, args ...any) error {
	var he *httpError
	if errors.As(err, &he) {
		return he
	}
	he = c.refusal(status, append(args, err.Error())...)
	he.msg = err.Error()
	return he
}

// saying is err, a refusal (refuse), with prefix before its English
// text, its status, code and params as they were: a site that names the
// field or the step it came from in the server's words. Any other error
// is left as it is.
func saying(prefix string, err error) error {
	var he *httpError
	if !errors.As(err, &he) {
		return err
	}
	cp := *he
	cp.msg = prefix + he.msg
	return &cp
}

// commandCode is the code of an error a command to the Cube ended
// with, an engine's or a link's (engine.Result), nil for one of no kind
// it knows.
func commandCode(err error) *errCode {
	switch {
	case errors.Is(err, engine.ErrSuperseded):
		return codeCommandSuperseded
	case errors.Is(err, engine.ErrDiscarded):
		return codeCommandDiscarded
	case errors.Is(err, engine.ErrUnwanted):
		return codeCommandUnwanted
	case errors.Is(err, cubelan.ErrNotConnected):
		return codeCubeNotConnected
	case errors.Is(err, cubelan.ErrBusy):
		return codeCubeBusy
	case errors.Is(err, cubelan.ErrReplyTimeout):
		return codeCubeNoReply
	case errors.Is(err, context.DeadlineExceeded):
		return codeCommandTimedOut
	}
	return nil
}

// cubeRefusal is err, an error a command to the Cube ended with, as the
// refusal with status a page meets: a refusal as it is; else its own
// text with the code of its kind (commandCode), or command_failed with
// its text as the reason.
func cubeRefusal(status int, err error) error {
	var he *httpError
	if errors.As(err, &he) {
		return he
	}
	if c := commandCode(err); c != nil {
		he = c.refusal(status)
		he.msg = err.Error()
		return he
	}
	return passed(status, err, codeCommandFailed)
}

// failure is a failed step's or run's error as a result keeps it, in a
// command's row, a restore's run, the log and the views: its English
// text, and its code and params where it has them, which the page says
// in its language (refusalText in app.js); an error kept before it had
// a code keeps its text alone. Embedded, its fields stand beside the
// result's own: {"ok": false, "error": …, "code": …, "params": {…}}.
type failure struct {
	Error  string         `json:"error,omitempty"`
	Code   string         `json:"code,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

// asFailure is err as a result keeps it: its text, and the code and the
// params of a refusal, or the code of a command's error (commandCode);
// none for nil.
func asFailure(err error) failure {
	if err == nil {
		return failure{}
	}
	f := failure{Error: err.Error()}
	if name, params := codeOf(err); name != "" {
		f.Code, f.Params = name, params
	} else if c := commandCode(err); c != nil {
		f.Code = c.name
	}
	return f
}

// set puts f into m, a result kept as a map, beside its other fields:
// error, and code and params where f has them.
func (f failure) set(m map[string]any) {
	if f.Error == "" {
		return
	}
	m["error"] = f.Error
	if f.Code != "" {
		m["code"] = f.Code
	}
	if len(f.Params) > 0 {
		m["params"] = f.Params
	}
}

// setFailure gives the event err as its error: its text, and its code and
// params (asFailure).
func (ev *event) setFailure(err error) {
	f := asFailure(err)
	ev.Error, ev.Code, ev.Params = f.Error, f.Code, f.Params
}

// blocking is why a view's action is refused now, as GET
// /api/v1/reboot, /api/v1/restart and /api/v1/update say it: blocked,
// the refusal's English text, as before the codes, and its code and
// params beside it as blocked_code and blocked_params, which the page
// says in its language as it says a refusal.
type blocking struct {
	Blocked       string         `json:"blocked,omitempty"`
	BlockedCode   string         `json:"blocked_code,omitempty"`
	BlockedParams map[string]any `json:"blocked_params,omitempty"`
}

// blockingOf is why, the refusal an action meets now, as a view's
// blocking; none for nil.
func blockingOf(why error) blocking {
	f := asFailure(why)
	return blocking{Blocked: f.Error, BlockedCode: f.Code, BlockedParams: f.Params}
}

// storeRefusal is err, an error of the store's users, roles, tokens,
// Cubes, providers, identities and integrations, as the refusal a page
// meets, its English text text: the status and the code of its sentinel
// (storeCodes), nil where it is of no kind a page meets.
func storeRefusal(err error, text string) error {
	for _, m := range storeCodes {
		if errors.Is(err, m.sentinel) {
			he := m.code.refusal(m.status)
			he.msg = text
			return he
		}
	}
	return nil
}

// storeCodes are the store's sentinels a page meets, each with its
// status and its code; one that takes a parameter is mapped at its site.
var storeCodes = []struct {
	sentinel error
	status   int
	code     *errCode
}{
	{store.ErrNoUser, http.StatusNotFound, codeUserNotFound},
	{store.ErrNoRole, http.StatusNotFound, codeRoleNotFound},
	{store.ErrNoToken, http.StatusNotFound, codeTokenNotFound},
	{store.ErrUserExists, http.StatusConflict, codeUserExists},
	{store.ErrRoleExists, http.StatusConflict, codeRoleExists},
	{store.ErrLastOwner, http.StatusConflict, codeUserLastOwner},
	{store.ErrRoleInUse, http.StatusConflict, codeRoleInUse},
	{store.ErrBadRole, http.StatusBadRequest, codeRoleKeyBad},
	{store.ErrNoOpaque, http.StatusServiceUnavailable, codeOpaqueNoKeys},
	{store.ErrImmutable, http.StatusForbidden, codeRoleImmutable},
	{store.ErrIntegrationUser, http.StatusForbidden, codeUserIntegrationAlone},
	{store.ErrNoProvider, http.StatusNotFound, codeProviderNotFound},
	{store.ErrNoIdentity, http.StatusNotFound, codeIdentityNotFound},
	{store.ErrProviderExists, http.StatusConflict, codeProviderExists},
	{store.ErrIdentityExists, http.StatusConflict, codeIdentityExists},
	{store.ErrIdentityUsed, http.StatusConflict, codeIdentityUsed},
	{store.ErrBadEmail, http.StatusBadRequest, codeEmailBad},
	{store.ErrNoIntegration, http.StatusNotFound, codeIntegrationNotFound},
	{store.ErrNotPending, http.StatusGone, codeIntegrationNotPending},
	{store.ErrRequestGone, http.StatusGone, codeIntegrationRequestGone},
	{store.ErrBadLink, http.StatusBadRequest, codeIntegrationLinkBad},
	{store.ErrNotFound, http.StatusNotFound, codeCubeRowNotFound},
	{store.ErrExists, http.StatusConflict, codeCubeRowExists},
}

// backupNotTaken is the refusal of a backup backup.Take did not take,
// the one way it fails: the Cube has sent no greeting yet, its English
// text Take's.
func backupNotTaken(err error) error {
	return refuseSaying(http.StatusServiceUnavailable, err.Error(), codeCubeNotConnectedYet)
}

// passwordRefusal is the refusal of a new password that fails the rule
// of internal/password (password.Check), its English text the rule's:
// which rule, and its limit, so that the page says it as the server
// does, and no more.
func passwordRefusal(err error) error {
	switch {
	case errors.Is(err, password.ErrTooShort):
		return refuseSaying(http.StatusBadRequest, err.Error(), codePasswordTooShort, password.MinLength)
	case errors.Is(err, password.ErrTooLong):
		return refuseSaying(http.StatusBadRequest, err.Error(), codePasswordTooLong, password.MaxLength)
	case errors.Is(err, password.ErrContext):
		return refuseSaying(http.StatusBadRequest, err.Error(), codePasswordHasName)
	}
	return fail(http.StatusBadRequest, "%v", err)
}
