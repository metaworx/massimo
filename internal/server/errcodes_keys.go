// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The codes of the refusals AP KeyLife added to the API keys' routes
// (users_api.go; docs/api-v1.md, "Tokens" and "Error codes"): an end
// date that has passed or is no time, the exclusion from the idle limit
// for who manages the key's user alone, a key's life and its new secret
// refused for a session, an integration's key, the key itself as the
// bearer and a key that has ended. The token routes' refusals from
// before KeyLife (the settings file's key, a token of another user, the
// sign-in's) are AP ErrorCodes' Block 4's.

// An API key's end date (AP KeyLife, Block 4): one not after the
// store's now, POST and PUT …/tokens; and an expires_at that reads as
// the zero time, which would else be taken for none, a user's too.
var (
	codeKeyEndPassed = errorCode("key_end_passed",
		"expires_at: %s has passed: an API key's end date is in the future; Revoke ends a key now", "expires_at")
	codeEndDateYearOne = errorCode("end_date_year_one",
		"expires_at: %s is no end date: want a time such as 2026-12-24T18:00:00Z, or null for none", "expires_at")
)

// A key kept from the idle limit, or held to it again, by a caller that
// does not manage the key's user (AP KeyLife, D7-A), at POST and PUT
// …/tokens.
var codeKeyKeepForbidden = errorCode("key_keep_forbidden",
	"only who manages user %s keeps a key from the idle limit, or holds it to it again", "user")

// A key's life, PUT …/tokens/{id} (AP KeyLife, Blocks 3 and 4): a
// session's, an integration's key kept from the limit, and the key
// itself as the bearer (D4-A).
var (
	codeKeyLifeSession = errorCode("key_life_session",
		"token %d is a session: it lasts 30 days from its last use, and only an API key's life changes", "token")
	codeKeyLifeIntegrationKept = errorCode("key_life_integration_kept",
		"the key is an integration's: it is always held to the idle limit, and its integration registers again once it ended")
	codeKeyLifeOwnKey = errorCode("key_life_own_key", "a key changes nothing of its own life: sign in, or send another key")
)

// A key's new secret, POST …/tokens/{id}/rekey (AP KeyLife, Block 5): a
// session's, an integration's key, the key itself as the bearer, and a
// key that has ended.
var (
	codeNewSecretSession = errorCode("new_secret_session",
		"token %d is a session: only an API key takes a new secret; a session ends with Revoke or a sign-out", "token")
	codeNewSecretIntegration = errorCode("new_secret_integration",
		"the key is an integration's: it takes no new secret, since the integration learns its key by its approval's code alone; revoke it, and the integration registers again")
	codeNewSecretOwnKey = errorCode("new_secret_own_key",
		"a key changes nothing of its own life, its secret included: sign in, or send another key")
	codeNewSecretEnded = errorCode("new_secret_ended",
		"key %d has ended: only a live key takes a new secret; move its end date later or keep it from the idle limit first, or create another key", "key")
)
