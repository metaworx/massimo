// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The codes of users, roles, the sign-ins, tokens, providers,
// integrations, settings, push, preferences and commands (AP
// ErrorCodes, Block 4), declared as errcodes.go declares its own. A
// sign-in's refusal says no more than its English text: a wrong name
// and a wrong password are one code, and so are a token nobody holds
// and one that ended.

import (
	"errors"
	"net/http"
)

// writeRefusal writes the answer of a refusal (refuse, refuseSaying)
// made before a handler runs or outside one, as the route wrapper
// writes a handler's: its status and its body, code and params beside
// its English text. Any other error is our own, a 500.
func writeRefusal(w http.ResponseWriter, err error) {
	var he *httpError
	if !errors.As(err, &he) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, he.status, he.body())
}

// withRetryAfter gives a refusal (refuse) its Retry-After, in seconds.
func withRetryAfter(err error, seconds int) error {
	var he *httpError
	if errors.As(err, &he) {
		he.retryAfter = seconds
	}
	return err
}

// The server's door and the token check (auth.go, server.go): a server
// that asks for no key answers its local network only; a write another
// site's page sent; a token nobody holds or that ended, alike; an
// address or a name locked out after failed checks; a user or a role
// that may not use the API now; a capability the caller lacks.
var (
	codeServerOpenLocalOnly = errorCode("server_open_local_only", openRefused)
	codeCrossOriginRefused  = errorCode("cross_origin_refused", "cross-origin request refused")
	codeTokenWrong          = errorCode("token_wrong", "missing or wrong token")
	codeAddressLockedOut    = errorCode("address_locked_out", "too many failed token checks from this address; try again later")
	codeUserLockedOut       = errorCode("user_locked_out", "too many failed logins for this user; try again later")
	codeUserAccountDisabled = errorCode("user_account_disabled", "user %s is disabled", "user")
	codeUserAccountExpired  = errorCode("user_account_expired", "user %s has expired", "user")
	codeRoleDisabled        = errorCode("role_disabled", "role %s is disabled", "role")
	codeCapabilityRequired  = errorCode("capability_required", "capability %q required", "capability")
)

// The sign-ins by password, by OPAQUE, by an API key and through a
// provider (login_api.go, opaque_api.go, oidc.go): a server with no
// users to sign in as; the password's way off; one sign-in from an
// address at a time; a wrong name or password, which never says which;
// a sign-in started that is gone; the store's password checks all
// taken.
var (
	codeLoginNoStore        = errorCode("login_no_store", "this server has no store: use the API token")
	codeLoginNoUsers        = errorCode("login_no_users", "no users yet: create the first owner with POST /api/v1/users, or massimo user add")
	codeLoginPasswordOff    = errorCode("login_password_off", "the password sign-in is off on this server; sign in from the web UI, or with an API key")
	codeLoginRunning        = errorCode("login_running", "a sign-in from this address is already running; try again when it is done")
	codeLoginWrong          = errorCode("login_wrong", "wrong user or password")
	codeLoginNotStarted     = errorCode("login_not_started", "no sign-in started under this login, or it took too long; start again")
	codeLoginIntegrationKey = errorCode("login_integration_key", "a registered integration's key makes no session")
	codePasswordBusy        = errorCode("password_busy", "too many password checks at once; try again in a few seconds")
	codeOidcCodeUnknown     = errorCode("oidc_code_unknown", "no sign-in under this code, or it took too long; sign in again")
	codeOidcUserGone        = errorCode("oidc_user_gone", "the user is gone; sign in again")
)

// The first owner, made on an open server (users_api.go, opaque_api.go).
var (
	codeFirstOwnerNotLocal       = errorCode("first_owner_not_local", "the first owner is created from the local network, or on the box with massimo user add")
	codeFirstOwnerPasswordNeeded = errorCode("first_owner_password_needed", "the first owner needs a password")
	codeFirstOwnerExists         = errorCode("first_owner_exists", "the first owner exists already: sign in")
)

