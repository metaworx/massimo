// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import java.net.InetAddress

class FoundTest {
    private fun ip(s: String): InetAddress = InetAddress.getByName(s)

    private val fp = Fingerprint.of(byteArrayOf(5))

    private val txt = mapOf<String, ByteArray?>(
        "node" to "node-a".toByteArray(),
        "v" to "massimo v0.1.0".toByteArray(),
        "tls" to "1".toByteArray(),
        "fp" to fp.lowercase().toByteArray(),
        "name" to "heating-box".toByteArray(),
    )

    @Test
    fun theEventOfTheAnnouncement() {
        val e = Found.event("massimo", listOf(ip("fd00::20"), ip("192.168.1.20")), 8099, txt)!!
        assertEquals("found", e.getString("type"))
        val s = e.getJSONObject("service")
        assertEquals("heating-box", s.getString("name"))
        assertEquals("192.168.1.20", s.getString("host"))
        assertEquals(8099, s.getInt("port"))
        assertEquals("node-a", s.getString("node"))
        assertEquals("massimo v0.1.0", s.getString("version"))
        assertEquals(true, s.getBoolean("tls"))
        assertEquals(fp, s.getString("fp"))
    }

    @Test
    fun whatTheAnnouncementLeavesOut() {
        val s = Found.event("massimo", listOf(ip("192.168.1.20")), 8099, mapOf("tls" to null))!!.getJSONObject("service")
        assertEquals("massimo", s.getString("name"))
        assertEquals("", s.getString("node"))
        assertEquals(false, s.getBoolean("tls"))
        assertEquals("", s.getString("fp"))
    }

    @Test
    fun theAddressToReachItAt() {
        // Android writes an IPv6 address compressed, the JVM in full: the same address.
        assertEquals(ip("fd00::20"), ip(Found.pick(listOf(ip("fe80::1"), ip("fd00::20")))!!))
        assertNull(Found.pick(listOf(ip("fe80::1"))))
        assertNull(Found.pick(listOf(ip("127.0.0.1"))))
        assertNull(Found.pick(emptyList()))
        assertNull(Found.event("massimo", listOf(ip("192.168.1.20")), 0, txt))
    }
}
