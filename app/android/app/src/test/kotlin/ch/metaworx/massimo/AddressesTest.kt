// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** Where the server marked "Open at start" opens (AP MobileApp, D43-A): the LAN address first, as the connect page chooses. */
class AddressesTest {
    private val lan = "http://192.168.1.20:8099/"
    private val remote = "https://massimo.example.net/"
    private val s = Server("s1", "Home", "node-a", lan, remote, "", Server.ASK_START, false, start = true)

    private fun ok(u: String, node: String = "node-a") = ProbeResult(u, ok = true, node = node)

    private fun refused(u: String) = ProbeResult(u, ok = false, error = "refused")

    // after waits ms, then answers with what.
    private fun after(ms: Long, what: ProbeResult): ProbeResult {
        Thread.sleep(ms)
        return what
    }

    @Test
    fun theFirstThatIsTheServer() {
        assertEquals(lan, Addresses.answering(s, listOf(ok(lan), ok(remote))))
        assertEquals("the LAN address is another server", remote, Addresses.answering(s, listOf(ok(lan, "node-b"), ok(remote))))
        assertNull(Addresses.answering(s, listOf(refused(lan), refused(remote))))
        // a server without a store has no node to match
        assertEquals(lan, Addresses.answering(s.copy(node = ""), listOf(ok(lan, ""), ok(remote))))
    }

    @Test
    fun theLanAddressWinsWithoutWaitingForTheRemoteOne() {
        val start = System.nanoTime()
        val got = Addresses.pick(s, { u -> if (u == lan) ok(u) else after(3000, ok(u)) }, graceMs = 200)
        assertEquals(lan, got)
        assertTrue("waited for the remote address", (System.nanoTime() - start) / 1_000_000 < 2000)
    }

    @Test
    fun awayFromHomeTheRemoteOneAfterAShortGrace() {
        val start = System.nanoTime()
        // the LAN address times out, the remote one answers at once
        val got = Addresses.pick(s, { u -> if (u == lan) after(3000, refused(u)) else ok(u) }, graceMs = 200)
        val ms = (System.nanoTime() - start) / 1_000_000
        assertEquals(remote, got)
        assertTrue("waited $ms ms for the LAN address's timeout", ms < 2000)
    }

    @Test
    fun theLanAddressStillWinsWithinTheGrace() {
        assertEquals(lan, Addresses.pick(s, { u -> if (u == lan) after(100, ok(u)) else ok(u) }, graceMs = 1000))
    }

    @Test
    fun aLanAddressThatFailsLeavesTheRemoteOne() {
        assertEquals(remote, Addresses.pick(s, { u -> if (u == lan) refused(u) else after(100, ok(u)) }))
        assertEquals("the LAN one answers as another server", remote, Addresses.pick(s, { u -> if (u == lan) ok(u, "node-b") else after(100, ok(u)) }))
    }

    @Test
    fun noneAnsweringOpensTheList() {
        assertNull(Addresses.pick(s, { u -> refused(u) }))
        assertNull("a remote answer of another server", Addresses.pick(s, { u -> if (u == lan) refused(u) else ok(u, "node-b") }))
        assertNull("a probe that throws is no answer", Addresses.pick(s, { _ -> throw IllegalStateException("boom") }))
        assertNull(Addresses.pick(s.copy(lan = "", remote = ""), { u -> ok(u) }))
    }

    @Test
    fun oneAddressAlone() {
        assertEquals(remote, Addresses.pick(s.copy(lan = ""), { u -> ok(u) }))
        assertEquals(lan, Addresses.pick(s.copy(remote = ""), { u -> ok(u) }))
        assertNull(Addresses.pick(s.copy(remote = ""), { u -> refused(u) }))
    }
}