// The users and their tokens (users_api.go, login_api.go): a user's
// name by the store's rule (store.ErrBadName, which the page's check of
// a new name words alike, src/shared/username.tsx); what the caller may
// change of a user and which roles it may give; the own password's
// proof; an integration's users; the settings file's key, which the API
// does not end (settingsKey): neither revoked nor removed with its user.
var (
	codeUserNoStore                = errorCode("user_no_store", "this server has no store, so no users: it takes the API token")
	codeUserUnknown                = errorCode("user_unknown", "no user %q", "user")
	codeUserNameBad                = errorCode("user_name_bad", "bad user name")
	codeUserDisplayNameLong        = errorCode("user_display_name_long", "display_name: at most %d bytes", "max")
	codeUserIntegrationNoWayIn     = errorCode("user_integration_no_way_in", "the user %s is an integration's: it acts through its integration alone, with no key, password or sign-in of its own", "user")
	codeUserIntegrationGuest       = errorCode("user_integration_guest", "the user %s is an integration's: its own integration, a holder of integrations or of users changes it", "user")
	codeUserIntegrationOwn         = errorCode("user_integration_own", "the user %s is the integration's own: remove the integration itself, DELETE /api/v1/integrations/%s, which keeps it to be attached again", "user", "integration")
	codeUserOwnerOnly              = errorCode("user_owner_only", "only an owner changes an owner")
	codeUserHoldsSettingsKey       = errorCode("user_holds_settings_key", "user %s holds the settings file's MASSIMO_API_TOKEN: change or remove it in the settings file first", "user")
	codeIntegrationOwnUsersOnly    = errorCode("integration_own_users_only", "a call through an integration changes its own integration's users alone")
	codeRoleBeyondYours            = errorCode("role_beyond_yours", "the role %s has capabilities yours lacks", "role")
	codeRoleBeyondIntegration      = errorCode("role_beyond_integration", "the role %s lies beyond the maximum of its integration", "role")
	codeRoleIntegrationOnly        = errorCode("role_integration_only", "the role integration is an integration's and its users' alone")
	codeRoleOwnerOnly              = errorCode("role_owner_only", "only an owner gives the role owner")
	codeCapabilityRequiredToGive   = errorCode("capability_required_to_give", "capability %q required to give the role %s", "capability", "role")
	codeCapabilityRequiredToCreate = errorCode("capability_required_to_create", "capability %q required to create a user with the role %s", "capability", "role")
	codePasswordProofWrong         = errorCode("password_proof_wrong", "proof is wrong, used or too old: sign in again to prove the current password")
	codePasswordCurrentWrong       = errorCode("password_current_wrong", "current_password is wrong")
	codeTokenLabelLength           = errorCode("token_label_length", "label: 1 to %d bytes", "max")
	codeTokenUnknown               = errorCode("token_unknown", "user %s has no token %d", "user", "token")
	codeTokenSettingsKey           = errorCode("token_settings_key", "the token is the settings file's MASSIMO_API_TOKEN: change or remove it in the settings file")
)

// The roles and the capabilities (roles_api.go): a capability that is
// none, in a role's set (400) and as a path's (404, in its own words).
var (
	codeCapabilityUnknown = errorCode("capability_unknown", "unknown capability %q", "capability")
	codeRoleNameLong      = errorCode("role_name_long", "name: at most %d bytes", "max")
)

// The sign-in providers and the accounts linked at them
// (providers_api.go, oidc.go): a provider that is none or disabled; the
// last way a user signs in, which it keeps; a link of one's own only,
// waiting under its code a short while; an account another user has.
var (
	codeProviderUnknown        = errorCode("provider_unknown", "no provider %q", "provider")
	codeProviderKeyNeeded      = errorCode("provider_key_needed", "key is needed")
	codeIdentityLastWayIn      = errorCode("identity_last_way_in", "this is the last way you sign in: set a password or link another account first")
	codeIdentityLinkOwnOnly    = errorCode("identity_link_own_only", "a user links only its own accounts")
	codeIdentityConfirmOwnOnly = errorCode("identity_confirm_own_only", "a user confirms only its own links")
	codeIdentityLinkUnknown    = errorCode("identity_link_unknown", "no link waits under this code, or it took too long; link again")
	codeIdentityTaken          = errorCode("identity_taken", "the account is another user's")
)

// The integrations (integrations_api.go): registration requests off or
// too many from an address; a maximum beyond the role integration; an
// integration's name; an integration that is none, or still a request.
var (
	codeIntegrationRequestsOff      = errorCode("integration_requests_off", "this server takes no registration requests (setting %s)", "setting")
	codeIntegrationRequestsTooMany  = errorCode("integration_requests_too_many", "too many registration requests from this address")
	codeIntegrationCapabilityBeyond = errorCode("integration_capability_beyond", "max_caps: %q is not an integration's to have", "capability")
	codeIntegrationNameLength       = errorCode("integration_name_length", "name is 1 to %d bytes", "max")
	codeIntegrationUnknown          = errorCode("integration_unknown", "no integration %s", "integration")
)

// The settings the page sends (settings_api.go): auth.keep_open on a
// server with users; a value not among a setting's choices, each site
// in its own words; a duration outside its setting's range, with or
// without 0 for off.
var (
	codeSettingKeepOpenUsers    = errorCode("setting_keep_open_users", "%s counts only on a server without users: this one has users", "setting")
	codeSettingChoiceBad        = errorCode("setting_choice_bad", "%s: want one of %s", "setting", "choices")
	codeSettingDurationRange    = errorCode("setting_duration_range", "%s %s: between %s and %s", "setting", "value", "min", "max")
	codeSettingDurationRangeOff = errorCode("setting_duration_range_off", "%s %s: between %s and %s, or 0 for off", "setting", "value", "min", "max")
)

// The signed-in user's own preferences and push subscriptions
// (me_api.go, push_api.go); the commands and Idempotency-Key
// (commands.go).
var (
	codePreferenceNoUser        = errorCode("preference_no_user", "no user signed in: the preferences stay in the browser")
	codePreferenceUnknown       = errorCode("preference_unknown", "no preference %q", "preference")
	codePushSubscriptionUnknown = errorCode("push_subscription_unknown", "no subscription %d of yours", "subscription")
	codeIdempotencyRunning      = errorCode("idempotency_running", "a request with this Idempotency-Key is still running")
	codeIdempotencyReused       = errorCode("idempotency_reused", "Idempotency-Key reused for another request")
	codeCommandUnknown          = errorCode("command_unknown", "no command %q", "command")
)
