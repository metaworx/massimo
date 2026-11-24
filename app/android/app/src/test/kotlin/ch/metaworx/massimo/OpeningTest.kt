// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.junit.Assert.assertEquals
import org.junit.Test

/** What a tap on a server in the list does, row for row of the Analysis' table (AP MobileApp, "Opening from the list"). */
class OpeningTest {
    @Test
    fun withoutAKeptLoginThePageOpens() {
        // as before: with its session, or at its card
        for (ask in Server.ASKS) {
            for (held in Held.values()) assertEquals("$ask, $held", Opening.OPEN, Opening.of(ask, false, held))
        }
    }

    @Test
    fun askedAtEachReturnWithASessionInMemoryAsks() {
        // D72-B: each time it is opened from the list, the app hidden or not
        assertEquals(Opening.ASK, Opening.of(Server.ASK_RETURN, true, Held.MEMORY))
        // a sealed one is never the case of such a server, and asks all the same
        assertEquals(Opening.ASK, Opening.of(Server.ASK_RETURN, true, Held.SEALED))
    }

    @Test
    fun askedAtEachStartOrAfterALogoutWithASessionOpensWithIt() {
        // the fingerprint was asked in this run, or is asked after a logout
        assertEquals(Opening.OPEN, Opening.of(Server.ASK_START, true, Held.MEMORY))
        assertEquals(Opening.OPEN, Opening.of(Server.ASK_LOGOUT, true, Held.SEALED))
        assertEquals(Opening.OPEN, Opening.of(Server.ASK_START, true, Held.SEALED))
        assertEquals(Opening.OPEN, Opening.of(Server.ASK_LOGOUT, true, Held.MEMORY))
    }

    @Test
    fun aKeptLoginWithoutASessionLogsInWithTheKey() {
        // D74-A, whatever the server's choice
        for (ask in Server.ASKS) assertEquals(ask, Opening.LOGIN, Opening.of(ask, true, Held.NONE))
    }
}
