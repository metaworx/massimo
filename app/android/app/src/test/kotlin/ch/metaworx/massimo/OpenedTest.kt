// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.junit.Assert.assertFalse
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test

/** What an opening withholds from its page (AP MobileApp, F40; D72-B): on leaving for the list, and of a page's null. */
class OpenedTest {
    private val held = Bridge.Opened("0123456789abcdef01234567", "https://massimo.example.net", "https://massimo.example.net/")

    @Test
    fun leavingForTheListWithholdsAServerAskedAtEachReturn() {
        // Back from the list, or the page's own way back, must not reach the page past its prompt
        val left = held.leaving(guarded = true)
        assertTrue(left.withheld)
        assertSame("withheld already: the same opening", left, left.leaving(guarded = true))
        // any other server keeps its opening as it was: its rule opens it with its session
        assertSame(held, held.leaving(guarded = false))
        val atCard = held.copy(withheld = true)
        assertSame("a page at its card stays there", atCard, atCard.leaving(guarded = false))
    }

    @Test
    fun aPageAtItsCardForgetsNothing() {
        val atCard = held.copy(withheld = true)
        assertFalse("its null leaves the session withheld alone", atCard.takes(null))
        assertTrue("a session of its own replaces it", atCard.takes("session"))
        assertTrue(held.takes(null))
        assertTrue(held.takes("session"))
    }
}
