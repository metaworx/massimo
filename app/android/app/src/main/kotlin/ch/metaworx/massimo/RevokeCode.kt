// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import java.security.MessageDigest

/**
 * RevokeCode works out a token's revoke code (AP MobileApp v1.6, D112-A;
 * docs/app.md, "The revoke codes"): the hex of SHA-256 of
 * `massimo revoke v1` followed by the token as the server gave it, 64
 * lower-case hex digits, as the server's `store.RevokeCode` does; the one
 * vector `internal/store/tokens_test.go` and `RevokeCodeTest.kt` share
 * holds the two to each other. A code lets nothing in: the server ends the
 * token it names by it alone (`POST /api/v1/tokens/revoke`), so the app
 * keeps it unsealed, beside the record of a session or a key it forgets,
 * and posts it at the next contact with the server.
 */
object RevokeCode {
    private const val LABEL = "massimo revoke v1"
    private val FORM = Regex("^[0-9a-f]{64}$")

    /** of is the revoke code of the token [secret]. */
    fun of(secret: String): String =
        MessageDigest.getInstance("SHA-256").digest((LABEL + secret).toByteArray(Charsets.UTF_8)).joinToString("") { "%02x".format(it) }

    /** valid reports whether [code] has a code's form, so that a file's mistake is not posted. */
    fun valid(code: String): Boolean = FORM.matches(code)
}
