// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

/** Held is where the app holds a server's session: none, in its memory alone, or sealed (SessionKeeper). */
enum class Held { NONE, MEMORY, SEALED }

/**
 * Opening is what a tap on a server in the connect page's list does (AP
 * MobileApp, F40; D72-B, D74-A; docs/app.md, "The login by fingerprint"),
 * which the bridge's `open` applies, so that no page opens around it. A
 * prompt cancelled or failed opens the page at its sign-in card, the
 * session held withheld for that opening.
 */
enum class Opening {
    /** OPEN opens the page, with the session held, or at its card where none is. */
    OPEN,

    /** ASK asks for the fingerprint without the key, then opens the page with the session held. */
    ASK,

    /** LOGIN asks for the fingerprint with the kept key, then opens the page with the session it makes. */
    LOGIN,
    ;

    companion object {
        /**
         * of is what a tap does for a server asked at [ask] (one of
         * Server.ASKS), with a login by fingerprint [kept] or not, its
         * session [held] where it is: without a kept login the page opens,
         * with its session or at its card, as before; with one and no
         * session the prompt makes one with the key (D74-A); with one
         * and a session, a server asked at each return asks each time it
         * is opened (D72-B), any other opens with it, the fingerprint
         * having been asked in this run (at each start) or being asked
         * after a logout.
         */
        fun of(ask: String, kept: Boolean, held: Held): Opening = when {
            !kept -> OPEN
            held == Held.NONE -> LOGIN
            ask == Server.ASK_RETURN -> ASK
            else -> OPEN
        }
    }
}
